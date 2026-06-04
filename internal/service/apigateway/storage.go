package apigateway

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
	errRestAPINotFound    = "NotFoundException"
	errResourceNotFound   = "NotFoundException"
	errMethodNotFound     = "NotFoundException"
	errDeploymentNotFound = "NotFoundException"
	errStageNotFound      = "NotFoundException"
	errBadRequest         = "BadRequestException"
)

// Storage defines the API Gateway storage interface.
type Storage interface {
	CreateRestAPI(ctx context.Context, req *CreateRestAPIRequest) (*RestAPI, error)
	GetRestAPI(ctx context.Context, restAPIID string) (*RestAPI, error)
	GetRestAPIs(ctx context.Context, limit int32, position string) ([]*RestAPI, string, error)
	DeleteRestAPI(ctx context.Context, restAPIID string) error

	CreateResource(ctx context.Context, restAPIID, parentID, pathPart string) (*Resource, error)
	GetResource(ctx context.Context, restAPIID, resourceID string) (*Resource, error)
	GetResources(ctx context.Context, restAPIID string, limit int32, position string) ([]*Resource, string, error)
	DeleteResource(ctx context.Context, restAPIID, resourceID string) error

	PutMethod(ctx context.Context, restAPIID, resourceID, httpMethod string, req *PutMethodRequest) (*Method, error)
	DeleteMethod(ctx context.Context, restAPIID, resourceID, httpMethod string) error
	GetMethod(ctx context.Context, restAPIID, resourceID, httpMethod string) (*Method, error)

	PutIntegration(ctx context.Context, restAPIID, resourceID, httpMethod string, req *PutIntegrationRequest) (*Integration, error)
	GetIntegration(ctx context.Context, restAPIID, resourceID, httpMethod string) (*Integration, error)

	CreateDeployment(ctx context.Context, restAPIID string, req *CreateDeploymentRequest) (*Deployment, error)
	GetDeployment(ctx context.Context, restAPIID, deploymentID string) (*Deployment, error)
	GetDeployments(ctx context.Context, restAPIID string, limit int32, position string) ([]*Deployment, string, error)
	DeleteDeployment(ctx context.Context, restAPIID, deploymentID string) error

	CreateStage(ctx context.Context, restAPIID string, req *CreateStageRequest) (*Stage, error)
	GetStage(ctx context.Context, restAPIID, stageName string) (*Stage, error)
	GetStages(ctx context.Context, restAPIID string) ([]*Stage, error)
	DeleteStage(ctx context.Context, restAPIID, stageName string) error
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
	mu       sync.RWMutex            `json:"-"`
	RestAPIs map[string]*RestAPIData `json:"restApis"`
	dataDir  string
}

// RestAPIData holds REST API information and its resources.
type RestAPIData struct {
	API         *RestAPI               `json:"api"`
	Resources   map[string]*Resource   `json:"resources"` // keyed by resource ID
	Deployments map[string]*Deployment `json:"deployments"`
	Stages      map[string]*Stage      `json:"stages"` // keyed by stage name
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		RestAPIs: make(map[string]*RestAPIData),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "apigateway", s)
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

	if s.RestAPIs == nil {
		s.RestAPIs = make(map[string]*RestAPIData)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "apigateway", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "apigateway", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateRestAPI creates a new REST API.
func (s *MemoryStorage) CreateRestAPI(_ context.Context, req *CreateRestAPIRequest) (*RestAPI, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateID()
	rootResourceID := generateID()
	now := time.Now()

	api := &RestAPI{
		ID:                     id,
		Name:                   req.Name,
		Description:            req.Description,
		CreatedDate:            now,
		Version:                req.Version,
		APIKeySource:           req.APIKeySource,
		EndpointConfiguration:  req.EndpointConfiguration,
		DisableExecuteAPIEndpt: req.DisableExecuteAPIEndpt,
		Tags:                   req.Tags,
		RootResourceID:         rootResourceID,
	}

	// Create root resource.
	rootResource := &Resource{
		ID:              rootResourceID,
		Path:            "/",
		ResourceMethods: make(map[string]Method),
	}

	s.RestAPIs[id] = &RestAPIData{
		API:         api,
		Resources:   map[string]*Resource{rootResourceID: rootResource},
		Deployments: make(map[string]*Deployment),
		Stages:      make(map[string]*Stage),
	}

	s.saveLocked()

	return api, nil
}

// GetRestAPI returns a REST API by ID.
func (s *MemoryStorage) GetRestAPI(_ context.Context, restAPIID string) (*RestAPI, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	return data.API, nil
}

// GetRestAPIs returns all REST APIs.
func (s *MemoryStorage) GetRestAPIs(_ context.Context, limit int32, _ string) ([]*RestAPI, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 25
	}

	var apis []*RestAPI

	for _, data := range s.RestAPIs {
		apis = append(apis, data.API)

		if int32(len(apis)) >= limit { //nolint:gosec // slice length bounded by limit parameter
			break
		}
	}

	return apis, "", nil
}

// DeleteRestAPI deletes a REST API.
func (s *MemoryStorage) DeleteRestAPI(_ context.Context, restAPIID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.RestAPIs[restAPIID]; !exists {
		return &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	delete(s.RestAPIs, restAPIID)

	s.saveLocked()

	return nil
}

// CreateResource creates a new resource.
func (s *MemoryStorage) CreateResource(_ context.Context, restAPIID, parentID, pathPart string) (*Resource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	parent, exists := data.Resources[parentID]
	if !exists {
		return nil, &ServiceError{Code: errResourceNotFound, Message: "Invalid resource identifier specified"}
	}

	id := generateID()
	path := buildPath(parent.Path, pathPart)

	resource := &Resource{
		ID:              id,
		ParentID:        parentID,
		PathPart:        pathPart,
		Path:            path,
		ResourceMethods: make(map[string]Method),
	}

	data.Resources[id] = resource

	s.saveLocked()

	return resource, nil
}

// GetResource returns a resource by ID.
func (s *MemoryStorage) GetResource(_ context.Context, restAPIID, resourceID string) (*Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	resource, exists := data.Resources[resourceID]
	if !exists {
		return nil, &ServiceError{Code: errResourceNotFound, Message: "Invalid resource identifier specified"}
	}

	return resource, nil
}

// GetResources returns all resources for a REST API.
func (s *MemoryStorage) GetResources(_ context.Context, restAPIID string, limit int32, _ string) ([]*Resource, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, "", &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	if limit <= 0 {
		limit = 25
	}

	var resources []*Resource

	for _, r := range data.Resources {
		resources = append(resources, r)

		if int32(len(resources)) >= limit { //nolint:gosec // slice length bounded by limit parameter
			break
		}
	}

	return resources, "", nil
}

// DeleteResource deletes a resource.
func (s *MemoryStorage) DeleteResource(_ context.Context, restAPIID, resourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	resource, exists := data.Resources[resourceID]
	if !exists {
		return &ServiceError{Code: errResourceNotFound, Message: "Invalid resource identifier specified"}
	}

	// Cannot delete root resource.
	if resource.Path == "/" {
		return &ServiceError{Code: errBadRequest, Message: "Cannot delete root resource"}
	}

	delete(data.Resources, resourceID)

	s.saveLocked()

	return nil
}

// PutMethod creates or updates a method.
func (s *MemoryStorage) PutMethod(_ context.Context, restAPIID, resourceID, httpMethod string, req *PutMethodRequest) (*Method, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	resource, exists := data.Resources[resourceID]
	if !exists {
		return nil, &ServiceError{Code: errResourceNotFound, Message: "Invalid resource identifier specified"}
	}

	method := Method{
		HTTPMethod:        httpMethod,
		AuthorizationType: req.AuthorizationType,
		APIKeyRequired:    req.APIKeyRequired,
		OperationName:     req.OperationName,
	}

	resource.ResourceMethods[httpMethod] = method

	s.saveLocked()

	return &method, nil
}

// GetMethod returns a method.
func (s *MemoryStorage) GetMethod(_ context.Context, restAPIID, resourceID, httpMethod string) (*Method, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	resource, exists := data.Resources[resourceID]
	if !exists {
		return nil, &ServiceError{Code: errResourceNotFound, Message: "Invalid resource identifier specified"}
	}

	method, exists := resource.ResourceMethods[httpMethod]
	if !exists {
		return nil, &ServiceError{Code: errMethodNotFound, Message: "Invalid method identifier specified"}
	}

	return &method, nil
}

// DeleteMethod removes a method from a resource.
func (s *MemoryStorage) DeleteMethod(_ context.Context, restAPIID, resourceID, httpMethod string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	resource, exists := data.Resources[resourceID]
	if !exists {
		return &ServiceError{Code: errResourceNotFound, Message: "Invalid resource identifier specified"}
	}

	if _, exists := resource.ResourceMethods[httpMethod]; !exists {
		return &ServiceError{Code: errMethodNotFound, Message: "Invalid method identifier specified"}
	}

	delete(resource.ResourceMethods, httpMethod)

	s.saveLocked()

	return nil
}

// PutIntegration creates or updates an integration.
func (s *MemoryStorage) PutIntegration(_ context.Context, restAPIID, resourceID, httpMethod string, req *PutIntegrationRequest) (*Integration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	resource, exists := data.Resources[resourceID]
	if !exists {
		return nil, &ServiceError{Code: errResourceNotFound, Message: "Invalid resource identifier specified"}
	}

	method, exists := resource.ResourceMethods[httpMethod]
	if !exists {
		return nil, &ServiceError{Code: errMethodNotFound, Message: "Invalid method identifier specified"}
	}

	integration := &Integration{
		Type:                req.Type,
		HTTPMethod:          req.HTTPMethod,
		URI:                 req.URI,
		ConnectionType:      req.ConnectionType,
		ConnectionID:        req.ConnectionID,
		PassthroughBehavior: req.PassthroughBehavior,
		ContentHandling:     req.ContentHandling,
		TimeoutInMillis:     req.TimeoutInMillis,
		CacheNamespace:      req.CacheNamespace,
		CacheKeyParameters:  req.CacheKeyParameters,
		RequestParameters:   req.RequestParameters,
		RequestTemplates:    req.RequestTemplates,
	}

	method.MethodIntegration = integration
	resource.ResourceMethods[httpMethod] = method

	s.saveLocked()

	return integration, nil
}

// GetIntegration returns an integration.
func (s *MemoryStorage) GetIntegration(_ context.Context, restAPIID, resourceID, httpMethod string) (*Integration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	resource, exists := data.Resources[resourceID]
	if !exists {
		return nil, &ServiceError{Code: errResourceNotFound, Message: "Invalid resource identifier specified"}
	}

	method, exists := resource.ResourceMethods[httpMethod]
	if !exists {
		return nil, &ServiceError{Code: errMethodNotFound, Message: "Invalid method identifier specified"}
	}

	if method.MethodIntegration == nil {
		return nil, &ServiceError{Code: errMethodNotFound, Message: "Integration not found"}
	}

	return method.MethodIntegration, nil
}

// CreateDeployment creates a new deployment.
func (s *MemoryStorage) CreateDeployment(_ context.Context, restAPIID string, req *CreateDeploymentRequest) (*Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	id := generateID()
	now := time.Now()

	deployment := &Deployment{
		ID:          id,
		Description: req.Description,
		CreatedDate: now,
	}

	data.Deployments[id] = deployment

	// If stage name is specified, create or update the stage.
	if req.StageName != "" {
		stage := &Stage{
			StageName:       req.StageName,
			DeploymentID:    id,
			CreatedDate:     now,
			LastUpdatedDate: now,
		}
		data.Stages[req.StageName] = stage
	}

	s.saveLocked()

	return deployment, nil
}

// GetDeployment returns a deployment.
func (s *MemoryStorage) GetDeployment(_ context.Context, restAPIID, deploymentID string) (*Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	deployment, exists := data.Deployments[deploymentID]
	if !exists {
		return nil, &ServiceError{Code: errDeploymentNotFound, Message: "Invalid deployment identifier specified"}
	}

	return deployment, nil
}

// GetDeployments returns all deployments.
func (s *MemoryStorage) GetDeployments(_ context.Context, restAPIID string, limit int32, _ string) ([]*Deployment, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, "", &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	if limit <= 0 {
		limit = 25
	}

	var deployments []*Deployment

	for _, d := range data.Deployments {
		deployments = append(deployments, d)

		if int32(len(deployments)) >= limit { //nolint:gosec // slice length bounded by limit parameter
			break
		}
	}

	return deployments, "", nil
}

// DeleteDeployment deletes a deployment.
func (s *MemoryStorage) DeleteDeployment(_ context.Context, restAPIID, deploymentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	if _, exists := data.Deployments[deploymentID]; !exists {
		return &ServiceError{Code: errDeploymentNotFound, Message: "Invalid deployment identifier specified"}
	}

	delete(data.Deployments, deploymentID)

	s.saveLocked()

	return nil
}

// CreateStage creates a new stage.
func (s *MemoryStorage) CreateStage(_ context.Context, restAPIID string, req *CreateStageRequest) (*Stage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	// Verify deployment exists.
	if _, exists := data.Deployments[req.DeploymentID]; !exists {
		return nil, &ServiceError{Code: errDeploymentNotFound, Message: "Invalid deployment identifier specified"}
	}

	now := time.Now()

	stage := &Stage{
		StageName:           req.StageName,
		DeploymentID:        req.DeploymentID,
		Description:         req.Description,
		CacheClusterEnabled: req.CacheClusterEnabled,
		CacheClusterSize:    req.CacheClusterSize,
		CreatedDate:         now,
		LastUpdatedDate:     now,
		Tags:                req.Tags,
	}

	data.Stages[req.StageName] = stage

	s.saveLocked()

	return stage, nil
}

// GetStage returns a stage.
func (s *MemoryStorage) GetStage(_ context.Context, restAPIID, stageName string) (*Stage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	stage, exists := data.Stages[stageName]
	if !exists {
		return nil, &ServiceError{Code: errStageNotFound, Message: "Invalid stage identifier specified"}
	}

	return stage, nil
}

// GetStages returns all stages.
func (s *MemoryStorage) GetStages(_ context.Context, restAPIID string) ([]*Stage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return nil, &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	var stages []*Stage

	for _, stage := range data.Stages {
		stages = append(stages, stage)
	}

	return stages, nil
}

// DeleteStage deletes a stage.
func (s *MemoryStorage) DeleteStage(_ context.Context, restAPIID, stageName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.RestAPIs[restAPIID]
	if !exists {
		return &ServiceError{Code: errRestAPINotFound, Message: "Invalid REST API identifier specified"}
	}

	if _, exists := data.Stages[stageName]; !exists {
		return &ServiceError{Code: errStageNotFound, Message: "Invalid stage identifier specified"}
	}

	delete(data.Stages, stageName)

	s.saveLocked()

	return nil
}

// generateID generates a unique ID.
func generateID() string {
	return uuid.New().String()[:10]
}

// buildPath builds a full path from parent path and path part.
func buildPath(parentPath, pathPart string) string {
	if parentPath == "/" {
		return "/" + pathPart
	}

	return fmt.Sprintf("%s/%s", parentPath, pathPart)
}
