package apigateway

import (
	"sync"
	"time"
)

const (
	// defaultAuthorizerResultTTL is the TTL AWS applies when
	// authorizerResultTtlInSeconds is not set; 0 disables caching entirely.
	defaultAuthorizerResultTTL int32 = 300
	// maxAuthorizerResultTTL is the largest TTL AWS accepts (one hour).
	maxAuthorizerResultTTL int32 = 3600
)

// cacheTTL returns how long this authorizer's results stay cached. Zero means
// caching is disabled and every request invokes the function.
func (a *Authorizer) cacheTTL() time.Duration {
	return time.Duration(a.AuthorizerResultTTLInSeconds) * time.Second
}

// authorizerCache holds authorizer responses for their configured TTL. The
// cached value is the function's raw response, so the policy it carries is
// re-evaluated against the current request's method ARN rather than a decision
// being frozen — matching AWS, where a cached policy must cover every method of
// the API.
type authorizerCache struct {
	mu      sync.Mutex
	entries map[string]authorizerCacheEntry
}

// authorizerCacheEntry is one cached authorizer response and its deadline.
type authorizerCacheEntry struct {
	payload   []byte
	expiresAt time.Time
}

// get returns the cached response for key while it is still live.
func (c *authorizerCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if !time.Now().Before(entry.expiresAt) {
		delete(c.entries, key)

		return nil, false
	}

	return entry.payload, true
}

// put caches payload under key for ttl. A non-positive ttl caches nothing.
func (c *authorizerCache) put(key string, payload []byte, ttl time.Duration) {
	if ttl <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.entries == nil {
		c.entries = make(map[string]authorizerCacheEntry)
	}

	c.entries[key] = authorizerCacheEntry{payload: payload, expiresAt: time.Now().Add(ttl)}
}
