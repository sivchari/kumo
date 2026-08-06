package sfn

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// resourceStartExecutionBase is the Task Resource for starting a nested
// execution of another (or the same) state machine.
// startExecutionSyncSuffix/startExecutionSyncV2Suffix are its "Run a Job"
// (.sync) variants; unlike most optimized integrations, startExecution does
// NOT support .waitForTaskToken (see classifyStartExecutionResource).
const (
	resourceStartExecutionBase  = "arn:aws:states:::states:startExecution"
	startExecutionSyncSuffix    = ".sync"
	startExecutionSyncV2Suffix  = ".sync:2"
	resourceStartExecutionSync  = resourceStartExecutionBase + startExecutionSyncSuffix
	resourceStartExecutionSync2 = resourceStartExecutionBase + startExecutionSyncV2Suffix
)

// nestedExecutionMaxDepth caps how many states:startExecution calls may
// nest, guarding against a state machine recursively starting itself. Real
// AWS bounds this indirectly via running-execution quotas kumo doesn't
// model; this fixed depth is an emulator-only substitute.
const nestedExecutionMaxDepth = 5

// nestedExecutionPollInterval is how often a .sync/.sync:2 Task polls the
// child execution's status via DescribeExecution while waiting for it to
// complete.
const nestedExecutionPollInterval = 20 * time.Millisecond

// startExecutionVariant identifies which of the three
// arn:aws:states:::states:startExecution* integration patterns a Task
// Resource selects.
type startExecutionVariant int

const (
	// startExecutionAsync is the plain (fire-and-forget) integration: the
	// Task's output is {ExecutionArn, StartDate} and the Task completes as
	// soon as the child execution starts, without waiting for it.
	startExecutionAsync startExecutionVariant = iota
	// startExecutionSync is the .sync ("Run a Job") variant: the Task waits
	// for the child execution to complete, and its output mirrors
	// DescribeExecution with Output left as a JSON string.
	startExecutionSync
	// startExecutionSyncV2 is the .sync:2 variant: identical to
	// startExecutionSync except Output is parsed JSON rather than a string.
	startExecutionSyncV2
)

// classifyStartExecutionResource identifies which
// arn:aws:states:::states:startExecution* variant resource selects. ok is
// false for any other suffix (including .waitForTaskToken, which AWS does
// not support for this integration), so callers reject it explicitly
// instead of silently treating it as fire-and-forget.
func classifyStartExecutionResource(resource string) (variant startExecutionVariant, ok bool) {
	switch resource {
	case resourceStartExecutionBase:
		return startExecutionAsync, true
	case resourceStartExecutionSync:
		return startExecutionSync, true
	case resourceStartExecutionSync2:
		return startExecutionSyncV2, true
	default:
		return startExecutionAsync, false
	}
}

// nestedExecutionDepthKey is the context.Context key carrying the current
// nesting depth through one execution's synchronous call graph. It cannot
// cross into a child execution's background goroutine (StartExecution
// intentionally ignores its caller's context, since an execution must
// outlive the request that started it), so depth is instead passed
// explicitly to MemoryStorage.startExecutionAtDepth/runExecution.
type nestedExecutionDepthKey struct{}

// withNestedExecutionDepth returns a context carrying depth as the current
// nesting depth for states:startExecution calls made within it.
func withNestedExecutionDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, nestedExecutionDepthKey{}, depth)
}

// nestedExecutionDepthFromContext reads the current nesting depth from ctx,
// defaulting to 0 (a top-level execution) if none was installed.
func nestedExecutionDepthFromContext(ctx context.Context) int {
	depth, _ := ctx.Value(nestedExecutionDepthKey{}).(int)

	return depth
}

// executionStarter is the subset of Storage the states:startExecution
// integration needs. executionEngine calls back into the same MemoryStorage
// that constructed it, rather than looping an HTTP request the way
// SQS/Lambda/S3 integrations do: this recurses into kumo's own SFN engine,
// and the recursion-depth guard needs to travel via context.Context, which
// an HTTP round trip would complicate for no benefit.
type executionStarter interface {
	// startNestedExecution is StartExecution's counterpart for a nested
	// call: depth is threaded explicitly since context values do not
	// survive StartExecution's intentional context detachment.
	startNestedExecution(ctx context.Context, stateMachineArn, name, input string, depth int) (*Execution, error)
	DescribeExecution(ctx context.Context, executionArn string) (*Execution, error)
}

// executeStartExecutionTask executes a Task state whose Resource is
// arn:aws:states:::states:startExecution (optionally .sync/.sync:2): starts
// a child execution, then for .sync/.sync:2 waits for it and reports a
// child failure as this Task's own States.TaskFailed.
func (e *executionEngine) executeStartExecutionTask(ctx context.Context, resource string, params map[string]any) (string, error) {
	variant, ok := classifyStartExecutionResource(resource)
	if !ok {
		return "", fmt.Errorf("unsupported task resource %q", resource)
	}

	if e.starter == nil {
		return "", fmt.Errorf("states:startExecution: nested execution support is not available")
	}

	stateMachineArn, _ := params["StateMachineArn"].(string)
	if stateMachineArn == "" {
		return "", fmt.Errorf("states:startExecution: Parameters.StateMachineArn is required")
	}

	childInput, err := marshalStartExecutionInput(params["Input"])
	if err != nil {
		return "", fmt.Errorf("states:startExecution: %w", err)
	}

	execName, _ := params["Name"].(string)

	depth := nestedExecutionDepthFromContext(ctx)
	if depth >= nestedExecutionMaxDepth {
		return "", fmt.Errorf(
			"states:startExecution: nesting depth exceeded kumo's emulator-only bound of %d; this guards against a state machine recursively starting itself (real AWS instead limits this indirectly via account running-execution quotas)",
			nestedExecutionMaxDepth,
		)
	}

	child, err := e.starter.startNestedExecution(ctx, stateMachineArn, execName, childInput, depth+1)
	if err != nil {
		return "", fmt.Errorf("states:startExecution: %w", err)
	}

	if variant == startExecutionAsync {
		return marshalStartExecutionAsyncOutput(child)
	}

	completed, err := e.awaitChildExecution(ctx, child.ExecutionArn)
	if err != nil {
		return "", fmt.Errorf("states:startExecution%s: %w", startExecutionSuffixFor(variant), err)
	}

	if completed.Status != ExecutionStatusSucceeded {
		return "", fmt.Errorf(
			"child execution %q ended with status %s: %s: %s",
			completed.ExecutionArn, completed.Status, completed.Error, completed.Cause,
		)
	}

	if variant == startExecutionSyncV2 {
		return marshalSyncV2ExecutionOutput(completed)
	}

	return marshalSyncExecutionOutput(completed)
}

// startExecutionSuffixFor returns the Resource suffix for variant, for
// error messages.
func startExecutionSuffixFor(variant startExecutionVariant) string {
	if variant == startExecutionSyncV2 {
		return startExecutionSyncV2Suffix
	}

	return startExecutionSyncSuffix
}

// awaitChildExecution polls DescribeExecution until executionArn leaves
// RUNNING, or ctx is done -- the Task's TimeoutSeconds/Retry/Catch context
// applies here like any other Task invocation.
func (e *executionEngine) awaitChildExecution(ctx context.Context, executionArn string) (*Execution, error) {
	ticker := time.NewTicker(nestedExecutionPollInterval)
	defer ticker.Stop()

	for {
		exec, err := e.starter.DescribeExecution(ctx, executionArn)
		if err != nil {
			return nil, fmt.Errorf("describe child execution %q: %w", executionArn, err)
		}

		if exec.Status != ExecutionStatusRunning {
			return exec, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for child execution %q: %w", executionArn, ctx.Err())
		case <-ticker.C:
		}
	}
}

// marshalStartExecutionInput builds a child execution's input JSON from
// Parameters.Input; absent input defaults to "{}", matching AWS's
// StartExecution requirement that input always be at least an empty object.
func marshalStartExecutionInput(v any) (string, error) {
	if v == nil {
		return "{}", nil
	}

	encoded, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal Input: %w", err)
	}

	return string(encoded), nil
}

// marshalStartExecutionAsyncOutput builds the plain (fire-and-forget)
// startExecution Task's output: {executionArn, startDate}, matching AWS's
// StartExecution API response shape.
func marshalStartExecutionAsyncOutput(exec *Execution) (string, error) {
	encoded, err := json.Marshal(map[string]any{
		"executionArn": exec.ExecutionArn,
		"startDate":    float64(exec.StartDate.Unix()),
	})
	if err != nil {
		return "", fmt.Errorf("marshal states:startExecution output: %w", err)
	}

	return string(encoded), nil
}

// describeExecutionEnvelope builds the DescribeExecution-shaped fields
// shared by both .sync and .sync:2 output (everything except "output",
// which differs between the two -- see marshalSyncExecutionOutput/
// marshalSyncV2ExecutionOutput).
func describeExecutionEnvelope(exec *Execution) map[string]any {
	env := map[string]any{
		"executionArn":    exec.ExecutionArn,
		"stateMachineArn": exec.StateMachineArn,
		"name":            exec.Name,
		"status":          string(exec.Status),
		"startDate":       float64(exec.StartDate.Unix()),
		"input":           exec.Input,
	}

	if exec.StopDate != nil {
		env["stopDate"] = float64(exec.StopDate.Unix())
	}

	return env
}

// marshalSyncExecutionOutput builds the .sync Task's output: the
// DescribeExecution-style envelope with "output" left as a JSON string,
// exactly as the child execution itself reports it.
func marshalSyncExecutionOutput(exec *Execution) (string, error) {
	env := describeExecutionEnvelope(exec)
	env["output"] = exec.Output

	encoded, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("marshal states:startExecution.sync output: %w", err)
	}

	return string(encoded), nil
}

// marshalSyncV2ExecutionOutput is marshalSyncExecutionOutput's .sync:2
// counterpart: "output" holds the child execution's output as parsed JSON
// rather than a string.
func marshalSyncV2ExecutionOutput(exec *Execution) (string, error) {
	env := describeExecutionEnvelope(exec)

	if exec.Output != "" {
		env["output"] = json.RawMessage(exec.Output)
	}

	encoded, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("marshal states:startExecution.sync:2 output: %w", err)
	}

	return string(encoded), nil
}
