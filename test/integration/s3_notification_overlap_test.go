//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// TestS3_NotificationOverlapRejected verifies that
// PutBucketNotificationConfiguration rejects two TopicConfigurations with
// intersecting event types and overlapping key filters, matching AWS. See
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/notification-how-to-filtering.html
func TestS3_NotificationOverlapRejected(t *testing.T) {
	s3Client := newS3Client(t)
	ctx := t.Context()

	bucketName := "s3-notif-overlap-test"
	topicArn := "arn:aws:sns:us-east-1:000000000000:s3-notif-overlap-topic"

	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, _ = s3Client.DeleteBucket(cleanupCtx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	_, err = s3Client.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket: aws.String(bucketName),
		NotificationConfiguration: &types.NotificationConfiguration{
			TopicConfigurations: []types.TopicConfiguration{
				{
					Id:       aws.String("no-filter"),
					TopicArn: aws.String(topicArn),
					Events:   []types.Event{types.EventS3ObjectCreated},
				},
				{
					Id:       aws.String("prefix-filter"),
					TopicArn: aws.String(topicArn),
					Events:   []types.Event{types.EventS3ObjectCreated},
					Filter: &types.NotificationConfigurationFilter{
						Key: &types.S3KeyFilter{
							FilterRules: []types.FilterRule{
								{Name: types.FilterRuleNamePrefix, Value: aws.String("images")},
							},
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected InvalidArgument for overlapping TopicConfigurations, got nil error")
	}

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "InvalidArgument" {
		t.Fatalf("expected InvalidArgument, got: %T: %v", err, err)
	}
}
