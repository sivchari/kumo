package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
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
	r.Handle("GET", "/kumo/health", func(w http.ResponseWriter, _ *http.Request) {
		called = "kumo-health"

		w.WriteHeader(http.StatusOK)
	})

	// `/{bucket}` is the S3-style wildcard route. With the bug, a
	// PUT to `/kumo-audit-bad-bucket` would be sent to the /kumo
	// prefix router (no matching pattern) → 404.
	r.Handle("PUT", "/{bucket}", func(w http.ResponseWriter, _ *http.Request) {
		called = "bucket-put"

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
		{"prefix exact", "GET", "/kumo/health", "kumo-health"},
		{"bucket name shares prefix substring", "PUT", "/kumo-audit-bad-bucket", "bucket-put"},
		{"bucket name with longer admin prefix substring", "PUT", "/lambda-deploy-bucket", "bucket-put"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = ""

			req := httptest.NewRequest(tc.method, tc.path, http.NoBody)
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
	r.Handle("GET", "/{userPoolId}/.well-known/jwks.json", func(w http.ResponseWriter, req *http.Request) {
		called = "jwks"
		gotUserPoolID = req.PathValue("userPoolId")

		w.WriteHeader(http.StatusOK)
	})

	r.Handle("GET", "/{bucket}/{key...}", func(w http.ResponseWriter, _ *http.Request) {
		called = "s3-get"

		w.WriteHeader(http.StatusOK)
	})

	r.Handle("HEAD", "/{bucket}/{key...}", func(w http.ResponseWriter, _ *http.Request) {
		called = "s3-head"

		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name           string
		method         string
		path           string
		want           string
		wantUserPoolID string
	}{
		{"jwks GET", "GET", "/us-east-1_abc123def/.well-known/jwks.json", "jwks", "us-east-1_abc123def"},
		{"s3 object GET", "GET", "/mybucket/some/key.txt", "s3-get", ""},
		{"s3 object HEAD", "HEAD", "/mybucket/some/key.txt", "s3-head", ""},
		{"s3 deep key GET", "GET", "/mybucket/a/b/c/d.json", "s3-get", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = ""
			gotUserPoolID = ""

			req := httptest.NewRequest(tc.method, tc.path, http.NoBody)
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

// TestRouter_LocalStackHealth verifies the LocalStack-compatible health endpoint
// reports every registered service as "available" so LocalStack tooling (e.g.
// the example verify.sh, which curl -sf's /_localstack/health) works
// against kumo unchanged.
func TestRouter_LocalStackHealth(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRouter(logger)

	svcs := []string{"cognito-idp", "dynamodb", "lambda"}
	r.SetLocalStackHealth(svcs, "9.9.9")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_localstack/health", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}

	var resp struct {
		Services map[string]string `json:"services"`
		Edition  string            `json:"edition"`
		Version  string            `json:"version"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal body %q: %v", rec.Body.String(), err)
	}

	for _, svc := range svcs {
		if resp.Services[svc] != localStackServiceStatus {
			t.Errorf("services[%q]: got %q, want %q", svc, resp.Services[svc], localStackServiceStatus)
		}
	}

	if resp.Edition != localStackEdition {
		t.Errorf("edition: got %q, want %q", resp.Edition, localStackEdition)
	}

	if resp.Version != "9.9.9" {
		t.Errorf("version: got %q, want 9.9.9", resp.Version)
	}
}

// TestRouter_LocalStackHealthFallback covers a Router with no service list wired
// in (e.g. built directly in a test): the endpoint must still answer 200 with
// valid JSON rather than an empty body.
func TestRouter_LocalStackHealthFallback(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRouter(logger)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_localstack/health", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}

	var resp struct {
		Services map[string]string `json:"services"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal fallback body %q: %v", rec.Body.String(), err)
	}

	if len(resp.Services) != 0 {
		t.Errorf("services: got %v, want empty", resp.Services)
	}
}

// TestRouter_ExecuteAPIPathStyle verifies the LocalStack path-style invoke URL
// /_aws/execute-api/{apiId}/{stage}/{path} reaches the execute-api dispatch with
// the api id split off and the remainder passed as the "/{stage}/{path}" invoke
// path. The e2e example scripts use this form.
func TestRouter_ExecuteAPIPathStyle(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRouter(logger)

	gotAPIID := ""
	gotInvokePath := ""

	r.AddExecuteAPIHandler(func(w http.ResponseWriter, _ *http.Request, apiID, invokePath string) bool {
		gotAPIID = apiID
		gotInvokePath = invokePath

		w.WriteHeader(http.StatusOK)

		return true
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_aws/execute-api/abc123-0/local/programs/CARD_MANAGEMENT", http.NoBody)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}

	if gotAPIID != "abc123-0" {
		t.Errorf("apiID: got %q, want abc123-0", gotAPIID)
	}

	if gotInvokePath != "/local/programs/CARD_MANAGEMENT" {
		t.Errorf("invokePath: got %q, want /local/programs/CARD_MANAGEMENT", gotInvokePath)
	}
}

// TestRouter_ExecuteAPIPathStyleUnowned asserts that when no registered handler
// owns the api id, the path-style invoke answers 403 (real API Gateway behavior),
// mirroring the virtual-hosted branch.
func TestRouter_ExecuteAPIPathStyleUnowned(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRouter(logger)

	r.AddExecuteAPIHandler(func(http.ResponseWriter, *http.Request, string, string) bool { return false })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_aws/execute-api/zzz/local/x", http.NoBody)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want 403", rec.Code)
	}
}
