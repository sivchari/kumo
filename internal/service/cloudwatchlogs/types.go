// Package cloudwatchlogs provides CloudWatch Logs service emulation for kumo.
package cloudwatchlogs

// LogGroup represents a log group in CloudWatch Logs.
type LogGroup struct {
	LogGroupName      string
	LogGroupARN       string
	CreationTime      int64
	RetentionInDays   *int32
	MetricFilterCount int32
	StoredBytes       int64
	KmsKeyID          string
	DataProtection    string
	LogGroupClass     string
}

// LogStream represents a log stream in CloudWatch Logs.
type LogStream struct {
	LogStreamName       string
	CreationTime        int64
	FirstEventTimestamp *int64
	LastEventTimestamp  *int64
	LastIngestionTime   *int64
	UploadSequenceToken string
	LogStreamARN        string
	StoredBytes         int64
}

// LogEvent represents a log event in CloudWatch Logs.
type LogEvent struct {
	Timestamp int64
	Message   string
}

// OutputLogEvent represents a log event in get/filter responses.
type OutputLogEvent struct {
	Timestamp     int64  `json:"timestamp"`
	Message       string `json:"message"`
	IngestionTime int64  `json:"ingestionTime"`
}

// CreateLogGroupRequest is the request for CreateLogGroup.
type CreateLogGroupRequest struct {
	LogGroupName  string            `json:"logGroupName"`
	KmsKeyID      string            `json:"kmsKeyId,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
	LogGroupClass string            `json:"logGroupClass,omitempty"`
}

// DeleteLogGroupRequest is the request for DeleteLogGroup.
type DeleteLogGroupRequest struct {
	LogGroupName string `json:"logGroupName"`
}

// PutRetentionPolicyRequest is the request for PutRetentionPolicy.
type PutRetentionPolicyRequest struct {
	LogGroupName    string `json:"logGroupName"`
	RetentionInDays int32  `json:"retentionInDays"`
}

// DeleteRetentionPolicyRequest is the request for DeleteRetentionPolicy.
type DeleteRetentionPolicyRequest struct {
	LogGroupName string `json:"logGroupName"`
}

// CreateLogStreamRequest is the request for CreateLogStream.
type CreateLogStreamRequest struct {
	LogGroupName  string `json:"logGroupName"`
	LogStreamName string `json:"logStreamName"`
}

// DeleteLogStreamRequest is the request for DeleteLogStream.
type DeleteLogStreamRequest struct {
	LogGroupName  string `json:"logGroupName"`
	LogStreamName string `json:"logStreamName"`
}

// InputLogEvent represents a log event in put requests.
type InputLogEvent struct {
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

// PutLogEventsRequest is the request for PutLogEvents.
type PutLogEventsRequest struct {
	LogGroupName  string          `json:"logGroupName"`
	LogStreamName string          `json:"logStreamName"`
	LogEvents     []InputLogEvent `json:"logEvents"`
	SequenceToken string          `json:"sequenceToken,omitempty"`
}

// PutLogEventsResponse is the response for PutLogEvents.
type PutLogEventsResponse struct {
	NextSequenceToken     string                 `json:"nextSequenceToken,omitempty"`
	RejectedLogEventsInfo *RejectedLogEventsInfo `json:"rejectedLogEventsInfo,omitempty"`
	RejectedEntityInfo    *RejectedEntityInfo    `json:"rejectedEntityInfo,omitempty"`
}

// RejectedLogEventsInfo contains info about rejected log events.
type RejectedLogEventsInfo struct {
	TooNewLogEventStartIndex int32 `json:"tooNewLogEventStartIndex,omitempty"`
	TooOldLogEventEndIndex   int32 `json:"tooOldLogEventEndIndex,omitempty"`
	ExpiredLogEventEndIndex  int32 `json:"expiredLogEventEndIndex,omitempty"`
}

// RejectedEntityInfo contains info about rejected entity.
type RejectedEntityInfo struct {
	ErrorType string `json:"errorType,omitempty"`
}

// GetLogEventsRequest is the request for GetLogEvents.
type GetLogEventsRequest struct {
	LogGroupName  string `json:"logGroupName"`
	LogStreamName string `json:"logStreamName"`
	StartTime     *int64 `json:"startTime,omitempty"`
	EndTime       *int64 `json:"endTime,omitempty"`
	NextToken     string `json:"nextToken,omitempty"`
	Limit         *int32 `json:"limit,omitempty"`
	StartFromHead *bool  `json:"startFromHead,omitempty"`
	Unmask        bool   `json:"unmask,omitempty"`
}

// GetLogEventsResponse is the response for GetLogEvents.
type GetLogEventsResponse struct {
	Events            []OutputLogEvent `json:"events"`
	NextForwardToken  string           `json:"nextForwardToken,omitempty"`
	NextBackwardToken string           `json:"nextBackwardToken,omitempty"`
}

// FilterLogEventsRequest is the request for FilterLogEvents.
type FilterLogEventsRequest struct {
	LogGroupName        string   `json:"logGroupName,omitempty"`
	LogGroupIdentifier  string   `json:"logGroupIdentifier,omitempty"`
	LogStreamNames      []string `json:"logStreamNames,omitempty"`
	LogStreamNamePrefix string   `json:"logStreamNamePrefix,omitempty"`
	StartTime           *int64   `json:"startTime,omitempty"`
	EndTime             *int64   `json:"endTime,omitempty"`
	FilterPattern       string   `json:"filterPattern,omitempty"`
	NextToken           string   `json:"nextToken,omitempty"`
	Limit               *int32   `json:"limit,omitempty"`
	Unmask              bool     `json:"unmask,omitempty"`
}

// FilteredLogEvent represents a filtered log event.
type FilteredLogEvent struct {
	LogStreamName string `json:"logStreamName"`
	Timestamp     int64  `json:"timestamp"`
	Message       string `json:"message"`
	IngestionTime int64  `json:"ingestionTime"`
	EventID       string `json:"eventId"`
}

// SearchedLogStream represents a searched log stream.
type SearchedLogStream struct {
	LogStreamName      string `json:"logStreamName"`
	SearchedCompletely bool   `json:"searchedCompletely"`
}

// FilterLogEventsResponse is the response for FilterLogEvents.
type FilterLogEventsResponse struct {
	Events             []FilteredLogEvent  `json:"events,omitempty"`
	SearchedLogStreams []SearchedLogStream `json:"searchedLogStreams,omitempty"`
	NextToken          string              `json:"nextToken,omitempty"`
}

// DescribeLogGroupsRequest is the request for DescribeLogGroups.
type DescribeLogGroupsRequest struct {
	AccountIdentifiers    []string `json:"accountIdentifiers,omitempty"`
	LogGroupNamePrefix    string   `json:"logGroupNamePrefix,omitempty"`
	LogGroupNamePattern   string   `json:"logGroupNamePattern,omitempty"`
	NextToken             string   `json:"nextToken,omitempty"`
	Limit                 *int32   `json:"limit,omitempty"`
	IncludeLinkedAccounts bool     `json:"includeLinkedAccounts,omitempty"`
	LogGroupClass         string   `json:"logGroupClass,omitempty"`
}

// LogGroupResponse represents a log group in API responses.
type LogGroupResponse struct {
	LogGroupName         string   `json:"logGroupName"`
	LogGroupARN          string   `json:"arn"`
	CreationTime         int64    `json:"creationTime"`
	RetentionInDays      *int32   `json:"retentionInDays,omitempty"`
	MetricFilterCount    int32    `json:"metricFilterCount"`
	StoredBytes          int64    `json:"storedBytes"`
	KmsKeyID             string   `json:"kmsKeyId,omitempty"`
	DataProtectionStatus string   `json:"dataProtectionStatus,omitempty"`
	InheritedProperties  []string `json:"inheritedProperties,omitempty"`
	LogGroupClass        string   `json:"logGroupClass,omitempty"`
	LogGroupArn          string   `json:"logGroupArn,omitempty"`
}

// DescribeLogGroupsResponse is the response for DescribeLogGroups.
type DescribeLogGroupsResponse struct {
	LogGroups []LogGroupResponse `json:"logGroups"`
	NextToken string             `json:"nextToken,omitempty"`
}

// DescribeLogStreamsRequest is the request for DescribeLogStreams.
type DescribeLogStreamsRequest struct {
	LogGroupName        string `json:"logGroupName,omitempty"`
	LogGroupIdentifier  string `json:"logGroupIdentifier,omitempty"`
	LogStreamNamePrefix string `json:"logStreamNamePrefix,omitempty"`
	OrderBy             string `json:"orderBy,omitempty"`
	Descending          *bool  `json:"descending,omitempty"`
	NextToken           string `json:"nextToken,omitempty"`
	Limit               *int32 `json:"limit,omitempty"`
}

// LogStreamResponse represents a log stream in API responses.
type LogStreamResponse struct {
	LogStreamName       string `json:"logStreamName"`
	CreationTime        int64  `json:"creationTime"`
	FirstEventTimestamp *int64 `json:"firstEventTimestamp,omitempty"`
	LastEventTimestamp  *int64 `json:"lastEventTimestamp,omitempty"`
	LastIngestionTime   *int64 `json:"lastIngestionTime,omitempty"`
	UploadSequenceToken string `json:"uploadSequenceToken,omitempty"`
	Arn                 string `json:"arn,omitempty"`
	StoredBytes         int64  `json:"storedBytes"`
}

// DescribeLogStreamsResponse is the response for DescribeLogStreams.
type DescribeLogStreamsResponse struct {
	LogStreams []LogStreamResponse `json:"logStreams"`
	NextToken  string              `json:"nextToken,omitempty"`
}

// ErrorResponse represents a CloudWatch Logs error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// LogsError represents a CloudWatch Logs error.
type LogsError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *LogsError) Error() string {
	return e.Message
}
