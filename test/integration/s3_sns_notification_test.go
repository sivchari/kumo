//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/sivchari/golden"
)

// TestS3_NotificationToSNS verifies that PutObject delivers an S3 event
// notification to an SNS topic configured via
// PutBucketNotificationConfiguration (TopicConfiguration), and that SNS
// fans the message out to a subscribed SQS queue with the standard SNS
// notification envelope wrapping the S3 event payload in Message.
func TestS3_NotificationToSNS(t *testing.T) {
	s3Client := newS3Client(t)
	snsClient := newSNSClient(t)
	sqsClient := newSQSClient(t)
	ctx := t.Context()

	bucketName := "s3-notif-sns-test"
	topicName := "s3-notif-sns-topic"
	queueName := "s3-notif-sns-queue"

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

	// 2. Create SNS topic.
	topicOutput, err := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatal(err)
	}

	topicArn := aws.ToString(topicOutput.TopicArn)

	t.Cleanup(func() {
		_, _ = snsClient.DeleteTopic(context.Background(), &sns.DeleteTopicInput{
			TopicArn: topicOutput.TopicArn,
		})
	})

	// 3. Create SQS queue and subscribe it to the topic.
	createQueueOutput, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	queueURL := aws.ToString(createQueueOutput.QueueUrl)

	t.Cleanup(func() {
		_, _ = sqsClient.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: aws.String(queueURL),
		})
	})

	queueARN := "arn:aws:sqs:us-east-1:000000000000:" + queueName

	subOutput, err := snsClient.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: topicOutput.TopicArn,
		Protocol: aws.String("sqs"),
		Endpoint: aws.String(queueARN),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = snsClient.Unsubscribe(context.Background(), &sns.UnsubscribeInput{
			SubscriptionArn: subOutput.SubscriptionArn,
		})
	})

	// 4. Configure bucket notification with TopicConfiguration.
	notifXML := `<NotificationConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">` +
		`<TopicConfiguration>` +
		`<Id>sns-notif</Id>` +
		`<Topic>` + topicArn + `</Topic>` +
		`<Event>s3:ObjectCreated:*</Event>` +
		`</TopicConfiguration>` +
		`</NotificationConfiguration>`

	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		testEndpoint()+"/"+bucketName+"?notification",
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

	// 5. Upload an object to trigger the notification.
	putObjOutput, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("notif-test.txt"),
		Body:   bytes.NewReader([]byte("hello sns notification")),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ETag", "VersionId", "ResultMetadata")).Assert(t.Name()+"_put_object", putObjOutput)

	// 6. Receive the SNS-delivered message from the subscribed SQS queue.
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

		time.Sleep(100 * time.Millisecond)
	}

	if len(recvOutput.Messages) == 0 {
		t.Fatal("expected S3 event notification to be delivered to SNS and fanned out to SQS, but no message received")
	}

	// 7. The SNS envelope wraps the S3 event payload in the Message field.
	var envelope struct {
		Type     string `json:"Type"`
		TopicArn string `json:"TopicArn"`
		Subject  string `json:"Subject"`
		Message  string `json:"Message"`
	}

	msgBody := aws.ToString(recvOutput.Messages[0].Body)
	if err := json.Unmarshal([]byte(msgBody), &envelope); err != nil {
		t.Fatalf("failed to parse SQS message body as an SNS notification envelope: %v", err)
	}

	if envelope.Type != "Notification" {
		t.Errorf("expected envelope Type=Notification, got %q", envelope.Type)
	}

	if envelope.TopicArn != topicArn {
		t.Errorf("expected envelope TopicArn=%q, got %q", topicArn, envelope.TopicArn)
	}

	if envelope.Subject != "Amazon S3 Notification" {
		t.Errorf("expected envelope Subject=%q, got %q", "Amazon S3 Notification", envelope.Subject)
	}

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

	if err := json.Unmarshal([]byte(envelope.Message), &notification); err != nil {
		t.Fatalf("failed to parse envelope Message as an S3 event notification: %v", err)
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

	if record.S3.Object.Key != "notif-test.txt" {
		t.Errorf("expected object key=notif-test.txt, got %q", record.S3.Object.Key)
	}
}
