package lambda

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	// asyncQueueCapacity bounds each function's event queue. Enqueue never
	// blocks the Invoke handler; events beyond this are dropped and logged.
	asyncQueueCapacity = 1024

	// asyncInitialBackoff is the first retry delay after a delivery failure.
	asyncInitialBackoff = 100 * time.Millisecond

	// asyncMaxBackoff caps the exponential backoff between retries.
	asyncMaxBackoff = 5 * time.Second

	// asyncMaxEventAge is how long an event is retried after a system error
	// (endpoint unreachable) before it is dropped, mirroring Lambda's
	// default maximum event age for asynchronous invocation.
	asyncMaxEventAge = 6 * time.Hour

	// asyncMaxFunctionErrorRetries is the number of retries after the
	// endpoint responds with an error status, mirroring Lambda's default
	// of two retry attempts on function errors.
	asyncMaxFunctionErrorRetries = 2
)

// asyncDeliverer delivers one queued event to its execution target. It
// reports whether the attempt succeeded, failed as a function error (limited
// retries), or failed as a system error (retried until the event's
// deadline), mirroring AWS async invocation semantics. Implementations:
// endpointDeliverer (InvokeEndpoint) and runtimeDeliverer (Runtime API).
type asyncDeliverer interface {
	deliver(ctx context.Context, functionName string, payload []byte) deliveryResult
}

// asyncEvent is one queued asynchronous (InvocationType: Event) invocation.
type asyncEvent struct {
	deliverer asyncDeliverer
	payload   []byte
	deadline  time.Time
}

// asyncDispatcher queues Event invocations per function and delivers them in
// FIFO order via each event's asyncDeliverer (InvokeEndpoint or Runtime API).
// Delivery is retried with exponential backoff, so a 202 from Invoke means
// "accepted for delivery" rather than "attempted once" (issue #803).
type asyncDispatcher struct {
	client *http.Client
	done   chan struct{}
	wg     sync.WaitGroup

	initialBackoff time.Duration
	maxBackoff     time.Duration
	maxEventAge    time.Duration

	mu     sync.Mutex
	queues map[string]chan *asyncEvent
}

func newAsyncDispatcher() *asyncDispatcher {
	return &asyncDispatcher{
		client:         http.DefaultClient,
		done:           make(chan struct{}),
		initialBackoff: asyncInitialBackoff,
		maxBackoff:     asyncMaxBackoff,
		maxEventAge:    asyncMaxEventAge,
		queues:         make(map[string]chan *asyncEvent),
	}
}

// enqueue places an event on the function's FIFO queue without blocking the
// caller. The drain goroutine for the function is started on first use.
func (d *asyncDispatcher) enqueue(functionName string, deliverer asyncDeliverer, payload []byte) {
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)

	ev := &asyncEvent{
		deliverer: deliverer,
		payload:   payloadCopy,
		deadline:  time.Now().Add(d.maxEventAge),
	}

	d.mu.Lock()

	q, ok := d.queues[functionName]
	if !ok {
		q = make(chan *asyncEvent, asyncQueueCapacity)
		d.queues[functionName] = q

		d.wg.Add(1)

		go d.drain(functionName, q)
	}
	d.mu.Unlock()

	select {
	case q <- ev:
	default:
		slog.Error("async invoke queue full, event dropped", "function", functionName)
	}
}

// close stops all drain goroutines and waits for them to exit. In-flight
// requests are aborted through each drain goroutine's context.
func (d *asyncDispatcher) close() {
	close(d.done)
	d.wg.Wait()
}

// drain delivers queued events one at a time. Head-of-line blocking is
// deliberate: it preserves per-function delivery order.
func (d *asyncDispatcher) drain(functionName string, q chan *asyncEvent) {
	defer d.wg.Done()

	// Lifecycle context for this goroutine's deliveries, canceled when the
	// dispatcher closes so in-flight requests are aborted. Created here and
	// passed down instead of being stored on the dispatcher.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-d.done:
			cancel()
		case <-ctx.Done():
		}
	}()

	for {
		select {
		case <-d.done:
			return
		case ev := <-q:
			d.deliver(ctx, functionName, ev)
		}
	}
}

// deliveryResult classifies one delivery attempt.
type deliveryResult int

const (
	// asyncDelivered: the deliverer accepted the event.
	asyncDelivered deliveryResult = iota
	// asyncFunctionError: the target responded with an error.
	asyncFunctionError
	// asyncSystemError: the target could not be reached.
	asyncSystemError
	// asyncPermanentFailure: the event can never be delivered (e.g. bad endpoint URL).
	asyncPermanentFailure
)

// deliver hands the event to its deliverer, retrying failures until the event
// is delivered, exhausts its function-error retries, or expires.
func (d *asyncDispatcher) deliver(ctx context.Context, functionName string, ev *asyncEvent) {
	backoff := d.initialBackoff
	functionErrorRetries := 0

	for {
		switch ev.deliverer.deliver(ctx, functionName, ev.payload) {
		case asyncDelivered, asyncPermanentFailure:
			return
		case asyncFunctionError:
			if functionErrorRetries >= asyncMaxFunctionErrorRetries {
				slog.Error("async invoke dropped after function error retries",
					"function", functionName, "retries", functionErrorRetries)

				return
			}

			functionErrorRetries++
		case asyncSystemError:
			if time.Now().After(ev.deadline) {
				slog.Error("async invoke dropped, event expired", "function", functionName)

				return
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, d.maxBackoff)
	}
}

// endpointDeliverer delivers an event by POSTing it to a function's
// InvokeEndpoint. Counterpart of runtimeDeliverer (runtime.go) for functions
// backed by a Runtime API handler.
type endpointDeliverer struct {
	client   *http.Client
	endpoint string
}

// deliver performs a single delivery attempt.
func (e *endpointDeliverer) deliver(ctx context.Context, functionName string, payload []byte) deliveryResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(payload))
	if err != nil {
		slog.Error("async invoke failed to create request", "function", functionName, "error", err)

		return asyncPermanentFailure
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		slog.Warn("async invoke attempt failed, will retry", "function", functionName, "error", err)

		return asyncSystemError
	}

	_ = resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		slog.Warn("async invoke returned error status", "function", functionName, "status", resp.StatusCode)

		return asyncFunctionError
	}

	return asyncDelivered
}
