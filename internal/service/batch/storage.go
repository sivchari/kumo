package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errNotFound       = "ClientException"
	errInvalidRequest = "ClientException"
	errConflict       = "ClientException"
)

// Storage defines the interface for Batch storage operations.
type Storage interface {
	CreateComputeEnvironment(ctx context.Context, input *CreateComputeEnvironmentInput) (*ComputeEnvironment, error)
	DeleteComputeEnvironment(ctx context.Context, name string) error
	DescribeComputeEnvironments(ctx context.Context, names []string) ([]ComputeEnvironment, error)
	CreateJobQueue(ctx context.Context, input *CreateJobQueueInput) (*JobQueue, error)
	DeleteJobQueue(ctx context.Context, name string) error
	DescribeJobQueues(ctx context.Context, names []string) ([]JobQueue, error)
	RegisterJobDefinition(ctx context.Context, input *RegisterJobDefinitionInput) (*JobDefinition, error)
	SubmitJob(ctx context.Context, input *SubmitJobInput) (*Job, error)
	DescribeJobs(ctx context.Context, jobIDs []string) ([]Job, error)
	TerminateJob(ctx context.Context, jobID, reason string) error
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
	mu                  sync.RWMutex                   `json:"-"`
	ComputeEnvironments map[string]*ComputeEnvironment `json:"computeEnvironments"` // key: name
	JobQueues           map[string]*JobQueue           `json:"jobQueues"`           // key: name
	JobDefinitions      map[string]*JobDefinition      `json:"jobDefinitions"`      // key: name:revision
	Jobs                map[string]*Job                `json:"jobs"`                // key: jobID
	JobDefRevisions     map[string]int32               `json:"jobDefRevisions"`     // key: name -> latest revision
	dataDir             string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		ComputeEnvironments: make(map[string]*ComputeEnvironment),
		JobQueues:           make(map[string]*JobQueue),
		JobDefinitions:      make(map[string]*JobDefinition),
		Jobs:                make(map[string]*Job),
		JobDefRevisions:     make(map[string]int32),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "batch", s)
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

	if s.ComputeEnvironments == nil {
		s.ComputeEnvironments = make(map[string]*ComputeEnvironment)
	}

	if s.JobQueues == nil {
		s.JobQueues = make(map[string]*JobQueue)
	}

	if s.JobDefinitions == nil {
		s.JobDefinitions = make(map[string]*JobDefinition)
	}

	if s.Jobs == nil {
		s.Jobs = make(map[string]*Job)
	}

	if s.JobDefRevisions == nil {
		s.JobDefRevisions = make(map[string]int32)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "batch", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "batch", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateComputeEnvironment creates a new compute environment.
func (s *MemoryStorage) CreateComputeEnvironment(_ context.Context, input *CreateComputeEnvironmentInput) (*ComputeEnvironment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.ComputeEnvironmentName == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "computeEnvironmentName is required",
		}
	}

	if _, exists := s.ComputeEnvironments[input.ComputeEnvironmentName]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("Compute environment %s already exists", input.ComputeEnvironmentName),
		}
	}

	ceARN := fmt.Sprintf("arn:aws:batch:us-east-1:000000000000:compute-environment/%s", input.ComputeEnvironmentName)

	state := input.State
	if state == "" {
		state = CEStateEnabled
	}

	ce := &ComputeEnvironment{
		ComputeEnvironmentARN:  ceARN,
		ComputeEnvironmentName: input.ComputeEnvironmentName,
		ComputeResources:       input.ComputeResources,
		EksConfiguration:       input.EksConfiguration,
		ServiceRole:            input.ServiceRole,
		State:                  state,
		Status:                 CEStatusValid,
		Type:                   input.Type,
		Tags:                   input.Tags,
		UUID:                   uuid.New().String(),
	}

	s.ComputeEnvironments[input.ComputeEnvironmentName] = ce

	s.saveLocked()

	return ce, nil
}

// DeleteComputeEnvironment deletes a compute environment.
func (s *MemoryStorage) DeleteComputeEnvironment(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.ComputeEnvironments[name]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Compute environment %s not found", name),
		}
	}

	delete(s.ComputeEnvironments, name)

	s.saveLocked()

	return nil
}

// DescribeComputeEnvironments describes compute environments.
func (s *MemoryStorage) DescribeComputeEnvironments(_ context.Context, names []string) ([]ComputeEnvironment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []ComputeEnvironment

	if len(names) == 0 {
		// Return all compute environments.
		for _, ce := range s.ComputeEnvironments {
			result = append(result, *ce)
		}
	} else {
		// Return specified compute environments.
		for _, name := range names {
			if ce, exists := s.ComputeEnvironments[name]; exists {
				result = append(result, *ce)
			}
		}
	}

	return result, nil
}

// CreateJobQueue creates a new job queue.
func (s *MemoryStorage) CreateJobQueue(_ context.Context, input *CreateJobQueueInput) (*JobQueue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.JobQueueName == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "jobQueueName is required",
		}
	}

	if _, exists := s.JobQueues[input.JobQueueName]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("Job queue %s already exists", input.JobQueueName),
		}
	}

	jqARN := fmt.Sprintf("arn:aws:batch:us-east-1:000000000000:job-queue/%s", input.JobQueueName)

	state := input.State
	if state == "" {
		state = JQStateEnabled
	}

	jq := &JobQueue{
		ComputeEnvironmentOrder:  input.ComputeEnvironmentOrder,
		JobQueueARN:              jqARN,
		JobQueueName:             input.JobQueueName,
		JobStateTimeLimitActions: input.JobStateTimeLimitActions,
		Priority:                 input.Priority,
		SchedulingPolicyARN:      input.SchedulingPolicyARN,
		State:                    state,
		Status:                   JQStatusValid,
		Tags:                     input.Tags,
	}

	s.JobQueues[input.JobQueueName] = jq

	s.saveLocked()

	return jq, nil
}

// DeleteJobQueue deletes a job queue.
func (s *MemoryStorage) DeleteJobQueue(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.JobQueues[name]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Job queue %s not found", name),
		}
	}

	delete(s.JobQueues, name)

	s.saveLocked()

	return nil
}

// DescribeJobQueues describes job queues.
func (s *MemoryStorage) DescribeJobQueues(_ context.Context, names []string) ([]JobQueue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []JobQueue

	if len(names) == 0 {
		// Return all job queues.
		for _, jq := range s.JobQueues {
			result = append(result, *jq)
		}
	} else {
		// Return specified job queues.
		for _, name := range names {
			if jq, exists := s.JobQueues[name]; exists {
				result = append(result, *jq)
			}
		}
	}

	return result, nil
}

// RegisterJobDefinition registers a new job definition.
func (s *MemoryStorage) RegisterJobDefinition(_ context.Context, input *RegisterJobDefinitionInput) (*JobDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.JobDefinitionName == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "jobDefinitionName is required",
		}
	}

	if input.Type == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "type is required",
		}
	}

	// Increment revision.
	revision := s.JobDefRevisions[input.JobDefinitionName] + 1
	s.JobDefRevisions[input.JobDefinitionName] = revision

	jdARN := fmt.Sprintf("arn:aws:batch:us-east-1:000000000000:job-definition/%s:%d", input.JobDefinitionName, revision)
	jdKey := fmt.Sprintf("%s:%d", input.JobDefinitionName, revision)

	jd := &JobDefinition{
		ContainerProperties:  input.ContainerProperties,
		EksProperties:        input.EksProperties,
		JobDefinitionARN:     jdARN,
		JobDefinitionName:    input.JobDefinitionName,
		NodeProperties:       input.NodeProperties,
		Parameters:           input.Parameters,
		PlatformCapabilities: input.PlatformCapabilities,
		PropagateTags:        input.PropagateTags,
		RetryStrategy:        input.RetryStrategy,
		Revision:             revision,
		SchedulingPriority:   input.SchedulingPriority,
		Status:               "ACTIVE",
		Tags:                 input.Tags,
		Timeout:              input.Timeout,
		Type:                 input.Type,
	}

	s.JobDefinitions[jdKey] = jd

	s.saveLocked()

	return jd, nil
}

// SubmitJob submits a new job.
func (s *MemoryStorage) SubmitJob(_ context.Context, input *SubmitJobInput) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.JobName == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "jobName is required",
		}
	}

	if input.JobDefinition == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "jobDefinition is required",
		}
	}

	if input.JobQueue == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "jobQueue is required",
		}
	}

	jobID := uuid.New().String()
	jobARN := fmt.Sprintf("arn:aws:batch:us-east-1:000000000000:job/%s", jobID)

	job := &Job{
		CreatedAt:          nowMillis(),
		DependsOn:          input.DependsOn,
		JobARN:             jobARN,
		JobDefinition:      input.JobDefinition,
		JobID:              jobID,
		JobName:            input.JobName,
		JobQueue:           input.JobQueue,
		Parameters:         input.Parameters,
		PropagateTags:      input.PropagateTags,
		RetryStrategy:      input.RetryStrategy,
		SchedulingPriority: input.SchedulingPriorityOverride,
		ShareIdentifier:    input.ShareIdentifier,
		Status:             JobStatusSubmitted,
		Tags:               input.Tags,
		Timeout:            input.Timeout,
	}

	s.Jobs[jobID] = job

	s.saveLocked()

	return job, nil
}

// DescribeJobs describes jobs.
func (s *MemoryStorage) DescribeJobs(_ context.Context, jobIDs []string) ([]Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Job

	for _, id := range jobIDs {
		if job, exists := s.Jobs[id]; exists {
			result = append(result, *job)
		}
	}

	return result, nil
}

// TerminateJob terminates a job.
func (s *MemoryStorage) TerminateJob(_ context.Context, jobID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.Jobs[jobID]
	if !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Job %s not found", jobID),
		}
	}

	job.Status = JobStatusFailed
	job.StatusReason = reason
	job.IsTerminated = true
	job.StoppedAt = nowMillis()

	s.saveLocked()

	return nil
}
