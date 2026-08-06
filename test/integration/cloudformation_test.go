//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/sivchari/golden"
)

const (
	testTemplate = `{
		"AWSTemplateFormatVersion": "2010-09-09",
		"Description": "Test template for CloudFormation integration tests",
		"Resources": {
			"TestBucket": {
				"Type": "AWS::S3::Bucket",
				"Properties": {
					"BucketName": "test-bucket"
				}
			}
		}
	}`

	testTemplateWithParams = `{
		"AWSTemplateFormatVersion": "2010-09-09",
		"Description": "Test template with parameters",
		"Parameters": {
			"BucketName": {
				"Type": "String",
				"Default": "default-bucket",
				"Description": "Name of the S3 bucket"
			}
		},
		"Resources": {
			"TestBucket": {
				"Type": "AWS::S3::Bucket",
				"Properties": {
					"BucketName": {"Ref": "BucketName"}
				}
			}
		}
	}`
)

func newCloudFormationClient(t *testing.T) *cloudformation.Client {
	t.Helper()

	return cloudformation.NewFromConfig(awsConfig(t), func(o *cloudformation.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestCloudFormation_CreateAndDeleteStack(t *testing.T) {
	client := newCloudFormationClient(t)
	ctx := t.Context()

	stackName := "test-stack-create-delete"

	// Create stack.
	createOutput, err := client.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(testTemplate),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("StackId"))
	g.Assert(t.Name()+"_create", createOutput)

	t.Cleanup(func() {
		_, _ = client.DeleteStack(context.Background(), &cloudformation.DeleteStackInput{
			StackName: aws.String(stackName),
		})
	})

	// Verify stack was created.
	descOutput, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t, golden.WithIgnoreFields("StackId", "CreationTime", "LastUpdatedTime"))
	g2.Assert(t.Name()+"_describe", descOutput)

	// Delete stack.
	_, err = client.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify stack is deleted (should return not found).
	_, err = client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestCloudFormation_DescribeStacks(t *testing.T) {
	client := newCloudFormationClient(t)
	ctx := t.Context()

	stackName := "test-stack-describe"

	// Create stack.
	_, err := client.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(testTemplate),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteStack(context.Background(), &cloudformation.DeleteStackInput{
			StackName: aws.String(stackName),
		})
	})

	// Describe specific stack.
	descOutput, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("StackId", "CreationTime", "LastUpdatedTime"))
	g.Assert(t.Name()+"_specific", descOutput)

	// Describe all stacks (without filter).
	allOutput, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(allOutput.Stacks) < 1 {
		t.Error("expected at least 1 stack")
	}
}

func TestCloudFormation_ListStacks(t *testing.T) {
	client := newCloudFormationClient(t)
	ctx := t.Context()

	stackName := "test-stack-list"

	// Create stack.
	_, err := client.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(testTemplate),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteStack(context.Background(), &cloudformation.DeleteStackInput{
			StackName: aws.String(stackName),
		})
	})

	// List all stacks.
	listOutput, err := client.ListStacks(ctx, &cloudformation.ListStacksInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(listOutput.StackSummaries) < 1 {
		t.Error("expected at least 1 stack summary")
	}

	// Find our stack in the list.
	found := false

	for _, summary := range listOutput.StackSummaries {
		if *summary.StackName == stackName {
			found = true

			if string(summary.StackStatus) != "CREATE_COMPLETE" {
				t.Errorf("expected CREATE_COMPLETE, got %s", summary.StackStatus)
			}

			break
		}
	}

	if !found {
		t.Error("Stack not found in ListStacks response")
	}
}

func TestCloudFormation_UpdateStack(t *testing.T) {
	client := newCloudFormationClient(t)
	ctx := t.Context()

	stackName := "test-stack-update"

	// Create stack.
	_, err := client.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(testTemplate),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteStack(context.Background(), &cloudformation.DeleteStackInput{
			StackName: aws.String(stackName),
		})
	})

	// Update stack with new template.
	updatedTemplate := `{
		"AWSTemplateFormatVersion": "2010-09-09",
		"Description": "Updated test template",
		"Resources": {
			"TestBucket": {
				"Type": "AWS::S3::Bucket",
				"Properties": {
					"BucketName": "updated-bucket"
				}
			},
			"TestBucket2": {
				"Type": "AWS::S3::Bucket",
				"Properties": {
					"BucketName": "new-bucket"
				}
			}
		}
	}`

	updateOutput, err := client.UpdateStack(ctx, &cloudformation.UpdateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(updatedTemplate),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("StackId"))
	g.Assert(t.Name()+"_update", updateOutput)

	// Verify stack was updated.
	descOutput, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t, golden.WithIgnoreFields("StackId", "CreationTime", "LastUpdatedTime"))
	g2.Assert(t.Name()+"_describe", descOutput)
}

func TestCloudFormation_DescribeStackResources(t *testing.T) {
	client := newCloudFormationClient(t)
	ctx := t.Context()

	stackName := "test-stack-resources"

	// Create stack.
	_, err := client.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(testTemplate),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteStack(context.Background(), &cloudformation.DeleteStackInput{
			StackName: aws.String(stackName),
		})
	})

	// Describe stack resources.
	resourcesOutput, err := client.DescribeStackResources(ctx, &cloudformation.DescribeStackResourcesInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("StackId", "PhysicalResourceId", "Timestamp"))
	g.Assert(t.Name(), resourcesOutput)
}

func TestCloudFormation_GetTemplate(t *testing.T) {
	client := newCloudFormationClient(t)
	ctx := t.Context()

	stackName := "test-stack-get-template"

	// Create stack.
	_, err := client.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(testTemplate),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteStack(context.Background(), &cloudformation.DeleteStackInput{
			StackName: aws.String(stackName),
		})
	})

	// Get template.
	templateOutput, err := client.GetTemplate(ctx, &cloudformation.GetTemplateInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t)
	g.Assert(t.Name(), templateOutput)
}

func TestCloudFormation_ValidateTemplate(t *testing.T) {
	client := newCloudFormationClient(t)
	ctx := t.Context()

	// Validate template with parameters.
	validateOutput, err := client.ValidateTemplate(ctx, &cloudformation.ValidateTemplateInput{
		TemplateBody: aws.String(testTemplateWithParams),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t)
	g.Assert(t.Name()+"_valid", validateOutput)

	// Validate invalid template.
	_, err = client.ValidateTemplate(ctx, &cloudformation.ValidateTemplateInput{
		TemplateBody: aws.String("invalid json"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestCloudFormation_StackNotFound(t *testing.T) {
	client := newCloudFormationClient(t)
	ctx := t.Context()

	// Describe non-existent stack.
	_, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String("non-existent-stack"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent stack.
	_, err = client.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: aws.String("non-existent-stack"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Update non-existent stack.
	_, err = client.UpdateStack(ctx, &cloudformation.UpdateStackInput{
		StackName:    aws.String("non-existent-stack"),
		TemplateBody: aws.String(testTemplate),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestCloudFormation_DuplicateStack(t *testing.T) {
	client := newCloudFormationClient(t)
	ctx := t.Context()

	stackName := "test-stack-duplicate"

	// Create stack.
	_, err := client.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(testTemplate),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteStack(context.Background(), &cloudformation.DeleteStackInput{
			StackName: aws.String(stackName),
		})
	})

	// Try to create duplicate stack.
	_, err = client.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(testTemplate),
	})
	if err == nil {
		t.Error("expected error")
	}
}
