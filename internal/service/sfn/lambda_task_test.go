package sfn

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newEchoLambdaServer returns an httptest server that stands in for kumo's
// Lambda invoke endpoint: it echoes the request body back as the response,
// mirroring what a real Lambda function that returns its own event would do.
func newEchoLambdaServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))

	t.Cleanup(server.Close)

	return server
}

func TestTaskStatePlainLambdaARNSendsStateInputAsPayload(t *testing.T) {
	t.Parallel()

	server := newEchoLambdaServer(t)

	definition := fmt.Sprintf(`{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:%s",
				"End": true
			}
		}
	}`, "echo-fn")

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "lambda-arn-task", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"greeting":"hello"}`)

	if exec.Output != `{"greeting":"hello"}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"greeting":"hello"}`)
	}
}

func TestTaskStatePlainLambdaARNResolvesParametersAsPayload(t *testing.T) {
	t.Parallel()

	server := newEchoLambdaServer(t)

	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:echo-fn",
				"Parameters": {"name.$": "$.who", "greeting": "hi"},
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "lambda-arn-task-params", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"who":"world"}`)

	var output map[string]any
	if err := json.Unmarshal([]byte(exec.Output), &output); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if output["name"] != "world" || output["greeting"] != "hi" {
		t.Fatalf("execution output: got %v, want name=world greeting=hi", output)
	}
}

func TestExecuteLambdaFunctionTaskCallsExtractedFunctionName(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	engine := newExecutionEngine(server.URL)

	output, err := engine.executeLambdaFunctionTask(
		t.Context(),
		"Invoke",
		"arn:aws:lambda:us-east-1:000000000000:function:my-function",
		nil,
		`{"x":1}`,
	)
	if err != nil {
		t.Fatalf("executeLambdaFunctionTask: %v", err)
	}

	if output != `{"ok":true}` {
		t.Fatalf("output: got %q, want %q", output, `{"ok":true}`)
	}

	if !strings.HasSuffix(gotPath, "/functions/my-function/invocations") {
		t.Fatalf("invocation path: got %q, want suffix %q", gotPath, "/functions/my-function/invocations")
	}
}
