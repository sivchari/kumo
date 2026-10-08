package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// authHeader builds the Authorization header an AWS SDK sends for the given
// SigV4 signing name.
func authHeader(signingName string) string {
	return "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20261007/us-east-1/" + signingName +
		"/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc"
}

// newScopeTestRouter builds a router with two services registering the
// identical tagging pattern under different signing names, plus a legacy
// path-prefixed route. The returned pointer records which handler ran.
func newScopeTestRouter() (*Router, *string) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRouter(logger)

	called := new(string)
	record := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			*called = name

			w.WriteHeader(http.StatusOK)
		}
	}

	r.ScopedRouter("scheduler").HandleFunc(http.MethodGet, "/tags/{arn}", record("scheduler-tags"))
	r.ScopedRouter("kafka").HandleFunc(http.MethodGet, "/tags/{arn}", record("kafka-tags"))
	r.ScopedRouter("iam").HandleFunc(http.MethodGet, "/{$}", record("iam-root"))
	r.ScopedRouter("lambda").HandleFunc(http.MethodGet,
		"/_runtime/{functionName}/2018-06-01/runtime/invocation/next", record("lambda-runtime"))
	r.ScopedRouter("ses").HandleFunc(http.MethodGet, "/kumo/ses/v2/sent-emails", record("ses-debug"))
	r.Handle(http.MethodGet, "/scheduler/schedules/{name}", record("scheduler-legacy"))

	return r, called
}

var scopeDispatchCases = []struct {
	name       string
	auth       string
	path       string
	want       string
	wantStatus int
}{
	{
		name:       "scope picks scheduler on shared path",
		auth:       authHeader("scheduler"),
		path:       "/tags/arn%3Aaws%3Ascheduler%3A...",
		want:       "scheduler-tags",
		wantStatus: http.StatusOK,
	},
	{
		name:       "scope picks kafka on shared path",
		auth:       authHeader("kafka"),
		path:       "/tags/arn%3Aaws%3Akafka%3A...",
		want:       "kafka-tags",
		wantStatus: http.StatusOK,
	},
	{
		name:       "signed request with legacy prefix falls back to path routing",
		auth:       authHeader("scheduler"),
		path:       "/scheduler/schedules/test",
		want:       "scheduler-legacy",
		wantStatus: http.StatusOK,
	},
	{
		name:       "unknown signing name falls back to path routing",
		auth:       authHeader("s3"),
		path:       "/tags/arn",
		want:       "",
		wantStatus: http.StatusNotFound,
	},
	{
		name:       "unsigned request falls back to path routing",
		auth:       "",
		path:       "/tags/arn",
		want:       "",
		wantStatus: http.StatusNotFound,
	},
	{
		name:       "unsigned lambda runtime path is path-routed",
		auth:       "",
		path:       "/_runtime/fn/2018-06-01/runtime/invocation/next",
		want:       "lambda-runtime",
		wantStatus: http.StatusOK,
	},
	{
		name:       "unsigned kumo debug path is path-routed",
		auth:       "",
		path:       "/kumo/ses/v2/sent-emails",
		want:       "ses-debug",
		wantStatus: http.StatusOK,
	},
	{
		name:       "signed root request is served from its scope router",
		auth:       authHeader("iam"),
		path:       "/",
		want:       "iam-root",
		wantStatus: http.StatusOK,
	},
}

// TestRouter_ScopeDispatch verifies that signed requests are routed by the
// SigV4 credential scope: services registering the identical pattern (the
// shared tagging surface GET /tags/{arn}) are told apart by signing name,
// with no kumo path prefix in the URL.
func TestRouter_ScopeDispatch(t *testing.T) {
	t.Parallel()

	r, called := newScopeTestRouter()

	for _, tc := range scopeDispatchCases {
		t.Run(tc.name, func(t *testing.T) {
			*called = ""

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, http.NoBody)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if *called != tc.want {
				t.Fatalf("path=%s: got handler %q, want %q (status=%d)", tc.path, *called, tc.want, rec.Code)
			}

			if rec.Code != tc.wantStatus {
				t.Fatalf("path=%s: got status %d, want %d", tc.path, rec.Code, tc.wantStatus)
			}
		})
	}
}

// TestRouter_ScopeDispatch_SharedSigningName verifies that two services
// sharing one signing name (API Gateway v1 and v2 both sign as
// "apigateway") coexist in a single scope router.
func TestRouter_ScopeDispatch_SharedSigningName(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRouter(logger)

	called := ""
	record := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			called = name

			w.WriteHeader(http.StatusOK)
		}
	}

	r.ScopedRouter("apigateway").HandleFunc(http.MethodPost, "/restapis", record("v1"))
	r.ScopedRouter("apigateway").HandleFunc(http.MethodPost, "/v2/apis", record("v2"))

	for path, want := range map[string]string{"/restapis": "v1", "/v2/apis": "v2"} {
		called = ""

		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, http.NoBody)
		req.Header.Set("Authorization", authHeader("apigateway"))

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if called != want {
			t.Fatalf("path=%s: got handler %q, want %q (status=%d)", path, called, want, rec.Code)
		}
	}
}
