package sfn

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// distributedMapDefinitionForMapRun builds a single distributed-mode Map
// state using conditionalFailProcessorJSON (see toleratedfailure_test.go),
// with tolerance/name given verbatim so tests can control both the Map
// Run's recorded tolerance fields and whether it ends up FAILED or
// SUCCEEDED.
func distributedMapDefinitionForMapRun(mapStateName, toleranceField string) string {
	extra := ""
	if toleranceField != "" {
		extra = toleranceField + ","
	}

	return `{
		"StartAt": "` + mapStateName + `",
		"States": {
			"` + mapStateName + `": {
				"Type": "Map",
				` + extra + `
				"ItemProcessor": ` + conditionalFailProcessorJSON + `,
				"End": true
			}
		}
	}`
}

func TestDistributedMapRecordsSucceededMapRun(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "maprun-succeeded",
		distributedMapDefinitionForMapRun("Each", `"ToleratedFailurePercentage": 100`))

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, oneOfFourFailsInput)

	mapRuns, _, err := store.ListMapRuns(context.Background(), exec.ExecutionArn, 0, "")
	if err != nil {
		t.Fatalf("ListMapRuns: %v", err)
	}

	if len(mapRuns) != 1 {
		t.Fatalf("mapRuns: got %d, want 1", len(mapRuns))
	}

	mr := mapRuns[0]

	if mr.ExecutionArn != exec.ExecutionArn {
		t.Fatalf("ListMapRuns item ExecutionArn: got %q, want %q", mr.ExecutionArn, exec.ExecutionArn)
	}

	if mr.StateMachineArn != sm.StateMachineArn {
		t.Fatalf("ListMapRuns item StateMachineArn: got %q, want %q", mr.StateMachineArn, sm.StateMachineArn)
	}

	if !strings.Contains(mr.MapRunArn, ":mapRun:"+sm.Name+"/Each:") {
		t.Fatalf("MapRunArn %q: want it to contain %q", mr.MapRunArn, ":mapRun:"+sm.Name+"/Each:")
	}

	described, err := store.DescribeMapRun(context.Background(), mr.MapRunArn)
	if err != nil {
		t.Fatalf("DescribeMapRun: %v", err)
	}

	if described.Status != mapRunStatusSucceeded {
		t.Fatalf("DescribeMapRun Status: got %q, want %q", described.Status, mapRunStatusSucceeded)
	}

	if described.ItemCounts.Total != 4 || described.ItemCounts.Succeeded != 3 || described.ItemCounts.Failed != 1 {
		t.Fatalf("DescribeMapRun ItemCounts: got %+v, want {Total:4 Succeeded:3 Failed:1 ...}", described.ItemCounts)
	}

	if described.ExecutionCounts.Total != 4 {
		t.Fatalf("DescribeMapRun ExecutionCounts.Total: got %d, want 4", described.ExecutionCounts.Total)
	}

	if described.ToleratedFailurePercentage == nil || *described.ToleratedFailurePercentage != 100 {
		t.Fatalf("DescribeMapRun ToleratedFailurePercentage: got %v, want 100", described.ToleratedFailurePercentage)
	}

	if described.StopDate == nil {
		t.Fatal("DescribeMapRun StopDate: want non-nil for a finished Map Run")
	}
}

func TestDistributedMapRecordsFailedMapRunWhenToleranceExceeded(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "maprun-failed",
		distributedMapDefinitionForMapRun("Each", `"ToleratedFailureCount": 0`))

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, oneOfFourFailsInput)

	mapRuns, _, err := store.ListMapRuns(context.Background(), exec.ExecutionArn, 0, "")
	if err != nil {
		t.Fatalf("ListMapRuns: %v", err)
	}

	if len(mapRuns) != 1 {
		t.Fatalf("mapRuns: got %d, want 1", len(mapRuns))
	}

	described, err := store.DescribeMapRun(context.Background(), mapRuns[0].MapRunArn)
	if err != nil {
		t.Fatalf("DescribeMapRun: %v", err)
	}

	if described.Status != mapRunStatusFailed {
		t.Fatalf("DescribeMapRun Status: got %q, want %q", described.Status, mapRunStatusFailed)
	}
}

func TestInlineMapDoesNotRecordAMapRun(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {"Type": "Map", "ItemProcessor": ` + echoItemProcessorJSON + `, "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "maprun-inline", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `[1,2]`)

	mapRuns, _, err := store.ListMapRuns(context.Background(), exec.ExecutionArn, 0, "")
	if err != nil {
		t.Fatalf("ListMapRuns: %v", err)
	}

	if len(mapRuns) != 0 {
		t.Fatalf("mapRuns for an Inline-mode Map: got %d, want 0: %+v", len(mapRuns), mapRuns)
	}
}

func TestDescribeMapRunUnknownArnReportsResourceNotFound(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	_, err := store.DescribeMapRun(context.Background(), "arn:aws:states:us-east-1:000000000000:mapRun:missing/x:00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("DescribeMapRun: want error for an unknown mapRunArn, got nil")
	}

	var svcErr *ServiceError
	if !errors.As(err, &svcErr) || svcErr.Code != errResourceNotFound {
		t.Fatalf("DescribeMapRun error: got %v, want ServiceError code %q", err, errResourceNotFound)
	}
}

func TestListMapRunsUnknownExecutionArnReportsExecutionDoesNotExist(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	_, _, err := store.ListMapRuns(context.Background(), "arn:aws:states:us-east-1:000000000000:execution:missing:x", 0, "")
	if err == nil {
		t.Fatal("ListMapRuns: want error for an unknown executionArn, got nil")
	}

	var svcErr *ServiceError
	if !errors.As(err, &svcErr) || svcErr.Code != errExecutionDoesNotExist {
		t.Fatalf("ListMapRuns error: got %v, want ServiceError code %q", err, errExecutionDoesNotExist)
	}
}
