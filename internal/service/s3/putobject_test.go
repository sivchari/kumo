package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testBucketDefaultKMSKeyID = "bucket-default-key"

// TestPutObjectFallsBackToDestinationBucketDefaultEncryption verifies that,
// absent SSE headers on the request, PutObject picks up the destination
// bucket's default encryption, matching AWS's PutObject semantics.
func TestPutObjectFallsBackToDestinationBucketDefaultEncryption(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(context.Background(), "put-object-sse-test"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	if err := store.PutBucketEncryption(context.Background(), "put-object-sse-test", ServerSideEncryptionConfig{
		Rules: []ServerSideEncryptionRule{
			{SSEAlgorithm: sseAlgorithmAWSKMS, KMSMasterKeyID: testBucketDefaultKMSKeyID},
		},
	}); err != nil {
		t.Fatalf("PutBucketEncryption: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/put-object-sse-test/object.txt", strings.NewReader("hello"))
	req.SetPathValue("bucket", "put-object-sse-test")
	req.SetPathValue("key", "object.txt")

	w := httptest.NewRecorder()
	svc.PutObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	dstObj, err := store.GetObject(context.Background(), "put-object-sse-test", "object.txt")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}

	if dstObj.ServerSideEncryption != sseAlgorithmAWSKMS {
		t.Fatalf("ServerSideEncryption: got %q, want %s", dstObj.ServerSideEncryption, sseAlgorithmAWSKMS)
	}

	if dstObj.SSEKMSKeyID != testBucketDefaultKMSKeyID {
		t.Fatalf("SSEKMSKeyID: got %q, want %s", dstObj.SSEKMSKeyID, testBucketDefaultKMSKeyID)
	}
}

// TestPutObjectRequestSSEHeadersWinOverBucketDefault verifies that explicit
// SSE headers on the PutObject request take precedence over the destination
// bucket's default encryption.
func TestPutObjectRequestSSEHeadersWinOverBucketDefault(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(context.Background(), "put-object-sse-test"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	if err := store.PutBucketEncryption(context.Background(), "put-object-sse-test", ServerSideEncryptionConfig{
		Rules: []ServerSideEncryptionRule{
			{SSEAlgorithm: sseAlgorithmAWSKMS, KMSMasterKeyID: testBucketDefaultKMSKeyID},
		},
	}); err != nil {
		t.Fatalf("PutBucketEncryption: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/put-object-sse-test/object.txt", strings.NewReader("hello"))
	req.SetPathValue("bucket", "put-object-sse-test")
	req.SetPathValue("key", "object.txt")
	req.Header.Set("X-Amz-Server-Side-Encryption", sseAlgorithmAWSKMS)
	req.Header.Set("X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id", "request-key")

	w := httptest.NewRecorder()
	svc.PutObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	dstObj, err := store.GetObject(context.Background(), "put-object-sse-test", "object.txt")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}

	if dstObj.SSEKMSKeyID != "request-key" {
		t.Fatalf("SSEKMSKeyID: got %q, want request-key", dstObj.SSEKMSKeyID)
	}
}
