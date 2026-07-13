package cloudtrail

import "time"

// Trail represents a CloudTrail trail.
type Trail struct {
	Name                       string
	TrailARN                   string
	S3BucketName               string
	S3KeyPrefix                string
	IncludeGlobalServiceEvents bool
	IsMultiRegionTrail         bool
	HomeRegion                 string
	IsLogging                  bool
	LogFileValidationEnabled   bool
	CloudWatchLogsLogGroupArn  string
	CloudWatchLogsRoleArn      string
	KMSKeyID                   string
	HasCustomEventSelectors    bool
	HasInsightSelectors        bool
	IsOrganizationTrail        bool
	CreationTime               time.Time
	Tags                       map[string]string
}

// Tag is a single CloudTrail resource tag. CloudTrail represents tags as a list
// of key/value pairs (unlike Verified Permissions, which uses a map).
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value,omitempty"`
}

// Event represents a CloudTrail event for LookupEvents.
type Event struct {
	EventID         string
	EventName       string
	EventSource     string
	EventTime       time.Time
	Username        string
	CloudTrailEvent string
}

// Error represents a CloudTrail error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

// CreateTrailRequest represents the CreateTrail API request.
type CreateTrailRequest struct {
	Name                       string `json:"Name"`
	S3BucketName               string `json:"S3BucketName"`
	S3KeyPrefix                string `json:"S3KeyPrefix,omitempty"`
	IncludeGlobalServiceEvents *bool  `json:"IncludeGlobalServiceEvents,omitempty"`
	IsMultiRegionTrail         *bool  `json:"IsMultiRegionTrail,omitempty"`
	EnableLogFileValidation    *bool  `json:"EnableLogFileValidation,omitempty"`
	CloudWatchLogsLogGroupArn  string `json:"CloudWatchLogsLogGroupArn,omitempty"`
	CloudWatchLogsRoleArn      string `json:"CloudWatchLogsRoleArn,omitempty"`
	KMSKeyID                   string `json:"KmsKeyId,omitempty"`
	IsOrganizationTrail        *bool  `json:"IsOrganizationTrail,omitempty"`
	TagsList                   []Tag  `json:"TagsList,omitempty"`
}

// CreateTrailResponse represents the CreateTrail API response.
type CreateTrailResponse struct {
	Name                       string `json:"Name"`
	TrailARN                   string `json:"TrailARN"`
	S3BucketName               string `json:"S3BucketName"`
	S3KeyPrefix                string `json:"S3KeyPrefix,omitempty"`
	IncludeGlobalServiceEvents *bool  `json:"IncludeGlobalServiceEvents"`
	IsMultiRegionTrail         *bool  `json:"IsMultiRegionTrail"`
	LogFileValidationEnabled   *bool  `json:"LogFileValidationEnabled"`
	CloudWatchLogsLogGroupArn  string `json:"CloudWatchLogsLogGroupArn,omitempty"`
	CloudWatchLogsRoleArn      string `json:"CloudWatchLogsRoleArn,omitempty"`
	KMSKeyID                   string `json:"KmsKeyId,omitempty"`
	IsOrganizationTrail        *bool  `json:"IsOrganizationTrail"`
}

// DeleteTrailRequest represents the DeleteTrail API request.
type DeleteTrailRequest struct {
	Name string `json:"Name"`
}

// DeleteTrailResponse represents the DeleteTrail API response.
type DeleteTrailResponse struct{}

// GetTrailRequest represents the GetTrail API request.
type GetTrailRequest struct {
	Name string `json:"Name"`
}

// GetTrailResponse represents the GetTrail API response.
type GetTrailResponse struct {
	Trail *TrailOutput `json:"Trail"`
}

// TrailOutput represents the output format of a trail.
type TrailOutput struct {
	Name                       string `json:"Name"`
	TrailARN                   string `json:"TrailARN"`
	S3BucketName               string `json:"S3BucketName"`
	S3KeyPrefix                string `json:"S3KeyPrefix,omitempty"`
	IncludeGlobalServiceEvents bool   `json:"IncludeGlobalServiceEvents"`
	IsMultiRegionTrail         bool   `json:"IsMultiRegionTrail"`
	HomeRegion                 string `json:"HomeRegion"`
	LogFileValidationEnabled   bool   `json:"LogFileValidationEnabled"`
	CloudWatchLogsLogGroupArn  string `json:"CloudWatchLogsLogGroupArn,omitempty"`
	CloudWatchLogsRoleArn      string `json:"CloudWatchLogsRoleArn,omitempty"`
	KMSKeyID                   string `json:"KmsKeyId,omitempty"`
	HasCustomEventSelectors    bool   `json:"HasCustomEventSelectors"`
	HasInsightSelectors        bool   `json:"HasInsightSelectors"`
	IsOrganizationTrail        bool   `json:"IsOrganizationTrail"`
}

// DescribeTrailsRequest represents the DescribeTrails API request.
type DescribeTrailsRequest struct {
	TrailNameList       []string `json:"trailNameList,omitempty"`
	IncludeShadowTrails *bool    `json:"includeShadowTrails,omitempty"`
}

// DescribeTrailsResponse represents the DescribeTrails API response.
type DescribeTrailsResponse struct {
	TrailList []TrailOutput `json:"trailList"`
}

// StartLoggingRequest represents the StartLogging API request.
type StartLoggingRequest struct {
	Name string `json:"Name"`
}

// StartLoggingResponse represents the StartLogging API response.
type StartLoggingResponse struct{}

// StopLoggingRequest represents the StopLogging API request.
type StopLoggingRequest struct {
	Name string `json:"Name"`
}

// StopLoggingResponse represents the StopLogging API response.
type StopLoggingResponse struct{}

// LookupEventsRequest represents the LookupEvents API request.
type LookupEventsRequest struct {
	LookupAttributes []LookupAttribute `json:"LookupAttributes,omitempty"`
	StartTime        *float64          `json:"StartTime,omitempty"`
	EndTime          *float64          `json:"EndTime,omitempty"`
	MaxResults       *int32            `json:"MaxResults,omitempty"`
	NextToken        string            `json:"NextToken,omitempty"`
}

// LookupAttribute represents an attribute for filtering events.
type LookupAttribute struct {
	AttributeKey   string `json:"AttributeKey"`
	AttributeValue string `json:"AttributeValue"`
}

// LookupEventsResponse represents the LookupEvents API response.
type LookupEventsResponse struct {
	Events    []EventOutput `json:"Events"`
	NextToken string        `json:"NextToken,omitempty"`
}

// EventOutput represents the output format of an event.
type EventOutput struct {
	EventID         string   `json:"EventId"`
	EventName       string   `json:"EventName"`
	EventSource     string   `json:"EventSource"`
	EventTime       float64  `json:"EventTime"`
	Username        string   `json:"Username,omitempty"`
	CloudTrailEvent string   `json:"CloudTrailEvent,omitempty"`
	Resources       []string `json:"Resources,omitempty"`
}

// GetTrailStatusRequest represents the GetTrailStatus API request.
type GetTrailStatusRequest struct {
	Name string `json:"Name"`
}

// GetTrailStatusResponse represents the GetTrailStatus API response.
type GetTrailStatusResponse struct {
	IsLogging                         *bool   `json:"IsLogging"`
	LatestDeliveryTime                float64 `json:"LatestDeliveryTime,omitempty"`
	LatestNotificationTime            float64 `json:"LatestNotificationTime,omitempty"`
	StartLoggingTime                  float64 `json:"StartLoggingTime,omitempty"`
	StopLoggingTime                   float64 `json:"StopLoggingTime,omitempty"`
	LatestCloudWatchLogsDeliveryTime  float64 `json:"LatestCloudWatchLogsDeliveryTime,omitempty"`
	LatestDigestDeliveryTime          float64 `json:"LatestDigestDeliveryTime,omitempty"`
	LatestDeliveryError               string  `json:"LatestDeliveryError,omitempty"`
	LatestNotificationError           string  `json:"LatestNotificationError,omitempty"`
	LatestCloudWatchLogsDeliveryError string  `json:"LatestCloudWatchLogsDeliveryError,omitempty"`
}

// ListTagsRequest represents the ListTags API request.
type ListTagsRequest struct {
	ResourceIDList []string `json:"ResourceIdList"`
	NextToken      string   `json:"NextToken,omitempty"`
}

// ListTagsResponse represents the ListTags API response.
type ListTagsResponse struct {
	ResourceTagList []ResourceTag `json:"ResourceTagList"`
	NextToken       string        `json:"NextToken,omitempty"`
}

// ResourceTag pairs a resource ARN with its tag list in the ListTags response.
type ResourceTag struct {
	ResourceID string `json:"ResourceId"`
	TagsList   []Tag  `json:"TagsList"`
}

// AddTagsRequest represents the AddTags API request.
type AddTagsRequest struct {
	ResourceID string `json:"ResourceId"`
	TagsList   []Tag  `json:"TagsList"`
}

// AddTagsResponse represents the AddTags API response.
type AddTagsResponse struct{}

// RemoveTagsRequest represents the RemoveTags API request.
type RemoveTagsRequest struct {
	ResourceID string `json:"ResourceId"`
	TagsList   []Tag  `json:"TagsList"`
}

// RemoveTagsResponse represents the RemoveTags API response.
type RemoveTagsResponse struct{}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}
