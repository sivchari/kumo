//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/sivchari/golden"
)

func newSFNClient(t *testing.T) *sfn.Client {
	t.Helper()

	return sfn.NewFromConfig(awsConfig(t), func(o *sfn.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestSFN_CreateAndDescribeStateMachine(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	name := "test-state-machine"
	definition := `{
		"Comment": "A simple state machine",
		"StartAt": "Pass",
		"States": {
			"Pass": {
				"Type": "Pass",
				"End": true
			}
		}
	}`
	roleArn := "arn:aws:iam::000000000000:role/test-role"

	// Create state machine.
	createOutput, err := client.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
		Name:       aws.String(name),
		Definition: aws.String(definition),
		RoleArn:    aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("StateMachineArn", "CreationDate", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Describe state machine.
	describeOutput, err := client.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{
		StateMachineArn: createOutput.StateMachineArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("StateMachineArn", "CreationDate", "RevisionId", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestSFN_ListStateMachines(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	name := "test-list-state-machine"
	definition := `{"StartAt": "Pass", "States": {"Pass": {"Type": "Pass", "End": true}}}`
	roleArn := "arn:aws:iam::000000000000:role/test-role"

	// Create a state machine first.
	_, err := client.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
		Name:       aws.String(name),
		Definition: aws.String(definition),
		RoleArn:    aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List state machines.
	listOutput, err := client.ListStateMachines(ctx, &sfn.ListStateMachinesInput{
		MaxResults: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, sm := range listOutput.StateMachines {
		if *sm.Name == name {
			found = true

			break
		}
	}

	if !found {
		t.Error("created state machine not found in list")
	}
}

func TestSFN_StartAndDescribeExecution(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	name := "test-execution-state-machine"
	definition := `{"StartAt": "Pass", "States": {"Pass": {"Type": "Pass", "End": true}}}`
	roleArn := "arn:aws:iam::000000000000:role/test-role"

	// Create state machine.
	createOutput, err := client.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
		Name:       aws.String(name),
		Definition: aws.String(definition),
		RoleArn:    aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Start execution.
	execName := "test-execution"
	input := `{"key": "value"}`

	startOutput, err := client.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: createOutput.StateMachineArn,
		Name:            aws.String(execName),
		Input:           aws.String(input),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ExecutionArn", "StartDate", "ResultMetadata")).Assert(t.Name()+"_start", startOutput)

	// Describe execution. Wait for the terminal state first: even a
	// single-Pass execution can still be RUNNING on a slow runner.
	describeOutput := waitForSFNExecutionTerminal(t, client, *startOutput.ExecutionArn)
	golden.New(t, golden.WithIgnoreFields("ExecutionArn", "StateMachineArn", "StartDate", "StopDate", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestSFN_ListExecutions(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	name := "test-list-execution-state-machine"
	definition := `{"StartAt": "Pass", "States": {"Pass": {"Type": "Pass", "End": true}}}`
	roleArn := "arn:aws:iam::000000000000:role/test-role"

	// Create state machine.
	createOutput, err := client.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
		Name:       aws.String(name),
		Definition: aws.String(definition),
		RoleArn:    aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Start an execution.
	_, err = client.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: createOutput.StateMachineArn,
		Name:            aws.String("list-test-execution"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List executions.
	listOutput, err := client.ListExecutions(ctx, &sfn.ListExecutionsInput{
		StateMachineArn: createOutput.StateMachineArn,
		MaxResults:      100,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(listOutput.Executions) < 1 {
		t.Error("expected at least one execution")
	}
}

func TestSFN_GetExecutionHistory(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	name := "test-history-state-machine"
	definition := `{"StartAt": "Pass", "States": {"Pass": {"Type": "Pass", "End": true}}}`
	roleArn := "arn:aws:iam::000000000000:role/test-role"

	// Create state machine.
	createOutput, err := client.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
		Name:       aws.String(name),
		Definition: aws.String(definition),
		RoleArn:    aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Start execution.
	startOutput, err := client.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: createOutput.StateMachineArn,
		Name:            aws.String("history-test-execution"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get execution history.
	historyOutput, err := client.GetExecutionHistory(ctx, &sfn.GetExecutionHistoryInput{
		ExecutionArn: startOutput.ExecutionArn,
		MaxResults:   100,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Id", "Timestamp", "ResultMetadata")).Assert(t.Name(), historyOutput)
}

func TestSFN_DeleteStateMachine(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	name := "test-delete-state-machine"
	definition := `{"StartAt": "Pass", "States": {"Pass": {"Type": "Pass", "End": true}}}`
	roleArn := "arn:aws:iam::000000000000:role/test-role"

	// Create state machine.
	createOutput, err := client.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
		Name:       aws.String(name),
		Definition: aws.String(definition),
		RoleArn:    aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete state machine.
	_, err = client.DeleteStateMachine(ctx, &sfn.DeleteStateMachineInput{
		StateMachineArn: createOutput.StateMachineArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{
		StateMachineArn: createOutput.StateMachineArn,
	})
	if err == nil {
		t.Fatal("expected error for deleted state machine")
	}
}

func TestSFN_StateMachineNotFound(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	// Try to describe a non-existent state machine.
	_, err := client.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{
		StateMachineArn: aws.String("arn:aws:states:us-east-1:000000000000:stateMachine:nonexistent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent state machine")
	}
}

func TestSFN_ExpressStateMachine(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	name := "test-express-state-machine"
	definition := `{"StartAt": "Pass", "States": {"Pass": {"Type": "Pass", "End": true}}}`
	roleArn := "arn:aws:iam::000000000000:role/test-role"

	// Create EXPRESS state machine.
	createOutput, err := client.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
		Name:       aws.String(name),
		Definition: aws.String(definition),
		RoleArn:    aws.String(roleArn),
		Type:       "EXPRESS",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Describe to verify type.
	describeOutput, err := client.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{
		StateMachineArn: createOutput.StateMachineArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("StateMachineArn", "CreationDate", "RevisionId", "ResultMetadata")).Assert(t.Name(), describeOutput)
}

func TestSFN_TagOperations(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	name := "test-tag-operations"
	definition := `{"StartAt": "Pass", "States": {"Pass": {"Type": "Pass", "End": true}}}`
	roleArn := "arn:aws:iam::000000000000:role/test-role"

	// Create state machine.
	createOutput, err := client.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
		Name:       aws.String(name),
		Definition: aws.String(definition),
		RoleArn:    aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	smArn := *createOutput.StateMachineArn

	t.Cleanup(func() {
		_, _ = client.DeleteStateMachine(context.Background(), &sfn.DeleteStateMachineInput{
			StateMachineArn: aws.String(smArn),
		})
	})

	// TagResource: add two tags.
	_, err = client.TagResource(ctx, &sfn.TagResourceInput{
		ResourceArn: aws.String(smArn),
		Tags: []sfntypes.Tag{
			{Key: aws.String("Environment"), Value: aws.String("Test")},
			{Key: aws.String("Project"), Value: aws.String("kumo")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ListTagsForResource: verify tags.
	listOutput, err := client.ListTagsForResource(ctx, &sfn.ListTagsForResourceInput{
		ResourceArn: aws.String(smArn),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_after_tag", listOutput)

	// UntagResource: remove one tag.
	_, err = client.UntagResource(ctx, &sfn.UntagResourceInput{
		ResourceArn: aws.String(smArn),
		TagKeys:     []string{"Environment"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ListTagsForResource: verify remaining tag.
	listOutput2, err := client.ListTagsForResource(ctx, &sfn.ListTagsForResourceInput{
		ResourceArn: aws.String(smArn),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_after_untag", listOutput2)
}

func TestSFN_ActivityCRUD(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	name := "test-activity"

	// CreateActivity.
	createOutput, err := client.CreateActivity(ctx, &sfn.CreateActivityInput{
		Name: aws.String(name),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ActivityArn", "CreationDate", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	activityArn := *createOutput.ActivityArn

	t.Cleanup(func() {
		_, _ = client.DeleteActivity(context.Background(), &sfn.DeleteActivityInput{
			ActivityArn: aws.String(activityArn),
		})
	})

	// CreateActivity again with the same name: idempotent, same ARN.
	secondCreateOutput, err := client.CreateActivity(ctx, &sfn.CreateActivityInput{
		Name: aws.String(name),
	})
	if err != nil {
		t.Fatal(err)
	}

	if *secondCreateOutput.ActivityArn != activityArn {
		t.Fatalf("CreateActivity is not idempotent: got %q, want %q", *secondCreateOutput.ActivityArn, activityArn)
	}

	// DescribeActivity.
	describeOutput, err := client.DescribeActivity(ctx, &sfn.DescribeActivityInput{
		ActivityArn: aws.String(activityArn),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ActivityArn", "CreationDate", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)

	// ListActivities.
	listOutput, err := client.ListActivities(ctx, &sfn.ListActivitiesInput{
		MaxResults: 100,
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, a := range listOutput.Activities {
		if *a.Name == name {
			found = true

			break
		}
	}

	if !found {
		t.Error("created activity not found in list")
	}

	// DeleteActivity.
	_, err = client.DeleteActivity(ctx, &sfn.DeleteActivityInput{
		ActivityArn: aws.String(activityArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.DescribeActivity(ctx, &sfn.DescribeActivityInput{
		ActivityArn: aws.String(activityArn),
	})
	if err == nil {
		t.Fatal("expected error for deleted activity")
	}
}

// waitForSFNExecutionTerminal polls DescribeExecution until executionArn
// leaves RUNNING: unlike every other state machine in this file (a single
// Pass state, which completes essentially instantly), a
// states:startExecution.sync/.sync:2 Task's own execution can still be
// RUNNING by the time the test calls DescribeExecution, since it waits on
// its child execution in the background.
func waitForSFNExecutionTerminal(t *testing.T, client *sfn.Client, executionArn string) *sfn.DescribeExecutionOutput {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for {
		out, err := client.DescribeExecution(context.Background(), &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(executionArn),
		})
		if err != nil {
			t.Fatalf("DescribeExecution: %v", err)
		}

		if out.Status != sfntypes.ExecutionStatusRunning {
			return out
		}

		if time.Now().After(deadline) {
			t.Fatalf("execution %q did not terminate within the deadline", executionArn)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

// TestSFN_NestedExecution exercises the states:startExecution.sync:2
// integration end to end: a parent state machine's Task starts a child
// state machine and waits for it, and the Task's own output (the whole
// parent execution's Output here, since the Task is also the parent's only
// state) mirrors DescribeExecution with the child's own Output parsed as
// JSON rather than left as a string -- see
// https://docs.aws.amazon.com/step-functions/latest/dg/connect-stepfunctions.html.
func TestSFN_NestedExecution(t *testing.T) {
	client := newSFNClient(t)
	ctx := t.Context()

	roleArn := "arn:aws:iam::000000000000:role/test-role"

	childDefinition := `{
		"StartAt": "Build",
		"States": {"Build": {"Type": "Pass", "Result": {"MyKey": "MyValue"}, "End": true}}
	}`

	childOutput, err := client.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
		Name:       aws.String("test-nested-child"),
		Definition: aws.String(childDefinition),
		RoleArn:    aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	parentDefinition := fmt.Sprintf(`{
		"StartAt": "Nested",
		"States": {
			"Nested": {
				"Type": "Task",
				"Resource": "arn:aws:states:::states:startExecution.sync:2",
				"Parameters": {"StateMachineArn": %q, "Input": {}},
				"End": true
			}
		}
	}`, *childOutput.StateMachineArn)

	parentOutput, err := client.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
		Name:       aws.String("test-nested-parent"),
		Definition: aws.String(parentDefinition),
		RoleArn:    aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	startOutput, err := client.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: parentOutput.StateMachineArn,
		Input:           aws.String("{}"),
	})
	if err != nil {
		t.Fatal(err)
	}

	describeOutput := waitForSFNExecutionTerminal(t, client, *startOutput.ExecutionArn)

	golden.New(t, golden.WithIgnoreFields(
		"ExecutionArn", "StateMachineArn", "StartDate", "StopDate", "Name", "Output", "ResultMetadata",
	)).Assert(t.Name()+"_describe", describeOutput)

	if describeOutput.Status != sfntypes.ExecutionStatusSucceeded {
		t.Fatalf("parent execution status: got %s, want %s (output: %s)", describeOutput.Status, sfntypes.ExecutionStatusSucceeded, aws.ToString(describeOutput.Output))
	}

	var taskOutput struct {
		Status string         `json:"status"`
		Output map[string]any `json:"output"`
	}
	if err := json.Unmarshal([]byte(aws.ToString(describeOutput.Output)), &taskOutput); err != nil {
		t.Fatalf("unmarshal parent execution Output %q: %v", aws.ToString(describeOutput.Output), err)
	}

	if taskOutput.Status != string(sfntypes.ExecutionStatusSucceeded) {
		t.Fatalf(".sync:2 nested output.status: got %q, want %q", taskOutput.Status, sfntypes.ExecutionStatusSucceeded)
	}

	if taskOutput.Output["MyKey"] != "MyValue" {
		t.Fatalf(".sync:2 nested output.output: got %v, want parsed JSON {\"MyKey\":\"MyValue\"}", taskOutput.Output)
	}
}
