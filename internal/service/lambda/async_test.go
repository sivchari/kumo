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
	deliverer := &endpointDeliverer{client: d.client, endpoint: endpoint}

	for i := 1; i <= 3; i++ {
		d.enqueue("fn", deliverer, fmt.Appendf(nil, `{"seq":%d}`, i))
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

	d.enqueue("fn", &endpointDeliverer{client: d.client, endpoint: srv.URL}, []byte(`{}`))

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

	d.enqueue("fn", &endpointDeliverer{client: d.client, endpoint: "http://" + addr + "/invoke"}, []byte(`{}`))

	// Wait until the event is past its deadline and has been dropped.
	time.Sleep(200 * time.Millisecond)

	rec := &payloadRecorder{}
	serveAt(t, addr, rec)

	if waitFor(t, 300*time.Millisecond, func() bool { return len(rec.snapshot()) > 0 }) {
		t.Fatalf("expected expired event to be dropped, got deliveries %v", rec.snapshot())
	}
}

// runHandler simulates a Runtime API handler (lambda.Start) that repeatedly
// polls broker.next and answers with respond, until the test ends.
func runHandler(t *testing.T, broker *runtimeBroker, fn string, respond func(inv *runtimeInvocation) (payload []byte, errored bool)) {
	t.Helper()

	ctx := t.Context()

	go func() {
		for {
			inv, err := broker.next(ctx, fn)
			if err != nil {
				return
			}

			payload, errored := respond(inv)
			broker.respond(fn, inv.id, payload, errored)
		}
	}()
}

// TestAsyncDispatcher_RuntimeDeliveredWhileHandlerBusy reproduces the drop
// the unbuffered channel used to cause: two async Invokes sent back-to-back
// while the handler is slow on the first must both eventually be delivered,
// in FIFO order, instead of the second vanishing.
func TestAsyncDispatcher_RuntimeDeliveredWhileHandlerBusy(t *testing.T) {
	d := newTestDispatcher(t)
	broker := newRuntimeBroker()

	var (
		mu    sync.Mutex
		got   []string
		first = true
	)

	runHandler(t, broker, "fn", func(inv *runtimeInvocation) ([]byte, bool) {
		if first {
			first = false

			time.Sleep(50 * time.Millisecond)
		}

		mu.Lock()
		got = append(got, string(inv.payload))
		mu.Unlock()

		return inv.payload, false
	})

	deliverer := &runtimeDeliverer{broker: broker, fn: "fn"}
	d.enqueue("fn", deliverer, []byte(`{"seq":1}`))
	d.enqueue("fn", deliverer, []byte(`{"seq":2}`))

	if !waitFor(t, 5*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()

		return len(got) == 2
	}) {
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("expected 2 deliveries, got %v", got)
	}

	mu.Lock()
	defer mu.Unlock()

	want := []string{`{"seq":1}`, `{"seq":2}`}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("delivery %d = %s, want %s (order must be preserved)", i, got[i], want[i])
		}
	}
}

// TestAsyncDispatcher_RuntimeDeliveredAfterHandlerPollsLate reproduces the
// case the unbuffered channel dropped silently: the function is registered
// (so Invoke routes to the runtime path) but no handler is blocked in next
// when the event is queued. The event must survive the deliverer's timed-out
// first attempt (a system error) and be delivered once the handler resumes
// polling, instead of vanishing.
func TestAsyncDispatcher_RuntimeDeliveredAfterHandlerPollsLate(t *testing.T) {
	d := newTestDispatcher(t)
	broker := newRuntimeBroker()

	// Register the function (as broker.registered would report it) without
	// any goroutine blocked in next — mirrors a handler that polled once
	// before and is briefly not polling now.
	broker.get("fn")

	if !broker.registered("fn") {
		t.Fatal("expected function to be registered")
	}

	received := make(chan []byte, 1)

	deliverer := &runtimeDeliverer{broker: broker, fn: "fn", waitTimeout: 20 * time.Millisecond}
	d.enqueue("fn", deliverer, []byte(`{"late":true}`))

	// Let the deliverer's first wait (and at least one retry) time out
	// before the handler starts polling.
	time.Sleep(80 * time.Millisecond)

	go func() {
		inv, err := broker.next(t.Context(), "fn")
		if err != nil {
			return
		}

		received <- inv.payload
		broker.respond("fn", inv.id, inv.payload, false)
	}()

	select {
	case got := <-received:
		if string(got) != `{"late":true}` {
			t.Fatalf("got payload %s, want %s", got, `{"late":true}`)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("event was never delivered after handler resumed polling")
	}
}

// TestAsyncDispatcher_RuntimeFunctionErrorRetries verifies that a handler
// responding with an error result is retried exactly
// asyncMaxFunctionErrorRetries times before the event is dropped, matching
// the endpoint path's retry semantics.
func TestAsyncDispatcher_RuntimeFunctionErrorRetries(t *testing.T) {
	d := newTestDispatcher(t)
	broker := newRuntimeBroker()

	var (
		mu       sync.Mutex
		attempts int
	)

	runHandler(t, broker, "fn", func(inv *runtimeInvocation) ([]byte, bool) {
		mu.Lock()
		attempts++
		mu.Unlock()

		return inv.payload, true
	})

	count := func() int {
		mu.Lock()
		defer mu.Unlock()

		return attempts
	}

	d.enqueue("fn", &runtimeDeliverer{broker: broker, fn: "fn"}, []byte(`{}`))

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

// TestAsyncDispatcher_RuntimeFunctionErrorOnResponseTimeout verifies that a
// handler that polls and takes an invocation but never responds (its
// function timed out) is treated as a function error with limited retries —
// not a system error retried for up to asyncMaxEventAge — matching AWS
// async invocation semantics for a function timeout.
func TestAsyncDispatcher_RuntimeFunctionErrorOnResponseTimeout(t *testing.T) {
	d := newTestDispatcher(t)
	broker := newRuntimeBroker()

	var (
		mu       sync.Mutex
		attempts int
	)

	ctx := t.Context()

	go func() {
		for {
			// Poll and take the invocation, then never respond —
			// simulates a handler whose function timed out after
			// picking up the work.
			_, err := broker.next(ctx, "fn")
			if err != nil {
				return
			}

			mu.Lock()
			attempts++
			mu.Unlock()
		}
	}()

	count := func() int {
		mu.Lock()
		defer mu.Unlock()

		return attempts
	}

	deliverer := &runtimeDeliverer{broker: broker, fn: "fn", waitTimeout: 20 * time.Millisecond}
	d.enqueue("fn", deliverer, []byte(`{}`))

	wantAttempts := 1 + asyncMaxFunctionErrorRetries
	if !waitFor(t, 5*time.Second, func() bool { return count() == wantAttempts }) {
		t.Fatalf("expected %d attempts, got %d", wantAttempts, count())
	}

	// The event must be dropped after the final retry (function-error
	// semantics), not retried for up to asyncMaxEventAge as a system error.
	time.Sleep(200 * time.Millisecond)

	if got := count(); got != wantAttempts {
		t.Fatalf("expected exactly %d attempts, got %d", wantAttempts, got)
	}
}
