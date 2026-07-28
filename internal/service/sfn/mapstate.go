package sfn

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// executeMapStateWithPolicy executes a Map state's item processing,
// applying its Retry and Catch policy.
func (e *executionEngine) executeMapStateWithPolicy(ctx context.Context, name string, state *stateDefinition, input string) (string, string, error) {
	return e.runWithRetryCatch(ctx, name, state, input, func(ctx context.Context) (string, error) {
		return e.executeMapState(ctx, name, state, input)
	})
}

// executeMapState resolves ItemsPath into a JSON array and runs every item
// through the state's ItemProcessor (or its legacy alias, Iterator),
// bounding concurrency at MaxConcurrency when it is positive. Item outputs
// are collected, in item order, as a JSON array. The first item to fail
// cancels the remaining items and the Map state fails with that item's
// error.
//
// ItemSelector, ItemBatcher, ResultWriter, and distributed-mode Map are out
// of scope; only the inline processor mode is supported.
func (e *executionEngine) executeMapState(ctx context.Context, name string, state *stateDefinition, input string) (string, error) {
	processor, err := itemProcessorDefinition(state)
	if err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	items, err := resolveItemsPath(state.ItemsPath, input)
	if err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	itemCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	outputs, err := runMapItems(itemCtx, e, processor, items, state.MaxConcurrency, cancel)
	if err != nil {
		return "", fmt.Errorf("map state %q: %w", name, err)
	}

	result, err := json.Marshal(outputs)
	if err != nil {
		return "", fmt.Errorf("map state %q: marshal item outputs: %w", name, err)
	}

	return string(result), nil
}

// itemProcessorDefinition returns the Map state's sub-state-machine.
// ItemProcessor is the current field name; Iterator is the legacy alias. If
// both are present, ItemProcessor takes precedence.
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

// runMapItems executes the processor once per item, honoring MaxConcurrency
// (0 or negative means unlimited), and returns the item outputs in item
// order, or the error of the first item to fail.
func runMapItems(ctx context.Context, e *executionEngine, processor *stateMachineDefinition, items []json.RawMessage, maxConcurrency int, cancel context.CancelFunc) ([]json.RawMessage, error) {
	outputs := make([]json.RawMessage, len(items))

	var sem chan struct{}
	if maxConcurrency > 0 {
		sem = make(chan struct{}, maxConcurrency)
	}

	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)

	for i := range items {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					// Another item already failed and canceled ctx before
					// this queued item got a slot; skip it rather than
					// starting work whose result will be discarded.
					return
				}
			}

			out, err := e.execute(ctx, processor, string(items[i]))
			if err != nil {
				once.Do(func() {
					firstErr = fmt.Errorf("item %d: %w", i, err)

					cancel()
				})

				return
			}

			outputs[i] = json.RawMessage(out)
		}(i)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return outputs, nil
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
