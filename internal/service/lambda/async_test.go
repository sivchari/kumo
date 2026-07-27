package lambda

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
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
	deliverer := &endpointDeliverer{client: d.client, endpoint: endpoint, gate: d.gate("fn")}

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

	d.enqueue("fn", &endpointDeliverer{client: d.client, endpoint: srv.URL, gate: d.gate("fn")}, []byte(`{}`))

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

	d.enqueue("fn", &endpointDeliverer{client: d.client, endpoint: "http://" + addr + "/invoke", gate: d.gate("fn")}, []byte(`{}`))

	// Wait until the event is past its deadline and has been dropped.
	time.Sleep(200 * time.Millisecond)

	rec := &payloadRecorder{}
	serveAt(t, addr, rec)

	if waitFor(t, 300*time.Millisecond, func() bool { return len(rec.snapshot()) > 0 }) {
		t.Fatalf("expected expired event to be dropped, got deliveries %v", rec.snapshot())
	}
}

// TestAsyncDispatcher_CloseDoesNotHangOnStuckGate guards the per-function
// gate's context-cancelable acquire (issue #859): if a synchronous invoke
// acquires a function's gate and never releases it -- e.g. it is blocked
// forever inside its own HTTP call against a wedged InvokeEndpoint -- the
// async drain goroutine waiting on that same gate must still give up when
// the dispatcher closes, instead of blocking asyncDispatcher.close forever.
func TestAsyncDispatcher_CloseDoesNotHangOnStuckGate(t *testing.T) {
	t.Parallel()

	d := newAsyncDispatcher()
	d.initialBackoff = 5 * time.Millisecond
	d.maxBackoff = 20 * time.Millisecond

	gate := d.gate("fn")
	if !gate.acquire(t.Context()) {
		t.Fatal("failed to acquire gate")
	}
	// Deliberately never released: simulates a sync invoke stuck forever.

	d.enqueue("fn", &endpointDeliverer{client: d.client, endpoint: "http://127.0.0.1:0/invoke", gate: gate}, []byte(`{}`))

	// Give the drain goroutine time to start waiting on the (permanently
	// held) gate before closing.
	time.Sleep(20 * time.Millisecond)

	closed := make(chan struct{})

	go func() {
		d.close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("asyncDispatcher.close hung waiting on a gate held by a stuck peer")
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

// singleFlightServer is an InvokeEndpoint stand-in that fails the test the
// moment it observes two overlapping requests, mimicking a single-concurrency
// runtime such as the Lambda RIE.
type singleFlightServer struct {
	t        *testing.T
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	handled  atomic.Int32
	sleep    time.Duration
}

func newSingleFlightServer(t *testing.T, sleep time.Duration) (*httptest.Server, *singleFlightServer) {
	t.Helper()

	sfs := &singleFlightServer{t: t, sleep: sleep}

	return httptest.NewServer(http.HandlerFunc(sfs.serve)), sfs
}

func (sfs *singleFlightServer) serve(w http.ResponseWriter, _ *http.Request) {
	n := sfs.inFlight.Add(1)
	defer sfs.inFlight.Add(-1)

	for {
		seen := sfs.maxSeen.Load()
		if n <= seen || sfs.maxSeen.CompareAndSwap(seen, n) {
			break
		}
	}

	if n > 1 {
		sfs.t.Errorf("endpoint saw %d overlapping requests, want at most 1", n)
	}

	time.Sleep(sfs.sleep)
	sfs.handled.Add(1)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

// newInvokeTestService returns a Service backed by an in-memory storage with
// fn already registered against endpoint as its InvokeEndpoint.
func newInvokeTestService(t *testing.T, fn, endpoint string) *Service {
	t.Helper()

	return newInvokeTestServiceMulti(t, map[string]string{fn: endpoint})
}

// newInvokeTestServiceMulti returns a Service backed by an in-memory storage
// with each function in endpoints already registered against its
// InvokeEndpoint.
func newInvokeTestServiceMulti(t *testing.T, endpoints map[string]string) *Service {
	t.Helper()

	storage := NewMemoryStorage(defaultBaseURL)
	svc := New(storage, defaultBaseURL)

	t.Cleanup(func() {
		if err := svc.Close(); err != nil {
			t.Fatalf("close service: %v", err)
		}
	})

	for fn, endpoint := range endpoints {
		if _, err := storage.CreateFunction(t.Context(), &CreateFunctionRequest{
			FunctionName:   fn,
			Role:           "arn:aws:iam::000000000000:role/test",
			InvokeEndpoint: endpoint,
		}); err != nil {
			t.Fatalf("create function %s: %v", fn, err)
		}
	}

	return svc
}

// invokeRequest builds an Invoke request for fn with the given invocation
// type ("" for RequestResponse).
func invokeRequest(t *testing.T, fn, invocationType string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/2015-03-31/functions/"+fn+"/invocations", bytes.NewReader([]byte(`{}`)))
	if invocationType != "" {
		req.Header.Set("X-Amz-Invocation-Type", invocationType)
	}

	return req.WithContext(t.Context())
}

// TestInvoke_SyncAndAsyncSerializedPerFunction reproduces issue #859: a
// synchronous (RequestResponse) invoke and an asynchronous (Event) invoke
// against the same function's InvokeEndpoint must never be in flight at the
// same time, so a single-concurrency runtime (e.g. the Lambda RIE) never
// sees overlapping requests.
func TestInvoke_SyncAndAsyncSerializedPerFunction(t *testing.T) {
	t.Parallel()

	srv, sfs := newSingleFlightServer(t, 100*time.Millisecond)
	t.Cleanup(srv.Close)

	svc := newInvokeTestService(t, "fn", srv.URL)

	var wg sync.WaitGroup

	wg.Add(2)

	syncRec := httptest.NewRecorder()

	go func() {
		defer wg.Done()

		svc.Invoke(syncRec, invokeRequest(t, "fn", "RequestResponse"))
	}()

	go func() {
		defer wg.Done()

		asyncRec := httptest.NewRecorder()
		svc.Invoke(asyncRec, invokeRequest(t, "fn", "Event"))

		if asyncRec.Code != http.StatusAccepted {
			t.Errorf("async invoke status = %d, want %d", asyncRec.Code, http.StatusAccepted)
		}
	}()

	wg.Wait()

	if syncRec.Code != http.StatusOK {
		t.Errorf("sync invoke status = %d, want %d, body %s", syncRec.Code, http.StatusOK, syncRec.Body.String())
	}

	// The sync invoke has already completed above; the async delivery may
	// still be draining, so wait for it to land too before asserting the
	// endpoint's final view of concurrency.
	if !waitFor(t, 5*time.Second, func() bool { return sfs.handled.Load() == 2 }) {
		t.Fatalf("expected 2 deliveries to the endpoint, got %d", sfs.handled.Load())
	}

	if got := sfs.maxSeen.Load(); got > 1 {
		t.Errorf("endpoint saw up to %d overlapping requests, want at most 1", got)
	}
}

// newTrackedServer returns an httptest server whose handler closes started
// on receiving a request, then blocks until release is closed.
func newTrackedServer(t *testing.T, started chan struct{}, release <-chan struct{}) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	return srv
}

// invokeSyncAndAssertOK fires a RequestResponse invoke for fn and records a
// t.Errorf if it doesn't come back 200 OK. Intended to run as `go
// invokeSyncAndAssertOK(...)`; it calls wg.Done itself.
func invokeSyncAndAssertOK(t *testing.T, wg *sync.WaitGroup, svc *Service, fn string) {
	t.Helper()

	defer wg.Done()

	rec := httptest.NewRecorder()
	svc.Invoke(rec, invokeRequest(t, fn, "RequestResponse"))

	if rec.Code != http.StatusOK {
		t.Errorf("%s invoke status = %d, want %d", fn, rec.Code, http.StatusOK)
	}
}

// TestInvoke_DifferentFunctionsNotSerialized verifies that invocations
// targeting different functions are not gated against each other: two
// concurrent requests to two different functions' endpoints must be allowed
// to overlap.
func TestInvoke_DifferentFunctionsNotSerialized(t *testing.T) {
	t.Parallel()

	var (
		aStarted = make(chan struct{})
		bStarted = make(chan struct{})
		release  = make(chan struct{})
	)

	srvA := newTrackedServer(t, aStarted, release)
	srvB := newTrackedServer(t, bStarted, release)

	svc := newInvokeTestServiceMulti(t, map[string]string{"fn-a": srvA.URL, "fn-b": srvB.URL})

	var wg sync.WaitGroup

	wg.Add(2)

	go invokeSyncAndAssertOK(t, &wg, svc, "fn-a")
	go invokeSyncAndAssertOK(t, &wg, svc, "fn-b")

	// Both handlers must be able to start concurrently: neither is gated
	// against the other, so both reach their <-release wait without one
	// blocking on the other's in-flight request.
	select {
	case <-aStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("fn-a endpoint never started, functions may be wrongly serialized")
	}

	select {
	case <-bStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("fn-b endpoint never started while fn-a was in flight, functions are wrongly serialized")
	}

	close(release)
	wg.Wait()
}
