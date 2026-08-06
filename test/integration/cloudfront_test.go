//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/sivchari/golden"
)

func newCloudFrontClient(t *testing.T) *cloudfront.Client {
	t.Helper()

	return cloudfront.NewFromConfig(awsConfig(t), func(o *cloudfront.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestCloudFront_CreateDistribution(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	result, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-create-distribution"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"ARN",
		"DomainName",
		"LastModifiedTime",
		"ETag",
		"Location",
		"ResultMetadata",
	)).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDistribution(ctx, &cloudfront.DeleteDistributionInput{
		Id:      result.Distribution.Id,
		IfMatch: result.ETag,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCloudFront_GetDistribution(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	// Create distribution first.
	createResult, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-get-distribution"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      createResult.Distribution.Id,
			IfMatch: createResult.ETag,
		})
	})

	// Get distribution.
	getResult, err := client.GetDistribution(ctx, &cloudfront.GetDistributionInput{
		Id: createResult.Distribution.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"ARN",
		"DomainName",
		"LastModifiedTime",
		"ETag",
		"Location",
		"ResultMetadata",
	)).Assert(t.Name(), getResult)
}

func TestCloudFront_ListDistributions(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	// Create distribution first.
	createResult, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-list-distributions"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      createResult.Distribution.Id,
			IfMatch: createResult.ETag,
		})
	})

	// List distributions.
	listResult, err := client.ListDistributions(ctx, &cloudfront.ListDistributionsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if listResult == nil {
		t.Fatal("listResult is nil")
	}
	if listResult.DistributionList == nil {
		t.Fatal("listResult.DistributionList is nil")
	}

	// Find our distribution.
	found := false
	for _, dist := range listResult.DistributionList.Items {
		if *dist.Id == *createResult.Distribution.Id {
			found = true

			break
		}
	}
	if !found {
		t.Error("Distribution should be in list")
	}
}

func TestCloudFront_UpdateDistribution(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	// Create distribution first.
	createResult, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-update-distribution"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update distribution.
	updateResult, err := client.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{
		Id:      createResult.Distribution.Id,
		IfMatch: createResult.ETag,
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-update-distribution"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Updated comment"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"ARN",
		"DomainName",
		"LastModifiedTime",
		"ETag",
		"Location",
		"ResultMetadata",
	)).Assert(t.Name(), updateResult)

	// Clean up with new ETag.
	_, err = client.DeleteDistribution(ctx, &cloudfront.DeleteDistributionInput{
		Id:      createResult.Distribution.Id,
		IfMatch: updateResult.ETag,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCloudFront_CreateInvalidation(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	// Create distribution first.
	createResult, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-create-invalidation"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      createResult.Distribution.Id,
			IfMatch: createResult.ETag,
		})
	})

	// Create invalidation.
	invResult, err := client.CreateInvalidation(ctx, &cloudfront.CreateInvalidationInput{
		DistributionId: createResult.Distribution.Id,
		InvalidationBatch: &types.InvalidationBatch{
			CallerReference: aws.String("test-invalidation-1"),
			Paths: &types.Paths{
				Quantity: aws.Int32(1),
				Items:    []string{"/*"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"CreateTime",
		"Location",
		"ResultMetadata",
	)).Assert(t.Name(), invResult)
}

func TestCloudFront_GetInvalidation(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	// Create distribution first.
	createResult, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-get-invalidation"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      createResult.Distribution.Id,
			IfMatch: createResult.ETag,
		})
	})

	// Create invalidation.
	invResult, err := client.CreateInvalidation(ctx, &cloudfront.CreateInvalidationInput{
		DistributionId: createResult.Distribution.Id,
		InvalidationBatch: &types.InvalidationBatch{
			CallerReference: aws.String("test-get-invalidation-1"),
			Paths: &types.Paths{
				Quantity: aws.Int32(1),
				Items:    []string{"/images/*"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get invalidation.
	getResult, err := client.GetInvalidation(ctx, &cloudfront.GetInvalidationInput{
		DistributionId: createResult.Distribution.Id,
		Id:             invResult.Invalidation.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"CreateTime",
		"ResultMetadata",
	)).Assert(t.Name(), getResult)
}
