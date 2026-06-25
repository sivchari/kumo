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

// Markers for which route handled a request in the JWKS/S3 collision test.
const (
	resultJWKS   = "jwks"
	resultS3Get  = "s3-get"
	resultS3Head = "s3-head"
)

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
		{"bucket name with longer admin prefix substring", http.MethodPut, "/tables-deploy-bucket", resultBucketPut},
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

// TestRouter_JWKSRouteNotShadowedByS3 guards the Cognito JWKS endpoint
// `GET /{userPoolId}/.well-known/jwks.json`. Its pattern formally collides
// with S3's wildcard routes in the shared mux: GET implies HEAD, so the JWKS
// GET overlaps S3's explicit `HEAD /{bucket}/{key...}` with neither pattern
// more specific, which made http.ServeMux panic at registration. The router
// puts JWKS on a dedicated mux. This test registers the exact conflicting
// trio and asserts: no registration panic, JWKS wins on its path (with the
// userPoolId path value intact), and arbitrary `{bucket}/{key}` still reaches
// S3 for both GET and HEAD.
func TestRouter_JWKSRouteNotShadowedByS3(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRouter(logger)

	called := ""
	gotUserPoolID := ""

	// Registering these three in one mux is what used to panic.
	r.Handle(http.MethodGet, "/{userPoolId}/.well-known/jwks.json", func(w http.ResponseWriter, req *http.Request) {
		called = resultJWKS
		gotUserPoolID = req.PathValue("userPoolId")

		w.WriteHeader(http.StatusOK)
	})

	r.Handle(http.MethodGet, "/{bucket}/{key...}", func(w http.ResponseWriter, _ *http.Request) {
		called = resultS3Get

		w.WriteHeader(http.StatusOK)
	})

	r.Handle(http.MethodHead, "/{bucket}/{key...}", func(w http.ResponseWriter, _ *http.Request) {
		called = resultS3Head

		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name           string
		method         string
		path           string
		want           string
		wantUserPoolID string
	}{
		{"jwks GET", http.MethodGet, "/us-east-1_abc123def/.well-known/jwks.json", resultJWKS, "us-east-1_abc123def"},
		{"s3 object GET", http.MethodGet, "/mybucket/some/key.txt", resultS3Get, ""},
		{"s3 object HEAD", http.MethodHead, "/mybucket/some/key.txt", resultS3Head, ""},
		{"s3 deep key GET", http.MethodGet, "/mybucket/a/b/c/d.json", resultS3Get, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = ""
			gotUserPoolID = ""

			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, http.NoBody)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if called != tc.want {
				t.Fatalf("%s %s: got handler %q, want %q (status=%d)", tc.method, tc.path, called, tc.want, rec.Code)
			}

			if gotUserPoolID != tc.wantUserPoolID {
				t.Fatalf("%s %s: got userPoolId %q, want %q", tc.method, tc.path, gotUserPoolID, tc.wantUserPoolID)
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
		{"/tables/{bucket}", "/tables"},
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
