package sfn

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

// epsilon is the smallest virtual-time step: sleeping deadline-epsilon stays
// strictly before a deadline, and 2*epsilon crosses it.
const epsilon = time.Nanosecond

func TestAwaitTaskToken_TimeoutBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		taskFields string
		want       time.Duration
	}{
		{name: "heartbeat only", taskFields: `"HeartbeatSeconds": 10,`, want: 10 * time.Second},
		{name: "timeout only", taskFields: `"TimeoutSeconds": 30,`, want: 30 * time.Second},
		{name: "timeout before heartbeat", taskFields: `"HeartbeatSeconds": 60, "TimeoutSeconds": 20,`, want: 20 * time.Second},
		{name: "heartbeat before timeout", taskFields: `"HeartbeatSeconds": 5, "TimeoutSeconds": 60,`, want: 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				store, activityArn := newActivityTestStore(t)
				started := startBubbleExecution(t, store, activityTaskDefinition(activityArn, tt.taskFields))
				token := pollActivityToken(t, store, activityArn)

				time.Sleep(tt.want - epsilon)

				if exec := describeAfterSettle(t, store, started.ExecutionArn); exec.Status != ExecutionStatusRunning {
					t.Fatalf("status just before the deadline: got %q, want RUNNING", exec.Status)
				}

				time.Sleep(2 * epsilon)

				exec := describeAfterSettle(t, store, started.ExecutionArn)
				if exec.Status != ExecutionStatusFailed || exec.Error != errorStatesTimeout {
					t.Fatalf("status just after the deadline: got %q error %q, want FAILED %q", exec.Status, exec.Error, errorStatesTimeout)
				}

				if got := executionElapsed(t, exec); got != tt.want {
					t.Fatalf("time until States.Timeout: got %v, want %v", got, tt.want)
				}

				requireTaskTimedOut(t, store, token)
			})
		})
	}
}

func TestAwaitTaskToken_HeartbeatExtendsDeadline(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const heartbeat = 10 * time.Second

		store, activityArn := newActivityTestStore(t)
		started := startBubbleExecution(t, store, activityTaskDefinition(activityArn, `"HeartbeatSeconds": 10,`))
		token := pollActivityToken(t, store, activityArn)
		start := time.Now()

		// Three heartbeats just inside each window keep the task alive for
		// three times HeartbeatSeconds.
		for range 3 {
			time.Sleep(heartbeat - epsilon)

			if err := store.SendTaskHeartbeat(context.Background(), token); err != nil {
				t.Fatalf("SendTaskHeartbeat: %v", err)
			}

			if exec := describeAfterSettle(t, store, started.ExecutionArn); exec.Status != ExecutionStatusRunning {
				t.Fatalf("status after heartbeat at %v: got %q, want RUNNING", time.Since(start), exec.Status)
			}
		}

		if time.Since(start) <= heartbeat {
			t.Fatalf("elapsed %v does not exceed HeartbeatSeconds; the heartbeats proved nothing", time.Since(start))
		}

		if err := store.SendTaskSuccess(context.Background(), token, `{"done":true}`); err != nil {
			t.Fatalf("SendTaskSuccess: %v", err)
		}

		exec := awaitExecutionTerminal(t, store, started.ExecutionArn)
		if exec.Status != ExecutionStatusSucceeded || exec.Output != `{"done":true}` {
			t.Fatalf("execution: got status %q output %q (error %s), want SUCCEEDED %q", exec.Status, exec.Output, exec.Error, `{"done":true}`)
		}
	})
}

func TestAwaitTaskToken_DeadlineRestartsFromLastHeartbeat(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const (
			heartbeat       = 10 * time.Second
			firstBeatAt     = 9 * time.Second
			wantTimeoutTime = firstBeatAt + heartbeat
		)

		store, activityArn := newActivityTestStore(t)
		started := startBubbleExecution(t, store, activityTaskDefinition(activityArn, `"HeartbeatSeconds": 10,`))
		token := pollActivityToken(t, store, activityArn)

		time.Sleep(firstBeatAt)

		if err := store.SendTaskHeartbeat(context.Background(), token); err != nil {
			t.Fatalf("SendTaskHeartbeat: %v", err)
		}

		time.Sleep(heartbeat - epsilon)

		if exec := describeAfterSettle(t, store, started.ExecutionArn); exec.Status != ExecutionStatusRunning {
			t.Fatalf("status just before the extended deadline: got %q, want RUNNING", exec.Status)
		}

		time.Sleep(2 * epsilon)

		exec := describeAfterSettle(t, store, started.ExecutionArn)
		if exec.Status != ExecutionStatusFailed || exec.Error != errorStatesTimeout {
			t.Fatalf("status just after the extended deadline: got %q error %q, want FAILED %q", exec.Status, exec.Error, errorStatesTimeout)
		}

		if got := executionElapsed(t, exec); got != wantTimeoutTime {
			t.Fatalf("time until States.Timeout: got %v, want %v", got, wantTimeoutTime)
		}
	})
}

func TestAwaitTaskToken_HeartbeatsDoNotExtendTimeoutSeconds(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const (
			beatEvery = 8 * time.Second
			timeout   = 25 * time.Second
		)

		store, activityArn := newActivityTestStore(t)
		started := startBubbleExecution(t, store, activityTaskDefinition(activityArn, `"HeartbeatSeconds": 10, "TimeoutSeconds": 25,`))
		token := pollActivityToken(t, store, activityArn)

		// Beats at 8s, 16s and 24s each land within HeartbeatSeconds, so only
		// TimeoutSeconds can end the task.
		for range 3 {
			time.Sleep(beatEvery)

			if err := store.SendTaskHeartbeat(context.Background(), token); err != nil {
				t.Fatalf("SendTaskHeartbeat: %v", err)
			}
		}

		time.Sleep(timeout - 3*beatEvery - epsilon)

		if exec := describeAfterSettle(t, store, started.ExecutionArn); exec.Status != ExecutionStatusRunning {
			t.Fatalf("status just before TimeoutSeconds: got %q, want RUNNING", exec.Status)
		}

		time.Sleep(2 * epsilon)

		exec := describeAfterSettle(t, store, started.ExecutionArn)
		if exec.Status != ExecutionStatusFailed || exec.Error != errorStatesTimeout {
			t.Fatalf("status just after TimeoutSeconds: got %q error %q, want FAILED %q", exec.Status, exec.Error, errorStatesTimeout)
		}

		if got := executionElapsed(t, exec); got != timeout {
			t.Fatalf("time until States.Timeout: got %v, want %v", got, timeout)
		}

		requireTaskTimedOut(t, store, token)
	})
}
