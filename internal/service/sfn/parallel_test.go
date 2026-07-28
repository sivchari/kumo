package sfn

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParallelStateCollectsBranchOutputsInOrder(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Fork",
		"States": {
			"Fork": {
				"Type": "Parallel",
				"Branches": [
					{"StartAt": "A", "States": {"A": {"Type": "Pass", "Result": {"branch": "a"}, "End": true}}},
					{"StartAt": "B", "States": {"B": {"Type": "Pass", "Result": {"branch": "b"}, "End": true}}}
				],
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "parallel-ordered", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{}`)

	var outputs []map[string]string
	if err := json.Unmarshal([]byte(exec.Output), &outputs); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if len(outputs) != 2 || outputs[0]["branch"] != "a" || outputs[1]["branch"] != "b" {
		t.Fatalf("branch outputs: got %v, want [{branch:a} {branch:b}] in that order", outputs)
	}
}

func TestParallelStateFailsWhenABranchFails(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Fork",
		"States": {
			"Fork": {
				"Type": "Parallel",
				"Branches": [
					{"StartAt": "A", "States": {"A": {"Type": "Pass", "Result": {"branch": "a"}, "End": true}}},
					{"StartAt": "B", "States": {"B": {"Type": "Fail", "Error": "BranchBoom", "Cause": "branch b failed"}}}
				],
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "parallel-branch-fails", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{}`)

	if exec.Error != errorStatesBranchFailed {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesBranchFailed)
	}

	if !strings.Contains(exec.Cause, "BranchBoom") {
		t.Fatalf("execution cause %q must carry the branch failure", exec.Cause)
	}
}

func TestParallelStateCatchRecoversFromBranchFailure(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Fork",
		"States": {
			"Fork": {
				"Type": "Parallel",
				"Branches": [
					{"StartAt": "B", "States": {"B": {"Type": "Fail", "Error": "BranchBoom", "Cause": "branch b failed"}}}
				],
				"Catch": [{"ErrorEquals": ["States.BranchFailed"], "Next": "Fallback"}],
				"End": true
			},
			"Fallback": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "parallel-catch-recovers", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{}`)

	var errorOutput map[string]string
	if err := json.Unmarshal([]byte(exec.Output), &errorOutput); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if errorOutput["Error"] != errorStatesBranchFailed {
		t.Fatalf("caught error output Error: got %q, want %q", errorOutput["Error"], errorStatesBranchFailed)
	}

	if !strings.Contains(errorOutput["Cause"], "BranchBoom") {
		t.Fatalf("caught error output Cause %q must carry the branch failure", errorOutput["Cause"])
	}
}
