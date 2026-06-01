package apigatewayv2

import "time"

// API represents an API Gateway v2 API (HTTP or WebSocket).
type API struct {
	APIID                     string            `json:"apiId"`
	Name                      string            `json:"name"`
	ProtocolType              string            `json:"protocolType"`
	Description               string            `json:"description,omitempty"`
	Version                   string            `json:"version,omitempty"`
	APIKeySelectionExpression string            `json:"apiKeySelectionExpression,omitempty"`
	RouteSelectionExpression  string            `json:"routeSelectionExpression,omitempty"`
	DisableExecuteAPIEndpoint bool              `json:"disableExecuteApiEndpoint,omitempty"`
	DisableSchemaValidation   bool              `json:"disableSchemaValidation,omitempty"`
	IPAddressType             string            `json:"ipAddressType,omitempty"`
	CorsConfiguration         *Cors             `json:"corsConfiguration,omitempty"`
	APIEndpoint               string            `json:"apiEndpoint,omitempty"`
	APIGatewayManaged         bool              `json:"apiGatewayManaged,omitempty"`
	CreatedDate               time.Time         `json:"createdDate"`
	Tags                      map[string]string `json:"tags,omitempty"`
}

// Cors represents a CORS configuration. Supported only for HTTP APIs.
type Cors struct {
	AllowOrigins     []string `json:"allowOrigins,omitempty"`
	AllowMethods     []string `json:"allowMethods,omitempty"`
	AllowHeaders     []string `json:"allowHeaders,omitempty"`
	ExposeHeaders    []string `json:"exposeHeaders,omitempty"`
	AllowCredentials bool     `json:"allowCredentials,omitempty"`
	MaxAge           int32    `json:"maxAge,omitempty"`
}

// Route represents a route for an API.
type Route struct {
	RouteID                          string                          `json:"routeId"`
	RouteKey                         string                          `json:"routeKey"`
	Target                           string                          `json:"target,omitempty"`
	AuthorizationType                string                          `json:"authorizationType,omitempty"`
	AuthorizerID                     string                          `json:"authorizerId,omitempty"`
	AuthorizationScopes              []string                        `json:"authorizationScopes,omitempty"`
	APIKeyRequired                   bool                            `json:"apiKeyRequired,omitempty"`
	OperationName                    string                          `json:"operationName,omitempty"`
	RequestModels                    map[string]string               `json:"requestModels,omitempty"`
	RequestParameters                map[string]ParameterConstraints `json:"requestParameters,omitempty"`
	ModelSelectionExpression         string                          `json:"modelSelectionExpression,omitempty"`
	RouteResponseSelectionExpression string                          `json:"routeResponseSelectionExpression,omitempty"`
	APIGatewayManaged                bool                            `json:"apiGatewayManaged,omitempty"`
}

// ParameterConstraints represents validation constraints on a request parameter.
type ParameterConstraints struct {
	Required bool `json:"required,omitempty"`
}

// Integration represents an integration for an API.
type Integration struct {
	IntegrationID                          string                       `json:"integrationId"`
	Description                            string                       `json:"description,omitempty"`
	IntegrationType                        string                       `json:"integrationType,omitempty"`
	IntegrationSubtype                     string                       `json:"integrationSubtype,omitempty"`
	IntegrationMethod                      string                       `json:"integrationMethod,omitempty"`
	IntegrationURI                         string                       `json:"integrationUri,omitempty"`
	ConnectionType                         string                       `json:"connectionType,omitempty"`
	ConnectionID                           string                       `json:"connectionId,omitempty"`
	CredentialsArn                         string                       `json:"credentialsArn,omitempty"`
	PayloadFormatVersion                   string                       `json:"payloadFormatVersion,omitempty"`
	TimeoutInMillis                        int32                        `json:"timeoutInMillis,omitempty"`
	RequestParameters                      map[string]string            `json:"requestParameters,omitempty"`
	ResponseParameters                     map[string]map[string]string `json:"responseParameters,omitempty"`
	RequestTemplates                       map[string]string            `json:"requestTemplates,omitempty"`
	TemplateSelectionExpression            string                       `json:"templateSelectionExpression,omitempty"`
	PassthroughBehavior                    string                       `json:"passthroughBehavior,omitempty"`
	ContentHandlingStrategy                string                       `json:"contentHandlingStrategy,omitempty"`
	TLSConfig                              *TLSConfig                   `json:"tlsConfig,omitempty"`
	IntegrationResponseSelectionExpression string                       `json:"integrationResponseSelectionExpression,omitempty"`
	APIGatewayManaged                      bool                         `json:"apiGatewayManaged,omitempty"`
}

// TLSConfig represents the TLS configuration for a private integration.
type TLSConfig struct {
	ServerNameToVerify string `json:"serverNameToVerify,omitempty"`
}

// Stage represents a stage for an API.
type Stage struct {
	StageName                   string                   `json:"stageName"`
	Description                 string                   `json:"description,omitempty"`
	DeploymentID                string                   `json:"deploymentId,omitempty"`
	ClientCertificateID         string                   `json:"clientCertificateId,omitempty"`
	DefaultRouteSettings        *RouteSettings           `json:"defaultRouteSettings,omitempty"`
	RouteSettings               map[string]RouteSettings `json:"routeSettings,omitempty"`
	StageVariables              map[string]string        `json:"stageVariables,omitempty"`
	AccessLogSettings           *AccessLogSettings       `json:"accessLogSettings,omitempty"`
	AutoDeploy                  bool                     `json:"autoDeploy,omitempty"`
	LastDeploymentStatusMessage string                   `json:"lastDeploymentStatusMessage,omitempty"`
	APIGatewayManaged           bool                     `json:"apiGatewayManaged,omitempty"`
	CreatedDate                 time.Time                `json:"createdDate"`
	LastUpdatedDate             time.Time                `json:"lastUpdatedDate"`
	Tags                        map[string]string        `json:"tags,omitempty"`
}

// RouteSettings represents a collection of route settings.
type RouteSettings struct {
	DetailedMetricsEnabled bool    `json:"detailedMetricsEnabled,omitempty"`
	LoggingLevel           string  `json:"loggingLevel,omitempty"`
	DataTraceEnabled       bool    `json:"dataTraceEnabled,omitempty"`
	ThrottlingBurstLimit   int32   `json:"throttlingBurstLimit,omitempty"`
	ThrottlingRateLimit    float64 `json:"throttlingRateLimit,omitempty"`
}

// AccessLogSettings represents settings for logging access in a stage.
type AccessLogSettings struct {
	DestinationArn string `json:"destinationArn,omitempty"`
	Format         string `json:"format,omitempty"`
}

// Deployment represents a deployment for an API.
type Deployment struct {
	DeploymentID     string    `json:"deploymentId"`
	Description      string    `json:"description,omitempty"`
	DeploymentStatus string    `json:"deploymentStatus,omitempty"`
	AutoDeployed     bool      `json:"autoDeployed,omitempty"`
	CreatedDate      time.Time `json:"createdDate"`
}

// ---- Request types ----

// CreateAPIRequest represents a CreateApi request.
type CreateAPIRequest struct {
	Name                      string            `json:"name"`
	ProtocolType              string            `json:"protocolType"`
	Description               string            `json:"description,omitempty"`
	Version                   string            `json:"version,omitempty"`
	APIKeySelectionExpression string            `json:"apiKeySelectionExpression,omitempty"`
	RouteSelectionExpression  string            `json:"routeSelectionExpression,omitempty"`
	DisableExecuteAPIEndpoint bool              `json:"disableExecuteApiEndpoint,omitempty"`
	DisableSchemaValidation   bool              `json:"disableSchemaValidation,omitempty"`
	IPAddressType             string            `json:"ipAddressType,omitempty"`
	CorsConfiguration         *Cors             `json:"corsConfiguration,omitempty"`
	CredentialsArn            string            `json:"credentialsArn,omitempty"`
	Target                    string            `json:"target,omitempty"`
	RouteKey                  string            `json:"routeKey,omitempty"`
	Tags                      map[string]string `json:"tags,omitempty"`
}

// UpdateAPIRequest represents an UpdateApi request. Pointer fields allow
// partial updates: a nil field leaves the stored value unchanged.
type UpdateAPIRequest struct {
	Name                      *string `json:"name,omitempty"`
	Description               *string `json:"description,omitempty"`
	Version                   *string `json:"version,omitempty"`
	APIKeySelectionExpression *string `json:"apiKeySelectionExpression,omitempty"`
	RouteSelectionExpression  *string `json:"routeSelectionExpression,omitempty"`
	DisableExecuteAPIEndpoint *bool   `json:"disableExecuteApiEndpoint,omitempty"`
	DisableSchemaValidation   *bool   `json:"disableSchemaValidation,omitempty"`
	IPAddressType             *string `json:"ipAddressType,omitempty"`
	CorsConfiguration         *Cors   `json:"corsConfiguration,omitempty"`
}

// CreateRouteRequest represents a CreateRoute request.
type CreateRouteRequest struct {
	RouteKey                         string                          `json:"routeKey"`
	Target                           string                          `json:"target,omitempty"`
	AuthorizationType                string                          `json:"authorizationType,omitempty"`
	AuthorizerID                     string                          `json:"authorizerId,omitempty"`
	AuthorizationScopes              []string                        `json:"authorizationScopes,omitempty"`
	APIKeyRequired                   bool                            `json:"apiKeyRequired,omitempty"`
	OperationName                    string                          `json:"operationName,omitempty"`
	RequestModels                    map[string]string               `json:"requestModels,omitempty"`
	RequestParameters                map[string]ParameterConstraints `json:"requestParameters,omitempty"`
	ModelSelectionExpression         string                          `json:"modelSelectionExpression,omitempty"`
	RouteResponseSelectionExpression string                          `json:"routeResponseSelectionExpression,omitempty"`
}

// CreateIntegrationRequest represents a CreateIntegration request.
type CreateIntegrationRequest struct {
	Description                 string                       `json:"description,omitempty"`
	IntegrationType             string                       `json:"integrationType"`
	IntegrationSubtype          string                       `json:"integrationSubtype,omitempty"`
	IntegrationMethod           string                       `json:"integrationMethod,omitempty"`
	IntegrationURI              string                       `json:"integrationUri,omitempty"`
	ConnectionType              string                       `json:"connectionType,omitempty"`
	ConnectionID                string                       `json:"connectionId,omitempty"`
	CredentialsArn              string                       `json:"credentialsArn,omitempty"`
	PayloadFormatVersion        string                       `json:"payloadFormatVersion,omitempty"`
	TimeoutInMillis             int32                        `json:"timeoutInMillis,omitempty"`
	RequestParameters           map[string]string            `json:"requestParameters,omitempty"`
	ResponseParameters          map[string]map[string]string `json:"responseParameters,omitempty"`
	RequestTemplates            map[string]string            `json:"requestTemplates,omitempty"`
	TemplateSelectionExpression string                       `json:"templateSelectionExpression,omitempty"`
	PassthroughBehavior         string                       `json:"passthroughBehavior,omitempty"`
	ContentHandlingStrategy     string                       `json:"contentHandlingStrategy,omitempty"`
	TLSConfig                   *TLSConfig                   `json:"tlsConfig,omitempty"`
}

// CreateStageRequest represents a CreateStage request.
type CreateStageRequest struct {
	StageName            string                   `json:"stageName"`
	Description          string                   `json:"description,omitempty"`
	DeploymentID         string                   `json:"deploymentId,omitempty"`
	ClientCertificateID  string                   `json:"clientCertificateId,omitempty"`
	DefaultRouteSettings *RouteSettings           `json:"defaultRouteSettings,omitempty"`
	RouteSettings        map[string]RouteSettings `json:"routeSettings,omitempty"`
	StageVariables       map[string]string        `json:"stageVariables,omitempty"`
	AccessLogSettings    *AccessLogSettings       `json:"accessLogSettings,omitempty"`
	AutoDeploy           bool                     `json:"autoDeploy,omitempty"`
	Tags                 map[string]string        `json:"tags,omitempty"`
}

// CreateDeploymentRequest represents a CreateDeployment request.
type CreateDeploymentRequest struct {
	Description string `json:"description,omitempty"`
	StageName   string `json:"stageName,omitempty"`
}

// TagResourceRequest represents a TagResource request.
type TagResourceRequest struct {
	Tags map[string]string `json:"tags,omitempty"`
}

// ---- Response types ----

// APIResponse represents an Api response with an ISO8601 createdDate.
type APIResponse struct {
	APIID                     string            `json:"apiId"`
	Name                      string            `json:"name"`
	ProtocolType              string            `json:"protocolType"`
	Description               string            `json:"description,omitempty"`
	Version                   string            `json:"version,omitempty"`
	APIKeySelectionExpression string            `json:"apiKeySelectionExpression,omitempty"`
	RouteSelectionExpression  string            `json:"routeSelectionExpression,omitempty"`
	DisableExecuteAPIEndpoint bool              `json:"disableExecuteApiEndpoint,omitempty"`
	DisableSchemaValidation   bool              `json:"disableSchemaValidation,omitempty"`
	IPAddressType             string            `json:"ipAddressType,omitempty"`
	CorsConfiguration         *Cors             `json:"corsConfiguration,omitempty"`
	APIEndpoint               string            `json:"apiEndpoint,omitempty"`
	APIGatewayManaged         bool              `json:"apiGatewayManaged,omitempty"`
	CreatedDate               string            `json:"createdDate,omitempty"`
	Tags                      map[string]string `json:"tags,omitempty"`
}

// APIsResponse represents a GetApis response.
type APIsResponse struct {
	Items     []APIResponse `json:"items"`
	NextToken string        `json:"nextToken,omitempty"`
}

// RoutesResponse represents a GetRoutes response.
type RoutesResponse struct {
	Items     []Route `json:"items"`
	NextToken string  `json:"nextToken,omitempty"`
}

// IntegrationsResponse represents a GetIntegrations response.
type IntegrationsResponse struct {
	Items     []Integration `json:"items"`
	NextToken string        `json:"nextToken,omitempty"`
}

// StageResponse represents a Stage response with ISO8601 dates.
type StageResponse struct {
	StageName                   string                   `json:"stageName"`
	Description                 string                   `json:"description,omitempty"`
	DeploymentID                string                   `json:"deploymentId,omitempty"`
	ClientCertificateID         string                   `json:"clientCertificateId,omitempty"`
	DefaultRouteSettings        *RouteSettings           `json:"defaultRouteSettings,omitempty"`
	RouteSettings               map[string]RouteSettings `json:"routeSettings,omitempty"`
	StageVariables              map[string]string        `json:"stageVariables,omitempty"`
	AccessLogSettings           *AccessLogSettings       `json:"accessLogSettings,omitempty"`
	AutoDeploy                  bool                     `json:"autoDeploy,omitempty"`
	LastDeploymentStatusMessage string                   `json:"lastDeploymentStatusMessage,omitempty"`
	APIGatewayManaged           bool                     `json:"apiGatewayManaged,omitempty"`
	CreatedDate                 string                   `json:"createdDate,omitempty"`
	LastUpdatedDate             string                   `json:"lastUpdatedDate,omitempty"`
	Tags                        map[string]string        `json:"tags,omitempty"`
}

// StagesResponse represents a GetStages response.
type StagesResponse struct {
	Items     []StageResponse `json:"items"`
	NextToken string          `json:"nextToken,omitempty"`
}

// DeploymentResponse represents a Deployment response with an ISO8601 createdDate.
type DeploymentResponse struct {
	DeploymentID     string `json:"deploymentId"`
	Description      string `json:"description,omitempty"`
	DeploymentStatus string `json:"deploymentStatus,omitempty"`
	AutoDeployed     bool   `json:"autoDeployed,omitempty"`
	CreatedDate      string `json:"createdDate,omitempty"`
}

// DeploymentsResponse represents a GetDeployments response.
type DeploymentsResponse struct {
	Items     []DeploymentResponse `json:"items"`
	NextToken string               `json:"nextToken,omitempty"`
}

// TagsResponse represents a GetTags response.
type TagsResponse struct {
	Tags map[string]string `json:"tags"`
}

// ErrorResponse represents an API Gateway v2 error response.
type ErrorResponse struct {
	Message string `json:"message"`
}

// ServiceError represents a service error with an AWS error code.
type ServiceError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *ServiceError) Error() string {
	return e.Message
}
