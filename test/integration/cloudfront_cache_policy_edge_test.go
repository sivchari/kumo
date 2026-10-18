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

// TestCloudFront_EdgeHonoursCachePolicyTTLs — with an S3 origin (which sends
// Last-Modified and no Cache-Control), the edge caches according to the
// behaviour's managed cache policy: CachingDisabled never caches, while
// CachingOptimized serves the second request from the cache.
func TestCloudFront_EdgeHonoursCachePolicyTTLs(t *testing.T) {
	bucket := "test-cf-cache-policy-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	s3Client := newS3Client(t)

	if _, err := s3Client.CreateBucket(t.Context(), &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	if _, err := s3Client.PutObject(t.Context(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("object.txt"),
		Body:   strings.NewReader("cached?"),
	}); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	t.Cleanup(func() {
		_, _ = s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String("object.txt")})
		_, _ = s3Client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	for _, tc := range []struct {
		name, policyID, second string
	}{
		{"CachingDisabled", "4135ea2d-6df8-44a3-9df3-4b5a84be39ad", "Miss from kumo"},
		{"CachingOptimized", "658327ea-f89d-4fab-a63d-7e88639e58f6", "Hit from kumo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			distID := createS3OriginDistribution(t, "test-cf-"+tc.name, bucket, tc.policyID)

			first, body := callEdgePath(t, http.MethodGet, distID, "/object.txt", http.NoBody, "")
			if first.StatusCode != http.StatusOK || string(body) != "cached?" {
				t.Fatalf("edge GET: %d %q", first.StatusCode, body)
			}

			if first.Header.Get("Last-Modified") == "" || first.Header.Get("Cache-Control") != "" {
				t.Fatalf("S3 origin should send Last-Modified and no Cache-Control: %v", first.Header)
			}

			second, _ := callEdgePath(t, http.MethodGet, distID, "/object.txt", http.NoBody, "")

			if got := first.Header.Get("X-Cache"); got != "Miss from kumo" {
				t.Errorf("first X-Cache = %q, want Miss from kumo", got)
			}

			if got := second.Header.Get("X-Cache"); got != tc.second {
				t.Errorf("second X-Cache = %q, want %q", got, tc.second)
			}
		})
	}
}

// createS3OriginDistribution creates a distribution in front of the kumo S3
// bucket whose default behaviour uses the given cache policy, deletes it on
// cleanup, and returns the distribution id.
func createS3OriginDistribution(t *testing.T, name, bucket, cachePolicyID string) string {
	t.Helper()

	client := newCloudFrontClient(t)

	out, err := client.CreateDistribution(t.Context(), &cloudfront.CreateDistributionInput{
		DistributionConfig: &cloudfronttypes.DistributionConfig{
			CallerReference: aws.String(name + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)),
			Comment:         aws.String(name),
			Enabled:         aws.Bool(true),
			Origins: &cloudfronttypes.Origins{
				Quantity: aws.Int32(1),
				Items: []cloudfronttypes.Origin{{
					Id:             aws.String("s3"),
					DomainName:     aws.String(bucket + ".s3.us-east-1.amazonaws.com"),
					S3OriginConfig: &cloudfronttypes.S3OriginConfig{OriginAccessIdentity: aws.String("")},
				}},
			},
			DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
				TargetOriginId:       aws.String("s3"),
				ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String(cachePolicyID),
			},
		},
	})
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
