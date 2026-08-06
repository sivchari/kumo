//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	"github.com/sivchari/golden"
)

func newElasticBeanstalkClient(t *testing.T) *elasticbeanstalk.Client {
	t.Helper()

	return elasticbeanstalk.NewFromConfig(awsConfig(t), func(o *elasticbeanstalk.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestElasticBeanstalk_CreateAndDeleteApplication(t *testing.T) {
	client := newElasticBeanstalkClient(t)
	ctx := t.Context()
	appName := "test-app"

	createResult, err := client.CreateApplication(ctx, &elasticbeanstalk.CreateApplicationInput{
		ApplicationName: aws.String(appName),
		Description:     aws.String("test application"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ApplicationArn", "DateCreated", "DateUpdated", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	// Delete
	_, err = client.DeleteApplication(ctx, &elasticbeanstalk.DeleteApplicationInput{
		ApplicationName: aws.String(appName),
	})
	if err != nil {
		t.Fatalf("failed to delete application: %v", err)
	}
}

func TestElasticBeanstalk_DescribeApplications(t *testing.T) {
	client := newElasticBeanstalkClient(t)
	ctx := t.Context()
	appName := "test-describe-app"

	_, err := client.CreateApplication(ctx, &elasticbeanstalk.CreateApplicationInput{
		ApplicationName: aws.String(appName),
		Description:     aws.String("describe test"),
	})
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApplication(t.Context(), &elasticbeanstalk.DeleteApplicationInput{
			ApplicationName: aws.String(appName),
		})
	})

	describeResult, err := client.DescribeApplications(ctx, &elasticbeanstalk.DescribeApplicationsInput{
		ApplicationNames: []string{appName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ApplicationArn", "DateCreated", "DateUpdated", "ResultMetadata")).Assert(t.Name(), describeResult)
}

func TestElasticBeanstalk_UpdateApplication(t *testing.T) {
	client := newElasticBeanstalkClient(t)
	ctx := t.Context()
	appName := "test-update-app"

	_, err := client.CreateApplication(ctx, &elasticbeanstalk.CreateApplicationInput{
		ApplicationName: aws.String(appName),
	})
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApplication(t.Context(), &elasticbeanstalk.DeleteApplicationInput{
			ApplicationName: aws.String(appName),
		})
	})

	updateResult, err := client.UpdateApplication(ctx, &elasticbeanstalk.UpdateApplicationInput{
		ApplicationName: aws.String(appName),
		Description:     aws.String("updated description"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ApplicationArn", "DateCreated", "DateUpdated", "ResultMetadata")).Assert(t.Name(), updateResult)
}

func TestElasticBeanstalk_CreateAndTerminateEnvironment(t *testing.T) {
	client := newElasticBeanstalkClient(t)
	ctx := t.Context()
	appName := "test-env-app"
	envName := "test-env"

	_, err := client.CreateApplication(ctx, &elasticbeanstalk.CreateApplicationInput{
		ApplicationName: aws.String(appName),
	})
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApplication(t.Context(), &elasticbeanstalk.DeleteApplicationInput{
			ApplicationName: aws.String(appName),
		})
	})

	createResult, err := client.CreateEnvironment(ctx, &elasticbeanstalk.CreateEnvironmentInput{
		ApplicationName: aws.String(appName),
		EnvironmentName: aws.String(envName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("EnvironmentId", "EnvironmentArn", "DateCreated", "DateUpdated", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	// Terminate
	terminateResult, err := client.TerminateEnvironment(ctx, &elasticbeanstalk.TerminateEnvironmentInput{
		EnvironmentName: aws.String(envName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("EnvironmentId", "EnvironmentArn", "DateCreated", "DateUpdated", "ResultMetadata")).Assert(t.Name()+"_terminate", terminateResult)
}

func TestElasticBeanstalk_DescribeEnvironments(t *testing.T) {
	client := newElasticBeanstalkClient(t)
	ctx := t.Context()
	appName := "test-desc-env-app"
	envName := "test-desc-env"

	_, err := client.CreateApplication(ctx, &elasticbeanstalk.CreateApplicationInput{
		ApplicationName: aws.String(appName),
	})
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.TerminateEnvironment(t.Context(), &elasticbeanstalk.TerminateEnvironmentInput{
			EnvironmentName: aws.String(envName),
		})
		_, _ = client.DeleteApplication(t.Context(), &elasticbeanstalk.DeleteApplicationInput{
			ApplicationName: aws.String(appName),
		})
	})

	_, err = client.CreateEnvironment(ctx, &elasticbeanstalk.CreateEnvironmentInput{
		ApplicationName: aws.String(appName),
		EnvironmentName: aws.String(envName),
	})
	if err != nil {
		t.Fatalf("failed to create environment: %v", err)
	}

	describeResult, err := client.DescribeEnvironments(ctx, &elasticbeanstalk.DescribeEnvironmentsInput{
		EnvironmentNames: []string{envName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("EnvironmentId", "EnvironmentArn", "DateCreated", "DateUpdated", "ResultMetadata")).Assert(t.Name(), describeResult)
}

func TestElasticBeanstalk_DuplicateApplication(t *testing.T) {
	client := newElasticBeanstalkClient(t)
	ctx := t.Context()
	appName := "test-dup-app"

	_, err := client.CreateApplication(ctx, &elasticbeanstalk.CreateApplicationInput{
		ApplicationName: aws.String(appName),
	})
	if err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApplication(t.Context(), &elasticbeanstalk.DeleteApplicationInput{
			ApplicationName: aws.String(appName),
		})
	})

	// Duplicate application - error case, skip golden test.
	_, err = client.CreateApplication(ctx, &elasticbeanstalk.CreateApplicationInput{
		ApplicationName: aws.String(appName),
	})
	if err == nil {
		t.Fatal("expected error for duplicate application")
	}
}
