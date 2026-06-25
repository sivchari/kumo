package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- BucketObjectLock ------------------------------------------------

func TestBucketObjectLock_PutAndGet(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(), "")
	ctx := context.Background()

	store, ok := svc.storage.(*MemoryStorage)
	if !ok {
		t.Fatalf("storage is not *MemoryStorage")
	}

	_ = store.CreateBucket(ctx, "olb")

	putObjectLockFixture(t, svc, "olb")
	verifyObjectLockRoundTrip(t, svc, "olb")
}

// TestBucketObjectLock_PutDoesNotFallThroughToCreateBucket guards the original
// regression: ?object-lock PUT used to fall through to CreateBucket and return
// 409 BucketAlreadyOwnedByYou on an existing bucket.
func TestBucketObjectLock_PutDoesNotFallThroughToCreateBucket(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(), "")
	ctx := context.Background()

	store, ok := svc.storage.(*MemoryStorage)
	if !ok {
		t.Fatalf("storage is not *MemoryStorage")
	}

	_ = store.CreateBucket(ctx, "olb-regression")

	putObjectLockFixture(t, svc, "olb-regression")
}

// TestBucketObjectLock_GetUnconfigured verifies an unset bucket returns 404
// ObjectLockConfigurationNotFoundError rather than the generic stub.
func TestBucketObjectLock_GetUnconfigured(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(), "")
	ctx := context.Background()

	store, ok := svc.storage.(*MemoryStorage)
	if !ok {
		t.Fatalf("storage is not *MemoryStorage")
	}

	_ = store.CreateBucket(ctx, "olb-empty")

	req := httptest.NewRequest(http.MethodGet, "/olb-empty?object-lock", http.NoBody)
	req.SetPathValue("bucket", "olb-empty")

	w := httptest.NewRecorder()
	svc.handleBucketGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("GET status: got %d, want 404 (body=%s)", w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "ObjectLockConfigurationNotFoundError") {
		t.Fatalf("error code: got body=%s", w.Body.String())
	}
}

func objectLockFixtureBody() []byte {
	days := 365
	cfg := ObjectLockConfiguration{
		ObjectLockEnabled: "Enabled",
		Rule: &ObjectLockRule{
			DefaultRetention: &DefaultRetention{Mode: "COMPLIANCE", Days: &days},
		},
	}
	body, _ := xml.Marshal(cfg)

	return body
}

func putObjectLockFixture(t *testing.T, svc *Service, bucket string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"?object-lock", strings.NewReader(string(objectLockFixtureBody())))
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.handleBucketPut(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

func verifyObjectLockRoundTrip(t *testing.T, svc *Service, bucket string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"?object-lock", http.NoBody)
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.handleBucketGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	var got ObjectLockConfiguration
	if err := xml.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}

	if got.ObjectLockEnabled != "Enabled" {
		t.Fatalf("ObjectLockEnabled: got %q, want Enabled", got.ObjectLockEnabled)
	}

	if got.Rule == nil || got.Rule.DefaultRetention == nil {
		t.Fatalf("Rule.DefaultRetention: got %+v, want non-nil", got.Rule)
	}

	dr := got.Rule.DefaultRetention
	if dr.Mode != "COMPLIANCE" {
		t.Fatalf("Mode: got %q, want COMPLIANCE", dr.Mode)
	}

	if dr.Days == nil || *dr.Days != 365 {
		t.Fatalf("Days: got %v, want 365", dr.Days)
	}
}
