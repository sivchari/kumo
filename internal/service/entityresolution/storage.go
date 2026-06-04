package entityresolution

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "000000000000"
)

// Storage defines the Entity Resolution storage interface.
type Storage interface {
	CreateSchemaMapping(ctx context.Context, req *CreateSchemaMappingRequest) (*SchemaMapping, error)
	GetSchemaMapping(ctx context.Context, schemaName string) (*SchemaMapping, error)
	DeleteSchemaMapping(ctx context.Context, schemaName string) error
	ListSchemaMappings(ctx context.Context) ([]SchemaMappingSummary, error)

	CreateMatchingWorkflow(ctx context.Context, req *CreateMatchingWorkflowRequest) (*MatchingWorkflow, error)
	GetMatchingWorkflow(ctx context.Context, workflowName string) (*MatchingWorkflow, error)
	DeleteMatchingWorkflow(ctx context.Context, workflowName string) error
	ListMatchingWorkflows(ctx context.Context) ([]MatchingWorkflowSummary, error)

	CreateIDMappingWorkflow(ctx context.Context, req *CreateIDMappingWorkflowRequest) (*IDMappingWorkflow, error)
	GetIDMappingWorkflow(ctx context.Context, workflowName string) (*IDMappingWorkflow, error)
	DeleteIDMappingWorkflow(ctx context.Context, workflowName string) error
	ListIDMappingWorkflows(ctx context.Context) ([]IDMappingWorkflowSummary, error)

	ListProviderServices(ctx context.Context) ([]ProviderService, error)
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
	mu                 sync.RWMutex                  `json:"-"`
	Schemas            map[string]*SchemaMapping     `json:"schemas"`
	MatchingWorkflows  map[string]*MatchingWorkflow  `json:"matchingWorkflows"`
	IDMappingWorkflows map[string]*IDMappingWorkflow `json:"idMappingWorkflows"`
	dataDir            string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Schemas:            make(map[string]*SchemaMapping),
		MatchingWorkflows:  make(map[string]*MatchingWorkflow),
		IDMappingWorkflows: make(map[string]*IDMappingWorkflow),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "entityresolution", s)
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

	if m.Schemas == nil {
		m.Schemas = make(map[string]*SchemaMapping)
	}

	if m.MatchingWorkflows == nil {
		m.MatchingWorkflows = make(map[string]*MatchingWorkflow)
	}

	if m.IDMappingWorkflows == nil {
		m.IDMappingWorkflows = make(map[string]*IDMappingWorkflow)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "entityresolution", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "entityresolution", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateSchemaMapping creates a new schema mapping.
func (m *MemoryStorage) CreateSchemaMapping(_ context.Context, req *CreateSchemaMappingRequest) (*SchemaMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Schemas[req.SchemaName]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("Schema mapping %s already exists", req.SchemaName),
		}
	}

	now := float64(time.Now().Unix())
	schema := &SchemaMapping{
		SchemaName:        req.SchemaName,
		SchemaArn:         fmt.Sprintf("arn:aws:entityresolution:%s:%s:schemamapping/%s", defaultRegion, defaultAccountID, req.SchemaName),
		Description:       req.Description,
		MappedInputFields: req.MappedInputFields,
		CreatedAt:         now,
		UpdatedAt:         now,
		Tags:              req.Tags,
	}

	m.Schemas[req.SchemaName] = schema

	m.saveLocked()

	return schema, nil
}

// GetSchemaMapping returns a schema mapping.
func (m *MemoryStorage) GetSchemaMapping(_ context.Context, schemaName string) (*SchemaMapping, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schema, exists := m.Schemas[schemaName]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Schema mapping %s not found", schemaName),
		}
	}

	return schema, nil
}

// DeleteSchemaMapping deletes a schema mapping.
func (m *MemoryStorage) DeleteSchemaMapping(_ context.Context, schemaName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Schemas[schemaName]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Schema mapping %s not found", schemaName),
		}
	}

	delete(m.Schemas, schemaName)

	m.saveLocked()

	return nil
}

// ListSchemaMappings lists all schema mappings.
func (m *MemoryStorage) ListSchemaMappings(_ context.Context) ([]SchemaMappingSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summaries := make([]SchemaMappingSummary, 0, len(m.Schemas))
	for _, s := range m.Schemas {
		summaries = append(summaries, SchemaMappingSummary{
			SchemaName: s.SchemaName,
			SchemaArn:  s.SchemaArn,
			CreatedAt:  s.CreatedAt,
			UpdatedAt:  s.UpdatedAt,
		})
	}

	return summaries, nil
}

// CreateMatchingWorkflow creates a new matching workflow.
func (m *MemoryStorage) CreateMatchingWorkflow(_ context.Context, req *CreateMatchingWorkflowRequest) (*MatchingWorkflow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.MatchingWorkflows[req.WorkflowName]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("Matching workflow %s already exists", req.WorkflowName),
		}
	}

	now := float64(time.Now().Unix())
	workflow := &MatchingWorkflow{
		WorkflowName:         req.WorkflowName,
		WorkflowArn:          fmt.Sprintf("arn:aws:entityresolution:%s:%s:matchingworkflow/%s", defaultRegion, defaultAccountID, req.WorkflowName),
		Description:          req.Description,
		InputSourceConfig:    req.InputSourceConfig,
		OutputSourceConfig:   req.OutputSourceConfig,
		ResolutionTechniques: req.ResolutionTechniques,
		RoleArn:              req.RoleArn,
		CreatedAt:            now,
		UpdatedAt:            now,
		Tags:                 req.Tags,
	}

	m.MatchingWorkflows[req.WorkflowName] = workflow

	m.saveLocked()

	return workflow, nil
}

// GetMatchingWorkflow returns a matching workflow.
func (m *MemoryStorage) GetMatchingWorkflow(_ context.Context, workflowName string) (*MatchingWorkflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workflow, exists := m.MatchingWorkflows[workflowName]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Matching workflow %s not found", workflowName),
		}
	}

	return workflow, nil
}

// DeleteMatchingWorkflow deletes a matching workflow.
func (m *MemoryStorage) DeleteMatchingWorkflow(_ context.Context, workflowName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.MatchingWorkflows[workflowName]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Matching workflow %s not found", workflowName),
		}
	}

	delete(m.MatchingWorkflows, workflowName)

	m.saveLocked()

	return nil
}

// ListMatchingWorkflows lists all matching workflows.
func (m *MemoryStorage) ListMatchingWorkflows(_ context.Context) ([]MatchingWorkflowSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summaries := make([]MatchingWorkflowSummary, 0, len(m.MatchingWorkflows))
	for _, w := range m.MatchingWorkflows {
		summaries = append(summaries, MatchingWorkflowSummary{
			WorkflowName: w.WorkflowName,
			WorkflowArn:  w.WorkflowArn,
			CreatedAt:    w.CreatedAt,
			UpdatedAt:    w.UpdatedAt,
		})
	}

	return summaries, nil
}

// CreateIDMappingWorkflow creates a new ID mapping workflow.
func (m *MemoryStorage) CreateIDMappingWorkflow(_ context.Context, req *CreateIDMappingWorkflowRequest) (*IDMappingWorkflow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.IDMappingWorkflows[req.WorkflowName]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("ID mapping workflow %s already exists", req.WorkflowName),
		}
	}

	now := float64(time.Now().Unix())
	workflow := &IDMappingWorkflow{
		WorkflowName:        req.WorkflowName,
		WorkflowArn:         fmt.Sprintf("arn:aws:entityresolution:%s:%s:idmappingworkflow/%s", defaultRegion, defaultAccountID, req.WorkflowName),
		Description:         req.Description,
		InputSourceConfig:   req.InputSourceConfig,
		OutputSourceConfig:  req.OutputSourceConfig,
		IDMappingTechniques: req.IDMappingTechniques,
		RoleArn:             req.RoleArn,
		CreatedAt:           now,
		UpdatedAt:           now,
		Tags:                req.Tags,
	}

	m.IDMappingWorkflows[req.WorkflowName] = workflow

	m.saveLocked()

	return workflow, nil
}

// GetIDMappingWorkflow returns an ID mapping workflow.
func (m *MemoryStorage) GetIDMappingWorkflow(_ context.Context, workflowName string) (*IDMappingWorkflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workflow, exists := m.IDMappingWorkflows[workflowName]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("ID mapping workflow %s not found", workflowName),
		}
	}

	return workflow, nil
}

// DeleteIDMappingWorkflow deletes an ID mapping workflow.
func (m *MemoryStorage) DeleteIDMappingWorkflow(_ context.Context, workflowName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.IDMappingWorkflows[workflowName]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("ID mapping workflow %s not found", workflowName),
		}
	}

	delete(m.IDMappingWorkflows, workflowName)

	m.saveLocked()

	return nil
}

// ListIDMappingWorkflows lists all ID mapping workflows.
func (m *MemoryStorage) ListIDMappingWorkflows(_ context.Context) ([]IDMappingWorkflowSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summaries := make([]IDMappingWorkflowSummary, 0, len(m.IDMappingWorkflows))
	for _, w := range m.IDMappingWorkflows {
		summaries = append(summaries, IDMappingWorkflowSummary{
			WorkflowName: w.WorkflowName,
			WorkflowArn:  w.WorkflowArn,
			CreatedAt:    w.CreatedAt,
			UpdatedAt:    w.UpdatedAt,
		})
	}

	return summaries, nil
}

// ListProviderServices returns a static list of provider services.
func (m *MemoryStorage) ListProviderServices(_ context.Context) ([]ProviderService, error) {
	return []ProviderService{}, nil
}
