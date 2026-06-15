package elasticbeanstalk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "000000000000"

	errAppNotFound = "InvalidParameterValue"
	errAppExists   = "InvalidParameterValue"
	errEnvNotFound = "InvalidParameterValue"
)

// ServiceError represents an Elastic Beanstalk service error.
type ServiceError = service.CodedError

// Storage defines the Elastic Beanstalk storage interface.
type Storage interface {
	CreateApplication(ctx context.Context, req *CreateApplicationInput) (*ApplicationDescription, error)
	DescribeApplications(ctx context.Context, names []string) ([]ApplicationDescription, error)
	UpdateApplication(ctx context.Context, req *UpdateApplicationInput) (*ApplicationDescription, error)
	DeleteApplication(ctx context.Context, name string) error

	CreateEnvironment(ctx context.Context, req *CreateEnvironmentInput) (*EnvironmentDescription, error)
	DescribeEnvironments(ctx context.Context, appName string, envNames []string) ([]EnvironmentDescription, error)
	TerminateEnvironment(ctx context.Context, envName string) (*EnvironmentDescription, error)
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
	mu           sync.RWMutex                       `json:"-"`
	Applications map[string]*ApplicationDescription `json:"applications"`
	Environments map[string]*EnvironmentDescription `json:"environments"`
	region       string
	dataDir      string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Applications: make(map[string]*ApplicationDescription),
		Environments: make(map[string]*EnvironmentDescription),
		region:       region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "elasticbeanstalk", s)
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

	if m.Applications == nil {
		m.Applications = make(map[string]*ApplicationDescription)
	}

	if m.Environments == nil {
		m.Environments = make(map[string]*EnvironmentDescription)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "elasticbeanstalk", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "elasticbeanstalk", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateApplication creates a new application.
func (m *MemoryStorage) CreateApplication(_ context.Context, req *CreateApplicationInput) (*ApplicationDescription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Applications[req.ApplicationName]; exists {
		return nil, &ServiceError{
			Code:    errAppExists,
			Message: fmt.Sprintf("Application %s already exists.", req.ApplicationName),
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	app := &ApplicationDescription{
		ApplicationName: req.ApplicationName,
		Description:     req.Description,
		DateCreated:     now,
		DateUpdated:     now,
		ApplicationArn:  fmt.Sprintf("arn:aws:elasticbeanstalk:%s:%s:application/%s", m.region, defaultAccountID, req.ApplicationName),
	}

	m.Applications[req.ApplicationName] = app

	m.saveLocked()

	return app, nil
}

// DescribeApplications returns applications matching the filter.
func (m *MemoryStorage) DescribeApplications(_ context.Context, names []string) ([]ApplicationDescription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(names) > 0 {
		apps := make([]ApplicationDescription, 0, len(names))

		for _, name := range names {
			if app, exists := m.Applications[name]; exists {
				apps = append(apps, *app)
			}
		}

		return apps, nil
	}

	apps := make([]ApplicationDescription, 0, len(m.Applications))
	for _, app := range m.Applications {
		apps = append(apps, *app)
	}

	return apps, nil
}

// UpdateApplication updates an existing application.
func (m *MemoryStorage) UpdateApplication(_ context.Context, req *UpdateApplicationInput) (*ApplicationDescription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.Applications[req.ApplicationName]
	if !exists {
		return nil, &ServiceError{
			Code:    errAppNotFound,
			Message: fmt.Sprintf("No Application named '%s' found.", req.ApplicationName),
		}
	}

	if req.Description != "" {
		app.Description = req.Description
	}

	app.DateUpdated = time.Now().UTC().Format(time.RFC3339)

	m.saveLocked()

	return app, nil
}

// DeleteApplication deletes an application.
func (m *MemoryStorage) DeleteApplication(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Applications[name]; !exists {
		return &ServiceError{
			Code:    errAppNotFound,
			Message: fmt.Sprintf("No Application named '%s' found.", name),
		}
	}

	delete(m.Applications, name)

	m.saveLocked()

	return nil
}

// CreateEnvironment creates a new environment.
func (m *MemoryStorage) CreateEnvironment(_ context.Context, req *CreateEnvironmentInput) (*EnvironmentDescription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Environments[req.EnvironmentName]; exists {
		return nil, &ServiceError{
			Code:    errEnvNotFound,
			Message: fmt.Sprintf("Environment %s already exists.", req.EnvironmentName),
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	envID := "e-" + uuid.New().String()[:8]
	env := &EnvironmentDescription{
		ApplicationName:   req.ApplicationName,
		EnvironmentID:     envID,
		EnvironmentName:   req.EnvironmentName,
		Description:       req.Description,
		SolutionStackName: req.SolutionStackName,
		Status:            "Ready",
		Health:            "Green",
		DateCreated:       now,
		DateUpdated:       now,
		EnvironmentArn:    fmt.Sprintf("arn:aws:elasticbeanstalk:%s:%s:environment/%s/%s", m.region, defaultAccountID, req.ApplicationName, req.EnvironmentName),
	}

	m.Environments[req.EnvironmentName] = env

	m.saveLocked()

	return env, nil
}

// DescribeEnvironments returns environments matching the filter.
func (m *MemoryStorage) DescribeEnvironments(_ context.Context, appName string, envNames []string) ([]EnvironmentDescription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(envNames) > 0 {
		envs := make([]EnvironmentDescription, 0, len(envNames))

		for _, name := range envNames {
			if env, exists := m.Environments[name]; exists {
				if appName == "" || env.ApplicationName == appName {
					envs = append(envs, *env)
				}
			}
		}

		return envs, nil
	}

	envs := make([]EnvironmentDescription, 0, len(m.Environments))

	for _, env := range m.Environments {
		if appName == "" || env.ApplicationName == appName {
			envs = append(envs, *env)
		}
	}

	return envs, nil
}

// TerminateEnvironment terminates an environment.
func (m *MemoryStorage) TerminateEnvironment(_ context.Context, envName string) (*EnvironmentDescription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, exists := m.Environments[envName]
	if !exists {
		return nil, &ServiceError{
			Code:    errEnvNotFound,
			Message: fmt.Sprintf("No Environment found for EnvironmentName = '%s'.", envName),
		}
	}

	env.Status = "Terminated"

	delete(m.Environments, envName)

	m.saveLocked()

	return env, nil
}
