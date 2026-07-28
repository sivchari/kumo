package sfn

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// newFlakyLambdaServer returns an httptest server standing in for kumo's
// Lambda invoke endpoint: it fails the first failCount calls with a 500
// status (so the Task state's callLambda reports an error) and succeeds on
// every call after that. The returned counter tracks the number of calls
// received.
func newFlakyLambdaServer(t *testing.T, failCount int) (*httptest.Server, *int32) {
	t.Helper()

	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if int(n) <= failCount {
			http.Error(w, "boom", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	t.Cleanup(server.Close)

	return server, &attempts
}

func retryTaskDefinition(retryJSON string) string {
	return fmt.Sprintf(`{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:flaky-fn",
				"Retry": [%s],
				"End": true
			}
		}
	}`, retryJSON)
}

func TestTaskRetrySucceedsOnAttemptAfterMatchingFailures(t *testing.T) {
	t.Parallel()

	// The endpoint fails once, so a single retry (IntervalSeconds: 1)
	// resolves it on the second attempt.
	server, attempts := newFlakyLambdaServer(t, 1)

	definition := retryTaskDefinition(`{"ErrorEquals": ["States.TaskFailed"], "IntervalSeconds": 1, "MaxAttempts": 1}`)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "retry-succeeds", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{}`)

	if exec.Output != `{"ok":true}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"ok":true}`)
	}

	if got := atomic.LoadInt32(attempts); got != 2 {
		t.Fatalf("lambda invocation count: got %d, want 2 (1 failure + 1 successful retry)", got)
	}
}

func TestTaskRetryExhaustionFailsWithOriginalError(t *testing.T) {
	t.Parallel()

	// The endpoint always fails; MaxAttempts: 1 allows exactly one retry
	// before the interpreter gives up and fails the state.
	server, attempts := newFlakyLambdaServer(t, 1<<30)

	definition := retryTaskDefinition(`{"ErrorEquals": ["States.TaskFailed"], "IntervalSeconds": 1, "MaxAttempts": 1}`)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "retry-exhausted", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{}`)

	if exec.Error != errorStatesTaskFailed {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesTaskFailed)
	}

	if got := atomic.LoadInt32(attempts); got != 2 {
		t.Fatalf("lambda invocation count: got %d, want 2 (1 initial + 1 retry)", got)
	}
}

func TestTaskRetryErrorEqualsMismatchDoesNotRetry(t *testing.T) {
	t.Parallel()

	// The Retrier only matches "SomeOtherError", never the States.TaskFailed
	// the endpoint actually produces, so the state must fail immediately
	// without consuming any retry attempts.
	server, attempts := newFlakyLambdaServer(t, 1<<30)

	definition := retryTaskDefinition(`{"ErrorEquals": ["SomeOtherError"], "IntervalSeconds": 1, "MaxAttempts": 3}`)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "retry-mismatch", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{}`)

	if exec.Error != errorStatesTaskFailed {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesTaskFailed)
	}

	if got := atomic.LoadInt32(attempts); got != 1 {
		t.Fatalf("lambda invocation count: got %d, want 1 (no retry attempted)", got)
	}
}
