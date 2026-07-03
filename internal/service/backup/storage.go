package backup

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Storage defines the interface for Backup storage operations.
type Storage interface {
	CreateVault(name string, input *CreateBackupVaultInput) (*Vault, error)
	DescribeVault(name string) (*Vault, error)
	ListVaults() []Vault
	DeleteVault(name string) error

	CreatePlan(input *CreateBackupPlanInput) (*Plan, error)
	GetPlan(planID string) (*Plan, error)
	ListPlans() []PlanListMember
	DeletePlan(planID string) error

	CreateSelection(planID string, input *CreateBackupSelectionInput) (*Selection, error)
	GetSelection(planID, selectionID string) (*Selection, error)
	ListSelections(planID string) []SelectionListMember
	DeleteSelection(planID, selectionID string) error
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

// MemoryStorage is an in-memory implementation of Storage.
type MemoryStorage struct {
	mu         sync.RWMutex                     `json:"-"`
	Vaults     map[string]*Vault                `json:"vaults"`
	Plans      map[string]*Plan                 `json:"plans"`
	Selections map[string]map[string]*Selection `json:"selections"` // planID -> selectionID -> selection
	dataDir    string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Vaults:     make(map[string]*Vault),
		Plans:      make(map[string]*Plan),
		Selections: make(map[string]map[string]*Selection),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "backup", s)
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

	if m.Vaults == nil {
		m.Vaults = make(map[string]*Vault)
	}

	if m.Plans == nil {
		m.Plans = make(map[string]*Plan)
	}

	if m.Selections == nil {
		m.Selections = make(map[string]map[string]*Selection)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "backup", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "backup", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

func epochNow() float64 {
	return float64(time.Now().Unix())
}

// CreateVault creates a new backup vault.
func (m *MemoryStorage) CreateVault(name string, input *CreateBackupVaultInput) (*Vault, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Vaults[name]; exists {
		return nil, fmt.Errorf("AlreadyExistsException: backup vault %s already exists", name)
	}

	vault := &Vault{
		BackupVaultArn:  fmt.Sprintf("arn:aws:backup:us-east-1:000000000000:backup-vault:%s", name),
		BackupVaultName: name,
		CreationDate:    epochNow(),
	}

	if input != nil {
		vault.CreatorRequestID = input.CreatorRequestID
		vault.EncryptionKeyArn = input.EncryptionKeyArn
	}

	m.Vaults[name] = vault

	m.saveLocked()

	return vault, nil
}

// DescribeVault returns a backup vault by name.
func (m *MemoryStorage) DescribeVault(name string) (*Vault, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vault, ok := m.Vaults[name]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: backup vault %s not found", name)
	}

	return vault, nil
}

// ListVaults returns all backup vaults.
func (m *MemoryStorage) ListVaults() []Vault {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vaults := make([]Vault, 0, len(m.Vaults))
	for _, v := range m.Vaults {
		vaults = append(vaults, *v)
	}

	return vaults
}

// DeleteVault deletes a backup vault by name.
func (m *MemoryStorage) DeleteVault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Vaults[name]; !ok {
		return fmt.Errorf("ResourceNotFoundException: backup vault %s not found", name)
	}

	delete(m.Vaults, name)

	m.saveLocked()

	return nil
}

// CreatePlan creates a new backup plan.
func (m *MemoryStorage) CreatePlan(input *CreateBackupPlanInput) (*Plan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	planID := uuid.New().String()
	versionID := uuid.New().String()
	now := epochNow()

	rules := make([]Rule, 0, len(input.BackupPlan.Rules))
	for _, r := range input.BackupPlan.Rules {
		rules = append(rules, Rule{
			RuleName:                r.RuleName,
			RuleID:                  uuid.New().String(),
			TargetBackupVaultName:   r.TargetBackupVaultName,
			ScheduleExpression:      r.ScheduleExpression,
			StartWindowMinutes:      r.StartWindowMinutes,
			CompletionWindowMinutes: r.CompletionWindowMinutes,
		})
	}

	plan := &Plan{
		BackupPlanArn: fmt.Sprintf("arn:aws:backup:us-east-1:000000000000:backup-plan:%s", planID),
		BackupPlanID:  planID,
		BackupPlan: &PlanData{
			BackupPlanName: input.BackupPlan.BackupPlanName,
			Rules:          rules,
		},
		CreationDate: now,
		VersionID:    versionID,
	}

	m.Plans[planID] = plan

	m.saveLocked()

	return plan, nil
}

// GetPlan returns a backup plan by ID.
func (m *MemoryStorage) GetPlan(planID string) (*Plan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.Plans[planID]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: backup plan %s not found", planID)
	}

	return plan, nil
}

// ListPlans returns all backup plans.
func (m *MemoryStorage) ListPlans() []PlanListMember {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]PlanListMember, 0, len(m.Plans))
	for _, p := range m.Plans {
		plans = append(plans, PlanListMember{
			BackupPlanArn:  p.BackupPlanArn,
			BackupPlanID:   p.BackupPlanID,
			BackupPlanName: p.BackupPlan.BackupPlanName,
			CreationDate:   p.CreationDate,
			VersionID:      p.VersionID,
		})
	}

	return plans
}

// DeletePlan deletes a backup plan by ID.
func (m *MemoryStorage) DeletePlan(planID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Plans[planID]; !ok {
		return fmt.Errorf("ResourceNotFoundException: backup plan %s not found", planID)
	}

	delete(m.Plans, planID)
	delete(m.Selections, planID)

	m.saveLocked()

	return nil
}

// CreateSelection creates a new backup selection.
func (m *MemoryStorage) CreateSelection(planID string, input *CreateBackupSelectionInput) (*Selection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Plans[planID]; !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: backup plan %s not found", planID)
	}

	selectionID := uuid.New().String()

	selection := &Selection{
		BackupPlanID: planID,
		SelectionID:  selectionID,
		BackupSelection: &SelectionData{
			SelectionName: input.BackupSelection.SelectionName,
			IamRoleArn:    input.BackupSelection.IamRoleArn,
			Resources:     input.BackupSelection.Resources,
		},
		CreationDate: epochNow(),
	}

	if m.Selections[planID] == nil {
		m.Selections[planID] = make(map[string]*Selection)
	}

	m.Selections[planID][selectionID] = selection

	m.saveLocked()

	return selection, nil
}

// GetSelection returns a backup selection by plan ID and selection ID.
func (m *MemoryStorage) GetSelection(planID, selectionID string) (*Selection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	planSelections, ok := m.Selections[planID]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: backup selection %s not found", selectionID)
	}

	selection, ok := planSelections[selectionID]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: backup selection %s not found", selectionID)
	}

	return selection, nil
}

// ListSelections returns all backup selections for a plan.
func (m *MemoryStorage) ListSelections(planID string) []SelectionListMember {
	m.mu.RLock()
	defer m.mu.RUnlock()

	planSelections := m.Selections[planID]
	selections := make([]SelectionListMember, 0, len(planSelections))

	for _, s := range planSelections {
		selections = append(selections, SelectionListMember{
			BackupPlanID:  s.BackupPlanID,
			CreationDate:  s.CreationDate,
			IamRoleArn:    s.BackupSelection.IamRoleArn,
			SelectionID:   s.SelectionID,
			SelectionName: s.BackupSelection.SelectionName,
		})
	}

	return selections
}

// DeleteSelection deletes a backup selection.
func (m *MemoryStorage) DeleteSelection(planID, selectionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	planSelections, ok := m.Selections[planID]
	if !ok {
		return fmt.Errorf("ResourceNotFoundException: backup selection %s not found", selectionID)
	}

	if _, ok := planSelections[selectionID]; !ok {
		return fmt.Errorf("ResourceNotFoundException: backup selection %s not found", selectionID)
	}

	delete(planSelections, selectionID)

	m.saveLocked()

	return nil
}
