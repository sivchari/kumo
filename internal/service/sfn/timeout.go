package sfn

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// taskTimeoutError marks a Task state whose invocation attempt exceeded its
// TimeoutSeconds/TimeoutSecondsPath. Per the Amazon States Language spec
// this reports as States.Timeout which, unlike States.Runtime, is an
// ordinary named error: Retry/Catch can match it like any other Error Name.
type taskTimeoutError struct {
	state string
}

func (e *taskTimeoutError) Error() string {
	return fmt.Sprintf("state %q: task execution exceeded TimeoutSeconds", e.state)
}

// executeTaskStateWithTimeout wraps a single Task state invocation attempt
// in a context bounded by TimeoutSeconds/TimeoutSecondsPath. It is called
// once per Retry attempt (see executeTaskStateWithPolicy), so each attempt
// gets its own fresh timeout window: TimeoutSeconds bounds one execution
// attempt of the task, not the state's total retry budget.
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

// taskTimeout resolves a Task state's timeout duration from TimeoutSeconds
// or TimeoutSecondsPath, preferring the path form when both are set. It
// returns zero -- meaning "do not wrap the call in a timeout context at all"
// -- when neither field is set, since the spec's default of 99999999
// seconds (~3.17 years) is effectively unbounded.
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
