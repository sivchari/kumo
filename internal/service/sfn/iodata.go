package sfn

import (
	"encoding/json"
	"fmt"
	"strings"
)

// This file implements the Amazon States Language's standard data-flow
// field pipeline (InputPath, Parameters, ResultSelector, ResultPath,
// OutputPath), shared by every state type: raw input -> effective input
// (via InputPath/Parameters) -> result (the state's own work) -> effective
// result (via ResultSelector) -> effective output (result merged into
// effective input per ResultPath, then narrowed by OutputPath).
//
// A state that fails to apply one of these fields reports a plain error,
// which executionErrorCode surfaces as States.Runtime: not retriable, and
// never matched by Retry/Catch (including the States.ALL wildcard).

// parsePathField interprets a JSONPath-valued field decoded as raw JSON so
// an explicit "null" (discards data) can be told apart from the field
// being absent (defaults to "$"); both unmarshal identically via *string.
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

// resolveEffectiveInput computes a state's effective input: InputPath then
// Parameters resolved against it. Only for Task/Parallel/Pass -- Map's
// "Parameters" is the deprecated ItemSelector alias, so Map must not use
// this helper.
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

// applyResultSelector transforms result per ResultSelector, resolved like
// Parameters but against the result itself, not the state's input. A nil
// ResultSelector leaves the result unchanged.
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

// applyResultPath merges result into effectiveInput per ResultPath:
// default "$" replaces input with result; null discards result; any other
// Reference Path injects result at that location, creating intermediate
// objects as needed.
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

// injectResultPath injects result into base at an arbitrary-depth
// "$.a.b.c" Reference Path, creating intermediate objects as needed and
// overwriting whatever was already there.
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
