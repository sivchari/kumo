//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/emrserverless"
	"github.com/aws/aws-sdk-go-v2/service/emrserverless/types"
	"github.com/sivchari/golden"
)

func newEMRServerlessClient(t *testing.T) *emrserverless.Client {
	t.Helper()

	return emrserverless.NewFromConfig(awsConfig(t), func(o *emrserverless.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestEMRServerless_CreateAndGetApplication(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Name:         aws.String("test-spark-app"),
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	applicationID := createOutput.ApplicationId

	t.Cleanup(func() {
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: applicationID,
		})
	})

	// Get application.
	getOutput, err := client.GetApplication(ctx, &emrserverless.GetApplicationInput{
		ApplicationId: applicationID,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ApplicationId",
		"Arn",
		"CreatedAt",
		"UpdatedAt",
		"ResultMetadata",
	)).Assert(t.Name(), getOutput)
}

func TestEMRServerless_CreateHiveApplication(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create Hive application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Name:         aws.String("test-hive-app"),
		Type:         aws.String("Hive"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
	})

	// Verify application type.
	getOutput, err := client.GetApplication(ctx, &emrserverless.GetApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ApplicationId",
		"Arn",
		"CreatedAt",
		"UpdatedAt",
		"ResultMetadata",
	)).Assert(t.Name(), getOutput)
}

func TestEMRServerless_ListApplications(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create multiple applications.
	appIDs := make([]*string, 0)

	for i := 0; i < 3; i++ {
		output, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
			Type:         aws.String("Spark"),
			ReleaseLabel: aws.String("emr-6.9.0"),
		})
		if err != nil {
			t.Fatal(err)
		}
		appIDs = append(appIDs, output.ApplicationId)
	}

	t.Cleanup(func() {
		for _, id := range appIDs {
			_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
				ApplicationId: id,
			})
		}
	})

	// List applications.
	listOutput, err := client.ListApplications(ctx, &emrserverless.ListApplicationsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOutput.Applications) < 3 {
		t.Errorf("expected at least 3 applications, got %d", len(listOutput.Applications))
	}
}

func TestEMRServerless_ListApplicationsWithStateFilter(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
	})

	// List with CREATED state filter.
	listOutput, err := client.ListApplications(ctx, &emrserverless.ListApplicationsInput{
		States: []types.ApplicationState{types.ApplicationStateCreated},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOutput.Applications) < 1 {
		t.Errorf("expected at least 1 application, got %d", len(listOutput.Applications))
	}

	// List with STARTED state filter (should not include new app).
	_, err = client.ListApplications(ctx, &emrserverless.ListApplicationsInput{
		States: []types.ApplicationState{types.ApplicationStateStarted},
	})
	if err != nil {
		t.Fatal(err)
	}
	// May be empty or contain previously started apps.
}

func TestEMRServerless_UpdateApplication(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
	})

	// Update application.
	_, err = client.UpdateApplication(ctx, &emrserverless.UpdateApplicationInput{
		ApplicationId: createOutput.ApplicationId,
		ReleaseLabel:  aws.String("emr-7.0.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify update.
	getOutput, err := client.GetApplication(ctx, &emrserverless.GetApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ApplicationId",
		"Arn",
		"CreatedAt",
		"UpdatedAt",
		"ResultMetadata",
	)).Assert(t.Name(), getOutput)
}

func TestEMRServerless_DeleteApplication(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete application.
	_, err = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.GetApplication(ctx, &emrserverless.GetApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestEMRServerless_StartAndStopApplication(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		// Stop before delete if needed.
		_, _ = client.StopApplication(context.Background(), &emrserverless.StopApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
	})

	// Start application.
	_, err = client.StartApplication(ctx, &emrserverless.StartApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify application is started.
	getOutput, err := client.GetApplication(ctx, &emrserverless.GetApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}
	if getOutput.Application.State != types.ApplicationStateStarted {
		t.Errorf("expected state %v, got %v", types.ApplicationStateStarted, getOutput.Application.State)
	}

	// Stop application.
	_, err = client.StopApplication(context.Background(), &emrserverless.StopApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify application is stopped.
	getOutput, err = client.GetApplication(ctx, &emrserverless.GetApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}
	if getOutput.Application.State != types.ApplicationStateStopped {
		t.Errorf("expected state %v, got %v", types.ApplicationStateStopped, getOutput.Application.State)
	}
}

func TestEMRServerless_StartJobRun(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.StopApplication(context.Background(), &emrserverless.StopApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
	})

	// Start application.
	_, err = client.StartApplication(ctx, &emrserverless.StartApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Start job run.
	jobOutput, err := client.StartJobRun(ctx, &emrserverless.StartJobRunInput{
		ApplicationId:    createOutput.ApplicationId,
		ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/test-execution-role"),
		JobDriver: &types.JobDriverMemberSparkSubmit{
			Value: types.SparkSubmit{
				EntryPoint: aws.String("s3://bucket/script.py"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ApplicationId",
		"Arn",
		"JobRunId",
		"ResultMetadata",
	)).Assert(t.Name(), jobOutput)
}

func TestEMRServerless_GetJobRun(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.StopApplication(context.Background(), &emrserverless.StopApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
	})

	// Start application.
	_, err = client.StartApplication(ctx, &emrserverless.StartApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Start job run.
	jobOutput, err := client.StartJobRun(ctx, &emrserverless.StartJobRunInput{
		ApplicationId:    createOutput.ApplicationId,
		ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/test-execution-role"),
		JobDriver: &types.JobDriverMemberSparkSubmit{
			Value: types.SparkSubmit{
				EntryPoint: aws.String("s3://bucket/script.py"),
			},
		},
		Name: aws.String("test-job"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get job run.
	getOutput, err := client.GetJobRun(ctx, &emrserverless.GetJobRunInput{
		ApplicationId: createOutput.ApplicationId,
		JobRunId:      jobOutput.JobRunId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ApplicationId",
		"Arn",
		"JobRunId",
		"CreatedAt",
		"UpdatedAt",
		"ResultMetadata",
	)).Assert(t.Name(), getOutput)
}

func TestEMRServerless_ListJobRuns(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.StopApplication(context.Background(), &emrserverless.StopApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
	})

	// Start application.
	_, err = client.StartApplication(ctx, &emrserverless.StartApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Start multiple job runs.
	for i := 0; i < 3; i++ {
		_, err = client.StartJobRun(ctx, &emrserverless.StartJobRunInput{
			ApplicationId:    createOutput.ApplicationId,
			ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/test-execution-role"),
			JobDriver: &types.JobDriverMemberSparkSubmit{
				Value: types.SparkSubmit{
					EntryPoint: aws.String("s3://bucket/script.py"),
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// List job runs.
	listOutput, err := client.ListJobRuns(ctx, &emrserverless.ListJobRunsInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOutput.JobRuns) < 3 {
		t.Errorf("expected at least 3 job runs, got %d", len(listOutput.JobRuns))
	}
}

func TestEMRServerless_CancelJobRun(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.StopApplication(context.Background(), &emrserverless.StopApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
	})

	// Start application.
	_, err = client.StartApplication(ctx, &emrserverless.StartApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Start job run.
	jobOutput, err := client.StartJobRun(ctx, &emrserverless.StartJobRunInput{
		ApplicationId:    createOutput.ApplicationId,
		ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/test-execution-role"),
		JobDriver: &types.JobDriverMemberSparkSubmit{
			Value: types.SparkSubmit{
				EntryPoint: aws.String("s3://bucket/script.py"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Cancel job run.
	_, err = client.CancelJobRun(ctx, &emrserverless.CancelJobRunInput{
		ApplicationId: createOutput.ApplicationId,
		JobRunId:      jobOutput.JobRunId,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify job is cancelled.
	getOutput, err := client.GetJobRun(ctx, &emrserverless.GetJobRunInput{
		ApplicationId: createOutput.ApplicationId,
		JobRunId:      jobOutput.JobRunId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ApplicationId",
		"Arn",
		"JobRunId",
		"CreatedAt",
		"UpdatedAt",
		"ResultMetadata",
	)).Assert(t.Name(), getOutput)
}

func TestEMRServerless_NotFoundErrors(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Get non-existent application.
	_, err := client.GetApplication(ctx, &emrserverless.GetApplicationInput{
		ApplicationId: aws.String("nonexistent"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent application.
	_, err = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
		ApplicationId: aws.String("nonexistent"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Start non-existent application.
	_, err = client.StartApplication(ctx, &emrserverless.StartApplicationInput{
		ApplicationId: aws.String("nonexistent"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Stop non-existent application.
	_, err = client.StopApplication(context.Background(), &emrserverless.StopApplicationInput{
		ApplicationId: aws.String("nonexistent"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestEMRServerless_ConflictErrors(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application.
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.StopApplication(context.Background(), &emrserverless.StopApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
	})

	// Try to stop a CREATED application (should fail).
	_, err = client.StopApplication(context.Background(), &emrserverless.StopApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err == nil {
		t.Error("expected error")
	}

	// Start application.
	_, err = client.StartApplication(ctx, &emrserverless.StartApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Try to start a STARTED application (should fail).
	_, err = client.StartApplication(ctx, &emrserverless.StartApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err == nil {
		t.Error("expected error")
	}

	// Try to delete a STARTED application (should fail).
	_, err = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestEMRServerless_AutoStartApplication(t *testing.T) {
	client := newEMRServerlessClient(t)
	ctx := t.Context()

	// Create application with auto-start enabled (default).
	createOutput, err := client.CreateApplication(ctx, &emrserverless.CreateApplicationInput{
		Type:         aws.String("Spark"),
		ReleaseLabel: aws.String("emr-6.9.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.StopApplication(context.Background(), &emrserverless.StopApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
		_, _ = client.DeleteApplication(context.Background(), &emrserverless.DeleteApplicationInput{
			ApplicationId: createOutput.ApplicationId,
		})
	})

	// Start job run on CREATED application - should auto-start.
	_, err = client.StartJobRun(ctx, &emrserverless.StartJobRunInput{
		ApplicationId:    createOutput.ApplicationId,
		ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/test-execution-role"),
		JobDriver: &types.JobDriverMemberSparkSubmit{
			Value: types.SparkSubmit{
				EntryPoint: aws.String("s3://bucket/script.py"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify application is now STARTED.
	getOutput, err := client.GetApplication(ctx, &emrserverless.GetApplicationInput{
		ApplicationId: createOutput.ApplicationId,
	})
	if err != nil {
		t.Fatal(err)
	}
	if getOutput.Application.State != types.ApplicationStateStarted {
		t.Errorf("expected state %v, got %v", types.ApplicationStateStarted, getOutput.Application.State)
	}
}
