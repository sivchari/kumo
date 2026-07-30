package sfn

import (
	"fmt"
	"strings"
	"time"
)

// choiceRule is a single rule within a Choice state's Choices array. Per the
// Amazon States Language spec it is either a Data-test Expression (a
// "Variable" JSONPath plus exactly one comparison operator) or a Boolean
// Expression (And/Or/Not combining nested rules).
//
// Every comparison operator also has a "*Path" variant (e.g.
// "StringEqualsPath"), whose value is a Path resolved against the state's
// effective input to yield the comparison operand, instead of a static
// literal -- see resolveStringOperand/resolveNumericOperand/
// resolveBooleanOperand.
//
// JSON tags are lowerCamelCase rather than the Amazon States Language's own
// PascalCase (e.g. "Variable", "StringEquals"); encoding/json matches object
// keys to struct fields case-insensitively when there is no exact match, so
// definitions using the spec's PascalCase field names still decode correctly.
type choiceRule struct {
	Variable string `json:"variable"`
	Next     string `json:"next"`

	StringEquals            *string `json:"stringEquals"`
	StringLessThan          *string `json:"stringLessThan"`
	StringGreaterThan       *string `json:"stringGreaterThan"`
	StringLessThanEquals    *string `json:"stringLessThanEquals"`
	StringGreaterThanEquals *string `json:"stringGreaterThanEquals"`

	StringEqualsPath            *string `json:"stringEqualsPath"`
	StringLessThanPath          *string `json:"stringLessThanPath"`
	StringGreaterThanPath       *string `json:"stringGreaterThanPath"`
	StringLessThanEqualsPath    *string `json:"stringLessThanEqualsPath"`
	StringGreaterThanEqualsPath *string `json:"stringGreaterThanEqualsPath"`

	NumericEquals            *float64 `json:"numericEquals"`
	NumericLessThan          *float64 `json:"numericLessThan"`
	NumericGreaterThan       *float64 `json:"numericGreaterThan"`
	NumericLessThanEquals    *float64 `json:"numericLessThanEquals"`
	NumericGreaterThanEquals *float64 `json:"numericGreaterThanEquals"`

	NumericEqualsPath            *string `json:"numericEqualsPath"`
	NumericLessThanPath          *string `json:"numericLessThanPath"`
	NumericGreaterThanPath       *string `json:"numericGreaterThanPath"`
	NumericLessThanEqualsPath    *string `json:"numericLessThanEqualsPath"`
	NumericGreaterThanEqualsPath *string `json:"numericGreaterThanEqualsPath"`

	BooleanEquals     *bool   `json:"booleanEquals"`
	BooleanEqualsPath *string `json:"booleanEqualsPath"`

	TimestampEquals            *string `json:"timestampEquals"`
	TimestampLessThan          *string `json:"timestampLessThan"`
	TimestampGreaterThan       *string `json:"timestampGreaterThan"`
	TimestampLessThanEquals    *string `json:"timestampLessThanEquals"`
	TimestampGreaterThanEquals *string `json:"timestampGreaterThanEquals"`

	TimestampEqualsPath            *string `json:"timestampEqualsPath"`
	TimestampLessThanPath          *string `json:"timestampLessThanPath"`
	TimestampGreaterThanPath       *string `json:"timestampGreaterThanPath"`
	TimestampLessThanEqualsPath    *string `json:"timestampLessThanEqualsPath"`
	TimestampGreaterThanEqualsPath *string `json:"timestampGreaterThanEqualsPath"`

	IsPresent *bool `json:"isPresent"`
	IsNull    *bool `json:"isNull"`

	And []choiceRule `json:"and"`
	Or  []choiceRule `json:"or"`
	Not *choiceRule  `json:"not"`
}

// evaluateChoiceRule evaluates a single Choice Rule (Data-test Expression or
// Boolean Expression) against the parsed effective input.
func evaluateChoiceRule(rule *choiceRule, input map[string]any) (bool, error) {
	switch {
	case len(rule.And) > 0:
		return evaluateAnd(rule.And, input)
	case len(rule.Or) > 0:
		return evaluateOr(rule.Or, input)
	case rule.Not != nil:
		matched, err := evaluateChoiceRule(rule.Not, input)
		if err != nil {
			return false, err
		}

		return !matched, nil
	default:
		return evaluateDataTest(rule, input)
	}
}

// evaluateAnd evaluates an "And" Boolean Expression, short-circuiting on the
// first rule that does not match.
func evaluateAnd(rules []choiceRule, input map[string]any) (bool, error) {
	for i := range rules {
		matched, err := evaluateChoiceRule(&rules[i], input)
		if err != nil {
			return false, err
		}

		if !matched {
			return false, nil
		}
	}

	return true, nil
}

// evaluateOr evaluates an "Or" Boolean Expression, short-circuiting on the
// first rule that matches.
func evaluateOr(rules []choiceRule, input map[string]any) (bool, error) {
	for i := range rules {
		matched, err := evaluateChoiceRule(&rules[i], input)
		if err != nil {
			return false, err
		}

		if matched {
			return true, nil
		}
	}

	return false, nil
}

// evaluateDataTest evaluates a Data-test Expression: a Variable path plus
// exactly one comparison operator.
func evaluateDataTest(rule *choiceRule, input map[string]any) (bool, error) {
	if rule.Variable == "" {
		return false, fmt.Errorf("choice rule: Variable is required")
	}

	if rule.IsPresent != nil {
		_, err := resolveJSONPath(input, rule.Variable)

		return (err == nil) == *rule.IsPresent, nil
	}

	value, err := resolveJSONPath(input, rule.Variable)
	if err != nil {
		return false, fmt.Errorf("choice rule: resolve Variable %q: %w", rule.Variable, err)
	}

	if rule.IsNull != nil {
		return (value == nil) == *rule.IsNull, nil
	}

	if matched, handled, err := evaluateStringComparison(rule, value, input); handled {
		return matched, err
	}

	if matched, handled, err := evaluateNumericComparison(rule, value, input); handled {
		return matched, err
	}

	if matched, handled, err := evaluateBooleanComparison(rule, value, input); handled {
		return matched, err
	}

	if matched, handled, err := evaluateTimestampComparison(rule, value, input); handled {
		return matched, err
	}

	return false, fmt.Errorf("choice rule: no comparison operator set for Variable %q", rule.Variable)
}

// evaluateStringComparison evaluates the String* comparators, including
// their "*Path" variants. handled is false if rule sets none of them.
func evaluateStringComparison(rule *choiceRule, value any, input map[string]any) (matched, handled bool, err error) {
	switch {
	case rule.StringEquals != nil || rule.StringEqualsPath != nil:
		return compareStringWith(value, rule.StringEquals, rule.StringEqualsPath, input, func(cmp int) bool { return cmp == 0 })
	case rule.StringLessThan != nil || rule.StringLessThanPath != nil:
		return compareStringWith(value, rule.StringLessThan, rule.StringLessThanPath, input, func(cmp int) bool { return cmp < 0 })
	case rule.StringGreaterThan != nil || rule.StringGreaterThanPath != nil:
		return compareStringWith(value, rule.StringGreaterThan, rule.StringGreaterThanPath, input, func(cmp int) bool { return cmp > 0 })
	case rule.StringLessThanEquals != nil || rule.StringLessThanEqualsPath != nil:
		return compareStringWith(value, rule.StringLessThanEquals, rule.StringLessThanEqualsPath, input, func(cmp int) bool { return cmp <= 0 })
	case rule.StringGreaterThanEquals != nil || rule.StringGreaterThanEqualsPath != nil:
		return compareStringWith(value, rule.StringGreaterThanEquals, rule.StringGreaterThanEqualsPath, input, func(cmp int) bool { return cmp >= 0 })
	default:
		return false, false, nil
	}
}

// compareStringWith resolves a String* comparator's operand -- from a
// static value or, for the "*Path" variant, by resolving path against
// input -- and reports whether value satisfies it per matches.
func compareStringWith(value any, static, path *string, input map[string]any, matches func(cmp int) bool) (matched, handled bool, err error) {
	want, ok, err := resolveStringOperand(static, path, input)
	if err != nil {
		return false, true, err
	}

	if !ok {
		return false, true, nil
	}

	cmp, ok := stringCompare(value, want)

	return ok && matches(cmp), true, nil
}

// resolveStringOperand resolves a String*/Timestamp* comparator's operand:
// the static value if set, else the "*Path" variant's Path resolved
// against input. ok is false if path resolves to a non-string value, per
// the spec's "if the values are not both of the appropriate type... the
// comparison will return false".
func resolveStringOperand(static, path *string, input map[string]any) (want string, ok bool, err error) {
	if static != nil {
		return *static, true, nil
	}

	resolved, err := resolveJSONPath(input, *path)
	if err != nil {
		return "", false, fmt.Errorf("resolve comparator path %q: %w", *path, err)
	}

	s, isString := resolved.(string)

	return s, isString, nil
}

// evaluateNumericComparison evaluates the Numeric* comparators, including
// their "*Path" variants. handled is false if rule sets none of them.
func evaluateNumericComparison(rule *choiceRule, value any, input map[string]any) (matched, handled bool, err error) {
	switch {
	case rule.NumericEquals != nil || rule.NumericEqualsPath != nil:
		return compareNumericWith(value, rule.NumericEquals, rule.NumericEqualsPath, input, func(cmp int) bool { return cmp == 0 })
	case rule.NumericLessThan != nil || rule.NumericLessThanPath != nil:
		return compareNumericWith(value, rule.NumericLessThan, rule.NumericLessThanPath, input, func(cmp int) bool { return cmp < 0 })
	case rule.NumericGreaterThan != nil || rule.NumericGreaterThanPath != nil:
		return compareNumericWith(value, rule.NumericGreaterThan, rule.NumericGreaterThanPath, input, func(cmp int) bool { return cmp > 0 })
	case rule.NumericLessThanEquals != nil || rule.NumericLessThanEqualsPath != nil:
		return compareNumericWith(value, rule.NumericLessThanEquals, rule.NumericLessThanEqualsPath, input, func(cmp int) bool { return cmp <= 0 })
	case rule.NumericGreaterThanEquals != nil || rule.NumericGreaterThanEqualsPath != nil:
		return compareNumericWith(value, rule.NumericGreaterThanEquals, rule.NumericGreaterThanEqualsPath, input, func(cmp int) bool { return cmp >= 0 })
	default:
		return false, false, nil
	}
}

// compareNumericWith resolves a Numeric* comparator's operand -- from a
// static value or, for the "*Path" variant, by resolving path against
// input -- and reports whether value satisfies it per matches.
func compareNumericWith(value any, static *float64, path *string, input map[string]any, matches func(cmp int) bool) (matched, handled bool, err error) {
	want, ok, err := resolveNumericOperand(static, path, input)
	if err != nil {
		return false, true, err
	}

	if !ok {
		return false, true, nil
	}

	cmp, ok := numericCompare(value, want)

	return ok && matches(cmp), true, nil
}

// resolveNumericOperand resolves a Numeric* comparator's operand: the
// static value if set, else the "*Path" variant's Path resolved against
// input. ok is false if path resolves to a non-number value.
func resolveNumericOperand(static *float64, path *string, input map[string]any) (want float64, ok bool, err error) {
	if static != nil {
		return *static, true, nil
	}

	resolved, err := resolveJSONPath(input, *path)
	if err != nil {
		return 0, false, fmt.Errorf("resolve comparator path %q: %w", *path, err)
	}

	n, isNumber := resolved.(float64)

	return n, isNumber, nil
}

// evaluateBooleanComparison evaluates BooleanEquals/BooleanEqualsPath.
// handled is false if rule sets neither.
func evaluateBooleanComparison(rule *choiceRule, value any, input map[string]any) (matched, handled bool, err error) {
	if rule.BooleanEquals == nil && rule.BooleanEqualsPath == nil {
		return false, false, nil
	}

	want, ok, err := resolveBooleanOperand(rule.BooleanEquals, rule.BooleanEqualsPath, input)
	if err != nil {
		return false, true, err
	}

	if !ok {
		return false, true, nil
	}

	b, isBool := value.(bool)

	return isBool && b == want, true, nil
}

// resolveBooleanOperand resolves BooleanEquals/BooleanEqualsPath's operand:
// the static value if set, else BooleanEqualsPath's Path resolved against
// input. ok is false if path resolves to a non-boolean value.
func resolveBooleanOperand(static *bool, path *string, input map[string]any) (want, ok bool, err error) {
	if static != nil {
		return *static, true, nil
	}

	resolved, err := resolveJSONPath(input, *path)
	if err != nil {
		return false, false, fmt.Errorf("resolve comparator path %q: %w", *path, err)
	}

	b, isBool := resolved.(bool)

	return b, isBool, nil
}

// evaluateTimestampComparison evaluates the Timestamp* comparators,
// including their "*Path" variants. handled is false if rule sets none of
// them.
func evaluateTimestampComparison(rule *choiceRule, value any, input map[string]any) (matched, handled bool, err error) {
	switch {
	case rule.TimestampEquals != nil || rule.TimestampEqualsPath != nil:
		return compareTimestampWith(value, rule.TimestampEquals, rule.TimestampEqualsPath, input, func(cmp int) bool { return cmp == 0 })
	case rule.TimestampLessThan != nil || rule.TimestampLessThanPath != nil:
		return compareTimestampWith(value, rule.TimestampLessThan, rule.TimestampLessThanPath, input, func(cmp int) bool { return cmp < 0 })
	case rule.TimestampGreaterThan != nil || rule.TimestampGreaterThanPath != nil:
		return compareTimestampWith(value, rule.TimestampGreaterThan, rule.TimestampGreaterThanPath, input, func(cmp int) bool { return cmp > 0 })
	case rule.TimestampLessThanEquals != nil || rule.TimestampLessThanEqualsPath != nil:
		return compareTimestampWith(value, rule.TimestampLessThanEquals, rule.TimestampLessThanEqualsPath, input, func(cmp int) bool { return cmp <= 0 })
	case rule.TimestampGreaterThanEquals != nil || rule.TimestampGreaterThanEqualsPath != nil:
		return compareTimestampWith(value, rule.TimestampGreaterThanEquals, rule.TimestampGreaterThanEqualsPath, input, func(cmp int) bool { return cmp >= 0 })
	default:
		return false, false, nil
	}
}

// compareTimestampWith resolves a Timestamp* comparator's operand -- from a
// static value or, for the "*Path" variant, by resolving path against
// input -- and reports whether value satisfies it per matches. The operand
// is a string in both cases, so this reuses resolveStringOperand.
func compareTimestampWith(value any, static, path *string, input map[string]any, matches func(cmp int) bool) (matched, handled bool, err error) {
	want, ok, err := resolveStringOperand(static, path, input)
	if err != nil {
		return false, true, err
	}

	if !ok {
		return false, true, nil
	}

	cmp, ok := timestampCompare(value, want)

	return ok && matches(cmp), true, nil
}

// stringCompare compares value (expected to be a string) against want,
// returning ok=false if value is not a string.
func stringCompare(value any, want string) (cmp int, ok bool) {
	s, ok := value.(string)
	if !ok {
		return 0, false
	}

	return strings.Compare(s, want), true
}

// numericCompare compares value (expected to be a float64, as produced by
// encoding/json for JSON numbers) against want, returning ok=false if value
// is not a number.
func numericCompare(value any, want float64) (cmp int, ok bool) {
	n, ok := value.(float64)
	if !ok {
		return 0, false
	}

	switch {
	case n < want:
		return -1, true
	case n > want:
		return 1, true
	default:
		return 0, true
	}
}

// timestampCompare compares value (expected to be an RFC3339 string) against
// want, returning ok=false if either side is not a parseable RFC3339
// timestamp.
func timestampCompare(value any, want string) (cmp int, ok bool) {
	s, ok := value.(string)
	if !ok {
		return 0, false
	}

	valueTime, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0, false
	}

	wantTime, err := time.Parse(time.RFC3339, want)
	if err != nil {
		return 0, false
	}

	switch {
	case valueTime.Before(wantTime):
		return -1, true
	case valueTime.After(wantTime):
		return 1, true
	default:
		return 0, true
	}
}
