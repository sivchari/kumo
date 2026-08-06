package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- BucketNotification ---------------------------------------------

func TestBucketNotification_EventBridgeRoundTrip(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(), "")
	ctx := context.Background()

	store, ok := svc.storage.(*MemoryStorage)
	if !ok {
		t.Fatalf("storage is not *MemoryStorage")
	}

	_ = store.CreateBucket(ctx, "nb")

	putReq := httptest.NewRequest(
		http.MethodPut,
		"/nb?notification",
		strings.NewReader(`<NotificationConfiguration><EventBridgeConfiguration></EventBridgeConfiguration></NotificationConfiguration>`),
	)
	putReq.SetPathValue("bucket", "nb")

	putW := httptest.NewRecorder()
	svc.handleBucketPut(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("PUT status: got %d, want 200 (body=%s)", putW.Code, putW.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/nb?notification", http.NoBody)
	getReq.SetPathValue("bucket", "nb")

	getW := httptest.NewRecorder()
	svc.handleBucketGet(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("GET status: got %d, want 200 (body=%s)", getW.Code, getW.Body.String())
	}

	var got NotificationConfiguration
	if err := xml.Unmarshal(getW.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, getW.Body.String())
	}

	if got.EventBridgeConfig == nil {
		t.Fatalf("EventBridgeConfiguration was not returned: body=%s", getW.Body.String())
	}
}

// --- Object Restore --------------------------------------------------

func TestObjectRestore_FirstRequestReturns202(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(), "")
	ctx := context.Background()

	store, ok := svc.storage.(*MemoryStorage)
	if !ok {
		t.Fatalf("storage is not *MemoryStorage")
	}

	_ = store.CreateBucket(ctx, "rb")
	_, _ = store.PutObject(ctx, "rb", "k", strings.NewReader("x"), nil)

	body := `<RestoreRequest><Days>3</Days><Tier>Standard</Tier></RestoreRequest>`

	req := httptest.NewRequest(http.MethodPost, "/rb/k?restore", strings.NewReader(body))
	req.SetPathValue("bucket", "rb")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	svc.handleObjectPost(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("first restore: got %d, want 202 (body=%s)", w.Code, w.Body.String())
	}

	// Second restore on the same key → 200 OK (extending an existing one).
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/rb/k?restore", strings.NewReader(body))
	req2.SetPathValue("bucket", "rb")
	req2.SetPathValue("key", "k")
	svc.handleObjectPost(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second restore: got %d, want 200", w2.Code)
	}
}

func TestObjectRestore_404OnMissingObject(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(), "")
	ctx := context.Background()

	store, ok := svc.storage.(*MemoryStorage)
	if !ok {
		t.Fatalf("storage is not *MemoryStorage")
	}

	_ = store.CreateBucket(ctx, "rb")

	req := httptest.NewRequest(http.MethodPost, "/rb/missing?restore", strings.NewReader(`<RestoreRequest><Days>1</Days></RestoreRequest>`))
	req.SetPathValue("bucket", "rb")
	req.SetPathValue("key", "missing")

	w := httptest.NewRecorder()
	svc.handleObjectPost(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", w.Code)
	}
}
