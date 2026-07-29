package sfn

import (
	"encoding/json"
	"fmt"
	"sort"
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

// Diagnostic codes. The first block reuses AWS's own documented codes where
// kumo's check matches their meaning exactly. The second block is best
// effort: AWS does not document a code for these two checks, so kumo names
// them descriptively rather than guessing at an undocumented AWS string.
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

// mapEmulationGapFields are Map state fields the ASL spec defines but
// kumo's executor does not implement (see mapstate.go): ItemSelector,
// ItemBatcher and ResultWriter are unimplemented in every mode, and
// ItemReader is the marker for distributed-mode Map, which kumo only ever
// runs in inline (ItemProcessor/Iterator) mode.
var mapEmulationGapFields = []struct {
	name string
	get  func(*stateDefinition) json.RawMessage
}{
	{"ItemReader", func(s *stateDefinition) json.RawMessage { return s.ItemReader }},
	{"ItemSelector", func(s *stateDefinition) json.RawMessage { return s.ItemSelector }},
	{"ItemBatcher", func(s *stateDefinition) json.RawMessage { return s.ItemBatcher }},
	{"ResultWriter", func(s *stateDefinition) json.RawMessage { return s.ResultWriter }},
}

// validateStateMachineDefinition runs kumo's best-effort structural checks
// against a Step Functions definition and returns diagnostics in AWS's
// documented ValidateStateMachineDefinition shape. Definitions that
// CreateStateMachine already accepts must keep validating with no ERROR
// diagnostics; only genuinely broken or kumo-unsupported constructs are
// flagged, so this never rejects anything the engine can actually run.
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

// validateStateMachine validates one state machine's worth of States --
// the top-level definition, or a Parallel Branch / Map ItemProcessor
// sub-definition -- prefixing every diagnostic's location with
// locationPrefix so nested diagnostics point at the right JSON path.
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

// validateTaskState checks a Task state's required Resource field.
func (v *definitionValidator) validateTaskState(name string, state *stateDefinition, location string) {
	if state.Resource == "" {
		v.addError(codeInvalidResource, location+"/Resource", fmt.Sprintf("Task state %q is missing required field \"Resource\"", name))
	}
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
// recursively validated) and warns about ASL fields kumo's engine does not
// implement.
func (v *definitionValidator) validateMapState(name string, state *stateDefinition, location string) {
	switch {
	case state.ItemProcessor != nil:
		v.validateStateMachine(state.ItemProcessor, location+"/ItemProcessor")
	case state.Iterator != nil:
		v.validateStateMachine(state.Iterator, location+"/Iterator")
	default:
		v.addError(codeSchemaValidationFailed, location, fmt.Sprintf("Map state %q requires \"ItemProcessor\" (or the legacy \"Iterator\")", name))
	}

	v.warnUnsupportedMapFields(name, state, location)
}

// warnUnsupportedMapFields flags Map fields the engine silently ignores at
// runtime (see mapstate.go), so a definition that structurally validates
// does not still surprise the caller when it runs -- the class of gap
// reported in issue #861.
func (v *definitionValidator) warnUnsupportedMapFields(name string, state *stateDefinition, location string) {
	for _, field := range mapEmulationGapFields {
		if len(field.get(state)) == 0 {
			continue
		}

		v.addWarning(codeUnsupportedRuntimeConstruct, location+"/"+field.name, fmt.Sprintf(
			"Map state %q sets %q, which kumo's engine does not implement (only the inline ItemProcessor/Iterator mode is supported); this definition will pass validation but will not run as expected",
			name, field.name,
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
