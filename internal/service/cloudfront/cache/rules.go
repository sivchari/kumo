// Package cache implements the cache rule evaluation CloudFront
// performs at the edge, per RFC 7234 + the CloudFront-specific
// extensions documented at:
//
//	https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/Expiration.html
//
// The package is pure — no HTTP, no storage. The edge handler in
// edge.go applies these rules to live requests/responses.
package cache

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DistributionConfig is the subset of CloudFront's CacheBehavior that
// the rule evaluation needs. Real DefaultCacheBehavior carries more,
// but TTL clamping only depends on these three values.
type DistributionConfig struct {
	MinTTL     time.Duration
	DefaultTTL time.Duration
	MaxTTL     time.Duration
}

// EffectiveTTL implements the CloudFront precedence:
//
//  1. `CDN-Cache-Control` (RFC 9213) — CDN-targeted directives win
//     over the Cache-Control browsers see. CloudFront supports it.
//  2. If the origin sends `Cache-Control: s-maxage=N`, use N.
//  3. Otherwise if `Cache-Control: max-age=N`, use N.
//  4. Otherwise if `Expires: <date>` is in the future, use that delta.
//  5. Otherwise fall back to the distribution's DefaultTTL.
//
// The result is then clamped to [MinTTL, MaxTTL].
//
// Special cases that override the above:
//
//   - `Cache-Control: no-store` → returns 0 (do not cache).
//   - `Cache-Control: private`  → returns 0 (CloudFront treats this as
//     "shared cache must not store").
//
// `no-cache` is **not** zero — it caches but always revalidates; that
// distinction belongs to NoCacheDirective, not the TTL.
func EffectiveTTL(respHeader http.Header, cfg DistributionConfig, now time.Time) time.Duration {
	cc := mergedCacheControl(respHeader)
	if cc.NoStore || cc.Private {
		return 0
	}

	ttl := cfg.DefaultTTL

	switch {
	case cc.SMaxAge != nil:
		ttl = time.Duration(*cc.SMaxAge) * time.Second
	case cc.MaxAge != nil:
		ttl = time.Duration(*cc.MaxAge) * time.Second
	default:
		switch {
		case respHeader.Get("Expires") != "":
			ttl = expiresTTL(respHeader, now)
		case respHeader.Get("Last-Modified") != "":
			// RFC 9111 §4.2.2 heuristic freshness: 10% of the
			// last-modified-to-now interval, capped at the cache's
			// configured max so a years-old asset doesn't get a
			// years-long lifetime.
			ttl = heuristicTTL(respHeader, now)
		}
	}

	return clampTTL(ttl, cfg)
}

// expiresTTL evaluates the Expires header relative to `now`. Per RFC
// 9111 §5.3 a valid Expires is explicit freshness — even when the
// delta is non-positive (treat as already-stale).
func expiresTTL(respHeader http.Header, now time.Time) time.Duration {
	t, err := http.ParseTime(respHeader.Get("Expires"))
	if err != nil {
		return 0
	}

	delta := t.Sub(now)
	if delta < 0 {
		return 0
	}

	return delta
}

// heuristicTTL implements the RFC 9111 §4.2.2 10% heuristic. The
// fraction is conservative — most real CDNs use this number too.
// We bound by `now - Date` so a faulty Last-Modified can't push the
// TTL past the response's own age.
func heuristicTTL(respHeader http.Header, now time.Time) time.Duration {
	lm, err := http.ParseTime(respHeader.Get("Last-Modified"))
	if err != nil {
		return 0
	}

	delta := now.Sub(lm)
	if delta <= 0 {
		return 0
	}

	return delta / 10
}

// mergedCacheControl reads CDN-Cache-Control (RFC 9213) when present,
// otherwise Cache-Control. CloudFront documents the precedence the
// same way: CDN-Cache-Control is parsed at the edge and trumps the
// Cache-Control the browser ultimately sees.
//
// Surrogate-Control (the older Akamai/Fastly variant) is intentionally
// not read — RFC 9213 standardised on CDN-Cache-Control.
func mergedCacheControl(respHeader http.Header) Control {
	if cdn := respHeader.Get("CDN-Cache-Control"); cdn != "" {
		return parseControl(cdn)
	}

	return parseControl(respHeader.Get("Cache-Control"))
}

// CDNStaleDirectives is the (stale-while-revalidate, stale-if-error)
// pair from a response. CloudFront only honours these when they come
// from CDN-Cache-Control — the Cache-Control header is the
// browser-targeted policy and intentionally has no influence on the
// CDN's stale-serving behaviour. RFC 9213 §3.1 endorses the same
// split.
//
// Returns zero durations when neither directive is set or
// CDN-Cache-Control is absent.
type CDNStaleDirectives struct {
	StaleWhileRevalidate time.Duration
	StaleIfError         time.Duration
}

// ReadCDNStaleDirectives parses CDN-Cache-Control for swr / sie. It
// intentionally ignores Cache-Control to match CloudFront's published
// behaviour.
func ReadCDNStaleDirectives(respHeader http.Header) CDNStaleDirectives {
	cdn := respHeader.Get("CDN-Cache-Control")
	if cdn == "" {
		return CDNStaleDirectives{}
	}

	var out CDNStaleDirectives

	for _, part := range strings.Split(cdn, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(strings.Trim(value, `"`))

		switch key {
		case "stale-while-revalidate":
			if v, ok := parseNonNegativeInt(value); ok {
				out.StaleWhileRevalidate = time.Duration(v) * time.Second
			}
		case "stale-if-error":
			if v, ok := parseNonNegativeInt(value); ok {
				out.StaleIfError = time.Duration(v) * time.Second
			}
		}
	}

	return out
}

// IsCacheable decides whether the response can be put in the cache
// at all. Returns (false, reason) when storage is forbidden.
//
// Per RFC 9111 §3: any status code may be cached when the response
// carries **explicit** freshness information (Cache-Control max-age /
// s-maxage / CDN-Cache-Control max-age, or an Expires header).
// Without explicit freshness only the "heuristically cacheable" set
// (RFC 9110 §15) is stored — that's the small list CloudFront
// defaults to.
func IsCacheable(respHeader http.Header, statusCode int) (bool, string) {
	cc := mergedCacheControl(respHeader)
	if cc.NoStore {
		return false, "Cache-Control: no-store"
	}

	if cc.Private {
		return false, "Cache-Control: private"
	}

	if hasExplicitFreshness(respHeader) {
		return true, ""
	}

	if isHeuristicallyCacheableStatus(statusCode) {
		return true, ""
	}

	return false, "status " + strconv.Itoa(statusCode) + " is not heuristically cacheable"
}

// hasExplicitFreshness reports whether the response advertises a
// freshness lifetime — max-age / s-maxage on Cache-Control or
// CDN-Cache-Control, or a *valid* Expires header. Used to decide
// whether non-heuristic statuses may be cached.
//
// An Expires header that doesn't parse as an HTTP-date is treated as
// already-expired per RFC 9111 §5.3 and therefore does not constitute
// explicit freshness.
func hasExplicitFreshness(respHeader http.Header) bool {
	cc := mergedCacheControl(respHeader)
	if cc.MaxAge != nil || cc.SMaxAge != nil {
		return true
	}

	if exp := respHeader.Get("Expires"); exp != "" {
		if _, err := http.ParseTime(exp); err == nil {
			return true
		}
	}

	return false
}

// isHeuristicallyCacheableStatus is the subset of status codes RFC
// 9110 §15 marks as "heuristically cacheable". Without explicit
// freshness the cache only stores these.
func isHeuristicallyCacheableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusOK,
		http.StatusNonAuthoritativeInfo,
		http.StatusNoContent,
		http.StatusPartialContent,
		http.StatusMultipleChoices,
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusGone,
		http.StatusRequestURITooLong,
		http.StatusNotImplemented:
		return true
	}

	return false
}

// MustRevalidate reports whether a **fresh** cached entry must still
// trigger origin revalidation before it can be served. RFC 9111
// §5.2.2 distinguishes:
//
//   - `no-cache` — yes, revalidate even when fresh.
//   - `must-revalidate` / `proxy-revalidate` — only matters once the
//     entry is stale (and even then it just forbids serving stale
//     while disconnected); fresh entries are unaffected.
//
// So this returns true iff the no-cache directive is present.
func MustRevalidate(respHeader http.Header) bool {
	return mergedCacheControl(respHeader).NoCache
}

// VaryHeaders extracts the comma-separated list of request headers
// the origin's `Vary` response header pins as part of the cache key.
// The names are lowercased and deduplicated for stable key building.
//
// `Vary: *` is special — it disables caching entirely; the caller
// should treat that as "do not cache" rather than feeding it here.
func VaryHeaders(respHeader http.Header) []string {
	raw := respHeader.Values("Vary")
	if len(raw) == 0 {
		return nil
	}

	seen := make(map[string]struct{})

	for _, line := range raw {
		for _, name := range strings.Split(line, ",") {
			name = strings.TrimSpace(strings.ToLower(name))
			if name != "" {
				seen[name] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

// VaryDisablesCache returns true when the origin sent `Vary: *`,
// which RFC 7234 says forbids any further caching.
func VaryDisablesCache(respHeader http.Header) bool {
	for _, line := range respHeader.Values("Vary") {
		for _, name := range strings.Split(line, ",") {
			if strings.TrimSpace(name) == "*" {
				return true
			}
		}
	}

	return false
}

// IfNoneMatchSatisfied reports whether the request's If-None-Match
// matches the cached entry's ETag. Per RFC 9110 §13.1.2:
//
//   - `*` matches any existing representation
//   - any listed entity-tag (weak or strong) matching the cached one returns true
//
// When this returns true, the cache should respond 304 Not Modified
// instead of the full body.
func IfNoneMatchSatisfied(reqHeader, respHeader http.Header) bool {
	inm := reqHeader.Get("If-None-Match")
	if inm == "" {
		return false
	}

	if strings.TrimSpace(inm) == "*" {
		return true
	}

	cached := strings.TrimSpace(respHeader.Get("ETag"))
	if cached == "" {
		return false
	}

	for _, raw := range strings.Split(inm, ",") {
		if etagsEqualWeak(strings.TrimSpace(raw), cached) {
			return true
		}
	}

	return false
}

// IfModifiedSinceSatisfied reports whether the cached entry's
// Last-Modified is at or before the request's If-Modified-Since.
// When true the cache should respond 304 (the client's copy is fresh
// enough).
func IfModifiedSinceSatisfied(reqHeader, respHeader http.Header) bool {
	ims := reqHeader.Get("If-Modified-Since")
	if ims == "" {
		return false
	}

	imsTime, err := http.ParseTime(ims)
	if err != nil {
		return false
	}

	lm := respHeader.Get("Last-Modified")
	if lm == "" {
		return false
	}

	lmTime, err := http.ParseTime(lm)
	if err != nil {
		return false
	}

	return !lmTime.After(imsTime)
}

// etagsEqualWeak compares two ETag values per RFC 9110 §8.8.3.2 weak
// comparison (ignoring the W/ prefix).
func etagsEqualWeak(a, b string) bool {
	a = strings.TrimPrefix(a, "W/")
	b = strings.TrimPrefix(b, "W/")

	return a == b && a != ""
}

// ConditionalHeaders builds the If-None-Match / If-Modified-Since
// header pair the cache should send on a revalidation request to the
// origin, derived from the cached entry's validators.
func ConditionalHeaders(cachedHeader http.Header) http.Header {
	out := http.Header{}

	if etag := cachedHeader.Get("ETag"); etag != "" {
		out.Set("If-None-Match", etag)
	}

	if lm := cachedHeader.Get("Last-Modified"); lm != "" {
		out.Set("If-Modified-Since", lm)
	}

	return out
}

// InitialAge parses the upstream's `Age:` response header. Used to
// pre-age a freshly-stored cache entry — the origin may have sat in
// an upstream cache for some time before reaching us, and RFC 9111
// §5.1 says we must carry that age forward.
//
// Returns 0 when the header is absent / malformed / negative.
func InitialAge(respHeader http.Header) time.Duration {
	raw := respHeader.Get("Age")
	if raw == "" {
		return 0
	}

	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || n < 0 {
		return 0
	}

	return time.Duration(n) * time.Second
}

// RequestDirectives holds the subset of `Cache-Control` directives a
// client may put on a *request* (RFC 9111 §5.2.1).
type RequestDirectives struct {
	NoCache      bool   // "always revalidate this hit before serving"
	NoStore      bool   // "do not store the response"
	OnlyIfCached bool   // "serve from cache or 504"
	MaxAge       *int64 // "I'll only accept entries whose age <= MaxAge"
	MinFresh     *int64 // "I want at least MinFresh seconds of remaining freshness"
	MaxStale     *int64 // "I'll accept stale entries up to MaxStale seconds past TTL"
	HasMaxStale  bool   // distinguishes `max-stale` (no value, means infinity) from absent
}

// ParseRequestCacheControl reads the request's Cache-Control header.
// Same parsing logic as the response side, but exposes the
// request-applicable directives via a separate type so callers don't
// confuse the two.
func ParseRequestCacheControl(reqHeader http.Header) RequestDirectives {
	raw := reqHeader.Get("Cache-Control")
	if raw == "" {
		return RequestDirectives{}
	}

	var d RequestDirectives

	for _, part := range strings.Split(raw, ",") {
		applyRequestDirective(&d, strings.TrimSpace(part))
	}

	return d
}

// applyRequestDirective folds one parsed directive into d. Pulled out
// of ParseRequestCacheControl to satisfy the cyclop budget — and to
// give the per-directive parsing somewhere to grow if we add more.
func applyRequestDirective(d *RequestDirectives, part string) {
	if part == "" {
		return
	}

	key, value, _ := strings.Cut(part, "=")
	key = strings.ToLower(strings.TrimSpace(key))
	value = strings.TrimSpace(strings.Trim(value, `"`))

	switch key {
	case "no-cache":
		d.NoCache = true
	case "no-store":
		d.NoStore = true
	case "only-if-cached":
		d.OnlyIfCached = true
	case "max-age":
		if v, ok := parseNonNegativeInt(value); ok {
			d.MaxAge = &v
		}
	case "min-fresh":
		if v, ok := parseNonNegativeInt(value); ok {
			d.MinFresh = &v
		}
	case "max-stale":
		d.HasMaxStale = true

		if v, ok := parseNonNegativeInt(value); ok {
			d.MaxStale = &v
		}
	}
}

// parseNonNegativeInt parses a base-10 int64 and returns ok=false on
// error or negative values.
func parseNonNegativeInt(s string) (int64, bool) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0, false
	}

	return v, true
}

// EvaluateClient decides whether a cached entry can be served given
// the request's Cache-Control directives. age is the entry's current
// age (`time.Since(StoredAt) + InitialAge`). ttl is the entry's
// effective TTL.
//
// Returns:
//
//	servable=true  → cached entry satisfies the request
//	revalidate=true → cache may serve only after origin revalidation

// ClientDecision is the verdict EvaluateClient returns.
type ClientDecision struct {
	Servable   bool
	Revalidate bool
	Reason     string
}

// EvaluateClient runs the request-side directives against an entry.
func EvaluateClient(req RequestDirectives, age, ttl time.Duration) ClientDecision {
	if req.NoCache {
		return ClientDecision{Revalidate: true, Reason: "request: no-cache"}
	}

	if req.MaxAge != nil && age > time.Duration(*req.MaxAge)*time.Second {
		return ClientDecision{Revalidate: true, Reason: "request: max-age exceeded"}
	}

	if req.MinFresh != nil {
		remaining := ttl - age
		if remaining < time.Duration(*req.MinFresh)*time.Second {
			return ClientDecision{Revalidate: true, Reason: "request: min-fresh not met"}
		}
	}

	stale := age >= ttl
	if stale && req.HasMaxStale {
		if req.MaxStale == nil || age-ttl <= time.Duration(*req.MaxStale)*time.Second {
			return ClientDecision{Servable: true, Reason: "request: max-stale honoured"}
		}
	}

	if stale {
		return ClientDecision{Revalidate: true, Reason: "stale"}
	}

	return ClientDecision{Servable: true}
}

// ParseRange interprets a single-range "Range: bytes=START-END"
// header against a known content length. Returns (start, end, true)
// where end is inclusive (matching the Content-Range wire format).
//
// Multi-range requests (`bytes=0-99,200-299`) and unsatisfiable
// ranges return ok=false — the cache should fall through to a full
// fetch / 416 in those cases. Suffix ranges (`bytes=-100`) and
// open-ended (`bytes=100-`) are supported.
func ParseRange(header string, totalSize int64) (int64, int64, bool) {
	startRaw, endRaw, ok := splitByteRangeSpec(header)
	if !ok {
		return 0, 0, false
	}

	switch {
	case startRaw == "" && endRaw == "":
		return 0, 0, false
	case startRaw == "":
		return parseSuffixRange(endRaw, totalSize)
	case endRaw == "":
		return parseOpenRange(startRaw, totalSize)
	default:
		return parseClosedRange(startRaw, endRaw, totalSize)
	}
}

// splitByteRangeSpec strips the "bytes=" prefix and splits on the
// dash. Returns ok=false on multi-range or wrong unit.
func splitByteRangeSpec(header string) (string, string, bool) {
	spec, ok := strings.CutPrefix(header, "bytes=")
	if !ok {
		return "", "", false
	}

	if strings.Contains(spec, ",") {
		return "", "", false
	}

	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return "", "", false
	}

	return strings.TrimSpace(spec[:dash]), strings.TrimSpace(spec[dash+1:]), true
}

// parseSuffixRange resolves `bytes=-N` (the last N bytes).
func parseSuffixRange(endRaw string, totalSize int64) (int64, int64, bool) {
	n, err := strconv.ParseInt(endRaw, 10, 64)
	if err != nil || n <= 0 || totalSize <= 0 {
		return 0, 0, false
	}

	if n > totalSize {
		n = totalSize
	}

	return totalSize - n, totalSize - 1, true
}

// parseOpenRange resolves `bytes=START-` (from START to end of
// content).
func parseOpenRange(startRaw string, totalSize int64) (int64, int64, bool) {
	start, err := strconv.ParseInt(startRaw, 10, 64)
	if err != nil || start < 0 || start >= totalSize {
		return 0, 0, false
	}

	return start, totalSize - 1, true
}

// parseClosedRange resolves `bytes=START-END`.
func parseClosedRange(startRaw, endRaw string, totalSize int64) (int64, int64, bool) {
	start, err1 := strconv.ParseInt(startRaw, 10, 64)
	end, err2 := strconv.ParseInt(endRaw, 10, 64)

	if err1 != nil || err2 != nil || start < 0 || end < start || start >= totalSize {
		return 0, 0, false
	}

	if end >= totalSize {
		end = totalSize - 1
	}

	return start, end, true
}

// Key builds the deterministic cache key for a request. The base
// is method+URL; adding the values of any Vary headers (lowercased,
// in stable order) yields the per-variant key.
//
// Query string handling matches CloudFront's "all" forwarded mode —
// the full sorted query string contributes to the key. Cookie /
// header forwarding beyond Vary is out of scope for this PR.
func Key(req *http.Request, vary []string) string {
	var b strings.Builder

	b.WriteString(req.Method)
	b.WriteByte(' ')
	b.WriteString(req.URL.Path)

	if q := req.URL.Query(); len(q) > 0 {
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		b.WriteByte('?')

		for i, k := range keys {
			if i > 0 {
				b.WriteByte('&')
			}

			vals := append([]string(nil), q[k]...)
			sort.Strings(vals)

			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(strings.Join(vals, ","))
		}
	}

	for _, name := range vary {
		b.WriteByte('|')
		b.WriteString(name)
		b.WriteByte('=')
		b.WriteString(req.Header.Get(name))
	}

	return b.String()
}

// clampTTL applies the CloudFront [MinTTL, MaxTTL] clamp.
func clampTTL(ttl time.Duration, cfg DistributionConfig) time.Duration {
	if ttl < cfg.MinTTL {
		return cfg.MinTTL
	}

	if cfg.MaxTTL > 0 && ttl > cfg.MaxTTL {
		return cfg.MaxTTL
	}

	return ttl
}

// Control is the parsed shape of a Cache-Control header.
// Pointers distinguish "absent" from "zero".
type Control struct {
	NoStore        bool
	NoCache        bool
	Private        bool
	Public         bool
	MustRevalidate bool
	MaxAge         *int64
	SMaxAge        *int64
}

// parseControl parses a Cache-Control header value. Unknown
// directives are ignored (per RFC 7234 §5.2 — receivers MUST ignore
// directives they don't recognise).
func parseControl(raw string) Control {
	var cc Control

	if raw == "" {
		return cc
	}

	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		key, value, _ := strings.Cut(part, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(strings.Trim(value, `"`))

		switch key {
		case "no-store":
			cc.NoStore = true
		case "no-cache":
			cc.NoCache = true
		case "private":
			cc.Private = true
		case "public":
			cc.Public = true
		case "must-revalidate":
			cc.MustRevalidate = true
		case "max-age":
			// RFC 9111 §5.2 leaves duplicate-directive handling
			// implementation-defined; cache-tests expects the first
			// successfully-parsed value to win.
			if cc.MaxAge == nil {
				if v, err := strconv.ParseInt(value, 10, 64); err == nil {
					cc.MaxAge = &v
				}
			}
		case "s-maxage":
			if cc.SMaxAge == nil {
				if v, err := strconv.ParseInt(value, 10, 64); err == nil {
					cc.SMaxAge = &v
				}
			}
		}
	}

	return cc
}
