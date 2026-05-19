package finspace

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	errResourceNotFound = "ResourceNotFoundException"
	errConflict         = "ConflictException"
	errValidation       = "ValidationException"

	defaultRegion = "us-east-1"
	statusCreated = "CREATED"
)

// Storage is the interface for FinSpace storage operations.
type Storage interface {
	// KxEnvironment operations
	CreateKxEnvironment(ctx context.Context, req *CreateKxEnvironmentRequest) (*CreateKxEnvironmentResponse, error)
	GetKxEnvironment(ctx context.Context, environmentID string) (*GetKxEnvironmentResponse, error)
	DeleteKxEnvironment(ctx context.Context, environmentID string) error
	ListKxEnvironments(ctx context.Context, maxResults int, nextToken string) (*ListKxEnvironmentsResponse, error)
	UpdateKxEnvironment(ctx context.Context, req *UpdateKxEnvironmentRequest) (*UpdateKxEnvironmentResponse, error)

	// KxDatabase operations
	CreateKxDatabase(ctx context.Context, req *CreateKxDatabaseRequest) (*CreateKxDatabaseResponse, error)
	GetKxDatabase(ctx context.Context, environmentID, databaseName string) (*GetKxDatabaseResponse, error)
	DeleteKxDatabase(ctx context.Context, environmentID, databaseName string) error
	ListKxDatabases(ctx context.Context, environmentID string, maxResults int, nextToken string) (*ListKxDatabasesResponse, error)
	UpdateKxDatabase(ctx context.Context, req *UpdateKxDatabaseRequest) (*UpdateKxDatabaseResponse, error)

	// KxUser operations
	CreateKxUser(ctx context.Context, req *CreateKxUserRequest) (*CreateKxUserResponse, error)
	GetKxUser(ctx context.Context, environmentID, userName string) (*GetKxUserResponse, error)
	DeleteKxUser(ctx context.Context, environmentID, userName string) error
	ListKxUsers(ctx context.Context, environmentID string, maxResults int, nextToken string) (*ListKxUsersResponse, error)
	UpdateKxUser(ctx context.Context, req *UpdateKxUserRequest) (*UpdateKxUserResponse, error)

	// Tag operations
	TagResource(ctx context.Context, resourceARN string, tags map[string]string) error
	UntagResource(ctx context.Context, resourceARN string, tagKeys []string) error
	ListTagsForResource(ctx context.Context, resourceARN string) (map[string]string, error)
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

// MemoryStorage implements in-memory storage for FinSpace.
type MemoryStorage struct {
	mu           sync.RWMutex                 `json:"-"`
	Environments map[string]*KxEnvironment    `json:"environments"`
	Databases    map[string]*KxDatabase       `json:"databases"`
	Users        map[string]*KxUser           `json:"users"`
	Tags         map[string]map[string]string `json:"tags"`
	accountID    string
	region       string
	dataDir      string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Environments: make(map[string]*KxEnvironment),
		Databases:    make(map[string]*KxDatabase),
		Users:        make(map[string]*KxUser),
		Tags:         make(map[string]map[string]string),
		accountID:    "123456789012",
		region:       region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "finspace", s)
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

	if s.Environments == nil {
		s.Environments = make(map[string]*KxEnvironment)
	}

	if s.Databases == nil {
		s.Databases = make(map[string]*KxDatabase)
	}

	if s.Users == nil {
		s.Users = make(map[string]*KxUser)
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

	_ = storage.SaveBytes(s.dataDir, "finspace", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "finspace", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateKxEnvironment creates a new kdb environment.
func (s *MemoryStorage) CreateKxEnvironment(_ context.Context, req *CreateKxEnvironmentRequest) (*CreateKxEnvironmentResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate name
	for _, env := range s.Environments {
		if env.Name == req.Name {
			return nil, &Error{
				Code:    errConflict,
				Message: fmt.Sprintf("Environment with name %s already exists", req.Name),
			}
		}
	}

	now := float64(time.Now().Unix())
	environmentID := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:finspace:%s:%s:kxEnvironment/%s", s.region, s.accountID, environmentID)

	env := &KxEnvironment{
		AwsAccountID:      s.accountID,
		CreationTimestamp: now,
		Description:       req.Description,
		EnvironmentARN:    arn,
		EnvironmentID:     environmentID,
		KmsKeyID:          req.KmsKeyID,
		Name:              req.Name,
		Status:            statusCreated,
		UpdateTimestamp:   now,
	}

	s.Environments[environmentID] = env

	if len(req.Tags) > 0 {
		s.Tags[arn] = req.Tags
	}

	s.saveLocked()

	return &CreateKxEnvironmentResponse{
		CreationTimestamp: env.CreationTimestamp,
		Description:       env.Description,
		EnvironmentARN:    env.EnvironmentARN,
		EnvironmentID:     env.EnvironmentID,
		KmsKeyID:          env.KmsKeyID,
		Name:              env.Name,
		Status:            env.Status,
	}, nil
}

// GetKxEnvironment retrieves a kdb environment by ID.
func (s *MemoryStorage) GetKxEnvironment(_ context.Context, environmentID string) (*GetKxEnvironmentResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	env, exists := s.Environments[environmentID]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Environment with ID %s not found", environmentID),
		}
	}

	return &GetKxEnvironmentResponse{
		AvailabilityZoneIDs:       env.AvailabilityZoneIDs,
		AwsAccountID:              env.AwsAccountID,
		CertificateARN:            env.CertificateARN,
		CreationTimestamp:         env.CreationTimestamp,
		CustomDNSConfiguration:    env.CustomDNSConfiguration,
		DedicatedServiceAccountID: env.DedicatedServiceAccountID,
		Description:               env.Description,
		DNSStatus:                 env.DNSStatus,
		EnvironmentARN:            env.EnvironmentARN,
		EnvironmentID:             env.EnvironmentID,
		ErrorMessage:              env.ErrorMessage,
		KmsKeyID:                  env.KmsKeyID,
		Name:                      env.Name,
		Status:                    env.Status,
		TgwStatus:                 env.TgwStatus,
		UpdateTimestamp:           env.UpdateTimestamp,
	}, nil
}

// DeleteKxEnvironment deletes a kdb environment.
func (s *MemoryStorage) DeleteKxEnvironment(_ context.Context, environmentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	env, exists := s.Environments[environmentID]
	if !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Environment with ID %s not found", environmentID),
		}
	}

	delete(s.Tags, env.EnvironmentARN)
	delete(s.Environments, environmentID)

	s.saveLocked()

	return nil
}

// ListKxEnvironments lists kdb environments.
func (s *MemoryStorage) ListKxEnvironments(_ context.Context, maxResults int, _ string) (*ListKxEnvironmentsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults == 0 {
		maxResults = 10
	}

	environments := make([]*KxEnvironment, 0, len(s.Environments))

	for _, env := range s.Environments {
		environments = append(environments, env)

		if len(environments) >= maxResults {
			break
		}
	}

	return &ListKxEnvironmentsResponse{
		Environments: environments,
	}, nil
}

// UpdateKxEnvironment updates a kdb environment.
func (s *MemoryStorage) UpdateKxEnvironment(_ context.Context, req *UpdateKxEnvironmentRequest) (*UpdateKxEnvironmentResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	env, exists := s.Environments[req.EnvironmentID]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Environment with ID %s not found", req.EnvironmentID),
		}
	}

	if req.Name != "" {
		env.Name = req.Name
	}

	if req.Description != "" {
		env.Description = req.Description
	}

	env.UpdateTimestamp = float64(time.Now().Unix())

	s.saveLocked()

	return &UpdateKxEnvironmentResponse{
		AvailabilityZoneIDs:       env.AvailabilityZoneIDs,
		AwsAccountID:              env.AwsAccountID,
		CreationTimestamp:         env.CreationTimestamp,
		DedicatedServiceAccountID: env.DedicatedServiceAccountID,
		Description:               env.Description,
		DNSStatus:                 env.DNSStatus,
		EnvironmentARN:            env.EnvironmentARN,
		EnvironmentID:             env.EnvironmentID,
		KmsKeyID:                  env.KmsKeyID,
		Name:                      env.Name,
		Status:                    env.Status,
		TgwStatus:                 env.TgwStatus,
		UpdateTimestamp:           env.UpdateTimestamp,
	}, nil
}

// CreateKxDatabase creates a new kdb database.
func (s *MemoryStorage) CreateKxDatabase(_ context.Context, req *CreateKxDatabaseRequest) (*CreateKxDatabaseResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if environment exists
	if _, exists := s.Environments[req.EnvironmentID]; !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Environment with ID %s not found", req.EnvironmentID),
		}
	}

	key := fmt.Sprintf("%s/%s", req.EnvironmentID, req.DatabaseName)

	// Check for duplicate
	if _, exists := s.Databases[key]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("Database with name %s already exists in environment %s", req.DatabaseName, req.EnvironmentID),
		}
	}

	now := float64(time.Now().Unix())
	arn := fmt.Sprintf("arn:aws:finspace:%s:%s:kxEnvironment/%s/kxDatabase/%s", s.region, s.accountID, req.EnvironmentID, req.DatabaseName)

	db := &KxDatabase{
		CreatedTimestamp:      now,
		DatabaseARN:           arn,
		DatabaseName:          req.DatabaseName,
		Description:           req.Description,
		EnvironmentID:         req.EnvironmentID,
		LastModifiedTimestamp: now,
	}

	s.Databases[key] = db

	if len(req.Tags) > 0 {
		s.Tags[arn] = req.Tags
	}

	s.saveLocked()

	return &CreateKxDatabaseResponse{
		CreatedTimestamp: db.CreatedTimestamp,
		DatabaseARN:      db.DatabaseARN,
		DatabaseName:     db.DatabaseName,
		Description:      db.Description,
		EnvironmentID:    db.EnvironmentID,
	}, nil
}

// GetKxDatabase retrieves a kdb database.
func (s *MemoryStorage) GetKxDatabase(_ context.Context, environmentID, databaseName string) (*GetKxDatabaseResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", environmentID, databaseName)

	db, exists := s.Databases[key]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Database with name %s not found in environment %s", databaseName, environmentID),
		}
	}

	return &GetKxDatabaseResponse{
		CreatedTimestamp:         db.CreatedTimestamp,
		DatabaseARN:              db.DatabaseARN,
		DatabaseName:             db.DatabaseName,
		Description:              db.Description,
		EnvironmentID:            db.EnvironmentID,
		LastCompletedChangesetID: db.LastCompletedChangesetID,
		LastModifiedTimestamp:    db.LastModifiedTimestamp,
		NumBytes:                 db.NumBytes,
		NumChangesets:            db.NumChangesets,
		NumFiles:                 db.NumFiles,
	}, nil
}

// DeleteKxDatabase deletes a kdb database.
func (s *MemoryStorage) DeleteKxDatabase(_ context.Context, environmentID, databaseName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s/%s", environmentID, databaseName)

	db, exists := s.Databases[key]
	if !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Database with name %s not found in environment %s", databaseName, environmentID),
		}
	}

	delete(s.Tags, db.DatabaseARN)
	delete(s.Databases, key)

	s.saveLocked()

	return nil
}

// ListKxDatabases lists kdb databases.
func (s *MemoryStorage) ListKxDatabases(_ context.Context, environmentID string, maxResults int, _ string) (*ListKxDatabasesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults == 0 {
		maxResults = 10
	}

	databases := make([]*KxDatabase, 0)

	for key, db := range s.Databases {
		if db.EnvironmentID == environmentID {
			databases = append(databases, db)

			if len(databases) >= maxResults {
				break
			}
		}

		_ = key
	}

	return &ListKxDatabasesResponse{
		KxDatabases: databases,
	}, nil
}

// UpdateKxDatabase updates a kdb database.
func (s *MemoryStorage) UpdateKxDatabase(_ context.Context, req *UpdateKxDatabaseRequest) (*UpdateKxDatabaseResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s/%s", req.EnvironmentID, req.DatabaseName)

	db, exists := s.Databases[key]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Database with name %s not found in environment %s", req.DatabaseName, req.EnvironmentID),
		}
	}

	if req.Description != "" {
		db.Description = req.Description
	}

	db.LastModifiedTimestamp = float64(time.Now().Unix())

	s.saveLocked()

	return &UpdateKxDatabaseResponse{
		DatabaseARN:           db.DatabaseARN,
		DatabaseName:          db.DatabaseName,
		Description:           db.Description,
		EnvironmentID:         db.EnvironmentID,
		LastModifiedTimestamp: db.LastModifiedTimestamp,
	}, nil
}

// CreateKxUser creates a new kdb user.
func (s *MemoryStorage) CreateKxUser(_ context.Context, req *CreateKxUserRequest) (*CreateKxUserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if environment exists
	if _, exists := s.Environments[req.EnvironmentID]; !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Environment with ID %s not found", req.EnvironmentID),
		}
	}

	key := fmt.Sprintf("%s/%s", req.EnvironmentID, req.UserName)

	// Check for duplicate
	if _, exists := s.Users[key]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("User with name %s already exists in environment %s", req.UserName, req.EnvironmentID),
		}
	}

	now := float64(time.Now().Unix())
	arn := fmt.Sprintf("arn:aws:finspace:%s:%s:kxEnvironment/%s/kxUser/%s", s.region, s.accountID, req.EnvironmentID, req.UserName)

	user := &KxUser{
		CreateTimestamp: now,
		IamRole:         req.IamRole,
		UpdateTimestamp: now,
		UserARN:         arn,
		UserName:        req.UserName,
	}

	s.Users[key] = user

	if len(req.Tags) > 0 {
		s.Tags[arn] = req.Tags
	}

	s.saveLocked()

	return &CreateKxUserResponse{
		EnvironmentID: req.EnvironmentID,
		IamRole:       user.IamRole,
		UserARN:       user.UserARN,
		UserName:      user.UserName,
	}, nil
}

// GetKxUser retrieves a kdb user.
func (s *MemoryStorage) GetKxUser(_ context.Context, environmentID, userName string) (*GetKxUserResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", environmentID, userName)

	user, exists := s.Users[key]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("User with name %s not found in environment %s", userName, environmentID),
		}
	}

	return &GetKxUserResponse{
		EnvironmentID: environmentID,
		IamRole:       user.IamRole,
		UserARN:       user.UserARN,
		UserName:      user.UserName,
	}, nil
}

// DeleteKxUser deletes a kdb user.
func (s *MemoryStorage) DeleteKxUser(_ context.Context, environmentID, userName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s/%s", environmentID, userName)

	user, exists := s.Users[key]
	if !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("User with name %s not found in environment %s", userName, environmentID),
		}
	}

	delete(s.Tags, user.UserARN)
	delete(s.Users, key)

	s.saveLocked()

	return nil
}

// ListKxUsers lists kdb users.
func (s *MemoryStorage) ListKxUsers(_ context.Context, environmentID string, maxResults int, _ string) (*ListKxUsersResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults == 0 {
		maxResults = 10
	}

	users := make([]*KxUser, 0)

	for key, user := range s.Users {
		// Check if the key starts with the environmentID
		if len(key) > len(environmentID) && key[:len(environmentID)] == environmentID && key[len(environmentID)] == '/' {
			users = append(users, user)

			if len(users) >= maxResults {
				break
			}
		}
	}

	return &ListKxUsersResponse{
		Users: users,
	}, nil
}

// UpdateKxUser updates a kdb user.
func (s *MemoryStorage) UpdateKxUser(_ context.Context, req *UpdateKxUserRequest) (*UpdateKxUserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s/%s", req.EnvironmentID, req.UserName)

	user, exists := s.Users[key]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("User with name %s not found in environment %s", req.UserName, req.EnvironmentID),
		}
	}

	if req.IamRole != "" {
		user.IamRole = req.IamRole
	}

	user.UpdateTimestamp = float64(time.Now().Unix())

	s.saveLocked()

	return &UpdateKxUserResponse{
		EnvironmentID: req.EnvironmentID,
		IamRole:       user.IamRole,
		UserARN:       user.UserARN,
		UserName:      user.UserName,
	}, nil
}

// TagResource adds tags to a resource.
func (s *MemoryStorage) TagResource(_ context.Context, resourceARN string, tags map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Tags[resourceARN] == nil {
		s.Tags[resourceARN] = make(map[string]string)
	}

	maps.Copy(s.Tags[resourceARN], tags)

	s.saveLocked()

	return nil
}

// UntagResource removes tags from a resource.
func (s *MemoryStorage) UntagResource(_ context.Context, resourceARN string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Tags[resourceARN] == nil {
		return nil
	}

	for _, key := range tagKeys {
		delete(s.Tags[resourceARN], key)
	}

	s.saveLocked()

	return nil
}

// ListTagsForResource lists tags for a resource.
func (s *MemoryStorage) ListTagsForResource(_ context.Context, resourceARN string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := s.Tags[resourceARN]
	if tags == nil {
		tags = make(map[string]string)
	}

	return tags, nil
}
