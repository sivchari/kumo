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

	NumericEquals            *float64 `json:"numericEquals"`
	NumericLessThan          *float64 `json:"numericLessThan"`
	NumericGreaterThan       *float64 `json:"numericGreaterThan"`
	NumericLessThanEquals    *float64 `json:"numericLessThanEquals"`
	NumericGreaterThanEquals *float64 `json:"numericGreaterThanEquals"`

	BooleanEquals *bool `json:"booleanEquals"`

	TimestampEquals      *string `json:"timestampEquals"`
	TimestampLessThan    *string `json:"timestampLessThan"`
	TimestampGreaterThan *string `json:"timestampGreaterThan"`

	IsPresent *bool `json:"isPresent"`
	IsNull    *bool `json:"isNull"`

	And []choiceRule `json:"and"`
	Or  []choiceRule `json:"or"`
	Not *choiceRule  `json:"not"`
}

// evaluateChoiceRule evaluates a single Choice Rule (Data-test Expression or
// Boolean Expression) against the parsed state input.
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

	if matched, handled := evaluateStringComparison(rule, value); handled {
		return matched, nil
	}

	if matched, handled := evaluateNumericComparison(rule, value); handled {
		return matched, nil
	}

	if rule.BooleanEquals != nil {
		b, ok := value.(bool)

		return ok && b == *rule.BooleanEquals, nil
	}

	if matched, handled := evaluateTimestampComparison(rule, value); handled {
		return matched, nil
	}

	return false, fmt.Errorf("choice rule: no comparison operator set for Variable %q", rule.Variable)
}

// evaluateStringComparison evaluates the String* comparators. handled is
// false if rule does not set any of them.
func evaluateStringComparison(rule *choiceRule, value any) (matched, handled bool) {
	switch {
	case rule.StringEquals != nil:
		cmp, ok := stringCompare(value, *rule.StringEquals)

		return ok && cmp == 0, true
	case rule.StringLessThan != nil:
		cmp, ok := stringCompare(value, *rule.StringLessThan)

		return ok && cmp < 0, true
	case rule.StringGreaterThan != nil:
		cmp, ok := stringCompare(value, *rule.StringGreaterThan)

		return ok && cmp > 0, true
	case rule.StringLessThanEquals != nil:
		cmp, ok := stringCompare(value, *rule.StringLessThanEquals)

		return ok && cmp <= 0, true
	case rule.StringGreaterThanEquals != nil:
		cmp, ok := stringCompare(value, *rule.StringGreaterThanEquals)

		return ok && cmp >= 0, true
	default:
		return false, false
	}
}

// evaluateNumericComparison evaluates the Numeric* comparators. handled is
// false if rule does not set any of them.
func evaluateNumericComparison(rule *choiceRule, value any) (matched, handled bool) {
	switch {
	case rule.NumericEquals != nil:
		cmp, ok := numericCompare(value, *rule.NumericEquals)

		return ok && cmp == 0, true
	case rule.NumericLessThan != nil:
		cmp, ok := numericCompare(value, *rule.NumericLessThan)

		return ok && cmp < 0, true
	case rule.NumericGreaterThan != nil:
		cmp, ok := numericCompare(value, *rule.NumericGreaterThan)

		return ok && cmp > 0, true
	case rule.NumericLessThanEquals != nil:
		cmp, ok := numericCompare(value, *rule.NumericLessThanEquals)

		return ok && cmp <= 0, true
	case rule.NumericGreaterThanEquals != nil:
		cmp, ok := numericCompare(value, *rule.NumericGreaterThanEquals)

		return ok && cmp >= 0, true
	default:
		return false, false
	}
}

// evaluateTimestampComparison evaluates the Timestamp* comparators. handled
// is false if rule does not set any of them.
func evaluateTimestampComparison(rule *choiceRule, value any) (matched, handled bool) {
	switch {
	case rule.TimestampEquals != nil:
		cmp, ok := timestampCompare(value, *rule.TimestampEquals)

		return ok && cmp == 0, true
	case rule.TimestampLessThan != nil:
		cmp, ok := timestampCompare(value, *rule.TimestampLessThan)

		return ok && cmp < 0, true
	case rule.TimestampGreaterThan != nil:
		cmp, ok := timestampCompare(value, *rule.TimestampGreaterThan)

		return ok && cmp > 0, true
	default:
		return false, false
	}
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
