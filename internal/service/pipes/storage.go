package pipes

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// Storage defines the interface for pipe storage operations.
type Storage interface {
	// CreatePipe creates a new pipe.
	CreatePipe(ctx context.Context, req *CreatePipeInput) (*Pipe, error)

	// DescribePipe retrieves a pipe by name.
	DescribePipe(ctx context.Context, name string) (*Pipe, error)

	// UpdatePipe updates an existing pipe.
	UpdatePipe(ctx context.Context, req *UpdatePipeInput) (*Pipe, error)

	// DeletePipe deletes a pipe by name.
	DeletePipe(ctx context.Context, name string) (*Pipe, error)

	// ListPipes lists pipes with optional filters.
	ListPipes(ctx context.Context, req *ListPipesInput) (*ListPipesOutput, error)

	// StartPipe starts a stopped pipe.
	StartPipe(ctx context.Context, name string) (*Pipe, error)

	// StopPipe stops a running pipe.
	StopPipe(ctx context.Context, name string) (*Pipe, error)

	// TagResource adds tags to a pipe.
	TagResource(ctx context.Context, arn string, tags map[string]string) error

	// UntagResource removes tags from a pipe.
	UntagResource(ctx context.Context, arn string, tagKeys []string) error

	// ListTagsForResource lists tags for a pipe.
	ListTagsForResource(ctx context.Context, arn string) (map[string]string, error)
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
	mu      sync.RWMutex     `json:"-"`
	Pipes   map[string]*Pipe `json:"pipes"` // keyed by name
	dataDir string

	// publisher is the installed EventBridge target (nil until wiring runs).
	publisher EventPublisher
	// runCtx is the parent context for all per-pipe poller goroutines;
	// cancelled by Close.
	runCtx    context.Context //nolint:containedctx // intentional: scopes poller goroutines to storage lifetime
	runCancel context.CancelFunc
	// pollers maps a running pipe name to its poller cancel func.
	pollers map[string]context.CancelFunc
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	ctx, cancel := context.WithCancel(context.Background())

	s := &MemoryStorage{
		Pipes:     make(map[string]*Pipe),
		runCtx:    ctx,
		runCancel: cancel,
		pollers:   make(map[string]context.CancelFunc),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "pipes", s)
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

	if m.Pipes == nil {
		m.Pipes = make(map[string]*Pipe)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "pipes", m.MarshalJSON)
}

// Close stops all poller goroutines, then saves the storage state to disk if
// persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.runCancel != nil {
		m.runCancel()
	}

	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "pipes", m); err != nil {
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

// generatePipeArn generates an ARN for a pipe.
func generatePipeArn(name string) string {
	return fmt.Sprintf("arn:aws:pipes:%s:%s:pipe/%s", region, accountID, name)
}

// CreatePipe creates a new pipe.
func (m *MemoryStorage) CreatePipe(_ context.Context, req *CreatePipeInput) (*Pipe, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if pipe already exists.
	if _, exists := m.Pipes[req.Name]; exists {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("Pipe with name %s already exists", req.Name),
		}
	}

	// Validate required fields (first empty wins).
	for _, f := range []struct{ name, value string }{
		{"Source", req.Source},
		{"Target", req.Target},
		{"RoleArn", req.RoleArn},
	} {
		if f.value == "" {
			return nil, &Error{Code: errValidationException, Message: f.name + " is required"}
		}
	}

	now := time.Now()

	desiredState := req.DesiredState
	if desiredState == "" {
		desiredState = DesiredStateRunning
	}

	// For simulation, we immediately set the pipe to its desired state.
	currentState := CurrentStateRunning
	if desiredState == DesiredStateStopped {
		currentState = CurrentStateStopped
	}

	pipe := &Pipe{
		Arn:                  generatePipeArn(req.Name),
		Name:                 req.Name,
		Source:               req.Source,
		Target:               req.Target,
		RoleArn:              req.RoleArn,
		Description:          req.Description,
		DesiredState:         desiredState,
		CurrentState:         currentState,
		Enrichment:           req.Enrichment,
		EnrichmentParameters: req.EnrichmentParameters,
		SourceParameters:     req.SourceParameters,
		TargetParameters:     req.TargetParameters,
		LogConfiguration:     req.LogConfiguration,
		KmsKeyIdentifier:     req.KmsKeyIdentifier,
		Tags:                 req.Tags,
		CreationTime:         AWSTimestamp{Time: now},
		LastModifiedTime:     AWSTimestamp{Time: now},
	}

	m.Pipes[req.Name] = pipe

	m.saveLocked()
	m.startPollerLocked(pipe)

	return pipe, nil
}

// DescribePipe retrieves a pipe by name.
func (m *MemoryStorage) DescribePipe(_ context.Context, name string) (*Pipe, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pipe, exists := m.Pipes[name]
	if !exists {
		return nil, &Error{
			Code:    errNotFoundException,
			Message: fmt.Sprintf("Pipe %s does not exist", name),
		}
	}

	return pipe, nil
}

// UpdatePipe updates an existing pipe.
//
//nolint:funlen // field updates require more lines
func (m *MemoryStorage) UpdatePipe(_ context.Context, req *UpdatePipeInput) (*Pipe, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pipe, exists := m.Pipes[req.Name]
	if !exists {
		return nil, &Error{
			Code:    errNotFoundException,
			Message: fmt.Sprintf("Pipe %s does not exist", req.Name),
		}
	}

	// Validate required field.
	if req.RoleArn == "" {
		return nil, &Error{
			Code:    errValidationException,
			Message: "RoleArn is required",
		}
	}

	// Update fields.
	pipe.RoleArn = req.RoleArn
	pipe.LastModifiedTime = AWSTimestamp{Time: time.Now()}

	if req.Description != "" {
		pipe.Description = req.Description
	}

	if req.DesiredState != "" {
		pipe.DesiredState = req.DesiredState

		// For simulation, immediately update current state.
		switch req.DesiredState {
		case DesiredStateRunning:
			pipe.CurrentState = CurrentStateRunning
		case DesiredStateStopped:
			pipe.CurrentState = CurrentStateStopped
		}
	}

	if req.Enrichment != "" {
		pipe.Enrichment = req.Enrichment
	}

	if req.EnrichmentParameters != nil {
		pipe.EnrichmentParameters = req.EnrichmentParameters
	}

	if req.SourceParameters != nil {
		pipe.SourceParameters = req.SourceParameters
	}

	if req.TargetParameters != nil {
		pipe.TargetParameters = req.TargetParameters
	}

	if req.LogConfiguration != nil {
		pipe.LogConfiguration = req.LogConfiguration
	}

	if req.KmsKeyIdentifier != "" {
		pipe.KmsKeyIdentifier = req.KmsKeyIdentifier
	}

	m.saveLocked()

	// Refresh the poller so it picks up any source/target/state changes.
	m.stopPollerLocked(pipe.Name)
	m.startPollerLocked(pipe)

	return pipe, nil
}

// DeletePipe deletes a pipe by name.
func (m *MemoryStorage) DeletePipe(_ context.Context, name string) (*Pipe, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pipe, exists := m.Pipes[name]
	if !exists {
		return nil, &Error{
			Code:    errNotFoundException,
			Message: fmt.Sprintf("Pipe %s does not exist", name),
		}
	}

	// Stop the poller before removing the pipe.
	m.stopPollerLocked(name)

	// Update state to deleting.
	pipe.CurrentState = CurrentStateDeleting
	pipe.DesiredState = DesiredStateStopped
	pipe.LastModifiedTime = AWSTimestamp{Time: time.Now()}

	// Create a copy for the response before deleting.
	result := &Pipe{
		Arn:              pipe.Arn,
		Name:             pipe.Name,
		DesiredState:     pipe.DesiredState,
		CurrentState:     pipe.CurrentState,
		CreationTime:     pipe.CreationTime,
		LastModifiedTime: pipe.LastModifiedTime,
	}

	delete(m.Pipes, name)

	m.saveLocked()

	return result, nil
}

// ListPipes lists pipes with optional filters.
func (m *MemoryStorage) ListPipes(_ context.Context, req *ListPipesInput) (*ListPipesOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := req.Limit
	if limit <= 0 || limit > maxPageLimit {
		limit = defaultPageLimit
	}

	var pipes []*PipeSummary

	for _, pipe := range m.Pipes {
		// Apply filters.
		if req.NamePrefix != "" && !strings.HasPrefix(pipe.Name, req.NamePrefix) {
			continue
		}

		if req.SourcePrefix != "" && !strings.HasPrefix(pipe.Source, req.SourcePrefix) {
			continue
		}

		if req.TargetPrefix != "" && !strings.HasPrefix(pipe.Target, req.TargetPrefix) {
			continue
		}

		if req.CurrentState != "" && pipe.CurrentState != req.CurrentState {
			continue
		}

		if req.DesiredState != "" && pipe.DesiredState != req.DesiredState {
			continue
		}

		summary := &PipeSummary{
			Arn:              pipe.Arn,
			Name:             pipe.Name,
			Source:           pipe.Source,
			Target:           pipe.Target,
			DesiredState:     pipe.DesiredState,
			CurrentState:     pipe.CurrentState,
			StateReason:      pipe.StateReason,
			Enrichment:       pipe.Enrichment,
			CreationTime:     pipe.CreationTime,
			LastModifiedTime: pipe.LastModifiedTime,
		}

		pipes = append(pipes, summary)

		if int32(len(pipes)) >= limit { //nolint:gosec // G115: len(pipes) is bounded by limit which is int32
			break
		}
	}

	return &ListPipesOutput{
		Pipes: pipes,
	}, nil
}

// StartPipe starts a stopped pipe.
func (m *MemoryStorage) StartPipe(_ context.Context, name string) (*Pipe, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pipe, exists := m.Pipes[name]
	if !exists {
		return nil, &Error{
			Code:    errNotFoundException,
			Message: fmt.Sprintf("Pipe %s does not exist", name),
		}
	}

	// Check if pipe can be started.
	if pipe.CurrentState != CurrentStateStopped {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("Pipe %s is not in STOPPED state", name),
		}
	}

	pipe.DesiredState = DesiredStateRunning
	pipe.CurrentState = CurrentStateRunning
	pipe.LastModifiedTime = AWSTimestamp{Time: time.Now()}

	m.saveLocked()
	m.startPollerLocked(pipe)

	return pipe, nil
}

// StopPipe stops a running pipe.
func (m *MemoryStorage) StopPipe(_ context.Context, name string) (*Pipe, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pipe, exists := m.Pipes[name]
	if !exists {
		return nil, &Error{
			Code:    errNotFoundException,
			Message: fmt.Sprintf("Pipe %s does not exist", name),
		}
	}

	// Check if pipe can be stopped.
	if pipe.CurrentState != CurrentStateRunning {
		return nil, &Error{
			Code:    errConflictException,
			Message: fmt.Sprintf("Pipe %s is not in RUNNING state", name),
		}
	}

	pipe.DesiredState = DesiredStateStopped
	pipe.CurrentState = CurrentStateStopped
	pipe.LastModifiedTime = AWSTimestamp{Time: time.Now()}

	m.saveLocked()
	m.stopPollerLocked(name)

	return pipe, nil
}

// TagResource adds tags to a pipe.
func (m *MemoryStorage) TagResource(_ context.Context, arn string, tags map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find pipe by ARN.
	var pipe *Pipe

	for _, p := range m.Pipes {
		if p.Arn == arn {
			pipe = p

			break
		}
	}

	if pipe == nil {
		return &Error{
			Code:    errNotFoundException,
			Message: fmt.Sprintf("Resource %s not found", arn),
		}
	}

	if pipe.Tags == nil {
		pipe.Tags = make(map[string]string)
	}

	maps.Copy(pipe.Tags, tags)

	m.saveLocked()

	return nil
}

// UntagResource removes tags from a pipe.
func (m *MemoryStorage) UntagResource(_ context.Context, arn string, tagKeys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find pipe by ARN.
	var pipe *Pipe

	for _, p := range m.Pipes {
		if p.Arn == arn {
			pipe = p

			break
		}
	}

	if pipe == nil {
		return &Error{
			Code:    errNotFoundException,
			Message: fmt.Sprintf("Resource %s not found", arn),
		}
	}

	if pipe.Tags == nil {
		return nil
	}

	for _, key := range tagKeys {
		delete(pipe.Tags, key)
	}

	m.saveLocked()

	return nil
}

// ListTagsForResource lists tags for a pipe.
func (m *MemoryStorage) ListTagsForResource(_ context.Context, arn string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find pipe by ARN.
	var pipe *Pipe

	for _, p := range m.Pipes {
		if p.Arn == arn {
			pipe = p

			break
		}
	}

	if pipe == nil {
		return nil, &Error{
			Code:    errNotFoundException,
			Message: fmt.Sprintf("Resource %s not found", arn),
		}
	}

	// Return a copy of the tags.
	tags := make(map[string]string)
	maps.Copy(tags, pipe.Tags)

	return tags, nil
}

// Error represents a Pipes service error.
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
	case errNotFoundException:
		return 404
	case errValidationException:
		return 400
	default:
		return 500
	}
}
