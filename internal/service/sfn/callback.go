package sfn

import (
	"sync"

	"github.com/google/uuid"
)

// Task token error codes: TaskDoesNotExist is a token kumo has never seen;
// TaskTimedOut is one that was valid but has since resolved or already
// been reported on.
const (
	tokenErrDoesNotExist = "TaskDoesNotExist"
	tokenErrTimedOut     = "TaskTimedOut"
)

// callbackResult carries the outcome of SendTaskSuccess/SendTaskFailure
// for a waiting task token. err is nil on success; on failure it is a
// failStateError so it flows through Retry/Catch like any named error.
type callbackResult struct {
	output string
	err    error
}

// pendingCallback is a single task token's wait state, shared between the
// blocked executor goroutine and the SendTaskSuccess/Failure/Heartbeat
// handlers that resolve it. done and heartbeat are buffered so a resolving
// call never blocks on a waiter that already stopped listening.
type pendingCallback struct {
	done      chan callbackResult
	heartbeat chan struct{}
}

// tokenRegistry tracks every task token currently awaited by a callback or
// activity Task state. It is in-memory only: waiting tokens need not
// survive a restart since the execution goroutine they resume doesn't
// either.
//
// Its mutex is intentionally separate from MemoryStorage.mu and is only
// ever held for a single map operation, so SendTaskSuccess/Failure/
// Heartbeat requests are never blocked behind a long-running execution.
type tokenRegistry struct {
	mu     sync.Mutex
	active map[string]*pendingCallback
	// closed records every token that has ever left active, so a late
	// request for it reports TaskTimedOut instead of TaskDoesNotExist.
	closed map[string]bool
}

// newTokenRegistry creates an empty tokenRegistry.
func newTokenRegistry() *tokenRegistry {
	return &tokenRegistry{
		active: make(map[string]*pendingCallback),
		closed: make(map[string]bool),
	}
}

// register creates a new task token and its pendingCallback, returning both
// for the caller to inject the token into the integration's request payload
// (or GetActivityTaskResponse) and then wait on the pendingCallback.
func (r *tokenRegistry) register() (string, *pendingCallback) {
	token := uuid.New().String()
	pending := &pendingCallback{
		done:      make(chan callbackResult, 1),
		heartbeat: make(chan struct{}, 1),
	}

	r.mu.Lock()
	r.active[token] = pending
	r.mu.Unlock()

	return token, pending
}

// release moves token from active to closed without resolving it, for
// when a token is abandoned before SendTaskSuccess/Failure (integration
// call failed, or the wait timed out) so a late call reports TaskTimedOut.
func (r *tokenRegistry) release(token string) {
	r.mu.Lock()
	delete(r.active, token)
	r.closed[token] = true
	r.mu.Unlock()
}

// resolve delivers result to token's waiter and marks the token closed. It
// returns "" on success, or tokenErrDoesNotExist/tokenErrTimedOut if token
// is not currently active.
func (r *tokenRegistry) resolve(token string, result callbackResult) string {
	r.mu.Lock()

	pending, ok := r.active[token]
	if !ok {
		code := r.notActiveCodeLocked(token)

		r.mu.Unlock()

		return code
	}

	delete(r.active, token)
	r.closed[token] = true

	r.mu.Unlock()

	pending.done <- result

	return ""
}

// heartbeatToken signals token's waiter to reset its heartbeat deadline. It
// reports tokenErrDoesNotExist/tokenErrTimedOut the same way resolve does.
func (r *tokenRegistry) heartbeatToken(token string) string {
	r.mu.Lock()

	pending, ok := r.active[token]
	if !ok {
		code := r.notActiveCodeLocked(token)

		r.mu.Unlock()

		return code
	}

	r.mu.Unlock()

	// If a heartbeat is already pending delivery, the waiter will observe it
	// and reset its timer, so this one is redundant.
	select {
	case pending.heartbeat <- struct{}{}:
	default:
	}

	return ""
}

// notActiveCodeLocked reports which error a token not currently in active
// should surface. Callers must hold r.mu.
func (r *tokenRegistry) notActiveCodeLocked(token string) string {
	if r.closed[token] {
		return tokenErrTimedOut
	}

	return tokenErrDoesNotExist
}
