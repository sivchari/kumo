package sfn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const echoItemProcessorJSON = `{"StartAt": "Echo", "States": {"Echo": {"Type": "Pass", "End": true}}}`

// threeItemArrayJSON is the "[1,2,3]" Map input/output literal shared by
// several tests across this package (goconst flags it once repeated three
// or more times).
const threeItemArrayJSON = `[1,2,3]`

func TestMapStateProcessesItemsInOrder(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemProcessor": ` + echoItemProcessorJSON + `,
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-order", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, threeItemArrayJSON)

	if exec.Output != threeItemArrayJSON {
		t.Fatalf("execution output: got %q, want %q", exec.Output, threeItemArrayJSON)
	}
}

func TestMapStateResolvesItemsPath(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemsPath": "$.items",
				"ItemProcessor": ` + echoItemProcessorJSON + `,
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-itemspath", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"items":[10,20,30]}`)

	if exec.Output != `[10,20,30]` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `[10,20,30]`)
	}
}

func TestMapStateIteratorAliasWorks(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"Iterator": ` + echoItemProcessorJSON + `,
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-iterator-alias", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `["x","y"]`)

	if exec.Output != `["x","y"]` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `["x","y"]`)
	}
}

func TestMapStateItemFailureFailsTheMap(t *testing.T) {
	t.Parallel()

	// The item's processor has no listener at this port, so every item
	// invocation fails and reports States.TaskFailed.
	processor := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:missing-fn",
				"End": true
			}
		}
	}`

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemProcessor": ` + processor + `,
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL("http://127.0.0.1:1"))
	sm := createExecutionTestStateMachine(t, store, "map-item-fails", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, threeItemArrayJSON)

	if exec.Error != errorStatesTaskFailed {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesTaskFailed)
	}
}

// newConcurrencyTrackingServer returns an httptest server that briefly
// sleeps on every request and records the maximum number of requests it
// ever saw in flight at once, so a test can assert MaxConcurrency was
// honored.
func newConcurrencyTrackingServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()

	var current, maxSeen int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&current, 1)

		for {
			m := atomic.LoadInt32(&maxSeen)
			if n <= m || atomic.CompareAndSwapInt32(&maxSeen, m, n) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&current, -1)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	t.Cleanup(server.Close)

	return server, &maxSeen
}

func TestMapStateMaxConcurrencyOneSerializesItems(t *testing.T) {
	t.Parallel()

	server, maxSeen := newConcurrencyTrackingServer(t)

	processor := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:slow-fn",
				"End": true
			}
		}
	}`

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"MaxConcurrency": 1,
				"ItemProcessor": ` + processor + `,
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-max-concurrency", definition)

	startAndAwaitSuccess(t, store, sm.StateMachineArn, threeItemArrayJSON)

	if got := atomic.LoadInt32(maxSeen); got != 1 {
		t.Fatalf("max observed concurrent item invocations: got %d, want 1", got)
	}
}

func TestMapStateItemProcessorTaskRetryRecovers(t *testing.T) {
	t.Parallel()

	server, attempts := newFlakyLambdaServer(t, 1)

	processor := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:flaky-fn",
				"Retry": [{"ErrorEquals": ["States.TaskFailed"], "IntervalSeconds": 1, "MaxAttempts": 1}],
				"End": true
			}
		}
	}`

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemProcessor": ` + processor + `,
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-item-retry-recovers", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `[1]`)

	var outputs []map[string]bool
	if err := json.Unmarshal([]byte(exec.Output), &outputs); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if len(outputs) != 1 || !outputs[0]["ok"] {
		t.Fatalf("execution output: got %q, want a single-item array with ok:true", exec.Output)
	}

	if got := atomic.LoadInt32(attempts); got != 2 {
		t.Fatalf("lambda invocation count: got %d, want 2 (1 failure + 1 successful retry)", got)
	}
}
