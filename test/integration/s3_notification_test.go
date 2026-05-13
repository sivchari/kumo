//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestS3_LambdaNotificationForObjectCreatedEvents(t *testing.T) {
	s3Client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-s3-lambda-notification"
	functionName := "test-s3-lambda-notification"

	lambdaBackend, invocations := newLambdaBackend(t, 3)
	defer lambdaBackend.Close()

	createLambdaFunction(t, ctx, functionName, lambdaBackend.URL)
	t.Cleanup(func() {
		deleteLambdaFunction(t, functionName)
	})

	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		for _, key := range []string{"images/source.jpg", "images/copy.jpg", "images/multipart.jpg"} {
			_, _ = s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
		}
		_, _ = s3Client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
	})

	_, err = s3Client.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket: aws.String(bucket),
		NotificationConfiguration: &s3types.NotificationConfiguration{
			LambdaFunctionConfigurations: []s3types.LambdaFunctionConfiguration{
				{
					Id:                aws.String("lambda-images"),
					LambdaFunctionArn: aws.String("arn:aws:lambda:us-east-1:000000000000:function:" + functionName),
					Events:            []s3types.Event{s3types.EventS3ObjectCreated},
					Filter: &s3types.NotificationConfigurationFilter{
						Key: &s3types.S3KeyFilter{
							FilterRules: []s3types.FilterRule{
								{
									Name:  s3types.FilterRuleNamePrefix,
									Value: aws.String("images/"),
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

	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("images/source.jpg"),
		Body:   bytes.NewReader([]byte("source")),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertS3LambdaEvent(t, receiveLambdaBackendInvocation(t, invocations), "ObjectCreated:Put", "images/source.jpg")

	_, err = s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String("images/copy.jpg"),
		CopySource: aws.String(bucket + "/images/source.jpg"),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertS3LambdaEvent(t, receiveLambdaBackendInvocation(t, invocations), "ObjectCreated:Copy", "images/copy.jpg")

	completeMultipartForS3LambdaNotification(t, s3Client, bucket, "images/multipart.jpg")
	assertS3LambdaEvent(t, receiveLambdaBackendInvocation(t, invocations), "ObjectCreated:CompleteMultipartUpload", "images/multipart.jpg")
}

func newLambdaBackend(t *testing.T, capacity int) (*httptest.Server, <-chan []byte) {
	t.Helper()

	invocations := make(chan []byte, capacity)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read Lambda backend body: %v", err)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		invocations <- body
		w.WriteHeader(http.StatusAccepted)
	}))

	return server, invocations
}

func createLambdaFunction(t *testing.T, ctx context.Context, functionName, endpoint string) {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"FunctionName":   functionName,
		"Runtime":        "python3.12",
		"Role":           "arn:aws:iam::000000000000:role/test-role",
		"Handler":        "index.handler",
		"InvokeEndpoint": endpoint,
		"Code":           map[string]any{"ZipFile": []byte("fake-zip")},
	})
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:4566/lambda/2015-03-31/functions", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("CreateFunction status = %d, want 201", resp.StatusCode)
	}
}

func deleteLambdaFunction(t *testing.T, functionName string) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, "http://localhost:4566/lambda/2015-03-31/functions/"+functionName, http.NoBody)
	if err != nil {
		t.Errorf("delete lambda request: %v", err)

		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("delete lambda function: %v", err)

		return
	}

	_ = resp.Body.Close()
}

func completeMultipartForS3LambdaNotification(t *testing.T, client *s3.Client, bucket, key string) {
	t.Helper()

	ctx := t.Context()
	createOutput, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatal(err)
	}

	uploadPartOutput, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   createOutput.UploadId,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader([]byte("multipart")),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: createOutput.UploadId,
		MultipartUpload: &s3types.CompletedMultipartUpload{
			Parts: []s3types.CompletedPart{
				{
					ETag:       uploadPartOutput.ETag,
					PartNumber: aws.Int32(1),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func receiveLambdaBackendInvocation(t *testing.T, invocations <-chan []byte) []byte {
	t.Helper()

	select {
	case body := <-invocations:
		return body
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Lambda backend invocation")
	}

	return nil
}

func assertS3LambdaEvent(t *testing.T, body []byte, eventName, key string) {
	t.Helper()

	var envelope s3QueueNotificationEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("unmarshal Lambda event: %v body=%s", err, string(body))
	}

	if len(envelope.Records) != 1 {
		t.Fatalf("Records length = %d, want 1", len(envelope.Records))
	}

	record := envelope.Records[0]
	if record.EventName != eventName {
		t.Fatalf("eventName = %q, want %q", record.EventName, eventName)
	}

	if record.S3.Object.Key != key {
		t.Fatalf("key = %q, want %q", record.S3.Object.Key, key)
	}
}
