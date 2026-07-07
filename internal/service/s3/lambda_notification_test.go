package s3

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeLambdaInvoker is a test double for LambdaInvoker that captures every
// invocation on a buffered channel so tests can wait for the async
// emitLambdaNotifications goroutine without a blind time.Sleep, mirroring
// the channel-based wait pattern used by the eventbridge delivery tests.
type fakeLambdaInvoker struct {
	invocations chan lambdaInvocation
}

type lambdaInvocation struct {
	functionArn string
	payload     []byte
}

func newFakeLambdaInvoker() *fakeLambdaInvoker {
	return &fakeLambdaInvoker{invocations: make(chan lambdaInvocation, 10)}
}

func (f *fakeLambdaInvoker) InvokeAsync(_ context.Context, functionArn string, payload []byte) error {
	f.invocations <- lambdaInvocation{functionArn: functionArn, payload: payload}

	return nil
}

// waitForInvocation waits up to 2 seconds for an invocation to arrive.
func waitForInvocation(t *testing.T, f *fakeLambdaInvoker) lambdaInvocation {
	t.Helper()

	select {
	case inv := <-f.invocations:
		return inv
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive Lambda invocation within timeout")

		return lambdaInvocation{}
	}
}

// assertNoInvocation fails the test if an invocation arrives within a short
// grace period, used to confirm a filter mismatch or missing invoker
// correctly suppresses delivery.
func assertNoInvocation(t *testing.T, f *fakeLambdaInvoker) {
	t.Helper()

	select {
	case inv := <-f.invocations:
		t.Fatalf("unexpected Lambda invocation: functionArn=%s payload=%s", inv.functionArn, inv.payload)
	case <-time.After(200 * time.Millisecond):
	}
}

const testLambdaArn = "arn:aws:lambda:us-east-1:000000000000:function:test-fn"

func newLambdaNotificationTestService(t *testing.T, bucket string) (*MemoryStorage, *Service) {
	t.Helper()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(context.Background(), bucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	return store, svc
}

func putLambdaNotificationConfig(t *testing.T, svc *Service, bucket string, events []string) {
	t.Helper()

	body := `<NotificationConfiguration>` +
		`<CloudFunctionConfiguration>` +
		`<Id>test-config</Id>` +
		`<CloudFunction>` + testLambdaArn + `</CloudFunction>`
	for _, e := range events {
		body += `<Event>` + e + `</Event>`
	}

	body += `</CloudFunctionConfiguration></NotificationConfiguration>`

	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"?notification", strings.NewReader(body))
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.PutBucketNotificationConfiguration(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PutBucketNotificationConfiguration status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
}

func putObjectRequest(bucket, key, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, strings.NewReader(body))
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", key)

	return req
}

// TestLambdaNotification_PutObjectInvokesLambda covers Step 6 test 1: a
// LambdaFunctionConfiguration matching s3:ObjectCreated:* fires exactly once
// on PutObject, with the right bucket/key/eventName/configuration id.
func TestLambdaNotification_PutObjectInvokesLambda(t *testing.T) {
	t.Parallel()

	const bucket = "lambda-notify-put"

	_, svc := newLambdaNotificationTestService(t, bucket)
	putLambdaNotificationConfig(t, svc, bucket, []string{"s3:ObjectCreated:*"})

	invoker := newFakeLambdaInvoker()
	svc.SetLambdaInvoker(invoker)

	w := httptest.NewRecorder()
	svc.PutObject(w, putObjectRequest(bucket, "hello.txt", "hello world"))

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	inv := waitForInvocation(t, invoker)

	if inv.functionArn != testLambdaArn {
		t.Fatalf("functionArn = %q, want %q", inv.functionArn, testLambdaArn)
	}

	var notification EventNotification
	if err := json.Unmarshal(inv.payload, &notification); err != nil {
		t.Fatalf("payload not valid EventNotification JSON: %v", err)
	}

	if len(notification.Records) != 1 {
		t.Fatalf("Records length = %d, want 1", len(notification.Records))
	}

	rec := notification.Records[0]
	if rec.EventName != "s3:ObjectCreated:Put" {
		t.Errorf("EventName = %q, want s3:ObjectCreated:Put", rec.EventName)
	}

	if rec.S3.Bucket.Name != bucket {
		t.Errorf("Bucket.Name = %q, want %q", rec.S3.Bucket.Name, bucket)
	}

	if rec.S3.Object.Key != "hello.txt" {
		t.Errorf("Object.Key = %q, want hello.txt", rec.S3.Object.Key)
	}

	if rec.S3.ConfigurationID != "test-config" {
		t.Errorf("ConfigurationID = %q, want test-config", rec.S3.ConfigurationID)
	}

	assertNoInvocation(t, invoker)
}

// TestLambdaNotification_EventFilterMismatch covers Step 6 test 2: a config
// scoped to s3:ObjectCreated:Put must not fire for a CopyObject.
func TestLambdaNotification_EventFilterMismatch(t *testing.T) {
	t.Parallel()

	const bucket = "lambda-notify-filter"

	store, svc := newLambdaNotificationTestService(t, bucket)
	putLambdaNotificationConfig(t, svc, bucket, []string{"s3:ObjectCreated:Put"})

	invoker := newFakeLambdaInvoker()
	svc.SetLambdaInvoker(invoker)

	if _, err := store.PutObject(context.Background(), bucket, "src.txt", strings.NewReader("copy me"), nil); err != nil {
		t.Fatalf("PutObject (seed src): %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/dst.txt", http.NoBody)
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", "dst.txt")
	req.Header.Set("X-Amz-Copy-Source", "/"+bucket+"/src.txt")

	svc.CopyObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CopyObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	assertNoInvocation(t, invoker)
}

// TestLambdaNotification_NoInvokerInstalled covers Step 6 test 3: with no
// invoker wired up, PutObject must still succeed and must not panic even
// though a LambdaFunctionConfiguration is present.
func TestLambdaNotification_NoInvokerInstalled(t *testing.T) {
	t.Parallel()

	const bucket = "lambda-notify-noinvoker"

	_, svc := newLambdaNotificationTestService(t, bucket)
	putLambdaNotificationConfig(t, svc, bucket, []string{"s3:ObjectCreated:*"})

	w := httptest.NewRecorder()
	svc.PutObject(w, putObjectRequest(bucket, "hello.txt", "hello world"))

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	// Give the goroutine a moment to run; it must nil-guard and return
	// without panicking. There is nothing to observe beyond "no panic" and
	// "handler already returned 200 above" since Go test would already
	// have failed loudly if PutObject itself panicked.
	time.Sleep(50 * time.Millisecond)
}

// TestLambdaNotification_XMLRoundTrip covers Step 6 test 4: a PUT
// notification body shaped exactly as the AWS SDK serializes it
// (<CloudFunctionConfiguration> wrapping <CloudFunction>/<Event>/<Id>)
// round-trips through storage.
func TestLambdaNotification_XMLRoundTrip(t *testing.T) {
	t.Parallel()

	const bucket = "lambda-notify-roundtrip"

	store, svc := newLambdaNotificationTestService(t, bucket)
	putLambdaNotificationConfig(t, svc, bucket, []string{"s3:ObjectCreated:Put", "s3:ObjectCreated:Copy"})

	configs := store.GetLambdaConfigurations(context.Background(), bucket)
	if len(configs) != 1 {
		t.Fatalf("GetLambdaConfigurations length = %d, want 1", len(configs))
	}

	cfg := configs[0]
	if cfg.ID != "test-config" {
		t.Errorf("ID = %q, want test-config", cfg.ID)
	}

	if cfg.LambdaFunctionArn != testLambdaArn {
		t.Errorf("LambdaFunctionArn = %q, want %q", cfg.LambdaFunctionArn, testLambdaArn)
	}

	if len(cfg.Events) != 2 || cfg.Events[0] != "s3:ObjectCreated:Put" || cfg.Events[1] != "s3:ObjectCreated:Copy" {
		t.Errorf("Events = %v, want [s3:ObjectCreated:Put s3:ObjectCreated:Copy]", cfg.Events)
	}
}

// TestLambdaNotification_CompleteMultipartUploadFiresAllEmitters covers Step
// 6 test 5: CompleteMultipartUpload's success path must fire all three
// notification emitters (EventBridge, SQS, Lambda). This asserts at least
// that the Lambda fake receives the CompleteMultipartUpload event name.
func TestLambdaNotification_CompleteMultipartUploadFiresAllEmitters(t *testing.T) {
	t.Parallel()

	const bucket = "lambda-notify-mpu"

	store, svc := newLambdaNotificationTestService(t, bucket)
	putLambdaNotificationConfig(t, svc, bucket, []string{"s3:ObjectCreated:*"})

	invoker := newFakeLambdaInvoker()
	svc.SetLambdaInvoker(invoker)

	ctx := context.Background()

	upload, err := store.CreateMultipartUpload(ctx, bucket, "big.bin")
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	part, err := store.UploadPart(ctx, bucket, "big.bin", upload.UploadID, 1, strings.NewReader("all the bytes"))
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}

	completeBody := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>` + part.ETag + `</ETag></Part></CompleteMultipartUpload>`

	req := httptest.NewRequest(http.MethodPost, "/"+bucket+"/big.bin?uploadId="+upload.UploadID, strings.NewReader(completeBody))
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", "big.bin")

	w := httptest.NewRecorder()
	svc.CompleteMultipartUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	inv := waitForInvocation(t, invoker)

	var notification EventNotification
	if err := json.Unmarshal(inv.payload, &notification); err != nil {
		t.Fatalf("payload not valid EventNotification JSON: %v", err)
	}

	if len(notification.Records) != 1 {
		t.Fatalf("Records length = %d, want 1", len(notification.Records))
	}

	if got := notification.Records[0].EventName; got != "s3:ObjectCreated:CompleteMultipartUpload" {
		t.Errorf("EventName = %q, want s3:ObjectCreated:CompleteMultipartUpload", got)
	}
}
