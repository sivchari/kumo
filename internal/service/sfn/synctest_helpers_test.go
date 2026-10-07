package sfn

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"
)

// The helpers below must be called from inside a synctest bubble: they rely
// on its virtual clock, and executions started there run their goroutine in
// the same bubble.

// bubbleAwaitLimit bounds awaitExecutionTerminal in virtual time so a stuck
// execution fails the test instead of spinning the fake clock forever.
const bubbleAwaitLimit = time.Hour

// newActivityTestStore creates a storage with one registered activity whose
// long poll outlasts every virtual wait the synctest tests schedule.
func newActivityTestStore(t *testing.T) (store *MemoryStorage, activityArn string) {
	t.Helper()

	store = NewMemoryStorage()
	store.engine.activityPollTimeout = bubbleAwaitLimit

	activity, err := store.CreateActivity(context.Background(), "synctest-activity", nil)
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}

	return store, activity.ActivityArn
}

// activityTaskDefinition builds a one-state machine whose Task runs on the
// activity, with taskFields (e.g. `"HeartbeatSeconds": 10,`) spliced in.
func activityTaskDefinition(activityArn, taskFields string) string {
	return fmt.Sprintf(`{
		"StartAt": "Work",
		"States": {
			"Work": {
				"Type": "Task",
				"Resource": %q,
				%s
				"End": true
			}
		}
	}`, activityArn, taskFields)
}

// startBubbleExecution creates a state machine and starts an execution of
// it with an empty JSON object as input.
func startBubbleExecution(t *testing.T, store *MemoryStorage, definition string) *Execution {
	t.Helper()

	sm := createExecutionTestStateMachine(t, store, "synctest-sm", definition)

	started, err := store.StartExecution(context.Background(), sm.StateMachineArn, "", `{}`, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	return started
}

// pollActivityToken blocks until a task is scheduled on activityArn and
// returns its task token.
func pollActivityToken(t *testing.T, store *MemoryStorage, activityArn string) string {
	t.Helper()

	token, _, err := store.GetActivityTask(context.Background(), activityArn, "")
	if err != nil {
		t.Fatalf("GetActivityTask: %v", err)
	}

	if token == "" {
		t.Fatal("GetActivityTask returned no task token")
	}

	return token
}

// describeAfterSettle returns the execution's state once every goroutine in
// the bubble has blocked, so effects of timers that already fired are visible.
func describeAfterSettle(t *testing.T, store *MemoryStorage, executionArn string) *Execution {
	t.Helper()

	synctest.Wait()

	exec, err := store.DescribeExecution(context.Background(), executionArn)
	if err != nil {
		t.Fatalf("DescribeExecution: %v", err)
	}

	return exec
}

// awaitExecutionTerminal advances the virtual clock until the execution
// leaves RUNNING.
func awaitExecutionTerminal(t *testing.T, store *MemoryStorage, executionArn string) *Execution {
	t.Helper()

	deadline := time.Now().Add(bubbleAwaitLimit)

	for {
		exec := describeAfterSettle(t, store, executionArn)
		if exec.Status != ExecutionStatusRunning {
			return exec
		}

		if !time.Now().Before(deadline) {
			t.Fatalf("execution still RUNNING after %s of virtual time", bubbleAwaitLimit)
		}

		time.Sleep(time.Second)
	}
}

// executionElapsed returns the virtual time between an execution's start and stop.
func executionElapsed(t *testing.T, exec *Execution) time.Duration {
	t.Helper()

	if exec.StopDate == nil {
		t.Fatalf("execution %q has no StopDate (status %s)", exec.ExecutionArn, exec.Status)
	}

	return exec.StopDate.Sub(exec.StartDate)
}

// requireTaskTimedOut checks that every task token API rejects token as
// TaskTimedOut, i.e. the waiting state already released it.
func requireTaskTimedOut(t *testing.T, store *MemoryStorage, token string) {
	t.Helper()

	ctx := context.Background()

	calls := map[string]error{
		"SendTaskSuccess":   store.SendTaskSuccess(ctx, token, `{}`),
		"SendTaskFailure":   store.SendTaskFailure(ctx, token, "Late", "too late"),
		"SendTaskHeartbeat": store.SendTaskHeartbeat(ctx, token),
	}

	for name, err := range calls {
		var svcErr *ServiceError
		if !errors.As(err, &svcErr) || svcErr.Code != tokenErrTimedOut {
			t.Errorf("%s after release: got %v, want ServiceError code %q", name, err, tokenErrTimedOut)
		}
	}
}
