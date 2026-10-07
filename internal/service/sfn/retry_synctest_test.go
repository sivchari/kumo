package sfn

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"time"
)

type retryBackoffCase struct {
	name  string
	retry string
	// delays are the waits before each retry; the task is attempted
	// len(delays)+1 times.
	delays []time.Duration
}

var retryBackoffCases = []retryBackoffCase{
	{
		name:   "spec defaults",
		retry:  `{"ErrorEquals": ["States.ALL"]}`,
		delays: []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second},
	},
	{
		name:   "backoff rate 3",
		retry:  `{"ErrorEquals": ["States.ALL"], "IntervalSeconds": 2, "BackoffRate": 3, "MaxAttempts": 4}`,
		delays: []time.Duration{2 * time.Second, 6 * time.Second, 18 * time.Second, 54 * time.Second},
	},
	{
		name:   "max delay caps growth",
		retry:  `{"ErrorEquals": ["States.ALL"], "IntervalSeconds": 2, "BackoffRate": 2, "MaxAttempts": 5, "MaxDelaySeconds": 10}`,
		delays: []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 10 * time.Second, 10 * time.Second},
	},
	{
		name:   "fractional backoff rate",
		retry:  `{"ErrorEquals": ["States.ALL"], "IntervalSeconds": 2, "BackoffRate": 1.5, "MaxAttempts": 3}`,
		delays: []time.Duration{2 * time.Second, 3 * time.Second, 4500 * time.Millisecond},
	},
	{
		name:   "backoff rate 1 keeps the interval",
		retry:  `{"ErrorEquals": ["States.ALL"], "IntervalSeconds": 5, "BackoffRate": 1, "MaxAttempts": 3}`,
		delays: []time.Duration{5 * time.Second, 5 * time.Second, 5 * time.Second},
	},
}

func TestRunWithRetry_BackoffSchedule(t *testing.T) {
	t.Parallel()

	for _, tt := range retryBackoffCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				runRetryBackoffCase(t, tt)
			})
		})
	}
}

// runRetryBackoffCase fails every activity attempt inside the current
// synctest bubble and checks when each retry is scheduled.
func runRetryBackoffCase(t *testing.T, tt retryBackoffCase) {
	t.Helper()

	store, activityArn := newActivityTestStore(t)
	started := startBubbleExecution(t, store, activityTaskDefinition(activityArn, fmt.Sprintf(`"Retry": [%s],`, tt.retry)))
	start := time.Now()

	var wantElapsed time.Duration

	for attempt := 0; attempt <= len(tt.delays); attempt++ {
		if attempt > 0 {
			wantElapsed += tt.delays[attempt-1]
		}

		token := pollActivityToken(t, store, activityArn)

		if got := time.Since(start); got != wantElapsed {
			t.Fatalf("attempt %d scheduled at %v, want %v", attempt+1, got, wantElapsed)
		}

		if err := store.SendTaskFailure(context.Background(), token, "TransientError", "boom"); err != nil {
			t.Fatalf("SendTaskFailure (attempt %d): %v", attempt+1, err)
		}
	}

	exec := awaitExecutionTerminal(t, store, started.ExecutionArn)
	if exec.Status != ExecutionStatusFailed || exec.Error != "TransientError" {
		t.Fatalf("execution after retries exhausted: got status %q error %q, want FAILED TransientError", exec.Status, exec.Error)
	}

	// No wait follows the last failure: the budget is spent.
	if got := executionElapsed(t, exec); got != wantElapsed {
		t.Fatalf("execution duration: got %v, want %v", got, wantElapsed)
	}
}
