package sfn

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestActivityCRUDRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	created, err := store.CreateActivity(ctx, "my-activity", nil)
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}

	described, err := store.DescribeActivity(ctx, created.ActivityArn)
	if err != nil {
		t.Fatalf("DescribeActivity: %v", err)
	}

	if described.Name != "my-activity" {
		t.Fatalf("DescribeActivity Name: got %q, want %q", described.Name, "my-activity")
	}

	activities, _, err := store.ListActivities(ctx, 100, "")
	if err != nil {
		t.Fatalf("ListActivities: %v", err)
	}

	found := false

	for _, a := range activities {
		if a.ActivityArn == created.ActivityArn {
			found = true
		}
	}

	if !found {
		t.Fatalf("ListActivities: created activity %q not found in %+v", created.ActivityArn, activities)
	}

	if err := store.DeleteActivity(ctx, created.ActivityArn); err != nil {
		t.Fatalf("DeleteActivity: %v", err)
	}

	if _, err := store.DescribeActivity(ctx, created.ActivityArn); err == nil {
		t.Fatal("DescribeActivity after DeleteActivity: expected error")
	}
}

func TestCreateActivityIsIdempotentByName(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	first, err := store.CreateActivity(ctx, "dup-activity", []Tag{{Key: "k", Value: "v1"}})
	if err != nil {
		t.Fatalf("first CreateActivity: %v", err)
	}

	second, err := store.CreateActivity(ctx, "dup-activity", []Tag{{Key: "k", Value: "v2"}})
	if err != nil {
		t.Fatalf("second CreateActivity: %v", err)
	}

	if first.ActivityArn != second.ActivityArn {
		t.Fatalf("ActivityArn changed across idempotent CreateActivity calls: %q vs %q", first.ActivityArn, second.ActivityArn)
	}

	if !first.CreationDate.Equal(second.CreationDate) {
		t.Fatalf("CreationDate changed across idempotent CreateActivity calls: %v vs %v", first.CreationDate, second.CreationDate)
	}

	tags, err := store.ListTagsForResource(ctx, first.ActivityArn)
	if err != nil {
		t.Fatalf("ListTagsForResource: %v", err)
	}

	if len(tags) != 1 || tags[0].Value != "v1" {
		t.Fatalf("tags after idempotent re-create: got %+v, want the original [{k v1}] unchanged", tags)
	}
}

func TestDescribeActivityNotFound(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	_, err := store.DescribeActivity(context.Background(), "arn:aws:states:us-east-1:000000000000:activity:nonexistent")

	var svcErr *ServiceError
	if !errors.As(err, &svcErr) || svcErr.Code != errActivityDoesNotExist {
		t.Fatalf("DescribeActivity error: got %v, want ServiceError code %q", err, errActivityDoesNotExist)
	}
}

func TestGetActivityTaskDeliversExactlyOneTaskToOneOfTwoPollers(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	store.engine.activityPollTimeout = 300 * time.Millisecond

	ctx := context.Background()

	activity, err := store.CreateActivity(ctx, "poll-activity", nil)
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}

	results := make(chan string, 2)

	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			token, _, err := store.GetActivityTask(ctx, activity.ActivityArn, "")
			if err != nil {
				t.Errorf("GetActivityTask: %v", err)

				return
			}

			results <- token
		}()
	}

	// Give both pollers time to register as waiters before scheduling the
	// only task; otherwise enqueue could hand it to whichever poller
	// happened to register first, defeating the "two concurrent pollers"
	// setup this test is for.
	time.Sleep(50 * time.Millisecond)

	q := store.engine.activityQueues.get(activity.ActivityArn)
	q.enqueue(&activityTask{token: "the-only-task", input: "{}", pending: &pendingCallback{
		done:      make(chan callbackResult, 1),
		heartbeat: make(chan struct{}, 1),
	}})

	wg.Wait()
	close(results)

	var delivered []string

	for token := range results {
		if token != "" {
			delivered = append(delivered, token)
		}
	}

	if len(delivered) != 1 || delivered[0] != "the-only-task" {
		t.Fatalf("delivered tokens: got %v, want exactly one delivery of %q", delivered, "the-only-task")
	}
}

func TestGetActivityTaskReturnsEmptyTaskTokenOnPollTimeout(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	store.engine.activityPollTimeout = 50 * time.Millisecond

	activity, err := store.CreateActivity(context.Background(), "idle-activity", nil)
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}

	token, input, err := store.GetActivityTask(context.Background(), activity.ActivityArn, "")
	if err != nil {
		t.Fatalf("GetActivityTask: %v", err)
	}

	if token != "" || input != "" {
		t.Fatalf("GetActivityTask on idle activity: got taskToken=%q input=%q, want both empty", token, input)
	}
}

func TestGetActivityTaskUnknownActivityReportsActivityDoesNotExist(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	_, _, err := store.GetActivityTask(context.Background(), "arn:aws:states:us-east-1:000000000000:activity:nonexistent", "")

	var svcErr *ServiceError
	if !errors.As(err, &svcErr) || svcErr.Code != errActivityDoesNotExist {
		t.Fatalf("GetActivityTask error: got %v, want ServiceError code %q", err, errActivityDoesNotExist)
	}
}

func TestActivityTaskStateEndToEndWorkerPollsAndSucceeds(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	store.engine.activityPollTimeout = 2 * time.Second

	ctx := context.Background()

	activity, err := store.CreateActivity(ctx, "worker-activity", nil)
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}

	definition := `{
		"StartAt": "DoWork",
		"States": {
			"DoWork": {
				"Type": "Task",
				"Resource": "` + activity.ActivityArn + `",
				"End": true
			}
		}
	}`

	sm := createExecutionTestStateMachine(t, store, "activity-task", definition)

	started, err := store.StartExecution(ctx, sm.StateMachineArn, "", `{"n":1}`, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	token, input, err := store.GetActivityTask(ctx, activity.ActivityArn, "worker-1")
	if err != nil {
		t.Fatalf("GetActivityTask: %v", err)
	}

	if token == "" {
		t.Fatal("GetActivityTask: expected a task token")
	}

	if input != `{"n":1}` {
		t.Fatalf("GetActivityTask input: got %q, want %q", input, `{"n":1}`)
	}

	if err := store.SendTaskSuccess(ctx, token, `{"n":2}`); err != nil {
		t.Fatalf("SendTaskSuccess: %v", err)
	}

	exec := pollExecutionUntilTerminal(t, store, started.ExecutionArn)
	if exec.Status != ExecutionStatusSucceeded {
		t.Fatalf("execution status: got %q, want SUCCEEDED (error: %s, cause: %s)", exec.Status, exec.Error, exec.Cause)
	}

	if exec.Output != `{"n":2}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"n":2}`)
	}
}

func TestActivityTaskStateRetriesAfterSendTaskFailure(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	store.engine.activityPollTimeout = 2 * time.Second

	ctx := context.Background()

	activity, err := store.CreateActivity(ctx, "retry-activity", nil)
	if err != nil {
		t.Fatalf("CreateActivity: %v", err)
	}

	definition := `{
		"StartAt": "DoWork",
		"States": {
			"DoWork": {
				"Type": "Task",
				"Resource": "` + activity.ActivityArn + `",
				"Retry": [{"ErrorEquals": ["States.ALL"], "MaxAttempts": 1, "IntervalSeconds": 0}],
				"End": true
			}
		}
	}`

	sm := createExecutionTestStateMachine(t, store, "activity-task-retry", definition)

	started, err := store.StartExecution(ctx, sm.StateMachineArn, "", `{}`, "")
	if err != nil {
		t.Fatalf("StartExecution: %v", err)
	}

	// First attempt: fail it, expecting the Retry policy to schedule the
	// task again.
	firstToken, _, err := store.GetActivityTask(ctx, activity.ActivityArn, "")
	if err != nil {
		t.Fatalf("GetActivityTask (attempt 1): %v", err)
	}

	if err := store.SendTaskFailure(ctx, firstToken, "TransientError", "try again"); err != nil {
		t.Fatalf("SendTaskFailure: %v", err)
	}

	// Second attempt: succeed it.
	secondToken, _, err := store.GetActivityTask(ctx, activity.ActivityArn, "")
	if err != nil {
		t.Fatalf("GetActivityTask (attempt 2): %v", err)
	}

	if err := store.SendTaskSuccess(ctx, secondToken, `{"done":true}`); err != nil {
		t.Fatalf("SendTaskSuccess: %v", err)
	}

	exec := pollExecutionUntilTerminal(t, store, started.ExecutionArn)
	if exec.Status != ExecutionStatusSucceeded {
		t.Fatalf("execution status: got %q, want SUCCEEDED (error: %s, cause: %s)", exec.Status, exec.Error, exec.Cause)
	}
}
