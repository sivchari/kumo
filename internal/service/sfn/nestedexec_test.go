package sfn

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// childStateMachineArn computes the deterministic ARN CreateStateMachine
// assigns a state machine named name, matching storage.go's own
// "arn:aws:states:%s:%s:stateMachine:%s" format with NewMemoryStorage's
// default region/account -- so a test can reference a child (or its own)
// state machine's ARN in a Parameters.StateMachineArn literal before ever
// calling CreateStateMachine.
func childStateMachineArn(name string) string {
	return fmt.Sprintf("arn:aws:states:us-east-1:000000000000:stateMachine:%s", name)
}

// startExecutionParentDefinition builds a single-Task state machine whose
// Task uses resource (one of the arn:aws:states:::states:startExecution*
// variants) to start childArn.
func startExecutionParentDefinition(resource, childArn, inputJSON string) string {
	return fmt.Sprintf(`{
		"StartAt": "Nested",
		"States": {
			"Nested": {
				"Type": "Task",
				"Resource": %q,
				"Parameters": {"StateMachineArn": %q, "Input": %s},
				"End": true
			}
		}
	}`, resource, childArn, inputJSON)
}

func TestStartExecutionFireAndForgetOutputShapeAndDispatchesChild(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	child := createExecutionTestStateMachine(t, store, "nested-child-async", `{
		"StartAt": "Echo", "States": {"Echo": {"Type": "Pass", "End": true}}
	}`)

	parentDef := startExecutionParentDefinition(resourceStartExecutionBase, child.StateMachineArn, `{"greeting":"hi"}`)
	parent := createExecutionTestStateMachine(t, store, "nested-parent-async", parentDef)

	exec := startAndAwaitSuccess(t, store, parent.StateMachineArn, `{}`)

	var output struct {
		ExecutionArn string  `json:"executionArn"`
		StartDate    float64 `json:"startDate"`
	}

	if err := json.Unmarshal([]byte(exec.Output), &output); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if output.ExecutionArn == "" || !strings.Contains(output.ExecutionArn, ":execution:") {
		t.Fatalf("output.executionArn: got %q, want a non-empty execution ARN", output.ExecutionArn)
	}

	if output.StartDate == 0 {
		t.Fatalf("output.startDate: got 0, want a non-zero Unix timestamp")
	}

	// The fire-and-forget Task must not have waited for the child, but the
	// child must actually have been dispatched.
	childExec := waitForExecutionTerminal(t, store, output.ExecutionArn)
	if childExec.Status != ExecutionStatusSucceeded {
		t.Fatalf("child execution status: got %q, want SUCCEEDED", childExec.Status)
	}
}

func TestStartExecutionSyncWaitsAndReturnsOutputAsString(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	child := createExecutionTestStateMachine(t, store, "nested-child-sync", `{
		"StartAt": "Build",
		"States": {"Build": {"Type": "Pass", "Result": {"MyKey": "MyValue"}, "End": true}}
	}`)

	parentDef := startExecutionParentDefinition(resourceStartExecutionSync, child.StateMachineArn, `{}`)
	parent := createExecutionTestStateMachine(t, store, "nested-parent-sync", parentDef)

	exec := startAndAwaitSuccess(t, store, parent.StateMachineArn, `{}`)

	var output struct {
		Status string `json:"status"`
		Output string `json:"output"`
	}

	if err := json.Unmarshal([]byte(exec.Output), &output); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if output.Status != string(ExecutionStatusSucceeded) {
		t.Fatalf("output.status: got %q, want %q", output.Status, ExecutionStatusSucceeded)
	}

	if want := `{"MyKey":"MyValue"}`; output.Output != want {
		t.Fatalf(".sync output.output: got %q, want the child's own output as a JSON string %q", output.Output, want)
	}
}

func TestStartExecutionSyncV2ParsesOutputAsJSON(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	child := createExecutionTestStateMachine(t, store, "nested-child-syncv2", `{
		"StartAt": "Build",
		"States": {"Build": {"Type": "Pass", "Result": {"MyKey": "MyValue"}, "End": true}}
	}`)

	parentDef := startExecutionParentDefinition(resourceStartExecutionSync2, child.StateMachineArn, `{}`)
	parent := createExecutionTestStateMachine(t, store, "nested-parent-syncv2", parentDef)

	exec := startAndAwaitSuccess(t, store, parent.StateMachineArn, `{}`)

	var output struct {
		Output map[string]any `json:"output"`
	}

	if err := json.Unmarshal([]byte(exec.Output), &output); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if output.Output["MyKey"] != "MyValue" {
		t.Fatalf(".sync:2 output.output: got %v, want parsed JSON {\"MyKey\":\"MyValue\"}", output.Output)
	}
}

func TestStartExecutionSyncChildFailureReportsTaskFailed(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	child := createExecutionTestStateMachine(t, store, "nested-child-fails", `{
		"StartAt": "Boom",
		"States": {"Boom": {"Type": "Fail", "Error": "ChildBroke", "Cause": "it broke"}}
	}`)

	parentDef := startExecutionParentDefinition(resourceStartExecutionSync, child.StateMachineArn, `{}`)
	parent := createExecutionTestStateMachine(t, store, "nested-parent-child-fails", parentDef)

	exec := startAndAwaitFailure(t, store, parent.StateMachineArn, `{}`)

	if exec.Error != errorStatesTaskFailed {
		t.Fatalf("execution error: got %q, want %q (cause: %s)", exec.Error, errorStatesTaskFailed, exec.Cause)
	}

	if !strings.Contains(exec.Cause, "ChildBroke") {
		t.Fatalf("execution cause: got %q, want it to mention the child's Error %q", exec.Cause, "ChildBroke")
	}
}

func TestStartExecutionWaitForTaskTokenSuffixIsRejected(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	child := createExecutionTestStateMachine(t, store, "nested-child-wftt", `{
		"StartAt": "Echo", "States": {"Echo": {"Type": "Pass", "End": true}}
	}`)

	parentDef := startExecutionParentDefinition(resourceStartExecutionBase+callbackResourceSuffix, child.StateMachineArn, `{}`)
	parent := createExecutionTestStateMachine(t, store, "nested-parent-wftt", parentDef)

	exec := startAndAwaitFailure(t, store, parent.StateMachineArn, `{}`)

	if exec.Error != errorStatesTaskFailed {
		t.Fatalf("execution error: got %q, want %q (cause: %s)", exec.Error, errorStatesTaskFailed, exec.Cause)
	}

	if !strings.Contains(exec.Cause, callbackResourceSuffix) {
		t.Fatalf("execution cause: got %q, want it to mention %q as unsupported", exec.Cause, callbackResourceSuffix)
	}
}

func TestStartExecutionMissingStateMachineArnFailsCleanly(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Nested",
		"States": {
			"Nested": {
				"Type": "Task",
				"Resource": "arn:aws:states:::states:startExecution",
				"Parameters": {},
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "nested-missing-arn", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{}`)

	if exec.Error != errorStatesTaskFailed {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesTaskFailed)
	}

	if !strings.Contains(exec.Cause, "StateMachineArn") {
		t.Fatalf("execution cause: got %q, want it to mention the missing StateMachineArn", exec.Cause)
	}
}

// TestStartExecutionSelfRecursionHitsDepthCap builds a state machine whose
// only Task calls states:startExecution.sync against its own ARN, so every
// level waits for the next: the innermost execution to actually run (at
// nestedExecutionMaxDepth) refuses to recurse further and fails, and that
// failure -- via each level's own States.TaskFailed -- eventually
// propagates all the way back up to the top-level execution.
func TestStartExecutionSelfRecursionHitsDepthCap(t *testing.T) {
	t.Parallel()

	const name = "self-recursing"

	selfArn := childStateMachineArn(name)
	definition := startExecutionParentDefinition(resourceStartExecutionSync, selfArn, `{}`)

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, name, definition)

	exec := startAndAwaitFailureWithin(t, store, sm.StateMachineArn, `{}`, 15*time.Second)

	if exec.Error != errorStatesTaskFailed {
		t.Fatalf("execution error: got %q, want %q (cause: %s)", exec.Error, errorStatesTaskFailed, exec.Cause)
	}

	if !strings.Contains(exec.Cause, "nesting depth exceeded") {
		t.Fatalf("execution cause: got %q, want it to mention kumo's nesting depth cap", exec.Cause)
	}
}

// waitForExecutionTerminal polls DescribeExecution for executionArn until it
// leaves RUNNING, failing the test if it does not terminate in time. Unlike
// startAndAwaitTerminal (execution_error_test.go), it takes an already
// existing execution ARN rather than starting a new execution itself.
func waitForExecutionTerminal(t *testing.T, store *MemoryStorage, executionArn string) *Execution {
	t.Helper()

	ctx := context.Background()
	deadline := time.Now().Add(10 * time.Second)

	for time.Now().Before(deadline) {
		exec, err := store.DescribeExecution(ctx, executionArn)
		if err != nil {
			t.Fatalf("DescribeExecution: %v", err)
		}

		if exec.Status != ExecutionStatusRunning {
			return exec
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("execution %q did not terminate within the deadline", executionArn)

	return nil
}

// startAndAwaitFailureWithin is startAndAwaitFailure with a caller-supplied
// deadline, for tests (like deep self-recursion) that need more time than
// startAndAwaitTerminal's fixed 10 seconds.
func startAndAwaitFailureWithin(t *testing.T, store *MemoryStorage, stateMachineArn, input string, timeout time.Duration) *Execution {
	t.Helper()

	ctx := context.Background()

	started, err := store.StartExecution(ctx, stateMachineArn, "", input, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		exec, err := store.DescribeExecution(ctx, started.ExecutionArn)
		if err != nil {
			t.Fatalf("DescribeExecution: %v", err)
		}

		if exec.Status == ExecutionStatusFailed {
			return exec
		}

		if exec.Status != ExecutionStatusRunning {
			t.Fatalf("execution ended with status %q, want FAILED", exec.Status)
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("execution did not terminate within the deadline")

	return nil
}
