package cloudfront

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testLambdaURLOrigin  = "abc123.lambda-url.us-east-1.on.aws"
	testExecuteAPIOrigin = "api123.execute-api.us-east-1.amazonaws.com"
	backendBody          = "served-by-local-backend"
)

func TestKumoHostedOrigin(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		testLambdaURLOrigin:                      true,
		"abc123.lambda-url.localhost":            true,
		"abc123.lambda-url.localhost:4566":       true,
		"ABC123.Lambda-URL.us-east-1.on.aws":     true,
		testExecuteAPIOrigin:                     true,
		"api123.execute-api.localhost":           true,
		"abc123.lambda-url.us-east-1.on.aws:443": true,
		"example.com":                            false,
		"app.lambda-url.example.com":             false,
		"api.execute-api.example.com":            false,
		"abc123.lambda-url.on.aws":               false,
		"other-bucket.s3.amazonaws.com":          false,
		"lambda-url.example.com":                 false,
		".lambda-url.us-east-1.on.aws":           false,
		"a.b.lambda-url.us-east-1.on.aws":        false,
		"abc123.lambda-url.":                     false,
		"127.0.0.1:8080":                         false,
		"":                                       false,
	}

	for host, want := range cases {
		if got := kumoHostedOrigin(host); got != want {
			t.Errorf("kumoHostedOrigin(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestLocalBackendURL(t *testing.T) {
	t.Setenv("KUMO_S3_BACKEND", "")
	t.Setenv("KUMO_HOST", "")
	t.Setenv("KUMO_PORT", "")

	if got := localBackendURL(); got != "http://localhost:4566" {
		t.Errorf("default = %q", got)
	}

	t.Setenv("KUMO_PORT", "4567")

	if got := localBackendURL(); got != "http://localhost:4567" {
		t.Errorf("KUMO_PORT = %q", got)
	}

	t.Setenv("KUMO_S3_BACKEND", "http://127.0.0.1:9999/")

	if got := localBackendURL(); got != "http://127.0.0.1:9999" {
		t.Errorf("KUMO_S3_BACKEND must win and lose its trailing slash, got %q", got)
	}
}

// newRecordingBackend stands in for kumo's own listener and records the
// Host header and path of every request it receives.
func newRecordingBackend(t *testing.T) (*httptest.Server, *http.Request) {
	t.Helper()

	var seen http.Request

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = *r.Clone(r.Context())

		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(backendBody))
	}))
	t.Cleanup(backend.Close)

	return backend, &seen
}

// TestEdge_KumoHostedCustomOriginsResolveInProcess — a custom origin whose
// domain is a Lambda function URL or execute-api host that kumo serves itself
// is sent to kumo's own listener with the Host header preserved, so the router
// dispatches it as it would a direct request.
func TestEdge_KumoHostedCustomOriginsResolveInProcess(t *testing.T) {
	for _, origin := range []string{testLambdaURLOrigin, testExecuteAPIOrigin} {
		t.Run(origin, func(t *testing.T) {
			backend, seen := newRecordingBackend(t)
			t.Setenv("KUMO_S3_BACKEND", backend.URL)

			svc := New(NewMemoryStorage())
			createCustomOriginDistribution(t, svc, origin, &CustomOriginConfigXML{HTTPSPort: 443, OriginProtocolPolicy: originPolicyHTTPS})

			w := callEdge(t, svc, http.MethodGet, "/hello/world", nil)
			if w.Code != http.StatusOK || w.Body.String() != backendBody {
				t.Fatalf("got %d %q", w.Code, w.Body.String())
			}

			if seen.Host != origin || seen.URL.Path != "/hello/world" {
				t.Fatalf("backend saw Host=%q path=%q", seen.Host, seen.URL.Path)
			}
		})
	}
}

// TestEdge_OtherCustomOriginsStayRemote — an ordinary custom origin is still
// fetched from its own host, not from kumo's listener.
func TestEdge_OtherCustomOriginsStayRemote(t *testing.T) {
	backend, seen := newRecordingBackend(t)
	t.Setenv("KUMO_S3_BACKEND", "http://127.0.0.1:1") // must never be contacted

	host, port := splitHostPort(backend.URL[len("http://"):])
	svc := New(NewMemoryStorage())
	createCustomOriginDistribution(t, svc, host, &CustomOriginConfigXML{HTTPPort: port, OriginProtocolPolicy: originPolicyHTTP})

	w := callEdge(t, svc, http.MethodGet, "/x", nil)
	if w.Code != http.StatusOK || seen.Host == "" {
		t.Fatalf("got %d %q (backend host %q)", w.Code, w.Body.String(), seen.Host)
	}
}

func createCustomOriginDistribution(t *testing.T, svc *Service, domain string, cfg *CustomOriginConfigXML) {
	t.Helper()

	if _, err := svc.storage.CreateDistribution(t.Context(), &CreateDistributionRequest{
		CallerReference: "local-origin-" + domain,
		Enabled:         true,
		Origins: &OriginsXML{
			Quantity: 1,
			Items:    &OriginList{Origin: []OriginXML{{ID: "origin", DomainName: domain, CustomOriginConfig: cfg}}},
		},
		DefaultCacheBehavior: &DefaultCacheBehaviorXML{TargetOriginID: "origin", DefaultTTL: 60, MaxTTL: 3600},
	}); err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}
}
