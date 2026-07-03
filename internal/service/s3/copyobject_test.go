package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCopyObjectReplacesMetadataWhenDirectiveIsReplace(t *testing.T) {
	t.Parallel()

	store, svc := setupCopyObjectMetadataFixture(t)
	w := issueCopyObject(svc, map[string]string{
		"X-Amz-Metadata-Directive": "REPLACE",
		"X-Amz-Meta-Color":         "red",
		"Content-Type":             "application/json",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("CopyObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	dstObj, err := store.GetObject(context.Background(), "dst", "copied.txt")
	if err != nil {
		t.Fatalf("GetObject dst: %v", err)
	}

	if got := dstObj.Metadata["color"]; got != "red" {
		t.Fatalf("metadata color: got %q, want red", got)
	}

	if got := dstObj.Metadata["Content-Type"]; got != "application/json" {
		t.Fatalf("metadata Content-Type: got %q, want application/json", got)
	}
}

func TestCopyObjectCopiesSourceMetadataByDefault(t *testing.T) {
	t.Parallel()

	store, svc := setupCopyObjectMetadataFixture(t)
	w := issueCopyObject(svc, map[string]string{
		"X-Amz-Meta-Color": "red",
		"Content-Type":     "application/json",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("CopyObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	dstObj, err := store.GetObject(context.Background(), "dst", "copied.txt")
	if err != nil {
		t.Fatalf("GetObject dst: %v", err)
	}

	if got := dstObj.Metadata["color"]; got != "blue" {
		t.Fatalf("metadata color: got %q, want blue", got)
	}

	if got := dstObj.Metadata["Content-Type"]; got != "text/plain" {
		t.Fatalf("metadata Content-Type: got %q, want text/plain", got)
	}
}

func TestCopyObjectRejectsInvalidMetadataDirective(t *testing.T) {
	t.Parallel()

	store, svc := setupCopyObjectMetadataFixture(t)
	w := issueCopyObject(svc, map[string]string{
		"X-Amz-Metadata-Directive": "BROKEN",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("CopyObject status: got %d, want %d (body=%s)", w.Code, http.StatusBadRequest, w.Body.String())
	}

	if _, err := store.GetObject(context.Background(), "dst", "copied.txt"); err == nil {
		t.Fatal("destination object was stored for invalid metadata directive")
	}
}

func setupCopyObjectMetadataFixture(t *testing.T) (*MemoryStorage, *Service) {
	t.Helper()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	if err := store.CreateBucket(ctx, "src"); err != nil {
		t.Fatalf("CreateBucket src: %v", err)
	}

	if err := store.CreateBucket(ctx, "dst"); err != nil {
		t.Fatalf("CreateBucket dst: %v", err)
	}

	_, err := store.PutObject(ctx, "src", "source.txt", strings.NewReader("copy me"), map[string]string{
		"color":        "blue",
		"Content-Type": "text/plain",
	})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	return store, svc
}

func issueCopyObject(svc *Service, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/dst/copied.txt", http.NoBody)
	req.SetPathValue("bucket", "dst")
	req.SetPathValue("key", "copied.txt")
	req.Header.Set("X-Amz-Copy-Source", "/src/source.txt")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	w := httptest.NewRecorder()
	svc.CopyObject(w, req)

	return w
}
