//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	"github.com/sivchari/golden"
)

func newCloudControlClient(t *testing.T) *cloudcontrol.Client {
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

	return cloudcontrol.NewFromConfig(cfg, func(o *cloudcontrol.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestCloudControl_S3BucketLifecycle(t *testing.T) {
	client := newCloudControlClient(t)
	ctx := t.Context()
	typeName := "AWS::S3::Bucket"
	bucketName := "cloudcontrol-integration-bucket"
	desiredState := `{"BucketName":"cloudcontrol-integration-bucket"}`

	_, _ = client.DeleteResource(context.Background(), &cloudcontrol.DeleteResourceInput{
		TypeName:   aws.String(typeName),
		Identifier: aws.String(bucketName),
	})

	t.Cleanup(func() {
		_, _ = client.DeleteResource(context.Background(), &cloudcontrol.DeleteResourceInput{
			TypeName:   aws.String(typeName),
			Identifier: aws.String(bucketName),
		})
	})

	g := golden.New(t)

	createOutput, err := client.CreateResource(ctx, &cloudcontrol.CreateResourceInput{
		TypeName:     aws.String(typeName),
		DesiredState: aws.String(desiredState),
		ClientToken:  aws.String("cloudcontrol-s3-create"),
	})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}

	if got := aws.ToString(createOutput.ProgressEvent.Identifier); got != bucketName {
		t.Fatalf("CreateResource identifier = %q, want %q", got, bucketName)
	}

	g.Assert(t.Name()+"/create", stableCloudControlProgressEvent(createOutput.ProgressEvent))

	statusOutput, err := client.GetResourceRequestStatus(ctx, &cloudcontrol.GetResourceRequestStatusInput{
		RequestToken: createOutput.ProgressEvent.RequestToken,
	})
	if err != nil {
		t.Fatalf("GetResourceRequestStatus: %v", err)
	}

	g.Assert(t.Name()+"/status", stableCloudControlProgressEvent(statusOutput.ProgressEvent))

	getOutput, err := client.GetResource(ctx, &cloudcontrol.GetResourceInput{
		TypeName:   aws.String(typeName),
		Identifier: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("GetResource: %v", err)
	}

	g.Assert(t.Name()+"/get", stableCloudControlResourceDescription(
		aws.ToString(getOutput.TypeName),
		getOutput.ResourceDescription,
	))

	listOutput, err := client.ListResources(ctx, &cloudcontrol.ListResourcesInput{
		TypeName: aws.String(typeName),
	})
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}

	resource := findCloudControlResource(t, listOutput.ResourceDescriptions, bucketName)
	g.Assert(t.Name()+"/list", stableCloudControlResourceDescription("", &resource))

	deleteOutput, err := client.DeleteResource(ctx, &cloudcontrol.DeleteResourceInput{
		TypeName:    aws.String(typeName),
		Identifier:  aws.String(bucketName),
		ClientToken: aws.String("cloudcontrol-s3-delete"),
	})
	if err != nil {
		t.Fatalf("DeleteResource: %v", err)
	}

	g.Assert(t.Name()+"/delete", stableCloudControlProgressEvent(deleteOutput.ProgressEvent))

	_, err = client.GetResource(ctx, &cloudcontrol.GetResourceInput{
		TypeName:   aws.String(typeName),
		Identifier: aws.String(bucketName),
	})
	if err == nil {
		t.Fatalf("GetResource after delete error = nil")
	}

	var notFound *types.ResourceNotFoundException
	if !errors.As(err, &notFound) {
		t.Fatalf("GetResource after delete error = %T: %v, want ResourceNotFoundException", err, err)
	}
}

func findCloudControlResource(
	t *testing.T,
	resources []types.ResourceDescription,
	identifier string,
) types.ResourceDescription {
	t.Helper()

	for _, resource := range resources {
		if aws.ToString(resource.Identifier) == identifier {
			return resource
		}
	}

	t.Fatalf("resource %q not found in ListResources", identifier)

	return types.ResourceDescription{}
}

func stableCloudControlProgressEvent(event *types.ProgressEvent) map[string]any {
	if event == nil {
		return nil
	}

	return map[string]any{
		"ErrorCode":       string(event.ErrorCode),
		"Identifier":      aws.ToString(event.Identifier),
		"Operation":       string(event.Operation),
		"OperationStatus": string(event.OperationStatus),
		"RequestToken":    aws.ToString(event.RequestToken),
		"ResourceModel":   aws.ToString(event.ResourceModel),
		"StatusMessage":   aws.ToString(event.StatusMessage),
		"TypeName":        aws.ToString(event.TypeName),
	}
}

func stableCloudControlResourceDescription(
	typeName string,
	resource *types.ResourceDescription,
) map[string]any {
	if resource == nil {
		return nil
	}

	out := map[string]any{
		"Identifier": aws.ToString(resource.Identifier),
		"Properties": aws.ToString(resource.Properties),
	}

	if typeName != "" {
		out["TypeName"] = typeName
	}

	return out
}
