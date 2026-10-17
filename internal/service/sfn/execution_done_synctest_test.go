package sfn

import (
	"context"
	"encoding/json"
	"testing"
	"testing/synctest"
	"time"
)

func TestStartExecutionSyncCompletesWithoutPollingDelay(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStorage()

		child := createExecutionTestStateMachine(t, store, "done-child", holdDefinition)
		parentDef := startExecutionParentDefinition(resourceStartExecutionSync, child.StateMachineArn, `{}`)
		parent := createExecutionTestStateMachine(t, store, "done-parent", parentDef)

		started, err := store.StartExecution(context.Background(), parent.StateMachineArn, "", `{}`, "")
		if err != nil {
			t.Fatalf("StartExecution: %v", err)
		}

		exec := awaitExecutionTerminal(t, store, started.ExecutionArn)
		if exec.Status != ExecutionStatusSucceeded {
			t.Fatalf("parent: got status %q (%s: %s), want SUCCEEDED", exec.Status, exec.Error, exec.Cause)
		}

		// A polling waiter would finish up to one poll interval after the child.
		if got, want := executionElapsed(t, exec), holdSeconds*time.Second; got != want {
			t.Fatalf("parent duration: got %v, want exactly the child's %v", got, want)
		}
	})
}

func TestExecutionDoneObservesStoppedExecution(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStorage()
		started := startBubbleExecution(t, store, holdDefinition)

		done, err := store.executionDone(started.ExecutionArn)
		if err != nil {
			t.Fatalf("executionDone: %v", err)
		}

		synctest.Wait()

		select {
		case <-done:
			t.Fatal("done channel closed while the execution is RUNNING")
		default:
		}

		if _, err := store.StopExecution(context.Background(), started.ExecutionArn, "", ""); err != nil {
			t.Fatalf("StopExecution: %v", err)
		}

		select {
		case <-done:
		default:
			t.Fatal("done channel not closed after StopExecution")
		}

		exec, err := store.DescribeExecution(context.Background(), started.ExecutionArn)
		if err != nil {
			t.Fatalf("DescribeExecution: %v", err)
		}

		if exec.Status != ExecutionStatusAborted {
			t.Fatalf("status: got %q, want ABORTED", exec.Status)
		}
	})
}

func TestStartExecutionSyncReportsAbortedChild(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStorage()

		child := createExecutionTestStateMachine(t, store, "abort-child", holdDefinition)
		parentDef := startExecutionParentDefinition(resourceStartExecutionSync, child.StateMachineArn, `{}`)
		parent := createExecutionTestStateMachine(t, store, "abort-parent", parentDef)

		started, err := store.StartExecution(context.Background(), parent.StateMachineArn, "", `{}`, "")
		if err != nil {
			t.Fatalf("StartExecution: %v", err)
		}

		synctest.Wait()

		executions, _, err := store.ListExecutions(context.Background(), child.StateMachineArn, "", 0, "")
		if err != nil || len(executions) != 1 {
			t.Fatalf("ListExecutions: got %d executions, err %v, want exactly the child", len(executions), err)
		}

		if _, err := store.StopExecution(context.Background(), executions[0].ExecutionArn, "Operator", "stop child"); err != nil {
			t.Fatalf("StopExecution: %v", err)
		}

		exec := awaitExecutionTerminal(t, store, started.ExecutionArn)
		if exec.Status != ExecutionStatusFailed {
			t.Fatalf("parent: got status %q, want FAILED because the child was ABORTED", exec.Status)
		}
	})
}

func TestExecutionDoneForRestoredExecutions(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "restored-sm", holdDefinition)
	running := &Execution{ExecutionArn: "arn:running", StateMachineArn: sm.StateMachineArn, Status: ExecutionStatusRunning}
	finished := &Execution{ExecutionArn: "arn:finished", StateMachineArn: sm.StateMachineArn, Status: ExecutionStatusSucceeded}

	store.Executions[running.ExecutionArn] = &ExecutionData{Execution: running}
	store.Executions[finished.ExecutionArn] = &ExecutionData{Execution: finished}

	data, err := json.Marshal(store)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	restored := NewMemoryStorage()
	if err := json.Unmarshal(data, restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	runningDone, err := restored.executionDone(running.ExecutionArn)
	if err != nil {
		t.Fatalf("executionDone(running): %v", err)
	}

	select {
	case <-runningDone:
		t.Fatal("restored RUNNING execution reported done")
	default:
	}

	finishedDone, err := restored.executionDone(finished.ExecutionArn)
	if err != nil {
		t.Fatalf("executionDone(finished): %v", err)
	}

	select {
	case <-finishedDone:
	default:
		t.Fatal("restored terminal execution not reported done")
	}

	if _, err := restored.StopExecution(context.Background(), running.ExecutionArn, "", ""); err != nil {
		t.Fatalf("StopExecution on restored execution: %v", err)
	}

	select {
	case <-runningDone:
	default:
		t.Fatal("stopping a restored execution did not close its done channel")
	}
}

func TestExecutionDoneUnknownExecution(t *testing.T) {
	t.Parallel()

	if _, err := NewMemoryStorage().executionDone("arn:missing"); err == nil {
		t.Fatal("executionDone for an unknown execution: want error")
	}
}
