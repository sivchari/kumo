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
	pathOther   = "$.other"
	pathTSOther = "$.tsOther"

	valueFast = "fast"
	valueSlow = "slow"

	fieldName    = "name"
	fieldValue   = "value"
	fieldMode    = "mode"
	fieldOther   = "other"
	fieldTSOther = "tsOther"
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
		name:  "TimestampLessThanEquals equal",
		rule:  choiceRule{Variable: pathTS, TimestampLessThanEquals: strPtr("2020-01-01T00:00:00Z")},
		input: map[string]any{"ts": "2020-01-01T00:00:00Z"},
		want:  true,
	},
	{
		name:  "TimestampGreaterThanEquals equal",
		rule:  choiceRule{Variable: pathTS, TimestampGreaterThanEquals: strPtr("2020-01-01T00:00:00Z")},
		input: map[string]any{"ts": "2020-01-01T00:00:00Z"},
		want:  true,
	},
	{
		name:  "StringEqualsPath match",
		rule:  choiceRule{Variable: pathMode, StringEqualsPath: strPtr(pathOther)},
		input: map[string]any{fieldMode: valueFast, fieldOther: valueFast},
		want:  true,
	},
	{
		name:  "StringEqualsPath no match",
		rule:  choiceRule{Variable: pathMode, StringEqualsPath: strPtr(pathOther)},
		input: map[string]any{fieldMode: valueFast, fieldOther: valueSlow},
		want:  false,
	},
	{
		name:  "StringLessThanPath match",
		rule:  choiceRule{Variable: pathName, StringLessThanPath: strPtr(pathOther)},
		input: map[string]any{fieldName: "alice", fieldOther: "m"},
		want:  true,
	},
	{
		name:  "StringGreaterThanPath match",
		rule:  choiceRule{Variable: pathName, StringGreaterThanPath: strPtr(pathOther)},
		input: map[string]any{fieldName: "zeke", fieldOther: "m"},
		want:  true,
	},
	{
		name:  "StringLessThanEqualsPath equal",
		rule:  choiceRule{Variable: pathName, StringLessThanEqualsPath: strPtr(pathOther)},
		input: map[string]any{fieldName: "mode", fieldOther: "mode"},
		want:  true,
	},
	{
		name:  "StringGreaterThanEqualsPath equal",
		rule:  choiceRule{Variable: pathName, StringGreaterThanEqualsPath: strPtr(pathOther)},
		input: map[string]any{fieldName: "mode", fieldOther: "mode"},
		want:  true,
	},
	{
		// Mirrors the ASL spec's own worked example: {"Variable": "$.rating",
		// "NumericGreaterThanPath": "$.auditThreshold"}.
		name:  "NumericGreaterThanPath match",
		rule:  choiceRule{Variable: "$.rating", NumericGreaterThanPath: strPtr("$.auditThreshold")},
		input: map[string]any{"rating": float64(30), "auditThreshold": float64(20)},
		want:  true,
	},
	{
		name:  "NumericEqualsPath no match",
		rule:  choiceRule{Variable: pathValue, NumericEqualsPath: strPtr(pathOther)},
		input: map[string]any{fieldValue: float64(20), fieldOther: float64(21)},
		want:  false,
	},
	{
		name:  "NumericLessThanPath match",
		rule:  choiceRule{Variable: pathValue, NumericLessThanPath: strPtr(pathOther)},
		input: map[string]any{fieldValue: float64(20), fieldOther: float64(30)},
		want:  true,
	},
	{
		name:  "NumericGreaterThanEqualsPath equal",
		rule:  choiceRule{Variable: pathValue, NumericGreaterThanEqualsPath: strPtr(pathOther)},
		input: map[string]any{fieldValue: float64(20), fieldOther: float64(20)},
		want:  true,
	},
	{
		name:  "NumericLessThanEqualsPath equal",
		rule:  choiceRule{Variable: pathValue, NumericLessThanEqualsPath: strPtr(pathOther)},
		input: map[string]any{fieldValue: float64(20), fieldOther: float64(20)},
		want:  true,
	},
	{
		name:  "BooleanEqualsPath match",
		rule:  choiceRule{Variable: "$.enabled", BooleanEqualsPath: strPtr(pathOther)},
		input: map[string]any{"enabled": true, fieldOther: true},
		want:  true,
	},
	{
		name:  "BooleanEqualsPath type mismatch does not match",
		rule:  choiceRule{Variable: "$.enabled", BooleanEqualsPath: strPtr(pathOther)},
		input: map[string]any{"enabled": true, fieldOther: "not-a-bool"},
		want:  false,
	},
	{
		name:  "TimestampEqualsPath match",
		rule:  choiceRule{Variable: pathTS, TimestampEqualsPath: strPtr(pathTSOther)},
		input: map[string]any{"ts": "2020-01-01T00:00:00Z", fieldTSOther: "2020-01-01T00:00:00Z"},
		want:  true,
	},
	{
		name:  "TimestampLessThanPath match",
		rule:  choiceRule{Variable: pathTS, TimestampLessThanPath: strPtr(pathTSOther)},
		input: map[string]any{"ts": "2020-01-01T00:00:00Z", fieldTSOther: "2020-06-01T00:00:00Z"},
		want:  true,
	},
	{
		name:  "TimestampGreaterThanPath match",
		rule:  choiceRule{Variable: pathTS, TimestampGreaterThanPath: strPtr(pathTSOther)},
		input: map[string]any{"ts": "2020-06-01T00:00:00Z", fieldTSOther: "2020-01-01T00:00:00Z"},
		want:  true,
	},
	{
		name:  "TimestampLessThanEqualsPath equal",
		rule:  choiceRule{Variable: pathTS, TimestampLessThanEqualsPath: strPtr(pathTSOther)},
		input: map[string]any{"ts": "2020-01-01T00:00:00Z", fieldTSOther: "2020-01-01T00:00:00Z"},
		want:  true,
	},
	{
		name:  "TimestampGreaterThanEqualsPath equal",
		rule:  choiceRule{Variable: pathTS, TimestampGreaterThanEqualsPath: strPtr(pathTSOther)},
		input: map[string]any{"ts": "2020-01-01T00:00:00Z", fieldTSOther: "2020-01-01T00:00:00Z"},
		want:  true,
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
		name:  "IsNumeric true for numeric field",
		rule:  choiceRule{Variable: pathValue, IsNumeric: boolPtr(true)},
		input: map[string]any{fieldValue: float64(1)},
		want:  true,
	},
	{
		name:  "IsNumeric false for non-numeric field",
		rule:  choiceRule{Variable: pathValue, IsNumeric: boolPtr(false)},
		input: map[string]any{fieldValue: valueFast},
		want:  true,
	},
	{
		name:  "IsNumeric true expectation fails for non-numeric field",
		rule:  choiceRule{Variable: pathValue, IsNumeric: boolPtr(true)},
		input: map[string]any{fieldValue: valueFast},
		want:  false,
	},
	{
		name:  "IsString true for string field",
		rule:  choiceRule{Variable: pathValue, IsString: boolPtr(true)},
		input: map[string]any{fieldValue: valueFast},
		want:  true,
	},
	{
		name:  "IsString false for non-string field",
		rule:  choiceRule{Variable: pathValue, IsString: boolPtr(false)},
		input: map[string]any{fieldValue: float64(1)},
		want:  true,
	},
	{
		name:  "IsBoolean true for boolean field",
		rule:  choiceRule{Variable: pathValue, IsBoolean: boolPtr(true)},
		input: map[string]any{fieldValue: true},
		want:  true,
	},
	{
		name:  "IsBoolean false for non-boolean field",
		rule:  choiceRule{Variable: pathValue, IsBoolean: boolPtr(false)},
		input: map[string]any{fieldValue: valueFast},
		want:  true,
	},
	{
		name:  "IsTimestamp true for RFC3339 string field",
		rule:  choiceRule{Variable: pathValue, IsTimestamp: boolPtr(true)},
		input: map[string]any{fieldValue: "2020-01-01T00:00:00Z"},
		want:  true,
	},
	{
		name:  "IsTimestamp false for non-timestamp string field",
		rule:  choiceRule{Variable: pathValue, IsTimestamp: boolPtr(true)},
		input: map[string]any{fieldValue: valueFast},
		want:  false,
	},
	{
		name:  "StringMatches exact match with no wildcard",
		rule:  choiceRule{Variable: pathName, StringMatches: strPtr("alice")},
		input: map[string]any{fieldName: "alice"},
		want:  true,
	},
	{
		name:  "StringMatches leading wildcard",
		rule:  choiceRule{Variable: pathName, StringMatches: strPtr("*.log")},
		input: map[string]any{fieldName: "zebra.log"},
		want:  true,
	},
	{
		name:  "StringMatches trailing wildcard",
		rule:  choiceRule{Variable: pathName, StringMatches: strPtr("foo*")},
		input: map[string]any{fieldName: "foo23.log"},
		want:  true,
	},
	{
		name:  "StringMatches wildcard on both ends",
		rule:  choiceRule{Variable: pathName, StringMatches: strPtr("foo*.*")},
		input: map[string]any{fieldName: "foobar.zebra"},
		want:  true,
	},
	{
		name:  "StringMatches no match",
		rule:  choiceRule{Variable: pathName, StringMatches: strPtr("log-*.txt")},
		input: map[string]any{fieldName: "log-1.csv"},
		want:  false,
	},
	{
		name:  "StringMatches escaped wildcard is literal",
		rule:  choiceRule{Variable: pathName, StringMatches: strPtr(`a\*b`)},
		input: map[string]any{fieldName: "a*b"},
		want:  true,
	},
	{
		name:  "StringMatches escaped wildcard does not act as wildcard",
		rule:  choiceRule{Variable: pathName, StringMatches: strPtr(`a\*b`)},
		input: map[string]any{fieldName: "aXb"},
		want:  false,
	},
	{
		name:  "StringMatches escaped backslash is literal",
		rule:  choiceRule{Variable: pathName, StringMatches: strPtr(`a\\b`)},
		input: map[string]any{fieldName: `a\b`},
		want:  true,
	},
	{
		name:  "StringMatches against non-string value does not match",
		rule:  choiceRule{Variable: pathValue, StringMatches: strPtr("*")},
		input: map[string]any{fieldValue: float64(1)},
		want:  false,
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

func TestEvaluateChoiceRulePathComparatorUnresolvableOperandErrors(t *testing.T) {
	t.Parallel()

	// The "*Path" comparator's own Path -- as opposed to Variable -- fails
	// to resolve here, which must also surface as an error rather than a
	// silent non-match.
	rule := choiceRule{Variable: pathMode, StringEqualsPath: strPtr(pathMissing)}

	_, err := evaluateChoiceRule(&rule, map[string]any{fieldMode: valueFast})
	if err == nil {
		t.Fatal("evaluateChoiceRule: want error for unresolvable *Path comparator operand, got nil")
	}
}

func TestEvaluateChoiceRuleStringMatchesDanglingBackslashErrors(t *testing.T) {
	t.Parallel()

	rule := choiceRule{Variable: pathName, StringMatches: strPtr(`a\`)}

	_, err := evaluateChoiceRule(&rule, map[string]any{fieldName: "a"})
	if err == nil {
		t.Fatal("evaluateChoiceRule: want error for StringMatches pattern with a dangling backslash, got nil")
	}
}

func TestEvaluateChoiceRuleStringMatchesInvalidEscapeErrors(t *testing.T) {
	t.Parallel()

	rule := choiceRule{Variable: pathName, StringMatches: strPtr(`a\bc`)}

	_, err := evaluateChoiceRule(&rule, map[string]any{fieldName: "abc"})
	if err == nil {
		t.Fatal("evaluateChoiceRule: want error for StringMatches pattern escaping a character other than '*' or '\\', got nil")
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

// TestChoiceStateNumericGreaterThanPathComparesTwoInputFields mirrors the
// ASL spec's own worked example of a "*Path" comparator: {"Variable":
// "$.rating", "NumericGreaterThanPath": "$.auditThreshold"} compares two
// fields of the same input against each other, rather than a field against
// a static literal.
func TestChoiceStateNumericGreaterThanPathComparesTwoInputFields(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Decide",
		"States": {
			"Decide": {
				"Type": "Choice",
				"Choices": [{
					"Variable": "$.rating",
					"NumericGreaterThanPath": "$.auditThreshold",
					"Next": "StartAudit"
				}],
				"Default": "NoAudit"
			},
			"StartAudit": {"Type": "Pass", "Result": {"decision": "audit"}, "End": true},
			"NoAudit": {"Type": "Pass", "Result": {"decision": "none"}, "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "choice-numeric-greater-than-path", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"rating":90,"auditThreshold":80}`)

	if exec.Output != `{"decision":"audit"}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"decision":"audit"}`)
	}

	exec = startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"rating":50,"auditThreshold":80}`)

	if exec.Output != `{"decision":"none"}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"decision":"none"}`)
	}
}
