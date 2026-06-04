package dataexchange

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Storage defines the interface for Data Exchange storage operations.
type Storage interface {
	CreateDataSet(input *CreateDataSetInput) *DataSet
	GetDataSet(id string) (*DataSet, error)
	ListDataSets() []DataSet
	UpdateDataSet(id string, input *UpdateDataSetInput) (*DataSet, error)
	DeleteDataSet(id string) error

	CreateRevision(dataSetID string, input *CreateRevisionInput) (*Revision, error)
	GetRevision(dataSetID, revisionID string) (*Revision, error)
	ListRevisions(dataSetID string) ([]Revision, error)
	UpdateRevision(dataSetID, revisionID string, input *UpdateRevisionInput) (*Revision, error)
	DeleteRevision(dataSetID, revisionID string) error

	CreateJob(input *CreateJobInput) *Job
	GetJob(id string) (*Job, error)
	ListJobs() []Job
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
	mu        sync.RWMutex                    `json:"-"`
	DataSets  map[string]*DataSet             `json:"dataSets"`
	Revisions map[string]map[string]*Revision `json:"revisions"` // dataSetID -> revisionID -> revision
	Jobs      map[string]*Job                 `json:"jobs"`
	dataDir   string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		DataSets:  make(map[string]*DataSet),
		Revisions: make(map[string]map[string]*Revision),
		Jobs:      make(map[string]*Job),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "dataexchange", s)
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

	if m.DataSets == nil {
		m.DataSets = make(map[string]*DataSet)
	}

	if m.Revisions == nil {
		m.Revisions = make(map[string]map[string]*Revision)
	}

	if m.Jobs == nil {
		m.Jobs = make(map[string]*Job)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "dataexchange", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "dataexchange", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateDataSet creates a new data set.
func (m *MemoryStorage) CreateDataSet(input *CreateDataSetInput) *DataSet {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	now := time.Now().UTC()

	ds := &DataSet{
		Arn:         fmt.Sprintf("arn:aws:dataexchange:us-east-1:000000000000:data-sets/%s", id),
		AssetType:   input.AssetType,
		CreatedAt:   now,
		Description: input.Description,
		ID:          id,
		Name:        input.Name,
		Origin:      "OWNED",
		UpdatedAt:   now,
		Tags:        input.Tags,
	}

	m.DataSets[id] = ds

	m.saveLocked()

	return ds
}

// GetDataSet returns a data set by ID.
func (m *MemoryStorage) GetDataSet(id string) (*DataSet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ds, ok := m.DataSets[id]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: data set %s not found", id)
	}

	return ds, nil
}

// ListDataSets returns all data sets.
func (m *MemoryStorage) ListDataSets() []DataSet {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]DataSet, 0, len(m.DataSets))
	for _, ds := range m.DataSets {
		result = append(result, *ds)
	}

	return result
}

// UpdateDataSet updates a data set.
func (m *MemoryStorage) UpdateDataSet(id string, input *UpdateDataSetInput) (*DataSet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ds, ok := m.DataSets[id]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: data set %s not found", id)
	}

	if input.Name != "" {
		ds.Name = input.Name
	}

	if input.Description != "" {
		ds.Description = input.Description
	}

	ds.UpdatedAt = time.Now().UTC()

	m.saveLocked()

	return ds, nil
}

// DeleteDataSet deletes a data set by ID.
func (m *MemoryStorage) DeleteDataSet(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.DataSets[id]; !ok {
		return fmt.Errorf("ResourceNotFoundException: data set %s not found", id)
	}

	delete(m.DataSets, id)
	delete(m.Revisions, id)

	m.saveLocked()

	return nil
}

// CreateRevision creates a new revision for a data set.
func (m *MemoryStorage) CreateRevision(dataSetID string, input *CreateRevisionInput) (*Revision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.DataSets[dataSetID]; !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: data set %s not found", dataSetID)
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	rev := &Revision{
		Arn:       fmt.Sprintf("arn:aws:dataexchange:us-east-1:000000000000:data-sets/%s/revisions/%s", dataSetID, id),
		Comment:   input.Comment,
		CreatedAt: now,
		DataSetID: dataSetID,
		ID:        id,
		UpdatedAt: now,
	}

	if m.Revisions[dataSetID] == nil {
		m.Revisions[dataSetID] = make(map[string]*Revision)
	}

	m.Revisions[dataSetID][id] = rev

	m.saveLocked()

	return rev, nil
}

// GetRevision returns a revision by data set ID and revision ID.
func (m *MemoryStorage) GetRevision(dataSetID, revisionID string) (*Revision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dsRevisions, ok := m.Revisions[dataSetID]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: revision %s not found", revisionID)
	}

	rev, ok := dsRevisions[revisionID]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: revision %s not found", revisionID)
	}

	return rev, nil
}

// ListRevisions returns all revisions for a data set.
func (m *MemoryStorage) ListRevisions(dataSetID string) ([]Revision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.DataSets[dataSetID]; !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: data set %s not found", dataSetID)
	}

	dsRevisions := m.Revisions[dataSetID]
	result := make([]Revision, 0, len(dsRevisions))

	for _, rev := range dsRevisions {
		result = append(result, *rev)
	}

	return result, nil
}

// UpdateRevision updates a revision.
func (m *MemoryStorage) UpdateRevision(dataSetID, revisionID string, input *UpdateRevisionInput) (*Revision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dsRevisions, ok := m.Revisions[dataSetID]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: revision %s not found", revisionID)
	}

	rev, ok := dsRevisions[revisionID]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: revision %s not found", revisionID)
	}

	if input.Comment != "" {
		rev.Comment = input.Comment
	}

	if input.Finalized != nil {
		rev.Finalized = *input.Finalized
	}

	rev.UpdatedAt = time.Now().UTC()

	m.saveLocked()

	return rev, nil
}

// DeleteRevision deletes a revision.
func (m *MemoryStorage) DeleteRevision(dataSetID, revisionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dsRevisions, ok := m.Revisions[dataSetID]
	if !ok {
		return fmt.Errorf("ResourceNotFoundException: revision %s not found", revisionID)
	}

	if _, ok := dsRevisions[revisionID]; !ok {
		return fmt.Errorf("ResourceNotFoundException: revision %s not found", revisionID)
	}

	delete(dsRevisions, revisionID)

	m.saveLocked()

	return nil
}

// CreateJob creates a new job.
func (m *MemoryStorage) CreateJob(input *CreateJobInput) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	now := time.Now().UTC()

	job := &Job{
		Arn:       fmt.Sprintf("arn:aws:dataexchange:us-east-1:000000000000:jobs/%s", id),
		CreatedAt: now,
		ID:        id,
		State:     "WAITING",
		Type:      input.Type,
		UpdatedAt: now,
	}

	m.Jobs[id] = job

	m.saveLocked()

	return job
}

// GetJob returns a job by ID.
func (m *MemoryStorage) GetJob(id string) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.Jobs[id]
	if !ok {
		return nil, fmt.Errorf("ResourceNotFoundException: job %s not found", id)
	}

	return job, nil
}

// ListJobs returns all jobs.
func (m *MemoryStorage) ListJobs() []Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Job, 0, len(m.Jobs))
	for _, job := range m.Jobs {
		result = append(result, *job)
	}

	return result
}
