package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCopyObjectCopiesSourceTagsByDefault(t *testing.T) {
	t.Parallel()

	store, svc := setupCopyObjectTaggingFixture(t)
	w := issueTaggedCopyObject(svc, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("CopyObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	tags, err := store.GetObjectTagging(context.Background(), "dst", "copied.txt")
	if err != nil {
		t.Fatalf("GetObjectTagging dst: %v", err)
	}

	if got := tags["color"]; got != "blue" {
		t.Fatalf("tag color: got %q, want blue", got)
	}
}

func TestCopyObjectReplacesTagsWhenDirectiveIsReplace(t *testing.T) {
	t.Parallel()

	store, svc := setupCopyObjectTaggingFixture(t)
	w := issueTaggedCopyObject(svc, map[string]string{
		"X-Amz-Tagging-Directive": "REPLACE",
		"X-Amz-Tagging":           "color=red&env=prod",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("CopyObject status: got %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	tags, err := store.GetObjectTagging(context.Background(), "dst", "copied.txt")
	if err != nil {
		t.Fatalf("GetObjectTagging dst: %v", err)
	}

	if got := tags["color"]; got != "red" {
		t.Fatalf("tag color: got %q, want red", got)
	}

	if got := tags["env"]; got != "prod" {
		t.Fatalf("tag env: got %q, want prod", got)
	}
}

func TestCopyObjectRejectsInvalidTaggingDirective(t *testing.T) {
	t.Parallel()

	store, svc := setupCopyObjectTaggingFixture(t)
	w := issueTaggedCopyObject(svc, map[string]string{
		"X-Amz-Tagging-Directive": "BROKEN",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("CopyObject status: got %d, want %d (body=%s)", w.Code, http.StatusBadRequest, w.Body.String())
	}

	if _, err := store.GetObject(context.Background(), "dst", "copied.txt"); err == nil {
		t.Fatal("destination object was stored for invalid tagging directive")
	}
}

func setupCopyObjectTaggingFixture(t *testing.T) (*MemoryStorage, *Service) {
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

	if _, err := store.PutObject(ctx, "src", "source.txt", strings.NewReader("copy me"), nil); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	if err := store.PutObjectTagging(ctx, "src", "source.txt", map[string]string{"color": "blue"}); err != nil {
		t.Fatalf("PutObjectTagging: %v", err)
	}

	return store, svc
}

func issueTaggedCopyObject(svc *Service, headers map[string]string) *httptest.ResponseRecorder {
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
