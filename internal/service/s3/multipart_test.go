package s3

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompleteMultipartUploadRejectsEmptyPartList(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	if err := store.CreateBucket(ctx, "mpu-test"); err != nil {
		t.Fatal(err)
	}

	upload, err := store.CreateMultipartUpload(ctx, "mpu-test", "object", nil)
	if err != nil {
		t.Fatal(err)
	}

	part, err := store.UploadPart(ctx, "mpu-test", "object", upload.UploadID, 1, strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.CompleteMultipartUpload(ctx, "mpu-test", "object", upload.UploadID, nil)
	if err == nil {
		t.Fatal("expected empty part list to be rejected")
	}

	var multipartErr *MultipartError
	if !errors.As(err, &multipartErr) || multipartErr.Code != "MalformedXML" {
		t.Fatalf("expected MalformedXML, got %v", err)
	}

	obj, err := store.CompleteMultipartUpload(ctx, "mpu-test", "object", upload.UploadID, []PartRequest{
		{PartNumber: 1, ETag: part.ETag},
	})
	if err != nil {
		t.Fatalf("multipart upload should remain available after rejected empty completion: %v", err)
	}

	if string(obj.Body) != "hello" {
		t.Fatalf("body = %q, want %q", string(obj.Body), "hello")
	}
}

func TestCompleteMultipartUploadEmptyPartsHandlerReturnsMalformedXML(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "http://localhost:4566")
	ctx := context.Background()

	if err := store.CreateBucket(ctx, "mpu-handler-test"); err != nil {
		t.Fatal(err)
	}

	upload, err := store.CreateMultipartUpload(ctx, "mpu-handler-test", "object", nil)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/mpu-handler-test/object?uploadId="+upload.UploadID,
		strings.NewReader(`<CompleteMultipartUpload></CompleteMultipartUpload>`),
	)
	req.SetPathValue("bucket", "mpu-handler-test")
	req.SetPathValue("key", "object")

	w := httptest.NewRecorder()
	svc.CompleteMultipartUpload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, body=%s", w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "<Code>MalformedXML</Code>") {
		t.Fatalf("expected MalformedXML response, got %s", w.Body.String())
	}
}
