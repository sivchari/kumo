package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestListObjectsV1_BasicRoundTrip — without ?list-type=2 the bucket
// GET handler dispatches to the V1 marker-paginated response shape
// (no KeyCount, has Marker / NextMarker).
func TestListObjectsV1_BasicRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	_ = store.CreateBucket(ctx, "lb")

	for _, k := range []string{"a", "b", "c"} {
		_, _ = store.PutObject(ctx, "lb", k, strings.NewReader("x"), nil)
	}

	req := httptest.NewRequest(http.MethodGet, "/lb", http.NoBody)
	req.SetPathValue("bucket", "lb")

	w := httptest.NewRecorder()
	svc.handleBucketGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var got ListBucketResultV1
	if err := xml.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}

	if len(got.Contents) != 3 {
		t.Fatalf("contents: got %d, want 3", len(got.Contents))
	}

	if got.IsTruncated {
		t.Fatalf("IsTruncated: got true, want false (small list)")
	}

	// Crucial: V1 response must NOT contain a KeyCount element.
	if strings.Contains(w.Body.String(), "<KeyCount>") {
		t.Fatalf("V1 response leaked <KeyCount> tag: %s", w.Body.String())
	}
}

// TestListObjectsV1_MarkerPagination — marker skips entries <= marker
// and IsTruncated is true while there's more to return.
func TestListObjectsV1_MarkerPagination(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	_ = store.CreateBucket(ctx, "page")

	for _, k := range []string{"a", "b", "c", "d", "e"} {
		_, _ = store.PutObject(ctx, "page", k, strings.NewReader("x"), nil)
	}

	// Page 1: max-keys=2
	req := httptest.NewRequest(http.MethodGet, "/page?max-keys=2", http.NoBody)
	req.SetPathValue("bucket", "page")

	w := httptest.NewRecorder()
	svc.handleBucketGet(w, req)

	var page1 ListBucketResultV1
	_ = xml.Unmarshal(w.Body.Bytes(), &page1)

	if len(page1.Contents) != 2 {
		t.Fatalf("page1 contents: got %d, want 2", len(page1.Contents))
	}

	if !page1.IsTruncated {
		t.Fatalf("page1 IsTruncated: got false, want true (5 total, 2 fetched)")
	}

	// Page 2: marker=last key from page 1
	lastKey := page1.Contents[len(page1.Contents)-1].Key

	req2 := httptest.NewRequest(http.MethodGet, "/page?max-keys=2&marker="+lastKey, http.NoBody)
	req2.SetPathValue("bucket", "page")

	w2 := httptest.NewRecorder()
	svc.handleBucketGet(w2, req2)

	var page2 ListBucketResultV1
	_ = xml.Unmarshal(w2.Body.Bytes(), &page2)

	if len(page2.Contents) != 2 {
		t.Fatalf("page2 contents: got %d, want 2 (entries after marker=%s)", len(page2.Contents), lastKey)
	}

	for _, c := range page2.Contents {
		if c.Key <= lastKey {
			t.Fatalf("page2 key %q should be > marker %q", c.Key, lastKey)
		}
	}
}

// TestListObjectsV2_StillUsesContinuationToken — list-type=2 still
// dispatches to the V2 path (KeyCount present).
func TestListObjectsV2_StillUsesContinuationToken(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	_ = store.CreateBucket(ctx, "v2")
	_, _ = store.PutObject(ctx, "v2", "k", strings.NewReader("x"), nil)

	req := httptest.NewRequest(http.MethodGet, "/v2?list-type=2", http.NoBody)
	req.SetPathValue("bucket", "v2")

	w := httptest.NewRecorder()
	svc.handleBucketGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	if !strings.Contains(w.Body.String(), "<KeyCount>") {
		t.Fatalf("V2 response missing <KeyCount>: %s", w.Body.String())
	}
}
