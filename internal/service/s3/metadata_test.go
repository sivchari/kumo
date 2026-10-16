package s3

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	metaTestBucket = "meta-test-bucket"
	metaTestKey    = "meta-test-key"
)

// S3 returns user metadata in lower case.
func TestMetadataHeadersAreLowerCase(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	if err := store.CreateBucket(t.Context(), metaTestBucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	put := newObjectRequest(t, http.MethodPut, strings.NewReader("hello"))
	put.Header.Set("X-Amz-Meta-Report-Id", "report-1")

	w := httptest.NewRecorder()
	svc.PutObject(w, put)

	if w.Code != http.StatusOK {
		t.Fatalf("PutObject status: got %d, body=%s", w.Code, w.Body.String())
	}

	tests := []struct {
		name    string
		method  string
		rng     string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{name: "GetObject", method: http.MethodGet, handler: svc.GetObject},
		{name: "GetObject range", method: http.MethodGet, rng: "bytes=0-1", handler: svc.GetObject},
		{name: "HeadObject", method: http.MethodHead, handler: svc.HeadObject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := newObjectRequest(t, tt.method, http.NoBody)
			if tt.rng != "" {
				req.Header.Set("Range", tt.rng)
			}

			w := httptest.NewRecorder()
			tt.handler(w, req)

			//nolint:staticcheck // SA1008: S3 returns metadata headers in lower case.
			if got := w.Header()["x-amz-meta-report-id"]; len(got) != 1 || got[0] != "report-1" {
				t.Fatalf("x-amz-meta-report-id: got %v, want [report-1]; headers=%v", got, w.Header())
			}

			if _, ok := w.Header()["X-Amz-Meta-Report-Id"]; ok {
				t.Fatalf("unexpected canonicalised metadata header; headers=%v", w.Header())
			}
		})
	}
}

func newObjectRequest(t *testing.T, method string, body io.Reader) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, "/"+metaTestBucket+"/"+metaTestKey, body)
	req.SetPathValue("bucket", metaTestBucket)
	req.SetPathValue("key", metaTestKey)

	return req
}
