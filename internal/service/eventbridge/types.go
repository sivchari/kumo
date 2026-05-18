package eventbridge

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// EventBusState represents the state of an event bus.
type EventBusState string

// Event bus states.
const (
	EventBusStateActive  EventBusState = "ACTIVE"
	EventBusStateDeleted EventBusState = "DELETED"
)

// RuleState represents the state of a rule.
type RuleState string

// Rule states.
const (
	RuleStateEnabled  RuleState = "ENABLED"
	RuleStateDisabled RuleState = "DISABLED"
)

// EventBus represents an event bus.
type EventBus struct {
	Name         string    `json:"name"`
	Arn          string    `json:"arn"`
	Description  string    `json:"description,omitempty"`
	ManagedBy    string    `json:"managedBy,omitempty"`
	CreationTime time.Time `json:"creationTime"`
	LastModified time.Time `json:"lastModified"`
}

// Rule represents an EventBridge rule.
type Rule struct {
	Name               string    `json:"name"`
	Arn                string    `json:"arn"`
	EventBusName       string    `json:"eventBusName"`
	EventPattern       string    `json:"eventPattern,omitempty"`
	ScheduleExpression string    `json:"scheduleExpression,omitempty"`
	State              RuleState `json:"state"`
	Description        string    `json:"description,omitempty"`
	RoleArn            string    `json:"roleArn,omitempty"`
	CreationTime       time.Time `json:"creationTime"`
	LastModified       time.Time `json:"lastModified"`
}

// InputTransformer represents an input transformer for a target.
type InputTransformer struct {
	InputPathsMap map[string]string `json:"inputPathsMap,omitempty"`
	InputTemplate string            `json:"inputTemplate"`
}

// Target represents a rule target.
type Target struct {
	ID               string            `json:"id"`
	Arn              string            `json:"arn"`
	RoleArn          string            `json:"roleArn,omitempty"`
	Input            string            `json:"input,omitempty"`
	InputPath        string            `json:"inputPath,omitempty"`
	InputTransformer *InputTransformer `json:"inputTransformer,omitempty"`
	HTTPParameters   *HTTPParameters   `json:"httpParameters,omitempty"`
}

// EpochTime wraps time.Time to support JSON unmarshalling from both
// epoch seconds (number) and RFC3339 strings.
// AWS SDK v2 for Go serialises the Time field as epoch seconds.
type EpochTime struct {
	time.Time
}

// UnmarshalJSON handles epoch seconds (float64) and RFC3339 string formats.
func (t *EpochTime) UnmarshalJSON(data []byte) error {
	// Try as a number first (epoch seconds).
	var epoch float64
	if err := json.Unmarshal(data, &epoch); err == nil {
		sec, frac := math.Modf(epoch)
		t.Time = time.Unix(int64(sec), int64(frac*1e9))

		return nil
	}

	// Fall back to standard time.Time parsing (RFC3339).
	var stdTime time.Time
	if err := json.Unmarshal(data, &stdTime); err != nil {
		return fmt.Errorf("cannot parse time %s: %w", string(data), err)
	}

	t.Time = stdTime

	return nil
}

// MarshalJSON serialises as an RFC3339 string for consistency.
func (t EpochTime) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(t.Time)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal time: %w", err)
	}

	return data, nil
}

// PutEventsRequestEntry represents an entry in PutEvents request.
type PutEventsRequestEntry struct {
	Source       string     `json:"Source,omitempty"`
	DetailType   string     `json:"DetailType,omitempty"`
	Detail       string     `json:"Detail,omitempty"`
	EventBusName string     `json:"EventBusName,omitempty"`
	Time         *EpochTime `json:"Time,omitempty"`
	Resources    []string   `json:"Resources,omitempty"`
}

// PutEventsResultEntry represents an entry in PutEvents response.
type PutEventsResultEntry struct {
	EventID      string `json:"EventId,omitempty"`
	ErrorCode    string `json:"ErrorCode,omitempty"`
	ErrorMessage string `json:"ErrorMessage,omitempty"`
}

// CreateEventBusRequest is the request for CreateEventBus.
type CreateEventBusRequest struct {
	Name        string `json:"Name"`
	Description string `json:"Description,omitempty"`
}

// CreateEventBusResponse is the response for CreateEventBus.
type CreateEventBusResponse struct {
	EventBusArn string `json:"EventBusArn,omitempty"`
}

// DeleteEventBusRequest is the request for DeleteEventBus.
type DeleteEventBusRequest struct {
	Name string `json:"Name"`
}

// DeleteEventBusResponse is the response for DeleteEventBus.
type DeleteEventBusResponse struct{}

// DescribeEventBusRequest is the request for DescribeEventBus.
type DescribeEventBusRequest struct {
	Name string `json:"Name,omitempty"`
}

// DescribeEventBusResponse is the response for DescribeEventBus.
type DescribeEventBusResponse struct {
	Name        string `json:"Name,omitempty"`
	Arn         string `json:"Arn,omitempty"`
	Description string `json:"Description,omitempty"`
}

// ListEventBusesRequest is the request for ListEventBuses.
type ListEventBusesRequest struct {
	NamePrefix string `json:"NamePrefix,omitempty"`
	Limit      int32  `json:"Limit,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
}

// EventBusOutput represents an event bus in API responses.
type EventBusOutput struct {
	Name        string `json:"Name,omitempty"`
	Arn         string `json:"Arn,omitempty"`
	Description string `json:"Description,omitempty"`
	ManagedBy   string `json:"ManagedBy,omitempty"`
}

// ListEventBusesResponse is the response for ListEventBuses.
type ListEventBusesResponse struct {
	EventBuses []EventBusOutput `json:"EventBuses,omitempty"`
	NextToken  string           `json:"NextToken,omitempty"`
}

// PutRuleRequest is the request for PutRule.
type PutRuleRequest struct {
	Name               string `json:"Name"`
	EventBusName       string `json:"EventBusName,omitempty"`
	EventPattern       string `json:"EventPattern,omitempty"`
	ScheduleExpression string `json:"ScheduleExpression,omitempty"`
	State              string `json:"State,omitempty"`
	Description        string `json:"Description,omitempty"`
	RoleArn            string `json:"RoleArn,omitempty"`
}

// PutRuleResponse is the response for PutRule.
type PutRuleResponse struct {
	RuleArn string `json:"RuleArn,omitempty"`
}

// DeleteRuleRequest is the request for DeleteRule.
type DeleteRuleRequest struct {
	Name         string `json:"Name"`
	EventBusName string `json:"EventBusName,omitempty"`
	Force        bool   `json:"Force,omitempty"`
}

// DeleteRuleResponse is the response for DeleteRule.
type DeleteRuleResponse struct{}

// DescribeRuleRequest is the request for DescribeRule.
type DescribeRuleRequest struct {
	Name         string `json:"Name"`
	EventBusName string `json:"EventBusName,omitempty"`
}

// DescribeRuleResponse is the response for DescribeRule.
type DescribeRuleResponse struct {
	Name               string `json:"Name,omitempty"`
	Arn                string `json:"Arn,omitempty"`
	EventBusName       string `json:"EventBusName,omitempty"`
	EventPattern       string `json:"EventPattern,omitempty"`
	ScheduleExpression string `json:"ScheduleExpression,omitempty"`
	State              string `json:"State,omitempty"`
	Description        string `json:"Description,omitempty"`
	RoleArn            string `json:"RoleArn,omitempty"`
}

// ListRulesRequest is the request for ListRules.
type ListRulesRequest struct {
	EventBusName string `json:"EventBusName,omitempty"`
	NamePrefix   string `json:"NamePrefix,omitempty"`
	Limit        int32  `json:"Limit,omitempty"`
	NextToken    string `json:"NextToken,omitempty"`
}

// RuleOutput represents a rule in API responses.
type RuleOutput struct {
	Name               string `json:"Name,omitempty"`
	Arn                string `json:"Arn,omitempty"`
	EventBusName       string `json:"EventBusName,omitempty"`
	EventPattern       string `json:"EventPattern,omitempty"`
	ScheduleExpression string `json:"ScheduleExpression,omitempty"`
	State              string `json:"State,omitempty"`
	Description        string `json:"Description,omitempty"`
	RoleArn            string `json:"RoleArn,omitempty"`
}

// ListRulesResponse is the response for ListRules.
type ListRulesResponse struct {
	Rules     []RuleOutput `json:"Rules,omitempty"`
	NextToken string       `json:"NextToken,omitempty"`
}

// InputTransformerInput represents an input transformer in API requests.
type InputTransformerInput struct {
	InputPathsMap map[string]string `json:"InputPathsMap,omitempty"`
	InputTemplate string            `json:"InputTemplate"`
}

// TargetInput represents a target in API requests.
type TargetInput struct {
	ID               string                 `json:"Id"`
	Arn              string                 `json:"Arn"`
	RoleArn          string                 `json:"RoleArn,omitempty"`
	Input            string                 `json:"Input,omitempty"`
	InputPath        string                 `json:"InputPath,omitempty"`
	InputTransformer *InputTransformerInput `json:"InputTransformer,omitempty"`
	HTTPParameters   *HTTPParameters        `json:"HttpParameters,omitempty"`
}

// PutTargetsRequest is the request for PutTargets.
type PutTargetsRequest struct {
	Rule         string        `json:"Rule"`
	EventBusName string        `json:"EventBusName,omitempty"`
	Targets      []TargetInput `json:"Targets"`
}

// PutTargetsResultEntry represents an entry in PutTargets response.
type PutTargetsResultEntry struct {
	TargetID     string `json:"TargetId,omitempty"`
	ErrorCode    string `json:"ErrorCode,omitempty"`
	ErrorMessage string `json:"ErrorMessage,omitempty"`
}

// PutTargetsResponse is the response for PutTargets.
type PutTargetsResponse struct {
	FailedEntryCount int32                   `json:"FailedEntryCount"`
	FailedEntries    []PutTargetsResultEntry `json:"FailedEntries,omitempty"`
}

// RemoveTargetsRequest is the request for RemoveTargets.
type RemoveTargetsRequest struct {
	Rule         string   `json:"Rule"`
	EventBusName string   `json:"EventBusName,omitempty"`
	IDs          []string `json:"Ids"`
	Force        bool     `json:"Force,omitempty"`
}

// RemoveTargetsResultEntry represents an entry in RemoveTargets response.
type RemoveTargetsResultEntry struct {
	TargetID     string `json:"TargetId,omitempty"`
	ErrorCode    string `json:"ErrorCode,omitempty"`
	ErrorMessage string `json:"ErrorMessage,omitempty"`
}

// RemoveTargetsResponse is the response for RemoveTargets.
type RemoveTargetsResponse struct {
	FailedEntryCount int32                      `json:"FailedEntryCount"`
	FailedEntries    []RemoveTargetsResultEntry `json:"FailedEntries,omitempty"`
}

// PutEventsRequest is the request for PutEvents.
type PutEventsRequest struct {
	Entries []PutEventsRequestEntry `json:"Entries"`
}

// PutEventsResponse is the response for PutEvents.
type PutEventsResponse struct {
	FailedEntryCount int32                  `json:"FailedEntryCount"`
	Entries          []PutEventsResultEntry `json:"Entries,omitempty"`
}

// ListTargetsByRuleRequest is the request for ListTargetsByRule.
type ListTargetsByRuleRequest struct {
	Rule         string `json:"Rule"`
	EventBusName string `json:"EventBusName,omitempty"`
	Limit        int32  `json:"Limit,omitempty"`
	NextToken    string `json:"NextToken,omitempty"`
}

// InputTransformerOutput represents an input transformer in API responses.
type InputTransformerOutput struct {
	InputPathsMap map[string]string `json:"InputPathsMap,omitempty"`
	InputTemplate string            `json:"InputTemplate,omitempty"`
}

// TargetOutput represents a target in API responses.
type TargetOutput struct {
	ID               string                  `json:"Id,omitempty"`
	Arn              string                  `json:"Arn,omitempty"`
	RoleArn          string                  `json:"RoleArn,omitempty"`
	Input            string                  `json:"Input,omitempty"`
	InputPath        string                  `json:"InputPath,omitempty"`
	InputTransformer *InputTransformerOutput `json:"InputTransformer,omitempty"`
	HTTPParameters   *HTTPParameters         `json:"HttpParameters,omitempty"`
}

// ListTargetsByRuleResponse is the response for ListTargetsByRule.
type ListTargetsByRuleResponse struct {
	Targets   []TargetOutput `json:"Targets,omitempty"`
	NextToken string         `json:"NextToken,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// DeliveredEvent represents an event that was matched to a rule and delivered to a target.
type DeliveredEvent struct {
	EventID      string `json:"EventId"`
	Source       string `json:"Source"`
	DetailType   string `json:"DetailType"`
	Detail       string `json:"Detail,omitempty"`
	EventBusName string `json:"EventBusName"`
	RuleName     string `json:"RuleName"`
	TargetID     string `json:"TargetId"`
	TargetArn    string `json:"TargetArn"`
	Time         string `json:"Time,omitempty"`
}

// Connection represents an EventBridge connection.
type Connection struct {
	Name               string         `json:"name"`
	Arn                string         `json:"arn"`
	ConnectionState    string         `json:"connectionState"`
	AuthorizationType  string         `json:"authorizationType"`
	AuthParameters     AuthParameters `json:"authParameters,omitempty"`
	CreationTime       time.Time      `json:"creationTime"`
	LastModifiedTime   time.Time      `json:"lastModifiedTime"`
	LastAuthorizedTime time.Time      `json:"lastAuthorizedTime"`
}

// AuthParameters represents connection auth parameters.
type AuthParameters struct {
	APIKeyAuthParameters     *APIKeyAuthParameters     `json:"ApiKeyAuthParameters,omitempty"`
	BasicAuthParameters      *BasicAuthParameters      `json:"BasicAuthParameters,omitempty"`
	OAuthParameters          *OAuthParameters          `json:"OAuthParameters,omitempty"`
	InvocationHTTPParameters *InvocationHTTPParameters `json:"InvocationHttpParameters,omitempty"`
}

// APIKeyAuthParameters represents API key auth parameters.
type APIKeyAuthParameters struct {
	APIKeyName  string `json:"ApiKeyName"`
	APIKeyValue string `json:"ApiKeyValue"`
}

// BasicAuthParameters represents basic auth parameters.
type BasicAuthParameters struct {
	Username string `json:"Username"`
	Password string `json:"Password"`
}

// OAuthParameters represents OAuth auth parameters.
type OAuthParameters struct {
	AuthorizationEndpoint string                 `json:"AuthorizationEndpoint"`
	HTTPMethod            string                 `json:"HttpMethod"`
	ClientParameters      *OAuthClientParameters `json:"ClientParameters,omitempty"`
}

// OAuthClientParameters represents OAuth client parameters.
type OAuthClientParameters struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
}

// InvocationHTTPParameters represents HTTP parameters for invocation.
type InvocationHTTPParameters struct {
	HeaderParameters      []ConnectionHTTPParameter `json:"HeaderParameters,omitempty"`
	QueryStringParameters []ConnectionHTTPParameter `json:"QueryStringParameters,omitempty"`
	BodyParameters        []ConnectionHTTPParameter `json:"BodyParameters,omitempty"`
}

// ConnectionHTTPParameter represents an HTTP parameter.
type ConnectionHTTPParameter struct {
	Key      string `json:"Key"`
	Value    string `json:"Value"`
	IsSecret bool   `json:"IsValueSecret"`
}

// CreateConnectionRequest is the request for CreateConnection.
type CreateConnectionRequest struct {
	Name              string         `json:"Name"`
	AuthorizationType string         `json:"AuthorizationType"`
	AuthParameters    AuthParameters `json:"AuthParameters"`
	Description       string         `json:"Description,omitempty"`
}

// CreateConnectionResponse is the response for CreateConnection.
type CreateConnectionResponse struct {
	ConnectionArn    string  `json:"ConnectionArn"`
	ConnectionState  string  `json:"ConnectionState"`
	CreationTime     float64 `json:"CreationTime"`
	LastModifiedTime float64 `json:"LastModifiedTime"`
}

// DescribeConnectionRequest is the request for DescribeConnection.
type DescribeConnectionRequest struct {
	Name string `json:"Name"`
}

// DescribeConnectionResponse is the response for DescribeConnection.
type DescribeConnectionResponse struct {
	Name               string  `json:"Name"`
	ConnectionArn      string  `json:"ConnectionArn"`
	ConnectionState    string  `json:"ConnectionState"`
	AuthorizationType  string  `json:"AuthorizationType"`
	CreationTime       float64 `json:"CreationTime"`
	LastModifiedTime   float64 `json:"LastModifiedTime"`
	LastAuthorizedTime float64 `json:"LastAuthorizedTime"`
}

// DeleteConnectionRequest is the request for DeleteConnection.
type DeleteConnectionRequest struct {
	Name string `json:"Name"`
}

// DeleteConnectionResponse is the response for DeleteConnection.
type DeleteConnectionResponse struct {
	ConnectionArn   string `json:"ConnectionArn"`
	ConnectionState string `json:"ConnectionState"`
}

// APIDestination represents an EventBridge API destination.
type APIDestination struct {
	Name                         string    `json:"name"`
	Arn                          string    `json:"arn"`
	ConnectionArn                string    `json:"connectionArn"`
	InvocationEndpoint           string    `json:"invocationEndpoint"`
	HTTPMethod                   string    `json:"httpMethod"`
	InvocationRateLimitPerSecond int32     `json:"invocationRateLimitPerSecond"`
	APIDestinationState          string    `json:"apiDestinationState"`
	CreationTime                 time.Time `json:"creationTime"`
	LastModifiedTime             time.Time `json:"lastModifiedTime"`
}

// CreateAPIDestinationRequest is the request for CreateApiDestination.
type CreateAPIDestinationRequest struct {
	Name                         string `json:"Name"`
	ConnectionArn                string `json:"ConnectionArn"`
	InvocationEndpoint           string `json:"InvocationEndpoint"`
	HTTPMethod                   string `json:"HttpMethod"`
	InvocationRateLimitPerSecond int32  `json:"InvocationRateLimitPerSecond,omitempty"`
	Description                  string `json:"Description,omitempty"`
}

// CreateAPIDestinationResponse is the response for CreateApiDestination.
type CreateAPIDestinationResponse struct {
	APIDestinationArn   string  `json:"ApiDestinationArn"`
	APIDestinationState string  `json:"ApiDestinationState"`
	CreationTime        float64 `json:"CreationTime"`
	LastModifiedTime    float64 `json:"LastModifiedTime"`
}

// DescribeAPIDestinationRequest is the request for DescribeApiDestination.
type DescribeAPIDestinationRequest struct {
	Name string `json:"Name"`
}

// DescribeAPIDestinationResponse is the response for DescribeApiDestination.
type DescribeAPIDestinationResponse struct {
	Name                         string  `json:"Name"`
	APIDestinationArn            string  `json:"ApiDestinationArn"`
	ConnectionArn                string  `json:"ConnectionArn"`
	InvocationEndpoint           string  `json:"InvocationEndpoint"`
	HTTPMethod                   string  `json:"HttpMethod"`
	InvocationRateLimitPerSecond int32   `json:"InvocationRateLimitPerSecond"`
	APIDestinationState          string  `json:"ApiDestinationState"`
	CreationTime                 float64 `json:"CreationTime"`
	LastModifiedTime             float64 `json:"LastModifiedTime"`
}

// DeleteAPIDestinationRequest is the request for DeleteApiDestination.
type DeleteAPIDestinationRequest struct {
	Name string `json:"Name"`
}

// DeleteAPIDestinationResponse is the response for DeleteApiDestination.
type DeleteAPIDestinationResponse struct{}

// HTTPParameters represents HTTP parameters in a target.
type HTTPParameters struct {
	PathParameterValues   []string          `json:"PathParameterValues,omitempty"`
	HeaderParameters      map[string]string `json:"HeaderParameters,omitempty"`
	QueryStringParameters map[string]string `json:"QueryStringParameters,omitempty"`
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

// Tag represents an EventBridge resource tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// ListTagsForResourceRequest is the request for ListTagsForResource.
type ListTagsForResourceRequest struct {
	ResourceARN string `json:"ResourceARN"`
}

// ListTagsForResourceResponse is the response for ListTagsForResource.
type ListTagsForResourceResponse struct {
	Tags []Tag `json:"Tags"`
}

// TagResourceRequest is the request for TagResource.
type TagResourceRequest struct {
	ResourceARN string `json:"ResourceARN"`
	Tags        []Tag  `json:"Tags"`
}

// TagResourceResponse is the response for TagResource.
type TagResourceResponse struct{}

// UntagResourceRequest is the request for UntagResource.
type UntagResourceRequest struct {
	ResourceARN string   `json:"ResourceARN"`
	TagKeys     []string `json:"TagKeys"`
}

// UntagResourceResponse is the response for UntagResource.
type UntagResourceResponse struct{}
