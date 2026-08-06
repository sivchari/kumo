package sfn

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Diagnostic severities and validation results, matching the exact wire
// values AWS documents for ValidateStateMachineDefinition.
const (
	diagnosticSeverityError   = "ERROR"
	diagnosticSeverityWarning = "WARNING"

	validationResultOK   = "OK"
	validationResultFail = "FAIL"

	// defaultValidateMaxResults matches AWS's own documented default and cap
	// on the number of diagnostics returned per call.
	defaultValidateMaxResults = 100
)

// Diagnostic codes. First block reuses AWS's documented codes; second block
// is best-effort naming for checks AWS does not document a code for.
const (
	codeInvalidJSONDescription  = "INVALID_JSON_DESCRIPTION"
	codeSchemaValidationFailed  = "SCHEMA_VALIDATION_FAILED"
	codeInvalidResource         = "INVALID_RESOURCE"
	codeMissingEndState         = "MISSING_END_STATE"
	codeMissingTransitionTarget = "MISSING_TRANSITION_TARGET"

	codeUnreachableState            = "UNREACHABLE_STATE"
	codeUnsupportedRuntimeConstruct = "UNSUPPORTED_RUNTIME_CONSTRUCT"
)

// terminalStateTypes lists ASL state Types that need neither "Next" nor
// "End": true, per their own spec rules: Choice always branches via
// Choices/Default, and Succeed/Fail are always terminal.
var terminalStateTypes = map[string]bool{
	"Choice":  true,
	"Succeed": true,
	"Fail":    true,
}

// pathFieldTypes, resultFieldTypes, resultSelectorTypes, and
// retryCatchTypes are the state Types the ASL spec's state-type/field table
// (https://states-language.net/spec.html#statetypetable) allows each
// data-flow field group on: InputPath/OutputPath on all but Fail;
// ResultPath/Parameters on Task/Parallel/Map/Pass; ResultSelector on
// Task/Parallel/Map; Retry/Catch on Task/Parallel/Map.
var (
	pathFieldTypes = map[string]bool{
		"Task": true, "Parallel": true, "Map": true, "Pass": true,
		"Wait": true, "Choice": true, "Succeed": true,
	}
	resultFieldTypes      = map[string]bool{"Task": true, "Parallel": true, "Map": true, "Pass": true}
	resultSelectorTypes   = map[string]bool{"Task": true, "Parallel": true, "Map": true}
	retryCatchFieldTypes  = map[string]bool{"Task": true, "Parallel": true, "Map": true}
	stateDataFlowFieldSet = []stateFieldRule{
		{"InputPath", func(s *stateDefinition) bool { return len(s.InputPath) > 0 }, pathFieldTypes},
		{"OutputPath", func(s *stateDefinition) bool { return len(s.OutputPath) > 0 }, pathFieldTypes},
		{"ResultPath", func(s *stateDefinition) bool { return len(s.ResultPath) > 0 }, resultFieldTypes},
		{"Parameters", func(s *stateDefinition) bool { return s.Parameters != nil }, resultFieldTypes},
		{"ResultSelector", func(s *stateDefinition) bool { return s.ResultSelector != nil }, resultSelectorTypes},
		{"Retry", func(s *stateDefinition) bool { return len(s.Retry) > 0 }, retryCatchFieldTypes},
		{"Catch", func(s *stateDefinition) bool { return len(s.Catch) > 0 }, retryCatchFieldTypes},
	}
)

// stateFieldRule checks one JSON field's applicability across state Types:
// isSet reports whether the field is present on a given state, and allowed
// is the set of state Types the spec permits it on.
type stateFieldRule struct {
	name    string
	isSet   func(*stateDefinition) bool
	allowed map[string]bool
}

// mapDistributedOnlyFields are Map state fields AWS documents only under
// Distributed mode (see requireDistributedModeForFields in mapstate.go):
// absent from AWS's Inline Map field list, present only in Distributed Map's.
var mapDistributedOnlyFields = []struct {
	name  string
	isSet func(*stateDefinition) bool
}{
	{"ItemReader", func(s *stateDefinition) bool { return len(s.ItemReader) > 0 }},
	{"ItemBatcher", func(s *stateDefinition) bool { return len(s.ItemBatcher) > 0 }},
	{"ResultWriter", func(s *stateDefinition) bool { return len(s.ResultWriter) > 0 }},
	{"ToleratedFailurePercentage", func(s *stateDefinition) bool { return s.ToleratedFailurePercentage != nil }},
	{"ToleratedFailureCount", func(s *stateDefinition) bool { return s.ToleratedFailureCount != nil }},
}

// validateStateMachineDefinition runs kumo's best-effort structural checks
// and returns diagnostics in AWS's documented shape. Definitions
// CreateStateMachine already accepts must keep validating with no ERROR
// diagnostics -- only genuinely broken or kumo-unsupported constructs are
// flagged.
func validateStateMachineDefinition(definition string) []validateDiagnostic {
	var def stateMachineDefinition
	if err := json.Unmarshal([]byte(definition), &def); err != nil {
		return []validateDiagnostic{{
			Severity: diagnosticSeverityError,
			Code:     codeInvalidJSONDescription,
			Message:  fmt.Sprintf("Failed to parse the state machine definition: %v", err),
		}}
	}

	v := &definitionValidator{diagnostics: []validateDiagnostic{}}
	v.validateStateMachine(&def, "")

	return v.diagnostics
}

// definitionValidator accumulates diagnostics while walking a state machine
// definition and its nested Parallel Branches / Map ItemProcessor
// sub-definitions.
type definitionValidator struct {
	diagnostics []validateDiagnostic
}

func (v *definitionValidator) addError(code, location, message string) {
	v.diagnostics = append(v.diagnostics, validateDiagnostic{
		Severity: diagnosticSeverityError,
		Code:     code,
		Message:  message,
		Location: location,
	})
}

func (v *definitionValidator) addWarning(code, location, message string) {
	v.diagnostics = append(v.diagnostics, validateDiagnostic{
		Severity: diagnosticSeverityWarning,
		Code:     code,
		Message:  message,
		Location: location,
	})
}

// validateStateMachine validates one state machine's worth of States (the
// top-level definition, or a nested Branch/ItemProcessor), prefixing each
// diagnostic's location with locationPrefix.
func (v *definitionValidator) validateStateMachine(def *stateMachineDefinition, locationPrefix string) {
	if def.StartAt == "" {
		v.addError(codeSchemaValidationFailed, locationPrefix+"/StartAt", "state machine is missing required field \"StartAt\"")
	}

	if len(def.States) == 0 {
		v.addError(codeSchemaValidationFailed, locationPrefix+"/States", "state machine is missing required field \"States\"")

		return
	}

	if def.StartAt != "" {
		if _, ok := def.States[def.StartAt]; !ok {
			v.addError(codeMissingTransitionTarget, locationPrefix+"/StartAt",
				fmt.Sprintf("StartAt %q does not refer to a state in \"States\"", def.StartAt))
		}
	}

	names := sortedStateNames(def.States)

	for _, name := range names {
		state := def.States[name]
		v.validateState(name, &state, def.States, locationPrefix)
	}

	v.warnUnreachableStates(def, locationPrefix, names)
}

// sortedStateNames returns a state machine's state names in a deterministic
// order, so diagnostics are reported in a stable order across calls despite
// Go's randomized map iteration.
func sortedStateNames(states map[string]stateDefinition) []string {
	names := make([]string, 0, len(states))
	for name := range states {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// validateState runs every check that applies to a single state: transition
// targets, the Next-or-End requirement, Catch entries, and any type-specific
// required fields.
func (v *definitionValidator) validateState(name string, state *stateDefinition, states map[string]stateDefinition, locationPrefix string) {
	location := locationPrefix + "/States/" + name

	if state.Next != "" {
		if _, ok := states[state.Next]; !ok {
			v.addError(codeMissingTransitionTarget, location+"/Next",
				fmt.Sprintf("state %q: Next %q does not refer to a state in \"States\"", name, state.Next))
		}
	}

	if !state.End && state.Next == "" && !terminalStateTypes[state.Type] {
		v.addError(codeMissingEndState, location, fmt.Sprintf("state %q must have either \"Next\" or \"End\": true", name))
	}

	v.validateCatchers(name, state, states, location)
	v.validateStateFields(name, state, location)

	switch state.Type {
	case "Task":
		v.validateTaskState(name, state, location)
	case "Choice":
		v.validateChoiceState(name, state, states, location)
	case "Parallel":
		v.validateParallelState(name, state, location)
	case "Map":
		v.validateMapState(name, state, location)
	case "Wait":
		v.validateWaitState(name, state, location)
	}
}

// validateStateFields flags a data-flow field set on a state Type the
// spec's field table disallows (e.g. ResultPath on a Wait state), so a
// definition that validates doesn't still surprise the caller at runtime.
func (v *definitionValidator) validateStateFields(name string, state *stateDefinition, location string) {
	for _, rule := range stateDataFlowFieldSet {
		if !rule.isSet(state) || rule.allowed[state.Type] {
			continue
		}

		v.addError(codeSchemaValidationFailed, location+"/"+rule.name, fmt.Sprintf(
			"state %q: %q is not a supported field for %s states", name, rule.name, state.Type,
		))
	}
}

// validateCatchers checks every Catch entry's Next reference. ASL restricts
// Catch to Task, Parallel and Map states, but the check itself is generic:
// it simply validates whatever Catch entries are present.
func (v *definitionValidator) validateCatchers(name string, state *stateDefinition, states map[string]stateDefinition, location string) {
	for i, c := range state.Catch {
		catchLocation := fmt.Sprintf("%s/Catch/%d/Next", location, i)

		if c.Next == "" {
			v.addError(codeMissingTransitionTarget, catchLocation,
				fmt.Sprintf("state %q: Catch entry %d is missing required field \"Next\"", name, i))

			continue
		}

		if _, ok := states[c.Next]; !ok {
			v.addError(codeMissingTransitionTarget, catchLocation,
				fmt.Sprintf("state %q: Catch entry %d's Next %q does not refer to a state in \"States\"", name, i, c.Next))
		}
	}
}

// validateTaskState checks Resource is set, and for .waitForTaskToken
// tasks, that Parameters references $$.Task.Token -- otherwise no caller
// can ever resolve it and the state can only fail with States.Timeout. AWS
// rejects this at CreateStateMachine time.
func (v *definitionValidator) validateTaskState(name string, state *stateDefinition, location string) {
	if state.Resource == "" {
		v.addError(codeInvalidResource, location+"/Resource", fmt.Sprintf("Task state %q is missing required field \"Resource\"", name))

		return
	}

	if _, ok := isCallbackResource(state.Resource); ok && !hasTaskTokenReference(state.Parameters) {
		v.addError(codeSchemaValidationFailed, location+"/Parameters", fmt.Sprintf(
			"Task state %q uses %q, so \"Parameters\" must reference the task token via \"$$.Task.Token\"", name, callbackResourceSuffix,
		))
	}
}

// hasTaskTokenReference reports whether params contains a "$$.Task.Token"
// JSONPath reference anywhere, including nested Payload Template objects.
func hasTaskTokenReference(params map[string]any) bool {
	for key, value := range params {
		if strings.HasSuffix(key, ".$") && value == "$$.Task.Token" {
			return true
		}

		if sub, ok := value.(map[string]any); ok && hasTaskTokenReference(sub) {
			return true
		}
	}

	return false
}

// validateChoiceState checks a Choice state's Choices (non-empty, each with
// a valid Next) and Default (if set, a valid transition target).
func (v *definitionValidator) validateChoiceState(name string, state *stateDefinition, states map[string]stateDefinition, location string) {
	if len(state.Choices) == 0 {
		v.addError(codeSchemaValidationFailed, location+"/Choices", fmt.Sprintf("Choice state %q requires at least one entry in \"Choices\"", name))
	}

	for i := range state.Choices {
		choice := &state.Choices[i]
		choiceLocation := fmt.Sprintf("%s/Choices/%d/Next", location, i)

		if choice.Next == "" {
			v.addError(codeMissingTransitionTarget, choiceLocation,
				fmt.Sprintf("state %q: Choices entry %d is missing required field \"Next\"", name, i))

			continue
		}

		if _, ok := states[choice.Next]; !ok {
			v.addError(codeMissingTransitionTarget, choiceLocation,
				fmt.Sprintf("state %q: Choices entry %d's Next %q does not refer to a state in \"States\"", name, i, choice.Next))
		}
	}

	if state.Default != "" {
		if _, ok := states[state.Default]; !ok {
			v.addError(codeMissingTransitionTarget, location+"/Default",
				fmt.Sprintf("state %q: Default %q does not refer to a state in \"States\"", name, state.Default))
		}
	}
}

// validateParallelState checks a Parallel state's Branches (non-empty) and
// recursively validates each branch as its own state machine.
func (v *definitionValidator) validateParallelState(name string, state *stateDefinition, location string) {
	if len(state.Branches) == 0 {
		v.addError(codeSchemaValidationFailed, location+"/Branches", fmt.Sprintf("Parallel state %q requires at least one entry in \"Branches\"", name))

		return
	}

	for i := range state.Branches {
		v.validateStateMachine(&state.Branches[i], fmt.Sprintf("%s/Branches/%d", location, i))
	}
}

// validateMapState checks a Map state's ItemProcessor/Iterator (required,
// recursively validated), the Distributed-mode-only field rules, and
// ASL/kumo constructs still not implemented.
func (v *definitionValidator) validateMapState(name string, state *stateDefinition, location string) {
	var processor *stateMachineDefinition

	switch {
	case state.ItemProcessor != nil:
		processor = state.ItemProcessor
		v.validateStateMachine(processor, location+"/ItemProcessor")
	case state.Iterator != nil:
		processor = state.Iterator
		v.validateStateMachine(processor, location+"/Iterator")
	default:
		v.addError(codeSchemaValidationFailed, location, fmt.Sprintf("Map state %q requires \"ItemProcessor\" (or the legacy \"Iterator\")", name))
	}

	v.validateMapDistributedFields(name, state, processor, location)
}

// validateMapDistributedFields checks the Distributed-mode rules
// requireDistributedModeForFields enforces at runtime, and flags
// unimplemented constructs. processor is nil when neither ItemProcessor nor
// Iterator was set (already reported by validateMapState).
func (v *definitionValidator) validateMapDistributedFields(name string, state *stateDefinition, processor *stateMachineDefinition, location string) {
	if processor != nil && isDistributedMode(processor) {
		v.validateDistributedExecutionType(name, processor, location)
	} else {
		v.warnDistributedOnlyFields(name, state, location)
	}

	v.warnUnimplementedMapConstructs(name, state, location)
}

// validateDistributedExecutionType checks AWS's documented requirement
// that ProcessorConfig.Mode "DISTRIBUTED" must also set ExecutionType.
func (v *definitionValidator) validateDistributedExecutionType(name string, processor *stateMachineDefinition, location string) {
	if processor.ProcessorConfig.ExecutionType != "" {
		return
	}

	v.addError(codeSchemaValidationFailed, location+"/ItemProcessor/ProcessorConfig/ExecutionType", fmt.Sprintf(
		"Map state %q: ProcessorConfig.Mode %q requires ProcessorConfig.ExecutionType", name, mapProcessorModeDistributed,
	))
}

// warnDistributedOnlyFields flags Distributed-mode-only Map fields (see
// mapDistributedOnlyFields) set without ProcessorConfig.Mode "DISTRIBUTED":
// requireDistributedModeForFields fails these at runtime (issue #861).
func (v *definitionValidator) warnDistributedOnlyFields(name string, state *stateDefinition, location string) {
	for _, field := range mapDistributedOnlyFields {
		if !field.isSet(state) {
			continue
		}

		v.addWarning(codeUnsupportedRuntimeConstruct, location+"/"+field.name, fmt.Sprintf(
			"Map state %q sets %q, which requires ItemProcessor.ProcessorConfig.Mode %q; without it, this field fails at runtime",
			name, field.name, mapProcessorModeDistributed,
		))
	}
}

// warnUnimplementedMapConstructs flags ItemReader/ResultWriter constructs
// kumo's engine does not implement at all, so a definition that
// structurally validates doesn't surprise the caller at runtime.
func (v *definitionValidator) warnUnimplementedMapConstructs(name string, state *stateDefinition, location string) {
	v.warnUnimplementedItemReaderConstructs(name, state.ItemReader, location)
	v.warnUnimplementedResultWriterConstructs(name, state.ResultWriter, location)
}

// warnUnimplementedItemReaderConstructs flags an ItemReader Resource kumo
// does not implement: only s3:getObject and s3:listObjectsV2 are supported
// (see itemreader.go).
func (v *definitionValidator) warnUnimplementedItemReaderConstructs(name string, raw json.RawMessage, location string) {
	var def itemReaderDef
	if len(raw) == 0 || json.Unmarshal(raw, &def) != nil {
		return
	}

	if def.Resource != "" && def.Resource != itemReaderResourceS3GetObject && def.Resource != itemReaderResourceS3ListObjectsV2 {
		v.addWarning(codeUnsupportedRuntimeConstruct, location+"/ItemReader/Resource", fmt.Sprintf(
			"Map state %q: ItemReader Resource %q is not implemented; only %q and %q are supported",
			name, def.Resource, itemReaderResourceS3GetObject, itemReaderResourceS3ListObjectsV2,
		))
	}

	if strings.EqualFold(def.ReaderConfig.Transformation, itemReaderTransformationLoadAndFlatten) {
		v.addWarning(codeUnsupportedRuntimeConstruct, location+"/ItemReader/ReaderConfig/Transformation", fmt.Sprintf(
			"Map state %q: ItemReader ReaderConfig.Transformation %q is not implemented", name, itemReaderTransformationLoadAndFlatten,
		))
	}
}

// warnUnimplementedResultWriterConstructs flags ResultWriter combinations
// kumo doesn't implement: Transformation "NONE", and WriterConfig without
// Resource/Parameters ("preview" mode, valid per AWS but unimplemented).
func (v *definitionValidator) warnUnimplementedResultWriterConstructs(name string, raw json.RawMessage, location string) {
	var def resultWriterDef
	if len(raw) == 0 || json.Unmarshal(raw, &def) != nil || len(def.WriterConfig) == 0 {
		return
	}

	var cfg resultWriterConfig
	if json.Unmarshal(def.WriterConfig, &cfg) != nil {
		return
	}

	if strings.EqualFold(cfg.Transformation, resultWriterTransformationNone) {
		v.addWarning(codeUnsupportedRuntimeConstruct, location+"/ResultWriter/WriterConfig/Transformation", fmt.Sprintf(
			"Map state %q: ResultWriter WriterConfig.Transformation %q is not implemented; use COMPACT or FLATTEN instead", name, resultWriterTransformationNone,
		))
	}

	if def.Resource == "" {
		v.addWarning(codeUnsupportedRuntimeConstruct, location+"/ResultWriter/WriterConfig", fmt.Sprintf(
			"Map state %q: ResultWriter WriterConfig without Resource/Parameters (preview mode) is not implemented", name,
		))
	}
}

// validateWaitState checks that exactly one of Seconds, SecondsPath,
// Timestamp, TimestampPath is set, per the Amazon States Language spec.
func (v *definitionValidator) validateWaitState(name string, state *stateDefinition, location string) {
	set := 0

	for _, isSet := range []bool{
		state.Seconds != nil,
		state.SecondsPath != "",
		state.Timestamp != "",
		state.TimestampPath != "",
	} {
		if isSet {
			set++
		}
	}

	if set != 1 {
		v.addError(codeSchemaValidationFailed, location, fmt.Sprintf(
			"Wait state %q must set exactly one of \"Seconds\", \"SecondsPath\", \"Timestamp\", \"TimestampPath\" (found %d)",
			name, set,
		))
	}
}

// warnUnreachableStates flags every state that cannot be reached from
// StartAt by following Next, Default, Choices[].Next, or Catch[].Next
// edges.
func (v *definitionValidator) warnUnreachableStates(def *stateMachineDefinition, locationPrefix string, names []string) {
	if def.StartAt == "" {
		return
	}

	if _, ok := def.States[def.StartAt]; !ok {
		return
	}

	reachable := reachableStates(def)

	for _, name := range names {
		if !reachable[name] {
			v.addWarning(codeUnreachableState, locationPrefix+"/States/"+name, fmt.Sprintf("state %q is not reachable from \"StartAt\"", name))
		}
	}
}

// reachableStates returns the set of state names reachable from StartAt via
// breadth-first traversal of every transition edge.
func reachableStates(def *stateMachineDefinition) map[string]bool {
	reachable := map[string]bool{def.StartAt: true}
	queue := []string{def.StartAt}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		state, ok := def.States[current]
		if !ok {
			continue
		}

		for _, next := range stateTransitionTargets(&state) {
			if reachable[next] {
				continue
			}

			if _, exists := def.States[next]; !exists {
				continue
			}

			reachable[next] = true

			queue = append(queue, next)
		}
	}

	return reachable
}

// stateTransitionTargets lists every state name a state can transition to:
// Next, Default, each Choices[].Next, and each Catch[].Next.
func stateTransitionTargets(state *stateDefinition) []string {
	var targets []string

	if state.Next != "" {
		targets = append(targets, state.Next)
	}

	if state.Default != "" {
		targets = append(targets, state.Default)
	}

	for i := range state.Choices {
		if state.Choices[i].Next != "" {
			targets = append(targets, state.Choices[i].Next)
		}
	}

	for _, c := range state.Catch {
		if c.Next != "" {
			targets = append(targets, c.Next)
		}
	}

	return targets
}

// diagnosticsResult reports the ValidateStateMachineDefinition "result"
// value: FAIL if any ERROR-severity diagnostic is present, regardless of
// what the severity filter below ends up returning to the caller.
func diagnosticsResult(diagnostics []validateDiagnostic) string {
	for _, d := range diagnostics {
		if d.Severity == diagnosticSeverityError {
			return validationResultFail
		}
	}

	return validationResultOK
}

// filterAndCapDiagnostics applies the request's Severity filter (ERROR
// only, unless the caller asked for WARNING too) and MaxResults cap,
// reporting whether the result was truncated.
func filterAndCapDiagnostics(diagnostics []validateDiagnostic, severity string, maxResults int32) ([]validateDiagnostic, bool) {
	if severity != diagnosticSeverityWarning {
		filtered := make([]validateDiagnostic, 0, len(diagnostics))

		for _, d := range diagnostics {
			if d.Severity == diagnosticSeverityError {
				filtered = append(filtered, d)
			}
		}

		diagnostics = filtered
	}

	limit := int(maxResults)
	if limit <= 0 || limit > defaultValidateMaxResults {
		limit = defaultValidateMaxResults
	}

	if len(diagnostics) > limit {
		return diagnostics[:limit], true
	}

	return diagnostics, false
}
