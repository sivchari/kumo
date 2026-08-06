//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/sivchari/golden"
)

func newAthenaClient(t *testing.T) *athena.Client {
	t.Helper()

	return athena.NewFromConfig(awsConfig(t), func(o *athena.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestAthena_StartQueryExecution(t *testing.T) {
	client := newAthenaClient(t)
	ctx := t.Context()

	// Start query execution.
	startOutput, err := client.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String("SELECT 1"),
		QueryExecutionContext: &types.QueryExecutionContext{
			Database: aws.String("default"),
		},
		ResultConfiguration: &types.ResultConfiguration{
			OutputLocation: aws.String("s3://test-bucket/results/"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("QueryExecutionId", "ResultMetadata")).Assert(t.Name(), startOutput)
}

func TestAthena_GetQueryExecution(t *testing.T) {
	client := newAthenaClient(t)
	ctx := t.Context()

	// Start query execution.
	startOutput, err := client.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String("SELECT * FROM test"),
	})
	if err != nil {
		t.Fatal(err)
	}

	queryExecutionID := *startOutput.QueryExecutionId

	// Get query execution.
	getOutput, err := client.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
		QueryExecutionId: aws.String(queryExecutionID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("QueryExecutionId", "SubmissionDateTime", "CompletionDateTime", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestAthena_GetQueryResults(t *testing.T) {
	client := newAthenaClient(t)
	ctx := t.Context()

	// Start query execution.
	startOutput, err := client.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String("SELECT column1, column2 FROM test"),
	})
	if err != nil {
		t.Fatal(err)
	}

	queryExecutionID := *startOutput.QueryExecutionId

	// Get query results.
	resultsOutput, err := client.GetQueryResults(ctx, &athena.GetQueryResultsInput{
		QueryExecutionId: aws.String(queryExecutionID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), resultsOutput)
}

func TestAthena_ListQueryExecutions(t *testing.T) {
	client := newAthenaClient(t)
	ctx := t.Context()

	// Start a few query executions.
	for i := 0; i < 3; i++ {
		_, err := client.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
			QueryString: aws.String("SELECT 1"),
		})
		if err != nil {
			t.Fatalf("failed to start query execution %d: %v", i, err)
		}
	}

	// List query executions.
	listOutput, err := client.ListQueryExecutions(ctx, &athena.ListQueryExecutionsInput{
		MaxResults: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(listOutput.QueryExecutionIds) == 0 {
		t.Fatal("no query execution IDs returned")
	}
}

func TestAthena_CreateAndDeleteWorkGroup(t *testing.T) {
	client := newAthenaClient(t)
	ctx := t.Context()
	workGroupName := "test-workgroup-create-delete"

	// Create workgroup.
	_, err := client.CreateWorkGroup(ctx, &athena.CreateWorkGroupInput{
		Name:        aws.String(workGroupName),
		Description: aws.String("Test workgroup for integration tests"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete workgroup.
	_, err = client.DeleteWorkGroup(ctx, &athena.DeleteWorkGroupInput{
		WorkGroup: aws.String(workGroupName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAthena_StopQueryExecution(t *testing.T) {
	client := newAthenaClient(t)
	ctx := t.Context()

	// Start query execution.
	startOutput, err := client.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String("SELECT * FROM large_table"),
	})
	if err != nil {
		t.Fatal(err)
	}

	queryExecutionID := *startOutput.QueryExecutionId

	// Stop query execution.
	_, err = client.StopQueryExecution(ctx, &athena.StopQueryExecutionInput{
		QueryExecutionId: aws.String(queryExecutionID),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAthena_QueryExecutionWithWorkGroup(t *testing.T) {
	client := newAthenaClient(t)
	ctx := t.Context()
	workGroupName := "test-workgroup-query"

	// Create workgroup.
	_, err := client.CreateWorkGroup(ctx, &athena.CreateWorkGroupInput{
		Name: aws.String(workGroupName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteWorkGroup(context.Background(), &athena.DeleteWorkGroupInput{
			WorkGroup:             aws.String(workGroupName),
			RecursiveDeleteOption: aws.Bool(true),
		})
	})

	// Start query execution in the workgroup.
	startOutput, err := client.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String("SELECT 1"),
		WorkGroup:   aws.String(workGroupName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get query execution and verify workgroup.
	getOutput, err := client.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
		QueryExecutionId: startOutput.QueryExecutionId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("QueryExecutionId", "SubmissionDateTime", "CompletionDateTime", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestAthena_QueryExecutionNotFound(t *testing.T) {
	client := newAthenaClient(t)
	ctx := t.Context()

	// Try to get a non-existent query execution.
	_, err := client.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
		QueryExecutionId: aws.String("non-existent-id"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent query execution")
	}
}

func TestAthena_WorkGroupNotFound(t *testing.T) {
	client := newAthenaClient(t)
	ctx := t.Context()

	// Try to start query in a non-existent workgroup.
	_, err := client.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String("SELECT 1"),
		WorkGroup:   aws.String("non-existent-workgroup"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent workgroup")
	}
}
