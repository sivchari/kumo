package sfn

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTaskTokenCapturingLambdaServer returns an httptest server standing in
// for kumo's Lambda invoke endpoint: it extracts the callback task token
// from the request payload's "taskToken" field and sends it on tokenCh,
// then responds with an empty object (a callback task's own integration
// response is discarded -- see fireCallbackIntegration).
func newTaskTokenCapturingLambdaServer(t *testing.T, tokenCh chan<- string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		// executeLambdaInvoke posts the resolved Payload map verbatim as the
		// request body (no outer envelope), so the token sits directly at
		// the top level here.
		var payload struct {
			TaskToken string `json:"taskToken"`
		}

		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		tokenCh <- payload.TaskToken

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))

	t.Cleanup(server.Close)

	return server
}

// recvTaskToken waits for a token on tokenCh, failing the test if none
// arrives within a bounded deadline.
func recvTaskToken(t *testing.T, tokenCh <-chan string) string {
	t.Helper()

	select {
	case token := <-tokenCh:
		if token == "" {
			t.Fatal("captured task token is empty")
		}

		return token
	case <-time.After(5 * time.Second):
		t.Fatal("did not receive a task token in time")

		return ""
	}
}

// pollExecutionUntilTerminal polls DescribeExecution for executionArn until
// it leaves RUNNING, failing the test if it does not terminate in time.
func pollExecutionUntilTerminal(t *testing.T, store *MemoryStorage, executionArn string) *Execution {
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

	t.Fatal("execution did not terminate within the deadline")

	return nil
}

const callbackTaskDefinition = `{
	"StartAt": "Notify",
	"States": {
		"Notify": {
			"Type": "Task",
			"Resource": "arn:aws:states:::lambda:invoke.waitForTaskToken",
			"Parameters": {
				"FunctionName": "notify-fn",
				"Payload": {"taskToken.$": "$$.Task.Token"}
			},
			"ResultPath": "$.callback",
			"End": true
		}
	}
}`

func TestCallbackTaskPausesUntilSendTaskSuccessThenFlowsThroughResultPath(t *testing.T) {
	t.Parallel()

	tokenCh := make(chan string, 1)
	server := newTaskTokenCapturingLambdaServer(t, tokenCh)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "callback-success", callbackTaskDefinition)

	started, err := store.StartExecution(context.Background(), sm.StateMachineArn, "", `{"x":1}`, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	token := recvTaskToken(t, tokenCh)

	// The execution must still be paused, waiting on the token.
	time.Sleep(50 * time.Millisecond)

	running, err := store.DescribeExecution(context.Background(), started.ExecutionArn)
	if err != nil {
		t.Fatalf("DescribeExecution: %v", err)
	}

	if running.Status != ExecutionStatusRunning {
		t.Fatalf("execution status before SendTaskSuccess: got %q, want RUNNING", running.Status)
	}

	if err := store.SendTaskSuccess(context.Background(), token, `{"ok":true}`); err != nil {
		t.Fatalf("SendTaskSuccess: %v", err)
	}

	exec := pollExecutionUntilTerminal(t, store, started.ExecutionArn)
	if exec.Status != ExecutionStatusSucceeded {
		t.Fatalf("execution status: got %q, want SUCCEEDED (error: %s, cause: %s)", exec.Status, exec.Error, exec.Cause)
	}

	var output map[string]any
	if err := json.Unmarshal([]byte(exec.Output), &output); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if output["callback"] == nil {
		t.Fatalf("execution output %q: missing \"callback\" from ResultPath", exec.Output)
	}
}

func TestCallbackTaskSendTaskFailureTriggersCatch(t *testing.T) {
	t.Parallel()

	tokenCh := make(chan string, 1)
	server := newTaskTokenCapturingLambdaServer(t, tokenCh)

	definition := `{
		"StartAt": "Notify",
		"States": {
			"Notify": {
				"Type": "Task",
				"Resource": "arn:aws:states:::lambda:invoke.waitForTaskToken",
				"Parameters": {"FunctionName": "notify-fn", "Payload": {"taskToken.$": "$$.Task.Token"}},
				"Catch": [{"ErrorEquals": ["States.ALL"], "Next": "Fallback"}],
				"End": true
			},
			"Fallback": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "callback-failure", definition)

	started, err := store.StartExecution(context.Background(), sm.StateMachineArn, "", `{}`, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	token := recvTaskToken(t, tokenCh)

	if err := store.SendTaskFailure(context.Background(), token, "CustomError", "custom cause"); err != nil {
		t.Fatalf("SendTaskFailure: %v", err)
	}

	exec := pollExecutionUntilTerminal(t, store, started.ExecutionArn)
	if exec.Status != ExecutionStatusSucceeded {
		t.Fatalf("execution status: got %q, want SUCCEEDED via Catch (error: %s, cause: %s)", exec.Status, exec.Error, exec.Cause)
	}

	var errorOutput map[string]string
	if err := json.Unmarshal([]byte(exec.Output), &errorOutput); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if errorOutput["Error"] != "CustomError" || errorOutput["Cause"] != "custom cause" {
		t.Fatalf("caught error output: got %+v, want Error=CustomError Cause=custom cause", errorOutput)
	}
}

func TestSendTaskSuccessUnknownTokenReportsTaskDoesNotExist(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	err := store.SendTaskSuccess(context.Background(), "does-not-exist", `{}`)

	var svcErr *ServiceError
	if !errors.As(err, &svcErr) || svcErr.Code != tokenErrDoesNotExist {
		t.Fatalf("SendTaskSuccess error: got %v, want ServiceError code %q", err, tokenErrDoesNotExist)
	}
}

func TestSendTaskSuccessAlreadyResolvedTokenReportsTaskTimedOut(t *testing.T) {
	t.Parallel()

	tokenCh := make(chan string, 1)
	server := newTaskTokenCapturingLambdaServer(t, tokenCh)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "callback-double-resolve", callbackTaskDefinition)

	_, err := store.StartExecution(context.Background(), sm.StateMachineArn, "", `{}`, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	token := recvTaskToken(t, tokenCh)

	if err := store.SendTaskSuccess(context.Background(), token, `{}`); err != nil {
		t.Fatalf("first SendTaskSuccess: %v", err)
	}

	err = store.SendTaskSuccess(context.Background(), token, `{}`)

	var svcErr *ServiceError
	if !errors.As(err, &svcErr) || svcErr.Code != tokenErrTimedOut {
		t.Fatalf("second SendTaskSuccess error: got %v, want ServiceError code %q", err, tokenErrTimedOut)
	}
}

func TestCallbackTaskTimeoutSecondsFailsExecutionWithStatesTimeout(t *testing.T) {
	t.Parallel()

	tokenCh := make(chan string, 1)
	server := newTaskTokenCapturingLambdaServer(t, tokenCh)

	definition := `{
		"StartAt": "Notify",
		"States": {
			"Notify": {
				"Type": "Task",
				"Resource": "arn:aws:states:::lambda:invoke.waitForTaskToken",
				"TimeoutSeconds": 1,
				"Parameters": {"FunctionName": "notify-fn", "Payload": {"taskToken.$": "$$.Task.Token"}},
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "callback-timeout", definition)

	started, err := store.StartExecution(context.Background(), sm.StateMachineArn, "", `{}`, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	recvTaskToken(t, tokenCh)

	exec := pollExecutionUntilTerminal(t, store, started.ExecutionArn)
	if exec.Status != ExecutionStatusFailed {
		t.Fatalf("execution status: got %q, want FAILED", exec.Status)
	}

	if exec.Error != errorStatesTimeout {
		t.Fatalf("execution error: got %q, want %q (cause: %s)", exec.Error, errorStatesTimeout, exec.Cause)
	}
}

func TestCallbackTaskHeartbeatExpiryFailsExecutionWithStatesTimeout(t *testing.T) {
	t.Parallel()

	tokenCh := make(chan string, 1)
	server := newTaskTokenCapturingLambdaServer(t, tokenCh)

	definition := `{
		"StartAt": "Notify",
		"States": {
			"Notify": {
				"Type": "Task",
				"Resource": "arn:aws:states:::lambda:invoke.waitForTaskToken",
				"HeartbeatSeconds": 1,
				"Parameters": {"FunctionName": "notify-fn", "Payload": {"taskToken.$": "$$.Task.Token"}},
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "callback-heartbeat-timeout", definition)

	started, err := store.StartExecution(context.Background(), sm.StateMachineArn, "", `{}`, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	recvTaskToken(t, tokenCh)

	// No SendTaskHeartbeat is ever sent, so the 1-second HeartbeatSeconds
	// interval must expire and fail the state.
	exec := pollExecutionUntilTerminal(t, store, started.ExecutionArn)
	if exec.Status != ExecutionStatusFailed {
		t.Fatalf("execution status: got %q, want FAILED", exec.Status)
	}

	if exec.Error != errorStatesTimeout {
		t.Fatalf("execution error: got %q, want %q (cause: %s)", exec.Error, errorStatesTimeout, exec.Cause)
	}
}

func TestCallbackTaskHeartbeatsKeepWaitAlivePastTheInterval(t *testing.T) {
	t.Parallel()

	tokenCh := make(chan string, 1)
	server := newTaskTokenCapturingLambdaServer(t, tokenCh)

	definition := `{
		"StartAt": "Notify",
		"States": {
			"Notify": {
				"Type": "Task",
				"Resource": "arn:aws:states:::lambda:invoke.waitForTaskToken",
				"HeartbeatSeconds": 1,
				"Parameters": {"FunctionName": "notify-fn", "Payload": {"taskToken.$": "$$.Task.Token"}},
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "callback-heartbeat-alive", definition)

	started, err := store.StartExecution(context.Background(), sm.StateMachineArn, "", `{}`, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	token := recvTaskToken(t, tokenCh)

	// Send three heartbeats, each well inside the 1-second interval, for a
	// total elapsed time well past it, proving each heartbeat resets the
	// deadline rather than the wait timing out after the first interval.
	for range 3 {
		time.Sleep(600 * time.Millisecond)

		if err := store.SendTaskHeartbeat(context.Background(), token); err != nil {
			t.Fatalf("SendTaskHeartbeat: %v", err)
		}
	}

	if err := store.SendTaskSuccess(context.Background(), token, `{"done":true}`); err != nil {
		t.Fatalf("SendTaskSuccess: %v", err)
	}

	exec := pollExecutionUntilTerminal(t, store, started.ExecutionArn)
	if exec.Status != ExecutionStatusSucceeded {
		t.Fatalf("execution status: got %q, want SUCCEEDED (error: %s, cause: %s)", exec.Status, exec.Error, exec.Cause)
	}
}
