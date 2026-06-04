package appsync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errNotFound       = "NotFoundException"
	errInvalidRequest = "BadRequestException"
	errConflict       = "ConcurrentModificationException"
)

// Storage defines the interface for AppSync storage operations.
type Storage interface {
	CreateGraphqlAPI(ctx context.Context, input *CreateGraphqlAPIInput) (*GraphqlAPI, error)
	DeleteGraphqlAPI(ctx context.Context, apiID string) error
	GetGraphqlAPI(ctx context.Context, apiID string) (*GraphqlAPI, error)
	ListGraphqlAPIs(ctx context.Context, input *ListGraphqlAPIsInput) ([]GraphqlAPI, string, error)
	CreateDataSource(ctx context.Context, input *CreateDataSourceInput) (*DataSource, error)
	CreateResolver(ctx context.Context, input *CreateResolverInput) (*Resolver, error)
	StartSchemaCreation(ctx context.Context, apiID string, definition []byte) (*SchemaCreationStatus, error)
}

// APIData holds all data associated with a GraphQL API.
type APIData struct {
	API         *GraphqlAPI            `json:"api"`
	DataSources map[string]*DataSource `json:"dataSources"` // key: name
	Resolvers   map[string]*Resolver   `json:"resolvers"`   // key: typeName:fieldName
	Schema      *SchemaCreationStatus  `json:"schema"`
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
	mu      sync.RWMutex        `json:"-"`
	APIs    map[string]*APIData `json:"apis"` // key: apiID
	dataDir string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		APIs: make(map[string]*APIData),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "appsync", s)
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

	storage.ScheduleSave(s.dataDir, "appsync", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "appsync", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateGraphqlAPI creates a new GraphQL API.
func (s *MemoryStorage) CreateGraphqlAPI(_ context.Context, input *CreateGraphqlAPIInput) (*GraphqlAPI, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.Name == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "Name is required",
		}
	}

	if input.AuthenticationType == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "AuthenticationType is required",
		}
	}

	apiID := uuid.New().String()
	apiARN := fmt.Sprintf("arn:aws:appsync:us-east-1:000000000000:apis/%s", apiID)

	api := &GraphqlAPI{
		APIId:              apiID,
		Name:               input.Name,
		AuthenticationType: input.AuthenticationType,
		ARN:                apiARN,
		URIs: map[string]string{
			"GRAPHQL":  fmt.Sprintf("https://%s.appsync-api.us-east-1.amazonaws.com/graphql", apiID),
			"REALTIME": fmt.Sprintf("wss://%s.appsync-realtime-api.us-east-1.amazonaws.com/graphql", apiID),
		},
		Tags:                              input.Tags,
		LogConfig:                         input.LogConfig,
		UserPoolConfig:                    input.UserPoolConfig,
		OpenIDConnectConfig:               input.OpenIDConnectConfig,
		AdditionalAuthenticationProviders: input.AdditionalAuthenticationProviders,
		XrayEnabled:                       input.XrayEnabled,
		LambdaAuthorizerConfig:            input.LambdaAuthorizerConfig,
		Visibility:                        input.Visibility,
		APIType:                           input.APIType,
		MergedAPIExecutionRoleARN:         input.MergedAPIExecutionRoleARN,
		OwnerContact:                      input.OwnerContact,
		IntrospectionConfig:               input.IntrospectionConfig,
		QueryDepthLimit:                   input.QueryDepthLimit,
		ResolverCountLimit:                input.ResolverCountLimit,
		EnhancedMetricsConfig:             input.EnhancedMetricsConfig,
	}

	s.APIs[apiID] = &APIData{
		API:         api,
		DataSources: make(map[string]*DataSource),
		Resolvers:   make(map[string]*Resolver),
	}

	s.saveLocked()

	return api, nil
}

// DeleteGraphqlAPI deletes a GraphQL API.
func (s *MemoryStorage) DeleteGraphqlAPI(_ context.Context, apiID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.APIs[apiID]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("GraphQL API %s not found", apiID),
		}
	}

	delete(s.APIs, apiID)

	s.saveLocked()

	return nil
}

// GetGraphqlAPI retrieves a GraphQL API.
func (s *MemoryStorage) GetGraphqlAPI(_ context.Context, apiID string) (*GraphqlAPI, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("GraphQL API %s not found", apiID),
		}
	}

	return data.API, nil
}

// defaultMaxResults is the default number of results to return.
const defaultMaxResults = 25

// ListGraphqlAPIs lists all GraphQL APIs with pagination support.
func (s *MemoryStorage) ListGraphqlAPIs(_ context.Context, input *ListGraphqlAPIsInput) ([]GraphqlAPI, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all matching APIs.
	allAPIs := make([]GraphqlAPI, 0, len(s.APIs))

	for _, data := range s.APIs {
		// Filter by API type if specified.
		if input.APIType != "" && data.API.APIType != input.APIType {
			continue
		}

		// Filter by owner if specified.
		if input.Owner != "" && data.API.Owner != input.Owner {
			continue
		}

		allAPIs = append(allAPIs, *data.API)
	}

	// Sort by API ID for consistent pagination.
	sort.Slice(allAPIs, func(i, j int) bool {
		return allAPIs[i].APIId < allAPIs[j].APIId
	})

	// Determine start index from nextToken.
	startIndex := 0

	if input.NextToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(input.NextToken)
		if err == nil {
			if idx, err := strconv.Atoi(string(decoded)); err == nil && idx >= 0 && idx < len(allAPIs) {
				startIndex = idx
			}
		}
	}

	// Determine max results.
	maxResults := int(input.MaxResults)
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	// Apply pagination.
	endIndex := startIndex + maxResults
	if endIndex > len(allAPIs) {
		endIndex = len(allAPIs)
	}

	result := allAPIs[startIndex:endIndex]

	// Generate next token if there are more results.
	var nextToken string
	if endIndex < len(allAPIs) {
		nextToken = base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(endIndex)))
	}

	return result, nextToken, nil
}

// CreateDataSource creates a new data source for a GraphQL API.
func (s *MemoryStorage) CreateDataSource(_ context.Context, input *CreateDataSourceInput) (*DataSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[input.APIID]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("GraphQL API %s not found", input.APIID),
		}
	}

	if input.Name == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "Name is required",
		}
	}

	if input.Type == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "Type is required",
		}
	}

	if _, exists := data.DataSources[input.Name]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("Data source %s already exists", input.Name),
		}
	}

	dataSourceARN := fmt.Sprintf("arn:aws:appsync:us-east-1:000000000000:apis/%s/datasources/%s",
		input.APIID, input.Name)

	dataSource := &DataSource{
		DataSourceARN:            dataSourceARN,
		Name:                     input.Name,
		Description:              input.Description,
		Type:                     input.Type,
		ServiceRoleARN:           input.ServiceRoleARN,
		DynamoDBConfig:           input.DynamoDBConfig,
		LambdaConfig:             input.LambdaConfig,
		ElasticsearchConfig:      input.ElasticsearchConfig,
		OpenSearchServiceConfig:  input.OpenSearchServiceConfig,
		HTTPConfig:               input.HTTPConfig,
		RelationalDatabaseConfig: input.RelationalDatabaseConfig,
		EventBridgeConfig:        input.EventBridgeConfig,
		MetricsConfig:            input.MetricsConfig,
	}

	data.DataSources[input.Name] = dataSource

	s.saveLocked()

	return dataSource, nil
}

// CreateResolver creates a new resolver for a GraphQL API.
func (s *MemoryStorage) CreateResolver(_ context.Context, input *CreateResolverInput) (*Resolver, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[input.APIID]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("GraphQL API %s not found", input.APIID),
		}
	}

	if input.TypeName == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "TypeName is required",
		}
	}

	if input.FieldName == "" {
		return nil, &Error{
			Code:    errInvalidRequest,
			Message: "FieldName is required",
		}
	}

	resolverKey := fmt.Sprintf("%s:%s", input.TypeName, input.FieldName)

	if _, exists := data.Resolvers[resolverKey]; exists {
		return nil, &Error{
			Code:    errConflict,
			Message: fmt.Sprintf("Resolver for %s.%s already exists", input.TypeName, input.FieldName),
		}
	}

	resolverARN := fmt.Sprintf("arn:aws:appsync:us-east-1:000000000000:apis/%s/types/%s/resolvers/%s",
		input.APIID, input.TypeName, input.FieldName)

	resolver := &Resolver{
		TypeName:                input.TypeName,
		FieldName:               input.FieldName,
		DataSourceName:          input.DataSourceName,
		ResolverARN:             resolverARN,
		RequestMappingTemplate:  input.RequestMappingTemplate,
		ResponseMappingTemplate: input.ResponseMappingTemplate,
		Kind:                    input.Kind,
		PipelineConfig:          input.PipelineConfig,
		SyncConfig:              input.SyncConfig,
		CachingConfig:           input.CachingConfig,
		MaxBatchSize:            input.MaxBatchSize,
		Runtime:                 input.Runtime,
		Code:                    input.Code,
		MetricsConfig:           input.MetricsConfig,
	}

	data.Resolvers[resolverKey] = resolver

	s.saveLocked()

	return resolver, nil
}

// StartSchemaCreation starts schema creation for a GraphQL API.
func (s *MemoryStorage) StartSchemaCreation(_ context.Context, apiID string, _ []byte) (*SchemaCreationStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.APIs[apiID]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("GraphQL API %s not found", apiID),
		}
	}

	// In a real implementation, we would parse and validate the schema.
	// For the emulator, we just mark it as successful.
	status := &SchemaCreationStatus{
		Status:  SchemaStatusSuccess,
		Details: "Schema created successfully",
	}

	data.Schema = status

	s.saveLocked()

	return status, nil
}
