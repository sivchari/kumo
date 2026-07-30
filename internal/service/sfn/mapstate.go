package sfn

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// processorConfig is a Map state's ItemProcessor.ProcessorConfig, which
// selects Inline vs Distributed processing. JSON tags are lowerCamelCase;
// see itemBatcherDef in itembatcher.go for why.
type processorConfig struct {
	Mode          string `json:"mode"`
	ExecutionType string `json:"executionType"`
}

// ProcessorConfig.Mode values.
const (
	mapProcessorModeInline      = "INLINE"
	mapProcessorModeDistributed = "DISTRIBUTED"
)

// executeMapStateWithPolicy executes a Map state's full standard field
// pipeline: InputPath builds the effective input that ItemsPath resolves
// the Items Array against (Map's own "Parameters" field, when present, is
// the deprecated alias for ItemSelector rather than the standard
// effective-input Payload Template, so it is deliberately not folded in
// here -- see resolveEffectiveInput), Retry and Catch govern item
// processing as a whole, and on success ResultSelector/ResultPath/
// OutputPath shape the effective output.
func (e *executionEngine) executeMapStateWithPolicy(ctx context.Context, name string, state *stateDefinition, input string) (string, string, error) {
	effectiveInput, err := applyInputPath(state.InputPath, input)
	if err != nil {
		return "", "", fmt.Errorf("map state %q: %w", name, err)
	}

	return e.runRetryCatchResultPipeline(ctx, name, state, input, effectiveInput, func(ctx context.Context) (string, error) {
		return e.executeMapState(ctx, name, state, effectiveInput)
	})
}

// executeMapState resolves a Map state's item list (from ItemsPath or, in
// Distributed mode, ItemReader), applies ItemSelector, groups items into
// processor units (individually, or via ItemBatcher), and runs every unit
// through the state's ItemProcessor (or its legacy alias, Iterator),
// bounding concurrency at MaxConcurrency when it is positive.
//
// Both Inline and Distributed mode run each unit in-process in kumo, rather
// than as a real child *execution* (see maprun.go's mapRunCountsFromResults
// doc for the consequences of that simplification); a Distributed-mode run
// is still recorded as a queryable Map Run (see runMapProcessorUnits).
func (e *executionEngine) executeMapState(ctx context.Context, name string, state *stateDefinition, input string) (string, error) {
	processor, err := itemProcessorDefinition(state)
	if err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	if err := requireDistributedModeForFields(state, processor); err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	items, err := resolveMapItems(ctx, e, state, input)
	if err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	items, err = applyItemSelector(state.ItemSelector, items, input)
	if err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	units, err := buildProcessorUnits(state.ItemBatcher, items, input)
	if err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	return e.runMapProcessorUnits(ctx, name, state, processor, units, len(items), input)
}

// runMapProcessorUnits runs every processor unit and shapes the Map
// state's output: the ExceedToleratedFailureThreshold check, the plain
// item-output array, or -- when ResultWriter is set -- the
// ResultWriterDetails{Bucket, Key} shape (see resultwriter.go). For a
// Distributed-mode processor it also records the run as a Map Run (see
// maprun.go), whose ARN (when recorded) is included in the ResultWriter
// output.
func (e *executionEngine) runMapProcessorUnits(ctx context.Context, name string, state *stateDefinition, processor *stateMachineDefinition, units []mapUnit, totalItems int, input string) (string, error) {
	tolerance, err := resolveMapTolerance(state, input)
	if err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	maxConcurrency, err := resolveMaxConcurrency(state, input)
	if err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	itemCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	mapRunArn := e.startMapRunIfDistributed(ctx, name, processor, maxConcurrency, tolerance)

	results, err := runMapUnits(itemCtx, e, processor, units, maxConcurrency, tolerance.enabled(), cancel)
	if err != nil {
		e.finishMapRunIfDistributed(mapRunArn, mapRunStatusFailed, units, nil, totalItems)

		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	outputs, failedItems := collectMapResults(units, results)

	if tolerance.enabled() && tolerance.exceeds(failedItems, totalItems) {
		e.finishMapRunIfDistributed(mapRunArn, mapRunStatusFailed, units, results, totalItems)

		return "", &failStateError{
			errorName: errorStatesExceedToleratedFailureThreshold,
			cause: fmt.Sprintf(
				"map state %q: %d of %d items failed, exceeding the configured tolerated failure threshold",
				name, failedItems, totalItems,
			),
		}
	}

	e.finishMapRunIfDistributed(mapRunArn, mapRunStatusSucceeded, units, results, totalItems)

	if len(state.ResultWriter) > 0 {
		written, err := writeMapResultToS3(ctx, e, state.ResultWriter, outputs, input, mapRunArn)
		if err != nil {
			return "", fmt.Errorf("map state %q: %w", name, err)
		}

		return written, nil
	}

	result, err := json.Marshal(outputs)
	if err != nil {
		return "", fmt.Errorf("map state %q: marshal item outputs: %w", name, err)
	}

	return string(result), nil
}

// resolveMaxConcurrency resolves a Map state's MaxConcurrency, preferring
// MaxConcurrencyPath when set (the same precedence every other *Path field
// in this package gives its static counterpart -- see taskTimeout in
// timeout.go).
func resolveMaxConcurrency(state *stateDefinition, input string) (int, error) {
	if state.MaxConcurrencyPath == "" {
		return state.MaxConcurrency, nil
	}

	n, err := resolveNumberPath(state.MaxConcurrencyPath, input)
	if err != nil {
		return 0, fmt.Errorf("resolve MaxConcurrencyPath: %w", err)
	}

	return int(n), nil
}

// collectMapResults turns per-unit results into the Map state's output
// array and the total count of dataset items that failed. A failed unit
// contributes a JSON null for every item it represented, rather than being
// omitted, so the output array's length always matches the unit count --
// an emulator simplification, since AWS does not document the exact
// per-item output shape for a partially-failed Map Run that isn't exported
// via ResultWriter.
func collectMapResults(units []mapUnit, results []mapUnitResult) (outputs []json.RawMessage, failedItems int) {
	outputs = make([]json.RawMessage, len(results))

	for i, r := range results {
		if r.err != nil {
			failedItems += units[i].itemCount
			outputs[i] = json.RawMessage("null")

			continue
		}

		outputs[i] = r.output
	}

	return outputs, failedItems
}

// itemProcessorDefinition returns the Map state's sub-state-machine.
// ItemProcessor is the current field name; Iterator is the legacy alias for
// the same sub-state-machine. If both are present, ItemProcessor takes
// precedence.
func itemProcessorDefinition(state *stateDefinition) (*stateMachineDefinition, error) {
	switch {
	case state.ItemProcessor != nil:
		return state.ItemProcessor, nil
	case state.Iterator != nil:
		return state.Iterator, nil
	default:
		return nil, fmt.Errorf("map state requires ItemProcessor or Iterator")
	}
}

// isDistributedMode reports whether a Map state's ItemProcessor runs in
// Distributed mode. An absent ProcessorConfig, or a Mode other than
// "DISTRIBUTED", is Inline mode -- ASL's own documented default.
func isDistributedMode(processor *stateMachineDefinition) bool {
	return processor.ProcessorConfig != nil && strings.EqualFold(processor.ProcessorConfig.Mode, mapProcessorModeDistributed)
}

// requireDistributedModeForFields enforces AWS's rule that ItemReader,
// ItemBatcher, ResultWriter, ToleratedFailurePercentage, and
// ToleratedFailureCount are Distributed-mode-only Map state fields (see
// https://docs.aws.amazon.com/step-functions/latest/dg/state-map-inline.html,
// whose field list omits all five, versus
// https://docs.aws.amazon.com/step-functions/latest/dg/state-map-distributed.html,
// which documents them). validateMapDistributedFields in validate.go
// reports the same rule as a definition-time diagnostic.
func requireDistributedModeForFields(state *stateDefinition, processor *stateMachineDefinition) error {
	if isDistributedMode(processor) {
		return nil
	}

	fields := distributedOnlyFieldsSet(state)
	if len(fields) == 0 {
		return nil
	}

	return fmt.Errorf(
		"%s require ItemProcessor.ProcessorConfig.Mode %q",
		strings.Join(fields, ", "), mapProcessorModeDistributed,
	)
}

// distributedOnlyFieldsSet lists which of the Distributed-mode-only Map
// fields are present on state.
func distributedOnlyFieldsSet(state *stateDefinition) []string {
	var fields []string

	if len(state.ItemReader) > 0 {
		fields = append(fields, "ItemReader")
	}

	if len(state.ItemBatcher) > 0 {
		fields = append(fields, "ItemBatcher")
	}

	if len(state.ResultWriter) > 0 {
		fields = append(fields, "ResultWriter")
	}

	if state.ToleratedFailurePercentage != nil {
		fields = append(fields, "ToleratedFailurePercentage")
	}

	if state.ToleratedFailureCount != nil {
		fields = append(fields, "ToleratedFailureCount")
	}

	return fields
}

// resolveMapItems resolves a Map state's item list: from ItemReader (an S3
// dataset, Distributed mode only) if set, otherwise from ItemsPath against
// the state's own input.
func resolveMapItems(ctx context.Context, e *executionEngine, state *stateDefinition, input string) ([]json.RawMessage, error) {
	if len(state.ItemReader) > 0 {
		return readItemsFromS3(ctx, e, state.ItemReader, input)
	}

	return resolveItemsPath(state.ItemsPath, input)
}

// resolveItemsPath resolves a Map state's ItemsPath (default "$") against
// its raw JSON input and requires the result to be a JSON array.
func resolveItemsPath(path, input string) ([]json.RawMessage, error) {
	if path == "" {
		path = "$"
	}

	var data any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	value, err := resolveAnyJSONPath(path, data)
	if err != nil {
		return nil, fmt.Errorf("resolve ItemsPath %q: %w", path, err)
	}

	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("itemsPath %q did not resolve to an array", path)
	}

	return marshalItems(items)
}

// marshalItems re-encodes each already-decoded item as its own JSON text.
func marshalItems(items []any) ([]json.RawMessage, error) {
	raw := make([]json.RawMessage, len(items))

	for i, item := range items {
		encoded, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("marshal item %d: %w", i, err)
		}

		raw[i] = encoded
	}

	return raw, nil
}

// resolveAnyJSONPath resolves "$" or "$.field[.field...]" against an
// already-decoded JSON value of any shape. It mirrors resolveJSONPath's
// supported subset but, unlike resolveJSONPath, does not require the root
// value to be a JSON object, so ItemsPath can select the entire input when
// the input itself is a JSON array.
func resolveAnyJSONPath(path string, data any) (any, error) {
	if path == "$" {
		return data, nil
	}

	if !strings.HasPrefix(path, "$.") {
		return nil, fmt.Errorf("unsupported JSONPath %q: must start with $", path)
	}

	current := data

	for _, part := range strings.Split(strings.TrimPrefix(path, "$."), ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("jsonPath %q: cannot access field %q on non-object", path, part)
		}

		val, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("jsonPath %q: field %q not found", path, part)
		}

		current = val
	}

	return current, nil
}
