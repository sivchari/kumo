package cloudfront

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestEdge_HitMissPattern stands up a tiny origin and walks through
// the canonical Miss → Hit pattern. The origin counter pins how many
// times the upstream was actually contacted, which is the operational
// claim the cache makes.
func TestEdge_HitMissPattern(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))
	defer origin.Close()

	svc := setupEdge(t, origin.URL)

	first := callEdge(t, svc, "GET", "/static/index.html", nil)

	if first.Code != http.StatusOK {
		t.Fatalf("first miss: status %d", first.Code)
	}

	if first.Header().Get("X-Cache") != "Miss from kumo" {
		t.Fatalf("first should be Miss, got %q", first.Header().Get("X-Cache"))
	}

	second := callEdge(t, svc, "GET", "/static/index.html", nil)

	if second.Header().Get("X-Cache") != "Hit from kumo" {
		t.Fatalf("second should be Hit, got %q", second.Header().Get("X-Cache"))
	}

	if hits.Load() != 1 {
		t.Fatalf("origin contacted %d times, expected 1", hits.Load())
	}
}

// TestEdge_NoStoreSkipsCache — a response with `Cache-Control: no-store`
// must never be served from cache. Two consecutive requests therefore
// both hit the origin.
func TestEdge_NoStoreSkipsCache(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("dynamic"))
	}))
	defer origin.Close()

	svc := setupEdge(t, origin.URL)

	for i := 0; i < 2; i++ {
		_ = callEdge(t, svc, "GET", "/api/dynamic", nil)
	}

	if hits.Load() != 2 {
		t.Fatalf("no-store should bypass cache; origin hits = %d, want 2", hits.Load())
	}
}

// TestEdge_VarySplit confirms two requests differing in a Vary'd
// header land on different cache variants.
func TestEdge_VarySplit(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("Vary", "Accept-Language")
		_, _ = w.Write([]byte("lang=" + r.Header.Get("Accept-Language")))
	}))
	defer origin.Close()

	svc := setupEdge(t, origin.URL)

	en := callEdge(t, svc, "GET", "/translated", http.Header{"Accept-Language": {"en"}})
	ja := callEdge(t, svc, "GET", "/translated", http.Header{"Accept-Language": {"ja"}})

	if hits.Load() != 2 {
		t.Fatalf("Vary should split cache; origin hits = %d, want 2", hits.Load())
	}

	// Re-hit en should be cached.
	enAgain := callEdge(t, svc, "GET", "/translated", http.Header{"Accept-Language": {"en"}})

	if hits.Load() != 2 {
		t.Fatalf("repeat en should hit cache; origin hits = %d, want 2", hits.Load())
	}

	if enAgain.Header().Get("X-Cache") != "Hit from kumo" {
		t.Fatalf("expected Hit on cached variant, got %q", enAgain.Header().Get("X-Cache"))
	}

	if !strings.Contains(en.Body.String(), "lang=en") || !strings.Contains(ja.Body.String(), "lang=ja") {
		t.Fatalf("variant bodies got mixed up: en=%q ja=%q", en.Body.String(), ja.Body.String())
	}
}

func TestEdgeCache_StoreKeepsDifferentVaryHeaderNames(t *testing.T) {
	t.Parallel()

	c := newEdgeCache()

	c.store("dist", "/asset", &cacheEntry{
		Header:     http.Header{"Vary": {"Accept-Encoding"}},
		StoredAt:   time.Now(),
		TTL:        time.Minute,
		Vary:       []string{"accept-encoding"},
		VaryValues: map[string]string{"accept-encoding": ""},
	})
	c.store("dist", "/asset", &cacheEntry{
		Header:     http.Header{"Vary": {"Accept-Language"}},
		StoredAt:   time.Now(),
		TTL:        time.Minute,
		Vary:       []string{"accept-language"},
		VaryValues: map[string]string{"accept-language": ""},
	})

	if got := len(c.entries["dist"]["/asset"]); got != 2 {
		t.Fatalf("different Vary names must not replace each other; variants = %d, want 2", got)
	}
}

func TestEdgeCache_StoreEvictsOldestEntriesAtCap(t *testing.T) {
	t.Parallel()

	c := newEdgeCache()

	for i := 0; i < 1100; i++ {
		c.store("dist", fmt.Sprintf("/asset-%04d", i), &cacheEntry{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Cache-Control": {"public, max-age=60"}},
			Body:       []byte("payload"),
			StoredAt:   time.Unix(int64(i), 0),
			TTL:        time.Minute,
		})
	}

	if got := edgeCacheEntryCount(c); got > 1024 {
		t.Fatalf("edge cache entry count = %d, want <= 1024", got)
	}

	if _, ok := c.lookup("dist", "/asset-0000", httptest.NewRequest(http.MethodGet, "/asset-0000", http.NoBody)); ok {
		t.Fatalf("oldest entry should have been evicted")
	}

	if _, ok := c.lookup("dist", "/asset-1099", httptest.NewRequest(http.MethodGet, "/asset-1099", http.NoBody)); !ok {
		t.Fatalf("newest entry should remain cached")
	}
}

func TestEdge_StaleWhileRevalidateRaceFree(t *testing.T) {
	var hits atomic.Int64

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit := hits.Add(1)

		w.Header().Set("Cache-Control", "public, max-age=1")
		w.Header().Set("CDN-Cache-Control", "max-age=1, stale-while-revalidate=60")
		w.Header().Set("ETag", `"edge-race"`)

		if r.Header.Get("If-None-Match") != "" {
			w.WriteHeader(http.StatusNotModified)

			return
		}

		_, _ = fmt.Fprintf(w, "body-%d", hit)
	}))
	defer origin.Close()

	svc := setupEdge(t, origin.URL)
	_ = callEdge(t, svc, http.MethodGet, "/race", nil)

	backdateEdgeEntries(svc, 2*time.Second)

	var wg sync.WaitGroup

	for i := 0; i < 64; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			w := callEdge(t, svc, http.MethodGet, "/race", nil)
			if w.Code != http.StatusOK {
				t.Errorf("stale serve status = %d, want 200", w.Code)
			}
		}()
	}

	wg.Wait()

	// Give the background revalidation goroutine time to complete. Under
	// `go test -race`, the old implementation raced with the concurrent
	// stale serves while updating the cached entry in place.
	time.Sleep(50 * time.Millisecond)
}

// TestEdge_SMaxAgeOverride pins the s-maxage > max-age precedence the
// cache rules enforce. If the cache used max-age (5s) by mistake the
// second call would still hit, but we want to verify s-maxage (1s)
// won — by the time we re-call, the entry is stale.
//
// We don't actually sleep — instead we backdate the cached entry's
// StoredAt to simulate elapsed time.
func TestEdge_SMaxAgeOverride(t *testing.T) {
	t.Parallel()

	var hits atomic.Int64

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Cache-Control", "max-age=300, s-maxage=1")
		_, _ = w.Write([]byte("payload"))
	}))
	defer origin.Close()

	svc := setupEdge(t, origin.URL)

	_ = callEdge(t, svc, "GET", "/short-shared-ttl", nil)

	// Backdate the stored entry by 5s so the s-maxage=1 has expired
	// but max-age=300 hasn't. CloudFront should treat it as stale.
	for _, dm := range svc.edgeCache.entries {
		for _, variants := range dm {
			for _, e := range variants {
				e.StoredAt = e.StoredAt.Add(-5 * 1e9) // -5s
			}
		}
	}

	again := callEdge(t, svc, "GET", "/short-shared-ttl", nil)

	if again.Header().Get("X-Cache") != "Miss from kumo" {
		t.Fatalf("s-maxage=1 should have expired; got X-Cache=%q", again.Header().Get("X-Cache"))
	}

	if hits.Load() != 2 {
		t.Fatalf("origin hits = %d, want 2 (cache should have been stale)", hits.Load())
	}
}

func edgeCacheEntryCount(c *edgeCache) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := 0

	for _, dm := range c.entries {
		for _, variants := range dm {
			total += len(variants)
		}
	}

	return total
}

func backdateEdgeEntries(svc *Service, d time.Duration) {
	svc.edgeCache.mu.Lock()
	defer svc.edgeCache.mu.Unlock()

	for _, dm := range svc.edgeCache.entries {
		for _, variants := range dm {
			for _, e := range variants {
				e.StoredAt = e.StoredAt.Add(-d)
			}
		}
	}
}

// TestS3BucketFromDomain enumerates the virtual-hosted S3 hostname
// shapes the edge must recognise to route an S3OriginConfig back at
// kumo's own S3 service.
func TestS3BucketFromDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		domain string
		want   string
	}{
		{"mybucket.s3.amazonaws.com", "mybucket"},
		{"mybucket.s3.us-east-1.amazonaws.com", "mybucket"},
		{"mybucket.s3-us-west-2.amazonaws.com", "mybucket"},
		{"mybucket.s3.dualstack.us-east-1.amazonaws.com", "mybucket"},
		{"example.com", ""},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.domain, func(t *testing.T) {
			if got := s3BucketFromDomain(tc.domain); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEdge_S3Origin verifies an S3 origin gets routed at kumo's own
// S3 service (path-style) instead of out to real AWS. We point
// KUMO_S3_BACKEND at a tiny httptest server that mimics the S3
// path-style response.
func TestEdge_S3Origin(t *testing.T) {
	const objectBody = "hello-from-fake-s3"

	var gotPath string

	fakeS3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(objectBody))
	}))
	defer fakeS3.Close()

	t.Setenv("KUMO_S3_BACKEND", fakeS3.URL)

	svc := New(NewMemoryStorage())

	_, err := svc.storage.CreateDistribution(t.Context(), &CreateDistributionRequest{
		CallerReference: "s3-origin",
		Enabled:         true,
		Origins: &OriginsXML{
			Quantity: 1,
			Items: &OriginList{
				Origin: []OriginXML{{
					ID:             "s3-origin",
					DomainName:     "mybucket.s3.us-east-1.amazonaws.com",
					S3OriginConfig: &S3OriginConfigXML{OriginAccessIdentity: ""},
				}},
			},
		},
		DefaultCacheBehavior: &DefaultCacheBehaviorXML{
			TargetOriginID: "s3-origin",
			MinTTL:         0,
			DefaultTTL:     86400,
			MaxTTL:         31536000,
		},
	})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	w := callEdge(t, svc, http.MethodGet, "/path/to/object.txt", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, body=%s", w.Code, w.Body.String())
	}

	if w.Body.String() != objectBody {
		t.Fatalf("body: got %q, want %q", w.Body.String(), objectBody)
	}

	if gotPath != "/mybucket/path/to/object.txt" {
		t.Fatalf("upstream path: got %q, want /mybucket/path/to/object.txt", gotPath)
	}
}

// TestEdge_TargetOriginIDSelection — when a distribution has more
// than one origin, the cache forwards to the one named by
// DefaultCacheBehavior.TargetOriginID, not the first.
func TestEdge_TargetOriginIDSelection(t *testing.T) {
	t.Parallel()

	wrong := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("WRONG ORIGIN"))
	}))
	defer wrong.Close()

	right := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = w.Write([]byte("right"))
	}))
	defer right.Close()

	svc := New(NewMemoryStorage())

	wrongHost, wrongPort := splitHostPort(strings.TrimPrefix(wrong.URL, "http://"))
	rightHost, rightPort := splitHostPort(strings.TrimPrefix(right.URL, "http://"))

	_, err := svc.storage.CreateDistribution(t.Context(), &CreateDistributionRequest{
		CallerReference: "two-origin",
		Enabled:         true,
		Origins: &OriginsXML{
			Quantity: 2,
			Items: &OriginList{
				Origin: []OriginXML{
					{
						ID:                 "wrong",
						DomainName:         wrongHost,
						CustomOriginConfig: &CustomOriginConfigXML{HTTPPort: wrongPort, OriginProtocolPolicy: "http-only"},
					},
					{
						ID:                 "right",
						DomainName:         rightHost,
						CustomOriginConfig: &CustomOriginConfigXML{HTTPPort: rightPort, OriginProtocolPolicy: "http-only"},
					},
				},
			},
		},
		DefaultCacheBehavior: &DefaultCacheBehaviorXML{
			TargetOriginID: "right",
			MinTTL:         0,
			DefaultTTL:     86400,
			MaxTTL:         31536000,
		},
	})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	w := callEdge(t, svc, http.MethodGet, "/foo", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, body=%s", w.Code, w.Body.String())
	}

	if w.Body.String() != "right" {
		t.Fatalf("served from wrong origin: %q", w.Body.String())
	}
}

// TestEdge_404FromKumoOnUnknownDistribution — a request to
// /kumo/cdn/<bogus>/... returns 404 with no upstream contact.
func TestEdge_404FromKumoOnUnknownDistribution(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	req := httptest.NewRequest(http.MethodGet, "/kumo/cdn/dist-bogus/anything", http.NoBody)
	req.SetPathValue("distributionId", "dist-bogus")
	req.SetPathValue("path", "anything")

	w := httptest.NewRecorder()
	svc.Edge(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on unknown distribution, got %d", w.Code)
	}
}

// setupEdge wires a Service with one distribution pointing at the
// given upstream. The upstream URL is parsed and split into
// host:port + scheme so it lands in CustomOriginConfig the same way
// terraform would author it.
func setupEdge(t *testing.T, originURL string) *Service {
	t.Helper()

	svc := New(NewMemoryStorage())

	// Strip the http:// prefix to get host:port.
	host := strings.TrimPrefix(originURL, "http://")

	hostOnly, port := splitHostPort(host)

	_, err := svc.storage.CreateDistribution(context.Background(), &CreateDistributionRequest{
		CallerReference: "test",
		Enabled:         true,
		Origins: &OriginsXML{
			Quantity: 1,
			Items: &OriginList{
				Origin: []OriginXML{{
					ID:         "origin-1",
					DomainName: hostOnly,
					CustomOriginConfig: &CustomOriginConfigXML{
						HTTPPort:             port,
						OriginProtocolPolicy: "http-only",
					},
				}},
			},
		},
		DefaultCacheBehavior: &DefaultCacheBehaviorXML{
			TargetOriginID: "origin-1",
			MinTTL:         0,
			DefaultTTL:     86400,
			MaxTTL:         31536000,
		},
	})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	return svc
}

// splitHostPort separates "host:port" into ("host", port). Returns
// port=80 when no port is in the input.
func splitHostPort(hp string) (string, int) {
	idx := strings.LastIndex(hp, ":")
	if idx < 0 {
		return hp, 80
	}

	host := hp[:idx]
	port := 0

	for _, c := range hp[idx+1:] {
		if c < '0' || c > '9' {
			return host, 80
		}

		port = port*10 + int(c-'0')

		if port > 65535 {
			return host, 80
		}
	}

	return host, port
}

// callEdge invokes Service.Edge directly with the supplied path /
// headers. Returns the recorder so the caller can inspect status,
// headers, and body.
func callEdge(t *testing.T, svc *Service, _, path string, hdr http.Header) *httptest.ResponseRecorder {
	t.Helper()

	// Find the (only) distribution the test setup created.
	mem, ok := svc.storage.(*MemoryStorage)
	if !ok {
		t.Fatalf("expected *MemoryStorage, got %T", svc.storage)
	}

	var distID string
	for id := range mem.Distributions {
		distID = id
	}

	req := httptest.NewRequest(http.MethodGet, "/kumo/cdn/"+distID+strings.TrimPrefix(path, "/"), http.NoBody)
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
