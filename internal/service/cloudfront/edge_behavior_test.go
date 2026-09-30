package cloudfront

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	originDefault = "default-origin"
	originAPI     = "api-origin"
	originJPG     = "jpg"
	apiPattern    = "/api/*"
	jpgPattern    = "images/*.jpg"
	sampleGIF     = "images/sample.gif"
	exactFile     = "exact.txt"
	cacheHit      = "Hit from kumo"
	cacheMiss     = "Miss from kumo"
)

// TestMatchPathPattern covers the wildcard rules from the CloudFront
// developer guide: `*` is zero or more characters (across `/`), `?` exactly
// one, matching is case-sensitive and a leading `/` is implied.
func TestMatchPathPattern(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern, path string
		want          bool
	}{
		{jpgPattern, "images/sample.jpg", true},
		{jpgPattern, "images/product1/sample.jpg", true},
		{jpgPattern, sampleGIF, false},
		{"/images/*.jpg", "images/sample.jpg", true},
		{"images/*", sampleGIF, true},
		{"*.gif", sampleGIF, true},
		{"a*.jpg", "apple.jpg", true},
		{"a*.jpg", "abra/cadabra/magic.jpg", true},
		{"a??.jpg", "ant.jpg", true},
		{"a??.jpg", "apple.jpg", false},
		{"*.doc*", "report.docx", true},
		{"*.doc?", "report.doc", false},
		{"*.jpg", "LOGO.JPG", false},
		{apiPattern, "api", false},
		{apiPattern, "api/", true},
		{apiPattern, "api/v1/items", true},
		{"*", "", true},
		{"*", "anything/at/all", true},
		{exactFile, exactFile, true},
		{exactFile, "exact.txt.bak", false},
	}

	for _, tc := range cases {
		if got := matchPathPattern(tc.pattern, tc.path); got != tc.want {
			t.Errorf("matchPathPattern(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

// TestResolveBehavior — ordered behaviours win in list order, the default
// behaviour is the fallback, and a distribution without one yields nil.
func TestResolveBehavior(t *testing.T) {
	t.Parallel()

	dist := &Distribution{DistributionConfig: &DistributionConfig{
		DefaultCacheBehavior: &DefaultCacheBehavior{TargetOriginID: originDefault},
		CacheBehaviors: &CacheBehaviors{Quantity: 2, Items: []CacheBehavior{
			{PathPattern: jpgPattern, DefaultCacheBehavior: DefaultCacheBehavior{TargetOriginID: originJPG}},
			{PathPattern: "images/*", DefaultCacheBehavior: DefaultCacheBehavior{TargetOriginID: "images"}},
		}},
	}}

	for path, want := range map[string]string{
		"images/a.jpg":     originJPG,
		"images/a.gif":     "images",
		"other/a.jpg":      originDefault,
		"":                 originDefault,
		"images/sub/b.jpg": originJPG,
	} {
		if got := resolveBehavior(dist, path); got == nil || got.TargetOriginID != want {
			t.Errorf("resolveBehavior(%q) = %+v, want target %q", path, got, want)
		}
	}

	if resolveBehavior(&Distribution{DistributionConfig: &DistributionConfig{}}, "x") != nil {
		t.Error("a distribution without a default behaviour must resolve to nil")
	}

	if resolveBehavior(&Distribution{}, "x") != nil {
		t.Error("a distribution without a config must resolve to nil")
	}
}

// newTwoOriginDistribution creates a distribution with a default origin and
// an `/api/*` behaviour pointing at a second origin, each with its own TTLs.
func newTwoOriginDistribution(t *testing.T, svc *Service, defaultURL, apiURL string, defaultBehavior, apiBehavior *DefaultCacheBehaviorXML) {
	t.Helper()

	defaultHost, defaultPort := splitHostPort(strings.TrimPrefix(defaultURL, "http://"))
	apiHost, apiPort := splitHostPort(strings.TrimPrefix(apiURL, "http://"))

	defaultBehavior.TargetOriginID = originDefault
	apiBehavior.TargetOriginID = originAPI

	if _, err := svc.storage.CreateDistribution(t.Context(), &CreateDistributionRequest{
		CallerReference: "two-behaviours",
		Enabled:         true,
		Origins: &OriginsXML{Quantity: 2, Items: &OriginList{Origin: []OriginXML{
			{ID: originDefault, DomainName: defaultHost, CustomOriginConfig: &CustomOriginConfigXML{HTTPPort: defaultPort, OriginProtocolPolicy: originPolicyHTTP}},
			{ID: originAPI, DomainName: apiHost, CustomOriginConfig: &CustomOriginConfigXML{HTTPPort: apiPort, OriginProtocolPolicy: originPolicyHTTP}},
		}}},
		DefaultCacheBehavior: defaultBehavior,
		CacheBehaviors: &CacheBehaviorsXML{Quantity: 1, Items: []CacheBehaviorXML{
			{PathPattern: apiPattern, DefaultCacheBehaviorXML: *apiBehavior},
		}},
	}); err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}
}

func newNamedOrigin(t *testing.T, name string) *httptest.Server {
	t.Helper()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(name))
	}))
	t.Cleanup(origin.Close)

	return origin
}

// TestEdge_OrderedBehaviorSelectsOriginAndTTL — the matched behaviour decides
// both the origin and the cache TTLs: `/api/*` goes to the api origin and is
// cached with its own DefaultTTL, while the default behaviour (TTLs of zero)
// goes to the default origin and never caches.
func TestEdge_OrderedBehaviorSelectsOriginAndTTL(t *testing.T) {
	svc := New(NewMemoryStorage())
	newTwoOriginDistribution(t, svc, newNamedOrigin(t, originDefault).URL, newNamedOrigin(t, originAPI).URL,
		&DefaultCacheBehaviorXML{}, &DefaultCacheBehaviorXML{DefaultTTL: 60, MaxTTL: 3600})

	for _, tc := range []struct{ path, body, firstCache, secondCache string }{
		{"/api/items", originAPI, cacheMiss, cacheHit},
		{"/items", originDefault, cacheMiss, cacheMiss},
	} {
		first := callEdge(t, svc, http.MethodGet, tc.path, nil)
		second := callEdge(t, svc, http.MethodGet, tc.path, nil)

		if first.Body.String() != tc.body || second.Body.String() != tc.body {
			t.Errorf("%s served %q then %q, want %q", tc.path, first.Body.String(), second.Body.String(), tc.body)
		}

		if first.Header().Get("X-Cache") != tc.firstCache || second.Header().Get("X-Cache") != tc.secondCache {
			t.Errorf("%s X-Cache %q then %q, want %q then %q", tc.path, first.Header().Get("X-Cache"), second.Header().Get("X-Cache"), tc.firstCache, tc.secondCache)
		}
	}
}

// TestEdge_TrustedKeyGroupsFollowBehavior — signed-URL enforcement comes from
// the matched behaviour: a protected default behaviour rejects unsigned
// requests while an unprotected `/api/*` behaviour serves them.
func TestEdge_TrustedKeyGroupsFollowBehavior(t *testing.T) {
	svc := New(NewMemoryStorage())
	newTwoOriginDistribution(t, svc, newNamedOrigin(t, originDefault).URL, newNamedOrigin(t, originAPI).URL,
		&DefaultCacheBehaviorXML{TrustedKeyGroups: &TrustedKeyGroupsXML{Enabled: true, Quantity: 1, Items: []string{"kg-private"}}},
		&DefaultCacheBehaviorXML{})

	if w := callEdge(t, svc, http.MethodGet, "/private.txt", nil); w.Code != http.StatusForbidden {
		t.Errorf("protected default behaviour: got %d, want 403", w.Code)
	}

	if w := callEdge(t, svc, http.MethodGet, "/api/public", nil); w.Code != http.StatusOK || w.Body.String() != originAPI {
		t.Errorf("unprotected api behaviour: got %d %q", w.Code, w.Body.String())
	}
}
