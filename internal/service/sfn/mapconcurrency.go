package sfn

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// mapUnitResult is one mapUnit's outcome: output is nil when err is set.
type mapUnitResult struct {
	output json.RawMessage
	err    error
}

// mapUnitRunner holds the state shared by every goroutine started for one
// Map state's units, so runMapUnit does not need a long parameter list.
type mapUnitRunner struct {
	e         *executionEngine
	processor *stateMachineDefinition
	units     []mapUnit
	results   []mapUnitResult
	tolerant  bool
	cancel    context.CancelFunc
	sem       chan struct{}

	once     sync.Once
	firstErr error
}

// runMapUnits executes the processor once per unit, honoring MaxConcurrency
// (0 or negative means unlimited).
//
// When tolerant is false (the default: no ToleratedFailurePercentage/
// ToleratedFailureCount configured), the first unit to fail cancels the
// remaining units via cancel and its error is returned immediately --
// matching real AWS Inline-mode Map, where any iteration failure fails the
// whole state.
//
// When tolerant is true, every unit runs to completion regardless of
// individual failures, cancel is never called, and runMapUnits always
// returns a nil error; the caller (executeMapState) decides whether the
// aggregate failure count exceeds the configured threshold.
func runMapUnits(ctx context.Context, e *executionEngine, processor *stateMachineDefinition, units []mapUnit, maxConcurrency int, tolerant bool, cancel context.CancelFunc) ([]mapUnitResult, error) {
	r := &mapUnitRunner{
		e:         e,
		processor: processor,
		units:     units,
		results:   make([]mapUnitResult, len(units)),
		tolerant:  tolerant,
		cancel:    cancel,
	}

	if maxConcurrency > 0 {
		r.sem = make(chan struct{}, maxConcurrency)
	}

	var wg sync.WaitGroup

	for i := range units {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			r.runOne(ctx, i)
		}(i)
	}

	wg.Wait()

	if !r.tolerant && r.firstErr != nil {
		return nil, r.firstErr
	}

	return r.results, nil
}

// runOne runs a single unit, recording its result and, for the
// non-tolerant fast-fail path, canceling the remaining units on the first
// failure.
func (r *mapUnitRunner) runOne(ctx context.Context, i int) {
	if r.sem != nil {
		select {
		case r.sem <- struct{}{}:
			defer func() { <-r.sem }()
		case <-ctx.Done():
			// Another unit already failed and canceled ctx before this
			// queued unit got a slot; skip it rather than starting work
			// whose result will be discarded.
			return
		}
	}

	out, err := r.e.execute(ctx, r.processor, r.units[i].input)
	if err != nil {
		r.results[i].err = fmt.Errorf("item %d: %w", i, err)

		if !r.tolerant {
			r.once.Do(func() {
				r.firstErr = r.results[i].err

				r.cancel()
			})
		}

		return
	}

	r.results[i].output = json.RawMessage(out)
}
