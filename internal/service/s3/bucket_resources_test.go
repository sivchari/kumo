package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- BucketWebsite ---------------------------------------------------

func TestBucketWebsite_PutGetDelete(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(), "")
	ctx := context.Background()

	store, ok := svc.storage.(*MemoryStorage)
	if !ok {
		t.Fatalf("storage is not *MemoryStorage")
	}

	_ = store.CreateBucket(ctx, "wb")

	putWebsiteFixture(t, svc)
	verifyWebsiteRoundTrip(t, svc)
	verifyWebsiteDeleteThenGet404(t, svc)
}

func putWebsiteFixture(t *testing.T, svc *Service) {
	t.Helper()

	cfg := WebsiteConfiguration{
		IndexDocument: &WebsiteIndexDocument{Suffix: "index.html"},
		ErrorDocument: &WebsiteErrorDocument{Key: "error.html"},
	}
	body, _ := xml.Marshal(cfg)

	req := httptest.NewRequest(http.MethodPut, "/wb?website", strings.NewReader(string(body)))
	req.SetPathValue("bucket", "wb")

	w := httptest.NewRecorder()
	svc.handleBucketPut(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status: got %d (body=%s)", w.Code, w.Body.String())
	}
}

func verifyWebsiteRoundTrip(t *testing.T, svc *Service) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/wb?website", http.NoBody)
	req.SetPathValue("bucket", "wb")

	w := httptest.NewRecorder()
	svc.handleBucketGet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET status: got %d", w.Code)
	}

	var got WebsiteConfiguration
	if err := xml.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}

	if got.IndexDocument == nil || got.IndexDocument.Suffix != "index.html" {
		t.Fatalf("IndexDocument.Suffix: got %+v, want index.html", got.IndexDocument)
	}
}

func verifyWebsiteDeleteThenGet404(t *testing.T, svc *Service) {
	t.Helper()

	delReq := httptest.NewRequest(http.MethodDelete, "/wb?website", http.NoBody)
	delReq.SetPathValue("bucket", "wb")

	delW := httptest.NewRecorder()
	svc.handleBucketDelete(delW, delReq)

	if delW.Code != http.StatusNoContent {
		t.Fatalf("DELETE status: got %d, want 204", delW.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/wb?website", http.NoBody)
	getReq.SetPathValue("bucket", "wb")

	getW := httptest.NewRecorder()
	svc.handleBucketGet(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Fatalf("GET after DELETE status: got %d, want 404", getW.Code)
	}

	if !strings.Contains(getW.Body.String(), "NoSuchWebsiteConfiguration") {
		t.Fatalf("error code: got body=%s", getW.Body.String())
	}
}

// --- BucketLifecycle -------------------------------------------------

func putLifecycleFixture(t *testing.T, svc *Service) {
	t.Helper()

	cfg := LifecycleConfiguration{
		Rules: []LifecycleRule{
			{
				ID:         "expire-logs",
				Status:     "Enabled",
				Filter:     &LifecycleFilter{Prefix: "logs/"},
				Expiration: &LifecycleExpiration{Days: 30},
			},
		},
	}
	body, _ := xml.Marshal(cfg)

	req := httptest.NewRequest(http.MethodPut, "/lb?lifecycle", strings.NewReader(string(body)))
	req.SetPathValue("bucket", "lb")

	w := httptest.NewRecorder()
	svc.handleBucketPut(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status: got %d (body=%s)", w.Code, w.Body.String())
	}
}

func verifyLifecycleRoundTrip(t *testing.T, svc *Service) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/lb?lifecycle", http.NoBody)
	req.SetPathValue("bucket", "lb")

	w := httptest.NewRecorder()
	svc.handleBucketGet(w, req)

	var got LifecycleConfiguration
	if err := xml.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}

	if len(got.Rules) != 1 || got.Rules[0].ID != "expire-logs" {
		t.Fatalf("rules: got %+v", got.Rules)
	}

	if got.Rules[0].Expiration == nil || got.Rules[0].Expiration.Days != 30 {
		t.Fatalf("Expiration.Days: got %+v, want 30", got.Rules[0].Expiration)
	}
}

func TestBucketLifecycle_PutGetDelete(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage(), "")
	ctx := context.Background()

	store, ok := svc.storage.(*MemoryStorage)
	if !ok {
		t.Fatalf("storage is not *MemoryStorage")
	}

	_ = store.CreateBucket(ctx, "lb")

	putLifecycleFixture(t, svc)
	verifyLifecycleRoundTrip(t, svc)

	// GET on bucket without lifecycle now → 404 NoSuchLifecycleConfiguration
	_ = store.CreateBucket(ctx, "empty")

	emptyReq := httptest.NewRequest(http.MethodGet, "/empty?lifecycle", http.NoBody)
	emptyReq.SetPathValue("bucket", "empty")

	emptyW := httptest.NewRecorder()
	svc.handleBucketGet(emptyW, emptyReq)

	if emptyW.Code != http.StatusNotFound {
		t.Fatalf("GET on bucket without lifecycle: got %d, want 404", emptyW.Code)
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
