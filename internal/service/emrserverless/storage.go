package emrserverless

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Storage defines the interface for EMR Serverless storage operations.
type Storage interface {
	// Application operations.
	CreateApplication(ctx context.Context, req *CreateApplicationInput) (*Application, error)
	GetApplication(ctx context.Context, applicationID string) (*Application, error)
	ListApplications(ctx context.Context, req *ListApplicationsInput) (*ListApplicationsOutput, error)
	UpdateApplication(ctx context.Context, req *UpdateApplicationInput) (*Application, error)
	DeleteApplication(ctx context.Context, applicationID string) error
	StartApplication(ctx context.Context, applicationID string) error
	StopApplication(ctx context.Context, applicationID string) error

	// Job run operations.
	StartJobRun(ctx context.Context, req *StartJobRunInput) (*JobRun, error)
	GetJobRun(ctx context.Context, applicationID, jobRunID string) (*JobRun, error)
	ListJobRuns(ctx context.Context, req *ListJobRunsInput) (*ListJobRunsOutput, error)
	CancelJobRun(ctx context.Context, applicationID, jobRunID string) (*JobRun, error)
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

// MemoryStorage implements the Storage interface using in-memory storage.
type MemoryStorage struct {
	mu           sync.RWMutex                  `json:"-"`
	Applications map[string]*Application       `json:"applications"`
	JobRuns      map[string]map[string]*JobRun `json:"jobRuns"`
	dataDir      string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Applications: make(map[string]*Application),
		JobRuns:      make(map[string]map[string]*JobRun),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "emrserverless", s)
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
		m.Applications = make(map[string]*Application)
	}

	if m.JobRuns == nil {
		m.JobRuns = make(map[string]map[string]*JobRun)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "emrserverless", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "emrserverless", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

const (
	accountID     = "123456789012"
	defaultRegion = "us-east-1"
)

var region = defaultRegion //nolint:gochecknoglobals // set once at init

func init() {
	if v := os.Getenv("AWS_DEFAULT_REGION"); v != "" {
		region = v
	}
}

// generateApplicationID generates a unique application ID.
func generateApplicationID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")[:14]
}

// generateJobRunID generates a unique job run ID.
func generateJobRunID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")[:14]
}

// generateApplicationArn generates an ARN for an application.
func generateApplicationArn(applicationID string) string {
	return fmt.Sprintf("arn:aws:emr-serverless:%s:%s:/applications/%s", region, accountID, applicationID)
}

// generateJobRunArn generates an ARN for a job run.
func generateJobRunArn(applicationID, jobRunID string) string {
	return fmt.Sprintf("arn:aws:emr-serverless:%s:%s:/applications/%s/jobruns/%s", region, accountID, applicationID, jobRunID)
}

// CreateApplication creates a new application.
//
//nolint:funlen // validation and struct initialization require more lines
func (m *MemoryStorage) CreateApplication(_ context.Context, req *CreateApplicationInput) (*Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate required fields.
	if req.Type == "" {
		return nil, &Error{
			Code:    errValidationException,
			Message: "Type is required",
		}
	}

	if req.ReleaseLabel == "" {
		return nil, &Error{
			Code:    errValidationException,
			Message: "ReleaseLabel is required",
		}
	}

	now := time.Now()
	applicationID := generateApplicationID()
	arn := generateApplicationArn(applicationID)

	architecture := req.Architecture
	if architecture == "" {
		architecture = ArchitectureX8664
	}

	// Default auto stop configuration.
	autoStopConfig := req.AutoStopConfiguration
	if autoStopConfig == nil {
		autoStopConfig = &AutoStopConfiguration{
			Enabled:            true,
			IdleTimeoutMinutes: 15,
		}
	}

	// Default auto start configuration.
	autoStartConfig := req.AutoStartConfiguration
	if autoStartConfig == nil {
		autoStartConfig = &AutoStartConfiguration{
			Enabled: true,
		}
	}

	app := &Application{
		ApplicationID:           applicationID,
		Arn:                     arn,
		Name:                    req.Name,
		Type:                    req.Type,
		ReleaseLabel:            req.ReleaseLabel,
		State:                   ApplicationStateCreated,
		Architecture:            architecture,
		InitialCapacity:         req.InitialCapacity,
		MaximumCapacity:         req.MaximumCapacity,
		AutoStartConfiguration:  autoStartConfig,
		AutoStopConfiguration:   autoStopConfig,
		NetworkConfiguration:    req.NetworkConfiguration,
		MonitoringConfiguration: req.MonitoringConfiguration,
		Tags:                    req.Tags,
		CreatedAt:               AWSTimestamp{Time: now},
		UpdatedAt:               AWSTimestamp{Time: now},
	}

	m.Applications[applicationID] = app
	m.JobRuns[applicationID] = make(map[string]*JobRun)

	m.saveLocked()

	return app, nil
}

// GetApplication retrieves an application by ID.
func (m *MemoryStorage) GetApplication(_ context.Context, applicationID string) (*Application, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, exists := m.Applications[applicationID]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Application %s does not exist", applicationID),
		}
	}

	return app, nil
}

// ListApplications lists applications with optional filters.
func (m *MemoryStorage) ListApplications(_ context.Context, req *ListApplicationsInput) (*ListApplicationsOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := req.MaxResults
	if limit <= 0 || limit > maxPageLimit {
		limit = defaultPageLimit
	}

	stateFilter := make(map[string]bool)
	for _, state := range req.States {
		stateFilter[state] = true
	}

	var summaries []*ApplicationSummary

	for _, app := range m.Applications {
		// Apply state filter.
		if len(stateFilter) > 0 && !stateFilter[app.State] {
			continue
		}

		summary := &ApplicationSummary{
			ApplicationID: app.ApplicationID,
			Arn:           app.Arn,
			Name:          app.Name,
			Type:          app.Type,
			ReleaseLabel:  app.ReleaseLabel,
			State:         app.State,
			StateDetails:  app.StateDetails,
			Architecture:  app.Architecture,
			CreatedAt:     app.CreatedAt,
			UpdatedAt:     app.UpdatedAt,
		}

		summaries = append(summaries, summary)

		if int32(len(summaries)) >= limit { //nolint:gosec // G115: len(summaries) is bounded by limit which is int32
			break
		}
	}

	return &ListApplicationsOutput{
		Applications: summaries,
	}, nil
}

// UpdateApplication updates an existing application.
func (m *MemoryStorage) UpdateApplication(_ context.Context, req *UpdateApplicationInput) (*Application, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.Applications[req.ApplicationID]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Application %s does not exist", req.ApplicationID),
		}
	}

	// Application must be in CREATED or STOPPED state to update.
	if app.State != ApplicationStateCreated && app.State != ApplicationStateStopped {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("Application %s is not in a valid state for update", req.ApplicationID),
		}
	}

	// Update fields.
	if req.Architecture != "" {
		app.Architecture = req.Architecture
	}

	if req.ReleaseLabel != "" {
		app.ReleaseLabel = req.ReleaseLabel
	}

	if req.InitialCapacity != nil {
		app.InitialCapacity = req.InitialCapacity
	}

	if req.MaximumCapacity != nil {
		app.MaximumCapacity = req.MaximumCapacity
	}

	if req.AutoStartConfiguration != nil {
		app.AutoStartConfiguration = req.AutoStartConfiguration
	}

	if req.AutoStopConfiguration != nil {
		app.AutoStopConfiguration = req.AutoStopConfiguration
	}

	if req.NetworkConfiguration != nil {
		app.NetworkConfiguration = req.NetworkConfiguration
	}

	if req.MonitoringConfiguration != nil {
		app.MonitoringConfiguration = req.MonitoringConfiguration
	}

	app.UpdatedAt = AWSTimestamp{Time: time.Now()}

	m.saveLocked()

	return app, nil
}

// DeleteApplication deletes an application.
func (m *MemoryStorage) DeleteApplication(_ context.Context, applicationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.Applications[applicationID]
	if !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Application %s does not exist", applicationID),
		}
	}

	// Application must be in CREATED or STOPPED state to delete.
	if app.State != ApplicationStateCreated && app.State != ApplicationStateStopped {
		return &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("Application %s is not in a valid state for deletion", applicationID),
		}
	}

	// Check for running job runs.
	for _, jr := range m.JobRuns[applicationID] {
		if jr.State == JobRunStateRunning || jr.State == JobRunStatePending || jr.State == JobRunStateScheduled {
			return &Error{
				Code:    errConflictException,
				Message: fmt.Sprintf("Application %s has running job runs", applicationID),
			}
		}
	}

	// Mark as terminated and delete.
	app.State = ApplicationStateTerminated
	app.UpdatedAt = AWSTimestamp{Time: time.Now()}

	delete(m.Applications, applicationID)
	delete(m.JobRuns, applicationID)

	m.saveLocked()

	return nil
}

// StartApplication starts an application.
func (m *MemoryStorage) StartApplication(_ context.Context, applicationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.Applications[applicationID]
	if !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Application %s does not exist", applicationID),
		}
	}

	// Application must be in CREATED or STOPPED state to start.
	if app.State != ApplicationStateCreated && app.State != ApplicationStateStopped {
		return &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("Application %s is not in a valid state to start", applicationID),
		}
	}

	// For simulation, immediately set to STARTED.
	app.State = ApplicationStateStarted
	app.UpdatedAt = AWSTimestamp{Time: time.Now()}

	m.saveLocked()

	return nil
}

// StopApplication stops an application.
func (m *MemoryStorage) StopApplication(_ context.Context, applicationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.Applications[applicationID]
	if !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Application %s does not exist", applicationID),
		}
	}

	// Application must be in STARTED state to stop.
	if app.State != ApplicationStateStarted {
		return &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("Application %s is not in a valid state to stop", applicationID),
		}
	}

	// For simulation, immediately set to STOPPED.
	app.State = ApplicationStateStopped
	app.UpdatedAt = AWSTimestamp{Time: time.Now()}

	m.saveLocked()

	return nil
}

// StartJobRun starts a new job run.
//
//nolint:funlen // validation and struct initialization require more lines
func (m *MemoryStorage) StartJobRun(_ context.Context, req *StartJobRunInput) (*JobRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.Applications[req.ApplicationID]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Application %s does not exist", req.ApplicationID),
		}
	}

	// Validate required fields.
	if req.ExecutionRoleArn == "" {
		return nil, &Error{
			Code:    errValidationException,
			Message: "ExecutionRoleArn is required",
		}
	}

	if req.JobDriver == nil {
		return nil, &Error{
			Code:    errValidationException,
			Message: "JobDriver is required",
		}
	}

	// If auto-start is enabled and app is not started, start it.
	if app.AutoStartConfiguration != nil && app.AutoStartConfiguration.Enabled {
		if app.State == ApplicationStateCreated || app.State == ApplicationStateStopped {
			app.State = ApplicationStateStarted
			app.UpdatedAt = AWSTimestamp{Time: time.Now()}
		}
	}

	// Application must be in STARTED state to run jobs.
	if app.State != ApplicationStateStarted {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("Application %s is not in STARTED state", req.ApplicationID),
		}
	}

	now := time.Now()
	jobRunID := generateJobRunID()
	arn := generateJobRunArn(req.ApplicationID, jobRunID)

	mode := req.Mode
	if mode == "" {
		mode = JobRunModeBatch
	}

	jobRun := &JobRun{
		ApplicationID:           req.ApplicationID,
		JobRunID:                jobRunID,
		Arn:                     arn,
		Name:                    req.Name,
		State:                   JobRunStateRunning, // Immediately set to running for simulation
		Mode:                    mode,
		ReleaseLabel:            app.ReleaseLabel,
		ExecutionRole:           req.ExecutionRoleArn,
		JobDriver:               req.JobDriver,
		ConfigurationOverrides:  req.ConfigurationOverrides,
		Tags:                    req.Tags,
		ExecutionTimeoutMinutes: req.ExecutionTimeoutMinutes,
		CreatedAt:               AWSTimestamp{Time: now},
		UpdatedAt:               AWSTimestamp{Time: now},
		CreatedBy:               fmt.Sprintf("arn:aws:iam::%s:user/test-user", accountID),
	}

	m.JobRuns[req.ApplicationID][jobRunID] = jobRun

	m.saveLocked()

	return jobRun, nil
}

// GetJobRun retrieves a job run by ID.
func (m *MemoryStorage) GetJobRun(_ context.Context, applicationID, jobRunID string) (*JobRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Applications[applicationID]; !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Application %s does not exist", applicationID),
		}
	}

	jobRun, exists := m.JobRuns[applicationID][jobRunID]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("JobRun %s does not exist", jobRunID),
		}
	}

	return jobRun, nil
}

// ListJobRuns lists job runs with optional filters.
func (m *MemoryStorage) ListJobRuns(_ context.Context, req *ListJobRunsInput) (*ListJobRunsOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.Applications[req.ApplicationID]; !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Application %s does not exist", req.ApplicationID),
		}
	}

	limit := req.MaxResults
	if limit <= 0 || limit > maxPageLimit {
		limit = defaultPageLimit
	}

	stateFilter := make(map[string]bool)
	for _, state := range req.States {
		stateFilter[state] = true
	}

	var summaries []*JobRunSummary

	for _, jr := range m.JobRuns[req.ApplicationID] {
		// Apply state filter.
		if len(stateFilter) > 0 && !stateFilter[jr.State] {
			continue
		}

		// Apply mode filter.
		if req.Mode != "" && jr.Mode != req.Mode {
			continue
		}

		summary := &JobRunSummary{
			ApplicationID: jr.ApplicationID,
			JobRunID:      jr.JobRunID,
			Arn:           jr.Arn,
			Name:          jr.Name,
			State:         jr.State,
			StateDetails:  jr.StateDetails,
			Mode:          jr.Mode,
			ReleaseLabel:  jr.ReleaseLabel,
			CreatedAt:     jr.CreatedAt,
			UpdatedAt:     jr.UpdatedAt,
			CreatedBy:     jr.CreatedBy,
		}

		summaries = append(summaries, summary)

		if int32(len(summaries)) >= limit { //nolint:gosec // G115: len(summaries) is bounded by limit which is int32
			break
		}
	}

	return &ListJobRunsOutput{
		JobRuns: summaries,
	}, nil
}

// CancelJobRun cancels a job run.
func (m *MemoryStorage) CancelJobRun(_ context.Context, applicationID, jobRunID string) (*JobRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Applications[applicationID]; !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Application %s does not exist", applicationID),
		}
	}

	jobRun, exists := m.JobRuns[applicationID][jobRunID]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("JobRun %s does not exist", jobRunID),
		}
	}

	// Job run must be in a cancellable state.
	if jobRun.State != JobRunStateRunning && jobRun.State != JobRunStatePending &&
		jobRun.State != JobRunStateScheduled && jobRun.State != JobRunStateQueued {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("JobRun %s is not in a cancellable state", jobRunID),
		}
	}

	// For simulation, immediately set to CANCELLED.
	jobRun.State = JobRunStateCancelled
	jobRun.UpdatedAt = AWSTimestamp{Time: time.Now()}

	m.saveLocked()

	return jobRun, nil
}

// Error represents an EMR Serverless service error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ErrorCode returns the error code.
func (e *Error) ErrorCode() string {
	return e.Code
}

// HTTPStatusCode returns the HTTP status code for the error.
func (e *Error) HTTPStatusCode() int {
	switch e.Code {
	case errConflictException:
		return 409
	case errResourceNotFound:
		return 404
	case errValidationException:
		return 400
	case errServiceQuotaExceeded:
		return 402
	default:
		return 500
	}
}
