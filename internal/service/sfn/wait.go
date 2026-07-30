package sfn

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// executeWaitState executes a Wait state by sleeping for the duration
// determined by Seconds, SecondsPath, Timestamp, or TimestampPath, resolved
// against the effective input (InputPath applied to the raw input). Wait
// has no Parameters/Result/ResultPath: its output is always its (possibly
// filtered) input, narrowed once more by OutputPath.
//
// The wait is not artificially capped: kumo sleeps for the full requested
// duration but honors ctx cancellation, and every execution already runs
// under the timeout context applied in MemoryStorage.runExecution
// (min(definition TimeoutSeconds, executionTimeoutCap)), which bounds how
// long any single Wait can actually block.
func (e *executionEngine) executeWaitState(ctx context.Context, state *stateDefinition, input string) (string, error) {
	effectiveInput, err := applyInputPath(state.InputPath, input)
	if err != nil {
		return "", fmt.Errorf("wait state: %w", err)
	}

	d, err := waitDuration(state, effectiveInput)
	if err != nil {
		return "", fmt.Errorf("wait state: %w", err)
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("wait state: %w", ctx.Err())
	case <-timer.C:
	}

	output, err := applyOutputPath(state.OutputPath, effectiveInput)
	if err != nil {
		return "", fmt.Errorf("wait state: %w", err)
	}

	return output, nil
}

// waitDuration determines how long a Wait state should sleep from whichever
// of Seconds, SecondsPath, Timestamp, or TimestampPath is set.
func waitDuration(state *stateDefinition, input string) (time.Duration, error) {
	switch {
	case state.Seconds != nil:
		if *state.Seconds < 0 {
			return 0, fmt.Errorf("seconds must be non-negative")
		}

		return time.Duration(*state.Seconds) * time.Second, nil

	case state.SecondsPath != "":
		seconds, err := resolveNumberPath(state.SecondsPath, input)
		if err != nil {
			return 0, fmt.Errorf("resolve SecondsPath: %w", err)
		}

		return time.Duration(seconds * float64(time.Second)), nil

	case state.Timestamp != "":
		return durationUntilTimestamp(state.Timestamp)

	case state.TimestampPath != "":
		ts, err := resolveStringPath(state.TimestampPath, input)
		if err != nil {
			return 0, fmt.Errorf("resolve TimestampPath: %w", err)
		}

		return durationUntilTimestamp(ts)

	default:
		return 0, fmt.Errorf("wait state requires one of Seconds, SecondsPath, Timestamp, TimestampPath")
	}
}

// durationUntilTimestamp parses an RFC3339 timestamp and returns the
// duration from now until that time, floored at zero for timestamps already
// in the past.
func durationUntilTimestamp(ts string) (time.Duration, error) {
	target, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return 0, fmt.Errorf("parse timestamp %q: %w", ts, err)
	}

	d := time.Until(target)
	if d < 0 {
		d = 0
	}

	return d, nil
}

// resolveNumberPath resolves a JSONPath against input and requires the
// result to be a JSON number.
func resolveNumberPath(path, input string) (float64, error) {
	value, err := resolvePath(path, input)
	if err != nil {
		return 0, err
	}

	n, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("jsonPath %q did not resolve to a number", path)
	}

	return n, nil
}

// resolveStringPath resolves a JSONPath against input and requires the
// result to be a JSON string.
func resolveStringPath(path, input string) (string, error) {
	value, err := resolvePath(path, input)
	if err != nil {
		return "", err
	}

	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("jsonPath %q did not resolve to a string", path)
	}

	return s, nil
}

// resolvePath parses input as JSON and resolves a JSONPath against it.
func resolvePath(path, input string) (any, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	value, err := resolveJSONPath(data, path)
	if err != nil {
		return nil, fmt.Errorf("resolve JSONPath %q: %w", path, err)
	}

	return value, nil
}

// resolveOptionalIntPath resolves the dynamic "*Path" variant of an
// otherwise-static optional integer field, preferring path over static when
// both are set -- the same precedence taskTimeout gives
// TimeoutSecondsPath/TimeoutSeconds. It returns static unchanged (possibly
// nil) when path is empty. Shared by the several Map/ItemReader/ItemBatcher
// fields with this exact shape: ReaderConfig.MaxItems(Path),
// ItemBatcher.MaxItemsPerBatch(Path)/MaxInputBytesPerBatch(Path), and
// ToleratedFailureCount(Path).
func resolveOptionalIntPath(static *int, path, input string) (*int, error) {
	if path == "" {
		return static, nil
	}

	n, err := resolveNumberPath(path, input)
	if err != nil {
		return nil, err
	}

	resolved := int(n)

	return &resolved, nil
}

// resolveOptionalFloatPath is resolveOptionalIntPath's float64 counterpart,
// used by ToleratedFailurePercentage(Path).
func resolveOptionalFloatPath(static *float64, path, input string) (*float64, error) {
	if path == "" {
		return static, nil
	}

	n, err := resolveNumberPath(path, input)
	if err != nil {
		return nil, err
	}

	return &n, nil
}
