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

	pathEnabled  = "$.enabled"
	fieldEnabled = "enabled"

	testNameAlice = "alice"
	testFixed     = "fixed"

	testTimestampJan2020 = "2020-01-01T00:00:00Z"
	testTimestampJun2020 = "2020-06-01T00:00:00Z"
)

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
		rule:  choiceRule{Variable: pathMode, StringEquals: new(valueFast)},
		input: map[string]any{fieldMode: valueFast},
		want:  true,
	},
	{
		name:  "StringEquals no match",
		rule:  choiceRule{Variable: pathMode, StringEquals: new(valueFast)},
		input: map[string]any{fieldMode: valueSlow},
		want:  false,
	},
	{
		name:  "StringLessThan",
		rule:  choiceRule{Variable: pathName, StringLessThan: new("m")},
		input: map[string]any{fieldName: testNameAlice},
		want:  true,
	},
	{
		name:  "StringGreaterThan",
		rule:  choiceRule{Variable: pathName, StringGreaterThan: new("m")},
		input: map[string]any{fieldName: "zeke"},
		want:  true,
	},
	{
		name:  "StringLessThanEquals equal",
		rule:  choiceRule{Variable: pathName, StringLessThanEquals: new(fieldMode)},
		input: map[string]any{fieldName: fieldMode},
		want:  true,
	},
	{
		name:  "StringGreaterThanEquals equal",
		rule:  choiceRule{Variable: pathName, StringGreaterThanEquals: new(fieldMode)},
		input: map[string]any{fieldName: fieldMode},
		want:  true,
	},
	{
		name:  "NumericEquals match",
		rule:  choiceRule{Variable: pathValue, NumericEquals: new(float64(20))},
		input: map[string]any{fieldValue: float64(20)},
		want:  true,
	},
	{
		name:  "NumericLessThan match",
		rule:  choiceRule{Variable: pathValue, NumericLessThan: new(float64(30))},
		input: map[string]any{fieldValue: float64(20)},
		want:  true,
	},
	{
		name:  "NumericGreaterThan match",
		rule:  choiceRule{Variable: pathValue, NumericGreaterThan: new(float64(10))},
		input: map[string]any{fieldValue: float64(20)},
		want:  true,
	},
	{
		name:  "NumericLessThanEquals equal",
		rule:  choiceRule{Variable: pathValue, NumericLessThanEquals: new(float64(20))},
		input: map[string]any{fieldValue: float64(20)},
		want:  true,
	},
	{
		name:  "NumericGreaterThanEquals equal",
		rule:  choiceRule{Variable: pathValue, NumericGreaterThanEquals: new(float64(20))},
		input: map[string]any{fieldValue: float64(20)},
		want:  true,
	},
	{
		name:  "NumericGreaterThanEquals no match",
		rule:  choiceRule{Variable: pathValue, NumericGreaterThanEquals: new(float64(21))},
		input: map[string]any{fieldValue: float64(20)},
		want:  false,
	},
	{
		name:  "BooleanEquals match",
		rule:  choiceRule{Variable: pathEnabled, BooleanEquals: new(true)},
		input: map[string]any{fieldEnabled: true},
		want:  true,
	},
	{
		name:  "BooleanEquals no match",
		rule:  choiceRule{Variable: pathEnabled, BooleanEquals: new(true)},
		input: map[string]any{fieldEnabled: false},
		want:  false,
	},
	{
		name:  "TimestampEquals match",
		rule:  choiceRule{Variable: pathTS, TimestampEquals: new(testTimestampJan2020)},
		input: map[string]any{"ts": testTimestampJan2020},
		want:  true,
	},
	{
		name:  "TimestampLessThan match",
		rule:  choiceRule{Variable: pathTS, TimestampLessThan: new(testTimestampJun2020)},
		input: map[string]any{"ts": testTimestampJan2020},
		want:  true,
	},
	{
		name:  "TimestampGreaterThan match",
		rule:  choiceRule{Variable: pathTS, TimestampGreaterThan: new(testTimestampJan2020)},
		input: map[string]any{"ts": testTimestampJun2020},
		want:  true,
	},
	{
		name:  "TimestampGreaterThan malformed value does not match",
		rule:  choiceRule{Variable: pathTS, TimestampGreaterThan: new(testTimestampJan2020)},
		input: map[string]any{"ts": "not-a-timestamp"},
		want:  false,
	},
	{
		name:  "TimestampLessThanEquals equal",
		rule:  choiceRule{Variable: pathTS, TimestampLessThanEquals: new(testTimestampJan2020)},
		input: map[string]any{"ts": testTimestampJan2020},
		want:  true,
	},
	{
		name:  "TimestampGreaterThanEquals equal",
		rule:  choiceRule{Variable: pathTS, TimestampGreaterThanEquals: new(testTimestampJan2020)},
		input: map[string]any{"ts": testTimestampJan2020},
		want:  true,
	},
	{
		name:  "StringEqualsPath match",
		rule:  choiceRule{Variable: pathMode, StringEqualsPath: new(pathOther)},
		input: map[string]any{fieldMode: valueFast, fieldOther: valueFast},
		want:  true,
	},
	{
		name:  "StringEqualsPath no match",
		rule:  choiceRule{Variable: pathMode, StringEqualsPath: new(pathOther)},
		input: map[string]any{fieldMode: valueFast, fieldOther: valueSlow},
		want:  false,
	},
	{
		name:  "StringLessThanPath match",
		rule:  choiceRule{Variable: pathName, StringLessThanPath: new(pathOther)},
		input: map[string]any{fieldName: testNameAlice, fieldOther: "m"},
		want:  true,
	},
	{
		name:  "StringGreaterThanPath match",
		rule:  choiceRule{Variable: pathName, StringGreaterThanPath: new(pathOther)},
		input: map[string]any{fieldName: "zeke", fieldOther: "m"},
		want:  true,
	},
	{
		name:  "StringLessThanEqualsPath equal",
		rule:  choiceRule{Variable: pathName, StringLessThanEqualsPath: new(pathOther)},
		input: map[string]any{fieldName: fieldMode, fieldOther: fieldMode},
		want:  true,
	},
	{
		name:  "StringGreaterThanEqualsPath equal",
		rule:  choiceRule{Variable: pathName, StringGreaterThanEqualsPath: new(pathOther)},
		input: map[string]any{fieldName: fieldMode, fieldOther: fieldMode},
		want:  true,
	},
	{
		// Mirrors the ASL spec's own worked example: {"Variable": "$.rating",
		// "NumericGreaterThanPath": "$.auditThreshold"}.
		name:  "NumericGreaterThanPath match",
		rule:  choiceRule{Variable: "$.rating", NumericGreaterThanPath: new("$.auditThreshold")},
		input: map[string]any{"rating": float64(30), "auditThreshold": float64(20)},
		want:  true,
	},
	{
		name:  "NumericEqualsPath no match",
		rule:  choiceRule{Variable: pathValue, NumericEqualsPath: new(pathOther)},
		input: map[string]any{fieldValue: float64(20), fieldOther: float64(21)},
		want:  false,
	},
	{
		name:  "NumericLessThanPath match",
		rule:  choiceRule{Variable: pathValue, NumericLessThanPath: new(pathOther)},
		input: map[string]any{fieldValue: float64(20), fieldOther: float64(30)},
		want:  true,
	},
	{
		name:  "NumericGreaterThanEqualsPath equal",
		rule:  choiceRule{Variable: pathValue, NumericGreaterThanEqualsPath: new(pathOther)},
		input: map[string]any{fieldValue: float64(20), fieldOther: float64(20)},
		want:  true,
	},
	{
		name:  "NumericLessThanEqualsPath equal",
		rule:  choiceRule{Variable: pathValue, NumericLessThanEqualsPath: new(pathOther)},
		input: map[string]any{fieldValue: float64(20), fieldOther: float64(20)},
		want:  true,
	},
	{
		name:  "BooleanEqualsPath match",
		rule:  choiceRule{Variable: pathEnabled, BooleanEqualsPath: new(pathOther)},
		input: map[string]any{fieldEnabled: true, fieldOther: true},
		want:  true,
	},
	{
		name:  "BooleanEqualsPath type mismatch does not match",
		rule:  choiceRule{Variable: pathEnabled, BooleanEqualsPath: new(pathOther)},
		input: map[string]any{fieldEnabled: true, fieldOther: "not-a-bool"},
		want:  false,
	},
	{
		name:  "TimestampEqualsPath match",
		rule:  choiceRule{Variable: pathTS, TimestampEqualsPath: new(pathTSOther)},
		input: map[string]any{"ts": testTimestampJan2020, fieldTSOther: testTimestampJan2020},
		want:  true,
	},
	{
		name:  "TimestampLessThanPath match",
		rule:  choiceRule{Variable: pathTS, TimestampLessThanPath: new(pathTSOther)},
		input: map[string]any{"ts": testTimestampJan2020, fieldTSOther: testTimestampJun2020},
		want:  true,
	},
	{
		name:  "TimestampGreaterThanPath match",
		rule:  choiceRule{Variable: pathTS, TimestampGreaterThanPath: new(pathTSOther)},
		input: map[string]any{"ts": testTimestampJun2020, fieldTSOther: testTimestampJan2020},
		want:  true,
	},
	{
		name:  "TimestampLessThanEqualsPath equal",
		rule:  choiceRule{Variable: pathTS, TimestampLessThanEqualsPath: new(pathTSOther)},
		input: map[string]any{"ts": testTimestampJan2020, fieldTSOther: testTimestampJan2020},
		want:  true,
	},
	{
		name:  "TimestampGreaterThanEqualsPath equal",
		rule:  choiceRule{Variable: pathTS, TimestampGreaterThanEqualsPath: new(pathTSOther)},
		input: map[string]any{"ts": testTimestampJan2020, fieldTSOther: testTimestampJan2020},
		want:  true,
	},
	{
		name:  "IsPresent true for present field",
		rule:  choiceRule{Variable: pathMode, IsPresent: new(true)},
		input: map[string]any{fieldMode: valueFast},
		want:  true,
	},
	{
		name:  "IsPresent false for missing field",
		rule:  choiceRule{Variable: pathMissing, IsPresent: new(false)},
		input: map[string]any{fieldMode: valueFast},
		want:  true,
	},
	{
		name:  "IsPresent true expectation fails for missing field",
		rule:  choiceRule{Variable: pathMissing, IsPresent: new(true)},
		input: map[string]any{fieldMode: valueFast},
		want:  false,
	},
	{
		name:  "IsNull true for null field",
		rule:  choiceRule{Variable: pathValue, IsNull: new(true)},
		input: map[string]any{fieldValue: nil},
		want:  true,
	},
	{
		name:  "IsNull false for non-null field",
		rule:  choiceRule{Variable: pathValue, IsNull: new(false)},
		input: map[string]any{fieldValue: "x"},
		want:  true,
	},
	{
		name:  "IsNumeric true for numeric field",
		rule:  choiceRule{Variable: pathValue, IsNumeric: new(true)},
		input: map[string]any{fieldValue: float64(1)},
		want:  true,
	},
	{
		name:  "IsNumeric false for non-numeric field",
		rule:  choiceRule{Variable: pathValue, IsNumeric: new(false)},
		input: map[string]any{fieldValue: valueFast},
		want:  true,
	},
	{
		name:  "IsNumeric true expectation fails for non-numeric field",
		rule:  choiceRule{Variable: pathValue, IsNumeric: new(true)},
		input: map[string]any{fieldValue: valueFast},
		want:  false,
	},
	{
		name:  "IsString true for string field",
		rule:  choiceRule{Variable: pathValue, IsString: new(true)},
		input: map[string]any{fieldValue: valueFast},
		want:  true,
	},
	{
		name:  "IsString false for non-string field",
		rule:  choiceRule{Variable: pathValue, IsString: new(false)},
		input: map[string]any{fieldValue: float64(1)},
		want:  true,
	},
	{
		name:  "IsBoolean true for boolean field",
		rule:  choiceRule{Variable: pathValue, IsBoolean: new(true)},
		input: map[string]any{fieldValue: true},
		want:  true,
	},
	{
		name:  "IsBoolean false for non-boolean field",
		rule:  choiceRule{Variable: pathValue, IsBoolean: new(false)},
		input: map[string]any{fieldValue: valueFast},
		want:  true,
	},
	{
		name:  "IsTimestamp true for RFC3339 string field",
		rule:  choiceRule{Variable: pathValue, IsTimestamp: new(true)},
		input: map[string]any{fieldValue: testTimestampJan2020},
		want:  true,
	},
	{
		name:  "IsTimestamp false for non-timestamp string field",
		rule:  choiceRule{Variable: pathValue, IsTimestamp: new(true)},
		input: map[string]any{fieldValue: valueFast},
		want:  false,
	},
	{
		name:  "StringMatches exact match with no wildcard",
		rule:  choiceRule{Variable: pathName, StringMatches: new(testNameAlice)},
		input: map[string]any{fieldName: testNameAlice},
		want:  true,
	},
	{
		name:  "StringMatches leading wildcard",
		rule:  choiceRule{Variable: pathName, StringMatches: new("*.log")},
		input: map[string]any{fieldName: "zebra.log"},
		want:  true,
	},
	{
		name:  "StringMatches trailing wildcard",
		rule:  choiceRule{Variable: pathName, StringMatches: new("foo*")},
		input: map[string]any{fieldName: "foo23.log"},
		want:  true,
	},
	{
		name:  "StringMatches wildcard on both ends",
		rule:  choiceRule{Variable: pathName, StringMatches: new("foo*.*")},
		input: map[string]any{fieldName: "foobar.zebra"},
		want:  true,
	},
	{
		name:  "StringMatches no match",
		rule:  choiceRule{Variable: pathName, StringMatches: new("log-*.txt")},
		input: map[string]any{fieldName: "log-1.csv"},
		want:  false,
	},
	{
		name:  "StringMatches escaped wildcard is literal",
		rule:  choiceRule{Variable: pathName, StringMatches: new(`a\*b`)},
		input: map[string]any{fieldName: "a*b"},
		want:  true,
	},
	{
		name:  "StringMatches escaped wildcard does not act as wildcard",
		rule:  choiceRule{Variable: pathName, StringMatches: new(`a\*b`)},
		input: map[string]any{fieldName: "aXb"},
		want:  false,
	},
	{
		name:  "StringMatches escaped backslash is literal",
		rule:  choiceRule{Variable: pathName, StringMatches: new(`a\\b`)},
		input: map[string]any{fieldName: `a\b`},
		want:  true,
	},
	{
		name:  "StringMatches against non-string value does not match",
		rule:  choiceRule{Variable: pathValue, StringMatches: new("*")},
		input: map[string]any{fieldValue: float64(1)},
		want:  false,
	},
	{
		name: "And all match",
		rule: choiceRule{And: []choiceRule{
			{Variable: pathValue, IsPresent: new(true)},
			{Variable: pathValue, NumericGreaterThanEquals: new(float64(20))},
			{Variable: pathValue, NumericLessThan: new(float64(30))},
		}},
		input: map[string]any{fieldValue: float64(22)},
		want:  true,
	},
	{
		name: "And one mismatch",
		rule: choiceRule{And: []choiceRule{
			{Variable: pathValue, NumericGreaterThanEquals: new(float64(20))},
			{Variable: pathValue, NumericLessThan: new(float64(21))},
		}},
		input: map[string]any{fieldValue: float64(22)},
		want:  false,
	},
	{
		name: "Or one match",
		rule: choiceRule{Or: []choiceRule{
			{Variable: pathMode, StringEquals: new(valueFast)},
			{Variable: pathMode, StringEquals: new(valueSlow)},
		}},
		input: map[string]any{fieldMode: valueSlow},
		want:  true,
	},
	{
		name:  "Not negates match",
		rule:  choiceRule{Not: &choiceRule{Variable: pathMode, StringEquals: new(valueFast)}},
		input: map[string]any{fieldMode: valueSlow},
		want:  true,
	},
	{
		name:  "Not negates non-match",
		rule:  choiceRule{Not: &choiceRule{Variable: pathMode, StringEquals: new(valueFast)}},
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

	rule := choiceRule{Variable: pathMissing, StringEquals: new(valueFast)}

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
	rule := choiceRule{Variable: pathMode, StringEqualsPath: new(pathMissing)}

	_, err := evaluateChoiceRule(&rule, map[string]any{fieldMode: valueFast})
	if err == nil {
		t.Fatal("evaluateChoiceRule: want error for unresolvable *Path comparator operand, got nil")
	}
}

func TestEvaluateChoiceRuleStringMatchesDanglingBackslashErrors(t *testing.T) {
	t.Parallel()

	rule := choiceRule{Variable: pathName, StringMatches: new(`a\`)}

	_, err := evaluateChoiceRule(&rule, map[string]any{fieldName: "a"})
	if err == nil {
		t.Fatal("evaluateChoiceRule: want error for StringMatches pattern with a dangling backslash, got nil")
	}
}

func TestEvaluateChoiceRuleStringMatchesInvalidEscapeErrors(t *testing.T) {
	t.Parallel()

	rule := choiceRule{Variable: pathName, StringMatches: new(`a\bc`)}

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
