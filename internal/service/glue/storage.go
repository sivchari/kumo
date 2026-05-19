package glue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errEntityNotFound = "EntityNotFoundException"
	errAlreadyExists  = "AlreadyExistsException"
	errInvalidInput   = "InvalidInputException"
)

const defaultCatalogID = "default"

// Storage defines the interface for Glue storage operations.
type Storage interface {
	CreateDatabase(ctx context.Context, catalogID string, input *DatabaseInput) error
	GetDatabase(ctx context.Context, catalogID, name string) (*Database, error)
	GetDatabases(ctx context.Context, catalogID string, maxResults int32, nextToken string) ([]*Database, string, error)
	DeleteDatabase(ctx context.Context, catalogID, name string) error

	CreateTable(ctx context.Context, catalogID, databaseName string, input *TableInput) error
	GetTable(ctx context.Context, catalogID, databaseName, name string) (*Table, error)
	GetTables(ctx context.Context, catalogID, databaseName string, maxResults int32, nextToken string) ([]*Table, string, error)
	DeleteTable(ctx context.Context, catalogID, databaseName, name string) error

	CreateJob(ctx context.Context, input *CreateJobInput) (*Job, error)
	DeleteJob(ctx context.Context, jobName string) error
	StartJobRun(ctx context.Context, input *StartJobRunInput) (*JobRun, error)

	GetTags(ctx context.Context, resourceArn string) (map[string]string, error)
	TagResource(ctx context.Context, resourceArn string, tagsToAdd map[string]string) error
	UntagResource(ctx context.Context, resourceArn string, tagsToRemove []string) error
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

// MemoryStorage implements Storage with in-memory data structures.
type MemoryStorage struct {
	mu        sync.RWMutex                 `json:"-"`
	Databases map[string]*Database         `json:"databases"` // key: catalogID/databaseName
	Tables    map[string]*Table            `json:"tables"`    // key: catalogID/databaseName/tableName
	Jobs      map[string]*Job              `json:"jobs"`      // key: jobName
	JobRuns   map[string]*JobRun           `json:"jobRuns"`   // key: jobRunID
	Tags      map[string]map[string]string `json:"tags"`      // key: resourceArn -> tags
	dataDir   string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Databases: make(map[string]*Database),
		Tables:    make(map[string]*Table),
		Jobs:      make(map[string]*Job),
		JobRuns:   make(map[string]*JobRun),
		Tags:      make(map[string]map[string]string),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "glue", s)
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

	if s.Databases == nil {
		s.Databases = make(map[string]*Database)
	}

	if s.Tables == nil {
		s.Tables = make(map[string]*Table)
	}

	if s.Jobs == nil {
		s.Jobs = make(map[string]*Job)
	}

	if s.JobRuns == nil {
		s.JobRuns = make(map[string]*JobRun)
	}

	if s.Tags == nil {
		s.Tags = make(map[string]map[string]string)
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

	_ = storage.SaveBytes(s.dataDir, "glue", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "glue", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

func databaseKey(catalogID, name string) string {
	if catalogID == "" {
		catalogID = defaultCatalogID
	}

	return catalogID + "/" + name
}

func tableKey(catalogID, databaseName, tableName string) string {
	if catalogID == "" {
		catalogID = defaultCatalogID
	}

	return catalogID + "/" + databaseName + "/" + tableName
}

// CreateDatabase creates a new database.
func (s *MemoryStorage) CreateDatabase(_ context.Context, catalogID string, input *DatabaseInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.Name == "" {
		return &Error{
			Code:    errInvalidInput,
			Message: "Database name is required",
		}
	}

	key := databaseKey(catalogID, input.Name)

	if _, exists := s.Databases[key]; exists {
		return &Error{
			Code:    errAlreadyExists,
			Message: fmt.Sprintf("Database %s already exists", input.Name),
		}
	}

	db := &Database{
		Name:            input.Name,
		Description:     input.Description,
		LocationURI:     input.LocationURI,
		Parameters:      input.Parameters,
		CreateTime:      time.Now(),
		CatalogID:       catalogID,
		CreateTableMode: input.CreateTableMode,
	}

	s.Databases[key] = db

	s.saveLocked()

	return nil
}

// GetDatabase retrieves a database.
func (s *MemoryStorage) GetDatabase(_ context.Context, catalogID, name string) (*Database, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := databaseKey(catalogID, name)
	db, exists := s.Databases[key]

	if !exists {
		return nil, &Error{
			Code:    errEntityNotFound,
			Message: fmt.Sprintf("Database %s not found", name),
		}
	}

	return db, nil
}

// GetDatabases lists all databases.
func (s *MemoryStorage) GetDatabases(_ context.Context, catalogID string, maxResults int32, _ string) ([]*Database, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 100
	}

	if catalogID == "" {
		catalogID = defaultCatalogID
	}

	prefix := catalogID + "/"
	databases := make([]*Database, 0)

	for key, db := range s.Databases {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			databases = append(databases, db)

			if len(databases) >= int(maxResults) {
				break
			}
		}
	}

	return databases, "", nil
}

// DeleteDatabase deletes a database.
func (s *MemoryStorage) DeleteDatabase(_ context.Context, catalogID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := databaseKey(catalogID, name)

	if _, exists := s.Databases[key]; !exists {
		return &Error{
			Code:    errEntityNotFound,
			Message: fmt.Sprintf("Database %s not found", name),
		}
	}

	delete(s.Databases, key)

	s.saveLocked()

	return nil
}

// CreateTable creates a new table.
func (s *MemoryStorage) CreateTable(_ context.Context, catalogID, databaseName string, input *TableInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.Name == "" {
		return &Error{
			Code:    errInvalidInput,
			Message: "Table name is required",
		}
	}

	// Check if database exists.
	dbKey := databaseKey(catalogID, databaseName)
	if _, exists := s.Databases[dbKey]; !exists {
		return &Error{
			Code:    errEntityNotFound,
			Message: fmt.Sprintf("Database %s not found", databaseName),
		}
	}

	key := tableKey(catalogID, databaseName, input.Name)

	if _, exists := s.Tables[key]; exists {
		return &Error{
			Code:    errAlreadyExists,
			Message: fmt.Sprintf("Table %s already exists", input.Name),
		}
	}

	now := time.Now()
	table := &Table{
		Name:              input.Name,
		DatabaseName:      databaseName,
		Description:       input.Description,
		Owner:             input.Owner,
		CreateTime:        now,
		UpdateTime:        now,
		Retention:         input.Retention,
		StorageDescriptor: input.StorageDescriptor,
		PartitionKeys:     input.PartitionKeys,
		ViewOriginalText:  input.ViewOriginalText,
		ViewExpandedText:  input.ViewExpandedText,
		TableType:         input.TableType,
		Parameters:        input.Parameters,
		CatalogID:         catalogID,
	}

	s.Tables[key] = table

	s.saveLocked()

	return nil
}

// GetTable retrieves a table.
func (s *MemoryStorage) GetTable(_ context.Context, catalogID, databaseName, name string) (*Table, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := tableKey(catalogID, databaseName, name)
	table, exists := s.Tables[key]

	if !exists {
		return nil, &Error{
			Code:    errEntityNotFound,
			Message: fmt.Sprintf("Table %s not found", name),
		}
	}

	return table, nil
}

// GetTables lists tables in a database.
func (s *MemoryStorage) GetTables(_ context.Context, catalogID, databaseName string, maxResults int32, _ string) ([]*Table, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 100
	}

	if catalogID == "" {
		catalogID = defaultCatalogID
	}

	prefix := catalogID + "/" + databaseName + "/"
	tables := make([]*Table, 0)

	for key, table := range s.Tables {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			tables = append(tables, table)

			if len(tables) >= int(maxResults) {
				break
			}
		}
	}

	return tables, "", nil
}

// DeleteTable deletes a table.
func (s *MemoryStorage) DeleteTable(_ context.Context, catalogID, databaseName, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := tableKey(catalogID, databaseName, name)

	if _, exists := s.Tables[key]; !exists {
		return &Error{
			Code:    errEntityNotFound,
			Message: fmt.Sprintf("Table %s not found", name),
		}
	}

	delete(s.Tables, key)

	s.saveLocked()

	return nil
}

// CreateJob creates a new job.
func (s *MemoryStorage) CreateJob(_ context.Context, input *CreateJobInput) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.Name == "" {
		return nil, &Error{
			Code:    errInvalidInput,
			Message: "Job name is required",
		}
	}

	if input.Role == "" {
		return nil, &Error{
			Code:    errInvalidInput,
			Message: "Role is required",
		}
	}

	if _, exists := s.Jobs[input.Name]; exists {
		return nil, &Error{
			Code:    errAlreadyExists,
			Message: fmt.Sprintf("Job %s already exists", input.Name),
		}
	}

	now := time.Now()
	job := &Job{
		Name:                    input.Name,
		Description:             input.Description,
		Role:                    input.Role,
		Command:                 input.Command,
		DefaultArguments:        input.DefaultArguments,
		NonOverridableArguments: input.NonOverridableArguments,
		MaxRetries:              input.MaxRetries,
		AllocatedCapacity:       input.AllocatedCapacity,
		Timeout:                 input.Timeout,
		MaxCapacity:             input.MaxCapacity,
		WorkerType:              input.WorkerType,
		NumberOfWorkers:         input.NumberOfWorkers,
		GlueVersion:             input.GlueVersion,
		CreatedOn:               now,
		LastModifiedOn:          now,
		ExecutionProperty:       input.ExecutionProperty,
	}

	s.Jobs[input.Name] = job

	s.saveLocked()

	return job, nil
}

// DeleteJob deletes a job.
func (s *MemoryStorage) DeleteJob(_ context.Context, jobName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Jobs[jobName]; !exists {
		return &Error{
			Code:    errEntityNotFound,
			Message: fmt.Sprintf("Job %s not found", jobName),
		}
	}

	delete(s.Jobs, jobName)

	s.saveLocked()

	return nil
}

// StartJobRun starts a job run.
func (s *MemoryStorage) StartJobRun(_ context.Context, input *StartJobRunInput) (*JobRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.Jobs[input.JobName]
	if !exists {
		return nil, &Error{
			Code:    errEntityNotFound,
			Message: fmt.Sprintf("Job %s not found", input.JobName),
		}
	}

	runID := input.JobRunID
	if runID == "" {
		runID = "jr_" + uuid.New().String()
	}

	now := time.Now()
	jobRun := &JobRun{
		ID:                runID,
		Attempt:           0,
		JobName:           input.JobName,
		StartedOn:         now,
		LastModifiedOn:    now,
		JobRunState:       "RUNNING",
		Arguments:         input.Arguments,
		AllocatedCapacity: input.AllocatedCapacity,
		Timeout:           input.Timeout,
		MaxCapacity:       input.MaxCapacity,
		WorkerType:        input.WorkerType,
		NumberOfWorkers:   input.NumberOfWorkers,
		GlueVersion:       job.GlueVersion,
	}

	s.JobRuns[runID] = jobRun

	s.saveLocked()

	return jobRun, nil
}

// GetTags returns the tags for a resource.
func (s *MemoryStorage) GetTags(_ context.Context, resourceArn string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags, exists := s.Tags[resourceArn]
	if !exists {
		return map[string]string{}, nil
	}

	// Return a copy to avoid mutation.
	result := make(map[string]string, len(tags))
	for k, v := range tags {
		result[k] = v
	}

	return result, nil
}

// TagResource adds or overwrites tags on a resource.
func (s *MemoryStorage) TagResource(_ context.Context, resourceArn string, tagsToAdd map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Tags[resourceArn] == nil {
		s.Tags[resourceArn] = make(map[string]string)
	}

	for k, v := range tagsToAdd {
		s.Tags[resourceArn][k] = v
	}

	s.saveLocked()

	return nil
}

// UntagResource removes tags from a resource.
func (s *MemoryStorage) UntagResource(_ context.Context, resourceArn string, tagsToRemove []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tags, exists := s.Tags[resourceArn]
	if !exists {
		return nil
	}

	for _, key := range tagsToRemove {
		delete(tags, key)
	}

	s.saveLocked()

	return nil
}
