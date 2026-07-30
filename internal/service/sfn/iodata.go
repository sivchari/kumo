package sfn

import (
	"encoding/json"
	"fmt"
	"strings"
)

// This file implements the Amazon States Language's standard data-flow
// field pipeline (InputPath, Parameters, ResultSelector, ResultPath,
// OutputPath), shared by every state type per
// https://states-language.net/spec.html#filters:
//
//   - "raw input" is the JSON text a state receives.
//   - "effective input" is the raw input after InputPath (and, on the state
//     types that support it, Parameters).
//   - "result" is whatever the state's own work produces (a Task response,
//     a Parallel/Map branch-output array, a Pass state's Result-or-input).
//   - "effective result" is the result after ResultSelector.
//   - "effective output" is the effective result merged into the effective
//     input per ResultPath, then narrowed by OutputPath.
//
// A state that fails to apply one of these fields (a ResultPath that
// cannot address its base, an OutputPath that matches nothing, malformed
// JSON) reports a plain error, which executionErrorCode surfaces as
// States.Runtime: per AWS's own documentation, States.Runtime is "often
// caused by errors at runtime, such as attempting to apply InputPath or
// OutputPath on a null JSON payload," is not retriable, and is never
// matched by Retry/Catch (including the States.ALL wildcard) -- see
// errorNameMatches in retry.go.

// parsePathField interprets a JSONPath-valued field (InputPath, OutputPath,
// ResultPath, or a Catcher's ResultPath) decoded as raw JSON so that an
// explicit "null" -- which per the spec discards the raw input/result --
// can be told apart from the field being absent from the definition, which
// defaults to "$" (no filtering or replacement); both would otherwise
// unmarshal to the same value through a plain *string.
func parsePathField(raw json.RawMessage) (path string, isNull bool, err error) {
	if len(raw) == 0 {
		return "$", false, nil
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false, fmt.Errorf("parse path: %w", err)
	}

	if value == nil {
		return "", true, nil
	}

	path, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("path must be a string or null")
	}

	return path, false, nil
}

// applyInputPath narrows a state's raw input per InputPath (default "$",
// meaning unchanged). An explicit null InputPath discards the raw input
// entirely, yielding an empty JSON object as the effective input.
func applyInputPath(raw json.RawMessage, rawInput string) (string, error) {
	out, err := applyPathField(raw, rawInput)
	if err != nil {
		return "", fmt.Errorf("inputPath: %w", err)
	}

	return out, nil
}

// applyOutputPath narrows a state's output per OutputPath (default "$",
// meaning unchanged). An explicit null OutputPath discards both the input
// and result that produced it, yielding an empty JSON object.
func applyOutputPath(raw json.RawMessage, output string) (string, error) {
	out, err := applyPathField(raw, output)
	if err != nil {
		return "", fmt.Errorf("outputPath: %w", err)
	}

	return out, nil
}

// applyPathField implements the InputPath/OutputPath semantics shared by
// both fields: null discards the data (yielding "{}"), "$" (the default)
// leaves it unchanged, and any other Path narrows it via resolveAnyJSONPath.
func applyPathField(raw json.RawMessage, data string) (string, error) {
	path, isNull, err := parsePathField(raw)
	if err != nil {
		return "", err
	}

	if isNull {
		return "{}", nil
	}

	if path == "$" {
		return data, nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return "", fmt.Errorf("parse data for path %q: %w", path, err)
	}

	resolved, err := resolveAnyJSONPath(path, parsed)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}

	out, err := json.Marshal(resolved)
	if err != nil {
		return "", fmt.Errorf("marshal path %q result: %w", path, err)
	}

	return string(out), nil
}

// resolveEffectiveInput computes a state's effective input: InputPath
// applied to the raw input, then -- on the state types where Parameters
// means "the Payload Template that becomes the effective input" (Task,
// Parallel, Pass; Map's "Parameters" is instead the deprecated alias for
// ItemSelector, so callers for Map must not use this helper) -- Parameters
// resolved against that filtered input.
func resolveEffectiveInput(inputPath json.RawMessage, parameters map[string]any, rawInput string) (string, error) {
	filtered, err := applyInputPath(inputPath, rawInput)
	if err != nil {
		return "", err
	}

	if parameters == nil {
		return filtered, nil
	}

	resolved, err := resolveParameters(parameters, filtered)
	if err != nil {
		return "", fmt.Errorf("parameters: %w", err)
	}

	out, err := json.Marshal(resolved)
	if err != nil {
		return "", fmt.Errorf("parameters: marshal effective input: %w", err)
	}

	return string(out), nil
}

// applyResultSelector transforms a state's result per ResultSelector, a
// Payload Template resolved the same way as Parameters -- static values, or
// "$."-prefixed JSONPath references, except resolved against the result
// itself rather than the state's input. A nil ResultSelector leaves the
// result unchanged, per its spec default of "no effect".
func applyResultSelector(selector map[string]any, result string) (string, error) {
	if selector == nil {
		return result, nil
	}

	resolved, err := resolveParameters(selector, result)
	if err != nil {
		return "", fmt.Errorf("resultSelector: %w", err)
	}

	out, err := json.Marshal(resolved)
	if err != nil {
		return "", fmt.Errorf("resultSelector: marshal result: %w", err)
	}

	return string(out), nil
}

// applyResultPath merges a state's effective result into its effective
// input per ResultPath (default "$", meaning the result replaces the input
// entirely). An explicit null ResultPath discards the result, passing the
// effective input through unchanged. Any other Reference Path injects the
// result at that location, relative to the effective input, creating
// intermediate objects as needed.
func applyResultPath(raw json.RawMessage, effectiveInput, result string) (string, error) {
	path, isNull, err := parsePathField(raw)
	if err != nil {
		return "", fmt.Errorf("resultPath: %w", err)
	}

	if isNull {
		return effectiveInput, nil
	}

	if path == "$" {
		return result, nil
	}

	out, err := injectResultPath(path, effectiveInput, result)
	if err != nil {
		return "", fmt.Errorf("resultPath: %w", err)
	}

	return out, nil
}

// injectResultPath injects a JSON-encoded result into base at an
// arbitrary-depth "$.a.b.c" Reference Path, creating intermediate objects
// as needed and overwriting whatever was already at that location -- see
// the spec's "Input and Output Processing" section's worked example of
// "$.master.result.sum" building a chain of new fields.
func injectResultPath(path, base, result string) (string, error) {
	field, ok := strings.CutPrefix(path, "$.")
	if !ok || field == "" {
		return "", fmt.Errorf("unsupported path %q: must be \"$\" or \"$.field[.field...]\"", path)
	}

	var root map[string]any
	if err := json.Unmarshal([]byte(base), &root); err != nil {
		return "", fmt.Errorf("path %q requires an object base: %w", path, err)
	}

	if root == nil {
		root = map[string]any{}
	}

	var resultValue any
	if err := json.Unmarshal([]byte(result), &resultValue); err != nil {
		return "", fmt.Errorf("path %q: parse result: %w", path, err)
	}

	parts := strings.Split(field, ".")

	current := root
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}

		current = next
	}

	current[parts[len(parts)-1]] = resultValue

	out, err := json.Marshal(root)
	if err != nil {
		return "", fmt.Errorf("path %q: marshal result: %w", path, err)
	}

	return string(out), nil
}
