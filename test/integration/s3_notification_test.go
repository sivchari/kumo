//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type s3QueueNotificationEnvelope struct {
	Records []s3QueueNotificationRecord `json:"Records"` //nolint:tagliatelle // S3 notification payload uses Records.
}

type s3QueueNotificationRecord struct {
	EventName string `json:"eventName"`
	S3        struct {
		Bucket struct {
			Name string `json:"name"`
		} `json:"bucket"`
		Object struct {
			Key string `json:"key"`
		} `json:"object"`
	} `json:"s3"`
}

func TestS3_QueueNotificationToSQS(t *testing.T) {
	s3Client := newS3Client(t)
	sqsClient := newSQSClient(t)
	ctx := t.Context()
	bucket := "test-s3-queue-notification"
	key := "images/cat.jpg"
	queueName := "test-s3-queue-notification"

	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatal(err)
	}

	createQueueOutput, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		_, _ = s3Client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
		_, _ = sqsClient.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createQueueOutput.QueueUrl,
		})
	})

	queueARN := "arn:aws:sqs:us-east-1:000000000000:" + queueName
	_, err = s3Client.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket: aws.String(bucket),
		NotificationConfiguration: &s3types.NotificationConfiguration{
			QueueConfigurations: []s3types.QueueConfiguration{
				{
					Id:       aws.String("images"),
					QueueArn: aws.String(queueARN),
					Events:   []s3types.Event{s3types.EventS3ObjectCreatedPut},
					Filter: &s3types.NotificationConfigurationFilter{
						Key: &s3types.S3KeyFilter{
							FilterRules: []s3types.FilterRule{
								{
									Name:  s3types.FilterRuleNamePrefix,
									Value: aws.String("images/"),
								},
								{
									Name:  s3types.FilterRuleNameSuffix,
									Value: aws.String(".jpg"),
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	getConfigOutput, err := s3Client.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(getConfigOutput.QueueConfigurations) != 1 {
		t.Fatalf("QueueConfigurations length = %d, want 1", len(getConfigOutput.QueueConfigurations))
	}

	if aws.ToString(getConfigOutput.QueueConfigurations[0].QueueArn) != queueARN {
		t.Fatalf("QueueArn = %q, want %q", aws.ToString(getConfigOutput.QueueConfigurations[0].QueueArn), queueARN)
	}

	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte("jpeg")),
	})
	if err != nil {
		t.Fatal(err)
	}

	receiveOutput, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:        createQueueOutput.QueueUrl,
		WaitTimeSeconds: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(receiveOutput.Messages) != 1 {
		t.Fatalf("received messages = %d, want 1", len(receiveOutput.Messages))
	}

	var envelope s3QueueNotificationEnvelope

	if err := json.Unmarshal([]byte(aws.ToString(receiveOutput.Messages[0].Body)), &envelope); err != nil {
		t.Fatal(err)
	}

	if len(envelope.Records) != 1 {
		t.Fatalf("Records length = %d, want 1", len(envelope.Records))
	}

	record := envelope.Records[0]
	if record.EventName != "ObjectCreated:Put" {
		t.Fatalf("eventName = %q, want ObjectCreated:Put", record.EventName)
	}

	if record.S3.Bucket.Name != bucket {
		t.Fatalf("bucket = %q, want %q", record.S3.Bucket.Name, bucket)
	}

	if record.S3.Object.Key != key {
		t.Fatalf("key = %q, want %q", record.S3.Object.Key, key)
	}
}
