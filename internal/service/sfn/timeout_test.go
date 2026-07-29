package sfn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// slowLambdaSleep only needs to clear the 1-second TimeoutSeconds used by
// every test in this file by a small margin: net/http servers do not
// reliably observe a POST request's r.Context() cancellation once the
// client gives up (the connection isn't torn down until the handler itself
// returns), so httptest.Server.Close ends up waiting out this full duration
// regardless of how early the client-side call returns.
const slowLambdaSleep = 1200 * time.Millisecond

// newSlowLambdaServer returns an httptest server standing in for kumo's
// Lambda invoke endpoint that never responds in time for a Task state's
// TimeoutSeconds to matter.
func newSlowLambdaServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(slowLambdaSleep)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	t.Cleanup(server.Close)

	return server
}

func TestTaskTimeoutSecondsFailsExecutionWithStatesTimeout(t *testing.T) {
	t.Parallel()

	server := newSlowLambdaServer(t)

	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:slow-fn",
				"TimeoutSeconds": 1,
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "task-timeout", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{}`)

	if exec.Error != errorStatesTimeout {
		t.Fatalf("execution error: got %q, want %q (cause: %s)", exec.Error, errorStatesTimeout, exec.Cause)
	}
}

func TestTaskTimeoutSecondsPathFailsExecutionWithStatesTimeout(t *testing.T) {
	t.Parallel()

	server := newSlowLambdaServer(t)

	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:slow-fn",
				"TimeoutSecondsPath": "$.timeout",
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "task-timeout-path", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{"timeout":1}`)

	if exec.Error != errorStatesTimeout {
		t.Fatalf("execution error: got %q, want %q (cause: %s)", exec.Error, errorStatesTimeout, exec.Cause)
	}
}

func TestTaskTimeoutCaughtByCatchRecoversExecution(t *testing.T) {
	t.Parallel()

	server := newSlowLambdaServer(t)

	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:slow-fn",
				"TimeoutSeconds": 1,
				"Catch": [{"ErrorEquals": ["States.Timeout"], "Next": "Fallback"}],
				"End": true
			},
			"Fallback": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "task-timeout-catch", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{}`)

	var errorOutput map[string]string
	if err := json.Unmarshal([]byte(exec.Output), &errorOutput); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if errorOutput["Error"] != errorStatesTimeout {
		t.Fatalf("caught error output Error: got %q, want %q", errorOutput["Error"], errorStatesTimeout)
	}
}

func TestDefinitionTimeoutSecondsFailsExecutionWithStatesTimeout(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "LongWait",
		"TimeoutSeconds": 1,
		"States": {
			"LongWait": {"Type": "Wait", "Seconds": 5, "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "definition-timeout", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{}`)

	if exec.Error != errorStatesTimeout {
		t.Fatalf("execution error: got %q, want %q (cause: %s)", exec.Error, errorStatesTimeout, exec.Cause)
	}
}

func TestHeartbeatFieldsAreAcceptedWithoutError(t *testing.T) {
	t.Parallel()

	server := newEchoLambdaServer(t)

	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:echo-fn",
				"HeartbeatSeconds": 5,
				"HeartbeatSecondsPath": "$.heartbeat",
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "heartbeat-ignored", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"heartbeat":5}`)

	if exec.Output != `{"heartbeat":5}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"heartbeat":5}`)
	}
}
