package athena

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// QueryExecutionState represents the state of a query execution.
type QueryExecutionState string

// Query execution states.
const (
	QueryExecutionStateQueued    QueryExecutionState = "QUEUED"
	QueryExecutionStateRunning   QueryExecutionState = "RUNNING"
	QueryExecutionStateSucceeded QueryExecutionState = "SUCCEEDED"
	QueryExecutionStateFailed    QueryExecutionState = "FAILED"
	QueryExecutionStateCancelled QueryExecutionState = "CANCELLED"
)

// WorkGroupState represents the state of a workgroup.
type WorkGroupState string

// Workgroup states.
const (
	WorkGroupStateEnabled  WorkGroupState = "ENABLED"
	WorkGroupStateDisabled WorkGroupState = "DISABLED"
)

// QueryExecution represents a query execution.
type QueryExecution struct {
	QueryExecutionID      string
	Query                 string
	StatementType         string
	ResultConfiguration   *ResultConfiguration
	QueryExecutionContext *QueryExecutionContext
	Status                *QueryExecutionStatus
	Statistics            *QueryExecutionStatistics
	WorkGroup             string
	EngineVersion         *EngineVersion
	ExecutionParameters   []string
	SubstatementType      string
}

// ResultConfiguration represents the result configuration.
type ResultConfiguration struct {
	OutputLocation          string
	EncryptionConfiguration *EncryptionConfiguration
	ExpectedBucketOwner     string
	ACLConfiguration        *ACLConfiguration
}

// EncryptionConfiguration represents encryption configuration.
type EncryptionConfiguration struct {
	EncryptionOption string
	KmsKey           string
}

// ACLConfiguration represents ACL configuration.
type ACLConfiguration struct {
	S3AclOption string
}

// QueryExecutionContext represents the query execution context.
type QueryExecutionContext struct {
	Database string
	Catalog  string
}

// QueryExecutionStatus represents the status of a query execution.
type QueryExecutionStatus struct {
	State                  QueryExecutionState
	StateChangeReason      string
	SubmissionDateTime     time.Time
	CompletionDateTime     *time.Time
	QueryError             *QueryError
	PublishedKmsKey        string
	SourceOperationArn     string
	ReservedCapacityStatus *ReservedCapacityStatus
}

// QueryError represents an Athena query error.
type QueryError struct {
	ErrorCategory int32
	ErrorType     int32
	Retryable     bool
	ErrorMessage  string
}

// ReservedCapacityStatus represents reserved capacity status.
type ReservedCapacityStatus struct {
	UsedReservationID string
}

// QueryExecutionStatistics represents statistics about query execution.
type QueryExecutionStatistics struct {
	EngineExecutionTimeInMillis      int64
	DataScannedInBytes               int64
	DataManifestLocation             string
	TotalExecutionTimeInMillis       int64
	QueryQueueTimeInMillis           int64
	ServicePreProcessingTimeInMillis int64
	QueryPlanningTimeInMillis        int64
	ServiceProcessingTimeInMillis    int64
	ResultReuseInformation           *ResultReuseInformation
}

// ResultReuseInformation represents result reuse information.
type ResultReuseInformation struct {
	ReusedPreviousResult bool
}

// EngineVersion represents the engine version.
type EngineVersion struct {
	SelectedEngineVersion  string
	EffectiveEngineVersion string
}

// WorkGroup represents a workgroup.
type WorkGroup struct {
	Name                         string
	State                        WorkGroupState
	Configuration                *WorkGroupConfiguration
	Description                  string
	CreationTime                 time.Time
	IdentityCenterApplicationArn string
}

// WorkGroupConfiguration represents workgroup configuration.
type WorkGroupConfiguration struct {
	ResultConfiguration                     *ResultConfiguration
	EnforceWorkGroupConfiguration           bool
	PublishCloudWatchMetricsEnabled         bool
	BytesScannedCutoffPerQuery              int64
	RequesterPaysEnabled                    bool
	EngineVersion                           *EngineVersion
	AdditionalConfiguration                 string
	ExecutionRole                           string
	CustomerContentEncryptionConfiguration  *CustomerContentEncryptionConfiguration
	EnableMinimumEncryptionConfiguration    bool
	IdentityCenterConfiguration             *IdentityCenterConfiguration
	QueryResultsS3AccessGrantsConfiguration *QueryResultsS3AccessGrantsConfiguration
}

// CustomerContentEncryptionConfiguration represents customer content encryption configuration.
type CustomerContentEncryptionConfiguration struct {
	KmsKey string
}

// IdentityCenterConfiguration represents identity center configuration.
type IdentityCenterConfiguration struct {
	EnableIdentityCenter      bool
	IdentityCenterInstanceArn string
}

// QueryResultsS3AccessGrantsConfiguration represents S3 access grants configuration.
type QueryResultsS3AccessGrantsConfiguration struct {
	EnableS3AccessGrants  bool
	CreateUserLevelPrefix bool
	AuthenticationType    string
}

// ResultSet represents a result set.
type ResultSet struct {
	Rows              []Row
	ResultSetMetadata *ResultSetMetadata
}

// Row represents a row in the result set.
type Row struct {
	Data []Datum
}

// Datum represents a single data value.
type Datum struct {
	VarCharValue string
}

// ResultSetMetadata represents result set metadata.
type ResultSetMetadata struct {
	ColumnInfo []ColumnInfo
}

// ColumnInfo represents column information.
type ColumnInfo struct {
	CatalogName   string
	SchemaName    string
	TableName     string
	Name          string
	Label         string
	Type          string
	Precision     int32
	Scale         int32
	Nullable      string
	CaseSensitive bool
}

// StartQueryExecutionRequest is the request for StartQueryExecution.
type StartQueryExecutionRequest struct {
	QueryString              string                    `json:"QueryString"`
	ClientRequestToken       string                    `json:"ClientRequestToken,omitempty"`
	QueryExecutionContext    *QueryExecutionContext    `json:"QueryExecutionContext,omitempty"`
	ResultConfiguration      *ResultConfiguration      `json:"ResultConfiguration,omitempty"`
	WorkGroup                string                    `json:"WorkGroup,omitempty"`
	ExecutionParameters      []string                  `json:"ExecutionParameters,omitempty"`
	ResultReuseConfiguration *ResultReuseConfiguration `json:"ResultReuseConfiguration,omitempty"`
}

// ResultReuseConfiguration represents result reuse configuration.
type ResultReuseConfiguration struct {
	ResultReuseByAgeConfiguration *ResultReuseByAgeConfiguration `json:"ResultReuseByAgeConfiguration,omitempty"`
}

// ResultReuseByAgeConfiguration represents result reuse by age configuration.
type ResultReuseByAgeConfiguration struct {
	Enabled         bool  `json:"Enabled"`
	MaxAgeInMinutes int32 `json:"MaxAgeInMinutes,omitempty"`
}

// StartQueryExecutionResponse is the response for StartQueryExecution.
type StartQueryExecutionResponse struct {
	QueryExecutionID string `json:"QueryExecutionId"`
}

// StopQueryExecutionRequest is the request for StopQueryExecution.
type StopQueryExecutionRequest struct {
	QueryExecutionID string `json:"QueryExecutionId"`
}

// StopQueryExecutionResponse is the response for StopQueryExecution.
type StopQueryExecutionResponse struct{}

// GetQueryExecutionRequest is the request for GetQueryExecution.
type GetQueryExecutionRequest struct {
	QueryExecutionID string `json:"QueryExecutionId"`
}

// GetQueryExecutionResponse is the response for GetQueryExecution.
type GetQueryExecutionResponse struct {
	QueryExecution *QueryExecutionOutput `json:"QueryExecution"`
}

// QueryExecutionOutput represents query execution in API response.
type QueryExecutionOutput struct {
	QueryExecutionID      string                          `json:"QueryExecutionId"`
	Query                 string                          `json:"Query"`
	StatementType         string                          `json:"StatementType,omitempty"`
	ResultConfiguration   *ResultConfigurationOutput      `json:"ResultConfiguration,omitempty"`
	QueryExecutionContext *QueryExecutionContextOutput    `json:"QueryExecutionContext,omitempty"`
	Status                *QueryExecutionStatusOutput     `json:"Status"`
	Statistics            *QueryExecutionStatisticsOutput `json:"Statistics,omitempty"`
	WorkGroup             string                          `json:"WorkGroup,omitempty"`
	EngineVersion         *EngineVersionOutput            `json:"EngineVersion,omitempty"`
	ExecutionParameters   []string                        `json:"ExecutionParameters,omitempty"`
	SubstatementType      string                          `json:"SubstatementType,omitempty"`
}

// ResultConfigurationOutput represents result configuration in API response.
type ResultConfigurationOutput struct {
	OutputLocation          string                         `json:"OutputLocation,omitempty"`
	EncryptionConfiguration *EncryptionConfigurationOutput `json:"EncryptionConfiguration,omitempty"`
	ExpectedBucketOwner     string                         `json:"ExpectedBucketOwner,omitempty"`
	ACLConfiguration        *ACLConfigurationOutput        `json:"AclConfiguration,omitempty"`
}

// EncryptionConfigurationOutput represents encryption configuration in API response.
type EncryptionConfigurationOutput struct {
	EncryptionOption string `json:"EncryptionOption"`
	KmsKey           string `json:"KmsKey,omitempty"`
}

// ACLConfigurationOutput represents ACL configuration in API response.
type ACLConfigurationOutput struct {
	S3AclOption string `json:"S3AclOption"`
}

// QueryExecutionContextOutput represents query execution context in API response.
type QueryExecutionContextOutput struct {
	Database string `json:"Database,omitempty"`
	Catalog  string `json:"Catalog,omitempty"`
}

// QueryExecutionStatusOutput represents query execution status in API response.
type QueryExecutionStatusOutput struct {
	State              string   `json:"State"`
	StateChangeReason  string   `json:"StateChangeReason,omitempty"`
	SubmissionDateTime float64  `json:"SubmissionDateTime"`
	CompletionDateTime *float64 `json:"CompletionDateTime,omitempty"`
}

// QueryExecutionStatisticsOutput represents query execution statistics in API response.
type QueryExecutionStatisticsOutput struct {
	EngineExecutionTimeInMillis      int64  `json:"EngineExecutionTimeInMillis,omitempty"`
	DataScannedInBytes               int64  `json:"DataScannedInBytes,omitempty"`
	DataManifestLocation             string `json:"DataManifestLocation,omitempty"`
	TotalExecutionTimeInMillis       int64  `json:"TotalExecutionTimeInMillis,omitempty"`
	QueryQueueTimeInMillis           int64  `json:"QueryQueueTimeInMillis,omitempty"`
	ServicePreProcessingTimeInMillis int64  `json:"ServicePreProcessingTimeInMillis,omitempty"`
	QueryPlanningTimeInMillis        int64  `json:"QueryPlanningTimeInMillis,omitempty"`
	ServiceProcessingTimeInMillis    int64  `json:"ServiceProcessingTimeInMillis,omitempty"`
}

// EngineVersionOutput represents engine version in API response.
type EngineVersionOutput struct {
	SelectedEngineVersion  string `json:"SelectedEngineVersion,omitempty"`
	EffectiveEngineVersion string `json:"EffectiveEngineVersion,omitempty"`
}

// GetQueryResultsRequest is the request for GetQueryResults.
type GetQueryResultsRequest struct {
	QueryExecutionID string `json:"QueryExecutionId"`
	NextToken        string `json:"NextToken,omitempty"`
	MaxResults       int32  `json:"MaxResults,omitempty"`
}

// GetQueryResultsResponse is the response for GetQueryResults.
type GetQueryResultsResponse struct {
	UpdateCount int64            `json:"UpdateCount,omitempty"`
	ResultSet   *ResultSetOutput `json:"ResultSet,omitempty"`
	NextToken   string           `json:"NextToken,omitempty"`
}

// ResultSetOutput represents result set in API response.
type ResultSetOutput struct {
	Rows              []RowOutput              `json:"Rows,omitempty"`
	ResultSetMetadata *ResultSetMetadataOutput `json:"ResultSetMetadata,omitempty"`
}

// RowOutput represents a row in API response.
type RowOutput struct {
	Data []DatumOutput `json:"Data,omitempty"`
}

// DatumOutput represents a datum in API response.
type DatumOutput struct {
	VarCharValue string `json:"VarCharValue,omitempty"`
}

// ResultSetMetadataOutput represents result set metadata in API response.
type ResultSetMetadataOutput struct {
	ColumnInfo []ColumnInfoOutput `json:"ColumnInfo,omitempty"`
}

// ColumnInfoOutput represents column info in API response.
type ColumnInfoOutput struct {
	CatalogName   string `json:"CatalogName,omitempty"`
	SchemaName    string `json:"SchemaName,omitempty"`
	TableName     string `json:"TableName,omitempty"`
	Name          string `json:"Name"`
	Label         string `json:"Label,omitempty"`
	Type          string `json:"Type"`
	Precision     int32  `json:"Precision,omitempty"`
	Scale         int32  `json:"Scale,omitempty"`
	Nullable      string `json:"Nullable,omitempty"`
	CaseSensitive bool   `json:"CaseSensitive,omitempty"`
}

// ListQueryExecutionsRequest is the request for ListQueryExecutions.
type ListQueryExecutionsRequest struct {
	NextToken  string `json:"NextToken,omitempty"`
	MaxResults int32  `json:"MaxResults,omitempty"`
	WorkGroup  string `json:"WorkGroup,omitempty"`
}

// ListQueryExecutionsResponse is the response for ListQueryExecutions.
type ListQueryExecutionsResponse struct {
	QueryExecutionIDs []string `json:"QueryExecutionIds,omitempty"`
	NextToken         string   `json:"NextToken,omitempty"`
}

// CreateWorkGroupRequest is the request for CreateWorkGroup.
type CreateWorkGroupRequest struct {
	Name          string                  `json:"Name"`
	Configuration *WorkGroupConfiguration `json:"Configuration,omitempty"`
	Description   string                  `json:"Description,omitempty"`
	Tags          []Tag                   `json:"Tags,omitempty"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// CreateWorkGroupResponse is the response for CreateWorkGroup.
type CreateWorkGroupResponse struct{}

// DeleteWorkGroupRequest is the request for DeleteWorkGroup.
type DeleteWorkGroupRequest struct {
	WorkGroup             string `json:"WorkGroup"`
	RecursiveDeleteOption bool   `json:"RecursiveDeleteOption,omitempty"`
}

// DeleteWorkGroupResponse is the response for DeleteWorkGroup.
type DeleteWorkGroupResponse struct{}

// ErrorResponse represents an Athena error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError represents an Athena service error.
type ServiceError = service.CodedError
