package cloudfront

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	signingLambdaOrigin = "abc123.lambda-url.us-east-1.on.aws"
	viewerAuthorization = "AWS4-HMAC-SHA256 Credential=AKIAVIEWER/20261001/us-east-1/lambda/aws4_request, SignedHeaders=host;x-amz-date, Signature=viewer"
	testBodyHash        = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
)

var edgeAuthorizationPattern = regexp.MustCompile(`^AWS4-HMAC-SHA256 Credential=` + edgeCredentials().AccessKeyID + `/\d{8}/us-east-1/lambda/aws4_request, SignedHeaders=[a-z0-9;-]*host[a-z0-9;-]*, Signature=[0-9a-f]{64}$`)

func TestLambdaURLRegion(t *testing.T) {
	t.Setenv("AWS_DEFAULT_REGION", "")

	for domain, want := range map[string]string{
		signingLambdaOrigin:                       "us-east-1",
		"abc123.lambda-url.ap-northeast-1.on.aws": "ap-northeast-1",
		testLambdaURLLocal:                        defaultSigningRegion,
		"abc123.lambda-url.localhost:4566":        defaultSigningRegion,
	} {
		if got := lambdaURLRegion(domain); got != want {
			t.Errorf("lambdaURLRegion(%q) = %q, want %q", domain, got, want)
		}
	}

	t.Setenv("AWS_DEFAULT_REGION", "eu-west-1")

	if got := lambdaURLRegion(testLambdaURLLocal); got != "eu-west-1" {
		t.Errorf("localhost host must follow AWS_DEFAULT_REGION, got %q", got)
	}
}

// newSigningStorage returns a storage with one origin access control per
// signing behaviour (lambda) plus an s3 one, keyed by name.
func newSigningStorage(t *testing.T) (*MemoryStorage, map[string]string) {
	t.Helper()

	store := NewMemoryStorage()
	ids := map[string]string{}

	for name, cfg := range map[string]*OriginAccessControlConfig{
		oacSigningAlways:     {Name: oacSigningAlways, SigningProtocol: oacSigningProtocolSigV4, SigningBehavior: oacSigningAlways, OriginAccessControlOriginType: oacOriginTypeLambda},
		oacSigningNever:      {Name: oacSigningNever, SigningProtocol: oacSigningProtocolSigV4, SigningBehavior: oacSigningNever, OriginAccessControlOriginType: oacOriginTypeLambda},
		oacSigningNoOverride: {Name: oacSigningNoOverride, SigningProtocol: oacSigningProtocolSigV4, SigningBehavior: oacSigningNoOverride, OriginAccessControlOriginType: oacOriginTypeLambda},
		oacOriginTypeS3:      {Name: oacOriginTypeS3, SigningProtocol: oacSigningProtocolSigV4, SigningBehavior: oacSigningAlways, OriginAccessControlOriginType: oacOriginTypeS3},
	} {
		oac, err := store.CreateOriginAccessControl(context.Background(), cfg)
		if err != nil {
			t.Fatalf("CreateOriginAccessControl(%s): %v", name, err)
		}

		ids[name] = oac.ID
	}

	return store, ids
}

func TestResolveSigning_NotSigned(t *testing.T) {
	store, ids := newSigningStorage(t)
	svc := New(store)

	for name, o := range map[string]*Origin{
		"an origin without an OAC":                     {DomainName: signingLambdaOrigin},
		"a missing OAC":                                {DomainName: signingLambdaOrigin, OriginAccessControlID: "EMISSING"},
		"an s3 OAC (kumo's S3 ignores signatures)":     {DomainName: "b.s3.us-east-1.amazonaws.com", OriginAccessControlID: ids[oacOriginTypeS3]},
		"a lambda OAC whose signing behavior is never": {DomainName: signingLambdaOrigin, OriginAccessControlID: ids[oacSigningNever]},
	} {
		if signing, err := svc.resolveSigning(t.Context(), o, &DefaultCacheBehavior{}); !errors.Is(err, errNotSigned) || signing != nil {
			t.Errorf("%s: resolveSigning = %+v, %v; want errNotSigned", name, signing, err)
		}
	}
}

func TestOriginSigningDecision(t *testing.T) {
	store, ids := newSigningStorage(t)
	svc := New(store)
	forwarding := &DefaultCacheBehavior{ForwardedValues: &ForwardedValues{Headers: &Headers{Quantity: 1, Items: []string{"authorization"}}}}
	plain := &DefaultCacheBehavior{}

	resolve := func(o *Origin, behavior *DefaultCacheBehavior) *originSigning {
		t.Helper()

		signing, err := svc.resolveSigning(t.Context(), o, behavior)
		if err != nil {
			t.Fatalf("resolveSigning: %v", err)
		}

		return signing
	}

	always := resolve(&Origin{DomainName: signingLambdaOrigin, OriginAccessControlID: ids[oacSigningAlways]}, forwarding)
	if always == nil || !always.shouldSign(viewerAuthorization) || !always.shouldSign("") || always.region != "us-east-1" {
		t.Errorf("always = %+v", always)
	}

	noOverride := resolve(&Origin{DomainName: signingLambdaOrigin, OriginAccessControlID: ids[oacSigningNoOverride]}, forwarding)
	if noOverride == nil || noOverride.shouldSign(viewerAuthorization) || !noOverride.shouldSign("") {
		t.Errorf("no-override with a forwarded Authorization header = %+v", noOverride)
	}

	noOverridePlain := resolve(&Origin{DomainName: signingLambdaOrigin, OriginAccessControlID: ids[oacSigningNoOverride]}, plain)
	if noOverridePlain == nil || !noOverridePlain.shouldSign(viewerAuthorization) {
		t.Errorf("no-override without forwarding must sign even when the viewer signed = %+v", noOverridePlain)
	}
}

// failingOACStorage fails every origin access control lookup with a
// non-not-found error.
type failingOACStorage struct {
	Storage
}

var errOACLookup = errors.New("oac lookup failed")

func (failingOACStorage) GetOriginAccessControl(context.Context, string) (*OriginAccessControl, error) {
	return nil, errOACLookup
}

// TestResolveSigning_StorageFailure — only a missing OAC means "unsigned"; any
// other lookup failure reaches the viewer as a 502 instead of an opaque 403
// from the function URL.
func TestResolveSigning_StorageFailure(t *testing.T) {
	backend, seen := newRecordingBackend(t)
	t.Setenv("KUMO_S3_BACKEND", backend.URL)

	store, ids := newSigningStorage(t)
	svc := New(store)
	createSigningDistribution(t, svc, lambdaOrigin(ids[oacSigningAlways]), false)

	var distID string
	for id := range store.Distributions {
		distID = id
	}

	svc.storage = failingOACStorage{Storage: store}

	if _, err := svc.resolveSigning(t.Context(), &Origin{DomainName: signingLambdaOrigin, OriginAccessControlID: ids[oacSigningAlways]}, nil); !errors.Is(err, errOACLookup) {
		t.Fatalf("resolveSigning error = %v, want %v", err, errOACLookup)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/kumo/cdn/"+distID+"/hello", http.NoBody)
	req.SetPathValue("distributionId", distID)
	req.SetPathValue("path", "hello")

	w := httptest.NewRecorder()
	svc.Edge(w, req)

	if w.Code != http.StatusBadGateway || seen.Host != "" {
		t.Fatalf("got %d (backend contacted: %v), want 502 without contacting the origin", w.Code, seen.Host != "")
	}
}

// createSigningDistribution creates a single-origin distribution whose origin
// carries oacID; forwardAuthorization whitelists the header on the behaviour.
func createSigningDistribution(t *testing.T, svc *Service, origin *OriginXML, forwardAuthorization bool) {
	t.Helper()

	behavior := &DefaultCacheBehaviorXML{TargetOriginID: origin.ID}
	if forwardAuthorization {
		behavior.ForwardedValues = &ForwardedValuesXML{Headers: &HeadersXML{Quantity: 1, Items: []string{"Authorization"}}}
	}

	if _, err := svc.storage.CreateDistribution(t.Context(), &CreateDistributionRequest{
		CallerReference:      "signing-" + origin.OriginAccessControlID + origin.DomainName,
		Enabled:              true,
		Origins:              &OriginsXML{Quantity: 1, Items: &OriginList{Origin: []OriginXML{*origin}}},
		DefaultCacheBehavior: behavior,
	}); err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}
}

func lambdaOrigin(oacID string) *OriginXML {
	return &OriginXML{ID: "fn", DomainName: signingLambdaOrigin, CustomOriginConfig: &CustomOriginConfigXML{OriginProtocolPolicy: originPolicyHTTPS}, OriginAccessControlID: oacID}
}

// callEdgeWithBody is callEdge for requests that carry a body.
func callEdgeWithBody(t *testing.T, svc *Service, method, path, body string, hdr http.Header) *httptest.ResponseRecorder {
	t.Helper()

	mem, _ := svc.storage.(*MemoryStorage)

	var distID string
	for id := range mem.Distributions {
		distID = id
	}

	req := httptest.NewRequestWithContext(t.Context(), method, "/kumo/cdn/"+distID+"/"+strings.TrimPrefix(path, "/"), strings.NewReader(body))
	req.SetPathValue("distributionId", distID)
	req.SetPathValue("path", strings.TrimPrefix(path, "/"))

	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	w := httptest.NewRecorder()
	svc.Edge(w, req)

	return w
}

func TestEdge_LambdaOACSigning(t *testing.T) {
	cases := []struct {
		name                 string
		oac                  string
		forwardAuthorization bool
		viewerAuthorization  string
		wantEdgeSignature    bool
		wantViewerForwarded  bool
	}{
		{"always replaces the viewer signature", oacSigningAlways, true, viewerAuthorization, true, false},
		{"always signs unsigned requests", oacSigningAlways, false, "", true, false},
		{"never forwards as is", oacSigningNever, false, viewerAuthorization, false, true},
		{"no-override forwards a whitelisted viewer signature", oacSigningNoOverride, true, viewerAuthorization, false, true},
		{"no-override signs when the viewer sent none", oacSigningNoOverride, true, "", true, false},
		{"no-override signs when Authorization is not forwarded", oacSigningNoOverride, false, viewerAuthorization, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			backend, seen := newRecordingBackend(t)
			t.Setenv("KUMO_S3_BACKEND", backend.URL)

			store, ids := newSigningStorage(t)
			svc := New(store)
			createSigningDistribution(t, svc, lambdaOrigin(ids[tc.oac]), tc.forwardAuthorization)

			hdr := http.Header{}
			if tc.viewerAuthorization != "" {
				hdr.Set("Authorization", tc.viewerAuthorization)
			}

			w := callEdge(t, svc, http.MethodGet, "/hello", hdr)
			if w.Code != http.StatusOK || seen.Host != signingLambdaOrigin {
				t.Fatalf("got %d, backend host %q", w.Code, seen.Host)
			}

			got := seen.Header.Get("Authorization")

			switch {
			case tc.wantEdgeSignature && !edgeAuthorizationPattern.MatchString(got):
				t.Errorf("Authorization = %q, want an edge SigV4 signature", got)
			case tc.wantEdgeSignature && (seen.Header.Get("X-Amz-Date") == "" || seen.Header.Get("X-Amz-Content-Sha256") != emptyPayloadHash):
				t.Errorf("signed request must carry X-Amz-Date and the empty payload hash: %v", seen.Header)
			case tc.wantViewerForwarded && got != tc.viewerAuthorization:
				t.Errorf("Authorization = %q, want the viewer's header forwarded", got)
			case !tc.wantEdgeSignature && !tc.wantViewerForwarded && got != "":
				t.Errorf("Authorization = %q, want none", got)
			}
		})
	}
}

// TestEdge_LambdaOACSigningRequiresPayloadHash — AWS requires the viewer to
// send x-amz-content-sha256 with a body (Lambda accepts no unsigned payloads).
func TestEdge_LambdaOACSigningRequiresPayloadHash(t *testing.T) {
	backend, seen := newRecordingBackend(t)
	t.Setenv("KUMO_S3_BACKEND", backend.URL)

	store, ids := newSigningStorage(t)
	svc := New(store)
	createSigningDistribution(t, svc, lambdaOrigin(ids[oacSigningAlways]), false)

	if w := callEdgeWithBody(t, svc, http.MethodPost, "/submit", "test", nil); w.Code != http.StatusForbidden || seen.Host != "" {
		t.Fatalf("body without x-amz-content-sha256: got %d (backend contacted: %v)", w.Code, seen.Host != "")
	}

	hdr := http.Header{"X-Amz-Content-Sha256": {testBodyHash}}

	w := callEdgeWithBody(t, svc, http.MethodPost, "/submit", "test", hdr)
	if w.Code != http.StatusOK || !edgeAuthorizationPattern.MatchString(seen.Header.Get("Authorization")) || seen.Header.Get("X-Amz-Content-Sha256") != testBodyHash {
		t.Fatalf("body with hash: got %d, Authorization %q, hash %q", w.Code, seen.Header.Get("Authorization"), seen.Header.Get("X-Amz-Content-Sha256"))
	}
}

// TestEdge_LambdaOACSigningIgnoresBodyOfCacheableMethods — the edge sends
// GET/HEAD upstream body-less, so a viewer body there needs no payload hash.
func TestEdge_LambdaOACSigningIgnoresBodyOfCacheableMethods(t *testing.T) {
	backend, seen := newRecordingBackend(t)
	t.Setenv("KUMO_S3_BACKEND", backend.URL)

	store, ids := newSigningStorage(t)
	svc := New(store)
	createSigningDistribution(t, svc, lambdaOrigin(ids[oacSigningAlways]), false)

	w := callEdgeWithBody(t, svc, http.MethodGet, "/hello", "ignored", nil)
	if w.Code != http.StatusOK || !edgeAuthorizationPattern.MatchString(seen.Header.Get("Authorization")) || seen.Header.Get("X-Amz-Content-Sha256") != emptyPayloadHash {
		t.Fatalf("GET with a body: got %d, Authorization %q, hash %q", w.Code, seen.Header.Get("Authorization"), seen.Header.Get("X-Amz-Content-Sha256"))
	}
}

// TestEdge_S3OACIsNotSigned — kumo's S3 does not verify signatures, so an s3
// origin access control changes nothing on the wire.
func TestEdge_S3OACIsNotSigned(t *testing.T) {
	backend, seen := newRecordingBackend(t)
	t.Setenv("KUMO_S3_BACKEND", backend.URL)

	store, ids := newSigningStorage(t)
	svc := New(store)
	createSigningDistribution(t, svc, &OriginXML{ID: "s3", DomainName: oacTestS3Domain, S3OriginConfig: &S3OriginConfigXML{}, OriginAccessControlID: ids[oacOriginTypeS3]}, false)

	if w := callEdge(t, svc, http.MethodGet, "/object.txt", nil); w.Code != http.StatusOK || seen.Header.Get("Authorization") != "" || seen.URL.Path != "/mybucket/object.txt" {
		t.Fatalf("got %d, Authorization %q, path %q", w.Code, seen.Header.Get("Authorization"), seen.URL.Path)
	}
}

// TestEdge_LambdaOACSigningCoversRevalidation — the conditional request a
// stale cache entry triggers goes through the same signing as the first
// fetch, so a revalidation never reaches the function URL unsigned.
func TestEdge_LambdaOACSigningCoversRevalidation(t *testing.T) {
	var (
		mu   sync.Mutex
		seen []http.Header
	)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()

		w.Header().Set("Cache-Control", "public, max-age=1")
		w.Header().Set("ETag", `"signed-asset"`)

		if r.Header.Get("If-None-Match") != "" {
			w.WriteHeader(http.StatusNotModified)

			return
		}

		_, _ = w.Write([]byte(backendBody))
	}))
	t.Cleanup(backend.Close)
	t.Setenv("KUMO_S3_BACKEND", backend.URL)

	store, ids := newSigningStorage(t)
	svc := New(store)
	createSigningDistribution(t, svc, lambdaOrigin(ids[oacSigningAlways]), false)

	if w := callEdge(t, svc, http.MethodGet, "/asset", nil); w.Code != http.StatusOK {
		t.Fatalf("first fetch: %d", w.Code)
	}

	backdateEdgeEntries(svc, 2*time.Second)

	if w := callEdge(t, svc, http.MethodGet, "/asset", nil); w.Code != http.StatusOK || w.Body.String() != backendBody {
		t.Fatalf("stale fetch: %d %q", w.Code, w.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()

	if len(seen) != 2 || seen[1].Get("If-None-Match") == "" {
		t.Fatalf("expected a fetch followed by a conditional revalidation, got %d requests", len(seen))
	}

	for i, hdr := range seen {
		if !edgeAuthorizationPattern.MatchString(hdr.Get("Authorization")) {
			t.Errorf("request %d Authorization = %q, want an edge signature", i+1, hdr.Get("Authorization"))
		}
	}
}
