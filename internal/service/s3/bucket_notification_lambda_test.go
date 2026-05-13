package s3

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type lambdaInvocation struct {
	Path           string
	InvocationType string
	Body           []byte
}

func TestBucketNotification_LambdaConfigurationRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(t.Context(), "lambda-bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	body := `<NotificationConfiguration><CloudFunctionConfiguration><Id>lambda-images</Id><CloudFunction>arn:aws:lambda:us-east-1:000000000000:function:image-handler</CloudFunction><Event>s3:ObjectCreated:*</Event><Filter><S3Key><FilterRule><Name>suffix</Name><Value>.jpg</Value></FilterRule></S3Key></Filter></CloudFunctionConfiguration></NotificationConfiguration>`
	putReq := httptest.NewRequest(http.MethodPut, "/lambda-bucket?notification", strings.NewReader(body))
	putReq.SetPathValue("bucket", "lambda-bucket")

	putW := httptest.NewRecorder()
	svc.PutBucketNotificationConfiguration(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("PUT notification status: got %d, want 200 (body=%s)", putW.Code, putW.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/lambda-bucket?notification", http.NoBody)
	getReq.SetPathValue("bucket", "lambda-bucket")

	getW := httptest.NewRecorder()
	svc.GetBucketNotificationConfiguration(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("GET notification status: got %d, want 200 (body=%s)", getW.Code, getW.Body.String())
	}

	var got NotificationConfiguration
	if err := xml.Unmarshal(getW.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal notification: %v body=%s", err, getW.Body.String())
	}

	if len(got.LambdaFunctionConfigurations) != 1 {
		t.Fatalf("lambda configs: got %d, want 1", len(got.LambdaFunctionConfigurations))
	}

	if got.LambdaFunctionConfigurations[0].CloudFunction != "arn:aws:lambda:us-east-1:000000000000:function:image-handler" {
		t.Fatalf("lambda ARN: got %q", got.LambdaFunctionConfigurations[0].CloudFunction)
	}
}

func TestObjectCreatedEvents_DeliverLambdaNotifications(t *testing.T) {
	t.Parallel()

	lambdaServer, invocations := newLambdaInvocationCaptureServer(t)
	defer lambdaServer.Close()

	store := NewMemoryStorage()
	svc := New(store, lambdaServer.URL)

	if err := store.CreateBucket(t.Context(), "lambda-notify-bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	putLambdaNotificationConfig(t, svc, "lambda-notify-bucket", "image-handler")
	putObjectForLambdaNotification(t, svc, "lambda-notify-bucket", "images/source.jpg")
	assertLambdaNotification(t, receiveLambdaInvocation(t, invocations), "ObjectCreated:Put", "images/source.jpg")

	copyObjectForLambdaNotification(t, svc, "lambda-notify-bucket", "images/source.jpg", "images/copy.jpg")
	assertLambdaNotification(t, receiveLambdaInvocation(t, invocations), "ObjectCreated:Copy", "images/copy.jpg")

	completeMultipartForLambdaNotification(t, store, svc, "lambda-notify-bucket", "images/multipart.jpg")
	assertLambdaNotification(t, receiveLambdaInvocation(t, invocations), "ObjectCreated:CompleteMultipartUpload", "images/multipart.jpg")
}

func newLambdaInvocationCaptureServer(t *testing.T) (*httptest.Server, <-chan lambdaInvocation) {
	t.Helper()

	invocations := make(chan lambdaInvocation, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read Lambda invocation body: %v", err)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		invocations <- lambdaInvocation{
			Path:           r.URL.Path,
			InvocationType: r.Header.Get("X-Amz-Invocation-Type"),
			Body:           body,
		}

		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("{}"))
	}))

	return server, invocations
}

func putLambdaNotificationConfig(t *testing.T, svc *Service, bucket, functionName string) {
	t.Helper()

	body := `<NotificationConfiguration><CloudFunctionConfiguration><Id>all-created</Id><CloudFunction>arn:aws:lambda:us-east-1:000000000000:function:` +
		functionName +
		`</CloudFunction><Event>s3:ObjectCreated:*</Event><Filter><S3Key><FilterRule><Name>prefix</Name><Value>images/</Value></FilterRule></S3Key></Filter></CloudFunctionConfiguration></NotificationConfiguration>`
	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"?notification", strings.NewReader(body))
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.PutBucketNotificationConfiguration(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT notification status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

func putObjectForLambdaNotification(t *testing.T, svc *Service, bucket, key string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, strings.NewReader("body"))
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", key)

	w := httptest.NewRecorder()
	svc.PutObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

func copyObjectForLambdaNotification(t *testing.T, svc *Service, bucket, srcKey, dstKey string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+dstKey, http.NoBody)
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", dstKey)
	req.Header.Set("X-Amz-Copy-Source", "/"+bucket+"/"+srcKey)

	w := httptest.NewRecorder()
	svc.CopyObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CopyObject status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

func completeMultipartForLambdaNotification(t *testing.T, store *MemoryStorage, svc *Service, bucket, key string) {
	t.Helper()

	upload, err := store.CreateMultipartUpload(t.Context(), bucket, key)
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	part, err := store.UploadPart(t.Context(), bucket, key, upload.UploadID, 1, strings.NewReader("part"))
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}

	body := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>` + part.ETag + `</ETag></Part></CompleteMultipartUpload>`
	req := httptest.NewRequest(http.MethodPost, "/"+bucket+"/"+key+"?uploadId="+upload.UploadID, strings.NewReader(body))
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", key)

	w := httptest.NewRecorder()
	svc.CompleteMultipartUpload(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

func receiveLambdaInvocation(t *testing.T, invocations <-chan lambdaInvocation) lambdaInvocation {
	t.Helper()

	select {
	case invocation := <-invocations:
		return invocation
	case <-time.After(750 * time.Millisecond):
		t.Fatal("timed out waiting for Lambda invocation")
	}

	return lambdaInvocation{}
}

func assertLambdaNotification(t *testing.T, invocation lambdaInvocation, eventName, key string) {
	t.Helper()

	if invocation.Path != "/lambda/2015-03-31/functions/image-handler/invocations" {
		t.Fatalf("Lambda invocation path: got %q", invocation.Path)
	}

	if invocation.InvocationType != "Event" {
		t.Fatalf("Lambda invocation type: got %q, want Event", invocation.InvocationType)
	}

	var envelope s3NotificationEnvelope
	if err := json.Unmarshal(invocation.Body, &envelope); err != nil {
		t.Fatalf("unmarshal Lambda notification: %v body=%s", err, string(invocation.Body))
	}

	if len(envelope.Records) != 1 {
		t.Fatalf("Records: got %d, want 1", len(envelope.Records))
	}

	if envelope.Records[0].EventName != eventName {
		t.Fatalf("eventName: got %q, want %q", envelope.Records[0].EventName, eventName)
	}

	if envelope.Records[0].S3.Object.Key != key {
		t.Fatalf("object key: got %q, want %q", envelope.Records[0].S3.Object.Key, key)
	}
}
