package cloudfront

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/service/cloudfront/cache"
)

// cacheEntry is one cached response variant for a distribution.
type cacheEntry struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	StoredAt   time.Time
	TTL        time.Duration
	Vary       []string          // header names that pinned this variant
	VaryValues map[string]string // request values seen at store time
}

// edgeCache stores responses keyed by (distId, base) where base is
// the path+query portion. Each base can hold multiple variants — one
// per Vary'd header value combination. Concurrency-safe.
//
// The two-level layout matches RFC 7234's "secondary key": the base
// gets you to the resource, the variant list resolves the right
// representation given the request's Vary'd headers.
type edgeCache struct {
	mu      sync.Mutex
	entries map[string]map[string][]*cacheEntry // distId → base → variants
}

func newEdgeCache() *edgeCache {
	return &edgeCache{entries: make(map[string]map[string][]*cacheEntry)}
}

// lookup finds a variant whose Vary'd header values match those on the
// request. Returns (nil, false) if no matching variant exists.
func (c *edgeCache) lookup(distID, base string, r *http.Request) (*cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	dm, ok := c.entries[distID]
	if !ok {
		return nil, false
	}

	for _, v := range dm[base] {
		if matchesVary(v, r) {
			return v, true
		}
	}

	return nil, false
}

// store adds (or replaces) the variant for the given Vary values.
func (c *edgeCache) store(distID, base string, entry *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.entries[distID] == nil {
		c.entries[distID] = make(map[string][]*cacheEntry)
	}

	// Replace any existing variant with the same Vary signature.
	variants := c.entries[distID][base]
	for i, v := range variants {
		if sameVarySignature(v, entry) {
			variants[i] = entry
			c.entries[distID][base] = variants

			return
		}
	}

	c.entries[distID][base] = append(variants, entry)
}

// matchesVary reports whether the request's headers for the entry's
// recorded Vary list equal the ones it was stored with.
func matchesVary(e *cacheEntry, r *http.Request) bool {
	if len(e.Vary) == 0 {
		return true
	}

	for _, name := range e.Vary {
		if r.Header.Get(name) != e.VaryValues[name] {
			return false
		}
	}

	return true
}

// sameVarySignature checks two entries declare the same Vary headers
// AND the same values (used during store to overwrite stale variants).
func sameVarySignature(a, b *cacheEntry) bool {
	if len(a.Vary) != len(b.Vary) {
		return false
	}

	for _, name := range a.Vary {
		if a.VaryValues[name] != b.VaryValues[name] {
			return false
		}
	}

	return true
}

// Edge handles requests routed through `/kumo/cdn/{distId}/{path...}`.
// It implements the CloudFront edge caching contract:
//
//  1. Resolve the distribution + chosen origin.
//  2. For non-safe methods (PUT / POST / DELETE / PATCH), pass through
//     to the origin without consulting or storing the cache. RFC 9111
//     §4.4 requires the cache to invalidate on these too — handled in
//     a follow-up.
//  3. For safe methods (GET / HEAD), look up the cache. If fresh and
//     not flagged for forced revalidation, serve it with
//     `X-Cache: Hit from kumo` and the standard `Age` header.
//  4. Otherwise fetch from the origin, evaluate cacheability + TTL via
//     the rules in `cache/`, store the response when allowed, and
//     serve with `X-Cache: Miss from kumo`.
//
// Revalidation (`If-None-Match` / `If-Modified-Since`) is left for a
// follow-up PR — `MustRevalidate` is currently treated as "always
// miss".
func (s *Service) Edge(w http.ResponseWriter, r *http.Request) {
	distID := r.PathValue("distributionId")

	dist, err := s.storage.GetDistribution(r.Context(), distID)
	if err != nil {
		http.Error(w, "no such distribution: "+distID, http.StatusNotFound)

		return
	}

	originURL, ok := edgeOriginURL(dist, r.PathValue("path"), r.URL.RawQuery)
	if !ok {
		http.Error(w, "distribution has no usable origin", http.StatusServiceUnavailable)

		return
	}

	if !isCacheableMethod(r.Method) {
		s.passthrough(w, r, originURL)

		return
	}

	cfg, ok := edgeCacheConfig(dist)
	if !ok {
		http.Error(w, "distribution missing DefaultCacheBehavior", http.StatusServiceUnavailable)

		return
	}

	base := cache.Key(r, nil)
	if entry, hit := s.edgeCache.lookup(distID, base, r); hit {
		if served := serveIfFresh(w, entry); served {
			return
		}
	}

	upstream, err := fetchOrigin(originURL, r)
	if err != nil {
		http.Error(w, "origin fetch failed: "+err.Error(), http.StatusBadGateway)

		return
	}

	storeIfCacheable(s.edgeCache, distID, base, r, upstream, cfg)
	writeUpstream(w, upstream, "Miss from kumo", 0)
}

// isCacheableMethod returns true for the HTTP methods CloudFront ever
// considers caching. Everything else passes through.
func isCacheableMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead
}

// isHopByHopHeader reports whether the given header is per-hop and
// must not be propagated through a proxy (RFC 7230 §6.1).
func isHopByHopHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
		"Host":
		return true
	}

	return false
}

// passthrough forwards the request body verbatim, returns the response
// without touching the cache. Used for PUT / POST / DELETE / PATCH.
func (s *Service) passthrough(w http.ResponseWriter, r *http.Request, originURL string) {
	upstream, err := forwardOrigin(originURL, r)
	if err != nil {
		http.Error(w, "origin fetch failed: "+err.Error(), http.StatusBadGateway)

		return
	}

	writeUpstream(w, upstream, "Bypass from kumo", 0)
}

// edgeCacheConfig pulls the [MinTTL, DefaultTTL, MaxTTL] triple out of
// a Distribution's DefaultCacheBehavior. Returns ok=false when the
// distribution config isn't filled in (terraform almost always does).
func edgeCacheConfig(dist *Distribution) (cache.DistributionConfig, bool) {
	if dist == nil || dist.DistributionConfig == nil || dist.DistributionConfig.DefaultCacheBehavior == nil {
		return cache.DistributionConfig{}, false
	}

	dcb := dist.DistributionConfig.DefaultCacheBehavior

	return cache.DistributionConfig{
		MinTTL:     time.Duration(dcb.MinTTL) * time.Second,
		DefaultTTL: time.Duration(dcb.DefaultTTL) * time.Second,
		MaxTTL:     time.Duration(dcb.MaxTTL) * time.Second,
	}, true
}

// edgeOriginURL builds the upstream URL: `<scheme>://<DomainName><OriginPath><path>?<query>`.
// CustomOriginConfig.HTTPPort is honoured when the origin is HTTP.
func edgeOriginURL(dist *Distribution, path, rawQuery string) (string, bool) {
	if dist.DistributionConfig == nil || dist.DistributionConfig.Origins == nil {
		return "", false
	}

	origins := dist.DistributionConfig.Origins.Items
	if len(origins) == 0 {
		return "", false
	}

	o := origins[0]

	scheme := "https"
	host := o.DomainName

	if o.CustomOriginConfig != nil {
		switch o.CustomOriginConfig.OriginProtocolPolicy {
		case "http-only":
			scheme = "http"

			if o.CustomOriginConfig.HTTPPort > 0 && o.CustomOriginConfig.HTTPPort != 80 {
				host = o.DomainName + ":" + strconv.Itoa(o.CustomOriginConfig.HTTPPort)
			}
		case "https-only", "match-viewer":
			scheme = "https"
		}
	}

	full := scheme + "://" + host + o.OriginPath + "/" + path
	if rawQuery != "" {
		full += "?" + rawQuery
	}

	return full, true
}

// originResponse is the buffered subset of an http.Response that the
// edge handler needs after fetchOrigin returns. Returning a struct
// instead of *http.Response keeps the body lifecycle entirely inside
// fetchOrigin, so bodyclose / lostcancel linters stay happy.
type originResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// fetchOrigin sends a body-less request upstream (GET/HEAD) and
// returns the buffered response. We need the body twice (once to
// serve, once to cache), so it's read into memory here.
func fetchOrigin(target string, r *http.Request) (*originResponse, error) {
	return originRequest(target, r, http.NoBody)
}

// forwardOrigin proxies a non-cacheable request (PUT/POST/DELETE/PATCH)
// with its body intact. The cache is not consulted.
func forwardOrigin(target string, r *http.Request) (*originResponse, error) {
	return originRequest(target, r, r.Body)
}

// originRequest is the shared upstream request path used by both
// fetchOrigin and forwardOrigin.
func originRequest(target string, r *http.Request, reqBody io.Reader) (*originResponse, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("parse origin URL: %w", err)
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, parsed.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("build origin request: %w", err)
	}

	// Forward all request headers except hop-by-hop. CloudFront's
	// real forwarding policy is configurable — we keep it permissive
	// so the test surface (Test-ID / Req-Num / If-Modified-Since /
	// Cache-Control / etc) reaches the origin verbatim.
	for k, vs := range r.Header {
		if isHopByHopHeader(k) {
			continue
		}

		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("origin Do: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read origin body: %w", err)
	}

	return &originResponse{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: body}, nil
}

// storeIfCacheable evaluates the response against the cache rules and
// persists it as a variant of the (distId, base) bucket.
func storeIfCacheable(c *edgeCache, distID, base string, r *http.Request, resp *originResponse, cfg cache.DistributionConfig) {
	if cache.VaryDisablesCache(resp.Header) {
		return
	}

	cacheable, _ := cache.IsCacheable(resp.Header, resp.StatusCode)
	if !cacheable {
		return
	}

	ttl := cache.EffectiveTTL(resp.Header, cfg, time.Now())
	if ttl == 0 {
		return
	}

	vary := cache.VaryHeaders(resp.Header)
	values := make(map[string]string, len(vary))

	for _, name := range vary {
		values[name] = r.Header.Get(name)
	}

	c.store(distID, base, &cacheEntry{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       append([]byte(nil), resp.Body...),
		StoredAt:   time.Now(),
		TTL:        ttl,
		Vary:       vary,
		VaryValues: values,
	})
}

// serveIfFresh writes the cached entry when it's still within TTL.
// Returns true when something was written; false when the caller
// should fall through to an origin fetch.
func serveIfFresh(w http.ResponseWriter, entry *cacheEntry) bool {
	age := time.Since(entry.StoredAt)
	if age >= entry.TTL {
		return false
	}

	if cache.MustRevalidate(entry.Header) {
		return false
	}

	for k, vs := range entry.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	w.Header().Set("X-Cache", "Hit from kumo")
	w.Header().Set("Age", strconv.FormatInt(int64(age.Seconds()), 10))
	w.WriteHeader(entry.StatusCode)
	_, _ = w.Write(entry.Body)

	return true
}

// writeUpstream copies the upstream response body verbatim to the
// client, tagging it with the chosen X-Cache marker.
func writeUpstream(w http.ResponseWriter, resp *originResponse, cacheTag string, age time.Duration) {
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	w.Header().Set("X-Cache", cacheTag)

	if age > 0 {
		w.Header().Set("Age", strconv.FormatInt(int64(age.Seconds()), 10))
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}
