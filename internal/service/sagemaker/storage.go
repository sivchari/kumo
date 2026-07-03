package sagemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errResourceNotFound    = "ResourceNotFound"
	errResourceInUse       = "ResourceInUse"
	errValidationException = "ValidationException"
	errInternalFailure     = "InternalFailure"
)

// Default values.
const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "123456789012"
)

// Notebook instance statuses.
const statusInService = "InService"

// Training job statuses.
const trainingStatusCompleted = "Completed"

// Endpoint statuses.
const endpointStatusInService = "InService"

// Storage defines the SageMaker service storage interface.
type Storage interface {
	// Notebook instance operations
	CreateNotebookInstance(ctx context.Context, req *CreateNotebookInstanceRequest) (*NotebookInstance, error)
	DeleteNotebookInstance(ctx context.Context, name string) error
	DescribeNotebookInstance(ctx context.Context, name string) (*NotebookInstance, error)
	ListNotebookInstances(ctx context.Context, maxResults int32, nextToken string) ([]*NotebookInstance, string, error)

	// Training job operations
	CreateTrainingJob(ctx context.Context, req *CreateTrainingJobRequest) (*TrainingJob, error)
	DescribeTrainingJob(ctx context.Context, name string) (*TrainingJob, error)

	// Model operations
	CreateModel(ctx context.Context, req *CreateModelRequest) (*Model, error)
	DeleteModel(ctx context.Context, name string) error

	// Endpoint operations
	CreateEndpoint(ctx context.Context, req *CreateEndpointRequest) (*Endpoint, error)
	DeleteEndpoint(ctx context.Context, name string) error
	DescribeEndpoint(ctx context.Context, name string) (*Endpoint, error)
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
	mu                sync.RWMutex                 `json:"-"`
	NotebookInstances map[string]*NotebookInstance `json:"notebookInstances"`
	TrainingJobs      map[string]*TrainingJob      `json:"trainingJobs"`
	Models            map[string]*Model            `json:"models"`
	Endpoints         map[string]*Endpoint         `json:"endpoints"`
	region            string
	accountID         string
	dataDir           string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		NotebookInstances: make(map[string]*NotebookInstance),
		TrainingJobs:      make(map[string]*TrainingJob),
		Models:            make(map[string]*Model),
		Endpoints:         make(map[string]*Endpoint),
		region:            region,
		accountID:         defaultAccountID,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "sagemaker", s)
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

	if m.NotebookInstances == nil {
		m.NotebookInstances = make(map[string]*NotebookInstance)
	}

	if m.TrainingJobs == nil {
		m.TrainingJobs = make(map[string]*TrainingJob)
	}

	if m.Models == nil {
		m.Models = make(map[string]*Model)
	}

	if m.Endpoints == nil {
		m.Endpoints = make(map[string]*Endpoint)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "sagemaker", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "sagemaker", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateNotebookInstance creates a new notebook instance.
func (m *MemoryStorage) CreateNotebookInstance(_ context.Context, req *CreateNotebookInstanceRequest) (*NotebookInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.NotebookInstances[req.NotebookInstanceName]; exists {
		return nil, &Error{
			Code:    errResourceInUse,
			Message: fmt.Sprintf("Notebook instance %s already exists", req.NotebookInstanceName),
		}
	}

	now := time.Now()
	arn := fmt.Sprintf("arn:aws:sagemaker:%s:%s:notebook-instance/%s", m.region, m.accountID, req.NotebookInstanceName)

	instance := &NotebookInstance{
		NotebookInstanceName:   req.NotebookInstanceName,
		NotebookInstanceArn:    arn,
		NotebookInstanceStatus: statusInService,
		InstanceType:           req.InstanceType,
		RoleArn:                req.RoleArn,
		KmsKeyID:               req.KmsKeyID,
		SubnetID:               req.SubnetID,
		SecurityGroups:         req.SecurityGroupIDs,
		DirectInternetAccess:   req.DirectInternetAccess,
		VolumeSizeInGB:         req.VolumeSizeInGB,
		AcceleratorTypes:       req.AcceleratorTypes,
		DefaultCodeRepository:  req.DefaultCodeRepository,
		AdditionalCodeRepos:    req.AdditionalCodeRepos,
		RootAccess:             req.RootAccess,
		PlatformIdentifier:     req.PlatformIdentifier,
		InstanceMetadataConfig: req.InstanceMetadataConfig,
		CreationTime:           now,
		LastModifiedTime:       now,
	}

	// Set default volume size if not specified.
	if instance.VolumeSizeInGB == 0 {
		instance.VolumeSizeInGB = 5
	}

	// Set default direct internet access.
	if instance.DirectInternetAccess == "" {
		instance.DirectInternetAccess = "Enabled"
	}

	// Set default root access.
	if instance.RootAccess == "" {
		instance.RootAccess = "Enabled"
	}

	m.NotebookInstances[req.NotebookInstanceName] = instance

	m.saveLocked()

	return instance, nil
}

// DeleteNotebookInstance deletes a notebook instance.
func (m *MemoryStorage) DeleteNotebookInstance(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.NotebookInstances[name]; !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Notebook instance %s not found", name),
		}
	}

	delete(m.NotebookInstances, name)

	m.saveLocked()

	return nil
}

// DescribeNotebookInstance describes a notebook instance.
func (m *MemoryStorage) DescribeNotebookInstance(_ context.Context, name string) (*NotebookInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.NotebookInstances[name]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Notebook instance %s not found", name),
		}
	}

	return instance, nil
}

// ListNotebookInstances lists notebook instances.
func (m *MemoryStorage) ListNotebookInstances(_ context.Context, maxResults int32, _ string) ([]*NotebookInstance, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 100
	}

	result := make([]*NotebookInstance, 0, len(m.NotebookInstances))

	for _, instance := range m.NotebookInstances {
		result = append(result, instance)

		if len(result) >= int(maxResults) {
			break
		}
	}

	return result, "", nil
}

// CreateTrainingJob creates a new training job.
func (m *MemoryStorage) CreateTrainingJob(_ context.Context, req *CreateTrainingJobRequest) (*TrainingJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.TrainingJobs[req.TrainingJobName]; exists {
		return nil, &Error{
			Code:    errResourceInUse,
			Message: fmt.Sprintf("Training job %s already exists", req.TrainingJobName),
		}
	}

	now := time.Now()
	arn := fmt.Sprintf("arn:aws:sagemaker:%s:%s:training-job/%s", m.region, m.accountID, req.TrainingJobName)

	job := &TrainingJob{
		TrainingJobName:   req.TrainingJobName,
		TrainingJobArn:    arn,
		TrainingJobStatus: trainingStatusCompleted,
		SecondaryStatus:   "Completed",
		AlgorithmSpec:     req.AlgorithmSpec,
		RoleArn:           req.RoleArn,
		InputDataConfig:   req.InputDataConfig,
		OutputDataConfig:  req.OutputDataConfig,
		ResourceConfig:    req.ResourceConfig,
		StoppingCondition: req.StoppingCondition,
		CreationTime:      now,
		TrainingStartTime: &now,
		TrainingEndTime:   &now,
	}

	m.TrainingJobs[req.TrainingJobName] = job

	m.saveLocked()

	return job, nil
}

// DescribeTrainingJob describes a training job.
func (m *MemoryStorage) DescribeTrainingJob(_ context.Context, name string) (*TrainingJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.TrainingJobs[name]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Training job %s not found", name),
		}
	}

	return job, nil
}

// CreateModel creates a new model.
func (m *MemoryStorage) CreateModel(_ context.Context, req *CreateModelRequest) (*Model, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Models[req.ModelName]; exists {
		return nil, &Error{
			Code:    errResourceInUse,
			Message: fmt.Sprintf("Model %s already exists", req.ModelName),
		}
	}

	now := time.Now()
	arn := fmt.Sprintf("arn:aws:sagemaker:%s:%s:model/%s", m.region, m.accountID, req.ModelName)

	model := &Model{
		ModelName:              req.ModelName,
		ModelArn:               arn,
		PrimaryContainer:       req.PrimaryContainer,
		ExecutionRoleArn:       req.ExecutionRoleArn,
		EnableNetworkIsolation: req.EnableNetworkIsolation,
		CreationTime:           now,
	}

	m.Models[req.ModelName] = model

	m.saveLocked()

	return model, nil
}

// DeleteModel deletes a model.
func (m *MemoryStorage) DeleteModel(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Models[name]; !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Model %s not found", name),
		}
	}

	delete(m.Models, name)

	m.saveLocked()

	return nil
}

// CreateEndpoint creates a new endpoint.
func (m *MemoryStorage) CreateEndpoint(_ context.Context, req *CreateEndpointRequest) (*Endpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Endpoints[req.EndpointName]; exists {
		return nil, &Error{
			Code:    errResourceInUse,
			Message: fmt.Sprintf("Endpoint %s already exists", req.EndpointName),
		}
	}

	now := time.Now()
	arn := fmt.Sprintf("arn:aws:sagemaker:%s:%s:endpoint/%s", m.region, m.accountID, req.EndpointName)

	endpoint := &Endpoint{
		EndpointName:       req.EndpointName,
		EndpointArn:        arn,
		EndpointConfigName: req.EndpointConfigName,
		EndpointStatus:     endpointStatusInService,
		CreationTime:       now,
		LastModifiedTime:   now,
	}

	m.Endpoints[req.EndpointName] = endpoint

	m.saveLocked()

	return endpoint, nil
}

// DeleteEndpoint deletes an endpoint.
func (m *MemoryStorage) DeleteEndpoint(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Endpoints[name]; !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Endpoint %s not found", name),
		}
	}

	delete(m.Endpoints, name)

	m.saveLocked()

	return nil
}

// DescribeEndpoint describes an endpoint.
func (m *MemoryStorage) DescribeEndpoint(_ context.Context, name string) (*Endpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	endpoint, exists := m.Endpoints[name]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Endpoint %s not found", name),
		}
	}

	return endpoint, nil
}

// Ensure MemoryStorage implements Storage.
var _ Storage = (*MemoryStorage)(nil)
