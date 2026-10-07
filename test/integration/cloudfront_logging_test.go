//go:build integration

package integration

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/sivchari/golden"
)

const (
	loggingBucket        = "cf-logs.s3.amazonaws.com"
	loggingUpdatedBucket = "cf-logs-updated.s3.amazonaws.com"
)

func loggingConfig(enabled, includeCookies bool, bucket, prefix string) *cloudfronttypes.LoggingConfig {
	return &cloudfronttypes.LoggingConfig{
		Enabled:        aws.Bool(enabled),
		IncludeCookies: aws.Bool(includeCookies),
		Bucket:         aws.String(bucket),
		Prefix:         aws.String(prefix),
	}
}

func loggingDistributionConfig(ref string, logging *cloudfronttypes.LoggingConfig) *cloudfronttypes.DistributionConfig {
	return &cloudfronttypes.DistributionConfig{
		CallerReference: aws.String(ref),
		Comment:         aws.String("distribution logging"),
		Enabled:         aws.Bool(true),
		Logging:         logging,
		Origins:         &cloudfronttypes.Origins{Quantity: aws.Int32(1), Items: []cloudfronttypes.Origin{customOrigin("assets", "assets.example.com")}},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
			TargetOriginId:       aws.String("assets"),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyRedirectToHttps,
			CachePolicyId:        aws.String(cachingDisabledID),
		},
	}
}

// assertDistributionConfigGolden reads the config back and returns its ETag.
func assertDistributionConfigGolden(t *testing.T, client *cloudfront.Client, id *string, name string) *string {
	t.Helper()

	got, err := client.GetDistributionConfig(t.Context(), &cloudfront.GetDistributionConfigInput{Id: id})
	if err != nil {
		t.Fatalf("GetDistributionConfig: %v", err)
	}

	golden.New(t, cacheBehaviorGoldenIgnores).Assert(t.Name()+"_"+name, got)

	return got.ETag
}

func TestCloudFront_DistributionLoggingRoundTrip(t *testing.T) {
	client := newCloudFrontClient(t)
	ctx := t.Context()
	ref := "test-cf-logging-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	config := loggingDistributionConfig(ref, loggingConfig(true, false, loggingBucket, "initial/"))

	created, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{DistributionConfig: config})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	id := created.Distribution.Id
	etag := created.ETag

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{Id: id, IfMatch: etag})
	})

	golden.New(t, cacheBehaviorGoldenIgnores).Assert(t.Name()+"_create", created)

	got, err := client.GetDistribution(ctx, &cloudfront.GetDistributionInput{Id: id})
	if err != nil {
		t.Fatalf("GetDistribution: %v", err)
	}

	golden.New(t, cacheBehaviorGoldenIgnores).Assert(t.Name()+"_get", got)

	etag = assertDistributionConfigGolden(t, client, id, "config")

	config.Logging = loggingConfig(true, true, loggingUpdatedBucket, "updated/")

	updated, err := client.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{Id: id, IfMatch: etag, DistributionConfig: config})
	if err != nil {
		t.Fatalf("UpdateDistribution: %v", err)
	}

	etag = updated.ETag

	golden.New(t, cacheBehaviorGoldenIgnores).Assert(t.Name()+"_update", updated)

	// CloudFront drops Bucket and Prefix sent alongside Enabled=false.
	config.Logging = loggingConfig(false, false, loggingUpdatedBucket, "updated/")

	disabled, err := client.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{Id: id, IfMatch: etag, DistributionConfig: config})
	if err != nil {
		t.Fatalf("UpdateDistribution (disable): %v", err)
	}

	etag = disabled.ETag

	golden.New(t, cacheBehaviorGoldenIgnores).Assert(t.Name()+"_disabled", disabled)

	assertDistributionConfigGolden(t, client, id, "disabled_config")
}

func TestCloudFront_CreateDistributionWithTagsKeepsLogging(t *testing.T) {
	client := newCloudFrontClient(t)
	ref := "test-cf-logging-tags-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	created, err := client.CreateDistributionWithTags(t.Context(), &cloudfront.CreateDistributionWithTagsInput{
		DistributionConfigWithTags: &cloudfronttypes.DistributionConfigWithTags{
			DistributionConfig: loggingDistributionConfig(ref, loggingConfig(true, true, loggingBucket, "")),
			Tags:               &cloudfronttypes.Tags{Items: []cloudfronttypes.Tag{{Key: aws.String("Environment"), Value: aws.String("test")}}},
		},
	})
	if err != nil {
		t.Fatalf("CreateDistributionWithTags: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{Id: created.Distribution.Id, IfMatch: created.ETag})
	})

	logging := created.Distribution.DistributionConfig.Logging
	if logging == nil || !aws.ToBool(logging.Enabled) || !aws.ToBool(logging.IncludeCookies) ||
		aws.ToString(logging.Bucket) != loggingBucket || logging.Prefix == nil || *logging.Prefix != "" {
		t.Errorf("Logging = %+v", logging)
	}
}
