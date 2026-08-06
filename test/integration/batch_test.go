//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	"github.com/aws/aws-sdk-go-v2/service/batch/types"
	"github.com/sivchari/golden"
)

func TestBatch_CreateComputeEnvironment(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createBatchClient(t)

	ceName := "test-compute-env"

	// Create compute environment.
	result, err := client.CreateComputeEnvironment(ctx, &batch.CreateComputeEnvironmentInput{
		ComputeEnvironmentName: aws.String(ceName),
		Type:                   types.CETypeManaged,
		State:                  types.CEStateEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ComputeEnvironmentArn", "ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteComputeEnvironment(ctx, &batch.DeleteComputeEnvironmentInput{
		ComputeEnvironment: aws.String(ceName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBatch_DescribeComputeEnvironments(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createBatchClient(t)

	ceName := "describe-test-ce"

	// Create compute environment.
	_, err := client.CreateComputeEnvironment(ctx, &batch.CreateComputeEnvironmentInput{
		ComputeEnvironmentName: aws.String(ceName),
		Type:                   types.CETypeManaged,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Describe compute environments.
	result, err := client.DescribeComputeEnvironments(ctx, &batch.DescribeComputeEnvironmentsInput{
		ComputeEnvironments: []string{ceName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ComputeEnvironmentArn", "EcsClusterArn", "Uuid", "ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteComputeEnvironment(ctx, &batch.DeleteComputeEnvironmentInput{
		ComputeEnvironment: aws.String(ceName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBatch_CreateJobQueue(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createBatchClient(t)

	ceName := "jq-test-ce"
	jqName := "test-job-queue"

	// Create compute environment first.
	ceResult, err := client.CreateComputeEnvironment(ctx, &batch.CreateComputeEnvironmentInput{
		ComputeEnvironmentName: aws.String(ceName),
		Type:                   types.CETypeManaged,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create job queue.
	jqResult, err := client.CreateJobQueue(ctx, &batch.CreateJobQueueInput{
		JobQueueName: aws.String(jqName),
		Priority:     aws.Int32(1),
		ComputeEnvironmentOrder: []types.ComputeEnvironmentOrder{
			{
				ComputeEnvironment: ceResult.ComputeEnvironmentArn,
				Order:              aws.Int32(1),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("JobQueueArn", "ResultMetadata")).Assert(t.Name(), jqResult)

	// Clean up.
	_, err = client.DeleteJobQueue(ctx, &batch.DeleteJobQueueInput{
		JobQueue: aws.String(jqName),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteComputeEnvironment(ctx, &batch.DeleteComputeEnvironmentInput{
		ComputeEnvironment: aws.String(ceName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBatch_DescribeJobQueues(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createBatchClient(t)

	ceName := "describe-jq-test-ce"
	jqName := "describe-test-jq"

	// Create compute environment first.
	ceResult, err := client.CreateComputeEnvironment(ctx, &batch.CreateComputeEnvironmentInput{
		ComputeEnvironmentName: aws.String(ceName),
		Type:                   types.CETypeManaged,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create job queue.
	_, err = client.CreateJobQueue(ctx, &batch.CreateJobQueueInput{
		JobQueueName: aws.String(jqName),
		Priority:     aws.Int32(1),
		ComputeEnvironmentOrder: []types.ComputeEnvironmentOrder{
			{
				ComputeEnvironment: ceResult.ComputeEnvironmentArn,
				Order:              aws.Int32(1),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Describe job queues.
	result, err := client.DescribeJobQueues(ctx, &batch.DescribeJobQueuesInput{
		JobQueues: []string{jqName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("JobQueueArn", "ComputeEnvironmentOrder", "ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteJobQueue(ctx, &batch.DeleteJobQueueInput{
		JobQueue: aws.String(jqName),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteComputeEnvironment(ctx, &batch.DeleteComputeEnvironmentInput{
		ComputeEnvironment: aws.String(ceName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBatch_RegisterJobDefinition(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createBatchClient(t)

	jdName := "test-job-definition"

	// Register job definition.
	result, err := client.RegisterJobDefinition(ctx, &batch.RegisterJobDefinitionInput{
		JobDefinitionName: aws.String(jdName),
		Type:              types.JobDefinitionTypeContainer,
		ContainerProperties: &types.ContainerProperties{
			Image:  aws.String("busybox"),
			Vcpus:  aws.Int32(1),
			Memory: aws.Int32(512),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("JobDefinitionArn", "Revision", "ResultMetadata")).Assert(t.Name(), result)
}

func TestBatch_SubmitJob(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createBatchClient(t)

	ceName := "submit-test-ce"
	jqName := "submit-test-jq"
	jdName := "submit-test-jd"

	// Create compute environment.
	ceResult, err := client.CreateComputeEnvironment(ctx, &batch.CreateComputeEnvironmentInput{
		ComputeEnvironmentName: aws.String(ceName),
		Type:                   types.CETypeManaged,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create job queue.
	jqResult, err := client.CreateJobQueue(ctx, &batch.CreateJobQueueInput{
		JobQueueName: aws.String(jqName),
		Priority:     aws.Int32(1),
		ComputeEnvironmentOrder: []types.ComputeEnvironmentOrder{
			{
				ComputeEnvironment: ceResult.ComputeEnvironmentArn,
				Order:              aws.Int32(1),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Register job definition.
	jdResult, err := client.RegisterJobDefinition(ctx, &batch.RegisterJobDefinitionInput{
		JobDefinitionName: aws.String(jdName),
		Type:              types.JobDefinitionTypeContainer,
		ContainerProperties: &types.ContainerProperties{
			Image:  aws.String("busybox"),
			Vcpus:  aws.Int32(1),
			Memory: aws.Int32(512),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Submit job.
	jobResult, err := client.SubmitJob(ctx, &batch.SubmitJobInput{
		JobName:       aws.String("test-job"),
		JobQueue:      jqResult.JobQueueArn,
		JobDefinition: jdResult.JobDefinitionArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("JobArn", "JobId", "ResultMetadata")).Assert(t.Name(), jobResult)

	// Clean up.
	_, err = client.DeleteJobQueue(ctx, &batch.DeleteJobQueueInput{
		JobQueue: aws.String(jqName),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteComputeEnvironment(ctx, &batch.DeleteComputeEnvironmentInput{
		ComputeEnvironment: aws.String(ceName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBatch_DescribeJobs(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createBatchClient(t)

	ceName := "describe-job-test-ce"
	jqName := "describe-job-test-jq"
	jdName := "describe-job-test-jd"

	// Create compute environment.
	ceResult, err := client.CreateComputeEnvironment(ctx, &batch.CreateComputeEnvironmentInput{
		ComputeEnvironmentName: aws.String(ceName),
		Type:                   types.CETypeManaged,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create job queue.
	jqResult, err := client.CreateJobQueue(ctx, &batch.CreateJobQueueInput{
		JobQueueName: aws.String(jqName),
		Priority:     aws.Int32(1),
		ComputeEnvironmentOrder: []types.ComputeEnvironmentOrder{
			{
				ComputeEnvironment: ceResult.ComputeEnvironmentArn,
				Order:              aws.Int32(1),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Register job definition.
	jdResult, err := client.RegisterJobDefinition(ctx, &batch.RegisterJobDefinitionInput{
		JobDefinitionName: aws.String(jdName),
		Type:              types.JobDefinitionTypeContainer,
		ContainerProperties: &types.ContainerProperties{
			Image:  aws.String("busybox"),
			Vcpus:  aws.Int32(1),
			Memory: aws.Int32(512),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Submit job.
	jobResult, err := client.SubmitJob(ctx, &batch.SubmitJobInput{
		JobName:       aws.String("describe-test-job"),
		JobQueue:      jqResult.JobQueueArn,
		JobDefinition: jdResult.JobDefinitionArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Describe job.
	describeResult, err := client.DescribeJobs(ctx, &batch.DescribeJobsInput{
		Jobs: []string{*jobResult.JobId},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("JobArn", "JobId", "JobDefinition", "JobQueue", "CreatedAt", "StartedAt", "StoppedAt", "ResultMetadata")).Assert(t.Name(), describeResult)

	// Clean up.
	_, err = client.DeleteJobQueue(ctx, &batch.DeleteJobQueueInput{
		JobQueue: aws.String(jqName),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteComputeEnvironment(ctx, &batch.DeleteComputeEnvironmentInput{
		ComputeEnvironment: aws.String(ceName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBatch_TerminateJob(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createBatchClient(t)

	ceName := "terminate-job-test-ce"
	jqName := "terminate-job-test-jq"
	jdName := "terminate-job-test-jd"

	// Create compute environment.
	ceResult, err := client.CreateComputeEnvironment(ctx, &batch.CreateComputeEnvironmentInput{
		ComputeEnvironmentName: aws.String(ceName),
		Type:                   types.CETypeManaged,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create job queue.
	jqResult, err := client.CreateJobQueue(ctx, &batch.CreateJobQueueInput{
		JobQueueName: aws.String(jqName),
		Priority:     aws.Int32(1),
		ComputeEnvironmentOrder: []types.ComputeEnvironmentOrder{
			{
				ComputeEnvironment: ceResult.ComputeEnvironmentArn,
				Order:              aws.Int32(1),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Register job definition.
	jdResult, err := client.RegisterJobDefinition(ctx, &batch.RegisterJobDefinitionInput{
		JobDefinitionName: aws.String(jdName),
		Type:              types.JobDefinitionTypeContainer,
		ContainerProperties: &types.ContainerProperties{
			Image:  aws.String("busybox"),
			Vcpus:  aws.Int32(1),
			Memory: aws.Int32(512),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Submit job.
	jobResult, err := client.SubmitJob(ctx, &batch.SubmitJobInput{
		JobName:       aws.String("terminate-test-job"),
		JobQueue:      jqResult.JobQueueArn,
		JobDefinition: jdResult.JobDefinitionArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Terminate job.
	_, err = client.TerminateJob(ctx, &batch.TerminateJobInput{
		JobId:  jobResult.JobId,
		Reason: aws.String("Test termination"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify job is terminated.
	describeResult, err := client.DescribeJobs(ctx, &batch.DescribeJobsInput{
		Jobs: []string{*jobResult.JobId},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("JobArn", "JobId", "JobDefinition", "JobQueue", "CreatedAt", "StartedAt", "StoppedAt", "StatusReason", "ResultMetadata")).Assert(t.Name(), describeResult)

	// Clean up.
	_, err = client.DeleteJobQueue(ctx, &batch.DeleteJobQueueInput{
		JobQueue: aws.String(jqName),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteComputeEnvironment(ctx, &batch.DeleteComputeEnvironmentInput{
		ComputeEnvironment: aws.String(ceName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func createBatchClient(t *testing.T) *batch.Client {
	t.Helper()

	return batch.NewFromConfig(awsConfig(t), func(o *batch.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}
