package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	testETag = `"abc123"`
	testTime = "Mon, 01 Jan 2024 12:00:00 GMT"
)

func parseTestTime(t *testing.T) time.Time {
	t.Helper()

	parsed, err := http.ParseTime(testTime)
	if err != nil {
		t.Fatalf("parse test time: %v", err)
	}

	return parsed
}

// TestEvalGetObjectPreconditions covers the RFC 9110 §13.1 evaluation
// order for GET/HEAD precondition headers.
func TestEvalGetObjectPreconditions(t *testing.T) {
	t.Parallel()

	lastModified := parseTestTime(t)
	earlier := lastModified.Add(-1 * time.Hour).UTC().Format(timeFormatHTTP)
	later := lastModified.Add(time.Hour).UTC().Format(timeFormatHTTP)

	cases := []struct {
		name string
		hdr  http.Header
		want preconditionResult
	}{
		{"empty", http.Header{}, preconditionPass},
		{"if-match hit", http.Header{"If-Match": []string{testETag}}, preconditionPass},
		{"if-match miss", http.Header{"If-Match": []string{`"different"`}}, preconditionFailed},
		{"if-match wildcard", http.Header{"If-Match": []string{"*"}}, preconditionPass},
		{"if-none-match hit", http.Header{"If-None-Match": []string{testETag}}, preconditionNotModified},
		{"if-none-match miss", http.Header{"If-None-Match": []string{`"different"`}}, preconditionPass},
		{"if-modified-since older", http.Header{"If-Modified-Since": []string{earlier}}, preconditionPass},
		{"if-modified-since same/newer", http.Header{"If-Modified-Since": []string{later}}, preconditionNotModified},
		{"if-unmodified-since older → fail", http.Header{"If-Unmodified-Since": []string{earlier}}, preconditionFailed},
		{"if-unmodified-since newer → pass", http.Header{"If-Unmodified-Since": []string{later}}, preconditionPass},
		{"if-match wins over if-unmodified-since", http.Header{
			"If-Match":            []string{testETag},
			"If-Unmodified-Since": []string{earlier},
		}, preconditionPass},
		{"if-none-match wins over if-modified-since", http.Header{
			"If-None-Match":     []string{testETag},
			"If-Modified-Since": []string{earlier},
		}, preconditionNotModified},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evalGetObjectPreconditions(tc.hdr, testETag, lastModified)
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEvalCopySourcePreconditions(t *testing.T) {
	t.Parallel()

	lastModified := parseTestTime(t)
	earlier := lastModified.Add(-1 * time.Hour).UTC().Format(timeFormatHTTP)

	cases := []struct {
		name string
		hdr  http.Header
		want bool
	}{
		{"empty", http.Header{}, true},
		{"copy-source-if-match hit", http.Header{"X-Amz-Copy-Source-If-Match": []string{testETag}}, true},
		{"copy-source-if-match miss", http.Header{"X-Amz-Copy-Source-If-Match": []string{`"x"`}}, false},
		{"copy-source-if-none-match hit", http.Header{"X-Amz-Copy-Source-If-None-Match": []string{testETag}}, false},
		{"copy-source-if-unmodified-since older", http.Header{"X-Amz-Copy-Source-If-Unmodified-Since": []string{earlier}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evalCopySourcePreconditions(tc.hdr, testETag, lastModified)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestGetObject_ConditionalRequests confirms the HTTP-layer wiring:
// 304 on If-None-Match hit, 412 on If-Match miss, normal 200 otherwise.
func TestGetObject_ConditionalRequests(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	_ = store.CreateBucket(ctx, "cb")
	obj, _ := store.PutObject(ctx, "cb", "k", strings.NewReader("hello"), nil)

	cases := []struct {
		name       string
		header     string
		value      string
		wantStatus int
	}{
		{"plain GET", "", "", http.StatusOK},
		{"If-None-Match hit → 304", "If-None-Match", obj.ETag, http.StatusNotModified},
		{"If-Match miss → 412", "If-Match", `"never-matches"`, http.StatusPreconditionFailed},
		{"If-Match hit → 200", "If-Match", obj.ETag, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/cb/k", http.NoBody)
			req.SetPathValue("bucket", "cb")
			req.SetPathValue("key", "k")

			if tc.header != "" {
				req.Header.Set(tc.header, tc.value)
			}

			w := httptest.NewRecorder()
			svc.GetObject(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d (body=%s)", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

// TestGetObject_ResponseHeaderOverrides confirms response-* query
// parameters override the default Content-Type / Content-Disposition
// etc. on the GetObject response.
func TestGetObject_ResponseHeaderOverrides(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	_ = store.CreateBucket(ctx, "rb")
	_, _ = store.PutObject(ctx, "rb", "k", strings.NewReader("body"), nil)

	const (
		wantCT = "text/csv"
		wantCD = `attachment; filename="report.csv"`
		wantCC = "no-cache"
	)

	url := "/rb/k?response-content-type=" + wantCT +
		"&response-content-disposition=" + httpQueryEscape(wantCD) +
		"&response-cache-control=" + wantCC

	req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
	req.SetPathValue("bucket", "rb")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	svc.GetObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	if got := w.Header().Get("Content-Type"); got != wantCT {
		t.Fatalf("Content-Type: got %q, want %q", got, wantCT)
	}

	if got := w.Header().Get("Content-Disposition"); got != wantCD {
		t.Fatalf("Content-Disposition: got %q, want %q", got, wantCD)
	}

	if got := w.Header().Get("Cache-Control"); got != wantCC {
		t.Fatalf("Cache-Control: got %q, want %q", got, wantCC)
	}
}

// httpQueryEscape locally avoids the net/url import jolt — we only
// need it in one spot for a header value with quotes/spaces.
func httpQueryEscape(s string) string {
	out := strings.NewReplacer(
		" ", "%20",
		`"`, "%22",
		`;`, "%3B",
	).Replace(s)

	return out
}
