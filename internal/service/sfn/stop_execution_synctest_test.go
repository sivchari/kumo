package sfn

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

const holdSeconds = 60

// holdDefinition builds a state machine that waits holdSeconds and succeeds.
const holdDefinition = `{
	"StartAt": "Hold",
	"States": {"Hold": {"Type": "Wait", "Seconds": 60, "End": true}}
}`

func countTerminalEvents(t *testing.T, store *MemoryStorage, executionArn string) int {
	t.Helper()

	events, _, err := store.GetExecutionHistory(context.Background(), executionArn, 0, "", false)
	if err != nil {
		t.Fatalf("GetExecutionHistory: %v", err)
	}

	count := 0

	for _, ev := range events {
		if ev.Type == HistoryEventTypeExecutionSucceeded || ev.Type == HistoryEventTypeExecutionFailed || ev.Type == HistoryEventTypeExecutionAborted {
			count++
		}
	}

	return count
}

func TestStopExecutionIsNotOverwrittenByRunningGoroutine(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStorage()
		started := startBubbleExecution(t, store, holdDefinition)

		synctest.Wait()
		time.Sleep(10 * time.Second)

		stopped, err := store.StopExecution(context.Background(), started.ExecutionArn, "Operator", "stopped by test")
		if err != nil {
			t.Fatalf("StopExecution: %v", err)
		}

		if stopped.Status != ExecutionStatusAborted {
			t.Fatalf("StopExecution status: got %q, want ABORTED", stopped.Status)
		}

		// Well past the point where the Wait would have completed.
		time.Sleep(2 * holdSeconds * time.Second)

		exec := describeAfterSettle(t, store, started.ExecutionArn)
		if exec.Status != ExecutionStatusAborted {
			t.Fatalf("status after goroutine would have finished: got %q, want ABORTED", exec.Status)
		}

		if exec.Error != "Operator" || exec.Cause != "stopped by test" {
			t.Fatalf("error/cause: got %q/%q, want the StopExecution values", exec.Error, exec.Cause)
		}

		if got := countTerminalEvents(t, store, started.ExecutionArn); got != 1 {
			t.Fatalf("terminal history events: got %d, want 1", got)
		}
	})
}
