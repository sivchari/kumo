package cloudfront

import (
	"strings"
	"unicode/utf8"
)

// resolveBehavior picks the cache behaviour for a request path the way
// CloudFront does: the first ordered behaviour whose PathPattern matches, in
// list order, otherwise the default behaviour (nil when the distribution has
// none). path is the request path without its leading slash.
func resolveBehavior(dist *Distribution, path string) *DefaultCacheBehavior {
	if dist == nil || dist.DistributionConfig == nil {
		return nil
	}

	if behaviors := dist.DistributionConfig.CacheBehaviors; behaviors != nil {
		for i := range behaviors.Items {
			if matchPathPattern(behaviors.Items[i].PathPattern, path) {
				return &behaviors.Items[i].DefaultCacheBehavior
			}
		}
	}

	return dist.DistributionConfig.DefaultCacheBehavior
}

// matchPathPattern applies a CloudFront path pattern to a request path: `*`
// matches zero or more characters (including `/` and newlines), `?` exactly
// one, the comparison is case-sensitive, a leading `/` on the pattern is
// optional and the query string is never part of the path.
func matchPathPattern(pattern, path string) bool {
	pattern = strings.TrimPrefix(pattern, "/")

	var (
		p, s         int
		starP, starS = -1, 0
	)

	for s < len(path) {
		switch {
		case p < len(pattern) && pattern[p] == '*':
			starP, starS = p, s
			p++

			continue
		case p < len(pattern) && pattern[p] == '?':
			_, size := utf8.DecodeRuneInString(path[s:])
			p++
			s += size

			continue
		case p < len(pattern) && pattern[p] == path[s]:
			p++
			s++

			continue
		}

		if starP < 0 {
			return false
		}

		// Let the last `*` swallow one more character and retry after it.
		_, size := utf8.DecodeRuneInString(path[starS:])
		starS += size
		s = starS
		p = starP + 1
	}

	return strings.TrimLeft(pattern[p:], "*") == ""
}
