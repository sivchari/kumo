package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testObjTagBucket = "obj-tag-bucket"
	testObjTagKey    = "tagged.txt"
)

// newObjectTaggingFixture returns a service whose storage holds one bucket
// with one object carrying a single Owner tag.
func newObjectTaggingFixture(t *testing.T) (*MemoryStorage, *Service) {
	t.Helper()

	ctx := context.Background()
	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(ctx, testObjTagBucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	if _, err := store.PutObject(ctx, testObjTagBucket, testObjTagKey, strings.NewReader("body"), nil); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	if err := store.PutObjectTagging(ctx, testObjTagBucket, testObjTagKey,
		map[string]string{testTagKeyOwner: testTagValueOwner}); err != nil {
		t.Fatalf("PutObjectTagging: %v", err)
	}

	return store, svc
}

func issuePutObjectTagging(t *testing.T, svc *Service, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut,
		"/"+testObjTagBucket+"/"+testObjTagKey+"?tagging", strings.NewReader(body))
	req.SetPathValue("bucket", testObjTagBucket)
	req.SetPathValue("key", testObjTagKey)

	w := httptest.NewRecorder()
	svc.handleObjectPut(w, req)

	return w
}

func issuePutObjectWithTagging(t *testing.T, svc *Service, key, tagging string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut,
		"/"+testObjTagBucket+"/"+key, strings.NewReader("body"))
	req.SetPathValue("bucket", testObjTagBucket)
	req.SetPathValue("key", key)
	req.Header.Set("X-Amz-Tagging", tagging)

	w := httptest.NewRecorder()
	svc.PutObject(w, req)

	return w
}

func TestPutObjectTaggingRejectsDuplicateKeys(t *testing.T) {
	t.Parallel()

	store, svc := newObjectTaggingFixture(t)

	w := issuePutObjectTagging(t, svc, bucketTaggingXML(testTagKeyEnv, "dev", testTagKeyEnv, "prod"))
	assertS3ErrorCode(t, w, http.StatusBadRequest, errCodeInvalidTag)

	tags, err := store.GetObjectTagging(context.Background(), testObjTagBucket, testObjTagKey)
	if err != nil {
		t.Fatalf("GetObjectTagging: %v", err)
	}

	if len(tags) != 1 || tags[testTagKeyOwner] != testTagValueOwner {
		t.Errorf("tags = %v after a rejected PutObjectTagging, want the previous set kept", tags)
	}
}

func TestPutObjectTaggingAcceptsDistinctKeys(t *testing.T) {
	t.Parallel()

	store, svc := newObjectTaggingFixture(t)

	w := issuePutObjectTagging(t, svc, bucketTaggingXML(testTagKeyEnv, testTagValueEnv, "environment", "prod"))
	if w.Code != http.StatusOK {
		t.Fatalf("PutObjectTagging status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	tags, err := store.GetObjectTagging(context.Background(), testObjTagBucket, testObjTagKey)
	if err != nil {
		t.Fatalf("GetObjectTagging: %v", err)
	}

	if len(tags) != 2 || tags[testTagKeyEnv] != testTagValueEnv || tags["environment"] != testTagValueProd {
		t.Errorf("tags = %v, want Environment=dev and environment=prod", tags)
	}
}

func TestPutObjectRejectsDuplicateTaggingHeaderKeys(t *testing.T) {
	t.Parallel()

	store, svc := newObjectTaggingFixture(t)

	w := issuePutObjectWithTagging(t, svc, "dup.txt", "env=dev&env=prod")
	assertS3ErrorCode(t, w, http.StatusBadRequest, errCodeInvalidTag)

	if _, err := store.GetObject(context.Background(), testObjTagBucket, "dup.txt"); err == nil {
		t.Error("object was stored despite duplicate keys in x-amz-tagging")
	}
}

func TestPutObjectAcceptsDistinctTaggingHeaderKeys(t *testing.T) {
	t.Parallel()

	store, svc := newObjectTaggingFixture(t)

	w := issuePutObjectWithTagging(t, svc, "ok.txt", "env=dev&Env=prod")
	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	tags, err := store.GetObjectTagging(context.Background(), testObjTagBucket, "ok.txt")
	if err != nil {
		t.Fatalf("GetObjectTagging: %v", err)
	}

	if len(tags) != 2 || tags["env"] != "dev" || tags["Env"] != "prod" {
		t.Errorf("tags = %v, want env=dev and Env=prod", tags)
	}
}

func TestCopyObjectRejectsDuplicateTaggingHeaderKeys(t *testing.T) {
	t.Parallel()

	store, svc := setupCopyObjectTaggingFixture(t)

	w := issueTaggedCopyObject(t, svc, map[string]string{
		taggingDirectiveHeader: taggingDirectiveReplace,
		taggingHeader:          "env=dev&env=prod",
	})
	assertS3ErrorCode(t, w, http.StatusBadRequest, errCodeInvalidTag)

	if _, err := store.GetObject(context.Background(), "dst", "copied.txt"); err == nil {
		t.Error("destination object was stored despite duplicate keys in x-amz-tagging")
	}
}
