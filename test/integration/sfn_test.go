//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/sivchari/golden"
)

func newSFNClient(t *testing.T) *sfn.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return sfn.NewFromConfig(cfg, func(o *sfn.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
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

	// Describe execution.
	describeOutput, err := client.DescribeExecution(ctx, &sfn.DescribeExecutionInput{
		ExecutionArn: startOutput.ExecutionArn,
	})
	if err != nil {
		t.Fatal(err)
	}
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
