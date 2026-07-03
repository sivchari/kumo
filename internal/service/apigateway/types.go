package apigateway

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// RestAPI represents an API Gateway REST API.
type RestAPI struct {
	ID                     string            `json:"id"`
	Name                   string            `json:"name"`
	Description            string            `json:"description,omitempty"`
	CreatedDate            time.Time         `json:"createdDate"`
	Version                string            `json:"version,omitempty"`
	APIKeySource           string            `json:"apiKeySource,omitempty"`
	EndpointConfiguration  *EndpointConfig   `json:"endpointConfiguration,omitempty"`
	DisableExecuteAPIEndpt bool              `json:"disableExecuteApiEndpoint,omitempty"`
	Tags                   map[string]string `json:"tags,omitempty"`
	RootResourceID         string            `json:"-"` // Internal use.
}

// EndpointConfig represents the endpoint configuration for an API.
type EndpointConfig struct {
	Types          []string `json:"types,omitempty"`
	VpcEndpointIDs []string `json:"vpcEndpointIds,omitempty"`
	IPAddressType  string   `json:"ipAddressType,omitempty"`
}

// Resource represents an API Gateway resource.
type Resource struct {
	ID              string            `json:"id"`
	ParentID        string            `json:"parentId,omitempty"`
	PathPart        string            `json:"pathPart,omitempty"`
	Path            string            `json:"path"`
	ResourceMethods map[string]Method `json:"resourceMethods,omitempty"`
}

// Method represents an API Gateway method.
type Method struct {
	HTTPMethod        string       `json:"httpMethod"`
	AuthorizationType string       `json:"authorizationType,omitempty"`
	APIKeyRequired    bool         `json:"apiKeyRequired,omitempty"`
	OperationName     string       `json:"operationName,omitempty"`
	MethodIntegration *Integration `json:"methodIntegration,omitempty"`
}

// Integration represents an API Gateway integration.
type Integration struct {
	Type                string            `json:"type"`
	HTTPMethod          string            `json:"httpMethod,omitempty"`
	URI                 string            `json:"uri,omitempty"`
	ConnectionType      string            `json:"connectionType,omitempty"`
	ConnectionID        string            `json:"connectionId,omitempty"`
	PassthroughBehavior string            `json:"passthroughBehavior,omitempty"`
	ContentHandling     string            `json:"contentHandling,omitempty"`
	TimeoutInMillis     int32             `json:"timeoutInMillis,omitempty"`
	CacheNamespace      string            `json:"cacheNamespace,omitempty"`
	CacheKeyParameters  []string          `json:"cacheKeyParameters,omitempty"`
	RequestParameters   map[string]string `json:"requestParameters,omitempty"`
	RequestTemplates    map[string]string `json:"requestTemplates,omitempty"`
}

// Deployment represents an API Gateway deployment.
type Deployment struct {
	ID          string    `json:"id"`
	Description string    `json:"description,omitempty"`
	CreatedDate time.Time `json:"createdDate"`
}

// Stage represents an API Gateway stage.
type Stage struct {
	StageName           string            `json:"stageName"`
	DeploymentID        string            `json:"deploymentId"`
	Description         string            `json:"description,omitempty"`
	CacheClusterEnabled bool              `json:"cacheClusterEnabled,omitempty"`
	CacheClusterSize    string            `json:"cacheClusterSize,omitempty"`
	CreatedDate         time.Time         `json:"createdDate"`
	LastUpdatedDate     time.Time         `json:"lastUpdatedDate"`
	Tags                map[string]string `json:"tags,omitempty"`
}

// CreateRestAPIRequest represents a CreateRestApi request.
type CreateRestAPIRequest struct {
	Name                   string            `json:"name"`
	Description            string            `json:"description,omitempty"`
	Version                string            `json:"version,omitempty"`
	APIKeySource           string            `json:"apiKeySource,omitempty"`
	EndpointConfiguration  *EndpointConfig   `json:"endpointConfiguration,omitempty"`
	DisableExecuteAPIEndpt bool              `json:"disableExecuteApiEndpoint,omitempty"`
	Tags                   map[string]string `json:"tags,omitempty"`
}

// CreateRestAPIResponse represents a CreateRestApi response.
type CreateRestAPIResponse struct {
	ID                     string            `json:"id"`
	Name                   string            `json:"name"`
	Description            string            `json:"description,omitempty"`
	CreatedDate            float64           `json:"createdDate"`
	Version                string            `json:"version,omitempty"`
	APIKeySource           string            `json:"apiKeySource,omitempty"`
	EndpointConfiguration  *EndpointConfig   `json:"endpointConfiguration,omitempty"`
	DisableExecuteAPIEndpt bool              `json:"disableExecuteApiEndpoint,omitempty"`
	Tags                   map[string]string `json:"tags,omitempty"`
	RootResourceID         string            `json:"rootResourceId,omitempty"`
}

// GetRestAPIsResponse represents a GetRestApis response.
type GetRestAPIsResponse struct {
	Items    []CreateRestAPIResponse `json:"item,omitempty"`
	Position string                  `json:"position,omitempty"`
}

// CreateResourceRequest represents a CreateResource request.
type CreateResourceRequest struct {
	PathPart string `json:"pathPart"`
}

// ResourceResponse represents a Resource response.
type ResourceResponse struct {
	ID              string                  `json:"id"`
	ParentID        string                  `json:"parentId,omitempty"`
	PathPart        string                  `json:"pathPart,omitempty"`
	Path            string                  `json:"path"`
	ResourceMethods map[string]MethodOutput `json:"resourceMethods,omitempty"`
}

// MethodOutput represents a Method in response.
type MethodOutput struct {
	HTTPMethod        string             `json:"httpMethod,omitempty"`
	AuthorizationType string             `json:"authorizationType,omitempty"`
	APIKeyRequired    bool               `json:"apiKeyRequired,omitempty"`
	OperationName     string             `json:"operationName,omitempty"`
	MethodIntegration *IntegrationOutput `json:"methodIntegration,omitempty"`
}

// IntegrationOutput represents an Integration in response.
type IntegrationOutput struct {
	Type                string            `json:"type,omitempty"`
	HTTPMethod          string            `json:"httpMethod,omitempty"`
	URI                 string            `json:"uri,omitempty"`
	ConnectionType      string            `json:"connectionType,omitempty"`
	ConnectionID        string            `json:"connectionId,omitempty"`
	PassthroughBehavior string            `json:"passthroughBehavior,omitempty"`
	ContentHandling     string            `json:"contentHandling,omitempty"`
	TimeoutInMillis     int32             `json:"timeoutInMillis,omitempty"`
	CacheNamespace      string            `json:"cacheNamespace,omitempty"`
	CacheKeyParameters  []string          `json:"cacheKeyParameters,omitempty"`
	RequestParameters   map[string]string `json:"requestParameters,omitempty"`
	RequestTemplates    map[string]string `json:"requestTemplates,omitempty"`
}

// GetResourcesResponse represents a GetResources response.
type GetResourcesResponse struct {
	Items    []ResourceResponse `json:"item,omitempty"`
	Position string             `json:"position,omitempty"`
}

// PutMethodRequest represents a PutMethod request.
type PutMethodRequest struct {
	AuthorizationType string `json:"authorizationType"`
	APIKeyRequired    bool   `json:"apiKeyRequired,omitempty"`
	OperationName     string `json:"operationName,omitempty"`
}

// PutIntegrationRequest represents a PutIntegration request.
type PutIntegrationRequest struct {
	Type                string            `json:"type"`
	HTTPMethod          string            `json:"httpMethod,omitempty"`
	URI                 string            `json:"uri,omitempty"`
	ConnectionType      string            `json:"connectionType,omitempty"`
	ConnectionID        string            `json:"connectionId,omitempty"`
	PassthroughBehavior string            `json:"passthroughBehavior,omitempty"`
	ContentHandling     string            `json:"contentHandling,omitempty"`
	TimeoutInMillis     int32             `json:"timeoutInMillis,omitempty"`
	CacheNamespace      string            `json:"cacheNamespace,omitempty"`
	CacheKeyParameters  []string          `json:"cacheKeyParameters,omitempty"`
	RequestParameters   map[string]string `json:"requestParameters,omitempty"`
	RequestTemplates    map[string]string `json:"requestTemplates,omitempty"`
}

// CreateDeploymentRequest represents a CreateDeployment request.
type CreateDeploymentRequest struct {
	StageName   string `json:"stageName,omitempty"`
	Description string `json:"description,omitempty"`
}

// DeploymentResponse represents a Deployment response.
type DeploymentResponse struct {
	ID          string  `json:"id"`
	Description string  `json:"description,omitempty"`
	CreatedDate float64 `json:"createdDate"`
}

// GetDeploymentsResponse represents a GetDeployments response.
type GetDeploymentsResponse struct {
	Items    []DeploymentResponse `json:"item,omitempty"`
	Position string               `json:"position,omitempty"`
}

// CreateStageRequest represents a CreateStage request.
type CreateStageRequest struct {
	StageName           string            `json:"stageName"`
	DeploymentID        string            `json:"deploymentId"`
	Description         string            `json:"description,omitempty"`
	CacheClusterEnabled bool              `json:"cacheClusterEnabled,omitempty"`
	CacheClusterSize    string            `json:"cacheClusterSize,omitempty"`
	Tags                map[string]string `json:"tags,omitempty"`
}

// StageResponse represents a Stage response.
type StageResponse struct {
	StageName           string            `json:"stageName"`
	DeploymentID        string            `json:"deploymentId"`
	Description         string            `json:"description,omitempty"`
	CacheClusterEnabled bool              `json:"cacheClusterEnabled,omitempty"`
	CacheClusterSize    string            `json:"cacheClusterSize,omitempty"`
	CreatedDate         float64           `json:"createdDate"`
	LastUpdatedDate     float64           `json:"lastUpdatedDate"`
	Tags                map[string]string `json:"tags,omitempty"`
}

// GetStagesResponse represents a GetStages response.
type GetStagesResponse struct {
	Items []StageResponse `json:"item,omitempty"`
}

// ErrorResponse represents an API Gateway error response.
type ErrorResponse struct {
	Type    string `json:"__type,omitempty"`
	Message string `json:"message"`
}

// ServiceError represents a service error.
type ServiceError = service.CodedError
