package apigateway

import (
	"encoding/json"
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
	AuthorizerID      string       `json:"authorizerId,omitempty"`
	APIKeyRequired    bool         `json:"apiKeyRequired,omitempty"`
	OperationName     string       `json:"operationName,omitempty"`
	MethodIntegration *Integration `json:"methodIntegration,omitempty"`
}

// Authorizer represents an API Gateway authorizer. Only the fields kumo needs
// to run a REQUEST-type Lambda authorizer are modeled.
type Authorizer struct {
	ID                           string `json:"id"`
	RestAPIID                    string `json:"-"` // internal: owning REST API
	Name                         string `json:"name"`
	Type                         string `json:"type"` // "REQUEST" | "TOKEN"
	AuthorizerURI                string `json:"authorizerUri,omitempty"`
	IdentitySource               string `json:"identitySource,omitempty"`
	AuthorizerResultTTLInSeconds int32  `json:"authorizerResultTtlInSeconds,omitempty"`
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
	AuthorizerID      string             `json:"authorizerId,omitempty"`
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
	AuthorizerID      string `json:"authorizerId,omitempty"`
	APIKeyRequired    bool   `json:"apiKeyRequired,omitempty"`
	OperationName     string `json:"operationName,omitempty"`
}

// CreateAuthorizerRequest represents a CreateAuthorizer request.
type CreateAuthorizerRequest struct {
	Name                         string `json:"name"`
	Type                         string `json:"type"`
	AuthorizerURI                string `json:"authorizerUri,omitempty"`
	IdentitySource               string `json:"identitySource,omitempty"`
	AuthorizerResultTTLInSeconds int32  `json:"authorizerResultTtlInSeconds,omitempty"`
}

// AuthorizerResponse represents an Authorizer response.
type AuthorizerResponse struct {
	ID                           string `json:"id"`
	Name                         string `json:"name"`
	Type                         string `json:"type"`
	AuthorizerURI                string `json:"authorizerUri,omitempty"`
	IdentitySource               string `json:"identitySource,omitempty"`
	AuthorizerResultTTLInSeconds int32  `json:"authorizerResultTtlInSeconds,omitempty"`
}

// GetAuthorizersResponse represents a GetAuthorizers response.
type GetAuthorizersResponse struct {
	Items    []AuthorizerResponse `json:"item,omitempty"`
	Position string               `json:"position,omitempty"`
}

// AuthorizerEvent is the REQUEST-type Lambda authorizer input event
// (API Gateway v1 format).
type AuthorizerEvent struct {
	Type                  string               `json:"type"`
	MethodArn             string               `json:"methodArn"`
	Resource              string               `json:"resource"`
	Path                  string               `json:"path"`
	HTTPMethod            string               `json:"httpMethod"`
	Headers               map[string]string    `json:"headers"`
	QueryStringParameters map[string]string    `json:"queryStringParameters"`
	PathParameters        map[string]string    `json:"pathParameters"`
	RequestContext        AuthorizerRequestCtx `json:"requestContext"`
}

// AuthorizerRequestCtx is the requestContext block of an authorizer event.
type AuthorizerRequestCtx struct {
	APIID        string `json:"apiId"`
	Stage        string `json:"stage"`
	HTTPMethod   string `json:"httpMethod"`
	ResourcePath string `json:"resourcePath"`
}

// AuthorizerOutput is the REQUEST-type authorizer response: a principal plus an
// IAM policy document. The policy fields use AWS IAM PascalCase keys.
type AuthorizerOutput struct {
	PrincipalID    string         `json:"principalId"`
	PolicyDocument PolicyDocument `json:"policyDocument"`
	Context        map[string]any `json:"context,omitempty"`
}

// PolicyDocument is an IAM policy document.
type PolicyDocument struct {
	Version   string            `json:"Version"`
	Statement []PolicyStatement `json:"Statement"`
}

// PolicyStatement is a single IAM policy statement. Resource may be a string or
// an array of strings, so it is kept raw and normalized during evaluation.
type PolicyStatement struct {
	Action   json.RawMessage `json:"Action"`
	Effect   string          `json:"Effect"`
	Resource json.RawMessage `json:"Resource"`
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
