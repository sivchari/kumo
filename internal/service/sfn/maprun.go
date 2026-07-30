package sfn

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Map Run status values, matching AWS's documented DescribeMapRun "status".
const (
	mapRunStatusRunning   = "RUNNING"
	mapRunStatusSucceeded = "SUCCEEDED"
	mapRunStatusFailed    = "FAILED"
)

// executionArnContextKey is the context.Context key carrying the current
// (top-level or nested) execution's ARN through its own synchronous call
// graph, the same way nestedExecutionDepthKey carries nesting depth (see
// nestedexec.go) -- set once in MemoryStorage.runExecution and read back by
// executionEngine.startMapRunIfDistributed, which needs it to attribute a
// new Map Run to the execution that started it.
type executionArnContextKey struct{}

// withExecutionArn returns a context carrying executionArn as the execution
// currently running.
func withExecutionArn(ctx context.Context, executionArn string) context.Context {
	return context.WithValue(ctx, executionArnContextKey{}, executionArn)
}

// executionArnFromContext reads the current execution's ARN from ctx,
// returning "" if none was installed (e.g. a test engine exercised without
// going through MemoryStorage.runExecution).
func executionArnFromContext(ctx context.Context) string {
	arn, _ := ctx.Value(executionArnContextKey{}).(string)

	return arn
}

// mapRunTracker is the subset of Storage a Distributed Map state uses to
// record its Map Run (see startMapRunIfDistributed/finishMapRunIfDistributed
// below). Like executionStarter (nestedexec.go), executionEngine calls back
// into the same MemoryStorage that constructed it rather than an HTTP round
// trip: recording a Map Run is storage bookkeeping local to this process,
// not a call to another emulated AWS service.
type mapRunTracker interface {
	startMapRun(ctx context.Context, executionArn, mapStateName string, maxConcurrency int, tolerance mapTolerance) (mapRunArn string, err error)
	finishMapRun(mapRunArn, status string, itemCounts, executionCounts *MapRunCounts)
}

// startMapRunIfDistributed records a new Map Run for a Distributed-mode Map
// state (isDistributedMode(processor)), returning "" for an Inline-mode Map
// (Map Runs are a Distributed-mode-only AWS concept -- see
// https://docs.aws.amazon.com/step-functions/latest/dg/concepts-state-machine-executions.html#concepts-map-run)
// or when e.mapRuns/the current execution ARN is unavailable (an engine
// built directly via newExecutionEngine in tests, bypassing
// MemoryStorage.runExecution). Bookkeeping failures are logged and
// otherwise ignored -- see finishMapRunIfDistributed -- so the Map state
// itself still completes normally even if Map Run tracking could not.
func (e *executionEngine) startMapRunIfDistributed(ctx context.Context, name string, processor *stateMachineDefinition, maxConcurrency int, tolerance mapTolerance) string {
	if e.mapRuns == nil || !isDistributedMode(processor) {
		return ""
	}

	executionArn := executionArnFromContext(ctx)
	if executionArn == "" {
		return ""
	}

	mapRunArn, err := e.mapRuns.startMapRun(ctx, executionArn, name, maxConcurrency, tolerance)
	if err != nil {
		slog.Debug("SFN executor: failed to record Map Run", "state", name, "error", err)

		return ""
	}

	return mapRunArn
}

// finishMapRunIfDistributed finalizes the Map Run started by
// startMapRunIfDistributed, computing its item/execution counts from the
// processed units. results is nil when runMapUnits itself returned an error
// (the non-tolerant fast-fail path, which cancels and discards the
// remaining units' outcomes -- see mapRunCountsFromResults).
func (e *executionEngine) finishMapRunIfDistributed(mapRunArn, status string, units []mapUnit, results []mapUnitResult, totalItems int) {
	if e.mapRuns == nil || mapRunArn == "" {
		return
	}

	itemCounts, executionCounts := mapRunCountsFromResults(units, results, totalItems)
	e.mapRuns.finishMapRun(mapRunArn, status, &itemCounts, &executionCounts)
}

// mapRunCountsFromResults derives a finished Map Run's itemCounts (dataset
// items, weighted by each unit's itemCount -- see mapUnit in itembatcher.go
// for why a unit can represent more than one item under ItemBatcher) and
// executionCounts (processor invocations, one per unit) from its units'
// outcomes. kumo runs every unit in-process rather than as a real child
// *execution* (see mapstate.go), so Pending/Running/Aborted/TimedOut/
// ResultsWritten/FailuresNotRedrivable/PendingRedrive are always reported as
// 0 rather than fabricated -- kumo's Map processing has no notion of any of
// those states.
func mapRunCountsFromResults(units []mapUnit, results []mapUnitResult, totalItems int) (itemCounts, executionCounts MapRunCounts) {
	executionCounts.Total = int64(len(units))
	itemCounts.Total = int64(totalItems)

	if results == nil {
		// The non-tolerant fast-fail path (see runMapUnits in
		// mapconcurrency.go) cancels and discards the remaining units'
		// results once the first failure occurs, so kumo cannot attribute
		// success/failure per unit here; report the whole run as failed,
		// matching the Map state's own outcome.
		executionCounts.Failed = executionCounts.Total
		itemCounts.Failed = itemCounts.Total

		return itemCounts, executionCounts
	}

	for i, r := range results {
		if r.err != nil {
			executionCounts.Failed++
			itemCounts.Failed += int64(units[i].itemCount)

			continue
		}

		executionCounts.Succeeded++
		itemCounts.Succeeded += int64(units[i].itemCount)
	}

	return itemCounts, executionCounts
}

// startMapRun implements mapRunTracker: it looks up executionArn's state
// machine (for the Map Run ARN's name segment and the record's own
// StateMachineArn) and records a new RUNNING Map Run.
func (s *MemoryStorage) startMapRun(_ context.Context, executionArn, mapStateName string, maxConcurrency int, tolerance mapTolerance) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ed, exists := s.Executions[executionArn]
	if !exists {
		return "", fmt.Errorf("map run: execution %q not found", executionArn)
	}

	stateMachineArn := ed.Execution.StateMachineArn
	stateMachineName := stateMachineArn

	if sm, ok := s.StateMachines[stateMachineArn]; ok {
		stateMachineName = sm.Name
	}

	mapRunArn := buildMapRunArn(s.region, s.accountID, stateMachineName, mapStateName)

	s.MapRuns[mapRunArn] = &MapRun{
		MapRunArn:                  mapRunArn,
		ExecutionArn:               executionArn,
		StateMachineArn:            stateMachineArn,
		Status:                     mapRunStatusRunning,
		StartDate:                  time.Now(),
		MaxConcurrency:             maxConcurrency,
		ToleratedFailureCount:      tolerance.count,
		ToleratedFailurePercentage: tolerance.percentage,
	}

	s.saveLocked()

	return mapRunArn, nil
}

// finishMapRun implements mapRunTracker: it records status and the final
// item/execution counts on an existing Map Run. A mapRunArn kumo does not
// recognize is silently ignored -- startMapRunIfDistributed already logs
// the failure that would cause this.
func (s *MemoryStorage) finishMapRun(mapRunArn, status string, itemCounts, executionCounts *MapRunCounts) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mr, exists := s.MapRuns[mapRunArn]
	if !exists {
		return
	}

	now := time.Now()
	mr.Status = status
	mr.StopDate = &now
	mr.ItemCounts = *itemCounts
	mr.ExecutionCounts = *executionCounts

	s.saveLocked()
}

// buildMapRunArn builds a Map Run ARN in AWS's documented format
// arn:aws:states:{region}:{account}:mapRun:{stateMachineName}/{label}:{uuid}
// (see
// https://docs.aws.amazon.com/step-functions/latest/dg/state-map-distributed.html#state-map-distributed-nested-workflow).
// AWS's {label} segment is either a user-supplied Map state "Label" field or
// an auto-generated one; kumo does not model the "Label" field, so it uses
// the Map state's own name instead -- more useful for debugging than a
// random value, and, unlike AWS's auto-generated label, deterministic given
// the same definition.
func buildMapRunArn(region, accountID, stateMachineName, mapStateName string) string {
	return fmt.Sprintf("arn:aws:states:%s:%s:mapRun:%s/%s:%s", region, accountID, stateMachineName, mapStateName, uuid.New().String())
}

// copyMapRun returns a shallow copy of mr so callers never hold a pointer
// into MemoryStorage's own map (mirrors copyExecution in storage.go).
func copyMapRun(mr *MapRun) *MapRun {
	c := *mr

	return &c
}
