package eventbridge

import (
	"bytes"
	"encoding/json"
	"strings"
)

// matchEventPattern checks if an event matches an EventPattern.
// EventPattern is a JSON object where each key maps to an array of acceptable values.
// Pattern array elements can be literals (string/number) or content filter objects:
//
//	{"prefix": "x"}, {"suffix": "y"}, {"exists": true},
//	{"anything-but": [...]}, {"numeric": [">", 0, "<", 10]},
//	{"equals-ignore-case": "abc"}.
//
// An event matches if ALL keys in the pattern match.
func matchEventPattern(patternJSON string, event *PutEventsRequestEntry) bool {
	if patternJSON == "" {
		return true
	}

	var pattern map[string]json.RawMessage
	if err := json.Unmarshal([]byte(patternJSON), &pattern); err != nil {
		return false
	}

	for key, rawValues := range pattern {
		switch key {
		case "source":
			if !matchScalarField(rawValues, event.Source) {
				return false
			}
		case "detail-type":
			if !matchScalarField(rawValues, event.DetailType) {
				return false
			}
		case "detail":
			if !matchDetailField(rawValues, event.Detail) {
				return false
			}
		}
	}

	return true
}

// matchScalarField checks if a top-level scalar string field matches the pattern array.
// Top-level fields source/detail-type are always considered "exists" when non-empty.
func matchScalarField(patternArr json.RawMessage, fieldValue string) bool {
	exists := fieldValue != ""

	value, err := json.Marshal(fieldValue)
	if err != nil {
		return false
	}

	if !exists {
		return matchValueAgainstPatternArray(patternArr, nil, false)
	}

	return matchValueAgainstPatternArray(patternArr, value, true)
}

// matchDetailField checks if an event detail matches the detail pattern.
// The detail pattern is a nested JSON object mapping field names to arrays of pattern elements.
func matchDetailField(rawPattern json.RawMessage, detailJSON string) bool {
	var pattern map[string]json.RawMessage
	if err := json.Unmarshal(rawPattern, &pattern); err != nil {
		return false
	}

	detail := map[string]json.RawMessage{}

	if detailJSON != "" {
		_ = json.Unmarshal([]byte(detailJSON), &detail)
	}

	for key, patternValue := range pattern {
		detailValue, exists := detail[key]
		if !matchDetailValue(patternValue, detailValue, exists) {
			return false
		}
	}

	return true
}

// matchDetailValue checks if a detail value (or its absence) matches a pattern value.
// Handles nested objects and pattern element arrays.
func matchDetailValue(patternValue, detailValue json.RawMessage, exists bool) bool {
	patternStr := strings.TrimSpace(string(patternValue))

	// Nested object pattern: requires the field to exist as an object.
	if strings.HasPrefix(patternStr, "{") {
		if !exists {
			return false
		}

		return matchDetailField(patternValue, string(detailValue))
	}

	return matchValueAgainstPatternArray(patternValue, detailValue, exists)
}

// matchValueAgainstPatternArray returns true if value matches at least one element in the pattern array.
// When exists is false, only an `{"exists": false}` element matches.
func matchValueAgainstPatternArray(patternArr, value json.RawMessage, exists bool) bool {
	var elements []json.RawMessage
	if err := json.Unmarshal(patternArr, &elements); err != nil {
		return false
	}

	for _, elem := range elements {
		if matchPatternElement(elem, value, exists) {
			return true
		}
	}

	return false
}

// matchPatternElement matches a single pattern element (literal value or content filter object).
func matchPatternElement(elem, value json.RawMessage, exists bool) bool {
	s := strings.TrimSpace(string(elem))
	if strings.HasPrefix(s, "{") {
		return matchContentFilter(elem, value, exists)
	}

	if !exists {
		return false
	}

	return literalEquals(elem, value)
}

// literalEquals compares two JSON values for equality after canonicalisation.
func literalEquals(a, b json.RawMessage) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}

	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}

	ac, err := json.Marshal(av)
	if err != nil {
		return false
	}

	bc, err := json.Marshal(bv)
	if err != nil {
		return false
	}

	return bytes.Equal(ac, bc)
}

// matchContentFilter dispatches to the appropriate content filter matcher.
// A filter object has exactly one key indicating the filter type.
func matchContentFilter(filter, value json.RawMessage, exists bool) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(filter, &m); err != nil {
		return false
	}

	for op, opArg := range m {
		switch op {
		case "exists":
			return matchExistsFilter(opArg, exists)
		case "prefix":
			return exists && matchPrefixFilter(opArg, value)
		case "suffix":
			return exists && matchSuffixFilter(opArg, value)
		case "anything-but":
			return exists && matchAnythingButFilter(opArg, value)
		case "numeric":
			return exists && matchNumericFilter(opArg, value)
		case "equals-ignore-case":
			return exists && matchEqualsIgnoreCaseFilter(opArg, value)
		}
	}

	return false
}

// matchExistsFilter handles {"exists": true|false}.
func matchExistsFilter(arg json.RawMessage, exists bool) bool {
	var want bool
	if err := json.Unmarshal(arg, &want); err != nil {
		return false
	}

	return want == exists
}

// matchPrefixFilter handles {"prefix": "..."}.
func matchPrefixFilter(arg, value json.RawMessage) bool {
	var prefix, actual string
	if err := json.Unmarshal(arg, &prefix); err != nil {
		return false
	}

	if err := json.Unmarshal(value, &actual); err != nil {
		return false
	}

	return strings.HasPrefix(actual, prefix)
}

// matchSuffixFilter handles {"suffix": "..."}.
func matchSuffixFilter(arg, value json.RawMessage) bool {
	var suffix, actual string
	if err := json.Unmarshal(arg, &suffix); err != nil {
		return false
	}

	if err := json.Unmarshal(value, &actual); err != nil {
		return false
	}

	return strings.HasSuffix(actual, suffix)
}

// matchAnythingButFilter handles {"anything-but": "x"} or {"anything-but": ["x", "y"]}.
func matchAnythingButFilter(arg, value json.RawMessage) bool {
	// Single value form.
	var single string
	if err := json.Unmarshal(arg, &single); err == nil {
		var actual string
		if err := json.Unmarshal(value, &actual); err != nil {
			return false
		}

		return actual != single
	}

	// Array of strings form.
	var listStr []string
	if err := json.Unmarshal(arg, &listStr); err == nil {
		var actual string
		if err := json.Unmarshal(value, &actual); err != nil {
			return false
		}

		for _, v := range listStr {
			if actual == v {
				return false
			}
		}

		return true
	}

	// Array of numbers form.
	var listNum []float64
	if err := json.Unmarshal(arg, &listNum); err == nil {
		var actual float64
		if err := json.Unmarshal(value, &actual); err != nil {
			return false
		}

		for _, v := range listNum {
			if actual == v {
				return false
			}
		}

		return true
	}

	return false
}

// matchNumericFilter handles {"numeric": [op, num, op, num, ...]}.
// Supported ops: =, !=, >, >=, <, <=.
func matchNumericFilter(arg, value json.RawMessage) bool {
	var actual float64
	if err := json.Unmarshal(value, &actual); err != nil {
		return false
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(arg, &raw); err != nil {
		return false
	}

	if len(raw)%2 != 0 {
		return false
	}

	for i := 0; i < len(raw); i += 2 {
		var op string
		if err := json.Unmarshal(raw[i], &op); err != nil {
			return false
		}

		var operand float64
		if err := json.Unmarshal(raw[i+1], &operand); err != nil {
			return false
		}

		if !numericCompare(actual, op, operand) {
			return false
		}
	}

	return true
}

func numericCompare(actual float64, op string, operand float64) bool {
	switch op {
	case "=":
		return actual == operand
	case "!=":
		return actual != operand
	case ">":
		return actual > operand
	case ">=":
		return actual >= operand
	case "<":
		return actual < operand
	case "<=":
		return actual <= operand
	}

	return false
}

// matchEqualsIgnoreCaseFilter handles {"equals-ignore-case": "..."}.
func matchEqualsIgnoreCaseFilter(arg, value json.RawMessage) bool {
	var want, actual string
	if err := json.Unmarshal(arg, &want); err != nil {
		return false
	}

	if err := json.Unmarshal(value, &actual); err != nil {
		return false
	}

	return strings.EqualFold(want, actual)
}
