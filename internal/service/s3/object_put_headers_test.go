package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPutObject_TaggingHeaderIsReturnedByGetObjectTagging(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	if err := store.CreateBucket(ctx, "tag-bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/tag-bucket/k", strings.NewReader("body"))
	putReq.SetPathValue("bucket", "tag-bucket")
	putReq.SetPathValue("key", "k")
	putReq.Header.Set("X-Amz-Tagging", "env=dev&space=a%20b")

	putW := httptest.NewRecorder()
	svc.PutObject(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want 200 (body=%s)", putW.Code, putW.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/tag-bucket/k?tagging", http.NoBody)
	getReq.SetPathValue("bucket", "tag-bucket")
	getReq.SetPathValue("key", "k")

	getW := httptest.NewRecorder()
	svc.GetObjectTagging(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("GetObjectTagging status: got %d, want 200 (body=%s)", getW.Code, getW.Body.String())
	}

	var got Tagging
	if err := xml.Unmarshal(getW.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal tagging: %v body=%s", err, getW.Body.String())
	}

	want := map[string]string{"env": "dev", "space": "a b"}
	gotTags := make(map[string]string, len(got.TagSet.Tags))

	for _, tag := range got.TagSet.Tags {
		gotTags[tag.Key] = tag.Value
	}

	for key, wantValue := range want {
		if gotTags[key] != wantValue {
			t.Fatalf("tag %q: got %q, want %q (all=%v)", key, gotTags[key], wantValue, gotTags)
		}
	}
}

func TestPutObject_SSEKMSHeadersRoundTripThroughHeadAndGet(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	if err := store.CreateBucket(ctx, "sse-bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	putObjectWithSSEKMSHeaders(t, svc)

	for _, tc := range []struct {
		name string
		call func(*testing.T, *Service) *httptest.ResponseRecorder
	}{
		{
			name: "HeadObject",
			call: recordSSEKMSHeadObject,
		},
		{
			name: "GetObject",
			call: recordSSEKMSGetObject,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := tc.call(t, svc)
			if w.Code != http.StatusOK {
				t.Fatalf("%s status: got %d, want 200 (body=%s)", tc.name, w.Code, w.Body.String())
			}

			assertSSEKMSHeaders(t, tc.name, w.Header())
		})
	}
}

const (
	sseKMSBucketName = "sse-bucket"
	sseKMSObjectKey  = "k"
	sseKMSAlgorithm  = "aws:kms"
	sseKMSKeyID      = "arn:aws:kms:us-east-1:123456789012:key/test"
)

func putObjectWithSSEKMSHeaders(t *testing.T, svc *Service) {
	t.Helper()

	putReq := httptest.NewRequest(http.MethodPut, "/sse-bucket/k", strings.NewReader("body"))
	putReq.SetPathValue("bucket", sseKMSBucketName)
	putReq.SetPathValue("key", sseKMSObjectKey)
	putReq.Header.Set("X-Amz-Server-Side-Encryption", sseKMSAlgorithm)
	putReq.Header.Set("X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id", sseKMSKeyID)
	putReq.Header.Set("X-Amz-Server-Side-Encryption-Bucket-Key-Enabled", "true")

	putW := httptest.NewRecorder()
	svc.PutObject(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want 200 (body=%s)", putW.Code, putW.Body.String())
	}
}

func recordSSEKMSHeadObject(t *testing.T, svc *Service) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodHead, "/sse-bucket/k", http.NoBody)
	req.SetPathValue("bucket", sseKMSBucketName)
	req.SetPathValue("key", sseKMSObjectKey)

	w := httptest.NewRecorder()
	svc.HeadObject(w, req)

	return w
}

func recordSSEKMSGetObject(t *testing.T, svc *Service) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/sse-bucket/k", http.NoBody)
	req.SetPathValue("bucket", sseKMSBucketName)
	req.SetPathValue("key", sseKMSObjectKey)

	w := httptest.NewRecorder()
	svc.GetObject(w, req)

	return w
}

func assertSSEKMSHeaders(t *testing.T, name string, headers http.Header) {
	t.Helper()

	if got := headers.Get("x-amz-server-side-encryption"); got != sseKMSAlgorithm {
		t.Fatalf("%s SSE algorithm: got %q, want %q", name, got, sseKMSAlgorithm)
	}

	if got := headers.Get("x-amz-server-side-encryption-aws-kms-key-id"); got != sseKMSKeyID {
		t.Fatalf("%s SSE KMS key id: got %q, want %q", name, got, sseKMSKeyID)
	}

	if got := headers.Get("x-amz-server-side-encryption-bucket-key-enabled"); got != "true" {
		t.Fatalf("%s bucket key enabled: got %q, want true", name, got)
	}
}
