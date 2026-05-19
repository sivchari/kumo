package dlm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errResourceNotFound    = "ResourceNotFoundException"
	errInvalidRequest      = "InvalidRequestException"
	errLimitExceeded       = "LimitExceededException"
	errInternalServerError = "InternalServerException"
)

// Default values.
const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "123456789012"
)

// Storage defines the DLM service storage interface.
type Storage interface {
	CreateLifecyclePolicy(ctx context.Context, req *CreateLifecyclePolicyRequest) (*LifecyclePolicy, error)
	GetLifecyclePolicy(ctx context.Context, policyID string) (*LifecyclePolicy, error)
	GetLifecyclePolicies(ctx context.Context, policyIDs []string, state string, resourceTypes, targetTags []string) ([]*LifecyclePolicySummary, error)
	UpdateLifecyclePolicy(ctx context.Context, policyID string, req *UpdateLifecyclePolicyRequest) error
	DeleteLifecyclePolicy(ctx context.Context, policyID string) error
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu        sync.RWMutex                `json:"-"`
	Policies  map[string]*LifecyclePolicy `json:"policies"`
	region    string
	accountID string
	dataDir   string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Policies:  make(map[string]*LifecyclePolicy),
		region:    region,
		accountID: defaultAccountID,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "dlm", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
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

// UnmarshalJSON restores the storage state from JSON.
func (m *MemoryStorage) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(m)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if m.Policies == nil {
		m.Policies = make(map[string]*LifecyclePolicy)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(m)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(m.dataDir, "dlm", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "dlm", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateLifecyclePolicy creates a new lifecycle policy.
func (m *MemoryStorage) CreateLifecyclePolicy(_ context.Context, req *CreateLifecyclePolicyRequest) (*LifecyclePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policyID := generatePolicyID()
	policyArn := generatePolicyArn(m.region, m.accountID, policyID)
	now := time.Now()

	policy := &LifecyclePolicy{
		PolicyID:         policyID,
		PolicyArn:        policyArn,
		Description:      req.Description,
		State:            req.State,
		ExecutionRoleArn: req.ExecutionRoleArn,
		PolicyDetails:    req.PolicyDetails,
		Tags:             req.Tags,
		DateCreated:      now,
		DateModified:     now,
		DefaultPolicy:    req.DefaultPolicy != "",
	}

	m.Policies[policyID] = policy

	m.saveLocked()

	return policy, nil
}

// GetLifecyclePolicy retrieves a lifecycle policy by ID.
func (m *MemoryStorage) GetLifecyclePolicy(_ context.Context, policyID string) (*LifecyclePolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, exists := m.Policies[policyID]
	if !exists {
		return nil, &Error{Code: errResourceNotFound, Message: "Lifecycle policy not found: " + policyID}
	}

	return policy, nil
}

// GetLifecyclePolicies retrieves lifecycle policies with optional filters.
func (m *MemoryStorage) GetLifecyclePolicies(_ context.Context, policyIDs []string, state string, resourceTypes, targetTags []string) ([]*LifecyclePolicySummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LifecyclePolicySummary, 0, len(m.Policies))

	// If specific policy IDs are requested, filter by them.
	if len(policyIDs) > 0 {
		policyIDSet := make(map[string]bool)
		for _, id := range policyIDs {
			policyIDSet[id] = true
		}

		for _, policy := range m.Policies {
			if !policyIDSet[policy.PolicyID] {
				continue
			}

			if !matchesFilters(policy, state, resourceTypes, targetTags) {
				continue
			}

			result = append(result, toSummary(policy))
		}

		return result, nil
	}

	// Otherwise, return all policies that match filters.
	for _, policy := range m.Policies {
		if !matchesFilters(policy, state, resourceTypes, targetTags) {
			continue
		}

		result = append(result, toSummary(policy))
	}

	return result, nil
}

// UpdateLifecyclePolicy updates a lifecycle policy.
func (m *MemoryStorage) UpdateLifecyclePolicy(_ context.Context, policyID string, req *UpdateLifecyclePolicyRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, exists := m.Policies[policyID]
	if !exists {
		return &Error{Code: errResourceNotFound, Message: "Lifecycle policy not found: " + policyID}
	}

	if req.Description != "" {
		policy.Description = req.Description
	}

	if req.ExecutionRoleArn != "" {
		policy.ExecutionRoleArn = req.ExecutionRoleArn
	}

	if req.State != "" {
		policy.State = req.State
	}

	if req.PolicyDetails != nil {
		policy.PolicyDetails = req.PolicyDetails
	}

	policy.DateModified = time.Now()

	m.saveLocked()

	return nil
}

// DeleteLifecyclePolicy deletes a lifecycle policy.
func (m *MemoryStorage) DeleteLifecyclePolicy(_ context.Context, policyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Policies[policyID]; !exists {
		return &Error{Code: errResourceNotFound, Message: "Lifecycle policy not found: " + policyID}
	}

	delete(m.Policies, policyID)

	m.saveLocked()

	return nil
}

// Helper functions.

func generatePolicyID() string {
	return "policy-" + uuid.New().String()[:17]
}

func generatePolicyArn(region, accountID, policyID string) string {
	return fmt.Sprintf("arn:aws:dlm:%s:%s:policy/%s", region, accountID, policyID)
}

func matchesFilters(policy *LifecyclePolicy, state string, resourceTypes, targetTags []string) bool {
	if !matchesState(policy, state) {
		return false
	}

	if !matchesResourceTypes(policy, resourceTypes) {
		return false
	}

	if !matchesTargetTags(policy, targetTags) {
		return false
	}

	return true
}

func matchesState(policy *LifecyclePolicy, state string) bool {
	return state == "" || policy.State == state
}

func matchesResourceTypes(policy *LifecyclePolicy, resourceTypes []string) bool {
	if len(resourceTypes) == 0 || policy.PolicyDetails == nil {
		return true
	}

	for _, rt := range resourceTypes {
		for _, prt := range policy.PolicyDetails.ResourceTypes {
			if rt == prt {
				return true
			}
		}
	}

	return false
}

func matchesTargetTags(policy *LifecyclePolicy, targetTags []string) bool {
	if len(targetTags) == 0 || policy.PolicyDetails == nil {
		return true
	}

	for _, tt := range targetTags {
		for _, ptt := range policy.PolicyDetails.TargetTags {
			if tt == ptt.Key || tt == ptt.Value {
				return true
			}
		}
	}

	return false
}

func toSummary(policy *LifecyclePolicy) *LifecyclePolicySummary {
	summary := &LifecyclePolicySummary{
		PolicyID:      policy.PolicyID,
		Description:   policy.Description,
		State:         policy.State,
		Tags:          policy.Tags,
		DefaultPolicy: policy.DefaultPolicy,
	}

	if policy.PolicyDetails != nil {
		summary.PolicyType = policy.PolicyDetails.PolicyType
	}

	return summary
}

// Ensure MemoryStorage implements Storage.
var _ Storage = (*MemoryStorage)(nil)
