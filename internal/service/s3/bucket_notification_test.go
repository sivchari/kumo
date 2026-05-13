package s3

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type sqsNotificationRequest struct {
	QueueURL    string `json:"QueueUrl"`    //nolint:tagliatelle // AWS JSON protocol uses QueueUrl.
	MessageBody string `json:"MessageBody"` //nolint:tagliatelle // AWS JSON protocol uses MessageBody.
}

type s3NotificationEnvelope struct {
	Records []s3NotificationRecord `json:"Records"` //nolint:tagliatelle // S3 notification payload uses Records.
}

type s3NotificationRecord struct {
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

func TestBucketNotification_QueueConfigurationRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(t.Context(), "nb"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	body := `<NotificationConfiguration><QueueConfiguration><Id>images</Id><Queue>arn:aws:sqs:us-east-1:000000000000:image-events</Queue><Event>s3:ObjectCreated:*</Event><Filter><S3Key><FilterRule><Name>prefix</Name><Value>images/</Value></FilterRule></S3Key></Filter></QueueConfiguration></NotificationConfiguration>`
	putReq := httptest.NewRequest(http.MethodPut, "/nb?notification", strings.NewReader(body))
	putReq.SetPathValue("bucket", "nb")

	putW := httptest.NewRecorder()
	svc.PutBucketNotificationConfiguration(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("PUT notification status: got %d, want 200 (body=%s)", putW.Code, putW.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/nb?notification", http.NoBody)
	getReq.SetPathValue("bucket", "nb")

	getW := httptest.NewRecorder()
	svc.GetBucketNotificationConfiguration(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("GET notification status: got %d, want 200 (body=%s)", getW.Code, getW.Body.String())
	}

	var got NotificationConfiguration
	if err := xml.Unmarshal(getW.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal notification: %v body=%s", err, getW.Body.String())
	}

	if len(got.QueueConfigurations) != 1 {
		t.Fatalf("queue configs: got %d, want 1", len(got.QueueConfigurations))
	}

	if got.QueueConfigurations[0].Queue != "arn:aws:sqs:us-east-1:000000000000:image-events" {
		t.Fatalf("queue ARN: got %q", got.QueueConfigurations[0].Queue)
	}

	if got.QueueConfigurations[0].Events[0] != "s3:ObjectCreated:*" {
		t.Fatalf("event: got %q", got.QueueConfigurations[0].Events[0])
	}
}

func TestPutObject_DeliversQueueNotification(t *testing.T) {
	t.Parallel()

	sqsServer, requests := newSQSNotificationCaptureServer(t)
	defer sqsServer.Close()

	store := NewMemoryStorage()
	svc := New(store, sqsServer.URL)

	if err := store.CreateBucket(t.Context(), "notify-bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	putBucketNotificationConfig(t, svc, "notify-bucket", "s3-events")
	putObjectForQueueNotification(t, svc, "notify-bucket", "images/cat.jpg")

	req := receiveSQSNotificationRequest(t, requests)
	if req.QueueURL != sqsServer.URL+"/000000000000/s3-events" {
		t.Fatalf("QueueUrl: got %q", req.QueueURL)
	}

	assertObjectCreatedNotification(t, req.MessageBody, "notify-bucket", "images/cat.jpg")
}

func newSQSNotificationCaptureServer(t *testing.T) (*httptest.Server, <-chan sqsNotificationRequest) {
	t.Helper()

	requests := make(chan sqsNotificationRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Amz-Target"); got != "AmazonSQS.SendMessage" {
			t.Errorf("X-Amz-Target: got %q, want AmazonSQS.SendMessage", got)
		}

		var req sqsNotificationRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode SQS request: %v", err)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		requests <- req

		_, _ = w.Write([]byte(`{"MessageId":"msg-1","MD5OfMessageBody":"d41d8cd98f00b204e9800998ecf8427e"}`))
	}))

	return server, requests
}

func putBucketNotificationConfig(t *testing.T, svc *Service, bucket, queueName string) {
	t.Helper()

	body := fmt.Sprintf(
		`<NotificationConfiguration><QueueConfiguration><Id>all-created</Id><Queue>arn:aws:sqs:us-east-1:000000000000:%s</Queue><Event>s3:ObjectCreated:*</Event></QueueConfiguration></NotificationConfiguration>`,
		queueName,
	)
	putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"?notification", strings.NewReader(body))
	putReq.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.PutBucketNotificationConfiguration(w, putReq)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT notification status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

func putObjectForQueueNotification(t *testing.T, svc *Service, bucket, key string) {
	t.Helper()

	putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, strings.NewReader("jpeg"))
	putReq.SetPathValue("bucket", bucket)
	putReq.SetPathValue("key", key)

	w := httptest.NewRecorder()
	svc.PutObject(w, putReq)

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

func receiveSQSNotificationRequest(t *testing.T, requests <-chan sqsNotificationRequest) sqsNotificationRequest {
	t.Helper()

	select {
	case req := <-requests:
		return req
	case <-time.After(750 * time.Millisecond):
		t.Fatal("timed out waiting for SQS notification")
	}

	return sqsNotificationRequest{}
}

func assertObjectCreatedNotification(t *testing.T, body, bucket, key string) {
	t.Helper()

	var envelope s3NotificationEnvelope

	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("unmarshal message body: %v body=%s", err, body)
	}

	if len(envelope.Records) != 1 {
		t.Fatalf("Records: got %d, want 1", len(envelope.Records))
	}

	record := envelope.Records[0]
	if record.EventName != "ObjectCreated:Put" {
		t.Fatalf("eventName: got %q, want ObjectCreated:Put", record.EventName)
	}

	if record.S3.Bucket.Name != bucket {
		t.Fatalf("bucket name: got %q", record.S3.Bucket.Name)
	}

	if record.S3.Object.Key != key {
		t.Fatalf("object key: got %q", record.S3.Object.Key)
	}
}
