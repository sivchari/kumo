package cloudfront

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sivchari/kumo/internal/service/cloudfront/cache"
)

const (
	managedCachingDisabled  = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
	managedCachingOptimized = "658327ea-f89d-4fab-a63d-7e88639e58f6"
	amplifyDefault          = "4d1d2f1d-3a71-49ad-9e08-7ea5d843a556"
	testOriginID            = "origin"
	testCacheControl        = "Cache-Control"
)

func ptrInt64(v int64) *int64 { return &v }

func createTTLCachePolicy(t *testing.T, svc *Service, name string, minTTL, defaultTTL, maxTTL int64) string {
	t.Helper()

	policy, err := svc.storage.CreateCachePolicy(t.Context(), &CachePolicyConfig{
		Name:       name,
		MinTTL:     ptrInt64(minTTL),
		DefaultTTL: ptrInt64(defaultTTL),
		MaxTTL:     ptrInt64(maxTTL),
	})
	if err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	return policy.ID
}

func TestEdgeCacheConfig_CachePolicyTTLs(t *testing.T) {
	t.Parallel()

	const unsetTTLs = "unset-ttls"

	store := NewMemoryStorage()
	svc := New(store)
	custom := createTTLCachePolicy(t, svc, "custom-ttls", 5, 30, 120)
	legacy := cache.DistributionConfig{MinTTL: 2 * time.Second, DefaultTTL: 60 * time.Second, MaxTTL: time.Hour}

	// Only MinTTL is required; CreateCachePolicy stores the documented
	// defaults of the omitted DefaultTTL and MaxTTL.
	minOnly, err := svc.storage.CreateCachePolicy(t.Context(), &CachePolicyConfig{Name: "min-ttl-only", MinTTL: new(int64(5))})
	if err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	// A stored policy lacks them only when it bypassed CreateCachePolicy, as
	// in a snapshot edited by hand; the edge applies the same defaults.
	store.CachePolicies[unsetTTLs] = &CachePolicy{ID: unsetTTLs, Config: CachePolicyConfig{Name: unsetTTLs, MinTTL: new(int64(5))}}
	documentedDefaults := cache.DistributionConfig{MinTTL: 5 * time.Second, DefaultTTL: 24 * time.Hour, MaxTTL: 365 * 24 * time.Hour, CachePolicy: true}

	cases := []struct {
		name     string
		policyID string
		want     cache.DistributionConfig
	}{
		{"no cache policy keeps the legacy fields", "", legacy},
		{"managed CachingDisabled", managedCachingDisabled, cache.DistributionConfig{CachePolicy: true}},
		{
			"managed CachingOptimized", managedCachingOptimized,
			cache.DistributionConfig{MinTTL: time.Second, DefaultTTL: 24 * time.Hour, MaxTTL: 365 * 24 * time.Hour, CachePolicy: true},
		},
		{
			"custom policy from storage", custom,
			cache.DistributionConfig{MinTTL: 5 * time.Second, DefaultTTL: 30 * time.Second, MaxTTL: 2 * time.Minute, CachePolicy: true},
		},
		{"custom policy created with only MinTTL", minOnly.ID, documentedDefaults},
		{"stored policy without DefaultTTL and MaxTTL", unsetTTLs, documentedDefaults},
		{"managed policy without documented TTLs falls back", amplifyDefault, legacy},
		{"unknown policy falls back", "00000000-0000-0000-0000-000000000000", legacy},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := svc.edgeCacheConfig(t.Context(), &DefaultCacheBehavior{
				MinTTL: 2, DefaultTTL: 60, MaxTTL: 3600, CachePolicyID: tc.policyID,
			})
			if !ok || got != tc.want {
				t.Fatalf("got %+v (ok=%v), want %+v", got, ok, tc.want)
			}
		})
	}
}

// newHeaderOrigin serves a fixed body with the given response headers.
func newHeaderOrigin(t *testing.T, header http.Header) *httptest.Server {
	t.Helper()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, vs := range header {
			w.Header()[k] = vs
		}

		_, _ = w.Write([]byte("body"))
	}))
	t.Cleanup(origin.Close)

	return origin
}

func createBehaviorDistribution(t *testing.T, svc *Service, originURL string, behavior *DefaultCacheBehaviorXML) {
	t.Helper()

	host, port := splitHostPort(strings.TrimPrefix(originURL, "http://"))
	behavior.TargetOriginID = testOriginID

	if _, err := svc.storage.CreateDistribution(t.Context(), &CreateDistributionRequest{
		CallerReference: "cache-policy",
		Enabled:         true,
		Origins: &OriginsXML{Quantity: 1, Items: &OriginList{Origin: []OriginXML{{
			ID: testOriginID, DomainName: host,
			CustomOriginConfig: &CustomOriginConfigXML{HTTPPort: port, OriginProtocolPolicy: originPolicyHTTP},
		}}}},
		DefaultCacheBehavior: behavior,
	}); err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}
}

// TestEdge_CachePolicyDrivesCaching — the behaviour's cache policy decides
// whether the edge caches, not the legacy TTL fields (zero here, as with a
// Terraform cache_policy_id). The origin sends Last-Modified and no
// Cache-Control, like an S3 origin; the legacy row pins the unchanged
// heuristic for distributions without a cache policy.
func TestEdge_CachePolicyDrivesCaching(t *testing.T) {
	t.Parallel()

	lastModified := http.Header{"Last-Modified": {time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)}}

	cases := []struct {
		name     string
		header   http.Header
		policyID string  // a managed cache policy,
		custom   []int64 // or a custom one's MinTTL, DefaultTTL, MaxTTL
		second   string
	}{
		{name: "CachingDisabled", header: lastModified, policyID: managedCachingDisabled, second: cacheMiss},
		{name: "CachingOptimized", header: lastModified, policyID: managedCachingOptimized, second: cacheHit},
		{name: "custom MaxTTL 0", header: http.Header{testCacheControl: {"max-age=60"}}, custom: []int64{0, 0, 0}, second: cacheMiss},
		{name: "custom MinTTL does not cache no-store", header: http.Header{testCacheControl: {"no-store"}}, custom: []int64{60, 60, 600}, second: cacheMiss},
		{name: "legacy heuristic unchanged", header: lastModified, second: cacheHit},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := New(NewMemoryStorage())
			behavior := &DefaultCacheBehaviorXML{CachePolicyID: tc.policyID}

			if tc.custom != nil {
				behavior.CachePolicyID = createTTLCachePolicy(t, svc, "custom", tc.custom[0], tc.custom[1], tc.custom[2])
			}

			createBehaviorDistribution(t, svc, newHeaderOrigin(t, tc.header).URL, behavior)

			first := callEdge(t, svc, http.MethodGet, "/object", nil)
			second := callEdge(t, svc, http.MethodGet, "/object", nil)

			if got := first.Header().Get("X-Cache"); got != cacheMiss {
				t.Errorf("first X-Cache = %q, want %q", got, cacheMiss)
			}

			if got := second.Header().Get("X-Cache"); got != tc.second {
				t.Errorf("second X-Cache = %q, want %q", got, tc.second)
			}
		})
	}
}
