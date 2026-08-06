package sfn

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// taskTimeoutError marks a Task invocation that exceeded its
// TimeoutSeconds/TimeoutSecondsPath. Unlike States.Runtime, it reports as
// States.Timeout, an ordinary named error Retry/Catch can match.
type taskTimeoutError struct {
	state string
}

func (e *taskTimeoutError) Error() string {
	return fmt.Sprintf("state %q: task execution exceeded TimeoutSeconds", e.state)
}

// executeTaskStateWithTimeout wraps one Task invocation attempt in a context
// bounded by TimeoutSeconds/TimeoutSecondsPath. Called once per Retry
// attempt, so the timeout bounds a single attempt, not the retry budget.
func (e *executionEngine) executeTaskStateWithTimeout(ctx context.Context, name string, state *stateDefinition, input string) (string, error) {
	timeout, err := taskTimeout(state, input)
	if err != nil {
		return "", fmt.Errorf("task state %q: %w", name, err)
	}

	if timeout <= 0 {
		return e.executeTaskState(ctx, name, state, input)
	}

	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := e.executeTaskState(taskCtx, name, state, input)
	if err != nil && errors.Is(taskCtx.Err(), context.DeadlineExceeded) {
		return "", &taskTimeoutError{state: name}
	}

	return output, err
}

// taskTimeout resolves the timeout from TimeoutSeconds/TimeoutSecondsPath
// (path preferred). Returns zero (no timeout context) when neither is set,
// since the spec's default of 99999999 seconds is effectively unbounded.
func taskTimeout(state *stateDefinition, input string) (time.Duration, error) {
	switch {
	case state.TimeoutSecondsPath != "":
		seconds, err := resolveNumberPath(state.TimeoutSecondsPath, input)
		if err != nil {
			return 0, fmt.Errorf("resolve TimeoutSecondsPath: %w", err)
		}

		return time.Duration(seconds * float64(time.Second)), nil

	case state.TimeoutSeconds != nil:
		return time.Duration(*state.TimeoutSeconds) * time.Second, nil

	default:
		return 0, nil
	}
}
