//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/sivchari/golden"
)

func TestS3_EventBridgeNotification(t *testing.T) {
	s3Client := newS3Client(t)
	ebClient := newEventBridgeClient(t)
	sqsClient := newSQSClient(t)
	ctx := t.Context()

	bucketName := "s3-eb-notif-test"
	queueName := "s3-eb-notif-queue"

	// 1. Create S3 bucket.
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Enable EventBridge notification on the bucket.
	notifXML := `<NotificationConfiguration><EventBridgeConfiguration></EventBridgeConfiguration></NotificationConfiguration>`

	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		"http://localhost:4566/"+bucketName+"?notification",
		bytes.NewReader([]byte(notifXML)))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PutBucketNotificationConfiguration returned %d", resp.StatusCode)
	}

	// 3. Create SQS queue.
	createQueueOutput, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	queueURL := *createQueueOutput.QueueUrl

	// 4. Create EventBridge rule matching S3 Object Created events for this bucket.
	_, err = ebClient.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("s3-notif-rule"),
		EventPattern: aws.String(`{"source": ["aws.s3"], "detail-type": ["Object Created"], "detail": {"bucket": {"name": ["` + bucketName + `"]}}}`),
		State:        ebtypes.RuleStateEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 5. Add SQS target.
	putTargetsOutput, err := ebClient.PutTargets(ctx, &eventbridge.PutTargetsInput{
		Rule: aws.String("s3-notif-rule"),
		Targets: []ebtypes.Target{
			{
				Id:  aws.String("s3-notif-sqs"),
				Arn: aws.String("arn:aws:sqs:us-east-1:000000000000:" + queueName),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_put_targets", putTargetsOutput)

	// 6. Upload an object to trigger the notification.
	putObjOutput, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("test-file.txt"),
		Body:   bytes.NewReader([]byte("hello world")),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ETag", "VersionId", "ResultMetadata")).Assert(t.Name()+"_put_object", putObjOutput)

	// Wait for async EventBridge notification goroutine to complete.
	time.Sleep(500 * time.Millisecond)

	// 7. Receive message from SQS to confirm the event was delivered.
	var recvOutput *sqs.ReceiveMessageOutput

	for range 10 {
		recvOutput, err = sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:        aws.String(queueURL),
			WaitTimeSeconds: 1,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(recvOutput.Messages) > 0 {
			break
		}
	}

	if len(recvOutput.Messages) == 0 {
		t.Fatal("expected S3 event to be delivered to SQS queue, but no message received")
	}

	// 8. Verify the event payload.
	var envelope map[string]any
	if err := json.Unmarshal([]byte(*recvOutput.Messages[0].Body), &envelope); err != nil {
		t.Fatalf("failed to parse SQS message body: %v", err)
	}

	if envelope["source"] != "aws.s3" {
		t.Errorf("expected source=aws.s3, got %v", envelope["source"])
	}

	if envelope["detail-type"] != "Object Created" {
		t.Errorf("expected detail-type=Object Created, got %v", envelope["detail-type"])
	}
}

// TestS3_NotificationToSQS verifies that PutObject delivers an S3
// event notification to an SQS queue configured via
// PutBucketNotificationConfiguration (QueueConfiguration). This is
// the "classic" S3 -> SQS path (not via EventBridge).
func TestS3_NotificationToSQS(t *testing.T) {
	s3Client := newS3Client(t)
	sqsClient := newSQSClient(t)
	ctx := t.Context()

	bucketName := "s3-notif-sqs-test"
	queueName := "s3-notif-sqs-queue"

	// 1. Create S3 bucket.
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, _ = s3Client.DeleteObject(cleanupCtx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String("notif-test.txt"),
		})
		_, _ = s3Client.DeleteBucket(cleanupCtx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// 2. Create SQS queue.
	createQueueOutput, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	queueURL := *createQueueOutput.QueueUrl

	t.Cleanup(func() {
		_, _ = sqsClient.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: aws.String(queueURL),
		})
	})

	// 3. Configure bucket notification with QueueConfiguration.
	queueARN := "arn:aws:sqs:us-east-1:000000000000:" + queueName
	notifXML := `<NotificationConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">` +
		`<QueueConfiguration>` +
		`<Id>sqs-notif</Id>` +
		`<Queue>` + queueARN + `</Queue>` +
		`<Event>s3:ObjectCreated:*</Event>` +
		`</QueueConfiguration>` +
		`</NotificationConfiguration>`

	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		"http://localhost:4566/"+bucketName+"?notification",
		bytes.NewReader([]byte(notifXML)))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PutBucketNotificationConfiguration returned %d", resp.StatusCode)
	}

	// 4. Upload an object to trigger the notification.
	putObjOutput, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("notif-test.txt"),
		Body:   bytes.NewReader([]byte("hello sqs notification")),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ETag", "VersionId", "ResultMetadata")).Assert(t.Name()+"_put_object", putObjOutput)

	// Wait for async notification goroutine to complete.
	time.Sleep(500 * time.Millisecond)

	// 5. Receive message from SQS to confirm the event was delivered.
	var recvOutput *sqs.ReceiveMessageOutput

	for range 10 {
		recvOutput, err = sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:        aws.String(queueURL),
			WaitTimeSeconds: 1,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(recvOutput.Messages) > 0 {
			break
		}
	}

	if len(recvOutput.Messages) == 0 {
		t.Fatal("expected S3 event notification to be delivered to SQS queue, but no message received")
	}

	// 6. Verify the event payload matches the S3 event notification format.
	msgBody := aws.ToString(recvOutput.Messages[0].Body)

	var notification struct {
		Records []struct {
			EventSource string `json:"eventSource"`
			EventName   string `json:"eventName"`
			S3          struct {
				Bucket struct {
					Name string `json:"name"`
				} `json:"bucket"`
				Object struct {
					Key string `json:"key"`
				} `json:"object"`
			} `json:"s3"`
		} `json:"Records"`
	}

	if err := json.Unmarshal([]byte(msgBody), &notification); err != nil {
		t.Fatalf("failed to parse SQS message body as S3 event notification: %v", err)
	}

	if len(notification.Records) == 0 {
		t.Fatal("expected at least one record in the notification")
	}

	record := notification.Records[0]

	if record.EventSource != "aws:s3" {
		t.Errorf("expected eventSource=aws:s3, got %q", record.EventSource)
	}

	if record.EventName != "s3:ObjectCreated:Put" {
		t.Errorf("expected eventName=s3:ObjectCreated:Put, got %q", record.EventName)
	}

	if record.S3.Bucket.Name != bucketName {
		t.Errorf("expected bucket name=%q, got %q", bucketName, record.S3.Bucket.Name)
	}

	if !strings.Contains(record.S3.Object.Key, "notif-test.txt") {
		t.Errorf("expected object key to contain notif-test.txt, got %q", record.S3.Object.Key)
	}
}
