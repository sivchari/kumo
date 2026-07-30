package sfn

import (
	"sync"

	"github.com/google/uuid"
)

// Task token error codes, matching AWS's documented SendTaskSuccess/
// SendTaskFailure/SendTaskHeartbeat errors: TaskDoesNotExist ("the activity
// does not exist") is a token kumo has never seen; TaskTimedOut ("the task
// token has either expired or the task associated with the token has
// already been closed") is a token that was valid but has since resolved
// (success, failure, or its own timeout) or already been reported on.
const (
	tokenErrDoesNotExist = "TaskDoesNotExist"
	tokenErrTimedOut     = "TaskTimedOut"
)

// callbackResult carries the outcome delivered by SendTaskSuccess or
// SendTaskFailure for a single waiting task token. err is nil for a
// success; for a failure it is a failStateError carrying the reported
// Error/Cause, so it flows through Retry/Catch exactly like any other named
// error.
type callbackResult struct {
	output string
	err    error
}

// pendingCallback is a single task token's wait state, shared between the
// executor goroutine blocked on it (see awaitTaskToken in callback_task.go
// and executeActivityTask in activity_task.go) and the SendTaskSuccess/
// SendTaskFailure/SendTaskHeartbeat handlers that resolve it. done and
// heartbeat are both buffered so a resolving call never blocks on a waiter
// that has already stopped listening (e.g. TimeoutSeconds expired first).
type pendingCallback struct {
	done      chan callbackResult
	heartbeat chan struct{}
}

// tokenRegistry tracks every task token currently awaited by a callback
// Task state (.waitForTaskToken) or an activity Task state, and the
// executor goroutines waiting on them. It is entirely in-memory: waiting
// tokens do not need to survive a kumo restart, since the execution
// goroutine SendTaskSuccess/Failure ultimately resumes does not survive one
// either.
//
// The registry's own mutex is intentionally separate from
// MemoryStorage.mu: an executor goroutine blocks on pendingCallback.done
// without holding any lock (see awaitTaskToken), and every registry method
// below only ever holds this mutex for the duration of a single map
// operation, so SendTaskSuccess/Failure/Heartbeat requests are never
// blocked behind a long-running execution.
type tokenRegistry struct {
	mu     sync.Mutex
	active map[string]*pendingCallback
	// closed records every token that has ever left active, so a late
	// SendTaskSuccess/Failure/Heartbeat for it reports TaskTimedOut instead
	// of TaskDoesNotExist, which AWS reserves for a token that never
	// existed.
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

// release removes token from the active set without resolving it, moving it
// to closed. Callers use this when they abandon a token before any
// SendTaskSuccess/Failure resolves it (a callback integration call that
// itself failed, or the wait timing out), so a late SendTaskSuccess/Failure
// for that token correctly reports TaskTimedOut rather than succeeding.
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
