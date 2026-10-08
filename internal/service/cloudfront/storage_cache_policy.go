package cloudfront

import (
	"context"
	"encoding/xml"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ensureCachePolicyInit lazily initialises the cache policy map, which a
// snapshot written before the feature existed does not contain.
func (s *MemoryStorage) ensureCachePolicyInit() {
	if s.CachePolicies == nil {
		s.CachePolicies = make(map[string]*CachePolicy)
	}
}

// CreateCachePolicy stores a new custom cache policy. Names are unique per
// account (CachePolicyAlreadyExists).
func (s *MemoryStorage) CreateCachePolicy(_ context.Context, cfg *CachePolicyConfig) (*CachePolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureCachePolicyInit()

	stored, err := normalizeCachePolicyConfig(cfg)
	if err != nil {
		return nil, err
	}

	if err := s.cachePolicyNameAvailableLocked(stored.Name, ""); err != nil {
		return nil, err
	}

	policy := &CachePolicy{ID: uuid.NewString(), ETag: generateETag(), LastModifiedTime: time.Now().UTC(), Config: *stored}
	s.CachePolicies[policy.ID] = policy

	s.saveLocked()

	return policy.clone(), nil
}

// GetCachePolicy returns a copy of the cache policy.
func (s *MemoryStorage) GetCachePolicy(_ context.Context, id string) (*CachePolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, err := s.cachePolicyLocked(id)
	if err != nil {
		return nil, err
	}

	return policy.clone(), nil
}

// ListCachePolicies returns the custom cache policies sorted by id, starting
// after marker and limited to maxItems; nextMarker is the last id returned
// when more remain. kumo has no AWS-managed policies, so the managed filter
// yields nothing.
func (s *MemoryStorage) ListCachePolicies(_ context.Context, policyType, marker string, maxItems int) ([]*CachePolicy, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxItems <= 0 {
		maxItems = cachePolicyListMaxItems
	}

	if policyType == cachePolicyTypeManaged {
		return nil, "", nil
	}

	all := make([]*CachePolicy, 0, len(s.CachePolicies))

	for _, policy := range s.CachePolicies {
		all = append(all, policy.clone())
	}

	page, nextMarker := pageByID(all, func(p *CachePolicy) string { return p.ID }, marker, maxItems)

	return page, nextMarker, nil
}

// UpdateCachePolicy replaces the configuration when ifMatch is the current
// ETag and the new name is not taken by another policy.
func (s *MemoryStorage) UpdateCachePolicy(_ context.Context, id string, cfg *CachePolicyConfig, ifMatch string) (*CachePolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	policy, err := s.cachePolicyForWriteLocked(id, ifMatch)
	if err != nil {
		return nil, err
	}

	stored, err := normalizeCachePolicyConfig(cfg)
	if err != nil {
		return nil, err
	}

	if err := s.cachePolicyNameAvailableLocked(stored.Name, id); err != nil {
		return nil, err
	}

	policy.Config = *stored
	policy.ETag = generateETag()
	policy.LastModifiedTime = time.Now().UTC()

	s.saveLocked()

	return policy.clone(), nil
}

// DeleteCachePolicy removes the cache policy when ifMatch is the current
// ETag and no cache behavior of any distribution, enabled or not,
// references it.
func (s *MemoryStorage) DeleteCachePolicy(_ context.Context, id, ifMatch string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.cachePolicyForWriteLocked(id, ifMatch); err != nil {
		return err
	}

	if distID, ok := s.distributionReferencingLocked(func(b *DefaultCacheBehavior) bool { return b.CachePolicyID == id }); ok {
		return &Error{Code: errCachePolicyInUse, Message: fmt.Sprintf("Cannot delete the cache policy because it is attached to one or more cache behaviors of distribution %s.", distID)}
	}

	delete(s.CachePolicies, id)

	s.saveLocked()

	return nil
}

func (s *MemoryStorage) cachePolicyLocked(id string) (*CachePolicy, error) {
	policy, ok := s.CachePolicies[id]
	if !ok {
		return nil, &Error{Code: errNoSuchCachePolicy, Message: "The cache policy does not exist."}
	}

	return policy, nil
}

// cachePolicyForWriteLocked applies the If-Match protocol: the header is
// mandatory (InvalidIfMatchVersion) and must carry the current ETag
// (PreconditionFailed).
func (s *MemoryStorage) cachePolicyForWriteLocked(id, ifMatch string) (*CachePolicy, error) {
	if err := requireIfMatch(ifMatch); err != nil {
		return nil, err
	}

	policy, err := s.cachePolicyLocked(id)
	if err != nil {
		return nil, err
	}

	if err := checkIfMatch(policy.ETag, ifMatch); err != nil {
		return nil, err
	}

	return policy, nil
}

// cachePolicyNameAvailableLocked rejects a name already used by another cache policy.
func (s *MemoryStorage) cachePolicyNameAvailableLocked(name, exceptID string) error {
	for _, other := range s.CachePolicies {
		if other.ID != exceptID && other.Config.Name == name {
			return &Error{Code: errCachePolicyAlreadyExists, Message: "A cache policy with this name already exists. You must provide a unique name."}
		}
	}

	return nil
}

// normalizeCachePolicyConfig validates a request configuration and returns
// the copy to store, with the documented DefaultTTL / MaxTTL defaults
// applied when they were omitted.
func normalizeCachePolicyConfig(cfg *CachePolicyConfig) (*CachePolicyConfig, error) {
	if err := validateCachePolicyConfig(cfg); err != nil {
		return nil, err
	}

	out := cfg.clone()
	out.XMLName = xml.Name{}
	out.Xmlns = ""

	if out.DefaultTTL == nil {
		defaultTTL := max(int64(cachePolicyDefaultTTL), *out.MinTTL)
		out.DefaultTTL = &defaultTTL
	}

	if out.MaxTTL == nil {
		maxTTL := max(int64(cachePolicyDefaultMaxTTL), *out.DefaultTTL)
		out.MaxTTL = &maxTTL
	}

	return out, nil
}

// validateCachePolicyConfig enforces the required members, the documented
// value sets and Quantity / Items consistency.
func validateCachePolicyConfig(cfg *CachePolicyConfig) error {
	switch {
	case cfg.Name == "":
		return invalidArgument("Name is required.")
	case cfg.MinTTL == nil:
		return invalidArgument("MinTTL is required.")
	case utf8.RuneCountInString(cfg.Comment) > cachePolicyCommentMaxLength:
		return invalidArgument(fmt.Sprintf("The comment cannot be longer than %d characters.", cachePolicyCommentMaxLength))
	}

	p := cfg.Parameters
	if p == nil {
		return nil
	}

	switch {
	case p.EnableAcceptEncodingGzip == nil:
		return invalidArgument("EnableAcceptEncodingGzip is required.")
	case p.HeadersConfig == nil:
		return invalidArgument("HeadersConfig is required.")
	case p.CookiesConfig == nil:
		return invalidArgument("CookiesConfig is required.")
	case p.QueryStringsConfig == nil:
		return invalidArgument("QueryStringsConfig is required.")
	case !slices.Contains([]string{cachePolicyBehaviorNone, cachePolicyBehaviorWhitelist}, p.HeadersConfig.HeaderBehavior):
		return invalidArgument("HeaderBehavior must be one of none, whitelist.")
	case !isCachePolicyListBehavior(p.CookiesConfig.CookieBehavior):
		return invalidArgument("CookieBehavior must be one of none, whitelist, allExcept, all.")
	case !isCachePolicyListBehavior(p.QueryStringsConfig.QueryStringBehavior):
		return invalidArgument("QueryStringBehavior must be one of none, whitelist, allExcept, all.")
	}

	for _, names := range []*CachePolicyNames{p.HeadersConfig.Headers, p.CookiesConfig.Cookies, p.QueryStringsConfig.QueryStrings} {
		if err := validateCachePolicyNames(names); err != nil {
			return err
		}
	}

	return nil
}

func isCachePolicyListBehavior(behavior string) bool {
	return slices.Contains([]string{cachePolicyBehaviorNone, cachePolicyBehaviorWhitelist, cachePolicyBehaviorAllExcept, cachePolicyBehaviorAll}, behavior)
}

func validateCachePolicyNames(names *CachePolicyNames) error {
	switch {
	case names == nil:
		return nil
	case names.Quantity == nil:
		return invalidArgument("Quantity is required.")
	case *names.Quantity != names.Items.count():
		return &Error{Code: errInconsistentQuantities, Message: "The value of Quantity and the size of Items don't match."}
	}

	return nil
}

func (p *CachePolicy) clone() *CachePolicy {
	out := *p
	out.Config = *p.Config.clone()

	return &out
}

// clone deep-copies the configuration so neither callers nor later requests
// share pointers or slices with the stored policy.
func (c *CachePolicyConfig) clone() *CachePolicyConfig {
	out := *c
	out.DefaultTTL = clonePtr(c.DefaultTTL)
	out.MaxTTL = clonePtr(c.MaxTTL)
	out.MinTTL = clonePtr(c.MinTTL)

	if c.Parameters == nil {
		return &out
	}

	p := *c.Parameters
	p.EnableAcceptEncodingGzip = clonePtr(p.EnableAcceptEncodingGzip)
	p.EnableAcceptEncodingBrotli = clonePtr(p.EnableAcceptEncodingBrotli)

	if p.HeadersConfig != nil {
		h := *p.HeadersConfig
		h.Headers = h.Headers.clone()
		p.HeadersConfig = &h
	}

	if p.CookiesConfig != nil {
		ck := *p.CookiesConfig
		ck.Cookies = ck.Cookies.clone()
		p.CookiesConfig = &ck
	}

	if p.QueryStringsConfig != nil {
		q := *p.QueryStringsConfig
		q.QueryStrings = q.QueryStrings.clone()
		p.QueryStringsConfig = &q
	}

	out.Parameters = &p

	return &out
}

func (n *CachePolicyNames) clone() *CachePolicyNames {
	if n == nil {
		return nil
	}

	return &CachePolicyNames{Quantity: clonePtr(n.Quantity), Items: n.Items.clone()}
}

func (i *CachePolicyNameItems) clone() *CachePolicyNameItems {
	if i == nil {
		return nil
	}

	return &CachePolicyNameItems{Name: slices.Clone(i.Name)}
}

func (i *CachePolicyNameItems) count() int {
	if i == nil {
		return 0
	}

	return len(i.Name)
}

func clonePtr[T any](v *T) *T {
	if v == nil {
		return nil
	}

	out := *v

	return &out
}
