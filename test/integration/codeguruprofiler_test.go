//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codeguruprofiler"
	"github.com/aws/aws-sdk-go-v2/service/codeguruprofiler/types"
	"github.com/sivchari/golden"
)

func newCodeGuruProfilerClient(t *testing.T) *codeguruprofiler.Client {
	t.Helper()

	return codeguruprofiler.NewFromConfig(awsConfig(t), func(o *codeguruprofiler.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestCodeGuruProfiler_CreateProfilingGroup(t *testing.T) {
	client := newCodeGuruProfilerClient(t)
	ctx := t.Context()

	result, err := client.CreateProfilingGroup(ctx, &codeguruprofiler.CreateProfilingGroupInput{
		ProfilingGroupName: aws.String("test-group"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruProfiler_CreateProfilingGroupWithConfig(t *testing.T) {
	client := newCodeGuruProfilerClient(t)
	ctx := t.Context()

	result, err := client.CreateProfilingGroup(ctx, &codeguruprofiler.CreateProfilingGroupInput{
		ProfilingGroupName: aws.String("config-group"),
		ComputePlatform:    types.ComputePlatformAwslambda,
		AgentOrchestrationConfig: &types.AgentOrchestrationConfig{
			ProfilingEnabled: aws.Bool(false),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruProfiler_DescribeProfilingGroup(t *testing.T) {
	client := newCodeGuruProfilerClient(t)
	ctx := t.Context()

	_, err := client.CreateProfilingGroup(ctx, &codeguruprofiler.CreateProfilingGroupInput{
		ProfilingGroupName: aws.String("describe-group"),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.DescribeProfilingGroup(ctx, &codeguruprofiler.DescribeProfilingGroupInput{
		ProfilingGroupName: aws.String("describe-group"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruProfiler_UpdateProfilingGroup(t *testing.T) {
	client := newCodeGuruProfilerClient(t)
	ctx := t.Context()

	_, err := client.CreateProfilingGroup(ctx, &codeguruprofiler.CreateProfilingGroupInput{
		ProfilingGroupName: aws.String("update-group"),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.UpdateProfilingGroup(ctx, &codeguruprofiler.UpdateProfilingGroupInput{
		ProfilingGroupName: aws.String("update-group"),
		AgentOrchestrationConfig: &types.AgentOrchestrationConfig{
			ProfilingEnabled: aws.Bool(false),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruProfiler_DeleteProfilingGroup(t *testing.T) {
	client := newCodeGuruProfilerClient(t)
	ctx := t.Context()

	_, err := client.CreateProfilingGroup(ctx, &codeguruprofiler.CreateProfilingGroupInput{
		ProfilingGroupName: aws.String("delete-group"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteProfilingGroup(ctx, &codeguruprofiler.DeleteProfilingGroupInput{
		ProfilingGroupName: aws.String("delete-group"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DescribeProfilingGroup(ctx, &codeguruprofiler.DescribeProfilingGroupInput{
		ProfilingGroupName: aws.String("delete-group"),
	})
	if err == nil {
		t.Fatal("expected error for deleted profiling group")
	}
}

func TestCodeGuruProfiler_ListProfilingGroups(t *testing.T) {
	client := newCodeGuruProfilerClient(t)
	ctx := t.Context()

	_, err := client.CreateProfilingGroup(ctx, &codeguruprofiler.CreateProfilingGroupInput{
		ProfilingGroupName: aws.String("list-group"),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListProfilingGroups(ctx, &codeguruprofiler.ListProfilingGroupsInput{})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestCodeGuruProfiler_ProfilingGroupNotFound(t *testing.T) {
	client := newCodeGuruProfilerClient(t)
	ctx := t.Context()

	_, err := client.DescribeProfilingGroup(ctx, &codeguruprofiler.DescribeProfilingGroupInput{
		ProfilingGroupName: aws.String("nonexistent-group"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent profiling group")
	}
}
