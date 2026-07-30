package sfn

import (
	"encoding/json"
	"testing"
)

func TestTaskCatchRoutesToFallbackWithDefaultResultPath(t *testing.T) {
	t.Parallel()

	// No listener on this port, so the task invocation itself fails and
	// reports States.TaskFailed, which the Catch below matches.
	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:missing-fn",
				"Catch": [{"ErrorEquals": ["States.TaskFailed"], "Next": "Fallback"}],
				"End": true
			},
			"Fallback": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage(WithBaseURL("http://127.0.0.1:1"))
	sm := createExecutionTestStateMachine(t, store, "catch-default", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"orig":"data"}`)

	var errorOutput map[string]string
	if err := json.Unmarshal([]byte(exec.Output), &errorOutput); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if errorOutput["Error"] != errorStatesTaskFailed {
		t.Fatalf("caught error output Error: got %q, want %q", errorOutput["Error"], errorStatesTaskFailed)
	}

	if errorOutput["Cause"] == "" {
		t.Fatal("caught error output Cause: got empty string, want a non-empty cause")
	}
}

func TestTaskCatchInjectsErrorViaResultPath(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:missing-fn",
				"Catch": [{"ErrorEquals": ["States.ALL"], "ResultPath": "$.err", "Next": "Fallback"}],
				"End": true
			},
			"Fallback": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage(WithBaseURL("http://127.0.0.1:1"))
	sm := createExecutionTestStateMachine(t, store, "catch-resultpath", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"orig":"data"}`)

	var output struct {
		Orig string            `json:"orig"`
		Err  map[string]string `json:"err"`
	}

	if err := json.Unmarshal([]byte(exec.Output), &output); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if output.Orig != "data" {
		t.Fatalf("original input field: got %q, want %q", output.Orig, "data")
	}

	if output.Err["Error"] != errorStatesTaskFailed {
		t.Fatalf("injected error output Error: got %q, want %q", output.Err["Error"], errorStatesTaskFailed)
	}
}

func TestTaskCatchInjectsErrorViaDeepResultPath(t *testing.T) {
	t.Parallel()

	// The Catcher's ResultPath is now backed by the same arbitrary-depth
	// applyResultPath used elsewhere, so a multi-level path must create
	// intermediate objects exactly like a state's top-level ResultPath does.
	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:missing-fn",
				"Catch": [{"ErrorEquals": ["States.ALL"], "ResultPath": "$.a.b.c", "Next": "Fallback"}],
				"End": true
			},
			"Fallback": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage(WithBaseURL("http://127.0.0.1:1"))
	sm := createExecutionTestStateMachine(t, store, "catch-deep-resultpath", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"orig":"data"}`)

	var output struct {
		Orig string `json:"orig"`
		A    struct {
			B struct {
				C map[string]string `json:"c"`
			} `json:"b"`
		} `json:"a"`
	}

	if err := json.Unmarshal([]byte(exec.Output), &output); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if output.Orig != "data" {
		t.Fatalf("original input field: got %q, want %q", output.Orig, "data")
	}

	if output.A.B.C["Error"] != errorStatesTaskFailed {
		t.Fatalf("injected error output at $.a.b.c: got %q, want %q", output.A.B.C["Error"], errorStatesTaskFailed)
	}
}

func TestTaskCatchDoesNotMatchStatesRuntimeEvenWithStatesAll(t *testing.T) {
	t.Parallel()

	// An unsupported Resource format fails with a plain engine error, which
	// executionErrorCode reports as States.Runtime -- never caught, even by
	// the States.ALL wildcard.
	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "not-a-supported-resource",
				"Catch": [{"ErrorEquals": ["States.ALL"], "Next": "Fallback"}],
				"End": true
			},
			"Fallback": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "catch-runtime-uncaught", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{}`)

	if exec.Error != errorStatesRuntime {
		t.Fatalf("execution error: got %q, want %q (States.ALL must not catch States.Runtime)", exec.Error, errorStatesRuntime)
	}
}
