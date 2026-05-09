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
