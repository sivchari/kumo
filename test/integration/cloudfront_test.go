//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/sivchari/golden"
)

func newCloudFrontClient(t *testing.T) *cloudfront.Client {
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

	return cloudfront.NewFromConfig(cfg, func(o *cloudfront.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
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

func TestCloudFront_CreateDistributionWithTags(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	result, err := client.CreateDistributionWithTags(ctx, &cloudfront.CreateDistributionWithTagsInput{
		DistributionConfigWithTags: &types.DistributionConfigWithTags{
			DistributionConfig: &types.DistributionConfig{
				CallerReference: aws.String("test-create-distribution-with-tags"),
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
				Comment: aws.String("Test distribution with tags"),
				Enabled: aws.Bool(true),
			},
			Tags: &types.Tags{
				Items: []types.Tag{
					{Key: aws.String("env"), Value: aws.String("local")},
					{Key: aws.String("team"), Value: aws.String("idp")},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      result.Distribution.Id,
			IfMatch: result.ETag,
		})
	})

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"ARN",
		"DomainName",
		"LastModifiedTime",
		"ETag",
		"Location",
		"ResultMetadata",
	)).Assert(t.Name()+"_create", result)

	// Tags applied at creation must be readable back via ListTagsForResource.
	tagsResult, err := client.ListTagsForResource(ctx, &cloudfront.ListTagsForResourceInput{
		Resource: result.Distribution.ARN,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ResultMetadata",
	)).Assert(t.Name()+"_tags", tagsResult)
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

// TestCloudFront_CreateDistribution_CachePolicyNoSigning provisions a
// CachePolicy-based distribution without trusted signers/key groups but with
// CachedMethods, mirroring the migrated IdP frontend distribution. It guards
// the DefaultCacheBehavior response shape the Terraform AWS provider flattens
// (docs/idp-parity 11): the response must always carry TrustedSigners and
// TrustedKeyGroups (Enabled=false), and must preserve CachedMethods.
func TestCloudFront_CreateDistribution_CachePolicyNoSigning(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	result, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-cache-policy-no-signing"),
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
				ViewerProtocolPolicy: types.ViewerProtocolPolicyRedirectToHttps,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
				AllowedMethods: &types.AllowedMethods{
					Quantity: aws.Int32(2),
					Items:    []types.Method{types.MethodGet, types.MethodHead},
					CachedMethods: &types.CachedMethods{
						Quantity: aws.Int32(2),
						Items:    []types.Method{types.MethodGet, types.MethodHead},
					},
				},
			},
			Comment: aws.String("Test distribution cache policy no signing"),
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
