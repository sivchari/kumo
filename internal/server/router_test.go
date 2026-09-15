package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// resultBucketPut marks that the S3-style wildcard bucket route handled the
// request, used by the boundary-matching test cases below.
const resultBucketPut = "bucket-put"

// TestRouter_PrefixMatchRespectsBoundary regression-tests a bug where
// `extractRoutePrefix` and `Router.ServeHTTP` matched prefixes by raw
// string-prefix, so a path like `/kumo-audit-bad-bucket` was routed to
// the `/kumo` prefix router (which only handles `/_kumo/*` and
// similar), shadowing the S3 wildcard route `PUT /{bucket}` and
// returning 404. The fix requires the prefix to be followed by `/` or
// end-of-string.
func TestRouter_PrefixMatchRespectsBoundary(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRouter(logger)

	called := ""

	// `/kumo` is a registered prefix (used by /_kumo/health etc.).
	r.Handle(http.MethodGet, "/kumo/health", func(w http.ResponseWriter, _ *http.Request) {
		called = "kumo-health"

		w.WriteHeader(http.StatusOK)
	})

	// `/{bucket}` is the S3-style wildcard route. With the bug, a
	// PUT to `/kumo-audit-bad-bucket` would be sent to the /kumo
	// prefix router (no matching pattern) → 404.
	r.Handle(http.MethodPut, "/{bucket}", func(w http.ResponseWriter, _ *http.Request) {
		called = resultBucketPut

		w.WriteHeader(http.StatusOK)
	})

	// A bucket whose name *equals* an admin prefix (e.g. PUT /kumo) is
	// an unavoidable shadow until kumo grows a Host- or scheme-based
	// admin-route discriminator. The cases below cover only the
	// previously-broken substring case that the fix resolves.
	cases := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{"prefix exact", http.MethodGet, "/kumo/health", "kumo-health"},
		{"bucket name shares prefix substring", http.MethodPut, "/kumo-audit-bad-bucket", resultBucketPut},
		{"bucket name with longer admin prefix substring", http.MethodPut, "/lambda-deploy-bucket", resultBucketPut},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = ""

			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, http.NoBody)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if called != tc.want {
				t.Fatalf("path=%s: got handler %q, want %q (status=%d)",
					tc.path, called, tc.want, rec.Code)
			}
		})
	}
}

// TestExtractRoutePrefix_BoundaryGuard makes sure pattern registration
// itself doesn't classify a wildcard like `/{bucket}` as a `/kumo`
// prefixed route.
func TestExtractRoutePrefix_BoundaryGuard(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern string
		want    string
	}{
		{"/kumo/health", "/kumo"},
		{"/lambda/2015-03-31/functions", "/lambda"},
		{"/{bucket}", ""},
		{"/{bucket}/{key...}", ""},
		{"/kumosomething", ""}, // no slash boundary → not a prefix
	}

	for _, tc := range cases {
		t.Run(tc.pattern, func(t *testing.T) {
			got := extractRoutePrefix(tc.pattern)
			if got != tc.want {
				t.Errorf("extractRoutePrefix(%q) = %q, want %q", tc.pattern, got, tc.want)
			}
		})
	}
}
