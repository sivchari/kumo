package athena

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes for Athena.
const (
	errInvalidRequestException = "InvalidRequestException"
)

// Storage defines the interface for Athena storage.
type Storage interface {
	StartQueryExecution(ctx context.Context, query string, workGroup string, context *QueryExecutionContext, resultConfig *ResultConfiguration, executionParams []string) (*QueryExecution, error)
	StopQueryExecution(ctx context.Context, queryExecutionID string) error
	GetQueryExecution(ctx context.Context, queryExecutionID string) (*QueryExecution, error)
	GetQueryResults(ctx context.Context, queryExecutionID string, nextToken string, maxResults int32) (*ResultSet, string, error)
	ListQueryExecutions(ctx context.Context, workGroup string, nextToken string, maxResults int32) ([]string, string, error)
	CreateWorkGroup(ctx context.Context, name string, configuration *WorkGroupConfiguration, description string, tags []Tag) error
	DeleteWorkGroup(ctx context.Context, name string, recursiveDelete bool) error
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
	mu              sync.RWMutex               `json:"-"`
	QueryExecutions map[string]*QueryExecution `json:"queryExecutions"`
	WorkGroups      map[string]*WorkGroup      `json:"workGroups"`
	QueryResults    map[string]*ResultSet      `json:"queryResults"`
	dataDir         string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		QueryExecutions: make(map[string]*QueryExecution),
		WorkGroups:      make(map[string]*WorkGroup),
		QueryResults:    make(map[string]*ResultSet),
	}

	// Create the default "primary" workgroup.
	s.WorkGroups["primary"] = &WorkGroup{
		Name:         "primary",
		State:        WorkGroupStateEnabled,
		CreationTime: time.Now(),
	}

	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "athena", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(s)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.QueryExecutions == nil {
		s.QueryExecutions = make(map[string]*QueryExecution)
	}

	if s.WorkGroups == nil {
		s.WorkGroups = make(map[string]*WorkGroup)
	}

	if s.QueryResults == nil {
		s.QueryResults = make(map[string]*ResultSet)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(s)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(s.dataDir, "athena", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "athena", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// StartQueryExecution starts a new query execution.
func (s *MemoryStorage) StartQueryExecution(_ context.Context, query, workGroup string, execContext *QueryExecutionContext, resultConfig *ResultConfiguration, executionParams []string) (*QueryExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if workGroup == "" {
		workGroup = "primary"
	}

	// Verify workgroup exists.
	if _, ok := s.WorkGroups[workGroup]; !ok {
		return nil, &ServiceError{
			Code:    errInvalidRequestException,
			Message: fmt.Sprintf("WorkGroup %s is not found.", workGroup),
		}
	}

	queryExecutionID := uuid.New().String()
	now := time.Now()

	qe := &QueryExecution{
		QueryExecutionID:      queryExecutionID,
		Query:                 query,
		StatementType:         "DML",
		ResultConfiguration:   resultConfig,
		QueryExecutionContext: execContext,
		Status: &QueryExecutionStatus{
			State:              QueryExecutionStateSucceeded,
			SubmissionDateTime: now,
			CompletionDateTime: &now,
		},
		Statistics: &QueryExecutionStatistics{
			EngineExecutionTimeInMillis:      100,
			DataScannedInBytes:               1024,
			TotalExecutionTimeInMillis:       150,
			QueryQueueTimeInMillis:           10,
			ServicePreProcessingTimeInMillis: 20,
			QueryPlanningTimeInMillis:        10,
			ServiceProcessingTimeInMillis:    10,
		},
		WorkGroup:           workGroup,
		ExecutionParameters: executionParams,
		EngineVersion: &EngineVersion{
			SelectedEngineVersion:  "AUTO",
			EffectiveEngineVersion: "Athena engine version 3",
		},
	}

	s.QueryExecutions[queryExecutionID] = qe
	s.QueryResults[queryExecutionID] = createMockResultSet()

	s.saveLocked()

	return qe, nil
}

func createMockResultSet() *ResultSet {
	return &ResultSet{
		Rows: []Row{
			{Data: []Datum{{VarCharValue: "column1"}, {VarCharValue: "column2"}}},
			{Data: []Datum{{VarCharValue: "value1"}, {VarCharValue: "value2"}}},
		},
		ResultSetMetadata: &ResultSetMetadata{
			ColumnInfo: []ColumnInfo{
				{Name: "column1", Type: "varchar", Nullable: "UNKNOWN"},
				{Name: "column2", Type: "varchar", Nullable: "UNKNOWN"},
			},
		},
	}
}

// StopQueryExecution stops a running query execution.
func (s *MemoryStorage) StopQueryExecution(_ context.Context, queryExecutionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	qe, ok := s.QueryExecutions[queryExecutionID]
	if !ok {
		return &ServiceError{
			Code:    errInvalidRequestException,
			Message: fmt.Sprintf("QueryExecution %s is not found.", queryExecutionID),
		}
	}

	// Only running or queued queries can be stopped.
	if qe.Status.State == QueryExecutionStateRunning || qe.Status.State == QueryExecutionStateQueued {
		now := time.Now()
		qe.Status.State = QueryExecutionStateCancelled
		qe.Status.StateChangeReason = "Query was cancelled by user."
		qe.Status.CompletionDateTime = &now
	}

	s.saveLocked()

	return nil
}

// GetQueryExecution retrieves a query execution by ID.
func (s *MemoryStorage) GetQueryExecution(_ context.Context, queryExecutionID string) (*QueryExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	qe, ok := s.QueryExecutions[queryExecutionID]
	if !ok {
		return nil, &ServiceError{
			Code:    errInvalidRequestException,
			Message: fmt.Sprintf("QueryExecution %s is not found.", queryExecutionID),
		}
	}

	return qe, nil
}

// GetQueryResults retrieves results for a query execution.
func (s *MemoryStorage) GetQueryResults(_ context.Context, queryExecutionID, _ string, _ int32) (*ResultSet, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	qe, ok := s.QueryExecutions[queryExecutionID]
	if !ok {
		return nil, "", &ServiceError{
			Code:    errInvalidRequestException,
			Message: fmt.Sprintf("QueryExecution %s is not found.", queryExecutionID),
		}
	}

	if qe.Status.State != QueryExecutionStateSucceeded {
		return nil, "", &ServiceError{
			Code:    errInvalidRequestException,
			Message: "Query has not yet finished. Current state: " + string(qe.Status.State),
		}
	}

	rs, ok := s.QueryResults[queryExecutionID]
	if !ok {
		// Return empty result set.
		return &ResultSet{
			Rows:              []Row{},
			ResultSetMetadata: &ResultSetMetadata{ColumnInfo: []ColumnInfo{}},
		}, "", nil
	}

	return rs, "", nil
}

// ListQueryExecutions lists query execution IDs.
func (s *MemoryStorage) ListQueryExecutions(_ context.Context, workGroup, _ string, maxResults int32) ([]string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 50
	}

	ids := make([]string, 0)

	for id, qe := range s.QueryExecutions {
		if workGroup != "" && qe.WorkGroup != workGroup {
			continue
		}

		ids = append(ids, id)

		if len(ids) >= int(maxResults) {
			break
		}
	}

	return ids, "", nil
}

// CreateWorkGroup creates a new workgroup.
func (s *MemoryStorage) CreateWorkGroup(_ context.Context, name string, configuration *WorkGroupConfiguration, description string, _ []Tag) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.WorkGroups[name]; ok {
		return &ServiceError{
			Code:    errInvalidRequestException,
			Message: fmt.Sprintf("WorkGroup %s already exists.", name),
		}
	}

	wg := &WorkGroup{
		Name:          name,
		State:         WorkGroupStateEnabled,
		Configuration: configuration,
		Description:   description,
		CreationTime:  time.Now(),
	}

	s.WorkGroups[name] = wg

	s.saveLocked()

	return nil
}

// DeleteWorkGroup deletes a workgroup.
func (s *MemoryStorage) DeleteWorkGroup(_ context.Context, name string, recursiveDelete bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "primary" {
		return &ServiceError{
			Code:    errInvalidRequestException,
			Message: "Cannot delete the primary workgroup.",
		}
	}

	if _, ok := s.WorkGroups[name]; !ok {
		return &ServiceError{
			Code:    errInvalidRequestException,
			Message: fmt.Sprintf("WorkGroup %s is not found.", name),
		}
	}

	// Check if there are any query executions in this workgroup.
	if !recursiveDelete {
		for _, qe := range s.QueryExecutions {
			if qe.WorkGroup == name {
				return &ServiceError{
					Code:    errInvalidRequestException,
					Message: fmt.Sprintf("WorkGroup %s has query executions. Set RecursiveDeleteOption to true to delete.", name),
				}
			}
		}
	}

	// Delete query executions if recursive delete.
	if recursiveDelete {
		for id, qe := range s.QueryExecutions {
			if qe.WorkGroup == name {
				delete(s.QueryExecutions, id)
				delete(s.QueryResults, id)
			}
		}
	}

	delete(s.WorkGroups, name)

	s.saveLocked()

	return nil
}
