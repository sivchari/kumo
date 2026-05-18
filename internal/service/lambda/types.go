// Package lambda provides Lambda service emulation for kumo.
package lambda

import (
	"time"
)

// Function represents a Lambda function.
type Function struct {
	FunctionName    string
	FunctionArn     string
	Runtime         string
	Role            string
	Handler         string
	Description     string
	Timeout         int
	MemorySize      int
	CodeSize        int64
	CodeSha256      string
	Version         string
	LastModified    time.Time
	State           string
	StateReason     string
	StateReasonCode string
	PackageType     string
	Architectures   []string
	Environment     *Environment
	Code            *FunctionCode
	Tags            map[string]string
	Policy          *ResourcePolicy
	InvokeEndpoint  string // kumo extension: HTTP endpoint to proxy invocations
}

// ResourcePolicy represents a Lambda function resource policy.
type ResourcePolicy struct {
	Version    string             `json:"Version"`
	ID         string             `json:"Id"`
	Statements []*PolicyStatement `json:"Statement"`
}

// PolicyStatement represents a single statement in a resource policy.
type PolicyStatement struct {
	Sid       string            `json:"Sid"`
	Effect    string            `json:"Effect"`
	Principal map[string]string `json:"Principal"`
	Action    string            `json:"Action"`
	Resource  string            `json:"Resource"`
	Condition map[string]any    `json:"Condition,omitempty"`
}

// Environment represents the function's environment variables.
type Environment struct {
	Variables map[string]string `json:"Variables,omitempty"`
}

// FunctionCode represents the function's code.
type FunctionCode struct {
	ZipFile         []byte `json:"ZipFile,omitempty"`
	S3Bucket        string `json:"S3Bucket,omitempty"`
	S3Key           string `json:"S3Key,omitempty"`
	S3ObjectVersion string `json:"S3ObjectVersion,omitempty"`
	ImageURI        string `json:"ImageURI,omitempty"`
}

// CreateFunctionRequest is the request for CreateFunction.
type CreateFunctionRequest struct {
	FunctionName   string            `json:"FunctionName"`
	Runtime        string            `json:"Runtime,omitempty"`
	Role           string            `json:"Role"`
	Handler        string            `json:"Handler,omitempty"`
	Code           FunctionCode      `json:"Code"`
	Description    string            `json:"Description,omitempty"`
	Timeout        int               `json:"Timeout,omitempty"`
	MemorySize     int               `json:"MemorySize,omitempty"`
	Publish        bool              `json:"Publish,omitempty"`
	PackageType    string            `json:"PackageType,omitempty"`
	Architectures  []string          `json:"Architectures,omitempty"`
	Environment    *Environment      `json:"Environment,omitempty"`
	Tags           map[string]string `json:"Tags,omitempty"`
	InvokeEndpoint string            `json:"InvokeEndpoint,omitempty"` // kumo extension
}

// CreateFunctionResponse is the response for CreateFunction.
type CreateFunctionResponse struct {
	FunctionName    string       `json:"FunctionName"`
	FunctionArn     string       `json:"FunctionArn"`
	Runtime         string       `json:"Runtime,omitempty"`
	Role            string       `json:"Role"`
	Handler         string       `json:"Handler,omitempty"`
	CodeSize        int64        `json:"CodeSize"`
	Description     string       `json:"Description,omitempty"`
	Timeout         int          `json:"Timeout"`
	MemorySize      int          `json:"MemorySize"`
	LastModified    string       `json:"LastModified"`
	CodeSha256      string       `json:"CodeSha256"`
	Version         string       `json:"Version"`
	State           string       `json:"State,omitempty"`
	StateReason     string       `json:"StateReason,omitempty"`
	StateReasonCode string       `json:"StateReasonCode,omitempty"`
	PackageType     string       `json:"PackageType,omitempty"`
	Architectures   []string     `json:"Architectures,omitempty"`
	Environment     *Environment `json:"Environment,omitempty"`
}

// GetFunctionResponse is the response for GetFunction.
type GetFunctionResponse struct {
	Configuration *FunctionConfiguration `json:"Configuration"`
	Code          *FunctionCodeLocation  `json:"Code,omitempty"`
	Tags          map[string]string      `json:"Tags,omitempty"`
}

// FunctionConfiguration contains function configuration details.
type FunctionConfiguration struct {
	FunctionName    string       `json:"FunctionName"`
	FunctionArn     string       `json:"FunctionArn"`
	Runtime         string       `json:"Runtime,omitempty"`
	Role            string       `json:"Role"`
	Handler         string       `json:"Handler,omitempty"`
	CodeSize        int64        `json:"CodeSize"`
	Description     string       `json:"Description,omitempty"`
	Timeout         int          `json:"Timeout"`
	MemorySize      int          `json:"MemorySize"`
	LastModified    string       `json:"LastModified"`
	CodeSha256      string       `json:"CodeSha256"`
	Version         string       `json:"Version"`
	State           string       `json:"State,omitempty"`
	StateReason     string       `json:"StateReason,omitempty"`
	StateReasonCode string       `json:"StateReasonCode,omitempty"`
	PackageType     string       `json:"PackageType,omitempty"`
	Architectures   []string     `json:"Architectures,omitempty"`
	Environment     *Environment `json:"Environment,omitempty"`
}

// FunctionCodeLocation contains the location of the function code.
type FunctionCodeLocation struct {
	RepositoryType string `json:"RepositoryType,omitempty"`
	Location       string `json:"Location,omitempty"`
}

// ListFunctionsResponse is the response for ListFunctions.
type ListFunctionsResponse struct {
	Functions  []*FunctionConfiguration `json:"Functions"`
	NextMarker string                   `json:"NextMarker,omitempty"`
}

// InvokeRequest is the request for Invoke.
type InvokeRequest struct {
	Payload        []byte `json:"-"`
	InvocationType string `json:"-"`
	LogType        string `json:"-"`
	ClientContext  string `json:"-"`
	Qualifier      string `json:"-"`
}

// InvokeResponse is the response for Invoke.
type InvokeResponse struct {
	StatusCode      int    `json:"-"`
	FunctionError   string `json:"-"`
	LogResult       string `json:"-"`
	Payload         []byte `json:"-"`
	ExecutedVersion string `json:"-"`
}

// UpdateFunctionCodeRequest is the request for UpdateFunctionCode.
type UpdateFunctionCodeRequest struct {
	ZipFile         []byte   `json:"ZipFile,omitempty"`
	S3Bucket        string   `json:"S3Bucket,omitempty"`
	S3Key           string   `json:"S3Key,omitempty"`
	S3ObjectVersion string   `json:"S3ObjectVersion,omitempty"`
	ImageURI        string   `json:"ImageURI,omitempty"`
	Publish         bool     `json:"Publish,omitempty"`
	Architectures   []string `json:"Architectures,omitempty"`
}

// UpdateFunctionConfigurationRequest is the request for UpdateFunctionConfiguration.
type UpdateFunctionConfigurationRequest struct {
	Description    string       `json:"Description,omitempty"`
	Handler        string       `json:"Handler,omitempty"`
	MemorySize     int          `json:"MemorySize,omitempty"`
	Role           string       `json:"Role,omitempty"`
	Runtime        string       `json:"Runtime,omitempty"`
	Timeout        int          `json:"Timeout,omitempty"`
	Environment    *Environment `json:"Environment,omitempty"`
	InvokeEndpoint string       `json:"InvokeEndpoint,omitempty"` // kumo extension
}

// FunctionError represents a Lambda error.
type FunctionError struct {
	Type    string `json:"Type"`
	Message string `json:"Message"`
}

// Error implements the error interface.
func (e *FunctionError) Error() string {
	return e.Message
}

// Error codes for Lambda.
const (
	ErrResourceNotFound      = "ResourceNotFoundException"
	ErrResourceConflict      = "ResourceConflictException"
	ErrInvalidParameterValue = "InvalidParameterValueException"
	ErrServiceException      = "ServiceException"
)

// EventSourceMapping represents a Lambda event source mapping.
type EventSourceMapping struct {
	UUID                           string  `json:"UUID"`
	FunctionArn                    string  `json:"FunctionArn"`
	EventSourceArn                 string  `json:"EventSourceArn,omitempty"`
	State                          string  `json:"State"`
	StateTransitionReason          string  `json:"StateTransitionReason,omitempty"`
	BatchSize                      int     `json:"BatchSize,omitempty"`
	MaximumBatchingWindowInSeconds int     `json:"MaximumBatchingWindowInSeconds,omitempty"`
	Enabled                        *bool   `json:"Enabled,omitempty"`
	LastModified                   float64 `json:"LastModified,omitempty"`
	LastProcessingResult           string  `json:"LastProcessingResult,omitempty"`
}

// CreateEventSourceMappingRequest is the request for CreateEventSourceMapping.
type CreateEventSourceMappingRequest struct {
	FunctionName                   string `json:"FunctionName"`
	EventSourceArn                 string `json:"EventSourceArn,omitempty"`
	BatchSize                      int    `json:"BatchSize,omitempty"`
	MaximumBatchingWindowInSeconds int    `json:"MaximumBatchingWindowInSeconds,omitempty"`
	Enabled                        *bool  `json:"Enabled,omitempty"`
}

// UpdateEventSourceMappingRequest is the request for UpdateEventSourceMapping.
type UpdateEventSourceMappingRequest struct {
	FunctionName                   string `json:"FunctionName,omitempty"`
	BatchSize                      int    `json:"BatchSize,omitempty"`
	MaximumBatchingWindowInSeconds int    `json:"MaximumBatchingWindowInSeconds,omitempty"`
	Enabled                        *bool  `json:"Enabled,omitempty"`
}

// ListEventSourceMappingsResponse is the response for ListEventSourceMappings.
type ListEventSourceMappingsResponse struct {
	EventSourceMappings []*EventSourceMapping `json:"EventSourceMappings"`
	NextMarker          string                `json:"NextMarker,omitempty"`
}

// listVersionsByFunctionResponse is the wire shape of ListVersionsByFunction.
type listVersionsByFunctionResponse struct {
	Versions   []functionConfigurationVersion `json:"Versions"`
	NextMarker string                         `json:"NextMarker,omitempty"`
}

// functionConfigurationVersion is the per-version function config block
// terraform-provider-aws reads on every refresh.
type functionConfigurationVersion struct {
	FunctionName string `json:"FunctionName"`
	FunctionArn  string `json:"FunctionArn"`
	Runtime      string `json:"Runtime,omitempty"`
	Role         string `json:"Role,omitempty"`
	Handler      string `json:"Handler,omitempty"`
	Version      string `json:"Version"`
	LastModified string `json:"LastModified,omitempty"`
}

// listAliasesResponse is the wire shape of ListAliases.
type listAliasesResponse struct {
	Aliases []aliasConfiguration `json:"Aliases"`
}

// aliasConfiguration mirrors AWS's AliasConfiguration; tags omitted because
// no aliases are ever produced by this stub.
type aliasConfiguration struct {
	Name            string `json:"Name"`
	AliasArn        string `json:"AliasArn"`
	FunctionVersion string `json:"FunctionVersion"`
}

// getFunctionCodeSigningConfigResponse mirrors AWS's response.
type getFunctionCodeSigningConfigResponse struct {
	FunctionName         string `json:"FunctionName"`
	CodeSigningConfigArn string `json:"CodeSigningConfigArn,omitempty"`
}

// listFunctionEventInvokeConfigsResponse mirrors AWS's response.
type listFunctionEventInvokeConfigsResponse struct {
	FunctionEventInvokeConfigs []map[string]any `json:"FunctionEventInvokeConfigs"`
}

// addPermissionRequest is the request body for AddPermission.
type addPermissionRequest struct {
	Action              string `json:"Action"`
	FunctionName        string `json:"FunctionName,omitempty"`
	Principal           string `json:"Principal"`
	SourceArn           string `json:"SourceArn,omitempty"`
	SourceAccount       string `json:"SourceAccount,omitempty"`
	StatementID         string `json:"StatementId"`
	EventSourceToken    string `json:"EventSourceToken,omitempty"`
	RevisionID          string `json:"RevisionId,omitempty"`
	PrincipalOrgID      string `json:"PrincipalOrgID,omitempty"`
	FunctionURLAuthType string `json:"FunctionUrlAuthType,omitempty"`
}

// addPermissionResponse is the response for AddPermission.
type addPermissionResponse struct {
	Statement string `json:"Statement"`
}

// getPolicyResponse is the response for GetPolicy when a policy exists.
type getPolicyResponse struct {
	Policy     string `json:"Policy"`
	RevisionID string `json:"RevisionId"`
}

// listTagsResponse is the response for ListTags.
type listTagsResponse struct {
	Tags map[string]string `json:"Tags"`
}

// tagResourceRequest is the request body for TagResource.
type tagResourceRequest struct {
	Tags map[string]string `json:"Tags"`
}
