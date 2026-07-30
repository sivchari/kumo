package sfn

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// isActivityResource reports whether resource is an activity ARN created
// via CreateActivity (arn:aws:states:{region}:{account}:activity:{name}),
// as opposed to the arn:aws:states:::service:action shape optimized
// integrations use, which always leaves the region/account segments empty.
func isActivityResource(resource string) bool {
	return strings.HasPrefix(resource, "arn:aws:states:") && strings.Contains(resource, ":activity:")
}

// activityQueueRegistry lazily creates one activityQueue per activity ARN,
// so each activity's pending tasks and waiting pollers are independent of
// every other activity's.
type activityQueueRegistry struct {
	mu     sync.Mutex
	queues map[string]*activityQueue
}

// newActivityQueueRegistry creates an empty activityQueueRegistry.
func newActivityQueueRegistry() *activityQueueRegistry {
	return &activityQueueRegistry{queues: make(map[string]*activityQueue)}
}

// get returns arn's activityQueue, creating it on first use.
func (r *activityQueueRegistry) get(arn string) *activityQueue {
	r.mu.Lock()
	defer r.mu.Unlock()

	q, ok := r.queues[arn]
	if !ok {
		q = newActivityQueue()
		r.queues[arn] = q
	}

	return q
}

// executeActivityTask executes a Task state whose Resource references an
// activity created via CreateActivity: it resolves the task's input
// (resolved Parameters if set, else the effective input verbatim -- the
// same fallback executeLambdaFunctionTask in executor.go uses for the
// directly-specified-function integration), enqueues it for a
// GetActivityTask poller, and then waits for SendTaskSuccess/SendTaskFailure
// exactly like a callback Task state (see awaitTaskToken in
// callback_task.go), honoring the same TimeoutSeconds/HeartbeatSeconds
// semantics.
func (e *executionEngine) executeActivityTask(ctx context.Context, name string, state *stateDefinition, resource, input string) (string, error) {
	taskInput := input

	if len(state.Parameters) > 0 {
		params, err := resolveParameters(state.Parameters, input)
		if err != nil {
			return "", fmt.Errorf("resolve parameters for state %q: %w", name, err)
		}

		marshaled, err := json.Marshal(params)
		if err != nil {
			return "", fmt.Errorf("marshal parameters for state %q: %w", name, err)
		}

		taskInput = string(marshaled)
	}

	token, pending := e.tokens.register()

	e.activityQueues.get(resource).enqueue(&activityTask{token: token, input: taskInput, pending: pending})

	return e.awaitTaskToken(ctx, name, token, state, pending)
}

// activityPollTimeout bounds GetActivityTask's long poll; AWS documents 60
// seconds as the maximum time the service holds the request open.
const activityPollTimeout = 60 * time.Second

// GetActivityTask long-polls for a task scheduled against activityArn,
// returning an empty taskToken if none arrives within the poll window --
// per AWS's documented behavior of responding with "a taskToken with a
// null string" once the 60-second poll elapses.
func (s *MemoryStorage) GetActivityTask(ctx context.Context, activityArn, _ string) (string, string, error) {
	s.mu.RLock()
	_, exists := s.Activities[activityArn]
	s.mu.RUnlock()

	if !exists {
		return "", "", &ServiceError{Code: errActivityDoesNotExist, Message: "Activity does not exist"}
	}

	task := s.engine.activityQueues.get(activityArn).poll(ctx, s.engine.activityPollTimeout)
	if task == nil {
		return "", "", nil
	}

	return task.token, task.input, nil
}

// SendTaskSuccess resolves taskToken with output, resuming the callback or
// activity Task state waiting on it.
func (s *MemoryStorage) SendTaskSuccess(_ context.Context, taskToken, output string) error {
	return tokenServiceError(s.engine.tokens.resolve(taskToken, callbackResult{output: output}))
}

// SendTaskFailure resolves taskToken with a failure, which the waiting
// state's Retry/Catch can match like any other named error.
func (s *MemoryStorage) SendTaskFailure(_ context.Context, taskToken, errorName, cause string) error {
	result := callbackResult{err: &failStateError{errorName: errorName, cause: cause}}

	return tokenServiceError(s.engine.tokens.resolve(taskToken, result))
}

// SendTaskHeartbeat resets taskToken's HeartbeatSeconds deadline.
func (s *MemoryStorage) SendTaskHeartbeat(_ context.Context, taskToken string) error {
	return tokenServiceError(s.engine.tokens.heartbeatToken(taskToken))
}

// tokenServiceError turns a tokenRegistry result code (tokenErrDoesNotExist,
// tokenErrTimedOut, or "" for success) into the ServiceError handlers.go
// expects, matching AWS's documented SendTaskSuccess/SendTaskFailure/
// SendTaskHeartbeat error messages.
func tokenServiceError(code string) error {
	switch code {
	case "":
		return nil
	case tokenErrTimedOut:
		return &ServiceError{Code: code, Message: "The task token has either expired or the task associated with the token has already been closed"}
	default:
		return &ServiceError{Code: code, Message: "The activity does not exist"}
	}
}
