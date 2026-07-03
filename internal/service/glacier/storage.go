package glacier

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "000000000000"

	errVaultNotFound = "ResourceNotFoundException"
)

// ServiceError represents a Glacier service error.
type ServiceError = service.CodedError

// Storage defines the Glacier storage interface.
type Storage interface {
	CreateVault(ctx context.Context, vaultName string) (*Vault, error)
	DescribeVault(ctx context.Context, vaultName string) (*Vault, error)
	DeleteVault(ctx context.Context, vaultName string) error
	ListVaults(ctx context.Context) ([]Vault, error)
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
	mu      sync.RWMutex      `json:"-"`
	Vaults  map[string]*Vault `json:"vaults"`
	dataDir string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Vaults: make(map[string]*Vault),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "glacier", s)
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

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "glacier", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "glacier", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateVault creates a new vault.
func (m *MemoryStorage) CreateVault(_ context.Context, vaultName string) (*Vault, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// CreateVault is idempotent - no error if vault already exists
	if v, exists := m.Vaults[vaultName]; exists {
		return v, nil
	}

	vault := &Vault{
		CreationDate:     time.Now().UTC().Format(time.RFC3339),
		NumberOfArchives: 0,
		SizeInBytes:      0,
		VaultARN:         fmt.Sprintf("arn:aws:glacier:%s:%s:vaults/%s", defaultRegion, defaultAccountID, vaultName),
		VaultName:        vaultName,
	}

	m.Vaults[vaultName] = vault

	m.saveLocked()

	return vault, nil
}

// DescribeVault returns a vault.
func (m *MemoryStorage) DescribeVault(_ context.Context, vaultName string) (*Vault, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vault, exists := m.Vaults[vaultName]
	if !exists {
		return nil, &ServiceError{
			Code:    errVaultNotFound,
			Message: fmt.Sprintf("Vault not found: arn:aws:glacier:%s:%s:vaults/%s", defaultRegion, defaultAccountID, vaultName),
		}
	}

	return vault, nil
}

// DeleteVault deletes a vault.
func (m *MemoryStorage) DeleteVault(_ context.Context, vaultName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Vaults[vaultName]; !exists {
		return &ServiceError{
			Code:    errVaultNotFound,
			Message: fmt.Sprintf("Vault not found: arn:aws:glacier:%s:%s:vaults/%s", defaultRegion, defaultAccountID, vaultName),
		}
	}

	delete(m.Vaults, vaultName)

	m.saveLocked()

	return nil
}

// ListVaults returns all vaults.
func (m *MemoryStorage) ListVaults(_ context.Context) ([]Vault, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vaults := make([]Vault, 0, len(m.Vaults))
	for _, v := range m.Vaults {
		vaults = append(vaults, *v)
	}

	return vaults, nil
}
