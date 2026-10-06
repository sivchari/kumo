package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testTaggedBucket  = "tagged-bucket"
	testTagKeyOwner   = "Owner"
	testTagValueOwner = "team-a"
	testTagKeyEnv     = "Environment"
	testTagValueEnv   = "dev"
)

// bucketTaggingXML renders a Tagging document from alternating key/value
// arguments.
func bucketTaggingXML(keyValues ...string) string {
	return "<Tagging><TagSet>" + tagElements(keyValues...) + "</TagSet></Tagging>"
}

// createBucketConfigurationXML renders a CreateBucket request body carrying a
// TagSet, the tag-on-create form clients use to create a bucket and its tags in
// one call.
func createBucketConfigurationXML(keyValues ...string) string {
	return `<CreateBucketConfiguration xmlns="` + s3Namespace + `"><Tags>` +
		tagElements(keyValues...) + `</Tags></CreateBucketConfiguration>`
}

// tagElements renders one Tag element per key/value pair, in the order given.
func tagElements(keyValues ...string) string {
	var b strings.Builder

	for i := 0; i+1 < len(keyValues); i += 2 {
		b.WriteString("<Tag><Key>" + keyValues[i] + "</Key><Value>" + keyValues[i+1] + "</Value></Tag>")
	}

	return b.String()
}

// newBucketTaggingFixture returns a service whose storage already holds one
// bucket, so tests can exercise the bucket-level ?tagging sub-resource without
// creating the bucket first.
func newBucketTaggingFixture(t *testing.T) (*MemoryStorage, *Service) {
	t.Helper()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(context.Background(), testTaggedBucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	return store, svc
}

// issueBucketSubresourceRequest drives the bucket-level route dispatcher for
// the ?tagging sub-resource rather than calling the sub-resource handler
// directly: routing PUT ?tagging to the tagging handler instead of falling
// through to CreateBucket is part of the behaviour under test.
func issueBucketSubresourceRequest(t *testing.T, svc *Service, method, bucket, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, "/"+bucket+"?tagging", strings.NewReader(body))
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()

	switch method {
	case http.MethodPut:
		svc.handleBucketPut(w, req)
	case http.MethodGet:
		svc.handleBucketGet(w, req)
	case http.MethodDelete:
		svc.handleBucketDelete(w, req)
	default:
		t.Fatalf("unsupported method %q", method)
	}

	return w
}

// assertS3ErrorCode decodes an XML error body and checks the error code.
func assertS3ErrorCode(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()

	if w.Code != status {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, status, w.Body.String())
	}

	var errResp ErrorResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}

	if errResp.Code != code {
		t.Errorf("error code = %q, want %q", errResp.Code, code)
	}
}

func TestPutBucketTaggingOnExistingBucketSucceeds(t *testing.T) {
	t.Parallel()

	_, svc := newBucketTaggingFixture(t)

	w := issueBucketSubresourceRequest(t, svc, http.MethodPut, testTaggedBucket,
		bucketTaggingXML(testTagKeyEnv, testTagValueEnv))

	if w.Code != http.StatusOK {
		t.Fatalf("PutBucketTagging status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestBucketTaggingRoundTrip(t *testing.T) {
	t.Parallel()

	store, svc := newBucketTaggingFixture(t)

	w := issueBucketSubresourceRequest(t, svc, http.MethodPut, testTaggedBucket,
		bucketTaggingXML(testTagKeyOwner, testTagValueOwner, testTagKeyEnv, testTagValueEnv))

	if w.Code != http.StatusOK {
		t.Fatalf("PutBucketTagging status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	tags, err := store.GetBucketTagging(context.Background(), testTaggedBucket)
	if err != nil {
		t.Fatalf("GetBucketTagging: %v", err)
	}

	if got := tags[testTagKeyEnv]; got != testTagValueEnv {
		t.Errorf("tag %s = %q, want %q", testTagKeyEnv, got, testTagValueEnv)
	}

	if got := tags[testTagKeyOwner]; got != testTagValueOwner {
		t.Errorf("tag %s = %q, want %q", testTagKeyOwner, got, testTagValueOwner)
	}

	w = issueBucketSubresourceRequest(t, svc, http.MethodGet, testTaggedBucket, "")

	if w.Code != http.StatusOK {
		t.Fatalf("GetBucketTagging status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	var got Tagging
	if err := xml.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal Tagging: %v", err)
	}

	want := []Tag{
		{Key: testTagKeyEnv, Value: testTagValueEnv},
		{Key: testTagKeyOwner, Value: testTagValueOwner},
	}

	if len(got.TagSet.Tags) != len(want) {
		t.Fatalf("tag count = %d, want %d (body=%s)", len(got.TagSet.Tags), len(want), w.Body.String())
	}

	for i, tag := range got.TagSet.Tags {
		if tag != want[i] {
			t.Errorf("tag[%d] = %+v, want %+v", i, tag, want[i])
		}
	}
}

func TestPutBucketTaggingReplacesPreviousTagSet(t *testing.T) {
	t.Parallel()

	store, svc := newBucketTaggingFixture(t)

	w := issueBucketSubresourceRequest(t, svc, http.MethodPut, testTaggedBucket,
		bucketTaggingXML(testTagKeyOwner, testTagValueOwner))
	if w.Code != http.StatusOK {
		t.Fatalf("PutBucketTagging status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	w = issueBucketSubresourceRequest(t, svc, http.MethodPut, testTaggedBucket,
		bucketTaggingXML(testTagKeyEnv, testTagValueEnv))
	if w.Code != http.StatusOK {
		t.Fatalf("PutBucketTagging status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	tags, err := store.GetBucketTagging(context.Background(), testTaggedBucket)
	if err != nil {
		t.Fatalf("GetBucketTagging: %v", err)
	}

	if _, ok := tags[testTagKeyOwner]; ok {
		t.Errorf("tag %s survived a replacing PutBucketTagging", testTagKeyOwner)
	}

	if got := tags[testTagKeyEnv]; got != testTagValueEnv {
		t.Errorf("tag %s = %q, want %q", testTagKeyEnv, got, testTagValueEnv)
	}
}

func TestGetBucketTaggingWithoutTagSetIsNotFound(t *testing.T) {
	t.Parallel()

	_, svc := newBucketTaggingFixture(t)

	w := issueBucketSubresourceRequest(t, svc, http.MethodGet, testTaggedBucket, "")

	assertS3ErrorCode(t, w, http.StatusNotFound, "NoSuchTagSet")
}

func TestPutBucketTaggingWithEmptyTagSetClearsTagSet(t *testing.T) {
	t.Parallel()

	_, svc := newBucketTaggingFixture(t)

	w := issueBucketSubresourceRequest(t, svc, http.MethodPut, testTaggedBucket,
		bucketTaggingXML(testTagKeyEnv, testTagValueEnv))
	if w.Code != http.StatusOK {
		t.Fatalf("PutBucketTagging status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	w = issueBucketSubresourceRequest(t, svc, http.MethodPut, testTaggedBucket, bucketTaggingXML())
	if w.Code != http.StatusOK {
		t.Fatalf("PutBucketTagging status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	w = issueBucketSubresourceRequest(t, svc, http.MethodGet, testTaggedBucket, "")

	assertS3ErrorCode(t, w, http.StatusNotFound, "NoSuchTagSet")
}

func TestDeleteBucketTaggingIsIdempotent(t *testing.T) {
	t.Parallel()

	_, svc := newBucketTaggingFixture(t)

	w := issueBucketSubresourceRequest(t, svc, http.MethodPut, testTaggedBucket,
		bucketTaggingXML(testTagKeyEnv, testTagValueEnv))
	if w.Code != http.StatusOK {
		t.Fatalf("PutBucketTagging status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	w = issueBucketSubresourceRequest(t, svc, http.MethodDelete, testTaggedBucket, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteBucketTagging status = %d, want %d (body=%s)", w.Code, http.StatusNoContent, w.Body.String())
	}

	w = issueBucketSubresourceRequest(t, svc, http.MethodGet, testTaggedBucket, "")
	assertS3ErrorCode(t, w, http.StatusNotFound, "NoSuchTagSet")

	w = issueBucketSubresourceRequest(t, svc, http.MethodDelete, testTaggedBucket, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("repeated DeleteBucketTagging status = %d, want %d (body=%s)", w.Code, http.StatusNoContent, w.Body.String())
	}
}

func TestPutBucketTaggingRejectsMalformedXML(t *testing.T) {
	t.Parallel()

	_, svc := newBucketTaggingFixture(t)

	w := issueBucketSubresourceRequest(t, svc, http.MethodPut, testTaggedBucket, "<Tagging>")

	assertS3ErrorCode(t, w, http.StatusBadRequest, errCodeMalformedXML)
}

func TestBucketTaggingOnMissingBucketIsNotFound(t *testing.T) {
	t.Parallel()

	_, svc := newBucketTaggingFixture(t)

	for _, tc := range []struct {
		method string
		body   string
	}{
		{http.MethodPut, bucketTaggingXML(testTagKeyEnv, testTagValueEnv)},
		{http.MethodGet, ""},
		{http.MethodDelete, ""},
	} {
		w := issueBucketSubresourceRequest(t, svc, tc.method, "no-such-bucket", tc.body)

		assertS3ErrorCode(t, w, http.StatusNotFound, errCodeNoSuchBucket)
	}
}

// issueCreateBucket drives PUT /{bucket} through the bucket dispatcher with the
// given CreateBucket request body.
func issueCreateBucket(t *testing.T, svc *Service, bucket, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/"+bucket, strings.NewReader(body))
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.handleBucketPut(w, req)

	return w
}

func TestCreateBucketWithTagSetStoresTags(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	w := issueCreateBucket(t, svc, testTaggedBucket,
		createBucketConfigurationXML(testTagKeyOwner, testTagValueOwner, testTagKeyEnv, testTagValueEnv))

	if w.Code != http.StatusOK {
		t.Fatalf("CreateBucket status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	tags, err := store.GetBucketTagging(context.Background(), testTaggedBucket)
	if err != nil {
		t.Fatalf("GetBucketTagging: %v", err)
	}

	if got := tags[testTagKeyEnv]; got != testTagValueEnv {
		t.Errorf("tag %s = %q, want %q", testTagKeyEnv, got, testTagValueEnv)
	}

	if got := tags[testTagKeyOwner]; got != testTagValueOwner {
		t.Errorf("tag %s = %q, want %q", testTagKeyOwner, got, testTagValueOwner)
	}
}

func TestCreateBucketWithoutBodyHasNoTagSet(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	w := issueCreateBucket(t, svc, testTaggedBucket, "")

	if w.Code != http.StatusOK {
		t.Fatalf("CreateBucket status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	w = issueBucketSubresourceRequest(t, svc, http.MethodGet, testTaggedBucket, "")

	assertS3ErrorCode(t, w, http.StatusNotFound, "NoSuchTagSet")
}

func TestCreateBucketRejectsMalformedConfigurationBody(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	w := issueCreateBucket(t, svc, testTaggedBucket, "<CreateBucketConfiguration>")

	assertS3ErrorCode(t, w, http.StatusBadRequest, errCodeMalformedXML)

	exists, err := store.BucketExists(context.Background(), testTaggedBucket)
	if err != nil {
		t.Fatalf("BucketExists: %v", err)
	}

	if exists {
		t.Error("bucket was created despite a malformed CreateBucketConfiguration body")
	}
}
