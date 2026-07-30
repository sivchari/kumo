package sfn

import (
	"context"
	"fmt"
	"time"
)

// callbackResourceSuffix marks a Task state's Resource as using the "Wait
// for a Callback with Task Token" service integration pattern
// (.waitForTaskToken), per
// https://docs.aws.amazon.com/step-functions/latest/dg/connect-to-resource.html.
const callbackResourceSuffix = ".waitForTaskToken"

// callbackIntegrations are the optimized service integrations kumo supports
// with .waitForTaskToken appended. AWS documents Lambda and SQS (among
// others) as supporting the "Wait for a Callback with Task Token" pattern;
// kumo implements the two integrations it already supports without the
// suffix (see executeTaskState in executor.go).
var callbackIntegrations = map[string]bool{
	"arn:aws:states:::lambda:invoke":   true,
	"arn:aws:states:::sqs:sendMessage": true,
}

// isCallbackResource reports whether resource uses the .waitForTaskToken
// integration pattern, returning the base resource (the integration ARN
// with the suffix stripped) when it does.
func isCallbackResource(resource string) (baseResource string, ok bool) {
	if len(resource) <= len(callbackResourceSuffix) {
		return "", false
	}

	suffixStart := len(resource) - len(callbackResourceSuffix)
	if resource[suffixStart:] != callbackResourceSuffix {
		return "", false
	}

	return resource[:suffixStart], true
}

// executeCallbackTask executes a Task state whose Resource uses the "Wait
// for a Callback with Task Token" pattern: it generates a task token,
// injects it into the resolved Parameters via the Context object
// ($$.Task.Token), fires the underlying integration once, and then blocks
// until SendTaskSuccess/SendTaskFailure resolves the token or the wait
// times out. TimeoutSeconds bounds the whole wait; HeartbeatSeconds, if
// set, must see a SendTaskHeartbeat within every interval or the wait times
// out early -- both report States.Timeout, per AWS's documentation for this
// pattern.
func (e *executionEngine) executeCallbackTask(ctx context.Context, name string, state *stateDefinition, baseResource, input string) (string, error) {
	if !callbackIntegrations[baseResource] {
		return "", fmt.Errorf("unsupported %s resource %q", callbackResourceSuffix, baseResource)
	}

	token, pending := e.tokens.register()

	params, err := resolveParametersWithContext(state.Parameters, input, taskTokenContext(token))
	if err != nil {
		e.tokens.release(token)

		return "", fmt.Errorf("resolve parameters: %w", err)
	}

	if err := e.fireCallbackIntegration(ctx, baseResource, params); err != nil {
		e.tokens.release(token)

		return "", err
	}

	return e.awaitTaskToken(ctx, name, token, state, pending)
}

// taskTokenContext builds the Context object subset ($$.Task.Token) a
// callback/activity Task state's Parameters may reference via
// resolveParametersWithContext. kumo does not model the rest of the
// Context object ($$.Execution, $$.State, ...); see mapItemContext in
// itemselector.go for the only other place the Context object is built.
func taskTokenContext(token string) map[string]any {
	return map[string]any{
		"Task": map[string]any{"Token": token},
	}
}

// fireCallbackIntegration performs the "fire" half of a callback task: the
// underlying integration call itself, discarding its response, since a
// callback task's real output comes from SendTaskSuccess, not the
// integration's own response.
func (e *executionEngine) fireCallbackIntegration(ctx context.Context, baseResource string, params map[string]any) error {
	switch baseResource {
	case "arn:aws:states:::sqs:sendMessage":
		_, err := e.executeSQSSendMessage(ctx, params)

		return err
	case "arn:aws:states:::lambda:invoke":
		_, err := e.executeLambdaInvoke(ctx, params)

		return err
	default:
		return fmt.Errorf("unsupported %s resource %q", callbackResourceSuffix, baseResource)
	}
}

// awaitTaskToken blocks until token is resolved by SendTaskSuccess/
// SendTaskFailure, a HeartbeatSeconds interval elapses with no
// SendTaskHeartbeat, or ctx is done (TimeoutSeconds, or an outer
// execution-level deadline). All three timeout paths report the same
// taskTimeoutError (see timeout.go), which executionErrorCode surfaces as
// States.Timeout; on any of them, token is released so a
// SendTaskSuccess/Failure that arrives afterward correctly reports
// TaskTimedOut instead of resolving a state that has already failed.
func (e *executionEngine) awaitTaskToken(ctx context.Context, name, token string, state *stateDefinition, pending *pendingCallback) (string, error) {
	heartbeat := heartbeatInterval(state)

	var heartbeatTimer *time.Timer

	var heartbeatC <-chan time.Time

	if heartbeat > 0 {
		heartbeatTimer = time.NewTimer(heartbeat)
		defer heartbeatTimer.Stop()

		heartbeatC = heartbeatTimer.C
	}

	for {
		select {
		case result := <-pending.done:
			return result.output, result.err
		case <-pending.heartbeat:
			heartbeatTimer.Reset(heartbeat)
		case <-ctx.Done():
			return e.timeoutOrLateResult(name, token, pending)
		case <-heartbeatC:
			return e.timeoutOrLateResult(name, token, pending)
		}
	}
}

// timeoutOrLateResult favors a callbackResult that arrived in the same
// instant a timeout fired -- select does not prioritize ready cases -- over
// reporting a spurious States.Timeout, then releases token.
func (e *executionEngine) timeoutOrLateResult(name, token string, pending *pendingCallback) (string, error) {
	select {
	case result := <-pending.done:
		return result.output, result.err
	default:
	}

	e.tokens.release(token)

	return "", &taskTimeoutError{state: name}
}

// heartbeatInterval resolves a Task state's HeartbeatSeconds, returning
// zero if unset. HeartbeatSecondsPath is decoded (see stateDefinition in
// retry.go) but not implemented -- it is a dynamic reference-path form AWS
// documents far less prominently than the static field.
func heartbeatInterval(state *stateDefinition) time.Duration {
	if state.HeartbeatSeconds == nil {
		return 0
	}

	return time.Duration(*state.HeartbeatSeconds) * time.Second
}
