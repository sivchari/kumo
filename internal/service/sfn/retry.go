package sfn

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// retrier is a single entry in a Task/Parallel/Map state's Retry field.
//
// JSON tags are lowerCamelCase rather than the Amazon States Language's own
// PascalCase (e.g. "ErrorEquals", "IntervalSeconds"); encoding/json matches
// object keys to struct fields case-insensitively when there is no exact
// match, so definitions using the spec's PascalCase field names still decode
// correctly.
type retrier struct {
	ErrorEquals     []string `json:"errorEquals"`
	IntervalSeconds *int     `json:"intervalSeconds"`
	MaxAttempts     *int     `json:"maxAttempts"`
	BackoffRate     *float64 `json:"backoffRate"`
	MaxDelaySeconds *int     `json:"maxDelaySeconds"`

	// JitterStrategy is accepted but not implemented: kumo is a
	// deterministic emulator, and randomizing retry delays would make retry
	// timing (and therefore test timing) non-reproducible.
	JitterStrategy string `json:"jitterStrategy"`
}

// catcher is a single entry in a Task/Parallel/Map state's Catch field. See
// retrier for why its JSON tags are lowerCamelCase.
type catcher struct {
	ErrorEquals []string `json:"errorEquals"`
	Next        string   `json:"next"`

	// ResultPath is left as raw JSON so an explicit "null" (discard the
	// Error Output, keep the original input) can be told apart from the
	// field being absent (default "$", full replacement by the Error
	// Output); both unmarshal to the same value through a plain *string.
	ResultPath json.RawMessage `json:"resultPath"`
}

// Retry defaults per the Amazon States Language spec.
const (
	defaultRetryIntervalSeconds = 1
	defaultRetryMaxAttempts     = 3
	defaultRetryBackoffRate     = 2.0
)

// runWithRetryCatch executes fn (a state's own work for one execution),
// applying that state's Retry and Catch policy: retries are attempted
// first, using the matching Retrier's backoff schedule; if retries do not
// resolve the error, or no Retrier matches, the first matching Catcher
// routes to its Next state with the Catcher's Error Output as input. The
// return shape mirrors executeState: nextOverride is non-empty only when a
// Catcher matched.
func (e *executionEngine) runWithRetryCatch(ctx context.Context, name string, state *stateDefinition, input string, fn func(context.Context) (string, error)) (string, string, error) {
	output, err := e.runWithRetry(ctx, state, fn)
	if err == nil {
		return output, "", nil
	}

	catchOutput, catchNext, matched, buildErr := applyCatch(state, input, err)
	if buildErr != nil {
		return "", "", fmt.Errorf("state %q: %w", name, buildErr)
	}

	if !matched {
		return "", "", err
	}

	return catchOutput, catchNext, nil
}

// runWithRetry executes fn, applying the state's Retry policy. Per the
// spec, the interpreter scans the Retriers in order and uses the first one
// whose ErrorEquals matches the reported error; each Retrier's attempt
// budget (MaxAttempts) is independent of the others, and the budgets reset
// whenever the state is entered anew, which holds naturally here since
// runWithRetry is scoped to a single state execution.
func (e *executionEngine) runWithRetry(ctx context.Context, state *stateDefinition, fn func(context.Context) (string, error)) (string, error) {
	attemptsUsed := make([]int, len(state.Retry))

	for {
		output, err := fn(ctx)
		if err == nil {
			return output, nil
		}

		idx := matchingRetrierIndex(state.Retry, executionErrorCode(err))
		if idx == -1 {
			return "", err
		}

		attemptsUsed[idx]++

		r := &state.Retry[idx]
		if attemptsUsed[idx] > retryMaxAttempts(r) {
			return "", err
		}

		if sleepErr := sleepForRetry(ctx, retryDelay(r, attemptsUsed[idx])); sleepErr != nil {
			return "", sleepErr
		}
	}
}

// matchingRetrierIndex returns the index of the first Retrier whose
// ErrorEquals matches errName, or -1 if none match.
func matchingRetrierIndex(retriers []retrier, errName string) int {
	for i := range retriers {
		if errorNameMatches(retriers[i].ErrorEquals, errName) {
			return i
		}
	}

	return -1
}

// errorNameMatches reports whether errName is matched by an ErrorEquals
// list from a Retrier or Catcher. States.Runtime is a special case: per AWS
// documentation it "isn't retriable, and will always cause the execution to
// fail" -- it is never matched, even by an explicit "States.Runtime" entry
// or by the States.ALL wildcard.
func errorNameMatches(errorEquals []string, errName string) bool {
	if errName == errorStatesRuntime {
		return false
	}

	for _, want := range errorEquals {
		if want == errorStatesAll || want == errName {
			return true
		}
	}

	return false
}

// retryMaxAttempts and retryBackoffRate apply the Retrier's spec defaults.
func retryMaxAttempts(r *retrier) int {
	if r.MaxAttempts != nil {
		return *r.MaxAttempts
	}

	return defaultRetryMaxAttempts
}

func retryBackoffRate(r *retrier) float64 {
	if r.BackoffRate != nil {
		return *r.BackoffRate
	}

	return defaultRetryBackoffRate
}

// retryIntervalSeconds applies the Retrier's spec default.
func retryIntervalSeconds(r *retrier) int {
	if r.IntervalSeconds != nil {
		return *r.IntervalSeconds
	}

	return defaultRetryIntervalSeconds
}

// retryDelay computes the sleep duration before retry attempt number
// attempt (1-based: the first retry is attempt 1), applying BackoffRate and,
// if set, capping at MaxDelaySeconds.
func retryDelay(r *retrier, attempt int) time.Duration {
	delay := float64(retryIntervalSeconds(r)) * math.Pow(retryBackoffRate(r), float64(attempt-1))

	if r.MaxDelaySeconds != nil && delay > float64(*r.MaxDelaySeconds) {
		delay = float64(*r.MaxDelaySeconds)
	}

	return time.Duration(delay * float64(time.Second))
}

// sleepForRetry sleeps for d, honoring ctx cancellation.
func sleepForRetry(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("retry wait: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// applyCatch scans the state's Catchers, in array order, for the first one
// whose ErrorEquals matches err's Error Name. matched is false if none did,
// in which case the caller must propagate err unchanged.
func applyCatch(state *stateDefinition, input string, err error) (output, next string, matched bool, buildErr error) {
	errName := executionErrorCode(err)
	cause := executionErrorCause(err)

	for i := range state.Catch {
		c := &state.Catch[i]
		if !errorNameMatches(c.ErrorEquals, errName) {
			continue
		}

		output, buildErr = buildCatchOutput(c.ResultPath, input, errName, cause)

		return output, c.Next, true, buildErr
	}

	return "", "", false, nil
}

// buildCatchOutput builds a Catcher's output: the Error Output object
// {"Error": errName, "Cause": cause}, optionally injected into the state's
// original input per ResultPath. The default (ResultPath absent) is "$",
// meaning the output is the Error Output alone; an explicit null ResultPath
// discards the Error Output and passes the original input through
// unchanged. Only "$" and single-level "$.field" paths are supported;
// anything deeper is reported as a plain error, which executionErrorCode
// surfaces as States.Runtime like any other unsupported definition
// construct.
func buildCatchOutput(rawResultPath json.RawMessage, input, errName, cause string) (string, error) {
	path, isNull, err := parseCatcherResultPath(rawResultPath)
	if err != nil {
		return "", err
	}

	if isNull {
		return input, nil
	}

	errorOutput := map[string]string{"Error": errName, "Cause": cause}

	if path == "$" {
		out, err := json.Marshal(errorOutput)
		if err != nil {
			return "", fmt.Errorf("marshal error output: %w", err)
		}

		return string(out), nil
	}

	return injectCatchResultPath(path, input, errorOutput)
}

// parseCatcherResultPath interprets a Catcher's raw ResultPath JSON.
func parseCatcherResultPath(raw json.RawMessage) (path string, isNull bool, err error) {
	if len(raw) == 0 {
		return "$", false, nil
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false, fmt.Errorf("parse ResultPath: %w", err)
	}

	if value == nil {
		return "", true, nil
	}

	path, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("resultPath must be a string or null")
	}

	return path, false, nil
}

// injectCatchResultPath injects errorOutput into input at a single-level
// "$.field" path, replacing or creating that top-level field.
func injectCatchResultPath(path, input string, errorOutput map[string]string) (string, error) {
	field, ok := strings.CutPrefix(path, "$.")
	if !ok || field == "" || strings.Contains(field, ".") {
		return "", fmt.Errorf("unsupported ResultPath %q: only \"$\" and single-level \"$.field\" are supported", path)
	}

	var inputData map[string]any
	if err := json.Unmarshal([]byte(input), &inputData); err != nil {
		return "", fmt.Errorf("resultPath %q requires an object input: %w", path, err)
	}

	if inputData == nil {
		inputData = map[string]any{}
	}

	inputData[field] = errorOutput

	out, err := json.Marshal(inputData)
	if err != nil {
		return "", fmt.Errorf("marshal ResultPath result: %w", err)
	}

	return string(out), nil
}
