//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/pipes"
	"github.com/aws/aws-sdk-go-v2/service/pipes/types"
	"github.com/sivchari/golden"
)

func newPipesClient(t *testing.T) *pipes.Client {
	t.Helper()

	return pipes.NewFromConfig(awsConfig(t), func(o *pipes.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestPipes_CreateAndDescribePipe(t *testing.T) {
	client := newPipesClient(t)
	ctx := t.Context()

	pipeName := "test-pipe-create-describe"
	source := "arn:aws:sqs:us-east-1:123456789012:test-source-queue"
	target := "arn:aws:lambda:us-east-1:123456789012:function:test-target"
	roleArn := "arn:aws:iam::123456789012:role/test-pipe-role"

	// Create pipe.
	createOutput, err := client.CreatePipe(ctx, &pipes.CreatePipeInput{
		Name:    aws.String(pipeName),
		Source:  aws.String(source),
		Target:  aws.String(target),
		RoleArn: aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/create", createOutput)

	t.Cleanup(func() {
		_, _ = client.DeletePipe(context.Background(), &pipes.DeletePipeInput{
			Name: aws.String(pipeName),
		})
	})

	// Describe pipe.
	descOutput, err := client.DescribePipe(ctx, &pipes.DescribePipeInput{
		Name: aws.String(pipeName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/describe", descOutput)
}

func TestPipes_CreatePipeWithStoppedState(t *testing.T) {
	client := newPipesClient(t)
	ctx := t.Context()

	pipeName := "test-pipe-stopped"
	source := "arn:aws:sqs:us-east-1:123456789012:test-source-queue"
	target := "arn:aws:lambda:us-east-1:123456789012:function:test-target"
	roleArn := "arn:aws:iam::123456789012:role/test-pipe-role"

	// Create pipe with STOPPED state.
	createOutput, err := client.CreatePipe(ctx, &pipes.CreatePipeInput{
		Name:         aws.String(pipeName),
		Source:       aws.String(source),
		Target:       aws.String(target),
		RoleArn:      aws.String(roleArn),
		DesiredState: types.RequestedPipeStateStopped,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/create", createOutput)

	t.Cleanup(func() {
		_, _ = client.DeletePipe(context.Background(), &pipes.DeletePipeInput{
			Name: aws.String(pipeName),
		})
	})

	// Describe pipe to verify state.
	descOutput, err := client.DescribePipe(ctx, &pipes.DescribePipeInput{
		Name: aws.String(pipeName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/describe", descOutput)
}

func TestPipes_UpdatePipe(t *testing.T) {
	client := newPipesClient(t)
	ctx := t.Context()

	pipeName := "test-pipe-update"
	source := "arn:aws:sqs:us-east-1:123456789012:test-source-queue"
	target := "arn:aws:lambda:us-east-1:123456789012:function:test-target"
	roleArn := "arn:aws:iam::123456789012:role/test-pipe-role"
	newRoleArn := "arn:aws:iam::123456789012:role/test-pipe-role-updated"
	description := "Updated description"

	// Create pipe.
	_, err := client.CreatePipe(ctx, &pipes.CreatePipeInput{
		Name:    aws.String(pipeName),
		Source:  aws.String(source),
		Target:  aws.String(target),
		RoleArn: aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeletePipe(context.Background(), &pipes.DeletePipeInput{
			Name: aws.String(pipeName),
		})
	})

	// Update pipe.
	updateOutput, err := client.UpdatePipe(ctx, &pipes.UpdatePipeInput{
		Name:        aws.String(pipeName),
		RoleArn:     aws.String(newRoleArn),
		Description: aws.String(description),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/update", updateOutput)

	// Verify update.
	descOutput, err := client.DescribePipe(ctx, &pipes.DescribePipeInput{
		Name: aws.String(pipeName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/describe", descOutput)
}

func TestPipes_DeletePipe(t *testing.T) {
	client := newPipesClient(t)
	ctx := t.Context()

	pipeName := "test-pipe-delete"
	source := "arn:aws:sqs:us-east-1:123456789012:test-source-queue"
	target := "arn:aws:lambda:us-east-1:123456789012:function:test-target"
	roleArn := "arn:aws:iam::123456789012:role/test-pipe-role"

	// Create pipe.
	_, err := client.CreatePipe(ctx, &pipes.CreatePipeInput{
		Name:    aws.String(pipeName),
		Source:  aws.String(source),
		Target:  aws.String(target),
		RoleArn: aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete pipe.
	deleteOutput, err := client.DeletePipe(ctx, &pipes.DeletePipeInput{
		Name: aws.String(pipeName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/delete", deleteOutput)

	// Verify pipe is deleted.
	_, err = client.DescribePipe(ctx, &pipes.DescribePipeInput{
		Name: aws.String(pipeName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPipes_ListPipes(t *testing.T) {
	client := newPipesClient(t)
	ctx := t.Context()

	// Create multiple pipes.
	pipeNames := []string{"test-pipe-list-1", "test-pipe-list-2", "test-pipe-list-3"}
	source := "arn:aws:sqs:us-east-1:123456789012:test-source-queue"
	target := "arn:aws:lambda:us-east-1:123456789012:function:test-target"
	roleArn := "arn:aws:iam::123456789012:role/test-pipe-role"

	for _, name := range pipeNames {
		_, err := client.CreatePipe(ctx, &pipes.CreatePipeInput{
			Name:    aws.String(name),
			Source:  aws.String(source),
			Target:  aws.String(target),
			RoleArn: aws.String(roleArn),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		for _, name := range pipeNames {
			_, _ = client.DeletePipe(context.Background(), &pipes.DeletePipeInput{
				Name: aws.String(name),
			})
		}
	})

	// List pipes.
	listOutput, err := client.ListPipes(ctx, &pipes.ListPipesInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOutput.Pipes) < 3 {
		t.Errorf("expected at least 3 pipes, got %d", len(listOutput.Pipes))
	}

	// List pipes with name prefix filter.
	listOutput, err = client.ListPipes(ctx, &pipes.ListPipesInput{
		NamePrefix: aws.String("test-pipe-list-"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/list_with_prefix", listOutput)
}

func TestPipes_StartAndStopPipe(t *testing.T) {
	client := newPipesClient(t)
	ctx := t.Context()

	pipeName := "test-pipe-start-stop"
	source := "arn:aws:sqs:us-east-1:123456789012:test-source-queue"
	target := "arn:aws:lambda:us-east-1:123456789012:function:test-target"
	roleArn := "arn:aws:iam::123456789012:role/test-pipe-role"

	// Create pipe with STOPPED state.
	_, err := client.CreatePipe(ctx, &pipes.CreatePipeInput{
		Name:         aws.String(pipeName),
		Source:       aws.String(source),
		Target:       aws.String(target),
		RoleArn:      aws.String(roleArn),
		DesiredState: types.RequestedPipeStateStopped,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeletePipe(context.Background(), &pipes.DeletePipeInput{
			Name: aws.String(pipeName),
		})
	})

	// Start pipe.
	startOutput, err := client.StartPipe(ctx, &pipes.StartPipeInput{
		Name: aws.String(pipeName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/start", startOutput)

	// Verify pipe is running.
	descOutput, err := client.DescribePipe(ctx, &pipes.DescribePipeInput{
		Name: aws.String(pipeName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/describe_after_start", descOutput)

	// Stop pipe.
	stopOutput, err := client.StopPipe(ctx, &pipes.StopPipeInput{
		Name: aws.String(pipeName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/stop", stopOutput)

	// Verify pipe is stopped.
	descOutput, err = client.DescribePipe(ctx, &pipes.DescribePipeInput{
		Name: aws.String(pipeName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/describe_after_stop", descOutput)
}

func TestPipes_TagOperations(t *testing.T) {
	client := newPipesClient(t)
	ctx := t.Context()

	pipeName := "test-pipe-tags"
	source := "arn:aws:sqs:us-east-1:123456789012:test-source-queue"
	target := "arn:aws:lambda:us-east-1:123456789012:function:test-target"
	roleArn := "arn:aws:iam::123456789012:role/test-pipe-role"

	// Create pipe with tags.
	initialTags := map[string]string{
		"Environment": "test",
		"Project":     "awsim",
	}
	createOutput, err := client.CreatePipe(ctx, &pipes.CreatePipeInput{
		Name:    aws.String(pipeName),
		Source:  aws.String(source),
		Target:  aws.String(target),
		RoleArn: aws.String(roleArn),
		Tags:    initialTags,
	})
	if err != nil {
		t.Fatal(err)
	}

	pipeArn := createOutput.Arn

	t.Cleanup(func() {
		_, _ = client.DeletePipe(context.Background(), &pipes.DeletePipeInput{
			Name: aws.String(pipeName),
		})
	})

	// List tags.
	listTagsOutput, err := client.ListTagsForResource(ctx, &pipes.ListTagsForResourceInput{
		ResourceArn: pipeArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"/list_initial_tags", listTagsOutput)

	// Add more tags.
	_, err = client.TagResource(ctx, &pipes.TagResourceInput{
		ResourceArn: pipeArn,
		Tags: map[string]string{
			"NewTag": "newvalue",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify tags were added.
	listTagsOutput, err = client.ListTagsForResource(ctx, &pipes.ListTagsForResourceInput{
		ResourceArn: pipeArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"/list_tags_after_add", listTagsOutput)

	// Remove a tag.
	_, err = client.UntagResource(ctx, &pipes.UntagResourceInput{
		ResourceArn: pipeArn,
		TagKeys:     []string{"NewTag"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify tag was removed.
	listTagsOutput, err = client.ListTagsForResource(ctx, &pipes.ListTagsForResourceInput{
		ResourceArn: pipeArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"/list_tags_after_remove", listTagsOutput)
}

func TestPipes_NotFoundErrors(t *testing.T) {
	client := newPipesClient(t)
	ctx := t.Context()

	// Describe non-existent pipe.
	_, err := client.DescribePipe(ctx, &pipes.DescribePipeInput{
		Name: aws.String("non-existent-pipe"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Update non-existent pipe.
	_, err = client.UpdatePipe(ctx, &pipes.UpdatePipeInput{
		Name:    aws.String("non-existent-pipe"),
		RoleArn: aws.String("arn:aws:iam::123456789012:role/test-role"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent pipe.
	_, err = client.DeletePipe(ctx, &pipes.DeletePipeInput{
		Name: aws.String("non-existent-pipe"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Start non-existent pipe.
	_, err = client.StartPipe(ctx, &pipes.StartPipeInput{
		Name: aws.String("non-existent-pipe"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Stop non-existent pipe.
	_, err = client.StopPipe(ctx, &pipes.StopPipeInput{
		Name: aws.String("non-existent-pipe"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPipes_ConflictErrors(t *testing.T) {
	client := newPipesClient(t)
	ctx := t.Context()

	pipeName := "test-pipe-conflict"
	source := "arn:aws:sqs:us-east-1:123456789012:test-source-queue"
	target := "arn:aws:lambda:us-east-1:123456789012:function:test-target"
	roleArn := "arn:aws:iam::123456789012:role/test-pipe-role"

	// Create pipe.
	_, err := client.CreatePipe(ctx, &pipes.CreatePipeInput{
		Name:    aws.String(pipeName),
		Source:  aws.String(source),
		Target:  aws.String(target),
		RoleArn: aws.String(roleArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeletePipe(context.Background(), &pipes.DeletePipeInput{
			Name: aws.String(pipeName),
		})
	})

	// Try to create duplicate pipe.
	_, err = client.CreatePipe(ctx, &pipes.CreatePipeInput{
		Name:    aws.String(pipeName),
		Source:  aws.String(source),
		Target:  aws.String(target),
		RoleArn: aws.String(roleArn),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Try to start a running pipe.
	_, err = client.StartPipe(ctx, &pipes.StartPipeInput{
		Name: aws.String(pipeName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPipes_CreatePipeWithDescription(t *testing.T) {
	client := newPipesClient(t)
	ctx := t.Context()

	pipeName := "test-pipe-description"
	source := "arn:aws:sqs:us-east-1:123456789012:test-source-queue"
	target := "arn:aws:lambda:us-east-1:123456789012:function:test-target"
	roleArn := "arn:aws:iam::123456789012:role/test-pipe-role"
	description := "Test pipe for EventBridge Pipes integration test"

	// Create pipe with description.
	_, err := client.CreatePipe(ctx, &pipes.CreatePipeInput{
		Name:        aws.String(pipeName),
		Source:      aws.String(source),
		Target:      aws.String(target),
		RoleArn:     aws.String(roleArn),
		Description: aws.String(description),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeletePipe(context.Background(), &pipes.DeletePipeInput{
			Name: aws.String(pipeName),
		})
	})

	// Verify description.
	descOutput, err := client.DescribePipe(ctx, &pipes.DescribePipeInput{
		Name: aws.String(pipeName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Arn", "CreationTime", "LastModifiedTime", "ResultMetadata")).Assert(t.Name()+"/describe", descOutput)
}
