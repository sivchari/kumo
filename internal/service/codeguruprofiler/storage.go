package codeguruprofiler

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Storage defines the interface for CodeGuru Profiler storage operations.
type Storage interface {
	CreateProfilingGroup(input *CreateProfilingGroupInput) *ProfilingGroup
	DescribeProfilingGroup(name string) (*ProfilingGroup, error)
	UpdateProfilingGroup(name string, input *UpdateProfilingGroupInput) (*ProfilingGroup, error)
	DeleteProfilingGroup(name string) error
	ListProfilingGroups() []ProfilingGroup
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
	mu      sync.RWMutex               `json:"-"`
	Groups  map[string]*ProfilingGroup `json:"groups"`
	dataDir string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Groups: make(map[string]*ProfilingGroup),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "codeguru-profiler", s)
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

	if m.Groups == nil {
		m.Groups = make(map[string]*ProfilingGroup)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "codeguru-profiler", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "codeguru-profiler", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateProfilingGroup creates a new profiling group.
func (m *MemoryStorage) CreateProfilingGroup(input *CreateProfilingGroupInput) *ProfilingGroup {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	id := uuid.New().String()

	computePlatform := input.ComputePlatform
	if computePlatform == "" {
		computePlatform = "Default"
	}

	group := &ProfilingGroup{
		AgentOrchestrationConfig: input.AgentOrchestrationConfig,
		Arn:                      fmt.Sprintf("arn:aws:codeguru-profiler:us-east-1:000000000000:profilingGroup/%s/%s", input.ProfilingGroupName, id),
		ComputePlatform:          computePlatform,
		CreatedAt:                now,
		Name:                     input.ProfilingGroupName,
		ProfilingStatus:          &ProfilingStatus{},
		Tags:                     input.Tags,
		UpdatedAt:                now,
	}

	if group.AgentOrchestrationConfig == nil {
		group.AgentOrchestrationConfig = &AgentOrchestrationConfig{
			ProfilingEnabled: true,
		}
	}

	m.Groups[input.ProfilingGroupName] = group

	m.saveLocked()

	return group
}

// DescribeProfilingGroup returns a profiling group by name.
func (m *MemoryStorage) DescribeProfilingGroup(name string) (*ProfilingGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, ok := m.Groups[name]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: profiling group %s not found", name)
	}

	return group, nil
}

// UpdateProfilingGroup updates a profiling group.
func (m *MemoryStorage) UpdateProfilingGroup(name string, input *UpdateProfilingGroupInput) (*ProfilingGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.Groups[name]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: profiling group %s not found", name)
	}

	if input.AgentOrchestrationConfig != nil {
		group.AgentOrchestrationConfig = input.AgentOrchestrationConfig
	}

	group.UpdatedAt = time.Now().UTC()

	m.saveLocked()

	return group, nil
}

// DeleteProfilingGroup deletes a profiling group by name.
func (m *MemoryStorage) DeleteProfilingGroup(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Groups[name]; !ok {
		return fmt.Errorf("ResourceNotFoundException: profiling group %s not found", name)
	}

	delete(m.Groups, name)

	m.saveLocked()

	return nil
}

// ListProfilingGroups returns all profiling groups.
func (m *MemoryStorage) ListProfilingGroups() []ProfilingGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ProfilingGroup, 0, len(m.Groups))
	for _, group := range m.Groups {
		result = append(result, *group)
	}

	return result
}
