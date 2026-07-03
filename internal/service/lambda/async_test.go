package lambda

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// reserveAddr reserves a TCP address and immediately releases it, so
// connections to it are refused until a server binds it again.
func reserveAddr(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig

	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	addr := lis.Addr().String()

	if err := lis.Close(); err != nil {
		t.Fatal(err)
	}

	return addr
}

// serveAt starts an HTTP server bound to addr for the rest of the test.
func serveAt(t *testing.T, addr string, handler http.Handler) {
	t.Helper()

	var lc net.ListenConfig

	lis, err := lc.Listen(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatalf("rebind %s: %v", addr, err)
	}

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(lis) }()

	t.Cleanup(func() { _ = srv.Close() })
}

// newTestDispatcher returns a dispatcher with short backoffs for tests.
func newTestDispatcher(t *testing.T) *asyncDispatcher {
	t.Helper()

	d := newAsyncDispatcher()
	d.initialBackoff = 5 * time.Millisecond
	d.maxBackoff = 20 * time.Millisecond

	t.Cleanup(d.close)

	return d
}

// payloadRecorder is an HTTP handler that records request bodies in order.
type payloadRecorder struct {
	mu       sync.Mutex
	payloads []string
}

func (rec *payloadRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	rec.mu.Lock()
	rec.payloads = append(rec.payloads, string(body))
	rec.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (rec *payloadRecorder) snapshot() []string {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	return append([]string(nil), rec.payloads...)
}

// waitFor polls cond until it returns true or the deadline passes.
func waitFor(t *testing.T, d time.Duration, cond func() bool) bool {
	t.Helper()

	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}

		time.Sleep(10 * time.Millisecond)
	}

	return cond()
}

// TestAsyncDispatcher_DeliversAfterEndpointRecovers reproduces issue #803:
// events enqueued while the endpoint is down must be delivered exactly once,
// in order, after the endpoint comes up.
func TestAsyncDispatcher_DeliversAfterEndpointRecovers(t *testing.T) {
	d := newTestDispatcher(t)
	addr := reserveAddr(t)

	endpoint := "http://" + addr + "/invoke"
	for i := 1; i <= 3; i++ {
		d.enqueue("fn", endpoint, fmt.Appendf(nil, `{"seq":%d}`, i))
	}

	// Give the dispatcher time to fail at least one attempt.
	time.Sleep(50 * time.Millisecond)

	rec := &payloadRecorder{}
	serveAt(t, addr, rec)

	if !waitFor(t, 5*time.Second, func() bool { return len(rec.snapshot()) == 3 }) {
		t.Fatalf("expected 3 deliveries, got %v", rec.snapshot())
	}

	// No duplicate deliveries after settling.
	time.Sleep(100 * time.Millisecond)

	got := rec.snapshot()
	want := []string{`{"seq":1}`, `{"seq":2}`, `{"seq":3}`}

	if len(got) != len(want) {
		t.Fatalf("expected exactly %d deliveries, got %v", len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("delivery %d = %s, want %s (order must be preserved)", i, got[i], want[i])
		}
	}
}

// TestAsyncDispatcher_FunctionErrorRetries verifies that an endpoint
// responding with an error status is retried exactly
// asyncMaxFunctionErrorRetries times before the event is dropped.
func TestAsyncDispatcher_FunctionErrorRetries(t *testing.T) {
	d := newTestDispatcher(t)

	var (
		mu       sync.Mutex
		attempts int
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()

		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	count := func() int {
		mu.Lock()
		defer mu.Unlock()

		return attempts
	}

	d.enqueue("fn", srv.URL, []byte(`{}`))

	wantAttempts := 1 + asyncMaxFunctionErrorRetries
	if !waitFor(t, 5*time.Second, func() bool { return count() == wantAttempts }) {
		t.Fatalf("expected %d attempts, got %d", wantAttempts, count())
	}

	// The event must be dropped after the final retry, not retried forever.
	time.Sleep(100 * time.Millisecond)

	if got := count(); got != wantAttempts {
		t.Fatalf("expected exactly %d attempts, got %d", wantAttempts, got)
	}
}

// TestAsyncDispatcher_ExpiredEventDropped verifies that an event whose
// endpoint stays unreachable past the maximum event age is dropped.
func TestAsyncDispatcher_ExpiredEventDropped(t *testing.T) {
	d := newTestDispatcher(t)
	d.maxEventAge = 30 * time.Millisecond

	addr := reserveAddr(t)

	d.enqueue("fn", "http://"+addr+"/invoke", []byte(`{}`))

	// Wait until the event is past its deadline and has been dropped.
	time.Sleep(200 * time.Millisecond)

	rec := &payloadRecorder{}
	serveAt(t, addr, rec)

	if waitFor(t, 300*time.Millisecond, func() bool { return len(rec.snapshot()) > 0 }) {
		t.Fatalf("expected expired event to be dropped, got deliveries %v", rec.snapshot())
	}
}
