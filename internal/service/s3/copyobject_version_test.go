package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCopyObjectCopiesSpecifiedSourceVersion(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	v1, v2 := setupVersionedSourceObject(t, store)

	req := httptest.NewRequest(http.MethodPut, "/dst/copied.txt", http.NoBody)
	req.SetPathValue("bucket", "dst")
	req.SetPathValue("key", "copied.txt")
	req.Header.Set("X-Amz-Copy-Source", "/src/source.txt?versionId="+v1)

	w := httptest.NewRecorder()
	svc.CopyObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CopyObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	if got := w.Header().Get("x-amz-copy-source-version-id"); got != v1 {
		t.Fatalf("copy source version header: got %q, want %q", got, v1)
	}

	dstObj, err := store.GetObject(ctx, "dst", "copied.txt")
	if err != nil {
		t.Fatalf("GetObject dst: %v", err)
	}

	if string(dstObj.Body) != "old" {
		t.Fatalf("copied body: got %q from latest %s, want old from %s", dstObj.Body, v2, v1)
	}
}

func TestUploadPartCopyCopiesSpecifiedSourceVersion(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	v1, v2 := setupVersionedSourceObject(t, store)

	upload, err := store.CreateMultipartUpload(ctx, "dst", "joined.txt")
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/dst/joined.txt?partNumber=1&uploadId="+upload.UploadID, http.NoBody)
	req.SetPathValue("bucket", "dst")
	req.SetPathValue("key", "joined.txt")
	req.Header.Set("X-Amz-Copy-Source", "/src/source.txt?versionId="+v1)

	w := httptest.NewRecorder()
	svc.UploadPartCopy(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UploadPartCopy status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	part, ok := upload.Parts[1]
	if !ok {
		t.Fatal("copied part was not stored")
	}

	if string(part.Body) != "old" {
		t.Fatalf("copied part body: got %q from latest %s, want old from %s", part.Body, v2, v1)
	}
}

func setupVersionedSourceObject(t *testing.T, store *MemoryStorage) (string, string) {
	t.Helper()

	ctx := context.Background()
	if err := store.CreateBucket(ctx, "src"); err != nil {
		t.Fatalf("CreateBucket src: %v", err)
	}

	if err := store.CreateBucket(ctx, "dst"); err != nil {
		t.Fatalf("CreateBucket dst: %v", err)
	}

	if err := store.PutBucketVersioning(ctx, "src", VersioningEnabled); err != nil {
		t.Fatalf("PutBucketVersioning: %v", err)
	}

	first, err := store.PutObject(ctx, "src", "source.txt", strings.NewReader("old"), nil)
	if err != nil {
		t.Fatalf("PutObject first: %v", err)
	}

	second, err := store.PutObject(ctx, "src", "source.txt", strings.NewReader("new"), nil)
	if err != nil {
		t.Fatalf("PutObject second: %v", err)
	}

	return first.VersionID, second.VersionID
}
