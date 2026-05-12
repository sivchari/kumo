package cloudfront

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/service/cloudfront/cache"
)

const maxEdgeCacheEntries = 1024

// cacheEntry is one cached response variant for a distribution.
type cacheEntry struct {
	StatusCode           int
	Header               http.Header
	Body                 []byte
	StoredAt             time.Time
	InitialAge           time.Duration // Age the upstream reported on store
	TTL                  time.Duration
	StaleWhileRevalidate time.Duration     // window after TTL during which a stale serve+async revalidate is allowed (CDN-Cache-Control only)
	StaleIfError         time.Duration     // window after TTL during which an origin error allows serving stale (CDN-Cache-Control only)
	Vary                 []string          // header names that pinned this variant
	VaryValues           map[string]string // request values seen at store time
	revalidating         bool              // guards against duplicate background revalidations
	revalidateMu         sync.Mutex        // protects `revalidating`
}

// age is the entry's current age — RFC 9111 §5.1: time we've held it
// + Age the origin reported when we stored it.
func (e *cacheEntry) age() time.Duration {
	return time.Since(e.StoredAt) + e.InitialAge
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
	c.evictOldestLocked(maxEdgeCacheEntries)
}

func (c *edgeCache) evictOldestLocked(maxEntries int) {
	if maxEntries <= 0 {
		return
	}

	total := 0
	oldestDistID := ""
	oldestBase := ""
	oldestIndex := -1
	oldestStoredAt := time.Time{}

	for distID, dm := range c.entries {
		for base, variants := range dm {
			for i, entry := range variants {
				total++

				if oldestIndex == -1 || entry.StoredAt.Before(oldestStoredAt) {
					oldestDistID = distID
					oldestBase = base
					oldestIndex = i
					oldestStoredAt = entry.StoredAt
				}
			}
		}
	}

	if total <= maxEntries || oldestIndex == -1 {
		return
	}

	variants := c.entries[oldestDistID][oldestBase]
	variants = append(variants[:oldestIndex], variants[oldestIndex+1:]...)

	if len(variants) == 0 {
		delete(c.entries[oldestDistID], oldestBase)
	} else {
		c.entries[oldestDistID][oldestBase] = variants
	}

	if len(c.entries[oldestDistID]) == 0 {
		delete(c.entries, oldestDistID)
	}
}

// matchesVary reports whether the request's headers for the entry's
// recorded Vary list equal the ones it was stored with.
//
// Comparison is normalised: leading/trailing whitespace stripped,
// internal whitespace runs collapsed, comma-separated tokens trimmed
// individually. Case sensitivity follows the field-line value rule
// (Accept-Language / Accept-Encoding are tokens — case-insensitive;
// others fall back to byte equality after whitespace normalisation).
func matchesVary(e *cacheEntry, r *http.Request) bool {
	if len(e.Vary) == 0 {
		return true
	}

	for _, name := range e.Vary {
		if !varyValueEqual(name, r.Header.Get(name), e.VaryValues[name]) {
			return false
		}
	}

	return true
}

// varyValueEqual compares two values for the named Vary header after
// normalisation.
func varyValueEqual(name, a, b string) bool {
	an := normaliseVaryValue(name, a)
	bn := normaliseVaryValue(name, b)

	return an == bn
}

// normaliseVaryValue trims and collapses internal whitespace. For
// token-list headers (Accept-Encoding / Accept-Language) it also
// lowercases — those headers' grammars treat values case-insensitively.
func normaliseVaryValue(name, value string) string {
	out := collapseWhitespace(strings.TrimSpace(value))

	switch strings.ToLower(name) {
	case "accept-encoding", "accept-language":
		// Per RFC 9110, content-coding / language-tag tokens are
		// case-insensitive.
		out = strings.ToLower(out)
	}

	return out
}

// collapseWhitespace replaces runs of HTAB/SP with a single SP and
// trims spaces around comma separators.
func collapseWhitespace(s string) string {
	var b strings.Builder

	prevSpace := false

	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteByte(' ')

				prevSpace = true
			}

			continue
		}

		prevSpace = false

		b.WriteRune(r)
	}

	out := b.String()
	out = strings.ReplaceAll(out, " ,", ",")
	out = strings.ReplaceAll(out, ", ", ",")

	return out
}

// sameVarySignature checks two entries declare the same Vary headers
// AND equivalent values (used during store to overwrite stale
// variants).
func sameVarySignature(a, b *cacheEntry) bool {
	if len(a.Vary) != len(b.Vary) {
		return false
	}

	for i, name := range a.Vary {
		if name != b.Vary[i] {
			return false
		}

		if !varyValueEqual(name, a.VaryValues[name], b.VaryValues[name]) {
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
//     to the origin without consulting or storing the cache.
//  3. For safe methods (GET / HEAD), look up the cache.
//  4. If fresh:
//     - client conditional (If-None-Match / If-Modified-Since)
//     satisfied by the cached entry → 304 Not Modified
//     - Range request → 206 Partial Content from the cached body
//     - otherwise → 200 with `X-Cache: Hit from kumo` and `Age`
//  5. If stale or MustRevalidate: revalidate with the origin using
//     conditional headers built from the cached validators. 304 from
//     origin extends the entry's freshness; 200 replaces it.
//  6. On a true miss: fetch, evaluate cacheability + TTL via `cache/`,
//     store when allowed, and serve with `X-Cache: Miss from kumo`.
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

	clientCC := cache.ParseRequestCacheControl(r.Header)
	base := cache.Key(r, nil)

	// Request `no-store` bypasses the cache on both serve and store.
	if !clientCC.NoStore && s.tryServeFromCache(w, r, distID, base, originURL, cfg, clientCC) {
		return
	}

	if clientCC.OnlyIfCached {
		http.Error(w, "only-if-cached: not in cache", http.StatusGatewayTimeout)

		return
	}

	upstream, err := fetchOrigin(originURL, r)
	if err != nil {
		http.Error(w, "origin fetch failed: "+err.Error(), http.StatusBadGateway)

		return
	}

	if !clientCC.NoStore {
		storeIfCacheable(s.edgeCache, distID, base, r, upstream, cfg)
	}

	writeUpstream(w, upstream, "Miss from kumo", 0)
}

// tryServeFromCache looks up the cache and either serves the entry,
// triggers a revalidation, or reports back that the caller must do a
// fresh origin fetch. Returns true when the response has already been
// written.
func (s *Service) tryServeFromCache(w http.ResponseWriter, r *http.Request, distID, base, originURL string, cfg cache.DistributionConfig, clientCC cache.RequestDirectives) bool {
	entry, hit := s.edgeCache.lookup(distID, base, r)
	if !hit {
		return false
	}

	age := entry.age()
	serverForcesRevalidate := cache.MustRevalidate(entry.Header)
	clientDecision := cache.EvaluateClient(clientCC, age, entry.TTL)

	if !serverForcesRevalidate && clientDecision.Servable {
		serveFromCache(w, r, entry, age)

		return true
	}

	// Stale-while-revalidate: per CloudFront's CDN-Cache-Control
	// reading of RFC 9213, a stale entry within the SWR window may
	// be served immediately if the client allows it. A background
	// revalidation refreshes the cache for the next request.
	staleness := age - entry.TTL
	if staleness > 0 && staleness <= entry.StaleWhileRevalidate && !serverForcesRevalidate && !clientCC.NoCache {
		s.kickBackgroundRevalidate(distID, base, r, entry, originURL, cfg)
		serveFromCache(w, r, entry, age)

		return true
	}

	if clientDecision.Revalidate || serverForcesRevalidate {
		return s.revalidate(w, r, distID, base, entry, originURL, cfg)
	}

	return false
}

// kickBackgroundRevalidate launches a goroutine that revalidates the
// entry against the origin, deduplicating concurrent attempts on the
// same key via a per-entry mutex flag. The current request is served
// stale by the caller; this goroutine just keeps the cache fresh for
// the next one.
func (s *Service) kickBackgroundRevalidate(distID, base string, r *http.Request, entry *cacheEntry, originURL string, cfg cache.DistributionConfig) {
	entry.revalidateMu.Lock()
	if entry.revalidating {
		entry.revalidateMu.Unlock()

		return
	}

	entry.revalidating = true
	entry.revalidateMu.Unlock()

	// Detach the request from the response writer's lifecycle —
	// the original handler returns immediately after we serve stale.
	cloned := r.Clone(context.Background())

	go func() {
		defer func() {
			entry.revalidateMu.Lock()
			entry.revalidating = false
			entry.revalidateMu.Unlock()
		}()

		cond := cache.ConditionalHeaders(entry.Header)
		if len(cond) == 0 {
			return
		}

		upstream, err := revalidateOrigin(originURL, cloned, cond)
		if err != nil {
			return
		}

		if upstream.StatusCode == http.StatusNotModified {
			s.edgeCache.store(distID, base, refreshedEntry(entry, upstream.Header, cfg, time.Now()))

			return
		}

		// 200 — replace via the normal store path.
		storeIfCacheable(s.edgeCache, distID, base, cloned, upstream, cfg)
	}()
}

// serveFromCache writes the cached entry, honouring client
// preconditions (If-None-Match / If-Modified-Since → 304) and Range
// requests (→ 206 Partial Content) before falling back to the full
// 200 response.
func serveFromCache(w http.ResponseWriter, r *http.Request, entry *cacheEntry, age time.Duration) {
	if cache.IfNoneMatchSatisfied(r.Header, entry.Header) || cache.IfModifiedSinceSatisfied(r.Header, entry.Header) {
		writeNotModified(w, entry, age)

		return
	}

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		start, end, ok := cache.ParseRange(rangeHeader, int64(len(entry.Body)))
		if ok {
			writePartialContent(w, entry, start, end, age)

			return
		}
	}

	writeFullCached(w, entry, age)
}

// revalidate sends a conditional request to the origin. Returns true
// when it served a response (either 304-refreshed cache or 200-replaced
// cache), false when the caller should fall through to a normal miss
// fetch (e.g. the cached entry has no validators).
func (s *Service) revalidate(w http.ResponseWriter, r *http.Request, distID, base string, entry *cacheEntry, originURL string, cfg cache.DistributionConfig) bool {
	cond := cache.ConditionalHeaders(entry.Header)
	if len(cond) == 0 {
		return false
	}

	upstream, err := revalidateOrigin(originURL, r, cond)
	if err != nil {
		http.Error(w, "origin revalidate failed: "+err.Error(), http.StatusBadGateway)

		return true
	}

	if upstream.StatusCode == http.StatusNotModified {
		// Refresh: keep the cached body, update headers + reset TTL.
		refreshed := refreshedEntry(entry, upstream.Header, cfg, time.Now())
		s.edgeCache.store(distID, base, refreshed)

		serveFromCache(w, r, refreshed, 0)

		return true
	}

	// 200 (or anything else) — replace the cache entry.
	storeIfCacheable(s.edgeCache, distID, base, r, upstream, cfg)
	writeUpstream(w, upstream, "Miss from kumo", 0)

	return true
}

// refreshedEntry returns a replacement cache entry with the headers a
// 304 response brought back. RFC 9111 Section 4.3.4 says the cache
// replaces validators / cache directives but keeps the rest.
func refreshedEntry(entry *cacheEntry, fresh http.Header, cfg cache.DistributionConfig, now time.Time) *cacheEntry {
	header := entry.Header.Clone()

	for _, h := range []string{"Cache-Control", "ETag", "Last-Modified", "Expires", "Vary", "Date"} {
		if v := fresh.Get(h); v != "" {
			header.Set(h, v)
		}
	}

	vary := append([]string(nil), entry.Vary...)
	varyValues := make(map[string]string, len(entry.VaryValues))

	for k, v := range entry.VaryValues {
		varyValues[k] = v
	}

	return &cacheEntry{
		StatusCode:           entry.StatusCode,
		Header:               header,
		Body:                 append([]byte(nil), entry.Body...),
		StoredAt:             now,
		InitialAge:           entry.InitialAge,
		TTL:                  cache.EffectiveTTL(header, cfg, now),
		StaleWhileRevalidate: entry.StaleWhileRevalidate,
		StaleIfError:         entry.StaleIfError,
		Vary:                 vary,
		VaryValues:           varyValues,
	}
}

// writeNotModified writes a 304 with only the headers RFC 9111 §4.1
// permits to accompany it.
func writeNotModified(w http.ResponseWriter, entry *cacheEntry, age time.Duration) {
	for _, h := range []string{"ETag", "Last-Modified", "Cache-Control", "Date", "Content-Location", "Vary"} {
		if v := entry.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}

	w.Header().Set("X-Cache", "Hit from kumo")

	if age > 0 {
		w.Header().Set("Age", strconv.FormatInt(int64(age.Seconds()), 10))
	}

	w.WriteHeader(http.StatusNotModified)
}

// writePartialContent writes a 206 from the cached body slice.
func writePartialContent(w http.ResponseWriter, entry *cacheEntry, start, end int64, age time.Duration) {
	for k, vs := range entry.Header {
		// Skip Content-Length — recomputed below from the slice.
		if http.CanonicalHeaderKey(k) == "Content-Length" {
			continue
		}

		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	length := end - start + 1

	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(entry.Body)))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("X-Cache", "Hit from kumo")

	if age > 0 {
		w.Header().Set("Age", strconv.FormatInt(int64(age.Seconds()), 10))
	}

	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(entry.Body[start : end+1])
}

// writeFullCached is the original 200-from-cache path.
func writeFullCached(w http.ResponseWriter, entry *cacheEntry, age time.Duration) {
	for k, vs := range entry.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	w.Header().Set("X-Cache", "Hit from kumo")
	w.Header().Set("Age", strconv.FormatInt(int64(age.Seconds()), 10))
	w.WriteHeader(entry.StatusCode)
	_, _ = w.Write(entry.Body)
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

// edgeOriginURL builds the upstream URL for the request, choosing the
// origin pinned by `DefaultCacheBehavior.TargetOriginID` and falling
// back to the first registered origin when the ID doesn't resolve.
//
// Two origin shapes are honoured:
//
//   - **CustomOriginConfig**: arbitrary HTTP(S) origin.
//     `<scheme>://<DomainName>[:<HTTPPort>]<OriginPath>/<path>`.
//   - **S3OriginConfig**: an AWS S3 bucket. The DomainName is the
//     virtual-hosted bucket DNS name (e.g.
//     `mybucket.s3.us-east-1.amazonaws.com`). At the edge we extract
//     the bucket name and target kumo's own S3 service in path-style
//     so the proxy stays inside this kumo (no real AWS round-trip).
//     The S3 base URL is `KUMO_S3_BACKEND` (default
//     `http://127.0.0.1:4566`).
func edgeOriginURL(dist *Distribution, path, rawQuery string) (string, bool) {
	o, ok := selectOrigin(dist)
	if !ok {
		return "", false
	}

	var full string

	switch {
	case o.S3OriginConfig != nil:
		full, ok = s3OriginURL(&o, path)
		if !ok {
			return "", false
		}
	case o.CustomOriginConfig != nil:
		full = customOriginURL(&o, path)
	default:
		// No config block — assume HTTPS to the bare DomainName.
		full = "https://" + o.DomainName + o.OriginPath + "/" + path
	}

	if rawQuery != "" {
		full += "?" + rawQuery
	}

	return full, true
}

// selectOrigin returns the origin pinned by the distribution's
// DefaultCacheBehavior.TargetOriginID, falling back to the first
// origin when the ID doesn't match.
func selectOrigin(dist *Distribution) (Origin, bool) {
	if dist.DistributionConfig == nil || dist.DistributionConfig.Origins == nil {
		return Origin{}, false
	}

	origins := dist.DistributionConfig.Origins.Items
	if len(origins) == 0 {
		return Origin{}, false
	}

	if dcb := dist.DistributionConfig.DefaultCacheBehavior; dcb != nil && dcb.TargetOriginID != "" {
		for _, o := range origins {
			if o.ID == dcb.TargetOriginID {
				return o, true
			}
		}
	}

	return origins[0], true
}

// customOriginURL builds the URL for an HTTP(S) custom origin.
func customOriginURL(o *Origin, path string) string {
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

	return scheme + "://" + host + o.OriginPath + "/" + path
}

// s3OriginURL points the request at kumo's own S3 service in
// path-style so the edge proxy stays in-process. Bucket name comes
// from the DomainName's leftmost label; everything to the right of
// the first dot is discarded.
//
// Example:
//
//	DomainName = "mybucket.s3.us-east-1.amazonaws.com"
//	  → bucket = "mybucket"
//	  → URL    = "<KUMO_S3_BACKEND>/mybucket<OriginPath>/<path>"
//
// Returns ok=false when the bucket can't be derived.
func s3OriginURL(o *Origin, path string) (string, bool) {
	bucket := s3BucketFromDomain(o.DomainName)
	if bucket == "" {
		return "", false
	}

	base := os.Getenv("KUMO_S3_BACKEND")
	if base == "" {
		base = "http://127.0.0.1:4566"
	}

	return strings.TrimRight(base, "/") + "/" + bucket + o.OriginPath + "/" + path, true
}

// s3BucketFromDomain extracts the bucket name from a virtual-hosted
// S3 DomainName. Accepts forms like
// `<bucket>.s3.amazonaws.com`, `<bucket>.s3.<region>.amazonaws.com`,
// `<bucket>.s3-<region>.amazonaws.com`. Returns "" when the input
// isn't recognisably an S3 host.
func s3BucketFromDomain(domain string) string {
	dot := strings.Index(domain, ".")
	if dot <= 0 {
		return ""
	}

	bucket := domain[:dot]
	rest := domain[dot+1:]

	if !strings.HasPrefix(rest, "s3.") &&
		!strings.HasPrefix(rest, "s3-") &&
		rest != "s3.amazonaws.com" {
		return ""
	}

	return bucket
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

// revalidateOrigin sends a body-less GET/HEAD with the supplied
// conditional headers attached. Used for stale-entry refresh; if the
// origin returns 304 the cache extends the existing entry, otherwise
// it replaces it.
func revalidateOrigin(target string, r *http.Request, conditional http.Header) (*originResponse, error) {
	clone := r.Clone(r.Context())

	// Drop client-supplied conditionals so the cache's own validators
	// drive the revalidation. RFC 9111 §4.3.1 sees these as the
	// cache's responsibility.
	clone.Header.Del("If-None-Match")
	clone.Header.Del("If-Modified-Since")

	for k, vs := range conditional {
		for _, v := range vs {
			clone.Header.Set(k, v)
		}
	}

	return originRequest(target, clone, http.NoBody)
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

	stale := cache.ReadCDNStaleDirectives(resp.Header)

	c.store(distID, base, &cacheEntry{
		StatusCode:           resp.StatusCode,
		Header:               resp.Header.Clone(),
		Body:                 append([]byte(nil), resp.Body...),
		StoredAt:             time.Now(),
		InitialAge:           cache.InitialAge(resp.Header),
		TTL:                  ttl,
		StaleWhileRevalidate: stale.StaleWhileRevalidate,
		StaleIfError:         stale.StaleIfError,
		Vary:                 vary,
		VaryValues:           values,
	})
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
