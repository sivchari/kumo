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

// asyncEvent is one queued asynchronous (InvocationType: Event) invocation.
type asyncEvent struct {
	endpoint string
	payload  []byte
	deadline time.Time
}

// asyncDispatcher queues Event invocations per function and delivers them to
// the function's InvokeEndpoint in FIFO order. Delivery is retried with
// exponential backoff, so a 202 from Invoke means "accepted for delivery"
// rather than "attempted once" (issue #803). Counterpart of runtimeBroker for
// the InvokeEndpoint execution path.
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
func (d *asyncDispatcher) enqueue(functionName, endpoint string, payload []byte) {
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)

	ev := &asyncEvent{
		endpoint: endpoint,
		payload:  payloadCopy,
		deadline: time.Now().Add(d.maxEventAge),
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
	// asyncDelivered: the endpoint accepted the event.
	asyncDelivered deliveryResult = iota
	// asyncFunctionError: the endpoint responded with an error status.
	asyncFunctionError
	// asyncSystemError: the endpoint could not be reached.
	asyncSystemError
	// asyncPermanentFailure: the event can never be delivered (bad endpoint).
	asyncPermanentFailure
)

// deliver posts the event to its endpoint, retrying failures until the event
// is delivered, exhausts its function-error retries, or expires.
func (d *asyncDispatcher) deliver(ctx context.Context, functionName string, ev *asyncEvent) {
	backoff := d.initialBackoff
	functionErrorRetries := 0

	for {
		switch d.post(ctx, functionName, ev) {
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

// post performs a single delivery attempt.
func (d *asyncDispatcher) post(ctx context.Context, functionName string, ev *asyncEvent) deliveryResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ev.endpoint, bytes.NewReader(ev.payload))
	if err != nil {
		slog.Error("async invoke failed to create request", "function", functionName, "error", err)

		return asyncPermanentFailure
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
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
