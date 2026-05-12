package cloudfront

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// SigningStorage is the in-memory store for PublicKey + KeyGroup
// resources. Distributions reference KeyGroups by ID via
// TrustedKeyGroups on a CacheBehavior.
//
// Lives on MemoryStorage but kept in its own file so the signing
// surface can be reviewed in isolation from Distribution / Invalidation.
type signingStore struct {
	PublicKeys map[string]*PublicKey `json:"publicKeys"`
	KeyGroups  map[string]*KeyGroup  `json:"keyGroups"`
}

// CreatePublicKey adds a new PublicKey. CallerReference dedupes — a
// duplicate returns 409 (PublicKeyAlreadyExists), matching real CF.
func (s *MemoryStorage) CreatePublicKey(_ context.Context, cfg *PublicKeyConfig) (*PublicKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureSigningInit()

	for _, k := range s.signing.PublicKeys {
		if k.PublicKeyConfig != nil && k.PublicKeyConfig.CallerReference == cfg.CallerReference {
			return nil, &Error{Code: errPublicKeyAlreadyExists, Message: fmt.Sprintf("A public key with caller reference %s already exists", cfg.CallerReference)}
		}
	}

	key := &PublicKey{
		ID:              generatePublicKeyID(),
		CreatedTime:     time.Now(),
		ETag:            generateETag(),
		PublicKeyConfig: cfg,
	}
	s.signing.PublicKeys[key.ID] = key

	return key, nil
}

// GetPublicKey retrieves a PublicKey by ID.
func (s *MemoryStorage) GetPublicKey(_ context.Context, id string) (*PublicKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	k, ok := s.signing.PublicKeys[id]
	if !ok {
		return nil, &Error{Code: errNoSuchPublicKey, Message: fmt.Sprintf("The public key %s does not exist", id)}
	}

	return k, nil
}

// ListPublicKeys returns every stored PublicKey, sorted by ID for
// deterministic output. (CloudFront does not paginate this endpoint
// in practice — at most ~25 keys per account.)
func (s *MemoryStorage) ListPublicKeys(_ context.Context) []*PublicKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*PublicKey, 0, len(s.signing.PublicKeys))
	for _, k := range s.signing.PublicKeys {
		out = append(out, k)
	}

	sortByID(out, func(k *PublicKey) string { return k.ID })

	return out
}

// DeletePublicKey removes a PublicKey. Returns PublicKeyInUse if any
// KeyGroup still references it — matches real CloudFront's safety net.
func (s *MemoryStorage) DeletePublicKey(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureSigningInit()

	if _, ok := s.signing.PublicKeys[id]; !ok {
		return &Error{Code: errNoSuchPublicKey, Message: fmt.Sprintf("The public key %s does not exist", id)}
	}

	for _, g := range s.signing.KeyGroups {
		if g.KeyGroupConfig == nil {
			continue
		}

		for _, ref := range g.KeyGroupConfig.Items {
			if ref == id {
				return &Error{Code: errPublicKeyInUse, Message: fmt.Sprintf("Public key %s is referenced by key group %s", id, g.ID)}
			}
		}
	}

	delete(s.signing.PublicKeys, id)

	return nil
}

// CreateKeyGroup adds a new KeyGroup. All referenced PublicKey IDs
// must exist; otherwise NoSuchPublicKey is returned.
func (s *MemoryStorage) CreateKeyGroup(_ context.Context, cfg *KeyGroupConfig) (*KeyGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureSigningInit()

	for _, ref := range cfg.Items {
		if _, ok := s.signing.PublicKeys[ref]; !ok {
			return nil, &Error{Code: errNoSuchPublicKey, Message: fmt.Sprintf("The public key %s does not exist", ref)}
		}
	}

	for _, g := range s.signing.KeyGroups {
		if g.KeyGroupConfig != nil && g.KeyGroupConfig.Name == cfg.Name {
			return nil, &Error{Code: errKeyGroupAlreadyExists, Message: fmt.Sprintf("A key group named %s already exists", cfg.Name)}
		}
	}

	group := &KeyGroup{
		ID:             generateKeyGroupID(),
		LastModified:   time.Now(),
		ETag:           generateETag(),
		KeyGroupConfig: cfg,
	}
	s.signing.KeyGroups[group.ID] = group

	return group, nil
}

// GetKeyGroup retrieves a KeyGroup by ID.
func (s *MemoryStorage) GetKeyGroup(_ context.Context, id string) (*KeyGroup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g, ok := s.signing.KeyGroups[id]
	if !ok {
		return nil, &Error{Code: errNoSuchKeyGroup, Message: fmt.Sprintf("The key group %s does not exist", id)}
	}

	return g, nil
}

// ListKeyGroups returns every stored KeyGroup, sorted by ID.
func (s *MemoryStorage) ListKeyGroups(_ context.Context) []*KeyGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*KeyGroup, 0, len(s.signing.KeyGroups))
	for _, g := range s.signing.KeyGroups {
		out = append(out, g)
	}

	sortByID(out, func(g *KeyGroup) string { return g.ID })

	return out
}

// DeleteKeyGroup removes a KeyGroup. Returns ResourceInUse if any
// Distribution references it via TrustedKeyGroups.
func (s *MemoryStorage) DeleteKeyGroup(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureSigningInit()

	if _, ok := s.signing.KeyGroups[id]; !ok {
		return &Error{Code: errNoSuchKeyGroup, Message: fmt.Sprintf("The key group %s does not exist", id)}
	}

	for _, d := range s.Distributions {
		if d.DistributionConfig == nil || d.DistributionConfig.DefaultCacheBehavior == nil {
			continue
		}

		dcb := d.DistributionConfig.DefaultCacheBehavior
		if dcb.TrustedKeyGroups == nil {
			continue
		}

		for _, ref := range dcb.TrustedKeyGroups.Items {
			if ref == id {
				return &Error{Code: errKeyGroupReferencedError, Message: fmt.Sprintf("Key group %s is referenced by distribution %s", id, d.ID)}
			}
		}
	}

	delete(s.signing.KeyGroups, id)

	return nil
}

// ensureSigningInit lazily initializes the signing maps. JSON
// unmarshal of legacy state files leaves them nil, so guard at every
// entry point. Caller MUST hold the appropriate lock.
func (s *MemoryStorage) ensureSigningInit() {
	if s.signing.PublicKeys == nil {
		s.signing.PublicKeys = make(map[string]*PublicKey)
	}

	if s.signing.KeyGroups == nil {
		s.signing.KeyGroups = make(map[string]*KeyGroup)
	}
}

func generatePublicKeyID() string {
	return "K" + randomID(13)
}

func generateKeyGroupID() string {
	return randomID(36) // UUID-like
}

func randomID(n int) string {
	b := make([]byte, (n+1)/2)

	_, _ = rand.Read(b)

	return strings.ToUpper(hex.EncodeToString(b))[:n]
}

func sortByID[T any](items []T, key func(T) string) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && key(items[j]) < key(items[j-1]); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}
