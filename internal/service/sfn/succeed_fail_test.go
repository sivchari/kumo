package sfn

import "testing"

func TestSucceedStateTerminatesWithInputAsOutput(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Done",
		"States": {
			"Done": {"Type": "Succeed"}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "succeed-passthrough", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"result":"ok"}`)

	if exec.Output != `{"result":"ok"}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"result":"ok"}`)
	}
}

func TestFailStateReportsCustomErrorAndCause(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Boom",
		"States": {
			"Boom": {"Type": "Fail", "Error": "CustomError", "Cause": "something went wrong"}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "fail-custom", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{}`)

	if exec.Error != "CustomError" {
		t.Fatalf("execution error: got %q, want %q", exec.Error, "CustomError")
	}

	if exec.Cause != "something went wrong" {
		t.Fatalf("execution cause: got %q, want %q", exec.Cause, "something went wrong")
	}
}
