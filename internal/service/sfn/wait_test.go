package sfn

import "testing"

func TestWaitStateSecondsZeroCompletesAndPassesInputThrough(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Wait",
		"States": {
			"Wait": {"Type": "Wait", "Seconds": 0, "Next": "Done"},
			"Done": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "wait-zero", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"value":1}`)

	if exec.Output != `{"value":1}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"value":1}`)
	}
}

func TestWaitStateSecondsOneCompletesAndPassesInputThrough(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Wait",
		"States": {
			"Wait": {"Type": "Wait", "Seconds": 1, "Next": "Done"},
			"Done": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "wait-one", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"value":1}`)

	if exec.Output != `{"value":1}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"value":1}`)
	}
}

func TestWaitDurationRequiresOneField(t *testing.T) {
	t.Parallel()

	_, err := waitDuration(&stateDefinition{Type: stateTypeWait}, "{}")
	if err == nil {
		t.Fatal("waitDuration: want error when no wait field is set, got nil")
	}
}

func TestWaitDurationSecondsPath(t *testing.T) {
	t.Parallel()

	state := &stateDefinition{Type: stateTypeWait, SecondsPath: "$.waitSeconds"}

	d, err := waitDuration(state, `{"waitSeconds": 0}`)
	if err != nil {
		t.Fatalf("waitDuration: %v", err)
	}

	if d != 0 {
		t.Fatalf("waitDuration() = %v, want 0", d)
	}
}
