package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// filteredLambdaNotificationXML is a notification configuration with the
// key filter shape the terraform AWS provider sends for
// aws_s3_bucket_notification's filter_prefix/filter_suffix arguments.
const filteredLambdaNotificationXML = `<NotificationConfiguration>` +
	`<CloudFunctionConfiguration>` +
	`<Id>filter-config</Id>` +
	`<CloudFunction>` + testLambdaArn + `</CloudFunction>` +
	`<Event>s3:ObjectCreated:*</Event>` +
	`<Filter><S3Key>` +
	`<FilterRule><Name>prefix</Name><Value>uploads/</Value></FilterRule>` +
	`<FilterRule><Name>suffix</Name><Value>.jpg</Value></FilterRule>` +
	`</S3Key></Filter>` +
	`</CloudFunctionConfiguration></NotificationConfiguration>`

func putNotificationConfigXML(t *testing.T, svc *Service, bucket, body string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"?notification", strings.NewReader(body))
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.PutBucketNotificationConfiguration(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PutBucketNotificationConfiguration status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
}

// TestNotificationFilter_RoundTrip covers the read-after-write path the
// terraform AWS provider depends on: filter rules sent in
// PutBucketNotificationConfiguration must come back verbatim from
// GetBucketNotificationConfiguration, otherwise every plan after apply
// re-adds filter_prefix/filter_suffix (perpetual diff).
func TestNotificationFilter_RoundTrip(t *testing.T) {
	t.Parallel()

	const bucket = "notif-filter-roundtrip"

	_, svc := newLambdaNotificationTestService(t, bucket)
	putNotificationConfigXML(t, svc, bucket, filteredLambdaNotificationXML)

	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"?notification", http.NoBody)
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.GetBucketNotificationConfiguration(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GetBucketNotificationConfiguration status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	var config NotificationConfiguration
	if err := xml.Unmarshal(w.Body.Bytes(), &config); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(config.LambdaFunctionConfigurations) != 1 {
		t.Fatalf("LambdaFunctionConfigurations length = %d, want 1", len(config.LambdaFunctionConfigurations))
	}

	cfg := config.LambdaFunctionConfigurations[0]
	if cfg.Filter == nil || cfg.Filter.S3Key == nil {
		t.Fatalf("Filter.S3Key missing from read-back: %s", w.Body.String())
	}

	rules := cfg.Filter.S3Key.FilterRules
	if len(rules) != 2 {
		t.Fatalf("FilterRules length = %d, want 2 (body=%s)", len(rules), w.Body.String())
	}

	want := map[string]string{"prefix": "uploads/", "suffix": ".jpg"}
	for _, rule := range rules {
		if want[rule.Name] != rule.Value {
			t.Errorf("FilterRule %q = %q, want %q", rule.Name, rule.Value, want[rule.Name])
		}
	}
}

// TestNotificationFilter_LambdaDelivery covers delivery-time semantics: a
// key filter must gate Lambda invocation the way AWS does — only keys
// matching every rule of a configuration's S3Key filter fire it.
func TestNotificationFilter_LambdaDelivery(t *testing.T) {
	t.Parallel()

	const bucket = "notif-filter-delivery"

	_, svc := newLambdaNotificationTestService(t, bucket)
	putNotificationConfigXML(t, svc, bucket, filteredLambdaNotificationXML)

	invoker := newFakeLambdaInvoker()
	svc.SetLambdaInvoker(invoker)

	for _, key := range []string{"other/cat.jpg", "uploads/cat.png"} {
		w := httptest.NewRecorder()
		svc.PutObject(w, putObjectRequest(bucket, key, "data"))

		if w.Code != http.StatusOK {
			t.Fatalf("PutObject(%s) status: got %d, want %d", key, w.Code, http.StatusOK)
		}

		assertNoInvocation(t, invoker)
	}

	w := httptest.NewRecorder()
	svc.PutObject(w, putObjectRequest(bucket, "uploads/cat.jpg", "data"))

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d", w.Code, http.StatusOK)
	}

	inv := waitForInvocation(t, invoker)
	if inv.functionArn != testLambdaArn {
		t.Fatalf("functionArn = %q, want %q", inv.functionArn, testLambdaArn)
	}

	assertNoInvocation(t, invoker)
}

// TestNotificationFilter_URLEncodedValues covers the AWS encoding contract
// for filter values: a space must be configured as "+" and other special
// characters percent-encoded, so matching decodes the rule value before
// comparing it with the raw object key.
func TestNotificationFilter_URLEncodedValues(t *testing.T) {
	t.Parallel()

	const bucket = "notif-filter-encoded"

	_, svc := newLambdaNotificationTestService(t, bucket)
	putNotificationConfigXML(t, svc, bucket, `<NotificationConfiguration>`+
		`<CloudFunctionConfiguration>`+
		`<Id>encoded-config</Id>`+
		`<CloudFunction>`+testLambdaArn+`</CloudFunction>`+
		`<Event>s3:ObjectCreated:*</Event>`+
		`<Filter><S3Key>`+
		`<FilterRule><Name>prefix</Name><Value>photos+2026/a%2Bb/</Value></FilterRule>`+
		`</S3Key></Filter>`+
		`</CloudFunctionConfiguration></NotificationConfiguration>`)

	invoker := newFakeLambdaInvoker()
	svc.SetLambdaInvoker(invoker)

	// "+" decodes to a space and "%2B" to a literal "+", so the raw
	// configured string must not match itself as a key.
	w := httptest.NewRecorder()
	svc.PutObject(w, putObjectRequest(bucket, "photos+2026/a%2Bb/x.jpg", "data"))

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d", w.Code, http.StatusOK)
	}

	assertNoInvocation(t, invoker)

	const matchingKey = "photos 2026/a+b/x.jpg"

	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+url.PathEscape(matchingKey), strings.NewReader("data"))
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", matchingKey)

	w = httptest.NewRecorder()
	svc.PutObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d", w.Code, http.StatusOK)
	}

	if inv := waitForInvocation(t, invoker); inv.functionArn != testLambdaArn {
		t.Fatalf("functionArn = %q, want %q", inv.functionArn, testLambdaArn)
	}
}

// fakeSQSPublisher captures SQS deliveries on a buffered channel, mirroring
// fakeLambdaInvoker.
type fakeSQSPublisher struct {
	deliveries chan string
}

func (f *fakeSQSPublisher) PublishToSQS(_ context.Context, queueArn, _ string) error {
	f.deliveries <- queueArn

	return nil
}

// TestNotificationFilter_SQSDelivery covers the same key filter gating on
// the SQS destination path.
func TestNotificationFilter_SQSDelivery(t *testing.T) {
	t.Parallel()

	const (
		bucket   = "notif-filter-sqs"
		queueArn = "arn:aws:sqs:us-east-1:000000000000:test-queue"
	)

	_, svc := newLambdaNotificationTestService(t, bucket)
	putNotificationConfigXML(t, svc, bucket, `<NotificationConfiguration>`+
		`<QueueConfiguration>`+
		`<Id>filter-config</Id>`+
		`<Queue>`+queueArn+`</Queue>`+
		`<Event>s3:ObjectCreated:*</Event>`+
		`<Filter><S3Key>`+
		`<FilterRule><Name>prefix</Name><Value>uploads/</Value></FilterRule>`+
		`</S3Key></Filter>`+
		`</QueueConfiguration></NotificationConfiguration>`)

	publisher := &fakeSQSPublisher{deliveries: make(chan string, 10)}
	svc.SetSQSPublisher(publisher)

	w := httptest.NewRecorder()
	svc.PutObject(w, putObjectRequest(bucket, "other/cat.jpg", "data"))

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d", w.Code, http.StatusOK)
	}

	select {
	case arn := <-publisher.deliveries:
		t.Fatalf("unexpected SQS delivery for non-matching key: %s", arn)
	case <-time.After(200 * time.Millisecond):
	}

	w = httptest.NewRecorder()
	svc.PutObject(w, putObjectRequest(bucket, "uploads/cat.jpg", "data"))

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d", w.Code, http.StatusOK)
	}

	select {
	case arn := <-publisher.deliveries:
		if arn != queueArn {
			t.Fatalf("queueArn = %q, want %q", arn, queueArn)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive SQS delivery within timeout")
	}
}
