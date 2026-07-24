package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const completeMultipartVersionBody = "hello"

func TestCompleteMultipartUploadCreatesVersionWhenBucketVersioningEnabled(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	if err := store.CreateBucket(ctx, "bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	if err := store.PutBucketVersioning(ctx, "bucket", VersioningEnabled); err != nil {
		t.Fatalf("PutBucketVersioning: %v", err)
	}

	upload, err := store.CreateMultipartUpload(ctx, "bucket", "object", nil)
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	part, err := store.UploadPart(ctx, "bucket", "object", upload.UploadID, 1, strings.NewReader(completeMultipartVersionBody))
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}

	obj, err := store.CompleteMultipartUpload(ctx, "bucket", "object", upload.UploadID, []PartRequest{
		{PartNumber: 1, ETag: part.ETag},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}

	if obj.VersionID == "" {
		t.Fatal("completed multipart object has empty version ID")
	}

	versioned, err := store.GetObjectVersion(ctx, "bucket", "object", obj.VersionID)
	if err != nil {
		t.Fatalf("GetObjectVersion: %v", err)
	}

	if string(versioned.Body) != completeMultipartVersionBody {
		t.Fatalf("versioned body: got %q, want %q", versioned.Body, completeMultipartVersionBody)
	}

	current, err := store.GetObject(ctx, "bucket", "object")
	if err != nil {
		t.Fatalf("GetObject current: %v", err)
	}

	if current.VersionID != obj.VersionID {
		t.Fatalf("current version ID: got %q, want %q", current.VersionID, obj.VersionID)
	}

	if string(current.Body) != completeMultipartVersionBody {
		t.Fatalf("current body: got %q, want %q", current.Body, completeMultipartVersionBody)
	}
}

func TestCompleteMultipartUploadUsesNullVersionWhenVersioningSuspended(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	if err := store.CreateBucket(ctx, "bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	if err := store.PutBucketVersioning(ctx, "bucket", VersioningSuspended); err != nil {
		t.Fatalf("PutBucketVersioning: %v", err)
	}

	first := completeSinglePartUpload(t, store, "first")
	if first.VersionID != VersionIDNull {
		t.Fatalf("first version ID: got %q, want %q", first.VersionID, VersionIDNull)
	}

	second := completeSinglePartUpload(t, store, "second")
	if second.VersionID != VersionIDNull {
		t.Fatalf("second version ID: got %q, want %q", second.VersionID, VersionIDNull)
	}

	versions, _, err := store.ListObjectVersions(ctx, "bucket", "object", "", 100)
	if err != nil {
		t.Fatalf("ListObjectVersions: %v", err)
	}

	if len(versions) != 1 {
		t.Fatalf("versions: got %d, want 1 null version", len(versions))
	}

	nullVersion, err := store.GetObjectVersion(ctx, "bucket", "object", VersionIDNull)
	if err != nil {
		t.Fatalf("GetObjectVersion null: %v", err)
	}

	if string(nullVersion.Body) != "second" {
		t.Fatalf("surviving null version body: got %q, want second", nullVersion.Body)
	}
}

func TestCompleteMultipartUploadHandlerSetsVersionIDHeader(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	if err := store.CreateBucket(ctx, "bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	if err := store.PutBucketVersioning(ctx, "bucket", VersioningEnabled); err != nil {
		t.Fatalf("PutBucketVersioning: %v", err)
	}

	upload, err := store.CreateMultipartUpload(ctx, "bucket", "object", nil)
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	part, err := store.UploadPart(ctx, "bucket", "object", upload.UploadID, 1, strings.NewReader(completeMultipartVersionBody))
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}

	svc := New(store, "http://example")

	body := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>` + part.ETag + `</ETag></Part></CompleteMultipartUpload>`
	req := httptest.NewRequest(http.MethodPost, "/bucket/object?uploadId="+upload.UploadID, strings.NewReader(body))
	req.SetPathValue("bucket", "bucket")
	req.SetPathValue("key", "object")

	w := httptest.NewRecorder()
	svc.CompleteMultipartUpload(w, req)

	if got := w.Code; got != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", got, http.StatusOK, w.Body.String())
	}

	if got := w.Header().Get("x-amz-version-id"); got == "" {
		t.Fatal("expected x-amz-version-id header to be set when bucket has versioning enabled")
	}
}

func completeSinglePartUpload(t *testing.T, store *MemoryStorage, body string) *Object {
	t.Helper()

	ctx := context.Background()

	upload, err := store.CreateMultipartUpload(ctx, "bucket", "object", nil)
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	part, err := store.UploadPart(ctx, "bucket", "object", upload.UploadID, 1, strings.NewReader(body))
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}

	obj, err := store.CompleteMultipartUpload(ctx, "bucket", "object", upload.UploadID, []PartRequest{
		{PartNumber: 1, ETag: part.ETag},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}

	return obj
}
