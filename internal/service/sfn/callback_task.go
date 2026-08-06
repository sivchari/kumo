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

// callbackIntegrations are the optimized service integrations kumo
// supports with .waitForTaskToken appended (AWS supports more; kumo only
// implements the two integrations it already has without the suffix).
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

// executeCallbackTask executes a .waitForTaskToken Task state: injects a
// task token into Parameters via $$.Task.Token, fires the integration
// once, then blocks until SendTaskSuccess/SendTaskFailure or a
// TimeoutSeconds/HeartbeatSeconds timeout (both report States.Timeout).
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

// taskTokenContext builds the $$.Task.Token Context object subset a
// callback/activity Task state's Parameters may reference. kumo does not
// model the rest of the Context object ($$.Execution, $$.State, ...).
func taskTokenContext(token string) map[string]any {
	return map[string]any{
		"Task": map[string]any{"Token": token},
	}
}

// fireCallbackIntegration performs the "fire" half of a callback task,
// discarding the integration's response since the real output comes from
// SendTaskSuccess, not this call.
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

// awaitTaskToken blocks until token is resolved, a HeartbeatSeconds
// interval elapses with no heartbeat, or ctx is done. All timeout paths
// release token, so a late SendTaskSuccess/Failure correctly reports
// TaskTimedOut instead of resolving an already-failed state.
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
// zero if unset. HeartbeatSecondsPath is decoded but not implemented.
func heartbeatInterval(state *stateDefinition) time.Duration {
	if state.HeartbeatSeconds == nil {
		return 0
	}

	return time.Duration(*state.HeartbeatSeconds) * time.Second
}
