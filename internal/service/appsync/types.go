// Package appsync provides AWS AppSync service emulation for kumo.
package appsync

import "github.com/sivchari/kumo/internal/service"

// Authentication types.
const (
	AuthTypeAPIKey           = "API_KEY"
	AuthTypeAWSIAM           = "AWS_IAM"
	AuthTypeCognitoUserPools = "AMAZON_COGNITO_USER_POOLS"
	AuthTypeOpenIDConnect    = "OPENID_CONNECT"
	AuthTypeAWSLambda        = "AWS_LAMBDA"
)

// Data source types.
const (
	DataSourceTypeAWSDynamoDB   = "AMAZON_DYNAMODB"
	DataSourceTypeAWSLambda     = "AWS_LAMBDA"
	DataSourceTypeElasticsearch = "AMAZON_ELASTICSEARCH"
	DataSourceTypeOpenSearch    = "AMAZON_OPENSEARCH_SERVICE"
	DataSourceTypeHTTP          = "HTTP"
	DataSourceTypeNone          = "NONE"
	DataSourceTypeRelationalDB  = "RELATIONAL_DATABASE"
	DataSourceTypeEventBridge   = "AMAZON_EVENTBRIDGE"
)

// Schema status.
const (
	SchemaStatusProcessing    = "PROCESSING"
	SchemaStatusActive        = "ACTIVE"
	SchemaStatusDeleting      = "DELETING"
	SchemaStatusFailed        = "FAILED"
	SchemaStatusSuccess       = "SUCCESS"
	SchemaStatusNotApplicable = "NOT_APPLICABLE"
)

// GraphqlAPI represents an AWS AppSync GraphQL API.
type GraphqlAPI struct {
	APIId                             string                   `json:"apiId,omitempty"`
	Name                              string                   `json:"name,omitempty"`
	AuthenticationType                string                   `json:"authenticationType,omitempty"`
	LogConfig                         *LogConfig               `json:"logConfig,omitempty"`
	UserPoolConfig                    *UserPoolConfig          `json:"userPoolConfig,omitempty"`
	OpenIDConnectConfig               *OpenIDConnectConfig     `json:"openIDConnectConfig,omitempty"`
	ARN                               string                   `json:"arn,omitempty"`
	URIs                              map[string]string        `json:"uris,omitempty"`
	Tags                              map[string]string        `json:"tags,omitempty"`
	AdditionalAuthenticationProviders []AuthenticationProvider `json:"additionalAuthenticationProviders,omitempty"`
	XrayEnabled                       bool                     `json:"xrayEnabled,omitempty"`
	WafWebACLARN                      string                   `json:"wafWebAclArn,omitempty"`
	LambdaAuthorizerConfig            *LambdaAuthorizerConfig  `json:"lambdaAuthorizerConfig,omitempty"`
	DNSMap                            map[string]string        `json:"dns,omitempty"`
	Visibility                        string                   `json:"visibility,omitempty"`
	APIType                           string                   `json:"apiType,omitempty"`
	MergedAPIExecutionRoleARN         string                   `json:"mergedApiExecutionRoleArn,omitempty"`
	Owner                             string                   `json:"owner,omitempty"`
	OwnerContact                      string                   `json:"ownerContact,omitempty"`
	IntrospectionConfig               string                   `json:"introspectionConfig,omitempty"`
	QueryDepthLimit                   int32                    `json:"queryDepthLimit,omitempty"`
	ResolverCountLimit                int32                    `json:"resolverCountLimit,omitempty"`
	EnhancedMetricsConfig             *EnhancedMetricsConfig   `json:"enhancedMetricsConfig,omitempty"`
}

// LogConfig represents logging configuration.
type LogConfig struct {
	FieldLogLevel         string `json:"fieldLogLevel,omitempty"`
	CloudWatchLogsRoleARN string `json:"cloudWatchLogsRoleArn,omitempty"`
	ExcludeVerboseContent bool   `json:"excludeVerboseContent,omitempty"`
}

// UserPoolConfig represents Cognito User Pool configuration.
type UserPoolConfig struct {
	UserPoolID       string `json:"userPoolId,omitempty"`
	AWSRegion        string `json:"awsRegion,omitempty"`
	DefaultAction    string `json:"defaultAction,omitempty"`
	AppIDClientRegex string `json:"appIdClientRegex,omitempty"`
}

// OpenIDConnectConfig represents OpenID Connect configuration.
type OpenIDConnectConfig struct {
	Issuer   string `json:"issuer,omitempty"`
	ClientID string `json:"clientId,omitempty"`
	IATTTL   int64  `json:"iatTTL,omitempty"`
	AuthTTL  int64  `json:"authTTL,omitempty"`
}

// AuthenticationProvider represents an additional authentication provider.
type AuthenticationProvider struct {
	AuthenticationType     string                  `json:"authenticationType,omitempty"`
	OpenIDConnectConfig    *OpenIDConnectConfig    `json:"openIDConnectConfig,omitempty"`
	UserPoolConfig         *UserPoolConfig         `json:"userPoolConfig,omitempty"`
	LambdaAuthorizerConfig *LambdaAuthorizerConfig `json:"lambdaAuthorizerConfig,omitempty"`
}

// LambdaAuthorizerConfig represents Lambda authorizer configuration.
type LambdaAuthorizerConfig struct {
	AuthorizerResultTTLInSeconds int32  `json:"authorizerResultTtlInSeconds,omitempty"`
	AuthorizerURI                string `json:"authorizerUri,omitempty"`
	IdentityValidationExpression string `json:"identityValidationExpression,omitempty"`
}

// EnhancedMetricsConfig represents enhanced metrics configuration.
type EnhancedMetricsConfig struct {
	ResolverLevelMetricsBehavior   string `json:"resolverLevelMetricsBehavior,omitempty"`
	DataSourceLevelMetricsBehavior string `json:"dataSourceLevelMetricsBehavior,omitempty"`
	OperationLevelMetricsConfig    string `json:"operationLevelMetricsConfig,omitempty"`
}

// DataSource represents an AppSync data source.
type DataSource struct {
	DataSourceARN            string                    `json:"dataSourceArn,omitempty"`
	Name                     string                    `json:"name,omitempty"`
	Description              string                    `json:"description,omitempty"`
	Type                     string                    `json:"type,omitempty"`
	ServiceRoleARN           string                    `json:"serviceRoleArn,omitempty"`
	DynamoDBConfig           *DynamoDBConfig           `json:"dynamodbConfig,omitempty"`
	LambdaConfig             *LambdaConfig             `json:"lambdaConfig,omitempty"`
	ElasticsearchConfig      *ElasticsearchConfig      `json:"elasticsearchConfig,omitempty"`
	OpenSearchServiceConfig  *OpenSearchServiceConfig  `json:"openSearchServiceConfig,omitempty"`
	HTTPConfig               *HTTPConfig               `json:"httpConfig,omitempty"`
	RelationalDatabaseConfig *RelationalDatabaseConfig `json:"relationalDatabaseConfig,omitempty"`
	EventBridgeConfig        *EventBridgeConfig        `json:"eventBridgeConfig,omitempty"`
	MetricsConfig            string                    `json:"metricsConfig,omitempty"`
}

// DynamoDBConfig represents DynamoDB data source configuration.
type DynamoDBConfig struct {
	TableName            string           `json:"tableName,omitempty"`
	AWSRegion            string           `json:"awsRegion,omitempty"`
	UseCallerCredentials bool             `json:"useCallerCredentials,omitempty"`
	DeltaSyncConfig      *DeltaSyncConfig `json:"deltaSyncConfig,omitempty"`
	Versioned            bool             `json:"versioned,omitempty"`
}

// DeltaSyncConfig represents delta sync configuration.
type DeltaSyncConfig struct {
	BaseTableTTL       int64  `json:"baseTableTTL,omitempty"`
	DeltaSyncTableName string `json:"deltaSyncTableName,omitempty"`
	DeltaSyncTableTTL  int64  `json:"deltaSyncTableTTL,omitempty"`
}

// LambdaConfig represents Lambda data source configuration.
type LambdaConfig struct {
	LambdaFunctionARN string `json:"lambdaFunctionArn,omitempty"`
}

// ElasticsearchConfig represents Elasticsearch data source configuration.
type ElasticsearchConfig struct {
	Endpoint  string `json:"endpoint,omitempty"`
	AWSRegion string `json:"awsRegion,omitempty"`
}

// OpenSearchServiceConfig represents OpenSearch data source configuration.
type OpenSearchServiceConfig struct {
	Endpoint  string `json:"endpoint,omitempty"`
	AWSRegion string `json:"awsRegion,omitempty"`
}

// HTTPConfig represents HTTP data source configuration.
type HTTPConfig struct {
	Endpoint            string               `json:"endpoint,omitempty"`
	AuthorizationConfig *AuthorizationConfig `json:"authorizationConfig,omitempty"`
}

// AuthorizationConfig represents authorization configuration for HTTP endpoints.
type AuthorizationConfig struct {
	AuthorizationType string        `json:"authorizationType,omitempty"`
	AWSIAMConfig      *AWSIAMConfig `json:"awsIamConfig,omitempty"`
}

// AWSIAMConfig represents AWS IAM configuration.
type AWSIAMConfig struct {
	SigningRegion      string `json:"signingRegion,omitempty"`
	SigningServiceName string `json:"signingServiceName,omitempty"`
}

// RelationalDatabaseConfig represents relational database configuration.
type RelationalDatabaseConfig struct {
	RelationalDatabaseSourceType string                 `json:"relationalDatabaseSourceType,omitempty"`
	RDSHTTPEndpointConfig        *RDSHTTPEndpointConfig `json:"rdsHttpEndpointConfig,omitempty"`
}

// RDSHTTPEndpointConfig represents RDS HTTP endpoint configuration.
type RDSHTTPEndpointConfig struct {
	AWSRegion           string `json:"awsRegion,omitempty"`
	DBClusterIdentifier string `json:"dbClusterIdentifier,omitempty"`
	DatabaseName        string `json:"databaseName,omitempty"`
	Schema              string `json:"schema,omitempty"`
	AWSSecretStoreARN   string `json:"awsSecretStoreArn,omitempty"`
}

// EventBridgeConfig represents EventBridge data source configuration.
type EventBridgeConfig struct {
	EventBusARN string `json:"eventBusArn,omitempty"`
}

// Resolver represents an AppSync resolver.
type Resolver struct {
	TypeName                string          `json:"typeName,omitempty"`
	FieldName               string          `json:"fieldName,omitempty"`
	DataSourceName          string          `json:"dataSourceName,omitempty"`
	ResolverARN             string          `json:"resolverArn,omitempty"`
	RequestMappingTemplate  string          `json:"requestMappingTemplate,omitempty"`
	ResponseMappingTemplate string          `json:"responseMappingTemplate,omitempty"`
	Kind                    string          `json:"kind,omitempty"`
	PipelineConfig          *PipelineConfig `json:"pipelineConfig,omitempty"`
	SyncConfig              *SyncConfig     `json:"syncConfig,omitempty"`
	CachingConfig           *CachingConfig  `json:"cachingConfig,omitempty"`
	MaxBatchSize            int32           `json:"maxBatchSize,omitempty"`
	Runtime                 *RuntimeConfig  `json:"runtime,omitempty"`
	Code                    string          `json:"code,omitempty"`
	MetricsConfig           string          `json:"metricsConfig,omitempty"`
}

// PipelineConfig represents pipeline resolver configuration.
type PipelineConfig struct {
	Functions []string `json:"functions,omitempty"`
}

// SyncConfig represents sync configuration for resolvers.
type SyncConfig struct {
	ConflictHandler             string                       `json:"conflictHandler,omitempty"`
	ConflictDetection           string                       `json:"conflictDetection,omitempty"`
	LambdaConflictHandlerConfig *LambdaConflictHandlerConfig `json:"lambdaConflictHandlerConfig,omitempty"`
}

// LambdaConflictHandlerConfig represents Lambda conflict handler configuration.
type LambdaConflictHandlerConfig struct {
	LambdaConflictHandlerARN string `json:"lambdaConflictHandlerArn,omitempty"`
}

// CachingConfig represents caching configuration.
type CachingConfig struct {
	TTL         int64    `json:"ttl,omitempty"`
	CachingKeys []string `json:"cachingKeys,omitempty"`
}

// RuntimeConfig represents resolver runtime configuration.
type RuntimeConfig struct {
	Name           string `json:"name,omitempty"`
	RuntimeVersion string `json:"runtimeVersion,omitempty"`
}

// SchemaCreationStatus represents schema creation status.
type SchemaCreationStatus struct {
	Status  string `json:"status,omitempty"`
	Details string `json:"details,omitempty"`
}

// CreateGraphqlAPIInput is the request for CreateGraphqlApi.
type CreateGraphqlAPIInput struct {
	Name                              string                   `json:"name"`
	AuthenticationType                string                   `json:"authenticationType"`
	LogConfig                         *LogConfig               `json:"logConfig,omitempty"`
	UserPoolConfig                    *UserPoolConfig          `json:"userPoolConfig,omitempty"`
	OpenIDConnectConfig               *OpenIDConnectConfig     `json:"openIDConnectConfig,omitempty"`
	Tags                              map[string]string        `json:"tags,omitempty"`
	AdditionalAuthenticationProviders []AuthenticationProvider `json:"additionalAuthenticationProviders,omitempty"`
	XrayEnabled                       bool                     `json:"xrayEnabled,omitempty"`
	LambdaAuthorizerConfig            *LambdaAuthorizerConfig  `json:"lambdaAuthorizerConfig,omitempty"`
	Visibility                        string                   `json:"visibility,omitempty"`
	APIType                           string                   `json:"apiType,omitempty"`
	MergedAPIExecutionRoleARN         string                   `json:"mergedApiExecutionRoleArn,omitempty"`
	OwnerContact                      string                   `json:"ownerContact,omitempty"`
	IntrospectionConfig               string                   `json:"introspectionConfig,omitempty"`
	QueryDepthLimit                   int32                    `json:"queryDepthLimit,omitempty"`
	ResolverCountLimit                int32                    `json:"resolverCountLimit,omitempty"`
	EnhancedMetricsConfig             *EnhancedMetricsConfig   `json:"enhancedMetricsConfig,omitempty"`
}

// CreateGraphqlAPIOutput is the response for CreateGraphqlApi.
type CreateGraphqlAPIOutput struct {
	GraphqlAPI *GraphqlAPI `json:"graphqlApi,omitempty"`
}

// DeleteGraphqlAPIInput is the request for DeleteGraphqlApi.
type DeleteGraphqlAPIInput struct {
	APIID string `json:"apiId"`
}

// GetGraphqlAPIInput is the request for GetGraphqlApi.
type GetGraphqlAPIInput struct {
	APIID string `json:"apiId"`
}

// GetGraphqlAPIOutput is the response for GetGraphqlApi.
type GetGraphqlAPIOutput struct {
	GraphqlAPI *GraphqlAPI `json:"graphqlApi,omitempty"`
}

// ListGraphqlAPIsInput is the request for ListGraphqlApis.
type ListGraphqlAPIsInput struct {
	NextToken  string `json:"nextToken,omitempty"`
	MaxResults int32  `json:"maxResults,omitempty"`
	APIType    string `json:"apiType,omitempty"`
	Owner      string `json:"owner,omitempty"`
}

// ListGraphqlAPIsOutput is the response for ListGraphqlApis.
type ListGraphqlAPIsOutput struct {
	GraphqlAPIs []GraphqlAPI `json:"graphqlApis,omitempty"`
	NextToken   string       `json:"nextToken,omitempty"`
}

// CreateDataSourceInput is the request for CreateDataSource.
type CreateDataSourceInput struct {
	APIID                    string                    `json:"apiId"`
	Name                     string                    `json:"name"`
	Description              string                    `json:"description,omitempty"`
	Type                     string                    `json:"type"`
	ServiceRoleARN           string                    `json:"serviceRoleArn,omitempty"`
	DynamoDBConfig           *DynamoDBConfig           `json:"dynamodbConfig,omitempty"`
	LambdaConfig             *LambdaConfig             `json:"lambdaConfig,omitempty"`
	ElasticsearchConfig      *ElasticsearchConfig      `json:"elasticsearchConfig,omitempty"`
	OpenSearchServiceConfig  *OpenSearchServiceConfig  `json:"openSearchServiceConfig,omitempty"`
	HTTPConfig               *HTTPConfig               `json:"httpConfig,omitempty"`
	RelationalDatabaseConfig *RelationalDatabaseConfig `json:"relationalDatabaseConfig,omitempty"`
	EventBridgeConfig        *EventBridgeConfig        `json:"eventBridgeConfig,omitempty"`
	MetricsConfig            string                    `json:"metricsConfig,omitempty"`
}

// CreateDataSourceOutput is the response for CreateDataSource.
type CreateDataSourceOutput struct {
	DataSource *DataSource `json:"dataSource,omitempty"`
}

// CreateResolverInput is the request for CreateResolver.
type CreateResolverInput struct {
	APIID                   string          `json:"apiId"`
	TypeName                string          `json:"typeName"`
	FieldName               string          `json:"fieldName"`
	DataSourceName          string          `json:"dataSourceName,omitempty"`
	RequestMappingTemplate  string          `json:"requestMappingTemplate,omitempty"`
	ResponseMappingTemplate string          `json:"responseMappingTemplate,omitempty"`
	Kind                    string          `json:"kind,omitempty"`
	PipelineConfig          *PipelineConfig `json:"pipelineConfig,omitempty"`
	SyncConfig              *SyncConfig     `json:"syncConfig,omitempty"`
	CachingConfig           *CachingConfig  `json:"cachingConfig,omitempty"`
	MaxBatchSize            int32           `json:"maxBatchSize,omitempty"`
	Runtime                 *RuntimeConfig  `json:"runtime,omitempty"`
	Code                    string          `json:"code,omitempty"`
	MetricsConfig           string          `json:"metricsConfig,omitempty"`
}

// CreateResolverOutput is the response for CreateResolver.
type CreateResolverOutput struct {
	Resolver *Resolver `json:"resolver,omitempty"`
}

// StartSchemaCreationInput is the request for StartSchemaCreation.
type StartSchemaCreationInput struct {
	APIID      string `json:"apiId"`
	Definition []byte `json:"definition"`
}

// StartSchemaCreationOutput is the response for StartSchemaCreation.
type StartSchemaCreationOutput struct {
	Status string `json:"status,omitempty"`
}

// ErrorResponse represents an AppSync error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error represents an AppSync error.
type Error = service.CodedError
