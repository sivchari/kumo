//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker"
	"github.com/aws/aws-sdk-go-v2/service/sagemaker/types"
	"github.com/sivchari/golden"
)

func newSageMakerClient(t *testing.T) *sagemaker.Client {
	t.Helper()

	return sagemaker.NewFromConfig(awsConfig(t), func(o *sagemaker.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestSageMaker_CreateAndDeleteNotebookInstance(t *testing.T) {
	client := newSageMakerClient(t)
	ctx := t.Context()

	instanceName := "test-notebook"

	// Create notebook instance.
	createOutput, err := client.CreateNotebookInstance(ctx, &sagemaker.CreateNotebookInstanceInput{
		NotebookInstanceName: aws.String(instanceName),
		InstanceType:         types.InstanceTypeMlT2Medium,
		RoleArn:              aws.String("arn:aws:iam::123456789012:role/sagemaker-role"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("NotebookInstanceArn")).Assert(t.Name()+"_create", createOutput)

	t.Cleanup(func() {
		_, _ = client.DeleteNotebookInstance(context.Background(), &sagemaker.DeleteNotebookInstanceInput{
			NotebookInstanceName: aws.String(instanceName),
		})
	})

	// Describe notebook instance.
	descOutput, err := client.DescribeNotebookInstance(ctx, &sagemaker.DescribeNotebookInstanceInput{
		NotebookInstanceName: aws.String(instanceName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("NotebookInstanceArn", "CreationTime", "LastModifiedTime")).Assert(t.Name()+"_describe", descOutput)

	// Delete notebook instance.
	_, err = client.DeleteNotebookInstance(ctx, &sagemaker.DeleteNotebookInstanceInput{
		NotebookInstanceName: aws.String(instanceName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify notebook instance is deleted.
	_, err = client.DescribeNotebookInstance(ctx, &sagemaker.DescribeNotebookInstanceInput{
		NotebookInstanceName: aws.String(instanceName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestSageMaker_ListNotebookInstances(t *testing.T) {
	client := newSageMakerClient(t)
	ctx := t.Context()

	// Create multiple notebook instances.
	instanceNames := []string{"test-notebook-1", "test-notebook-2"}

	for _, name := range instanceNames {
		_, err := client.CreateNotebookInstance(ctx, &sagemaker.CreateNotebookInstanceInput{
			NotebookInstanceName: aws.String(name),
			InstanceType:         types.InstanceTypeMlT2Medium,
			RoleArn:              aws.String("arn:aws:iam::123456789012:role/sagemaker-role"),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		for _, name := range instanceNames {
			_, _ = client.DeleteNotebookInstance(context.Background(), &sagemaker.DeleteNotebookInstanceInput{
				NotebookInstanceName: aws.String(name),
			})
		}
	})

	// List notebook instances.
	listOutput, err := client.ListNotebookInstances(ctx, &sagemaker.ListNotebookInstancesInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOutput.NotebookInstances) < 2 {
		t.Errorf("expected at least 2 notebook instances, got %d", len(listOutput.NotebookInstances))
	}
}

func TestSageMaker_CreateAndDescribeTrainingJob(t *testing.T) {
	client := newSageMakerClient(t)
	ctx := t.Context()

	jobName := "test-training-job"

	// Create training job.
	createOutput, err := client.CreateTrainingJob(ctx, &sagemaker.CreateTrainingJobInput{
		TrainingJobName: aws.String(jobName),
		AlgorithmSpecification: &types.AlgorithmSpecification{
			TrainingImage:     aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-image:latest"),
			TrainingInputMode: types.TrainingInputModeFile,
		},
		RoleArn: aws.String("arn:aws:iam::123456789012:role/sagemaker-role"),
		OutputDataConfig: &types.OutputDataConfig{
			S3OutputPath: aws.String("s3://my-bucket/output"),
		},
		ResourceConfig: &types.ResourceConfig{
			InstanceType:   types.TrainingInstanceTypeMlM4Xlarge,
			InstanceCount:  aws.Int32(1),
			VolumeSizeInGB: aws.Int32(50),
		},
		StoppingCondition: &types.StoppingCondition{
			MaxRuntimeInSeconds: aws.Int32(3600),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TrainingJobArn")).Assert(t.Name()+"_create", createOutput)

	// Describe training job.
	descOutput, err := client.DescribeTrainingJob(ctx, &sagemaker.DescribeTrainingJobInput{
		TrainingJobName: aws.String(jobName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TrainingJobArn", "CreationTime", "TrainingStartTime", "TrainingEndTime", "LastModifiedTime")).Assert(t.Name()+"_describe", descOutput)
}

func TestSageMaker_CreateAndDeleteModel(t *testing.T) {
	client := newSageMakerClient(t)
	ctx := t.Context()

	modelName := "test-model"

	// Create model.
	createOutput, err := client.CreateModel(ctx, &sagemaker.CreateModelInput{
		ModelName: aws.String(modelName),
		PrimaryContainer: &types.ContainerDefinition{
			Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-image:latest"),
		},
		ExecutionRoleArn: aws.String("arn:aws:iam::123456789012:role/sagemaker-role"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ModelArn")).Assert(t.Name()+"_create", createOutput)

	t.Cleanup(func() {
		_, _ = client.DeleteModel(context.Background(), &sagemaker.DeleteModelInput{
			ModelName: aws.String(modelName),
		})
	})

	// Delete model.
	_, err = client.DeleteModel(ctx, &sagemaker.DeleteModelInput{
		ModelName: aws.String(modelName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Try to delete again (should fail).
	_, err = client.DeleteModel(ctx, &sagemaker.DeleteModelInput{
		ModelName: aws.String(modelName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestSageMaker_CreateAndDeleteEndpoint(t *testing.T) {
	client := newSageMakerClient(t)
	ctx := t.Context()

	endpointName := "test-endpoint"
	endpointConfigName := "test-endpoint-config"

	// Create endpoint.
	createOutput, err := client.CreateEndpoint(ctx, &sagemaker.CreateEndpointInput{
		EndpointName:       aws.String(endpointName),
		EndpointConfigName: aws.String(endpointConfigName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("EndpointArn")).Assert(t.Name()+"_create", createOutput)

	t.Cleanup(func() {
		_, _ = client.DeleteEndpoint(context.Background(), &sagemaker.DeleteEndpointInput{
			EndpointName: aws.String(endpointName),
		})
	})

	// Describe endpoint.
	descOutput, err := client.DescribeEndpoint(ctx, &sagemaker.DescribeEndpointInput{
		EndpointName: aws.String(endpointName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("EndpointArn", "CreationTime", "LastModifiedTime")).Assert(t.Name()+"_describe", descOutput)

	// Delete endpoint.
	_, err = client.DeleteEndpoint(ctx, &sagemaker.DeleteEndpointInput{
		EndpointName: aws.String(endpointName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify endpoint is deleted.
	_, err = client.DescribeEndpoint(ctx, &sagemaker.DescribeEndpointInput{
		EndpointName: aws.String(endpointName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestSageMaker_NotebookInstanceNotFound(t *testing.T) {
	client := newSageMakerClient(t)
	ctx := t.Context()

	// Describe non-existent notebook instance.
	_, err := client.DescribeNotebookInstance(ctx, &sagemaker.DescribeNotebookInstanceInput{
		NotebookInstanceName: aws.String("non-existent-notebook"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent notebook instance.
	_, err = client.DeleteNotebookInstance(ctx, &sagemaker.DeleteNotebookInstanceInput{
		NotebookInstanceName: aws.String("non-existent-notebook"),
	})
	if err == nil {
		t.Error("expected error")
	}
}
