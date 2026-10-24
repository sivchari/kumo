//go:build integration

package integration

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// The X-Cache values of the kumo edge, and the S3 origin the tests below put
// in front of a kumo bucket.
const (
	headerXCache           = "X-Cache"
	edgeCacheHit           = "Hit from kumo"
	edgeCacheMiss          = "Miss from kumo"
	s3OriginID             = "s3"
	s3RegionalDomainSuffix = ".s3.us-east-1.amazonaws.com"
	edgeObjectBody         = "cached?"
)

// createEdgeTestBucket creates a kumo S3 bucket holding the given keys, each
// with edgeObjectBody, deletes it on cleanup, and returns its name.
func createEdgeTestBucket(t *testing.T, prefix string, keys ...string) string {
	t.Helper()

	bucket := prefix + strconv.FormatInt(time.Now().UnixNano(), 36)
	s3Client := newS3Client(t)

	if _, err := s3Client.CreateBucket(t.Context(), &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	t.Cleanup(func() {
		for _, key := range keys {
			_, _ = s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		}

		_, _ = s3Client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	for _, key := range keys {
		if _, err := s3Client.PutObject(t.Context(), &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   strings.NewReader(edgeObjectBody),
		}); err != nil {
			t.Fatalf("PutObject %s: %v", key, err)
		}
	}

	return bucket
}

// TestCloudFront_EdgeHonoursCachePolicyTTLs — with an S3 origin (which sends
// Last-Modified and no Cache-Control), the edge caches according to the
// behaviour's managed cache policy: CachingDisabled never caches, while
// CachingOptimized serves the second request from the cache.
func TestCloudFront_EdgeHonoursCachePolicyTTLs(t *testing.T) {
	bucket := createEdgeTestBucket(t, "test-cf-cache-policy-", "object.txt")

	for _, tc := range []struct {
		name, policyID, second string
	}{
		{"CachingDisabled", cachingDisabledID, edgeCacheMiss},
		{"CachingOptimized", managedCachingOptimizedID, edgeCacheHit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			distID := createS3OriginDistribution(t, "test-cf-"+tc.name, bucket, tc.policyID)

			first, body := callEdgePath(t, http.MethodGet, distID, "/object.txt", http.NoBody, "")
			if first.StatusCode != http.StatusOK || string(body) != edgeObjectBody {
				t.Fatalf("edge GET: %d %q", first.StatusCode, body)
			}

			if first.Header.Get("Last-Modified") == "" || first.Header.Get("Cache-Control") != "" {
				t.Fatalf("S3 origin should send Last-Modified and no Cache-Control: %v", first.Header)
			}

			second, _ := callEdgePath(t, http.MethodGet, distID, "/object.txt", http.NoBody, "")

			if got := first.Header.Get(headerXCache); got != edgeCacheMiss {
				t.Errorf("first X-Cache = %q, want %q", got, edgeCacheMiss)
			}

			if got := second.Header.Get(headerXCache); got != tc.second {
				t.Errorf("second X-Cache = %q, want %q", got, tc.second)
			}
		})
	}
}

// TestCloudFront_EdgeHonoursOrderedBehaviorCachePolicy — an ordered behaviour
// matched by its path pattern caches with its own cache policy. The default
// behaviour uses CachingOptimized and /api/* uses CachingDisabled, so an
// object under /api/ is fetched from the S3 origin every time while one
// outside it is served from the cache. Neither the default behaviour's
// policy nor the Last-Modified heuristic of the legacy TTL settings would
// miss the second /api/ request.
func TestCloudFront_EdgeHonoursOrderedBehaviorCachePolicy(t *testing.T) {
	bucket := createEdgeTestBucket(t, "test-cf-ordered-policy-", "object.txt", "api/object.txt")
	distID := createS3OriginDistribution(t, "test-cf-ordered-policy", bucket, managedCachingOptimizedID, cloudfronttypes.CacheBehavior{
		PathPattern:          aws.String(apiPathPattern),
		TargetOriginId:       aws.String(s3OriginID),
		ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
		CachePolicyId:        aws.String(cachingDisabledID),
	})

	for _, tc := range []struct{ path, second string }{
		{"/api/object.txt", edgeCacheMiss},
		{"/object.txt", edgeCacheHit},
	} {
		first, body := callEdgePath(t, http.MethodGet, distID, tc.path, http.NoBody, "")
		if first.StatusCode != http.StatusOK || string(body) != edgeObjectBody {
			t.Fatalf("edge GET %s: %d %q", tc.path, first.StatusCode, body)
		}

		second, _ := callEdgePath(t, http.MethodGet, distID, tc.path, http.NoBody, "")

		if got := first.Header.Get(headerXCache); got != edgeCacheMiss {
			t.Errorf("%s: first X-Cache = %q, want %q", tc.path, got, edgeCacheMiss)
		}

		if got := second.Header.Get(headerXCache); got != tc.second {
			t.Errorf("%s: second X-Cache = %q, want %q", tc.path, got, tc.second)
		}
	}
}

// createS3OriginDistribution creates a distribution in front of the kumo S3
// bucket whose default behaviour uses the given cache policy, followed by the
// given ordered behaviours, deletes it on cleanup, and returns the
// distribution id.
func createS3OriginDistribution(t *testing.T, name, bucket, cachePolicyID string, ordered ...cloudfronttypes.CacheBehavior) string {
	t.Helper()

	client := newCloudFrontClient(t)
	config := &cloudfronttypes.DistributionConfig{
		CallerReference: aws.String(name + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)),
		Comment:         aws.String(name),
		Enabled:         aws.Bool(true),
		Origins: &cloudfronttypes.Origins{
			Quantity: aws.Int32(1),
			Items: []cloudfronttypes.Origin{{
				Id:             aws.String(s3OriginID),
				DomainName:     aws.String(bucket + s3RegionalDomainSuffix),
				S3OriginConfig: &cloudfronttypes.S3OriginConfig{OriginAccessIdentity: aws.String("")},
			}},
		},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
			TargetOriginId:       aws.String(s3OriginID),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
			CachePolicyId:        aws.String(cachePolicyID),
		},
	}

	if len(ordered) > 0 {
		config.CacheBehaviors = &cloudfronttypes.CacheBehaviors{Quantity: aws.Int32(int32(len(ordered))), Items: ordered}
	}

	out, err := client.CreateDistribution(t.Context(), &cloudfront.CreateDistributionInput{DistributionConfig: config})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      out.Distribution.Id,
			IfMatch: out.ETag,
		})
	})

	return aws.ToString(out.Distribution.Id)
}
