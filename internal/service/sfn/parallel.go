package sfn

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// executeParallelStateWithPolicy builds the effective input from
// InputPath/Parameters, then runs it through the Retry/Catch/
// ResultSelector/ResultPath/OutputPath pipeline.
func (e *executionEngine) executeParallelStateWithPolicy(ctx context.Context, name string, state *stateDefinition, input string) (string, string, error) {
	effectiveInput, err := resolveEffectiveInput(state.InputPath, state.Parameters, input)
	if err != nil {
		return "", "", fmt.Errorf("state %q: %w", name, err)
	}

	return e.runRetryCatchResultPipeline(ctx, name, state, input, effectiveInput, func(ctx context.Context) (string, error) {
		return e.executeParallelState(ctx, name, state, effectiveInput)
	})
}

// executeParallelState runs each Branch concurrently and collects outputs in
// branch order as a JSON array. The first branch to fail cancels the rest
// and fails the state with States.BranchFailed, matching real Step Functions.
func (e *executionEngine) executeParallelState(ctx context.Context, name string, state *stateDefinition, input string) (string, error) {
	if len(state.Branches) == 0 {
		return "", fmt.Errorf("parallel state %q: Branches is required", name)
	}

	branchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	outputs, err := runBranches(branchCtx, e, state.Branches, input, cancel)
	if err != nil {
		return "", &failStateError{
			errorName: errorStatesBranchFailed,
			cause:     fmt.Sprintf("parallel state %q: %s", name, err),
		}
	}

	result, err := json.Marshal(outputs)
	if err != nil {
		return "", fmt.Errorf("parallel state %q: marshal branch outputs: %w", name, err)
	}

	return string(result), nil
}

// runBranches executes each branch concurrently, returning the branch
// outputs in branch order, or the error of the first branch to fail.
func runBranches(ctx context.Context, e *executionEngine, branches []stateMachineDefinition, input string, cancel context.CancelFunc) ([]json.RawMessage, error) {
	outputs := make([]json.RawMessage, len(branches))

	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)

	for i := range branches {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			out, err := e.execute(ctx, &branches[i], input)
			if err != nil {
				once.Do(func() {
					firstErr = fmt.Errorf("branch %d: %w", i, err)

					cancel()
				})

				return
			}

			outputs[i] = json.RawMessage(out)
		}(i)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return outputs, nil
}
