package lambda

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	testURLHost   = "lambda-url.localhost:4566"
	testOrigin    = "https://example.com"
	sigV4Header   = "AWS4-HMAC-SHA256 Credential=AKIATEST/20260918/us-east-1/lambda/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc"
	sigV4Query    = "X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AKIATEST%2F20260918%2Fus-east-1%2Flambda%2Faws4_request&X-Amz-Date=20260918T000000Z&X-Amz-Expires=300&X-Amz-SignedHeaders=host&X-Amz-Signature=abc"
	requestMethod = "Access-Control-Request-Method"
	requestHeader = "Access-Control-Request-Headers"
	allowedHeader = "content-type"
	exposedHeader = "x-request-id"
)

func TestLookupFunctionURL(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	created := createTestURL(t, storage)

	cfg, name, err := storage.LookupFunctionURL(context.Background(), strings.ToUpper(created.URLID))
	if err != nil || name != testURLFunction || cfg.URLID != created.URLID {
		t.Fatalf("LookupFunctionURL() = %+v, %q, %v", cfg, name, err)
	}

	cfg.Cors.AllowOrigins[0] = "https://tampered.example"

	again, _, _ := storage.LookupFunctionURL(context.Background(), created.URLID)
	if again.Cors.AllowOrigins[0] != testOrigin {
		t.Fatal("LookupFunctionURL must return a clone")
	}

	_, _, err = storage.LookupFunctionURL(context.Background(), "unknown")
	assertNotFound(t, err)
}

// sigV4Auth builds an Authorization header with a dummy signature.
func sigV4Auth(credential, signedHeaders string) string {
	return "AWS4-HMAC-SHA256 Credential=" + credential + ", SignedHeaders=" + signedHeaders + ", Signature=abc"
}

// signRequest adds a structurally valid SigV4 header the way an SDK would.
func signRequest(r *http.Request) {
	r.Header.Set(headerAmzDate, "20260918T000000Z")
	r.Header.Set("Authorization", sigV4Header)
}

func TestSigV4Identity(t *testing.T) {
	t.Parallel()

	const scope = "AKIATEST/20260918/us-east-1/lambda/aws4_request"

	cases := []struct {
		name   string
		header string
		query  string
		noDate bool
		wantOK bool
	}{
		{name: "header form", header: sigV4Header, wantOK: true},
		{name: "query form", query: sigV4Query, wantOK: true},
		{name: "unsigned"},
		{name: "algorithm only", header: "AWS4-HMAC-SHA256"},
		{name: "missing signature", header: "AWS4-HMAC-SHA256 Credential=" + scope + ", SignedHeaders=host"},
		{name: "wrong service scope", header: sigV4Auth("AKIATEST/20260918/us-east-1/s3/aws4_request", "host")},
		{name: "empty access key", header: sigV4Auth("/20260918/us-east-1/lambda/aws4_request", "host")},
		{name: "empty date scope", header: sigV4Auth("AKIATEST//us-east-1/lambda/aws4_request", "host")},
		{name: "malformed date scope", header: sigV4Auth("AKIATEST/2026-09-18/us-east-1/lambda/aws4_request", "host")},
		{name: "empty region scope", header: sigV4Auth("AKIATEST/20260918//lambda/aws4_request", "host")},
		{name: "missing request date", header: sigV4Header, noDate: true},
		{name: "signed header absent", header: sigV4Auth(scope, "host;x-custom")},
		{name: "signed headers without host", header: sigV4Auth(scope, "x-amz-date")},
		{name: "query missing expires", query: strings.Replace(sigV4Query, "&X-Amz-Expires=300", "", 1)},
		{name: "query missing signed headers", query: strings.Replace(sigV4Query, "&X-Amz-SignedHeaders=host", "", 1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://x."+testURLHost+"/?"+tc.query, http.NoBody)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}

			if tc.header != "" && !tc.noDate {
				r.Header.Set(headerAmzDate, "20260918T000000Z")
			}

			identity, ok := sigV4Identity(r, "111122223333")
			if ok != tc.wantOK {
				t.Fatalf("sigV4Identity() ok = %v, want %v", ok, tc.wantOK)
			}

			if ok && (identity.AccessKey != "AKIATEST" || identity.UserArn != "arn:aws:iam::111122223333:root") {
				t.Fatalf("identity = %+v", identity)
			}
		})
	}
}

func TestCORSPreflightMatching(t *testing.T) {
	t.Parallel()

	cors := &FunctionURLCORS{AllowOrigins: []string{testOrigin}, AllowMethods: []string{http.MethodGet, http.MethodPost}, AllowHeaders: []string{"Content-Type"}}

	cases := []struct {
		name, origin, method, headers string
		want                          bool
	}{
		{"allowed", testOrigin, http.MethodPost, allowedHeader, true},
		{"no requested headers", testOrigin, http.MethodGet, "", true},
		{"origin not allowed", "https://evil.example", http.MethodGet, "", false},
		{"method not allowed", testOrigin, http.MethodDelete, "", false},
		{"header not allowed", testOrigin, http.MethodGet, "authorization", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "http://x."+testURLHost+"/", http.NoBody)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set(requestMethod, tc.method)

			if tc.headers != "" {
				r.Header.Set(requestHeader, tc.headers)
			}

			if !isCORSPreflight(r) {
				t.Fatal("request must be recognised as a preflight")
			}

			if got := preflightAllowed(cors, r); got != tc.want {
				t.Fatalf("preflightAllowed() = %v, want %v", got, tc.want)
			}
		})
	}

	wildcard := &FunctionURLCORS{AllowOrigins: []string{"*"}, AllowMethods: []string{"*"}, AllowHeaders: []string{"*"}}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "http://x."+testURLHost+"/", http.NoBody)
	r.Header.Set("Origin", "https://anywhere.example")
	r.Header.Set(requestMethod, http.MethodDelete)
	r.Header.Set(requestHeader, "x-anything")

	if !preflightAllowed(wildcard, r) {
		t.Fatal("wildcards must allow any origin, method and header")
	}

	plain := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "http://x."+testURLHost+"/", http.NoBody)
	if isCORSPreflight(plain) {
		t.Fatal("OPTIONS without Origin and Access-Control-Request-Method is not a preflight")
	}
}

func TestCORSHeaders(t *testing.T) {
	t.Parallel()

	cors := &FunctionURLCORS{AllowCredentials: true, AllowOrigins: []string{testOrigin}, AllowMethods: []string{http.MethodGet, http.MethodPost}, AllowHeaders: []string{allowedHeader}, ExposeHeaders: []string{exposedHeader}, MaxAge: 3600}

	rec := httptest.NewRecorder()
	writeCORSPreflight(rec, cors, testOrigin)

	want := map[string]string{
		"Access-Control-Allow-Origin": testOrigin, "Access-Control-Allow-Methods": "GET,POST", "Access-Control-Allow-Headers": allowedHeader,
		"Access-Control-Expose-Headers": exposedHeader, "Access-Control-Max-Age": "3600", "Access-Control-Allow-Credentials": "true", "Vary": "Origin",
	}

	for key, value := range want {
		if rec.Header().Get(key) != value {
			t.Errorf("%s = %q, want %q", key, rec.Header().Get(key), value)
		}
	}

	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Errorf("preflight = %d %q, want 200 with an empty body", rec.Code, rec.Body.String())
	}

	// Ordinary responses keep the function's own CORS headers next to the configured ones.
	h := http.Header{}
	h.Set("Access-Control-Allow-Origin", "https://from-function")
	addCORSHeaders(h, cors, testOrigin)

	if got := h.Values("Access-Control-Allow-Origin"); len(got) != 2 {
		t.Errorf("Access-Control-Allow-Origin = %v, want both values", got)
	}

	if h.Get("Access-Control-Expose-Headers") != exposedHeader || h.Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("configured headers missing: %v", h)
	}

	wildcard := httptest.NewRecorder()
	writeCORSPreflight(wildcard, &FunctionURLCORS{AllowOrigins: []string{"*"}}, testOrigin)

	if wildcard.Header().Get("Access-Control-Allow-Origin") != "*" || wildcard.Header().Get("Vary") != "" {
		t.Errorf("wildcard origin = %v", wildcard.Header())
	}
}

// newFunctionURLService returns a Lambda service whose invoke endpoint is a
// stub that records invocations and answers with payload.
func newFunctionURLService(t *testing.T, payload string) (*Service, *MemoryStorage, *atomic.Int32) {
	t.Helper()

	var invocations atomic.Int32

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		invocations.Add(1)

		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(stub.Close)

	storage := newStorageWithFunction(t)
	svc := New(storage, defaultBaseURL)
	svc.invokeBaseURL = stub.URL

	return svc, storage, &invocations
}

func functionURLRequest(t *testing.T, cfg *FunctionURLConfig, method, path string) *http.Request {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), method, "http://"+cfg.URLID+"."+testURLHost+path, http.NoBody)
	r.RemoteAddr = "192.0.2.1:5555"

	return r
}

func TestHandleFunctionURL_InvokesAndMapsResponse(t *testing.T) {
	t.Parallel()

	svc, storage, invocations := newFunctionURLService(t, `{"statusCode":201,"headers":{"x-fn":"1"},"body":"hi"}`)
	cfg := createTestURL(t, storage)

	rec := httptest.NewRecorder()
	if !svc.HandleFunctionURL(rec, functionURLRequest(t, cfg, http.MethodGet, "/items"), cfg.URLID) {
		t.Fatal("HandleFunctionURL must claim a known url id")
	}

	if rec.Code != http.StatusCreated || rec.Body.String() != "hi" || rec.Header().Get("x-fn") != "1" || invocations.Load() != 1 {
		t.Fatalf("response %d %q %v (invocations %d)", rec.Code, rec.Body.String(), rec.Header(), invocations.Load())
	}

	if svc.HandleFunctionURL(httptest.NewRecorder(), functionURLRequest(t, cfg, http.MethodGet, "/"), "unknown") {
		t.Fatal("an unknown url id must not be claimed")
	}
}

func TestHandleFunctionURL_AWSIAMRequiresSignedRequests(t *testing.T) {
	t.Parallel()

	svc, storage, invocations := newFunctionURLService(t, `{"statusCode":200}`)
	cfg := createTestURL(t, storage)

	iam := authTypeIAMT
	if _, err := storage.UpdateFunctionURLConfig(context.Background(), testURLFunction, FunctionURLConfigUpdate{AuthType: &iam}); err != nil {
		t.Fatalf("UpdateFunctionURLConfig() error = %v", err)
	}

	rec := httptest.NewRecorder()
	svc.HandleFunctionURL(rec, functionURLRequest(t, cfg, http.MethodGet, "/"), cfg.URLID)

	if rec.Code != http.StatusForbidden || invocations.Load() != 0 || !strings.Contains(rec.Body.String(), "Forbidden") {
		t.Fatalf("unsigned: %d %q (invocations %d)", rec.Code, rec.Body.String(), invocations.Load())
	}

	signed := functionURLRequest(t, cfg, http.MethodGet, "/")
	signRequest(signed)

	rec = httptest.NewRecorder()
	svc.HandleFunctionURL(rec, signed, cfg.URLID)

	if rec.Code != http.StatusOK || invocations.Load() != 1 {
		t.Fatalf("signed: %d (invocations %d)", rec.Code, invocations.Load())
	}
}

func TestHandleFunctionURL_CORS(t *testing.T) {
	t.Parallel()

	svc, storage, invocations := newFunctionURLService(t, `{"statusCode":200,"headers":{"access-control-allow-origin":"https://from-function"},"body":"ok"}`)
	cfg := createTestURL(t, storage) // Cors allows https://example.com, GET/POST, content-type

	preflight := functionURLRequest(t, cfg, http.MethodOptions, "/")
	preflight.Header.Set("Origin", testOrigin)
	preflight.Header.Set(requestMethod, http.MethodPost)

	rec := httptest.NewRecorder()
	svc.HandleFunctionURL(rec, preflight, cfg.URLID)

	if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Methods") != "GET,POST" || invocations.Load() != 0 {
		t.Fatalf("preflight: %d %v (invocations %d)", rec.Code, rec.Header(), invocations.Load())
	}

	preflight.Header.Set(requestMethod, http.MethodDelete)

	rec = httptest.NewRecorder()
	svc.HandleFunctionURL(rec, preflight, cfg.URLID)

	if rec.Code != http.StatusForbidden || invocations.Load() != 0 {
		t.Fatalf("disallowed preflight: %d (invocations %d)", rec.Code, invocations.Load())
	}

	get := functionURLRequest(t, cfg, http.MethodGet, "/")
	get.Header.Set("Origin", testOrigin)

	rec = httptest.NewRecorder()
	svc.HandleFunctionURL(rec, get, cfg.URLID)

	if got := rec.Header().Values("Access-Control-Allow-Origin"); rec.Code != http.StatusOK || len(got) != 2 {
		t.Fatalf("ordinary response must carry configured and function CORS headers: %d %v", rec.Code, got)
	}

	var body map[string]any
	if json.Unmarshal(rec.Body.Bytes(), &body) == nil {
		t.Fatalf("body must be the function's plain string, got JSON %v", body)
	}
}

func TestHandleFunctionURL_FunctionErrorIs502(t *testing.T) {
	t.Parallel()

	storage := newStorageWithFunction(t)
	cfg := createTestURL(t, storage)

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Amz-Function-Error", "Unhandled")
		_, _ = w.Write([]byte(`{"errorMessage":"boom"}`))
	}))
	t.Cleanup(stub.Close)

	svc := New(storage, defaultBaseURL)
	svc.invokeBaseURL = stub.URL

	rec := httptest.NewRecorder()
	svc.HandleFunctionURL(rec, functionURLRequest(t, cfg, http.MethodGet, "/"), cfg.URLID)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("function error must map to 502, got %d", rec.Code)
	}
}
