package sfn

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
// retrier for why its JSON tags are lowerCamelCase, and parsePathField in
// iodata.go for why ResultPath is left as raw JSON.
type catcher struct {
	ErrorEquals []string        `json:"errorEquals"`
	Next        string          `json:"next"`
	ResultPath  json.RawMessage `json:"resultPath"`
}

// Retry defaults per the Amazon States Language spec.
const (
	defaultRetryIntervalSeconds = 1
	defaultRetryMaxAttempts     = 3
	defaultRetryBackoffRate     = 2.0
)

// runRetryCatchResultPipeline executes work (a state's own work for one
// execution, already given whatever effective input the caller computed
// for it -- see executeTaskStateWithPolicy, executeParallelStateWithPolicy,
// and executeMapStateWithPolicy in executor.go/parallel.go/mapstate.go,
// which each fold InputPath/Parameters differently), applying that state's
// Retry and Catch policy: retries are attempted first, using the matching
// Retrier's backoff schedule; if retries do not resolve the error, or no
// Retrier matches, the first matching Catcher routes to its Next state
// with the Catcher's Error Output merged into the state's raw
// (un-filtered) input via the Catcher's own ResultPath.
//
// On success, the result passes through ResultSelector, ResultPath, and
// OutputPath to produce the effective output (see applyResultPipeline). On
// a Catch match, only OutputPath applies to the Catcher's output --
// ResultSelector/ResultPath are skipped, since the Catcher already shaped
// its own output via its own ResultPath -- because OutputPath is a
// state-level field that applies to the state's output regardless of
// which path produced it (see applyCatchResult). An unmatched error
// propagates unchanged: per AWS, a data-flow failure itself (e.g. an
// unresolvable ResultPath) reports as States.Runtime, which is never
// caught (see errorNameMatches), so such failures never reach Retry/Catch
// in the first place.
//
// The return shape mirrors executeState: nextOverride is non-empty only
// when a Catcher matched.
func (e *executionEngine) runRetryCatchResultPipeline(
	ctx context.Context, name string, state *stateDefinition, rawInput, effectiveInput string,
	work func(context.Context) (string, error),
) (string, string, error) {
	result, err := e.runWithRetry(ctx, state, work)
	if err != nil {
		return e.applyCatchResult(name, state, rawInput, err)
	}

	return e.applyResultPipeline(name, state, effectiveInput, result)
}

// applyCatchResult scans state's Catchers for a match against workErr,
// applying OutputPath to the matching Catcher's output. If no Catcher
// matches, workErr propagates unchanged so the caller fails the state.
func (e *executionEngine) applyCatchResult(name string, state *stateDefinition, rawInput string, workErr error) (string, string, error) {
	catchOutput, catchNext, matched, buildErr := applyCatch(state, rawInput, workErr)
	if buildErr != nil {
		return "", "", fmt.Errorf("state %q: %w", name, buildErr)
	}

	if !matched {
		return "", "", workErr
	}

	out, err := applyOutputPath(state.OutputPath, catchOutput)
	if err != nil {
		return "", "", fmt.Errorf("state %q: %w", name, err)
	}

	return out, catchNext, nil
}

// applyResultPipeline turns a state's own successful result into its
// effective output: ResultSelector produces the effective result,
// ResultPath merges it into effectiveInput, and OutputPath narrows the
// merged output.
func (e *executionEngine) applyResultPipeline(name string, state *stateDefinition, effectiveInput, result string) (string, string, error) {
	effectiveResult, err := applyResultSelector(state.ResultSelector, result)
	if err != nil {
		return "", "", fmt.Errorf("state %q: %w", name, err)
	}

	merged, err := applyResultPath(state.ResultPath, effectiveInput, effectiveResult)
	if err != nil {
		return "", "", fmt.Errorf("state %q: %w", name, err)
	}

	out, err := applyOutputPath(state.OutputPath, merged)
	if err != nil {
		return "", "", fmt.Errorf("state %q: %w", name, err)
	}

	return out, "", nil
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
// {"Error": errName, "Cause": cause}, merged into the state's original
// (raw) input per the Catcher's own ResultPath -- "works exactly like a
// state's top-level ResultPath" per the spec's JSONPath Catchers section,
// so it reuses applyResultPath, supporting the same "$", null, and
// arbitrary-depth "$.a.b.c" semantics.
func buildCatchOutput(rawResultPath json.RawMessage, input, errName, cause string) (string, error) {
	errorOutput := map[string]string{"Error": errName, "Cause": cause}

	resultJSON, err := json.Marshal(errorOutput)
	if err != nil {
		return "", fmt.Errorf("marshal error output: %w", err)
	}

	out, err := applyResultPath(rawResultPath, input, string(resultJSON))
	if err != nil {
		return "", fmt.Errorf("catch: %w", err)
	}

	return out, nil
}
