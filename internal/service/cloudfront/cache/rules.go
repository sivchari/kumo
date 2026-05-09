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
//  1. If the origin sends `Cache-Control: s-maxage=N`, use N.
//  2. Otherwise if `Cache-Control: max-age=N`, use N.
//  3. Otherwise if `Expires: <date>` is in the future, use that delta.
//  4. Otherwise fall back to the distribution's DefaultTTL.
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
// distinction belongs to MustRevalidate, not the TTL.
func EffectiveTTL(respHeader http.Header, cfg DistributionConfig, now time.Time) time.Duration {
	cc := parseControl(respHeader.Get("Cache-Control"))
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
		if exp := respHeader.Get("Expires"); exp != "" {
			if t, err := http.ParseTime(exp); err == nil {
				delta := t.Sub(now)
				if delta > 0 {
					ttl = delta
				}
			}
		}
	}

	return clampTTL(ttl, cfg)
}

// IsCacheable mirrors the CloudFront decision tree for "can the
// response be put in the cache at all". Returns (false, reason) when
// the response must not be stored. Reason is human-readable for
// surfacing in X-Cache-Reason or logs.
func IsCacheable(respHeader http.Header, statusCode int) (bool, string) {
	cc := parseControl(respHeader.Get("Cache-Control"))
	if cc.NoStore {
		return false, "Cache-Control: no-store"
	}

	if cc.Private {
		return false, "Cache-Control: private"
	}

	// CloudFront caches a fixed set of status codes by default.
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
		return true, ""
	default:
		return false, "status " + strconv.Itoa(statusCode) + " is not cacheable by default"
	}
}

// MustRevalidate reports whether a cached entry must be revalidated
// with the origin before being served, even if it's still fresh by
// TTL. CloudFront treats `Cache-Control: no-cache` and `must-revalidate`
// the same way at the edge.
func MustRevalidate(respHeader http.Header) bool {
	cc := parseControl(respHeader.Get("Cache-Control"))

	return cc.NoCache || cc.MustRevalidate
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
	if err != nil || n <= 0 {
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
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				cc.MaxAge = &v
			}
		case "s-maxage":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				cc.SMaxAge = &v
			}
		}
	}

	return cc
}
