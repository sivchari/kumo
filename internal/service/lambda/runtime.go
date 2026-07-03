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

// runtimeInvokeTimeout bounds how long an invocation waits to be picked up by
// a polling handler and to receive its response.
const runtimeInvokeTimeout = 30 * time.Second

// errRuntimeTimeout is returned when no polling handler services an
// invocation within runtimeInvokeTimeout.
var errRuntimeTimeout = errors.New("no runtime handler available")

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

// invoke hands an invocation to a polling handler. For async it returns once
// queued; for sync it waits for the handler's response.
func (b *runtimeBroker) invoke(ctx context.Context, fn string, payload []byte, async bool) (runtimeResult, error) {
	fr := b.get(fn)
	inv := &runtimeInvocation{id: uuid.New().String(), payload: payload}

	if async {
		go func() {
			select {
			case fr.invocations <- inv:
			case <-time.After(runtimeInvokeTimeout):
			}
		}()

		return runtimeResult{}, nil
	}

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
	case <-time.After(runtimeInvokeTimeout):
		return runtimeResult{}, errRuntimeTimeout
	}

	select {
	case res := <-resCh:
		return res, nil
	case <-ctx.Done():
		return runtimeResult{}, fmt.Errorf("invocation canceled: %w", ctx.Err())
	case <-time.After(runtimeInvokeTimeout):
		return runtimeResult{}, errRuntimeTimeout
	}
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

	// The aws-lambda-go runtime requires a parseable deadline header.
	deadline := time.Now().Add(runtimeInvokeTimeout).UnixMilli()

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
