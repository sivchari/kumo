package sfn

import "testing"

// comprehensiveValidDefinition exercises every state type kumo's engine
// supports (Task, Choice, Wait, Succeed, Fail, Parallel, Map, plus Retry and
// Catch), standing in for every currently-passing definition used in kumo's
// own test suite, which must keep validating OK.
const comprehensiveValidDefinition = `{
	"StartAt": "Decide",
	"States": {
		"Decide": {
			"Type": "Choice",
			"Choices": [{"Variable": "$.mode", "StringEquals": "fast", "Next": "Fork"}],
			"Default": "Wait"
		},
		"Wait": {"Type": "Wait", "Seconds": 1, "Next": "Fork"},
		"Fork": {
			"Type": "Parallel",
			"Branches": [
				{"StartAt": "A", "States": {"A": {"Type": "Pass", "End": true}}}
			],
			"Retry": [{"ErrorEquals": ["States.ALL"], "MaxAttempts": 1}],
			"Catch": [{"ErrorEquals": ["States.ALL"], "Next": "Failed"}],
			"Next": "Items"
		},
		"Items": {
			"Type": "Map",
			"ItemProcessor": {"StartAt": "Item", "States": {"Item": {"Type": "Pass", "End": true}}},
			"Next": "Invoke"
		},
		"Invoke": {
			"Type": "Task",
			"Resource": "arn:aws:lambda:us-east-1:000000000000:function:fn",
			"Catch": [{"ErrorEquals": ["States.ALL"], "Next": "Failed"}],
			"Next": "Done"
		},
		"Done": {"Type": "Succeed"},
		"Failed": {"Type": "Fail", "Error": "BadThing", "Cause": "it broke"}
	}
}`

func TestValidateStateMachineDefinitionExistingDefinitionsStillOK(t *testing.T) {
	t.Parallel()

	for name, definition := range map[string]string{
		"comprehensive":  comprehensiveValidDefinition,
		"execution test": executionTestDefinition,
		"choice repro":   choiceReproDefinition,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			diagnostics := validateStateMachineDefinition(definition)
			if got := diagnosticsResult(diagnostics); got != validationResultOK {
				t.Fatalf("result: got %q, want %q (diagnostics: %+v)", got, validationResultOK, diagnostics)
			}
		})
	}
}

// diagnosticTest is one case in validateDiagnosticTests: a definition and
// the single diagnostic it must produce, identified by code and location.
type diagnosticTest struct {
	name         string
	definition   string
	wantSeverity string
	wantCode     string
	wantLocation string
}

var validateDiagnosticTests = []diagnosticTest{
	{
		name: "missing StartAt target",
		definition: `{
			"StartAt": "Nope",
			"States": {"Done": {"Type": "Pass", "End": true}}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeMissingTransitionTarget,
		wantLocation: "/StartAt",
	},
	{
		name: "dangling Next",
		definition: `{
			"StartAt": "Start",
			"States": {"Start": {"Type": "Pass", "Next": "Nope"}}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeMissingTransitionTarget,
		wantLocation: "/States/Start/Next",
	},
	{
		name: "missing Task Resource",
		definition: `{
			"StartAt": "Invoke",
			"States": {"Invoke": {"Type": "Task", "End": true}}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeInvalidResource,
		wantLocation: "/States/Invoke/Resource",
	},
	{
		name: "dead-end state missing Next or End",
		definition: `{
			"StartAt": "Start",
			"States": {"Start": {"Type": "Pass"}}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeMissingEndState,
		wantLocation: "/States/Start",
	},
	{
		name: "empty Choices",
		definition: `{
			"StartAt": "Decide",
			"States": {"Decide": {"Type": "Choice", "Choices": []}}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeSchemaValidationFailed,
		wantLocation: "/States/Decide/Choices",
	},
	{
		name: "empty Branches",
		definition: `{
			"StartAt": "Fork",
			"States": {"Fork": {"Type": "Parallel", "Branches": [], "End": true}}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeSchemaValidationFailed,
		wantLocation: "/States/Fork/Branches",
	},
	{
		name: "Map missing ItemProcessor and Iterator",
		definition: `{
			"StartAt": "Items",
			"States": {"Items": {"Type": "Map", "End": true}}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeSchemaValidationFailed,
		wantLocation: "/States/Items",
	},
	{
		name: "Wait with no duration field",
		definition: `{
			"StartAt": "W",
			"States": {"W": {"Type": "Wait", "End": true}}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeSchemaValidationFailed,
		wantLocation: "/States/W",
	},
	{
		name: "Wait with more than one duration field",
		definition: `{
			"StartAt": "W",
			"States": {"W": {"Type": "Wait", "Seconds": 1, "SecondsPath": "$.s", "End": true}}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeSchemaValidationFailed,
		wantLocation: "/States/W",
	},
	{
		name: "dangling Catch Next",
		definition: `{
			"StartAt": "Invoke",
			"States": {
				"Invoke": {
					"Type": "Task",
					"Resource": "arn:aws:lambda:us-east-1:000000000000:function:fn",
					"Catch": [{"ErrorEquals": ["States.ALL"], "Next": "Nope"}],
					"End": true
				}
			}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeMissingTransitionTarget,
		wantLocation: "/States/Invoke/Catch/0/Next",
	},
	{
		name: "unreachable state",
		definition: `{
			"StartAt": "Start",
			"States": {
				"Start": {"Type": "Pass", "End": true},
				"Orphan": {"Type": "Pass", "End": true}
			}
		}`,
		wantSeverity: diagnosticSeverityWarning,
		wantCode:     codeUnreachableState,
		wantLocation: "/States/Orphan",
	},
	{
		name: "Map field kumo does not implement",
		definition: `{
			"StartAt": "Items",
			"States": {
				"Items": {
					"Type": "Map",
					"ItemProcessor": {"StartAt": "Item", "States": {"Item": {"Type": "Pass", "End": true}}},
					"ResultWriter": {"Resource": "arn:aws:states:::s3:putObject"},
					"End": true
				}
			}
		}`,
		wantSeverity: diagnosticSeverityWarning,
		wantCode:     codeUnsupportedRuntimeConstruct,
		wantLocation: "/States/Items/ResultWriter",
	},
	{
		name: "nested branch dangling Next",
		definition: `{
			"StartAt": "Fork",
			"States": {
				"Fork": {
					"Type": "Parallel",
					"Branches": [
						{"StartAt": "A", "States": {"A": {"Type": "Pass", "Next": "Nope"}}}
					],
					"End": true
				}
			}
		}`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeMissingTransitionTarget,
		wantLocation: "/States/Fork/Branches/0/States/A/Next",
	},
	{
		name:         "invalid JSON",
		definition:   `{not json`,
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeInvalidJSONDescription,
		wantLocation: "",
	},
}

func TestValidateStateMachineDefinitionDiagnostics(t *testing.T) {
	t.Parallel()

	for _, tt := range validateDiagnosticTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertHasDiagnostic(t, validateStateMachineDefinition(tt.definition), &tt)
		})
	}
}

// assertHasDiagnostic fails the test unless diagnostics contains an entry
// matching tt's expected code, location, and severity, with a non-empty
// message.
func assertHasDiagnostic(t *testing.T, diagnostics []validateDiagnostic, tt *diagnosticTest) {
	t.Helper()

	for _, d := range diagnostics {
		if d.Code != tt.wantCode || d.Location != tt.wantLocation {
			continue
		}

		if d.Severity != tt.wantSeverity {
			t.Fatalf("diagnostic %q at %q: severity got %q, want %q", d.Code, d.Location, d.Severity, tt.wantSeverity)
		}

		if d.Message == "" {
			t.Fatalf("diagnostic %q at %q: want non-empty message", d.Code, d.Location)
		}

		return
	}

	t.Fatalf("diagnostics %+v: want one with code %q at location %q", diagnostics, tt.wantCode, tt.wantLocation)
}

func TestValidateStateMachineDefinitionValidDefinitionHasNoDiagnostics(t *testing.T) {
	t.Parallel()

	diagnostics := validateStateMachineDefinition(comprehensiveValidDefinition)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics: got %+v, want none", diagnostics)
	}
}

func TestDiagnosticsResultFailsOnlyForErrorSeverity(t *testing.T) {
	t.Parallel()

	warningOnly := []validateDiagnostic{{Severity: diagnosticSeverityWarning}}
	if got := diagnosticsResult(warningOnly); got != validationResultOK {
		t.Fatalf("result for warning-only diagnostics: got %q, want %q", got, validationResultOK)
	}

	withError := []validateDiagnostic{{Severity: diagnosticSeverityWarning}, {Severity: diagnosticSeverityError}}
	if got := diagnosticsResult(withError); got != validationResultFail {
		t.Fatalf("result with an ERROR diagnostic present: got %q, want %q", got, validationResultFail)
	}
}

func TestFilterAndCapDiagnosticsDefaultsToErrorOnly(t *testing.T) {
	t.Parallel()

	diagnostics := []validateDiagnostic{
		{Severity: diagnosticSeverityError, Code: "E"},
		{Severity: diagnosticSeverityWarning, Code: "W"},
	}

	filtered, truncated := filterAndCapDiagnostics(diagnostics, "", 0)
	if truncated {
		t.Fatal("truncated: got true, want false")
	}

	if len(filtered) != 1 || filtered[0].Code != "E" {
		t.Fatalf("filtered diagnostics: got %+v, want only the ERROR entry", filtered)
	}

	filtered, truncated = filterAndCapDiagnostics(diagnostics, diagnosticSeverityWarning, 0)
	if truncated {
		t.Fatal("truncated: got true, want false")
	}

	if len(filtered) != 2 {
		t.Fatalf("filtered diagnostics with severity=WARNING: got %+v, want both entries", filtered)
	}
}

func TestFilterAndCapDiagnosticsCapsAtMaxResults(t *testing.T) {
	t.Parallel()

	diagnostics := make([]validateDiagnostic, 0, 3)
	for i := 0; i < 3; i++ {
		diagnostics = append(diagnostics, validateDiagnostic{Severity: diagnosticSeverityError})
	}

	filtered, truncated := filterAndCapDiagnostics(diagnostics, "", 2)
	if !truncated {
		t.Fatal("truncated: got false, want true")
	}

	if len(filtered) != 2 {
		t.Fatalf("filtered diagnostics: got %d entries, want 2", len(filtered))
	}
}
