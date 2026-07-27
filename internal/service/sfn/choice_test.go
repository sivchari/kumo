package sfn

import (
	"testing"
)

// JSONPath and value literals shared across the choiceRuleTests table,
// factored into consts to stay under goconst's repeated-literal threshold.
const (
	pathMode    = "$.mode"
	pathName    = "$.name"
	pathValue   = "$.value"
	pathTS      = "$.ts"
	pathMissing = "$.missing"

	valueFast = "fast"
	valueSlow = "slow"

	fieldName  = "name"
	fieldValue = "value"
	fieldMode  = "mode"
)

func float64Ptr(v float64) *float64 { return &v }

func strPtr(v string) *string { return &v }

func boolPtr(v bool) *bool { return &v }

// choiceRuleTests exercises the individual comparison operators via
// evaluateChoiceRule. Kept as a package-level var so the driving test
// function itself stays short.
var choiceRuleTests = []struct {
	name  string
	rule  choiceRule
	input map[string]any
	want  bool
}{
	{
		name:  "StringEquals match",
		rule:  choiceRule{Variable: pathMode, StringEquals: strPtr(valueFast)},
		input: map[string]any{fieldMode: valueFast},
		want:  true,
	},
	{
		name:  "StringEquals no match",
		rule:  choiceRule{Variable: pathMode, StringEquals: strPtr(valueFast)},
		input: map[string]any{fieldMode: valueSlow},
		want:  false,
	},
	{
		name:  "StringLessThan",
		rule:  choiceRule{Variable: pathName, StringLessThan: strPtr("m")},
		input: map[string]any{fieldName: "alice"},
		want:  true,
	},
	{
		name:  "StringGreaterThan",
		rule:  choiceRule{Variable: pathName, StringGreaterThan: strPtr("m")},
		input: map[string]any{fieldName: "zeke"},
		want:  true,
	},
	{
		name:  "StringLessThanEquals equal",
		rule:  choiceRule{Variable: pathName, StringLessThanEquals: strPtr("mode")},
		input: map[string]any{fieldName: "mode"},
		want:  true,
	},
	{
		name:  "StringGreaterThanEquals equal",
		rule:  choiceRule{Variable: pathName, StringGreaterThanEquals: strPtr("mode")},
		input: map[string]any{fieldName: "mode"},
		want:  true,
	},
	{
		name:  "NumericEquals match",
		rule:  choiceRule{Variable: pathValue, NumericEquals: float64Ptr(20)},
		input: map[string]any{fieldValue: float64(20)},
		want:  true,
	},
	{
		name:  "NumericLessThan match",
		rule:  choiceRule{Variable: pathValue, NumericLessThan: float64Ptr(30)},
		input: map[string]any{fieldValue: float64(20)},
		want:  true,
	},
	{
		name:  "NumericGreaterThan match",
		rule:  choiceRule{Variable: pathValue, NumericGreaterThan: float64Ptr(10)},
		input: map[string]any{fieldValue: float64(20)},
		want:  true,
	},
	{
		name:  "NumericLessThanEquals equal",
		rule:  choiceRule{Variable: pathValue, NumericLessThanEquals: float64Ptr(20)},
		input: map[string]any{fieldValue: float64(20)},
		want:  true,
	},
	{
		name:  "NumericGreaterThanEquals equal",
		rule:  choiceRule{Variable: pathValue, NumericGreaterThanEquals: float64Ptr(20)},
		input: map[string]any{fieldValue: float64(20)},
		want:  true,
	},
	{
		name:  "NumericGreaterThanEquals no match",
		rule:  choiceRule{Variable: pathValue, NumericGreaterThanEquals: float64Ptr(21)},
		input: map[string]any{fieldValue: float64(20)},
		want:  false,
	},
	{
		name:  "BooleanEquals match",
		rule:  choiceRule{Variable: "$.enabled", BooleanEquals: boolPtr(true)},
		input: map[string]any{"enabled": true},
		want:  true,
	},
	{
		name:  "BooleanEquals no match",
		rule:  choiceRule{Variable: "$.enabled", BooleanEquals: boolPtr(true)},
		input: map[string]any{"enabled": false},
		want:  false,
	},
	{
		name:  "TimestampEquals match",
		rule:  choiceRule{Variable: pathTS, TimestampEquals: strPtr("2020-01-01T00:00:00Z")},
		input: map[string]any{"ts": "2020-01-01T00:00:00Z"},
		want:  true,
	},
	{
		name:  "TimestampLessThan match",
		rule:  choiceRule{Variable: pathTS, TimestampLessThan: strPtr("2020-06-01T00:00:00Z")},
		input: map[string]any{"ts": "2020-01-01T00:00:00Z"},
		want:  true,
	},
	{
		name:  "TimestampGreaterThan match",
		rule:  choiceRule{Variable: pathTS, TimestampGreaterThan: strPtr("2020-01-01T00:00:00Z")},
		input: map[string]any{"ts": "2020-06-01T00:00:00Z"},
		want:  true,
	},
	{
		name:  "TimestampGreaterThan malformed value does not match",
		rule:  choiceRule{Variable: pathTS, TimestampGreaterThan: strPtr("2020-01-01T00:00:00Z")},
		input: map[string]any{"ts": "not-a-timestamp"},
		want:  false,
	},
	{
		name:  "IsPresent true for present field",
		rule:  choiceRule{Variable: pathMode, IsPresent: boolPtr(true)},
		input: map[string]any{fieldMode: valueFast},
		want:  true,
	},
	{
		name:  "IsPresent false for missing field",
		rule:  choiceRule{Variable: pathMissing, IsPresent: boolPtr(false)},
		input: map[string]any{fieldMode: valueFast},
		want:  true,
	},
	{
		name:  "IsPresent true expectation fails for missing field",
		rule:  choiceRule{Variable: pathMissing, IsPresent: boolPtr(true)},
		input: map[string]any{fieldMode: valueFast},
		want:  false,
	},
	{
		name:  "IsNull true for null field",
		rule:  choiceRule{Variable: pathValue, IsNull: boolPtr(true)},
		input: map[string]any{fieldValue: nil},
		want:  true,
	},
	{
		name:  "IsNull false for non-null field",
		rule:  choiceRule{Variable: pathValue, IsNull: boolPtr(false)},
		input: map[string]any{fieldValue: "x"},
		want:  true,
	},
	{
		name: "And all match",
		rule: choiceRule{And: []choiceRule{
			{Variable: pathValue, IsPresent: boolPtr(true)},
			{Variable: pathValue, NumericGreaterThanEquals: float64Ptr(20)},
			{Variable: pathValue, NumericLessThan: float64Ptr(30)},
		}},
		input: map[string]any{fieldValue: float64(22)},
		want:  true,
	},
	{
		name: "And one mismatch",
		rule: choiceRule{And: []choiceRule{
			{Variable: pathValue, NumericGreaterThanEquals: float64Ptr(20)},
			{Variable: pathValue, NumericLessThan: float64Ptr(21)},
		}},
		input: map[string]any{fieldValue: float64(22)},
		want:  false,
	},
	{
		name: "Or one match",
		rule: choiceRule{Or: []choiceRule{
			{Variable: pathMode, StringEquals: strPtr(valueFast)},
			{Variable: pathMode, StringEquals: strPtr(valueSlow)},
		}},
		input: map[string]any{fieldMode: valueSlow},
		want:  true,
	},
	{
		name:  "Not negates match",
		rule:  choiceRule{Not: &choiceRule{Variable: pathMode, StringEquals: strPtr(valueFast)}},
		input: map[string]any{fieldMode: valueSlow},
		want:  true,
	},
	{
		name:  "Not negates non-match",
		rule:  choiceRule{Not: &choiceRule{Variable: pathMode, StringEquals: strPtr(valueFast)}},
		input: map[string]any{fieldMode: valueFast},
		want:  false,
	},
}

func TestEvaluateChoiceRule(t *testing.T) {
	t.Parallel()

	for _, tt := range choiceRuleTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := evaluateChoiceRule(&tt.rule, tt.input)
			if err != nil {
				t.Fatalf("evaluateChoiceRule: %v", err)
			}

			if got != tt.want {
				t.Fatalf("evaluateChoiceRule() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateChoiceRuleMissingVariableErrors(t *testing.T) {
	t.Parallel()

	rule := choiceRule{Variable: pathMissing, StringEquals: strPtr(valueFast)}

	_, err := evaluateChoiceRule(&rule, map[string]any{fieldMode: valueFast})
	if err == nil {
		t.Fatal("evaluateChoiceRule: want error for unresolvable Variable, got nil")
	}
}

const choiceReproDefinition = `{
	"StartAt": "Decide",
	"States": {
		"Decide": {
			"Type": "Choice",
			"Choices": [{"Variable": "$.mode", "StringEquals": "fast", "Next": "Fast"}],
			"Default": "Slow"
		},
		"Fast": {"Type": "Pass", "Result": {"picked": "fast"}, "End": true},
		"Slow": {"Type": "Pass", "Result": {"picked": "slow"}, "End": true}
	}
}`

func TestChoiceStateRoutesToMatchedNext(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "choice-fast", choiceReproDefinition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"mode":"fast"}`)

	if exec.Output != `{"picked":"fast"}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"picked":"fast"}`)
	}
}

func TestChoiceStateFallsBackToDefault(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "choice-slow", choiceReproDefinition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"mode":"slow"}`)

	if exec.Output != `{"picked":"slow"}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"picked":"slow"}`)
	}
}

func TestChoiceStateNoMatchNoDefaultReportsNoChoiceMatched(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Decide",
		"States": {
			"Decide": {
				"Type": "Choice",
				"Choices": [{"Variable": "$.mode", "StringEquals": "fast", "Next": "Fast"}]
			},
			"Fast": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "choice-no-match", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{"mode":"slow"}`)

	if exec.Error != errorStatesNoChoiceMatched {
		t.Fatalf("execution error: got %q, want %q (cause: %s)", exec.Error, errorStatesNoChoiceMatched, exec.Cause)
	}
}

func TestChoiceStateNumericAndCombinator(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Decide",
		"States": {
			"Decide": {
				"Type": "Choice",
				"Choices": [{
					"And": [
						{"Variable": "$.value", "NumericGreaterThanEquals": 20},
						{"Variable": "$.value", "NumericLessThan": 30}
					],
					"Next": "Twenties"
				}],
				"Default": "Other"
			},
			"Twenties": {"Type": "Pass", "Result": {"range": "twenties"}, "End": true},
			"Other": {"Type": "Pass", "Result": {"range": "other"}, "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "choice-and", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"value":22}`)

	if exec.Output != `{"range":"twenties"}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"range":"twenties"}`)
	}
}

func TestChoiceStateIsPresentGuardsMissingField(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Decide",
		"States": {
			"Decide": {
				"Type": "Choice",
				"Choices": [{
					"Not": {"Variable": "$.mode", "IsPresent": true},
					"Next": "NoMode"
				}],
				"Default": "HasMode"
			},
			"NoMode": {"Type": "Pass", "Result": {"picked": "no-mode"}, "End": true},
			"HasMode": {"Type": "Pass", "Result": {"picked": "has-mode"}, "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "choice-ispresent", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{}`)

	if exec.Output != `{"picked":"no-mode"}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"picked":"no-mode"}`)
	}
}
