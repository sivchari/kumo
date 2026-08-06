//go:build integration

package integration

import (
	"context"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go-v2/service/s3control/types"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
	"github.com/sivchari/golden"
)

// s3ControlEndpointResolver is a custom endpoint resolver for S3 Control that
// disables the account ID subdomain behavior.
type s3ControlEndpointResolver struct{}

func (r *s3ControlEndpointResolver) ResolveEndpoint(_ context.Context, params s3control.EndpointParameters) (smithyendpoints.Endpoint, error) {
	u, _ := url.Parse(testEndpoint())

	return smithyendpoints.Endpoint{URI: *u}, nil
}

func newS3ControlClient(t *testing.T) *s3control.Client {
	t.Helper()

	return s3control.NewFromConfig(awsConfig(t), func(o *s3control.Options) {
		o.EndpointResolverV2 = &s3ControlEndpointResolver{}
	})
}

func TestS3Control_PublicAccessBlock(t *testing.T) {
	client := newS3ControlClient(t)
	ctx := t.Context()
	accountID := "123456789012"

	// Put public access block.
	_, err := client.PutPublicAccessBlock(ctx, &s3control.PutPublicAccessBlockInput{
		AccountId: aws.String(accountID),
		PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(true),
			IgnorePublicAcls:      aws.Bool(true),
			BlockPublicPolicy:     aws.Bool(true),
			RestrictPublicBuckets: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get public access block.
	getOutput, err := client.GetPublicAccessBlock(ctx, &s3control.GetPublicAccessBlockInput{
		AccountId: aws.String(accountID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("get", getOutput)

	// Delete public access block.
	_, err = client.DeletePublicAccessBlock(ctx, &s3control.DeletePublicAccessBlockInput{
		AccountId: aws.String(accountID),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's deleted - should return error.
	_, err = client.GetPublicAccessBlock(ctx, &s3control.GetPublicAccessBlockInput{
		AccountId: aws.String(accountID),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestS3Control_AccessPoint(t *testing.T) {
	client := newS3ControlClient(t)
	ctx := t.Context()
	accountID := "123456789012"
	apName := "test-access-point"
	bucketName := "test-bucket"

	// Create access point.
	createOutput, err := client.CreateAccessPoint(ctx, &s3control.CreateAccessPointInput{
		AccountId: aws.String(accountID),
		Name:      aws.String(apName),
		Bucket:    aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "AccessPointArn", "Alias"),
	)
	g.Assert("create", createOutput)

	// Get access point.
	getOutput, err := client.GetAccessPoint(ctx, &s3control.GetAccessPointInput{
		AccountId: aws.String(accountID),
		Name:      aws.String(apName),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "AccessPointArn", "Alias", "Endpoints"),
	)
	g2.Assert("get", getOutput)

	// List access points.
	listOutput, err := client.ListAccessPoints(ctx, &s3control.ListAccessPointsInput{
		AccountId: aws.String(accountID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "AccessPointArn", "Alias"),
	)
	g3.Assert("list", listOutput)

	// Delete access point.
	_, err = client.DeleteAccessPoint(ctx, &s3control.DeleteAccessPointInput{
		AccountId: aws.String(accountID),
		Name:      aws.String(apName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's deleted.
	_, err = client.GetAccessPoint(ctx, &s3control.GetAccessPointInput{
		AccountId: aws.String(accountID),
		Name:      aws.String(apName),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
