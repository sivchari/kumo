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

// executeMapStateWithPolicy executes a Map state's item processing,
// applying its Retry and Catch policy.
func (e *executionEngine) executeMapStateWithPolicy(ctx context.Context, name string, state *stateDefinition, input string) (string, string, error) {
	return e.runWithRetryCatch(ctx, name, state, input, func(ctx context.Context) (string, error) {
		return e.executeMapState(ctx, name, state, input)
	})
}

// executeMapState resolves a Map state's item list (from ItemsPath or, in
// Distributed mode, ItemReader), applies ItemSelector, groups items into
// processor units (individually, or via ItemBatcher), and runs every unit
// through the state's ItemProcessor (or its legacy alias, Iterator),
// bounding concurrency at MaxConcurrency when it is positive.
//
// Both Inline and Distributed mode run in-process in kumo: kumo has no
// child-execution or Map Run resource, so DescribeMapRun/ListMapRuns and
// child-execution tracking are out of scope (see the ResultWriter
// simplification documented in resultwriter.go for the same reason).
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
// ResultWriterDetails{Bucket, Key} shape (see resultwriter.go).
func (e *executionEngine) runMapProcessorUnits(ctx context.Context, name string, state *stateDefinition, processor *stateMachineDefinition, units []mapUnit, totalItems int, input string) (string, error) {
	tolerance := resolveMapTolerance(state)

	itemCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results, err := runMapUnits(itemCtx, e, processor, units, state.MaxConcurrency, tolerance.enabled(), cancel)
	if err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	outputs, failedItems := collectMapResults(units, results)

	if tolerance.enabled() && tolerance.exceeds(failedItems, totalItems) {
		return "", &failStateError{
			errorName: errorStatesExceedToleratedFailureThreshold,
			cause: fmt.Sprintf(
				"map state %q: %d of %d items failed, exceeding the configured tolerated failure threshold",
				name, failedItems, totalItems,
			),
		}
	}

	if len(state.ResultWriter) > 0 {
		written, err := writeMapResultToS3(ctx, e, state.ResultWriter, outputs, input)
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
