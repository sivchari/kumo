package s3

import (
	"context"
	"errors"
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

// TestMatchesAnyETag_QuoteNormalization confirms If-Match/If-None-Match
// tokens compare equal to a stored ETag whether or not either side is
// quoted, matching AWS S3's lenient comparison.
func TestMatchesAnyETag_QuoteNormalization(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		headerValue string
		etag        string
		want        bool
	}{
		{"both quoted, match", `"abc123"`, `"abc123"`, true},
		{"header unquoted, stored quoted", `abc123`, `"abc123"`, true},
		{"header quoted, comma list, second matches", `"x", "abc123"`, `"abc123"`, true},
		{"mismatch", `"xyz"`, `"abc123"`, false},
		{"wildcard always matches", "*", `"abc123"`, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := matchesAnyETag(tc.headerValue, tc.etag); got != tc.want {
				t.Fatalf("matchesAnyETag(%q, %q) = %v, want %v", tc.headerValue, tc.etag, got, tc.want)
			}
		})
	}
}

// TestParsePutCondition covers If-Match / If-None-Match header parsing
// for PutObject/CompleteMultipartUpload, including S3's restriction
// that If-None-Match only supports "*" on writes.
func TestParsePutCondition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		hdr    http.Header
		want   PutCondition
		wantOK bool
	}{
		{"no headers", http.Header{}, PutCondition{}, true},
		{"If-None-Match: *", http.Header{"If-None-Match": []string{"*"}}, PutCondition{IfNoneMatchAny: true}, true},
		{"If-None-Match with an ETag is not implemented", http.Header{"If-None-Match": []string{`"abc"`}}, PutCondition{}, false},
		{"If-Match carried through verbatim", http.Header{"If-Match": []string{`"abc123"`}}, PutCondition{IfMatch: `"abc123"`}, true},
		{"both headers, If-Match kept and If-None-Match:* recognized", http.Header{
			"If-Match":      []string{`"abc123"`},
			"If-None-Match": []string{"*"},
		}, PutCondition{IfMatch: `"abc123"`, IfNoneMatchAny: true}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parsePutCondition(tc.hdr)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}

			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestCheckPutCondition exercises checkPutCondition directly against a
// MemoryBucket, including RFC 9110 evaluation order (If-Match before
// If-None-Match) and the delete-marker-as-absent case.
//
//nolint:funlen // Table of independent t.Run subtests covering every precondition/state combination.
func TestCheckPutCondition(t *testing.T) {
	t.Parallel()

	newBucketWithObject := func() *MemoryBucket {
		return &MemoryBucket{
			Objects: map[string]*Object{
				"k": {Key: "k", ETag: testETag},
			},
		}
	}

	t.Run("no condition always passes", func(t *testing.T) {
		t.Parallel()

		if err := checkPutCondition(newBucketWithObject(), "k", PutCondition{}); err != nil {
			t.Fatalf("got %v, want nil", err)
		}
	})

	t.Run("If-None-Match:* on missing key passes", func(t *testing.T) {
		t.Parallel()

		b := &MemoryBucket{Objects: map[string]*Object{}}
		if err := checkPutCondition(b, "k", PutCondition{IfNoneMatchAny: true}); err != nil {
			t.Fatalf("got %v, want nil", err)
		}
	})

	t.Run("If-None-Match:* on existing key fails", func(t *testing.T) {
		t.Parallel()

		expectObjectErrorCode(t, checkPutCondition(newBucketWithObject(), "k", PutCondition{IfNoneMatchAny: true}), "PreconditionFailed")
	})

	t.Run("If-Match hit passes", func(t *testing.T) {
		t.Parallel()

		if err := checkPutCondition(newBucketWithObject(), "k", PutCondition{IfMatch: testETag}); err != nil {
			t.Fatalf("got %v, want nil", err)
		}
	})

	t.Run("If-Match miss fails", func(t *testing.T) {
		t.Parallel()

		expectObjectErrorCode(t, checkPutCondition(newBucketWithObject(), "k", PutCondition{IfMatch: `"never-matches"`}), "PreconditionFailed")
	})

	t.Run("If-Match on missing key is NoSuchKey", func(t *testing.T) {
		t.Parallel()

		b := &MemoryBucket{Objects: map[string]*Object{}}
		expectObjectErrorCode(t, checkPutCondition(b, "k", PutCondition{IfMatch: testETag}), "NoSuchKey")
	})

	t.Run("If-Match evaluated before If-None-Match", func(t *testing.T) {
		t.Parallel()

		// Both conditions target the same existing key: If-Match
		// mismatches (RFC 9110 order says it's evaluated first), so
		// that's the failure reported even though If-None-Match:*
		// would also fail here.
		err := checkPutCondition(newBucketWithObject(), "k", PutCondition{IfMatch: `"never-matches"`, IfNoneMatchAny: true})
		expectObjectErrorCode(t, err, "PreconditionFailed")
	})

	t.Run("delete marker is treated as absent", func(t *testing.T) {
		t.Parallel()

		b := &MemoryBucket{Objects: map[string]*Object{
			"k": {Key: "k", IsDeleteMarker: true},
		}}

		if err := checkPutCondition(b, "k", PutCondition{IfNoneMatchAny: true}); err != nil {
			t.Fatalf("If-None-Match:* over a delete marker: got %v, want nil", err)
		}

		expectObjectErrorCode(t, checkPutCondition(b, "k", PutCondition{IfMatch: testETag}), "NoSuchKey")
	})
}

// expectObjectErrorCode asserts err is an *ObjectError with the given Code.
func expectObjectErrorCode(t *testing.T, err error, code string) {
	t.Helper()

	var objErr *ObjectError
	if !errors.As(err, &objErr) {
		t.Fatalf("got err %v (%T), want *ObjectError code %s", err, err, code)
	}

	if objErr.Code != code {
		t.Fatalf("got ObjectError code %s, want %s", objErr.Code, code)
	}
}

// TestPutObject_ConditionalRequests confirms the HTTP-layer wiring for
// PutObject's If-Match / If-None-Match preconditions.
//
//nolint:funlen // Table-driven test covering every precondition/state combination over HTTP.
func TestPutObject_ConditionalRequests(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		seed       bool // seed an initial object at "k" before the conditional PUT
		header     string
		value      string // "seeded"/"seeded-unquoted" substitute the seeded object's ETag
		wantStatus int
	}{
		{"no headers, no existing object", false, "", "", http.StatusOK},
		{"no headers, existing object", true, "", "", http.StatusOK},
		{"If-None-Match:* on missing key succeeds", false, "If-None-Match", "*", http.StatusOK},
		{"If-None-Match:* on existing key fails", true, "If-None-Match", "*", http.StatusPreconditionFailed},
		{"If-None-Match non-* is not implemented", false, "If-None-Match", `"abc"`, http.StatusNotImplemented},
		{"If-Match hit succeeds", true, "If-Match", "seeded", http.StatusOK},
		{"If-Match hit unquoted succeeds", true, "If-Match", "seeded-unquoted", http.StatusOK},
		{"If-Match miss fails", true, "If-Match", `"never-matches"`, http.StatusPreconditionFailed},
		{"If-Match on missing key is NoSuchKey", false, "If-Match", `"whatever"`, http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := NewMemoryStorage()
			svc := New(store, "")
			ctx := context.Background()

			_ = store.CreateBucket(ctx, "pb")

			var seededETag string

			if tc.seed {
				obj, err := store.PutObject(ctx, "pb", "k", strings.NewReader("old"), nil)
				if err != nil {
					t.Fatalf("seed PutObject: %v", err)
				}

				seededETag = obj.ETag
			}

			req := httptest.NewRequest(http.MethodPut, "/pb/k", strings.NewReader("new"))
			req.SetPathValue("bucket", "pb")
			req.SetPathValue("key", "k")

			switch tc.value {
			case "seeded":
				req.Header.Set(tc.header, seededETag)
			case "seeded-unquoted":
				req.Header.Set(tc.header, strings.Trim(seededETag, `"`))
			default:
				if tc.header != "" {
					req.Header.Set(tc.header, tc.value)
				}
			}

			w := httptest.NewRecorder()
			svc.PutObject(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d (body=%s)", w.Code, tc.wantStatus, w.Body.String())
			}

			if tc.wantStatus == http.StatusOK && w.Header().Get("ETag") == "" {
				t.Fatalf("expected an ETag header on success")
			}
		})
	}
}

// TestPutObject_ConditionalRequests_DeleteMarker confirms a key whose
// current version is a delete marker (versioning enabled, then deleted)
// is treated as absent: If-None-Match:* must succeed just as it would
// for a key that never existed.
func TestPutObject_ConditionalRequests_DeleteMarker(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	_ = store.CreateBucket(ctx, "db")
	_ = store.PutBucketVersioning(ctx, "db", VersioningEnabled)

	if _, err := store.PutObject(ctx, "db", "k", strings.NewReader("v1"), nil); err != nil {
		t.Fatalf("seed PutObject: %v", err)
	}

	if _, err := store.DeleteObject(ctx, "db", "k"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/db/k", strings.NewReader("v2"))
	req.SetPathValue("bucket", "db")
	req.SetPathValue("key", "k")
	req.Header.Set("If-None-Match", "*")

	w := httptest.NewRecorder()
	svc.PutObject(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("If-None-Match:* after a versioned delete: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

// TestCompleteMultipartUpload_ConditionalRequest confirms the HTTP-layer
// wiring for CompleteMultipartUpload's If-Match / If-None-Match
// preconditions, mirroring PutObject's.
func TestCompleteMultipartUpload_ConditionalRequest(t *testing.T) {
	t.Parallel()

	const bucket = "cmu-cond-http"

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	_ = store.CreateBucket(ctx, bucket)

	if _, err := store.PutObject(ctx, bucket, "k", strings.NewReader("existing"), nil); err != nil {
		t.Fatalf("seed PutObject: %v", err)
	}

	upload, err := store.CreateMultipartUpload(ctx, bucket, "k", nil)
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	part, err := store.UploadPart(ctx, bucket, "k", upload.UploadID, 1, strings.NewReader("new body"))
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}

	completeBody := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>` + part.ETag + `</ETag></Part></CompleteMultipartUpload>`

	req := httptest.NewRequest(http.MethodPost, "/"+bucket+"/k?uploadId="+upload.UploadID, strings.NewReader(completeBody))
	req.SetPathValue("bucket", bucket)
	req.SetPathValue("key", "k")
	req.Header.Set("If-None-Match", "*")

	w := httptest.NewRecorder()
	svc.CompleteMultipartUpload(w, req)

	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("status: got %d, want %d (body=%s)", w.Code, http.StatusPreconditionFailed, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "<Code>PreconditionFailed</Code>") {
		t.Fatalf("expected PreconditionFailed error body, got %s", w.Body.String())
	}

	// The failed conditional complete must leave the in-progress upload
	// alone so the caller can retry (e.g. after resolving the conflict).
	if _, stillThere := store.Buckets[bucket].MultipartUploads[upload.UploadID]; !stillThere {
		t.Fatalf("expected in-progress upload to survive a failed conditional complete")
	}

	// Retrying unconditionally succeeds and consumes the upload.
	req2 := httptest.NewRequest(http.MethodPost, "/"+bucket+"/k?uploadId="+upload.UploadID, strings.NewReader(completeBody))
	req2.SetPathValue("bucket", bucket)
	req2.SetPathValue("key", "k")

	w2 := httptest.NewRecorder()
	svc.CompleteMultipartUpload(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("unconditional retry status: got %d, want %d (body=%s)", w2.Code, http.StatusOK, w2.Body.String())
	}

	if _, stillThere := store.Buckets[bucket].MultipartUploads[upload.UploadID]; stillThere {
		t.Fatalf("expected upload to be removed after a successful complete")
	}
}
