package lambda

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// newRuntimeTestService returns a Service with fn created with the given
// Timeout (seconds) and no InvokeEndpoint, so it is served only by Runtime
// API handlers.
func newRuntimeTestService(t *testing.T, fn string, timeoutSeconds int) *Service {
	t.Helper()

	storage := NewMemoryStorage(defaultBaseURL)
	svc := New(storage, defaultBaseURL)

	t.Cleanup(func() {
		if err := svc.Close(); err != nil {
			t.Fatalf("close service: %v", err)
		}
	})

	if _, err := storage.CreateFunction(t.Context(), &CreateFunctionRequest{
		FunctionName: fn,
		Role:         "arn:aws:iam::000000000000:role/test",
		Timeout:      timeoutSeconds,
	}); err != nil {
		t.Fatalf("create function %s: %v", fn, err)
	}

	return svc
}

// pollNext runs RuntimeNext for fn in the background, as a handler's
// long-poll would, and returns a channel that yields the response once an
// invocation is handed out.
func pollNext(t *testing.T, svc *Service, fn string) <-chan *httptest.ResponseRecorder {
	t.Helper()

	done := make(chan *httptest.ResponseRecorder, 1)

	go func() {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
			"/_runtime/"+fn+"/2018-06-01/runtime/invocation/next", nil)
		svc.RuntimeNext(rec, req)
		done <- rec
	}()

	if !waitFor(t, 5*time.Second, func() bool { return svc.broker.registered(fn) }) {
		t.Fatal("handler never registered by polling next")
	}

	return done
}

// TestRuntimeNext_DeadlineFollowsFunctionTimeout verifies that the deadline a
// handler receives is the function's Timeout counted from pickup, not a
// fixed kumo-side value.
func TestRuntimeNext_DeadlineFollowsFunctionTimeout(t *testing.T) {
	t.Parallel()

	svc := newRuntimeTestService(t, "fn", 900)
	next := pollNext(t, svc, "fn")

	invoked := make(chan *httptest.ResponseRecorder, 1)

	go func() {
		rec := httptest.NewRecorder()
		svc.Invoke(rec, invokeRequest(t, "fn", ""))
		invoked <- rec
	}()

	var rec *httptest.ResponseRecorder

	select {
	case rec = <-next:
	case <-time.After(5 * time.Second):
		t.Fatal("handler never received the invocation")
	}

	deadlineMs, err := strconv.ParseInt(rec.Header().Get("Lambda-Runtime-Deadline-Ms"), 10, 64)
	if err != nil {
		t.Fatalf("parse deadline header: %v", err)
	}

	remaining := time.Until(time.UnixMilli(deadlineMs))
	if remaining < 899*time.Second || remaining > 900*time.Second {
		t.Errorf("remaining time = %v, want ~900s (the function's Timeout)", remaining)
	}

	// Complete the invocation so the Invoke goroutine returns.
	svc.broker.respond("fn", rec.Header().Get("Lambda-Runtime-Aws-Request-Id"), []byte(`{}`), false)

	select {
	case res := <-invoked:
		if res.Code != http.StatusOK {
			t.Errorf("invoke status = %d, want %d", res.Code, http.StatusOK)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("invoke never returned after the handler responded")
	}
}

// TestInvoke_RuntimeFunctionTimeoutIsFunctionError verifies that a
// synchronous invocation whose handler does not respond within the
// function's Timeout is reported the way Lambda reports a function timeout:
// 200 with X-Amz-Function-Error and an error payload, not a service error.
func TestInvoke_RuntimeFunctionTimeoutIsFunctionError(t *testing.T) {
	t.Parallel()

	svc := newRuntimeTestService(t, "fn", 1)
	next := pollNext(t, svc, "fn")

	start := time.Now()
	rec := httptest.NewRecorder()

	svc.Invoke(rec, invokeRequest(t, "fn", "")) // the handler takes it and never responds

	elapsed := time.Since(start)

	select {
	case <-next:
	default:
		t.Fatal("handler never received the invocation")
	}

	if elapsed < time.Second || elapsed > 5*time.Second {
		t.Errorf("invoke returned after %v, want ~1s (the function's Timeout)", elapsed)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if got := rec.Header().Get("X-Amz-Function-Error"); got != "Unhandled" {
		t.Errorf("X-Amz-Function-Error = %q, want %q", got, "Unhandled")
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload %s: %v", rec.Body.String(), err)
	}

	if payload["errorType"] != "Sandbox.Timedout" {
		t.Errorf("errorType = %q, want %q", payload["errorType"], "Sandbox.Timedout")
	}

	if payload["errorMessage"] != "Task timed out after 1.00 seconds" {
		t.Errorf("errorMessage = %q", payload["errorMessage"])
	}
}
