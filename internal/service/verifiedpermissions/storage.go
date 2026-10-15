package verifiedpermissions

import (
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	snapshotName      = "verifiedpermissions"
	accountID         = "000000000000"
	validationModeOff = "OFF"
	policyTypeStatic  = "STATIC"
)

// Option configures a MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables on-disk persistence under the given directory.
func WithDataDir(dir string) Option {
	return func(m *MemoryStorage) {
		m.dataDir = dir
	}
}

// MemoryStorage holds policy stores, schemas, policies, and identity sources
// in memory, with optional JSON persistence when a data directory is set.
type MemoryStorage struct {
	mu              sync.RWMutex
	Stores          map[string]*PolicyStore               `json:"stores"`
	Schemas         map[string]*Schema                    `json:"schemas"`
	Policies        map[string]map[string]*Policy         `json:"policies"`
	IdentitySources map[string]map[string]*IdentitySource `json:"identitySources"`
	Tags            map[string]map[string]string          `json:"tags"`
	dataDir         string
}

// NewMemoryStorage creates an empty storage, restoring prior state when a data
// directory is configured.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	m := &MemoryStorage{
		Stores:          make(map[string]*PolicyStore),
		Schemas:         make(map[string]*Schema),
		Policies:        make(map[string]map[string]*Policy),
		IdentitySources: make(map[string]map[string]*IdentitySource),
		Tags:            make(map[string]map[string]string),
	}

	for _, o := range opts {
		o(m)
	}

	if m.dataDir != "" {
		_ = storage.Load(m.dataDir, snapshotName, m)
	}

	return m
}

// MarshalJSON serializes the store under its read lock.
func (m *MemoryStorage) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(m)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the store, re-initializing nil maps.
func (m *MemoryStorage) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(m)}
	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if m.Stores == nil {
		m.Stores = make(map[string]*PolicyStore)
	}

	if m.Schemas == nil {
		m.Schemas = make(map[string]*Schema)
	}

	if m.Policies == nil {
		m.Policies = make(map[string]map[string]*Policy)
	}

	if m.IdentitySources == nil {
		m.IdentitySources = make(map[string]map[string]*IdentitySource)
	}

	if m.Tags == nil {
		m.Tags = make(map[string]map[string]string)
	}

	return nil
}

// Close persists the final snapshot synchronously when persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, snapshotName, m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// saveLocked schedules a debounced snapshot. It must be called while holding
// the write lock; ScheduleSave only records the callback, so there is no
// re-entrant lock acquisition.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, snapshotName, m.MarshalJSON)
}

// CreatePolicyStore adds a new policy store and returns it.
func (m *MemoryStorage) CreatePolicyStore(mode, description string) *PolicyStore {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mode == "" {
		mode = validationModeOff
	}

	now := time.Now().UTC()
	id := newID("PS")
	store := &PolicyStore{
		ID:              id,
		ARN:             storeARN(id),
		ValidationMode:  mode,
		Description:     description,
		CreatedDate:     now,
		LastUpdatedDate: now,
	}
	m.Stores[id] = store

	m.saveLocked()

	return store
}

// GetPolicyStore returns the store by id.
func (m *MemoryStorage) GetPolicyStore(id string) (*PolicyStore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	store, ok := m.Stores[id]
	if !ok {
		return nil, notFound("policy store " + id + " not found")
	}

	return store, nil
}

// DeletePolicyStore removes a store and all of its child resources.
func (m *MemoryStorage) DeletePolicyStore(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Stores[id]; !ok {
		return notFound("policy store " + id + " not found")
	}

	delete(m.Stores, id)
	delete(m.Schemas, id)
	delete(m.Policies, id)
	delete(m.IdentitySources, id)

	m.saveLocked()

	return nil
}

// ListPolicyStores returns all stores ordered by creation time.
func (m *MemoryStorage) ListPolicyStores() []*PolicyStore {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return sortedByCreated(m.Stores, func(s *PolicyStore) time.Time { return s.CreatedDate })
}

// PutSchema stores (or replaces) the schema for a policy store.
func (m *MemoryStorage) PutSchema(storeID, document string) (*Schema, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Stores[storeID]; !ok {
		return nil, notFound("policy store " + storeID + " not found")
	}

	now := time.Now().UTC()
	created := now

	if existing := m.Schemas[storeID]; existing != nil {
		created = existing.CreatedDate
	}

	schema := &Schema{
		PolicyStoreID:   storeID,
		Document:        document,
		Namespaces:      extractNamespaces(document),
		CreatedDate:     created,
		LastUpdatedDate: now,
	}
	m.Schemas[storeID] = schema

	m.saveLocked()

	return schema, nil
}

// GetSchema returns the schema for a policy store.
func (m *MemoryStorage) GetSchema(storeID string) (*Schema, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.Stores[storeID]; !ok {
		return nil, notFound("policy store " + storeID + " not found")
	}

	schema, ok := m.Schemas[storeID]
	if !ok {
		return nil, notFound("schema for policy store " + storeID + " not found")
	}

	return schema, nil
}

// CreatePolicy adds a static policy to a store.
func (m *MemoryStorage) CreatePolicy(storeID, description, statement string) (*Policy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Stores[storeID]; !ok {
		return nil, notFound("policy store " + storeID + " not found")
	}

	now := time.Now().UTC()
	id := newID("PB")
	policy := &Policy{
		ID:              id,
		PolicyStoreID:   storeID,
		PolicyType:      policyTypeStatic,
		Description:     description,
		Statement:       statement,
		CreatedDate:     now,
		LastUpdatedDate: now,
	}

	if m.Policies[storeID] == nil {
		m.Policies[storeID] = make(map[string]*Policy)
	}

	m.Policies[storeID][id] = policy

	m.saveLocked()

	return policy, nil
}

// GetPolicy returns a policy by id.
func (m *MemoryStorage) GetPolicy(storeID, policyID string) (*Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.policyLocked(storeID, policyID)
}

// ListPolicies returns all policies in a store ordered by creation time.
func (m *MemoryStorage) ListPolicies(storeID string) ([]*Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.Stores[storeID]; !ok {
		return nil, notFound("policy store " + storeID + " not found")
	}

	return sortedByCreated(m.Policies[storeID], func(p *Policy) time.Time { return p.CreatedDate }), nil
}

// UpdatePolicy replaces a policy's statement and description.
func (m *MemoryStorage) UpdatePolicy(storeID, policyID, description, statement string) (*Policy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, err := m.policyLocked(storeID, policyID)
	if err != nil {
		return nil, err
	}

	if statement != "" {
		policy.Statement = statement
	}

	policy.Description = description
	policy.LastUpdatedDate = time.Now().UTC()

	m.saveLocked()

	return policy, nil
}

// DeletePolicy removes a policy from a store.
func (m *MemoryStorage) DeletePolicy(storeID, policyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := m.policyLocked(storeID, policyID); err != nil {
		return err
	}

	delete(m.Policies[storeID], policyID)

	m.saveLocked()

	return nil
}

// PoliciesFor returns a shallow copy of a store's policies for evaluation.
func (m *MemoryStorage) PoliciesFor(storeID string) (map[string]*Policy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.Stores[storeID]; !ok {
		return nil, notFound("policy store " + storeID + " not found")
	}

	out := make(map[string]*Policy, len(m.Policies[storeID]))
	maps.Copy(out, m.Policies[storeID])

	return out, nil
}

// CreateIdentitySource registers a Cognito identity source for a store.
func (m *MemoryStorage) CreateIdentitySource(storeID, principalEntityType, userPoolARN string, clientIDs []string) (*IdentitySource, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Stores[storeID]; !ok {
		return nil, notFound("policy store " + storeID + " not found")
	}

	now := time.Now().UTC()
	id := newID("IS")
	src := &IdentitySource{
		ID:                  id,
		PolicyStoreID:       storeID,
		PrincipalEntityType: principalEntityType,
		UserPoolARN:         userPoolARN,
		ClientIDs:           clientIDs,
		CreatedDate:         now,
		LastUpdatedDate:     now,
	}

	if m.IdentitySources[storeID] == nil {
		m.IdentitySources[storeID] = make(map[string]*IdentitySource)
	}

	m.IdentitySources[storeID][id] = src

	m.saveLocked()

	return src, nil
}

// GetIdentitySource returns an identity source by id.
func (m *MemoryStorage) GetIdentitySource(storeID, sourceID string) (*IdentitySource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.identitySourceLocked(storeID, sourceID)
}

// ListIdentitySources returns all identity sources in a store.
func (m *MemoryStorage) ListIdentitySources(storeID string) ([]*IdentitySource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.Stores[storeID]; !ok {
		return nil, notFound("policy store " + storeID + " not found")
	}

	return sortedByCreated(m.IdentitySources[storeID], func(s *IdentitySource) time.Time { return s.CreatedDate }), nil
}

// DeleteIdentitySource removes an identity source from a store.
func (m *MemoryStorage) DeleteIdentitySource(storeID, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := m.identitySourceLocked(storeID, sourceID); err != nil {
		return err
	}

	delete(m.IdentitySources[storeID], sourceID)

	m.saveLocked()

	return nil
}

// ListTags returns a copy of the tags registered for a resource ARN. An unknown
// ARN yields an empty (non-nil) map so the Terraform provider's read-time
// ListTagsForResource stays stable even when no tags were ever set.
func (m *MemoryStorage) ListTags(resourceARN string) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]string, len(m.Tags[resourceARN]))
	maps.Copy(out, m.Tags[resourceARN])

	return out
}

// TagResource merges the given tags into a resource ARN's tag set.
func (m *MemoryStorage) TagResource(resourceARN string, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(tags) == 0 {
		return
	}

	if m.Tags[resourceARN] == nil {
		m.Tags[resourceARN] = make(map[string]string, len(tags))
	}

	maps.Copy(m.Tags[resourceARN], tags)

	m.saveLocked()
}

// UntagResource removes the given tag keys from a resource ARN's tag set.
func (m *MemoryStorage) UntagResource(resourceARN string, keys []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, k := range keys {
		delete(m.Tags[resourceARN], k)
	}

	m.saveLocked()
}

// policyLocked resolves a policy. Callers must hold the lock.
func (m *MemoryStorage) policyLocked(storeID, policyID string) (*Policy, error) {
	if _, ok := m.Stores[storeID]; !ok {
		return nil, notFound("policy store " + storeID + " not found")
	}

	policy, ok := m.Policies[storeID][policyID]
	if !ok {
		return nil, notFound("policy " + policyID + " not found")
	}

	return policy, nil
}

// identitySourceLocked resolves an identity source. Callers must hold the lock.
func (m *MemoryStorage) identitySourceLocked(storeID, sourceID string) (*IdentitySource, error) {
	if _, ok := m.Stores[storeID]; !ok {
		return nil, notFound("policy store " + storeID + " not found")
	}

	src, ok := m.IdentitySources[storeID][sourceID]
	if !ok {
		return nil, notFound("identity source " + sourceID + " not found")
	}

	return src, nil
}

// sortedByCreated returns a resource map's values as a slice ordered by
// creation time. Callers must hold the lock for the duration of the call, since
// it iterates the supplied map.
func sortedByCreated[T any](resources map[string]T, created func(T) time.Time) []T {
	items := make([]T, 0, len(resources))
	for _, v := range resources {
		items = append(items, v)
	}

	sort.Slice(items, func(i, j int) bool {
		return created(items[i]).Before(created(items[j]))
	})

	return items
}

// newID returns a unique opaque id with the given two-letter prefix.
func newID(prefix string) string {
	return prefix + strings.ReplaceAll(uuid.NewString(), "-", "")
}

// storeARN builds the policy-store ARN (the region segment is empty for AVP).
func storeARN(id string) string {
	return fmt.Sprintf("arn:aws:verifiedpermissions::%s:policy-store/%s", accountID, id)
}

// extractNamespaces returns the sorted top-level namespace keys of a Cedar
// schema JSON document.
func extractNamespaces(document string) []string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(document), &raw); err != nil {
		return []string{}
	}

	namespaces := make([]string, 0, len(raw))
	for ns := range raw {
		namespaces = append(namespaces, ns)
	}

	sort.Strings(namespaces)

	return namespaces
}
