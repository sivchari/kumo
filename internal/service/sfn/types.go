package sfn

import "time"

// StateMachineStatus represents the status of a state machine.
type StateMachineStatus string

// State machine status constants.
const (
	StateMachineStatusActive   StateMachineStatus = "ACTIVE"
	StateMachineStatusDeleting StateMachineStatus = "DELETING"
)

// StateMachineType represents the type of a state machine.
type StateMachineType string

// State machine type constants.
const (
	StateMachineTypeStandard StateMachineType = "STANDARD"
	StateMachineTypeExpress  StateMachineType = "EXPRESS"
)

// ExecutionStatus represents the status of an execution.
type ExecutionStatus string

// Execution status constants.
const (
	ExecutionStatusRunning        ExecutionStatus = "RUNNING"
	ExecutionStatusSucceeded      ExecutionStatus = "SUCCEEDED"
	ExecutionStatusFailed         ExecutionStatus = "FAILED"
	ExecutionStatusTimedOut       ExecutionStatus = "TIMED_OUT"
	ExecutionStatusAborted        ExecutionStatus = "ABORTED"
	ExecutionStatusPendingRedrive ExecutionStatus = "PENDING_REDRIVE"
)

// HistoryEventType represents the type of a history event.
type HistoryEventType string

// History event type constants.
const (
	HistoryEventTypeExecutionStarted   HistoryEventType = "ExecutionStarted"
	HistoryEventTypeExecutionSucceeded HistoryEventType = "ExecutionSucceeded"
	HistoryEventTypeExecutionFailed    HistoryEventType = "ExecutionFailed"
	HistoryEventTypeExecutionAborted   HistoryEventType = "ExecutionAborted"
	HistoryEventTypeExecutionTimedOut  HistoryEventType = "ExecutionTimedOut"
	HistoryEventTypeTaskStateEntered   HistoryEventType = "TaskStateEntered"
	HistoryEventTypeTaskStateExited    HistoryEventType = "TaskStateExited"
)

// StateMachine represents a Step Functions state machine.
type StateMachine struct {
	StateMachineArn      string
	Name                 string
	Definition           string
	RoleArn              string
	Type                 StateMachineType
	Status               StateMachineStatus
	CreationDate         time.Time
	Description          string
	LoggingConfiguration *LoggingConfiguration
	TracingConfiguration *TracingConfiguration
	Label                string
	RevisionID           string
}

// LoggingConfiguration represents the logging configuration.
type LoggingConfiguration struct {
	Level                string           `json:"level,omitempty"`
	IncludeExecutionData bool             `json:"includeExecutionData,omitempty"`
	Destinations         []LogDestination `json:"destinations,omitempty"`
}

// LogDestination represents a log destination.
type LogDestination struct {
	CloudWatchLogsLogGroup *CloudWatchLogsLogGroup `json:"cloudWatchLogsLogGroup,omitempty"`
}

// CloudWatchLogsLogGroup represents a CloudWatch Logs log group.
type CloudWatchLogsLogGroup struct {
	LogGroupArn string `json:"logGroupArn,omitempty"`
}

// TracingConfiguration represents the tracing configuration.
type TracingConfiguration struct {
	Enabled bool `json:"enabled,omitempty"`
}

// Execution represents a Step Functions execution.
type Execution struct {
	ExecutionArn        string
	StateMachineArn     string
	Name                string
	Status              ExecutionStatus
	StartDate           time.Time
	StopDate            *time.Time
	Input               string
	InputDetails        *CloudWatchEventsExecutionDataDetails
	Output              string
	OutputDetails       *CloudWatchEventsExecutionDataDetails
	Error               string
	Cause               string
	TraceHeader         string
	RedriveCount        int32
	RedriveDate         *time.Time
	RedriveStatus       string
	RedriveStatusReason string
}

// CloudWatchEventsExecutionDataDetails contains details about execution data.
type CloudWatchEventsExecutionDataDetails struct {
	Included bool `json:"included,omitempty"`
}

// HistoryEvent represents a history event in an execution.
type HistoryEvent struct {
	Timestamp                      time.Time
	Type                           HistoryEventType
	ID                             int64
	PreviousEventID                int64
	ExecutionStartedEventDetails   *ExecutionStartedEventDetails
	ExecutionSucceededEventDetails *ExecutionSucceededEventDetails
	ExecutionFailedEventDetails    *ExecutionFailedEventDetails
	ExecutionAbortedEventDetails   *ExecutionAbortedEventDetails
	ExecutionTimedOutEventDetails  *ExecutionTimedOutEventDetails
}

// ExecutionStartedEventDetails contains details about an ExecutionStarted event.
type ExecutionStartedEventDetails struct {
	Input        string                                `json:"input,omitempty"`
	InputDetails *CloudWatchEventsExecutionDataDetails `json:"inputDetails,omitempty"`
	RoleArn      string                                `json:"roleArn,omitempty"`
}

// ExecutionSucceededEventDetails contains details about an ExecutionSucceeded event.
type ExecutionSucceededEventDetails struct {
	Output        string                                `json:"output,omitempty"`
	OutputDetails *CloudWatchEventsExecutionDataDetails `json:"outputDetails,omitempty"`
}

// ExecutionFailedEventDetails contains details about an ExecutionFailed event.
type ExecutionFailedEventDetails struct {
	Error string `json:"error,omitempty"`
	Cause string `json:"cause,omitempty"`
}

// ExecutionAbortedEventDetails contains details about an ExecutionAborted event.
type ExecutionAbortedEventDetails struct {
	Error string `json:"error,omitempty"`
	Cause string `json:"cause,omitempty"`
}

// ExecutionTimedOutEventDetails contains details about an ExecutionTimedOut event.
type ExecutionTimedOutEventDetails struct {
	Error string `json:"error,omitempty"`
	Cause string `json:"cause,omitempty"`
}

// CreateStateMachineRequest is the request for CreateStateMachine.
type CreateStateMachineRequest struct {
	Name                 string                `json:"name"`
	Definition           string                `json:"definition"`
	RoleArn              string                `json:"roleArn"`
	Type                 string                `json:"type,omitempty"`
	LoggingConfiguration *LoggingConfiguration `json:"loggingConfiguration,omitempty"`
	TracingConfiguration *TracingConfiguration `json:"tracingConfiguration,omitempty"`
	Tags                 []Tag                 `json:"tags,omitempty"`
	Publish              bool                  `json:"publish,omitempty"`
	VersionDescription   string                `json:"versionDescription,omitempty"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CreateStateMachineResponse is the response for CreateStateMachine.
type CreateStateMachineResponse struct {
	StateMachineArn        string  `json:"stateMachineArn"`
	CreationDate           float64 `json:"creationDate"`
	StateMachineVersionArn string  `json:"stateMachineVersionArn,omitempty"`
}

// DeleteStateMachineRequest is the request for DeleteStateMachine.
type DeleteStateMachineRequest struct {
	StateMachineArn string `json:"stateMachineArn"`
}

// DeleteStateMachineResponse is the response for DeleteStateMachine.
type DeleteStateMachineResponse struct{}

// DescribeStateMachineRequest is the request for DescribeStateMachine.
type DescribeStateMachineRequest struct {
	StateMachineArn string `json:"stateMachineArn"`
}

// DescribeStateMachineResponse is the response for DescribeStateMachine.
type DescribeStateMachineResponse struct {
	StateMachineArn      string                `json:"stateMachineArn"`
	Name                 string                `json:"name"`
	Status               string                `json:"status"`
	Definition           string                `json:"definition"`
	RoleArn              string                `json:"roleArn"`
	Type                 string                `json:"type"`
	CreationDate         float64               `json:"creationDate"`
	LoggingConfiguration *LoggingConfiguration `json:"loggingConfiguration,omitempty"`
	TracingConfiguration *TracingConfiguration `json:"tracingConfiguration,omitempty"`
	Label                string                `json:"label,omitempty"`
	RevisionID           string                `json:"revisionId,omitempty"`
	Description          string                `json:"description,omitempty"`
}

// ListStateMachinesRequest is the request for ListStateMachines.
type ListStateMachinesRequest struct {
	MaxResults int32  `json:"maxResults,omitempty"`
	NextToken  string `json:"nextToken,omitempty"`
}

// ListStateMachinesResponse is the response for ListStateMachines.
type ListStateMachinesResponse struct {
	StateMachines []StateMachineListItem `json:"stateMachines"`
	NextToken     string                 `json:"nextToken,omitempty"`
}

// StateMachineListItem represents a state machine in a list.
type StateMachineListItem struct {
	StateMachineArn string  `json:"stateMachineArn"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	CreationDate    float64 `json:"creationDate"`
}

// StartExecutionRequest is the request for StartExecution.
type StartExecutionRequest struct {
	StateMachineArn string `json:"stateMachineArn"`
	Name            string `json:"name,omitempty"`
	Input           string `json:"input,omitempty"`
	TraceHeader     string `json:"traceHeader,omitempty"`
}

// StartExecutionResponse is the response for StartExecution.
type StartExecutionResponse struct {
	ExecutionArn string  `json:"executionArn"`
	StartDate    float64 `json:"startDate"`
}

// StopExecutionRequest is the request for StopExecution.
type StopExecutionRequest struct {
	ExecutionArn string `json:"executionArn"`
	Error        string `json:"error,omitempty"`
	Cause        string `json:"cause,omitempty"`
}

// StopExecutionResponse is the response for StopExecution.
type StopExecutionResponse struct {
	StopDate float64 `json:"stopDate"`
}

// DescribeExecutionRequest is the request for DescribeExecution.
type DescribeExecutionRequest struct {
	ExecutionArn string `json:"executionArn"`
}

// DescribeExecutionResponse is the response for DescribeExecution.
type DescribeExecutionResponse struct {
	ExecutionArn        string                                `json:"executionArn"`
	StateMachineArn     string                                `json:"stateMachineArn"`
	Name                string                                `json:"name"`
	Status              string                                `json:"status"`
	StartDate           float64                               `json:"startDate"`
	StopDate            float64                               `json:"stopDate,omitempty"`
	Input               string                                `json:"input,omitempty"`
	InputDetails        *CloudWatchEventsExecutionDataDetails `json:"inputDetails,omitempty"`
	Output              string                                `json:"output,omitempty"`
	OutputDetails       *CloudWatchEventsExecutionDataDetails `json:"outputDetails,omitempty"`
	Error               string                                `json:"error,omitempty"`
	Cause               string                                `json:"cause,omitempty"`
	TraceHeader         string                                `json:"traceHeader,omitempty"`
	RedriveCount        int32                                 `json:"redriveCount,omitempty"`
	RedriveDate         float64                               `json:"redriveDate,omitempty"`
	RedriveStatus       string                                `json:"redriveStatus,omitempty"`
	RedriveStatusReason string                                `json:"redriveStatusReason,omitempty"`
}

// ListExecutionsRequest is the request for ListExecutions.
type ListExecutionsRequest struct {
	StateMachineArn string `json:"stateMachineArn"`
	StatusFilter    string `json:"statusFilter,omitempty"`
	MaxResults      int32  `json:"maxResults,omitempty"`
	NextToken       string `json:"nextToken,omitempty"`
	RedriveFilter   string `json:"redriveFilter,omitempty"`
}

// ListExecutionsResponse is the response for ListExecutions.
type ListExecutionsResponse struct {
	Executions []ExecutionListItem `json:"executions"`
	NextToken  string              `json:"nextToken,omitempty"`
}

// ExecutionListItem represents an execution in a list.
type ExecutionListItem struct {
	ExecutionArn           string  `json:"executionArn"`
	StateMachineArn        string  `json:"stateMachineArn"`
	Name                   string  `json:"name"`
	Status                 string  `json:"status"`
	StartDate              float64 `json:"startDate"`
	StopDate               float64 `json:"stopDate,omitempty"`
	RedriveCount           int32   `json:"redriveCount,omitempty"`
	RedriveDate            float64 `json:"redriveDate,omitempty"`
	StateMachineAliasArn   string  `json:"stateMachineAliasArn,omitempty"`
	StateMachineVersionArn string  `json:"stateMachineVersionArn,omitempty"`
}

// GetExecutionHistoryRequest is the request for GetExecutionHistory.
type GetExecutionHistoryRequest struct {
	ExecutionArn         string `json:"executionArn"`
	MaxResults           int32  `json:"maxResults,omitempty"`
	NextToken            string `json:"nextToken,omitempty"`
	ReverseOrder         bool   `json:"reverseOrder,omitempty"`
	IncludeExecutionData bool   `json:"includeExecutionData,omitempty"`
}

// GetExecutionHistoryResponse is the response for GetExecutionHistory.
type GetExecutionHistoryResponse struct {
	Events    []HistoryEventOutput `json:"events"`
	NextToken string               `json:"nextToken,omitempty"`
}

// HistoryEventOutput represents a history event in the output.
type HistoryEventOutput struct {
	Timestamp                      float64                         `json:"timestamp"`
	Type                           string                          `json:"type"`
	ID                             int64                           `json:"id"`
	PreviousEventID                int64                           `json:"previousEventId,omitempty"`
	ExecutionStartedEventDetails   *ExecutionStartedEventDetails   `json:"executionStartedEventDetails,omitempty"`
	ExecutionSucceededEventDetails *ExecutionSucceededEventDetails `json:"executionSucceededEventDetails,omitempty"`
	ExecutionFailedEventDetails    *ExecutionFailedEventDetails    `json:"executionFailedEventDetails,omitempty"`
	ExecutionAbortedEventDetails   *ExecutionAbortedEventDetails   `json:"executionAbortedEventDetails,omitempty"`
	ExecutionTimedOutEventDetails  *ExecutionTimedOutEventDetails  `json:"executionTimedOutEventDetails,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError represents a service-level error.
type ServiceError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *ServiceError) Error() string {
	return e.Message
}

// ListTagsForResourceRequest is the request for ListTagsForResource.
type ListTagsForResourceRequest struct {
	ResourceArn string `json:"resourceArn"`
}

// ListTagsForResourceResponse is the response for ListTagsForResource.
type ListTagsForResourceResponse struct {
	Tags []Tag `json:"tags"`
}

// TagResourceRequest is the request for TagResource.
type TagResourceRequest struct {
	ResourceArn string `json:"resourceArn"`
	Tags        []Tag  `json:"tags"`
}

// TagResourceResponse is the response for TagResource.
type TagResourceResponse struct{}

// UntagResourceRequest is the request for UntagResource.
type UntagResourceRequest struct {
	ResourceArn string   `json:"resourceArn"`
	TagKeys     []string `json:"tagKeys"`
}

// UntagResourceResponse is the response for UntagResource.
type UntagResourceResponse struct{}

// ListStateMachineVersionsRequest is the request for ListStateMachineVersions.
type ListStateMachineVersionsRequest struct {
	StateMachineArn string `json:"stateMachineArn"`
	MaxResults      int32  `json:"maxResults,omitempty"`
	NextToken       string `json:"nextToken,omitempty"`
}

// ListStateMachineVersionsResponse is the response for ListStateMachineVersions.
type ListStateMachineVersionsResponse struct {
	StateMachineVersions []map[string]string `json:"stateMachineVersions"`
	NextToken            string              `json:"nextToken,omitempty"`
}

// ListStateMachineAliasesRequest is the request for ListStateMachineAliases.
type ListStateMachineAliasesRequest struct {
	StateMachineArn string `json:"stateMachineArn"`
	MaxResults      int32  `json:"maxResults,omitempty"`
	NextToken       string `json:"nextToken,omitempty"`
}

// ListStateMachineAliasesResponse is the response for ListStateMachineAliases.
type ListStateMachineAliasesResponse struct {
	StateMachineAliases []map[string]string `json:"stateMachineAliases"`
	NextToken           string              `json:"nextToken,omitempty"`
}

// ValidateStateMachineDefinitionRequest is the request for ValidateStateMachineDefinition.
type ValidateStateMachineDefinitionRequest struct {
	Definition string `json:"definition"`
	Type       string `json:"type,omitempty"`
}

// ValidateStateMachineDefinitionResponse is the response for ValidateStateMachineDefinition.
type ValidateStateMachineDefinitionResponse struct {
	Result      string               `json:"result"`
	Diagnostics []validateDiagnostic `json:"diagnostics"`
}

// validateDiagnostic is a single ValidateStateMachineDefinition diagnostic.
type validateDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Location string `json:"location,omitempty"`
}
