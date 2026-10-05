//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

func cloudFrontTagsTestConfig(callerReference string) *types.DistributionConfig {
	return &types.DistributionConfig{
		CallerReference: aws.String(callerReference),
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
		Comment: aws.String("Tagged distribution"),
		Enabled: aws.Bool(true),
	}
}

func TestCloudFront_CreateDistributionWithTags(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	result, err := client.CreateDistributionWithTags(ctx, &cloudfront.CreateDistributionWithTagsInput{
		DistributionConfigWithTags: &types.DistributionConfigWithTags{
			DistributionConfig: cloudFrontTagsTestConfig("test-create-distribution-with-tags"),
			Tags: &types.Tags{
				Items: []types.Tag{
					{Key: aws.String("Environment"), Value: aws.String("test")},
					{Key: aws.String("Team"), Value: aws.String("platform")},
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

	ignore := golden.WithIgnoreFields(
		"Id",
		"ARN",
		"DomainName",
		"LastModifiedTime",
		"ETag",
		"Location",
		"ResultMetadata",
	)

	golden.New(t, ignore).Assert(t.Name()+"_create", result)

	tags, err := client.ListTagsForResource(ctx, &cloudfront.ListTagsForResourceInput{
		Resource: result.Distribution.ARN,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_tags", tags)
}

func TestCloudFront_TagAndUntagResource(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	created, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: cloudFrontTagsTestConfig("test-tag-and-untag-resource"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      created.Distribution.Id,
			IfMatch: created.ETag,
		})
	})

	arn := created.Distribution.ARN
	ignore := golden.WithIgnoreFields("ResultMetadata")

	empty, err := client.ListTagsForResource(ctx, &cloudfront.ListTagsForResourceInput{Resource: arn})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, ignore).Assert(t.Name()+"_empty", empty)

	if _, err := client.TagResource(ctx, &cloudfront.TagResourceInput{
		Resource: arn,
		Tags: &types.Tags{
			Items: []types.Tag{
				{Key: aws.String("Environment"), Value: aws.String("test")},
				{Key: aws.String("Team"), Value: aws.String("platform")},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Re-tagging an existing key overwrites its value.
	if _, err := client.TagResource(ctx, &cloudfront.TagResourceInput{
		Resource: arn,
		Tags: &types.Tags{
			Items: []types.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	tagged, err := client.ListTagsForResource(ctx, &cloudfront.ListTagsForResourceInput{Resource: arn})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, ignore).Assert(t.Name()+"_tagged", tagged)

	if _, err := client.UntagResource(ctx, &cloudfront.UntagResourceInput{
		Resource: arn,
		TagKeys:  &types.TagKeys{Items: []string{"Team"}},
	}); err != nil {
		t.Fatal(err)
	}

	untagged, err := client.ListTagsForResource(ctx, &cloudfront.ListTagsForResourceInput{Resource: arn})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, ignore).Assert(t.Name()+"_untagged", untagged)
}

func TestCloudFront_TagResource_NoSuchResource(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	_, err := client.ListTagsForResource(ctx, &cloudfront.ListTagsForResourceInput{
		Resource: aws.String("arn:aws:cloudfront::000000000000:distribution/EDOESNOTEXIST"),
	})

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an API error, got %v", err)
	}

	golden.New(t).Assert(t.Name(), apiErr.ErrorCode())
}
