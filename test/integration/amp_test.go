//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/amp"
	"github.com/sivchari/golden"
)

func newAMPClient(t *testing.T) *amp.Client {
	t.Helper()

	return amp.NewFromConfig(awsConfig(t), func(o *amp.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestAMP_CreateWorkspace(t *testing.T) {
	client := newAMPClient(t)
	ctx := t.Context()

	result, err := client.CreateWorkspace(ctx, &amp.CreateWorkspaceInput{
		Alias: aws.String("create-amp-workspace"),
		Tags:  map[string]string{"env": "test"},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteWorkspace(context.Background(), &amp.DeleteWorkspaceInput{
			WorkspaceId: result.WorkspaceId,
		})
	})

	golden.New(t, golden.WithIgnoreFields("WorkspaceId", "Arn", "ResultMetadata")).Assert(t.Name(), result)
}

func TestAMP_DescribeWorkspace(t *testing.T) {
	client := newAMPClient(t)
	ctx := t.Context()

	created, err := client.CreateWorkspace(ctx, &amp.CreateWorkspaceInput{
		Alias: aws.String("describe-amp-workspace"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteWorkspace(context.Background(), &amp.DeleteWorkspaceInput{
			WorkspaceId: created.WorkspaceId,
		})
	})

	described, err := client.DescribeWorkspace(ctx, &amp.DescribeWorkspaceInput{
		WorkspaceId: created.WorkspaceId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("WorkspaceId", "Arn", "CreatedAt", "PrometheusEndpoint", "ResultMetadata")).Assert(t.Name(), described)
}

func TestAMP_ListWorkspaces(t *testing.T) {
	client := newAMPClient(t)
	ctx := t.Context()

	alias := "list-amp-workspace"

	created, err := client.CreateWorkspace(ctx, &amp.CreateWorkspaceInput{
		Alias: aws.String(alias),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteWorkspace(context.Background(), &amp.DeleteWorkspaceInput{
			WorkspaceId: created.WorkspaceId,
		})
	})

	listed, err := client.ListWorkspaces(ctx, &amp.ListWorkspacesInput{
		Alias: aws.String(alias),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("WorkspaceId", "Arn", "CreatedAt", "ResultMetadata", "NextToken")).Assert(t.Name(), listed)
}

func TestAMP_DeleteWorkspace(t *testing.T) {
	client := newAMPClient(t)
	ctx := t.Context()

	created, err := client.CreateWorkspace(ctx, &amp.CreateWorkspaceInput{
		Alias: aws.String("delete-amp-workspace"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteWorkspace(ctx, &amp.DeleteWorkspaceInput{
		WorkspaceId: created.WorkspaceId,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DescribeWorkspace(ctx, &amp.DescribeWorkspaceInput{
		WorkspaceId: created.WorkspaceId,
	})
	if err == nil {
		t.Fatal("expected error for deleted workspace")
	}
}
