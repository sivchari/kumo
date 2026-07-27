package sfn

import (
	"context"
	"strings"
	"testing"
	"time"
)

const executionTestDefinition = `{
	"StartAt": "Done",
	"States": {
		"Done": {"Type": "Pass", "End": true}
	}
}`

func TestStartExecutionARNUsesLowercaseExecutionSegment(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "arn-casing", executionTestDefinition)

	exec, err := store.StartExecution(context.Background(), sm.StateMachineArn, "run1", "{}", "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	if !strings.Contains(exec.ExecutionArn, ":execution:") {
		t.Fatalf("execution ARN %q must use the lowercase :execution: segment", exec.ExecutionArn)
	}
}

func TestFailedExecutionReportsRuntimeErrorForUnsupportedState(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Decide",
		"States": {
			"Decide": {
				"Type": "Choice",
				"Choices": [{"Variable": "$.mode", "StringEquals": "fast", "Next": "Done"}],
				"Default": "Done"
			},
			"Done": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "runtime-error", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, "{}")

	if exec.Error != errorStatesRuntime {
		t.Fatalf("engine failure error code: got %q, want %q (cause: %s)", exec.Error, errorStatesRuntime, exec.Cause)
	}
}

func TestFailedExecutionReportsTaskFailedForTaskInvocationError(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "CallLambda",
		"States": {
			"CallLambda": {
				"Type": "Task",
				"Resource": "arn:aws:states:::lambda:invoke",
				"Parameters": {"FunctionName": "missing-function"},
				"End": true
			}
		}
	}`

	// The engine's base URL points at a closed port, so the task invocation
	// itself fails and must surface as States.TaskFailed.
	store := NewMemoryStorage(WithBaseURL("http://127.0.0.1:1"))
	sm := createExecutionTestStateMachine(t, store, "task-error", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, "{}")

	if exec.Error != errorStatesTaskFailed {
		t.Fatalf("task failure error code: got %q, want %q (cause: %s)", exec.Error, errorStatesTaskFailed, exec.Cause)
	}
}

func createExecutionTestStateMachine(t *testing.T, store *MemoryStorage, name, definition string) *StateMachine {
	t.Helper()

	sm, err := store.CreateStateMachine(context.Background(), &CreateStateMachineRequest{
		Name:       name,
		Definition: definition,
		RoleArn:    "arn:aws:iam::000000000000:role/sfn-test",
	})
	if err != nil {
		t.Fatalf("CreateStateMachine: %v", err)
	}

	return sm
}

func startAndAwaitFailure(t *testing.T, store *MemoryStorage, stateMachineArn, input string) *Execution {
	t.Helper()

	exec := startAndAwaitTerminal(t, store, stateMachineArn, input)

	if exec.Status != ExecutionStatusFailed {
		t.Fatalf("execution ended with status %q, want FAILED", exec.Status)
	}

	return exec
}

// startAndAwaitSuccess starts an execution and polls DescribeExecution until
// it reaches SUCCEEDED, failing the test if it instead fails or times out.
func startAndAwaitSuccess(t *testing.T, store *MemoryStorage, stateMachineArn, input string) *Execution {
	t.Helper()

	exec := startAndAwaitTerminal(t, store, stateMachineArn, input)

	if exec.Status != ExecutionStatusSucceeded {
		t.Fatalf("execution ended with status %q, want SUCCEEDED (error: %s, cause: %s)", exec.Status, exec.Error, exec.Cause)
	}

	return exec
}

// startAndAwaitTerminal starts an execution and polls DescribeExecution until
// it leaves RUNNING, failing the test if it does not terminate in time.
func startAndAwaitTerminal(t *testing.T, store *MemoryStorage, stateMachineArn, input string) *Execution {
	t.Helper()

	ctx := context.Background()

	started, err := store.StartExecution(ctx, stateMachineArn, "", input, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		exec, err := store.DescribeExecution(ctx, started.ExecutionArn)
		if err != nil {
			t.Fatalf("DescribeExecution: %v", err)
		}

		if exec.Status != ExecutionStatusRunning {
			return exec
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("execution did not terminate within the deadline")

	return nil
}
