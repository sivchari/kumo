package lambda

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// runtimePickupTimeout bounds how long an invocation waits to be picked up by
// a polling handler. It is a kumo-side limit with no AWS counterpart; once a
// handler has the invocation, the function's own Timeout applies instead.
const runtimePickupTimeout = 30 * time.Second

// defaultFunctionTimeout is Lambda's default function timeout, used when a
// function has no Timeout configured.
const defaultFunctionTimeout = 3 * time.Second

// errRuntimeNoPoller is returned when no handler polls next and picks up an
// invocation within the pickup timeout — nobody ever started the work, which
// AWS treats as a system-level delivery failure (retried with backoff).
var errRuntimeNoPoller = errors.New("no runtime handler available to poll the invocation")

// errRuntimeResponseTimeout is returned when a handler polled next and took
// the invocation but never posted a response or error before the function's
// timeout — the function timed out, which AWS treats as a function error
// (limited retries).
var errRuntimeResponseTimeout = errors.New("runtime handler did not respond before the function timeout")

// runtimeBroker bridges kumo invocations to handlers that speak the AWS
// Lambda Runtime API (lambda.Start). A handler polls next for its function;
// kumo hands it queued invocations and collects the responses.
type runtimeBroker struct {
	mu    sync.Mutex
	funcs map[string]*funcRuntime
}

type funcRuntime struct {
	invocations chan *runtimeInvocation

	mu      sync.Mutex
	pending map[string]chan runtimeResult
}

type runtimeInvocation struct {
	id      string
	payload []byte

	// timeout is the function's execution timeout. The handler that picks
	// up the invocation is given a deadline this far in the future.
	timeout time.Duration
}

type runtimeResult struct {
	payload []byte
	errored bool
}

func newRuntimeBroker() *runtimeBroker {
	return &runtimeBroker{funcs: make(map[string]*funcRuntime)}
}

// registered reports whether a handler has ever polled for this function,
// i.e. the function is backed by a Runtime API handler.
func (b *runtimeBroker) registered(fn string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, ok := b.funcs[fn]

	return ok
}

// get returns (creating if needed) the per-function runtime state.
func (b *runtimeBroker) get(fn string) *funcRuntime {
	b.mu.Lock()
	defer b.mu.Unlock()

	fr, ok := b.funcs[fn]
	if !ok {
		fr = &funcRuntime{
			invocations: make(chan *runtimeInvocation),
			pending:     make(map[string]chan runtimeResult),
		}
		b.funcs[fn] = fr
	}

	return fr
}

// functionTimeout returns fn's configured execution timeout.
func functionTimeout(fn *Function) time.Duration {
	if fn.Timeout <= 0 {
		return defaultFunctionTimeout
	}

	return time.Duration(fn.Timeout) * time.Second
}

// invoke hands an invocation to a polling handler and waits for its
// response. It waits up to pickupTimeout for a handler to take the
// invocation from next, then up to timeout (the function's execution
// timeout, also reported to the handler as its deadline) for a
// response/error. Callers that want async (fire-and-forget-but-not-really)
// semantics should queue a runtimeDeliverer on the asyncDispatcher instead
// of calling invoke directly — see invokeViaRuntime.
func (b *runtimeBroker) invoke(ctx context.Context, fn string, payload []byte, pickupTimeout, timeout time.Duration) (runtimeResult, error) {
	fr := b.get(fn)
	inv := &runtimeInvocation{id: uuid.New().String(), payload: payload, timeout: timeout}

	resCh := make(chan runtimeResult, 1)

	fr.mu.Lock()
	fr.pending[inv.id] = resCh
	fr.mu.Unlock()

	defer func() {
		fr.mu.Lock()
		delete(fr.pending, inv.id)
		fr.mu.Unlock()
	}()

	select {
	case fr.invocations <- inv:
	case <-ctx.Done():
		return runtimeResult{}, fmt.Errorf("invocation canceled: %w", ctx.Err())
	case <-time.After(pickupTimeout):
		return runtimeResult{}, errRuntimeNoPoller
	}

	select {
	case res := <-resCh:
		return res, nil
	case <-ctx.Done():
		return runtimeResult{}, fmt.Errorf("invocation canceled: %w", ctx.Err())
	case <-time.After(timeout):
		return runtimeResult{}, errRuntimeResponseTimeout
	}
}

// runtimeDeliverer delivers an event by handing it to a Runtime API handler
// through runtimeBroker's synchronous path — the async queue itself now
// provides the asynchrony, so the deliverer only ever waits for one poll/
// response round trip per attempt. Counterpart of endpointDeliverer
// (async.go) for functions backed by an InvokeEndpoint.
type runtimeDeliverer struct {
	broker *runtimeBroker
	fn     string

	// timeout is the function's execution timeout: how long one delivery
	// attempt waits for the handler's response once it has picked up the
	// event. Zero means defaultFunctionTimeout.
	timeout time.Duration

	// pickupTimeout bounds how long one delivery attempt waits for a handler
	// to pick up the event. Zero means runtimePickupTimeout; tests inject a
	// short value so retry scenarios don't need to wait out the real 30s
	// default.
	pickupTimeout time.Duration
}

// deliver hands the event to a polling handler and waits for its response.
// errRuntimeResponseTimeout means a handler took the invocation but never
// responded — the function itself timed out, so this is a function error
// with limited retries, matching AWS async semantics. Any other failure
// (nobody polled, context canceled, ...) means the work was never picked up,
// so it is a system error retried with backoff until the event's deadline. A
// handler-reported error is likewise a function error.
func (r *runtimeDeliverer) deliver(ctx context.Context, _ string, payload []byte) deliveryResult {
	pickupTimeout := r.pickupTimeout
	if pickupTimeout == 0 {
		pickupTimeout = runtimePickupTimeout
	}

	timeout := r.timeout
	if timeout == 0 {
		timeout = defaultFunctionTimeout
	}

	res, err := r.broker.invoke(ctx, r.fn, payload, pickupTimeout, timeout)
	if err != nil {
		if errors.Is(err, errRuntimeResponseTimeout) {
			return asyncFunctionError
		}

		return asyncSystemError
	}

	if res.errored {
		return asyncFunctionError
	}

	return asyncDelivered
}

// next blocks until an invocation is queued for the function or ctx is done.
func (b *runtimeBroker) next(ctx context.Context, fn string) (*runtimeInvocation, error) {
	fr := b.get(fn)

	select {
	case inv := <-fr.invocations:
		return inv, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("next canceled: %w", ctx.Err())
	}
}

// respond delivers a handler's result to the waiting invoker.
func (b *runtimeBroker) respond(fn, id string, payload []byte, errored bool) {
	fr := b.get(fn)

	fr.mu.Lock()
	ch := fr.pending[id]
	fr.mu.Unlock()

	if ch != nil {
		ch <- runtimeResult{payload: payload, errored: errored}
	}
}

// ---- Runtime API HTTP handlers ----
//
// These implement the subset of the AWS Lambda Runtime API that lambda.Start
// uses, under /_runtime/{functionName}/2018-06-01/runtime/... . A handler is
// pointed at kumo with AWS_LAMBDA_RUNTIME_API=<host>/_runtime/{functionName}.

// RuntimeNext handles GET .../runtime/invocation/next (long-poll).
func (s *Service) RuntimeNext(w http.ResponseWriter, r *http.Request) {
	fn := runtimeFunctionName(r.URL.Path)
	if fn == "" {
		writeFunctionError(w, ErrInvalidParameterValue, "FunctionName is required", http.StatusBadRequest)

		return
	}

	inv, err := s.broker.next(r.Context(), fn)
	if err != nil {
		// Client (handler) disconnected or shutting down.
		return
	}

	// As on AWS, the deadline is when the function times out: its Timeout
	// counted from the moment the handler picks up the invocation.
	// https://docs.aws.amazon.com/lambda/latest/dg/runtimes-api.html#runtimes-api-next
	deadline := time.Now().Add(inv.timeout).UnixMilli()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Lambda-Runtime-Aws-Request-Id", inv.id)
	w.Header().Set("Lambda-Runtime-Deadline-Ms", strconv.FormatInt(deadline, 10))
	w.Header().Set("Lambda-Runtime-Invoked-Function-Arn", "arn:aws:lambda:local:000000000000:function:"+fn)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(inv.payload)
}

// RuntimeResponse handles POST .../runtime/invocation/{requestId}/response.
func (s *Service) RuntimeResponse(w http.ResponseWriter, r *http.Request) {
	s.runtimeResult(w, r, false)
}

// RuntimeError handles POST .../runtime/invocation/{requestId}/error.
func (s *Service) RuntimeError(w http.ResponseWriter, r *http.Request) {
	s.runtimeResult(w, r, true)
}

func (s *Service) runtimeResult(w http.ResponseWriter, r *http.Request, errored bool) {
	fn := runtimeFunctionName(r.URL.Path)
	id := runtimeRequestID(r.URL.Path)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeFunctionError(w, ErrInvalidParameterValue, "failed to read body", http.StatusBadRequest)

		return
	}

	s.broker.respond(fn, id, body, errored)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}

// RuntimeInitError handles POST .../runtime/init/error (best-effort).
func (s *Service) RuntimeInitError(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}

// runtimeFunctionName extracts {functionName} from a /_runtime/{fn}/... path.
func runtimeFunctionName(path string) string {
	return segmentAfter(path, "_runtime")
}

// runtimeRequestID extracts {requestId} from a .../invocation/{id}/... path.
func runtimeRequestID(path string) string {
	return segmentAfter(path, "invocation")
}

func segmentAfter(path, marker string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == marker && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}
