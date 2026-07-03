package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestParseByteRange enumerates the satisfiable / unsatisfiable
// shapes the GetObject Range path hands off to.
func TestParseByteRange(t *testing.T) {
	t.Parallel()

	const totalSize int64 = 1000

	cases := []struct {
		header string
		start  int64
		end    int64
		ok     bool
	}{
		{"bytes=0-99", 0, 99, true},
		{"bytes=100-199", 100, 199, true},
		{"bytes=900-9999", 900, 999, true},  // clamped at end-of-object
		{"bytes=-100", 900, 999, true},      // suffix
		{"bytes=500-", 500, 999, true},      // open-ended
		{"bytes=1000-", 0, 0, false},        // start past end
		{"bytes=200-100", 0, 0, false},      // inverted
		{"bytes=0-99,200-299", 0, 0, false}, // multi-range
		{"items=0-99", 0, 0, false},         // wrong unit
		{"", 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			start, end, ok := parseByteRange(tc.header, totalSize)
			if ok != tc.ok || (ok && (start != tc.start || end != tc.end)) {
				t.Fatalf("got (%d, %d, %v), want (%d, %d, %v)",
					start, end, ok, tc.start, tc.end, tc.ok)
			}
		})
	}
}

func TestParseByteRangeRejectsSuffixRangeForEmptyObject(t *testing.T) {
	t.Parallel()

	if start, end, ok := parseByteRange("bytes=-1", 0); ok {
		t.Fatalf("got (%d, %d, true), want unsatisfiable", start, end)
	}
}

// rangeCase pins one Range scenario for the HTTP-layer table test.
type rangeCase struct {
	name        string
	rangeHeader string
	wantStatus  int
	wantBody    string
	wantCR      string // expected Content-Range value, empty for 200
}

var rangeCases = []rangeCase{
	{"closed", "bytes=2-5", http.StatusPartialContent, "2345", "bytes 2-5/16"},
	{"suffix", "bytes=-3", http.StatusPartialContent, "def", "bytes 13-15/16"},
	{"open-ended", "bytes=10-", http.StatusPartialContent, "abcdef", "bytes 10-15/16"},
	{"clamped end", "bytes=10-99", http.StatusPartialContent, "abcdef", "bytes 10-15/16"},
	{"unsatisfiable", "bytes=99-", http.StatusRequestedRangeNotSatisfiable, "", ""},
	{"wrong unit falls through to 200", "items=0-3", http.StatusOK, "0123456789abcdef", ""},
	{"absent header returns full 200", "", http.StatusOK, "0123456789abcdef", ""},
}

// TestGetObject_RangeRoundTrip puts an object and then asks for a
// range of it through the HTTP handler. Confirms 206 + Content-Range +
// the right body slice.
func TestGetObject_RangeRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	const body = "0123456789abcdef" // 16 bytes

	if err := store.CreateBucket(context.Background(), "range-test"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	if _, err := store.PutObject(context.Background(), "range-test", "blob", strings.NewReader(body), nil); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	for _, tc := range rangeCases {
		t.Run(tc.name, func(t *testing.T) {
			runRangeCase(t, svc, tc)
		})
	}
}

// runRangeCase exercises one rangeCase against the GetObject handler.
func runRangeCase(t *testing.T, svc *Service, tc rangeCase) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/range-test/blob", http.NoBody)
	req.SetPathValue("bucket", "range-test")
	req.SetPathValue("key", "blob")

	if tc.rangeHeader != "" {
		req.Header.Set("Range", tc.rangeHeader)
	}

	w := httptest.NewRecorder()
	svc.GetObject(w, req)

	if w.Code != tc.wantStatus {
		t.Fatalf("status: got %d, want %d (body=%s)", w.Code, tc.wantStatus, w.Body.String())
	}

	if tc.wantStatus == http.StatusPartialContent {
		if w.Body.String() != tc.wantBody {
			t.Fatalf("body: got %q, want %q", w.Body.String(), tc.wantBody)
		}

		if got := w.Header().Get("Content-Range"); got != tc.wantCR {
			t.Fatalf("Content-Range: got %q, want %q", got, tc.wantCR)
		}
	}

	if tc.wantStatus == http.StatusOK && w.Body.String() != tc.wantBody {
		t.Fatalf("full body: got %q, want %q", w.Body.String(), tc.wantBody)
	}
}

// TestGetObject_AdvertisesAcceptRanges — every successful response
// signals byte-range support so well-behaved clients know they can
// resume / chunk.
func TestGetObject_AdvertisesAcceptRanges(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")

	_ = store.CreateBucket(context.Background(), "ar-test")
	_, _ = store.PutObject(context.Background(), "ar-test", "k",
		strings.NewReader("hello"), nil)

	req := httptest.NewRequest(http.MethodGet, "/ar-test/k", http.NoBody)
	req.SetPathValue("bucket", "ar-test")
	req.SetPathValue("key", "k")

	w := httptest.NewRecorder()
	svc.GetObject(w, req)

	if w.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("Accept-Ranges: got %q, want bytes", w.Header().Get("Accept-Ranges"))
	}

	got, _ := io.ReadAll(w.Result().Body)
	if string(got) != "hello" {
		t.Fatalf("body: got %q, want hello", got)
	}
}
