package apigatewayv2

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
	errNotFound   = "NotFoundException"
	errBadRequest = "BadRequestException"
)

// Storage defines the API Gateway v2 storage interface.
type Storage interface {
	CreateAPI(ctx context.Context, req *CreateAPIRequest) (*API, error)
	GetAPI(ctx context.Context, apiID string) (*API, error)
	GetAPIs(ctx context.Context) ([]*API, error)
	UpdateAPI(ctx context.Context, apiID string, req *UpdateAPIRequest) (*API, error)
	DeleteAPI(ctx context.Context, apiID string) error

	CreateRoute(ctx context.Context, apiID string, req *CreateRouteRequest) (*Route, error)
	GetRoute(ctx context.Context, apiID, routeID string) (*Route, error)
	GetRoutes(ctx context.Context, apiID string) ([]*Route, error)
	UpdateRoute(ctx context.Context, apiID, routeID string, req *CreateRouteRequest) (*Route, error)
	DeleteRoute(ctx context.Context, apiID, routeID string) error

	CreateIntegration(ctx context.Context, apiID string, req *CreateIntegrationRequest) (*Integration, error)
	GetIntegration(ctx context.Context, apiID, integrationID string) (*Integration, error)
	GetIntegrations(ctx context.Context, apiID string) ([]*Integration, error)
	UpdateIntegration(ctx context.Context, apiID, integrationID string, req *CreateIntegrationRequest) (*Integration, error)
	DeleteIntegration(ctx context.Context, apiID, integrationID string) error

	CreateStage(ctx context.Context, apiID string, req *CreateStageRequest) (*Stage, error)
	GetStage(ctx context.Context, apiID, stageName string) (*Stage, error)
	GetStages(ctx context.Context, apiID string) ([]*Stage, error)
	UpdateStage(ctx context.Context, apiID, stageName string, req *CreateStageRequest) (*Stage, error)
	DeleteStage(ctx context.Context, apiID, stageName string) error

	CreateDeployment(ctx context.Context, apiID string, req *CreateDeploymentRequest) (*Deployment, error)
	GetDeployment(ctx context.Context, apiID, deploymentID string) (*Deployment, error)
	GetDeployments(ctx context.Context, apiID string) ([]*Deployment, error)
	DeleteDeployment(ctx context.Context, apiID, deploymentID string) error

	GetTags(ctx context.Context, arn string) (map[string]string, error)
	TagResource(ctx context.Context, arn string, tags map[string]string) error
	UntagResource(ctx context.Context, arn string, keys []string) error
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// WithEndpoint overrides the execute-api endpoint host used to build apiEndpoint.
func WithEndpoint(host string) Option {
	return func(s *MemoryStorage) {
		s.endpointHost = host
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu           sync.RWMutex        `json:"-"`
	APIs         map[string]*APIData `json:"apis"`
	dataDir      string
	endpointHost string
}

// APIData holds an API and its child resources.
type APIData struct {
	API          *API                    `json:"api"`
	Routes       map[string]*Route       `json:"routes"`
	Integrations map[string]*Integration `json:"integrations"`
	Stages       map[string]*Stage       `json:"stages"`
	Deployments  map[string]*Deployment  `json:"deployments"`
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		APIs:         make(map[string]*APIData),
		endpointHost: defaultEndpointHost,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "apigatewayv2", s)
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

	if s.APIs == nil {
		s.APIs = make(map[string]*APIData)
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

	_ = storage.SaveBytes(s.dataDir, "apigatewayv2", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "apigatewayv2", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateAPI creates a new API.
func (s *MemoryStorage) CreateAPI(_ context.Context, req *CreateAPIRequest) (*API, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateID()

	routeSelectionExpression := req.RouteSelectionExpression
	if routeSelectionExpression == "" {
		routeSelectionExpression = defaultRouteSelectionExpression
	}

	api := &API{
		APIID:                     id,
		Name:                      req.Name,
		ProtocolType:              req.ProtocolType,
		Description:               req.Description,
		Version:                   req.Version,
		APIKeySelectionExpression: req.APIKeySelectionExpression,
		RouteSelectionExpression:  routeSelectionExpression,
		DisableExecuteAPIEndpoint: req.DisableExecuteAPIEndpoint,
		DisableSchemaValidation:   req.DisableSchemaValidation,
		IPAddressType:             req.IPAddressType,
		CorsConfiguration:         req.CorsConfiguration,
		APIEndpoint:               fmt.Sprintf("https://%s.%s", id, s.endpointHost),
		CreatedDate:               time.Now().UTC(),
		Tags:                      req.Tags,
	}

	s.APIs[id] = &APIData{
		API:          api,
		Routes:       make(map[string]*Route),
		Integrations: make(map[string]*Integration),
		Stages:       make(map[string]*Stage),
		Deployments:  make(map[string]*Deployment),
	}

	s.saveLocked()

	return api, nil
}

// GetAPI returns an API by ID.
func (s *MemoryStorage) GetAPI(_ context.Context, apiID string) (*API, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	return data.API, nil
}

// GetAPIs returns all APIs.
func (s *MemoryStorage) GetAPIs(_ context.Context) ([]*API, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apis := make([]*API, 0, len(s.APIs))
	for _, data := range s.APIs {
		apis = append(apis, data.API)
	}

	return apis, nil
}

// UpdateAPI updates an existing API.
func (s *MemoryStorage) UpdateAPI(_ context.Context, apiID string, req *UpdateAPIRequest) (*API, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	api := data.API

	if req.Name != nil {
		api.Name = *req.Name
	}

	if req.Description != nil {
		api.Description = *req.Description
	}

	if req.Version != nil {
		api.Version = *req.Version
	}

	if req.APIKeySelectionExpression != nil {
		api.APIKeySelectionExpression = *req.APIKeySelectionExpression
	}

	if req.RouteSelectionExpression != nil {
		api.RouteSelectionExpression = *req.RouteSelectionExpression
	}

	if req.DisableExecuteAPIEndpoint != nil {
		api.DisableExecuteAPIEndpoint = *req.DisableExecuteAPIEndpoint
	}

	if req.DisableSchemaValidation != nil {
		api.DisableSchemaValidation = *req.DisableSchemaValidation
	}

	if req.IPAddressType != nil {
		api.IPAddressType = *req.IPAddressType
	}

	if req.CorsConfiguration != nil {
		api.CorsConfiguration = req.CorsConfiguration
	}

	s.saveLocked()

	return api, nil
}

// DeleteAPI deletes an API.
func (s *MemoryStorage) DeleteAPI(_ context.Context, apiID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.APIs[apiID]; !exists {
		return &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	delete(s.APIs, apiID)

	s.saveLocked()

	return nil
}

// CreateRoute creates a new route.
func (s *MemoryStorage) CreateRoute(_ context.Context, apiID string, req *CreateRouteRequest) (*Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	id := generateID()

	route := &Route{
		RouteID:                          id,
		RouteKey:                         req.RouteKey,
		Target:                           req.Target,
		AuthorizationType:                req.AuthorizationType,
		AuthorizerID:                     req.AuthorizerID,
		AuthorizationScopes:              req.AuthorizationScopes,
		APIKeyRequired:                   req.APIKeyRequired,
		OperationName:                    req.OperationName,
		RequestModels:                    req.RequestModels,
		RequestParameters:                req.RequestParameters,
		ModelSelectionExpression:         req.ModelSelectionExpression,
		RouteResponseSelectionExpression: req.RouteResponseSelectionExpression,
	}

	data.Routes[id] = route

	s.saveLocked()

	return route, nil
}

// GetRoute returns a route by ID.
func (s *MemoryStorage) GetRoute(_ context.Context, apiID, routeID string) (*Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	route, exists := data.Routes[routeID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid route identifier specified"}
	}

	return route, nil
}

// GetRoutes returns all routes for an API.
func (s *MemoryStorage) GetRoutes(_ context.Context, apiID string) ([]*Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	routes := make([]*Route, 0, len(data.Routes))
	for _, r := range data.Routes {
		routes = append(routes, r)
	}

	return routes, nil
}

// UpdateRoute updates an existing route.
func (s *MemoryStorage) UpdateRoute(_ context.Context, apiID, routeID string, req *CreateRouteRequest) (*Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	route, exists := data.Routes[routeID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid route identifier specified"}
	}

	if req.RouteKey != "" {
		route.RouteKey = req.RouteKey
	}

	if req.Target != "" {
		route.Target = req.Target
	}

	if req.AuthorizationType != "" {
		route.AuthorizationType = req.AuthorizationType
	}

	if req.AuthorizerID != "" {
		route.AuthorizerID = req.AuthorizerID
	}

	if req.AuthorizationScopes != nil {
		route.AuthorizationScopes = req.AuthorizationScopes
	}

	route.APIKeyRequired = req.APIKeyRequired

	if req.OperationName != "" {
		route.OperationName = req.OperationName
	}

	if req.RequestModels != nil {
		route.RequestModels = req.RequestModels
	}

	if req.RequestParameters != nil {
		route.RequestParameters = req.RequestParameters
	}

	s.saveLocked()

	return route, nil
}

// DeleteRoute deletes a route.
func (s *MemoryStorage) DeleteRoute(_ context.Context, apiID, routeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	if _, exists := data.Routes[routeID]; !exists {
		return &ServiceError{Code: errNotFound, Message: "Invalid route identifier specified"}
	}

	delete(data.Routes, routeID)

	s.saveLocked()

	return nil
}

// CreateIntegration creates a new integration.
func (s *MemoryStorage) CreateIntegration(_ context.Context, apiID string, req *CreateIntegrationRequest) (*Integration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	id := generateID()

	integration := &Integration{
		IntegrationID:               id,
		Description:                 req.Description,
		IntegrationType:             req.IntegrationType,
		IntegrationSubtype:          req.IntegrationSubtype,
		IntegrationMethod:           req.IntegrationMethod,
		IntegrationURI:              req.IntegrationURI,
		ConnectionType:              req.ConnectionType,
		ConnectionID:                req.ConnectionID,
		CredentialsArn:              req.CredentialsArn,
		PayloadFormatVersion:        req.PayloadFormatVersion,
		TimeoutInMillis:             req.TimeoutInMillis,
		RequestParameters:           req.RequestParameters,
		ResponseParameters:          req.ResponseParameters,
		RequestTemplates:            req.RequestTemplates,
		TemplateSelectionExpression: req.TemplateSelectionExpression,
		PassthroughBehavior:         req.PassthroughBehavior,
		ContentHandlingStrategy:     req.ContentHandlingStrategy,
		TLSConfig:                   req.TLSConfig,
	}

	data.Integrations[id] = integration

	s.saveLocked()

	return integration, nil
}

// GetIntegration returns an integration by ID.
func (s *MemoryStorage) GetIntegration(_ context.Context, apiID, integrationID string) (*Integration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	integration, exists := data.Integrations[integrationID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid integration identifier specified"}
	}

	return integration, nil
}

// GetIntegrations returns all integrations for an API.
func (s *MemoryStorage) GetIntegrations(_ context.Context, apiID string) ([]*Integration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	integrations := make([]*Integration, 0, len(data.Integrations))
	for _, i := range data.Integrations {
		integrations = append(integrations, i)
	}

	return integrations, nil
}

// UpdateIntegration updates an existing integration.
func (s *MemoryStorage) UpdateIntegration(_ context.Context, apiID, integrationID string, req *CreateIntegrationRequest) (*Integration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	integration, exists := data.Integrations[integrationID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid integration identifier specified"}
	}

	mergeStr(&integration.Description, req.Description)
	mergeStr(&integration.IntegrationType, req.IntegrationType)
	mergeStr(&integration.IntegrationMethod, req.IntegrationMethod)
	mergeStr(&integration.IntegrationURI, req.IntegrationURI)
	mergeStr(&integration.ConnectionType, req.ConnectionType)
	mergeStr(&integration.ConnectionID, req.ConnectionID)
	mergeStr(&integration.PayloadFormatVersion, req.PayloadFormatVersion)
	mergeStr(&integration.PassthroughBehavior, req.PassthroughBehavior)

	if req.TimeoutInMillis != 0 {
		integration.TimeoutInMillis = req.TimeoutInMillis
	}

	if req.RequestParameters != nil {
		integration.RequestParameters = req.RequestParameters
	}

	if req.ResponseParameters != nil {
		integration.ResponseParameters = req.ResponseParameters
	}

	if req.RequestTemplates != nil {
		integration.RequestTemplates = req.RequestTemplates
	}

	if req.TLSConfig != nil {
		integration.TLSConfig = req.TLSConfig
	}

	s.saveLocked()

	return integration, nil
}

// mergeStr assigns v to *dst only when v is non-empty, leaving the existing
// value untouched for partial updates.
func mergeStr(dst *string, v string) {
	if v != "" {
		*dst = v
	}
}

// DeleteIntegration deletes an integration.
func (s *MemoryStorage) DeleteIntegration(_ context.Context, apiID, integrationID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	if _, exists := data.Integrations[integrationID]; !exists {
		return &ServiceError{Code: errNotFound, Message: "Invalid integration identifier specified"}
	}

	delete(data.Integrations, integrationID)

	s.saveLocked()

	return nil
}

// CreateStage creates a new stage.
func (s *MemoryStorage) CreateStage(_ context.Context, apiID string, req *CreateStageRequest) (*Stage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	if _, exists := data.Stages[req.StageName]; exists {
		return nil, &ServiceError{Code: "ConflictException", Message: "Stage already exists"}
	}

	now := time.Now().UTC()

	stage := &Stage{
		StageName:            req.StageName,
		Description:          req.Description,
		DeploymentID:         req.DeploymentID,
		ClientCertificateID:  req.ClientCertificateID,
		DefaultRouteSettings: req.DefaultRouteSettings,
		RouteSettings:        req.RouteSettings,
		StageVariables:       req.StageVariables,
		AccessLogSettings:    req.AccessLogSettings,
		AutoDeploy:           req.AutoDeploy,
		CreatedDate:          now,
		LastUpdatedDate:      now,
		Tags:                 req.Tags,
	}

	data.Stages[req.StageName] = stage

	s.saveLocked()

	return stage, nil
}

// GetStage returns a stage by name.
func (s *MemoryStorage) GetStage(_ context.Context, apiID, stageName string) (*Stage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	stage, exists := data.Stages[stageName]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid stage identifier specified"}
	}

	return stage, nil
}

// GetStages returns all stages for an API.
func (s *MemoryStorage) GetStages(_ context.Context, apiID string) ([]*Stage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	stages := make([]*Stage, 0, len(data.Stages))
	for _, stage := range data.Stages {
		stages = append(stages, stage)
	}

	return stages, nil
}

// UpdateStage updates an existing stage.
func (s *MemoryStorage) UpdateStage(_ context.Context, apiID, stageName string, req *CreateStageRequest) (*Stage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	stage, exists := data.Stages[stageName]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid stage identifier specified"}
	}

	if req.Description != "" {
		stage.Description = req.Description
	}

	if req.DeploymentID != "" {
		stage.DeploymentID = req.DeploymentID
	}

	if req.DefaultRouteSettings != nil {
		stage.DefaultRouteSettings = req.DefaultRouteSettings
	}

	if req.RouteSettings != nil {
		stage.RouteSettings = req.RouteSettings
	}

	if req.StageVariables != nil {
		stage.StageVariables = req.StageVariables
	}

	if req.AccessLogSettings != nil {
		stage.AccessLogSettings = req.AccessLogSettings
	}

	stage.AutoDeploy = req.AutoDeploy
	stage.LastUpdatedDate = time.Now().UTC()

	s.saveLocked()

	return stage, nil
}

// DeleteStage deletes a stage.
func (s *MemoryStorage) DeleteStage(_ context.Context, apiID, stageName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	if _, exists := data.Stages[stageName]; !exists {
		return &ServiceError{Code: errNotFound, Message: "Invalid stage identifier specified"}
	}

	delete(data.Stages, stageName)

	s.saveLocked()

	return nil
}

// CreateDeployment creates a new deployment.
func (s *MemoryStorage) CreateDeployment(_ context.Context, apiID string, req *CreateDeploymentRequest) (*Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	id := generateID()

	deployment := &Deployment{
		DeploymentID:     id,
		Description:      req.Description,
		DeploymentStatus: deploymentStatusDeployed,
		CreatedDate:      time.Now().UTC(),
	}

	data.Deployments[id] = deployment

	// If a stage name is specified, associate the deployment with it.
	if req.StageName != "" {
		if stage, ok := data.Stages[req.StageName]; ok {
			stage.DeploymentID = id
			stage.LastUpdatedDate = time.Now().UTC()
		}
	}

	s.saveLocked()

	return deployment, nil
}

// GetDeployment returns a deployment by ID.
func (s *MemoryStorage) GetDeployment(_ context.Context, apiID, deploymentID string) (*Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	deployment, exists := data.Deployments[deploymentID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid deployment identifier specified"}
	}

	return deployment, nil
}

// GetDeployments returns all deployments for an API.
func (s *MemoryStorage) GetDeployments(_ context.Context, apiID string) ([]*Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	deployments := make([]*Deployment, 0, len(data.Deployments))
	for _, d := range data.Deployments {
		deployments = append(deployments, d)
	}

	return deployments, nil
}

// DeleteDeployment deletes a deployment.
func (s *MemoryStorage) DeleteDeployment(_ context.Context, apiID, deploymentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	if _, exists := data.Deployments[deploymentID]; !exists {
		return &ServiceError{Code: errNotFound, Message: "Invalid deployment identifier specified"}
	}

	delete(data.Deployments, deploymentID)

	s.saveLocked()

	return nil
}

// GetTags returns a copy of the tags for the resource identified by the ARN.
func (s *MemoryStorage) GetTags(_ context.Context, arn string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags, err := s.resolveTagTargetLocked(arn, false)
	if err != nil {
		return nil, err
	}

	out := make(map[string]string, len(tags))
	for k, v := range tags {
		out[k] = v
	}

	return out, nil
}

// TagResource adds or updates tags on the resource identified by the ARN.
func (s *MemoryStorage) TagResource(_ context.Context, arn string, tags map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	target, err := s.resolveTagTargetLocked(arn, true)
	if err != nil {
		return err
	}

	for k, v := range tags {
		target[k] = v
	}

	s.saveLocked()

	return nil
}

// UntagResource removes tags from the resource identified by the ARN.
func (s *MemoryStorage) UntagResource(_ context.Context, arn string, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	target, err := s.resolveTagTargetLocked(arn, false)
	if err != nil {
		return err
	}

	for _, k := range keys {
		delete(target, k)
	}

	s.saveLocked()

	return nil
}

// resolveTagTargetLocked returns the tag map of the API or stage identified by
// the ARN. Maps are reference types, so mutating the returned map updates the
// stored resource. When create is true a nil tag map is initialized and stored
// before being returned.
func (s *MemoryStorage) resolveTagTargetLocked(arn string, create bool) (map[string]string, error) {
	apiID, stageName := parseResourceARN(arn)
	if apiID == "" {
		return nil, &ServiceError{Code: errBadRequest, Message: "Invalid resource ARN"}
	}

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &ServiceError{Code: errNotFound, Message: "Invalid API identifier specified"}
	}

	if stageName != "" {
		stage, ok := data.Stages[stageName]
		if !ok {
			return nil, &ServiceError{Code: errNotFound, Message: "Invalid stage identifier specified"}
		}

		if stage.Tags == nil && create {
			stage.Tags = make(map[string]string)
		}

		return stage.Tags, nil
	}

	if data.API.Tags == nil && create {
		data.API.Tags = make(map[string]string)
	}

	return data.API.Tags, nil
}

// generateID generates a unique short ID.
func generateID() string {
	return uuid.New().String()[:10]
}
