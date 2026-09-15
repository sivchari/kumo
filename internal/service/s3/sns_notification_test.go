package s3

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeSNSPublisher is a test double for SNSPublisher that captures every
// publish on a buffered channel so tests can wait for the async
// emitSNSNotifications goroutine without a blind time.Sleep, mirroring the
// channel-based wait pattern used by the Lambda delivery tests.
type fakeSNSPublisher struct {
	publications chan snsPublication
}

type snsPublication struct {
	topicARN string
	message  string
	subject  string
}

func newFakeSNSPublisher() *fakeSNSPublisher {
	return &fakeSNSPublisher{publications: make(chan snsPublication, 10)}
}

func (f *fakeSNSPublisher) Publish(_ context.Context, topicARN, message, subject string) error {
	f.publications <- snsPublication{topicARN: topicARN, message: message, subject: subject}

	return nil
}

// waitForPublication waits up to 2 seconds for a publication to arrive.
func waitForPublication(t *testing.T, f *fakeSNSPublisher) snsPublication {
	t.Helper()

	select {
	case pub := <-f.publications:
		return pub
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive SNS publication within timeout")

		return snsPublication{}
	}
}

// assertNoPublication fails the test if a publication arrives within a
// short grace period, used to confirm a filter mismatch or missing
// publisher correctly suppresses delivery.
func assertNoPublication(t *testing.T, f *fakeSNSPublisher) {
	t.Helper()

	select {
	case pub := <-f.publications:
		t.Fatalf("unexpected SNS publication: topicARN=%s message=%s", pub.topicARN, pub.message)
	case <-time.After(200 * time.Millisecond):
	}
}

const testTopicArn = "arn:aws:sns:us-east-1:000000000000:test-topic"

func newSNSNotificationTestService(t *testing.T, bucket string) (*MemoryStorage, *Service) {
	t.Helper()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(context.Background(), bucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	return store, svc
}

func putSNSNotificationConfig(t *testing.T, svc *Service, bucket string, events []string, filter string) {
	t.Helper()

	body := `<NotificationConfiguration>` +
		`<TopicConfiguration>` +
		`<Id>test-config</Id>` +
		`<Topic>` + testTopicArn + `</Topic>`
	for _, e := range events {
		body += `<Event>` + e + `</Event>`
	}

	body += filter + `</TopicConfiguration></NotificationConfiguration>`

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/"+bucket+"?notification", strings.NewReader(body))
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.PutBucketNotificationConfiguration(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PutBucketNotificationConfiguration status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
}

// TestSNSNotification_PutObjectPublishesToSNS covers a TopicConfiguration
// matching s3:ObjectCreated:* firing exactly once on PutObject, with the
// right topic ARN/bucket/key/eventName/configuration id.
func TestSNSNotification_PutObjectPublishesToSNS(t *testing.T) {
	t.Parallel()

	const bucket = "sns-notify-put"

	_, svc := newSNSNotificationTestService(t, bucket)
	putSNSNotificationConfig(t, svc, bucket, []string{eventObjectCreatedAll}, "")

	publisher := newFakeSNSPublisher()
	svc.SetSNSPublisher(publisher)

	w := httptest.NewRecorder()
	svc.PutObject(w, putObjectRequest(t, bucket, "hello.txt", "hello world"))

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	pub := waitForPublication(t, publisher)

	if pub.topicARN != testTopicArn {
		t.Fatalf("topicARN = %q, want %q", pub.topicARN, testTopicArn)
	}

	if pub.subject != snsNotificationSubject {
		t.Fatalf("subject = %q, want %q", pub.subject, snsNotificationSubject)
	}

	var notification EventNotification
	if err := json.Unmarshal([]byte(pub.message), &notification); err != nil {
		t.Fatalf("message not valid EventNotification JSON: %v", err)
	}

	if len(notification.Records) != 1 {
		t.Fatalf("Records length = %d, want 1", len(notification.Records))
	}

	rec := notification.Records[0]
	if rec.EventName != testObjectCreatedPutEvent {
		t.Errorf("EventName = %q, want %s", rec.EventName, testObjectCreatedPutEvent)
	}

	if rec.S3.Bucket.Name != bucket {
		t.Errorf("Bucket.Name = %q, want %q", rec.S3.Bucket.Name, bucket)
	}

	if rec.S3.Object.Key != "hello.txt" {
		t.Errorf("Object.Key = %q, want hello.txt", rec.S3.Object.Key)
	}

	if rec.S3.ConfigurationID != testNotificationConfigID {
		t.Errorf("ConfigurationID = %q, want %s", rec.S3.ConfigurationID, testNotificationConfigID)
	}

	assertNoPublication(t, publisher)
}

// TestSNSNotification_EventFilterMismatch covers a config scoped to
// s3:ObjectCreated:Put that must not fire for a CopyObject.
func TestSNSNotification_EventFilterMismatch(t *testing.T) {
	t.Parallel()

	const bucket = "sns-notify-eventfilter"

	store, svc := newSNSNotificationTestService(t, bucket)
	putSNSNotificationConfig(t, svc, bucket, []string{testObjectCreatedPutEvent}, "")

	publisher := newFakeSNSPublisher()
	svc.SetSNSPublisher(publisher)

	if _, err := store.PutObject(context.Background(), bucket, "src.txt", strings.NewReader("copy me"), nil); err != nil {
		t.Fatalf("PutObject (seed src): %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/"+bucket+"/dst.txt", http.NoBody)
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", "dst.txt")
	req.Header.Set("X-Amz-Copy-Source", "/"+bucket+"/src.txt")

	svc.CopyObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CopyObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	assertNoPublication(t, publisher)
}

// TestSNSNotification_KeyFilterMismatch covers a config scoped by a
// prefix/suffix key filter that must not fire for a non-matching key.
func TestSNSNotification_KeyFilterMismatch(t *testing.T) {
	t.Parallel()

	const bucket = "sns-notify-keyfilter"

	filter := `<Filter><S3Key>` +
		`<FilterRule><Name>prefix</Name><Value>uploads/</Value></FilterRule>` +
		`<FilterRule><Name>suffix</Name><Value>.jpg</Value></FilterRule>` +
		`</S3Key></Filter>`

	_, svc := newSNSNotificationTestService(t, bucket)
	putSNSNotificationConfig(t, svc, bucket, []string{eventObjectCreatedAll}, filter)

	publisher := newFakeSNSPublisher()
	svc.SetSNSPublisher(publisher)

	w := httptest.NewRecorder()
	svc.PutObject(w, putObjectRequest(t, bucket, "other/hello.txt", "hello world"))

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	assertNoPublication(t, publisher)
}

// TestSNSNotification_KeyFilterMatch covers a config scoped by a
// prefix/suffix key filter that must fire for a matching key.
func TestSNSNotification_KeyFilterMatch(t *testing.T) {
	t.Parallel()

	const bucket = "sns-notify-keyfilter-match"

	filter := `<Filter><S3Key>` +
		`<FilterRule><Name>prefix</Name><Value>uploads/</Value></FilterRule>` +
		`<FilterRule><Name>suffix</Name><Value>.jpg</Value></FilterRule>` +
		`</S3Key></Filter>`

	_, svc := newSNSNotificationTestService(t, bucket)
	putSNSNotificationConfig(t, svc, bucket, []string{eventObjectCreatedAll}, filter)

	publisher := newFakeSNSPublisher()
	svc.SetSNSPublisher(publisher)

	w := httptest.NewRecorder()
	svc.PutObject(w, putObjectRequest(t, bucket, "uploads/photo.jpg", "binary data"))

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	waitForPublication(t, publisher)
}

// TestSNSNotification_NoPublisherInstalled covers no publisher wired up:
// PutObject must still succeed and must not panic even though a
// TopicConfiguration is present.
func TestSNSNotification_NoPublisherInstalled(t *testing.T) {
	t.Parallel()

	const bucket = "sns-notify-nopublisher"

	_, svc := newSNSNotificationTestService(t, bucket)
	putSNSNotificationConfig(t, svc, bucket, []string{eventObjectCreatedAll}, "")

	w := httptest.NewRecorder()
	svc.PutObject(w, putObjectRequest(t, bucket, "hello.txt", "hello world"))

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	// Give the goroutine a moment to run; it must nil-guard and return
	// without panicking. There is nothing to observe beyond "no panic" and
	// "handler already returned 200 above" since Go test would already
	// have failed loudly if PutObject itself panicked.
	time.Sleep(50 * time.Millisecond)
}

// TestSNSNotification_XMLRoundTrip covers a PUT notification body shaped
// exactly as the AWS SDK serializes it (<TopicConfiguration> wrapping
// <Topic>/<Event>/<Id>/<Filter>) round-tripping through storage and GET.
func TestSNSNotification_XMLRoundTrip(t *testing.T) {
	t.Parallel()

	const bucket = "sns-notify-roundtrip"

	filter := `<Filter><S3Key>` +
		`<FilterRule><Name>prefix</Name><Value>uploads/</Value></FilterRule>` +
		`</S3Key></Filter>`

	store, svc := newSNSNotificationTestService(t, bucket)
	putSNSNotificationConfig(t, svc, bucket, []string{testObjectCreatedPutEvent, "s3:ObjectCreated:Copy"}, filter)

	configs := store.GetTopicConfigurations(context.Background(), bucket)
	if len(configs) != 1 {
		t.Fatalf("GetTopicConfigurations length = %d, want 1", len(configs))
	}

	cfg := configs[0]
	if cfg.ID != testNotificationConfigID {
		t.Errorf("ID = %q, want %s", cfg.ID, testNotificationConfigID)
	}

	if cfg.TopicArn != testTopicArn {
		t.Errorf("TopicArn = %q, want %q", cfg.TopicArn, testTopicArn)
	}

	if len(cfg.Events) != 2 || cfg.Events[0] != testObjectCreatedPutEvent || cfg.Events[1] != "s3:ObjectCreated:Copy" {
		t.Errorf("Events = %v, want [%s s3:ObjectCreated:Copy]", cfg.Events, testObjectCreatedPutEvent)
	}

	if cfg.Filter == nil || cfg.Filter.S3Key == nil || len(cfg.Filter.S3Key.FilterRules) != 1 {
		t.Fatalf("Filter = %+v, want a single prefix rule", cfg.Filter)
	}

	if cfg.Filter.S3Key.FilterRules[0].Value != "uploads/" {
		t.Errorf("Filter prefix = %q, want uploads/", cfg.Filter.S3Key.FilterRules[0].Value)
	}

	// GET must echo the same TopicConfiguration back.
	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/"+bucket+"?notification", http.NoBody)
	getReq.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.GetBucketNotificationConfiguration(w, getReq)

	if w.Code != http.StatusOK {
		t.Fatalf("GetBucketNotificationConfiguration status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	var got NotificationConfiguration
	if err := xml.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse GET response XML: %v", err)
	}

	if len(got.TopicConfigurations) != 1 || got.TopicConfigurations[0].TopicArn != testTopicArn {
		t.Fatalf("GET TopicConfigurations = %+v, want a single entry with TopicArn %q", got.TopicConfigurations, testTopicArn)
	}
}

// TestSNSNotification_CompleteMultipartUploadFiresAllEmitters covers
// CompleteMultipartUpload's success path firing the SNS emitter alongside
// SQS/Lambda/EventBridge. This asserts the SNS fake receives the
// CompleteMultipartUpload event name.
func TestSNSNotification_CompleteMultipartUploadFiresAllEmitters(t *testing.T) {
	t.Parallel()

	const bucket = "sns-notify-mpu"

	store, svc := newSNSNotificationTestService(t, bucket)
	putSNSNotificationConfig(t, svc, bucket, []string{eventObjectCreatedAll}, "")

	publisher := newFakeSNSPublisher()
	svc.SetSNSPublisher(publisher)

	ctx := context.Background()

	upload, err := store.CreateMultipartUpload(ctx, bucket, "big.bin", nil)
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	part, err := store.UploadPart(ctx, bucket, "big.bin", upload.UploadID, 1, strings.NewReader("all the bytes"))
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}

	completeBody := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>` + part.ETag + `</ETag></Part></CompleteMultipartUpload>`

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/"+bucket+"/big.bin?uploadId="+upload.UploadID, strings.NewReader(completeBody))
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", "big.bin")

	w := httptest.NewRecorder()
	svc.CompleteMultipartUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	pub := waitForPublication(t, publisher)

	var notification EventNotification
	if err := json.Unmarshal([]byte(pub.message), &notification); err != nil {
		t.Fatalf("message not valid EventNotification JSON: %v", err)
	}

	if len(notification.Records) != 1 {
		t.Fatalf("Records length = %d, want 1", len(notification.Records))
	}

	if got := notification.Records[0].EventName; got != "s3:ObjectCreated:CompleteMultipartUpload" {
		t.Errorf("EventName = %q, want s3:ObjectCreated:CompleteMultipartUpload", got)
	}
}
