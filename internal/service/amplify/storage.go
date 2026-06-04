package amplify

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

const (
	defaultAccountID = "000000000000"
	defaultRegion    = "us-east-1"
	defaultPlatform  = "WEB"

	errAppNotFound    = "NotFoundException"
	errBranchNotFound = "NotFoundException"
)

// ServiceError represents an Amplify service error.
type ServiceError struct {
	Code    string
	Message string
	Status  int
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Storage defines the Amplify storage interface.
type Storage interface {
	CreateApp(ctx context.Context, input *CreateAppInput) (*App, error)
	GetApp(ctx context.Context, appID string) (*App, error)
	ListApps(ctx context.Context) ([]App, error)
	UpdateApp(ctx context.Context, appID string, input *UpdateAppInput) (*App, error)
	DeleteApp(ctx context.Context, appID string) (*App, error)
	CreateBranch(ctx context.Context, appID string, input *CreateBranchInput) (*Branch, error)
	GetBranch(ctx context.Context, appID, branchName string) (*Branch, error)
	ListBranches(ctx context.Context, appID string) ([]Branch, error)
	DeleteBranch(ctx context.Context, appID, branchName string) (*Branch, error)
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
	mu       sync.RWMutex                  `json:"-"`
	Apps     map[string]*App               `json:"apps"`
	Branches map[string]map[string]*Branch `json:"branches"` // appID -> branchName -> Branch
	region   string
	dataDir  string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Apps:     make(map[string]*App),
		Branches: make(map[string]map[string]*Branch),
		region:   region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "amplify", s)
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

	if m.Apps == nil {
		m.Apps = make(map[string]*App)
	}

	if m.Branches == nil {
		m.Branches = make(map[string]map[string]*Branch)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "amplify", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "amplify", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateApp creates a new Amplify app.
func (m *MemoryStorage) CreateApp(_ context.Context, input *CreateAppInput) (*App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	appID := uuid.New().String()[:12]
	now := epochNow()

	platform := defaultPlatform
	if input.Platform != "" {
		platform = input.Platform
	}

	app := &App{
		AppArn:                fmt.Sprintf("arn:aws:amplify:%s:%s:apps/%s", m.region, defaultAccountID, appID),
		AppID:                 appID,
		CreateTime:            now,
		DefaultDomain:         fmt.Sprintf("%s.amplifyapp.com", appID),
		Description:           input.Description,
		EnableBasicAuth:       boolValue(input.EnableBasicAuth),
		EnableBranchAutoBuild: boolValue(input.EnableBranchAutoBuild),
		EnvironmentVariables:  ensureMap(input.EnvironmentVariables),
		Name:                  input.Name,
		Platform:              platform,
		Repository:            input.Repository,
		UpdateTime:            now,
		Tags:                  input.Tags,
	}

	m.Apps[appID] = app
	m.Branches[appID] = make(map[string]*Branch)

	m.saveLocked()

	return app, nil
}

// GetApp returns an app by ID.
func (m *MemoryStorage) GetApp(_ context.Context, appID string) (*App, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, exists := m.Apps[appID]
	if !exists {
		return nil, &ServiceError{
			Code:    errAppNotFound,
			Message: fmt.Sprintf("App not found for appId: %s", appID),
			Status:  404,
		}
	}

	return app, nil
}

// ListApps returns all apps.
func (m *MemoryStorage) ListApps(_ context.Context) ([]App, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]App, 0, len(m.Apps))
	for _, app := range m.Apps {
		apps = append(apps, *app)
	}

	return apps, nil
}

// UpdateApp updates an existing app.
func (m *MemoryStorage) UpdateApp(_ context.Context, appID string, input *UpdateAppInput) (*App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.Apps[appID]
	if !exists {
		return nil, &ServiceError{
			Code:    errAppNotFound,
			Message: fmt.Sprintf("App not found for appId: %s", appID),
			Status:  404,
		}
	}

	if input.Name != "" {
		app.Name = input.Name
	}

	if input.Description != "" {
		app.Description = input.Description
	}

	if input.Platform != "" {
		app.Platform = input.Platform
	}

	if input.EnableBasicAuth != nil {
		app.EnableBasicAuth = *input.EnableBasicAuth
	}

	if input.EnableBranchAutoBuild != nil {
		app.EnableBranchAutoBuild = *input.EnableBranchAutoBuild
	}

	if input.EnvironmentVariables != nil {
		app.EnvironmentVariables = input.EnvironmentVariables
	}

	app.UpdateTime = epochNow()

	m.saveLocked()

	return app, nil
}

// DeleteApp deletes an app.
func (m *MemoryStorage) DeleteApp(_ context.Context, appID string) (*App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.Apps[appID]
	if !exists {
		return nil, &ServiceError{
			Code:    errAppNotFound,
			Message: fmt.Sprintf("App not found for appId: %s", appID),
			Status:  404,
		}
	}

	delete(m.Apps, appID)

	delete(m.Branches, appID)

	m.saveLocked()

	return app, nil
}

// CreateBranch creates a new branch for an app.
func (m *MemoryStorage) CreateBranch(_ context.Context, appID string, input *CreateBranchInput) (*Branch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Apps[appID]; !exists {
		return nil, &ServiceError{
			Code:    errAppNotFound,
			Message: fmt.Sprintf("App not found for appId: %s", appID),
			Status:  404,
		}
	}

	now := epochNow()

	stage := "NONE"
	if input.Stage != "" {
		stage = input.Stage
	}

	branch := &Branch{
		ActiveJobID:              "",
		BranchArn:                fmt.Sprintf("arn:aws:amplify:%s:%s:apps/%s/branches/%s", m.region, defaultAccountID, appID, input.BranchName),
		BranchName:               input.BranchName,
		CreateTime:               now,
		CustomDomains:            []string{},
		Description:              input.Description,
		DisplayName:              input.BranchName,
		EnableAutoBuild:          boolValue(input.EnableAutoBuild),
		EnableNotification:       boolValue(input.EnableNotification),
		EnablePullRequestPreview: false,
		EnvironmentVariables:     ensureMap(input.EnvironmentVariables),
		Framework:                input.Framework,
		Stage:                    stage,
		TTL:                      "5",
		TotalNumberOfJobs:        "0",
		UpdateTime:               now,
		Tags:                     input.Tags,
	}

	m.Branches[appID][input.BranchName] = branch

	m.saveLocked()

	return branch, nil
}

// GetBranch returns a branch by name.
func (m *MemoryStorage) GetBranch(_ context.Context, appID, branchName string) (*Branch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	appBranches, exists := m.Branches[appID]
	if !exists {
		return nil, &ServiceError{
			Code:    errAppNotFound,
			Message: fmt.Sprintf("App not found for appId: %s", appID),
			Status:  404,
		}
	}

	branch, exists := appBranches[branchName]
	if !exists {
		return nil, &ServiceError{
			Code:    errBranchNotFound,
			Message: fmt.Sprintf("Branch not found for branchName: %s", branchName),
			Status:  404,
		}
	}

	return branch, nil
}

// ListBranches returns all branches for an app.
func (m *MemoryStorage) ListBranches(_ context.Context, appID string) ([]Branch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Apps[appID]; !exists {
		return nil, &ServiceError{
			Code:    errAppNotFound,
			Message: fmt.Sprintf("App not found for appId: %s", appID),
			Status:  404,
		}
	}

	appBranches := m.Branches[appID]
	branches := make([]Branch, 0, len(appBranches))

	for _, branch := range appBranches {
		branches = append(branches, *branch)
	}

	return branches, nil
}

// DeleteBranch deletes a branch.
func (m *MemoryStorage) DeleteBranch(_ context.Context, appID, branchName string) (*Branch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	appBranches, exists := m.Branches[appID]
	if !exists {
		return nil, &ServiceError{
			Code:    errAppNotFound,
			Message: fmt.Sprintf("App not found for appId: %s", appID),
			Status:  404,
		}
	}

	branch, exists := appBranches[branchName]
	if !exists {
		return nil, &ServiceError{
			Code:    errBranchNotFound,
			Message: fmt.Sprintf("Branch not found for branchName: %s", branchName),
			Status:  404,
		}
	}

	delete(appBranches, branchName)

	m.saveLocked()

	return branch, nil
}

// Helper functions.

func boolValue(b *bool) bool {
	if b == nil {
		return false
	}

	return *b
}

func ensureMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}

	return m
}

func epochNow() float64 {
	return float64(time.Now().Unix())
}
