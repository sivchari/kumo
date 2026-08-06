// Package xray provides AWS X-Ray service emulation for kumo.
package xray

import (
	"encoding/json"
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// AWSTimestamp is a time.Time that marshals to/from Unix timestamp (float64).
// AWS APIs use Unix timestamps in JSON requests and responses.
type AWSTimestamp struct {
	time.Time
}

// MarshalJSON implements json.Marshaler.
func (t AWSTimestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return json.Marshal(nil) //nolint:wrapcheck // MarshalJSON interface requirement
	}

	return json.Marshal(float64(t.Unix()) + float64(t.Nanosecond())/1e9) //nolint:wrapcheck // MarshalJSON interface requirement
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *AWSTimestamp) UnmarshalJSON(data []byte) error {
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err //nolint:wrapcheck // UnmarshalJSON interface requirement
	}

	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	t.Time = time.Unix(sec, nsec)

	return nil
}

// Ptr returns a pointer to the AWSTimestamp.
func (t AWSTimestamp) Ptr() *AWSTimestamp {
	if t.IsZero() {
		return nil
	}

	return &t
}

// ToTime returns the underlying time.Time.
func (t AWSTimestamp) ToTime() time.Time {
	return t.Time
}

// TraceSegment represents a segment in a trace.
type TraceSegment struct {
	ID       string
	Document string
}

// TraceSummary represents a trace summary.
type TraceSummary struct {
	ID                string
	Duration          float64
	ResponseTime      float64
	HasFault          bool
	HasError          bool
	HasThrottle       bool
	IsPartial         bool
	HTTP              *HTTPInfo
	Annotations       map[string][]ValueWithAnnotation
	Users             []TraceUser
	ServiceIDs        []ServiceID
	EntryPoint        *ServiceID
	FaultRootCauses   []FaultRootCause
	ErrorRootCauses   []ErrorRootCause
	AvailabilityZones []AvailabilityZone
	InstanceIDs       []InstanceID
	ResourceARNs      []ResourceARN
	MatchedEventTime  *time.Time
	Revision          int32
}

// HTTPInfo contains HTTP information for a trace.
type HTTPInfo struct {
	HTTPMethod string `json:"HttpMethod,omitempty"`
	HTTPStatus int32  `json:"HttpStatus,omitempty"`
	ClientIP   string `json:"ClientIp,omitempty"`
	UserAgent  string `json:"UserAgent,omitempty"`
	HTTPURL    string `json:"HttpURL,omitempty"`
}

// ValueWithAnnotation represents a value with annotation.
type ValueWithAnnotation struct {
	AnnotationValue *AnnotationValue `json:"AnnotationValue,omitempty"`
	ServiceIDs      []ServiceID      `json:"ServiceIds,omitempty"`
}

// AnnotationValue represents an annotation value.
type AnnotationValue struct {
	NumberValue  *float64 `json:"NumberValue,omitempty"`
	BooleanValue *bool    `json:"BooleanValue,omitempty"`
	StringValue  *string  `json:"StringValue,omitempty"`
}

// TraceUser represents a trace user.
type TraceUser struct {
	UserName   string      `json:"UserName,omitempty"`
	ServiceIDs []ServiceID `json:"ServiceIds,omitempty"`
}

// ServiceID represents a service ID.
type ServiceID struct {
	Name      string   `json:"Name,omitempty"`
	Names     []string `json:"Names,omitempty"`
	AccountID string   `json:"AccountId,omitempty"`
	Type      string   `json:"Type,omitempty"`
}

// FaultRootCause represents a fault root cause.
type FaultRootCause struct {
	Services        []FaultRootCauseService `json:"Services,omitempty"`
	ClientImpacting bool                    `json:"ClientImpacting,omitempty"`
}

// FaultRootCauseService represents a fault root cause service.
type FaultRootCauseService struct {
	Name       string   `json:"Name,omitempty"`
	Names      []string `json:"Names,omitempty"`
	Type       string   `json:"Type,omitempty"`
	AccountID  string   `json:"AccountId,omitempty"`
	EntityPath []string `json:"EntityPath,omitempty"`
	Inferred   bool     `json:"Inferred,omitempty"`
}

// ErrorRootCause represents an error root cause.
type ErrorRootCause struct {
	Services        []ErrorRootCauseService `json:"Services,omitempty"`
	ClientImpacting bool                    `json:"ClientImpacting,omitempty"`
}

// ErrorRootCauseService represents an error root cause service.
type ErrorRootCauseService struct {
	Name       string   `json:"Name,omitempty"`
	Names      []string `json:"Names,omitempty"`
	Type       string   `json:"Type,omitempty"`
	AccountID  string   `json:"AccountId,omitempty"`
	EntityPath []string `json:"EntityPath,omitempty"`
	Inferred   bool     `json:"Inferred,omitempty"`
}

// AvailabilityZone represents an availability zone.
type AvailabilityZone struct {
	Name string `json:"Name,omitempty"`
}

// InstanceID represents an instance ID.
type InstanceID struct {
	ID string `json:"Id,omitempty"`
}

// ResourceARN represents a resource ARN.
type ResourceARN struct {
	ARN string `json:"ARN,omitempty"`
}

// Trace represents a full trace.
type Trace struct {
	ID            string
	Duration      float64
	LimitExceeded bool
	Segments      []*Segment
}

// Segment represents a trace segment.
type Segment struct {
	ID       string
	Document string
}

// Group represents an X-Ray group.
type Group struct {
	GroupName             string
	GroupARN              string
	FilterExpression      string
	InsightsConfiguration *InsightsConfiguration
}

// InsightsConfiguration represents insights configuration.
type InsightsConfiguration struct {
	InsightsEnabled      bool `json:"InsightsEnabled,omitempty"`
	NotificationsEnabled bool `json:"NotificationsEnabled,omitempty"`
}

// ServiceNode represents a service in service graph.
type ServiceNode struct {
	ReferenceID           int32            `json:"ReferenceId,omitempty"`
	Name                  string           `json:"Name,omitempty"`
	Names                 []string         `json:"Names,omitempty"`
	Root                  bool             `json:"Root,omitempty"`
	AccountID             string           `json:"AccountId,omitempty"`
	Type                  string           `json:"Type,omitempty"`
	State                 string           `json:"State,omitempty"`
	StartTime             *time.Time       `json:"StartTime,omitempty"`
	EndTime               *time.Time       `json:"EndTime,omitempty"`
	Edges                 []Edge           `json:"Edges,omitempty"`
	SummaryStatistics     *ServiceStats    `json:"SummaryStatistics,omitempty"`
	DurationHistogram     []HistogramEntry `json:"DurationHistogram,omitempty"`
	ResponseTimeHistogram []HistogramEntry `json:"ResponseTimeHistogram,omitempty"`
}

// Edge represents an edge in service graph.
type Edge struct {
	ReferenceID           int32            `json:"ReferenceId,omitempty"`
	StartTime             *time.Time       `json:"StartTime,omitempty"`
	EndTime               *time.Time       `json:"EndTime,omitempty"`
	SummaryStatistics     *EdgeStats       `json:"SummaryStatistics,omitempty"`
	ResponseTimeHistogram []HistogramEntry `json:"ResponseTimeHistogram,omitempty"`
	Aliases               []Alias          `json:"Aliases,omitempty"`
}

// ServiceStats represents service statistics.
type ServiceStats struct {
	OkCount           int64       `json:"OkCount,omitempty"`
	ErrorStatistics   *ErrorStats `json:"ErrorStatistics,omitempty"`
	FaultStatistics   *FaultStats `json:"FaultStatistics,omitempty"`
	TotalCount        int64       `json:"TotalCount,omitempty"`
	TotalResponseTime float64     `json:"TotalResponseTime,omitempty"`
}

// EdgeStats represents edge statistics.
type EdgeStats struct {
	OkCount           int64       `json:"OkCount,omitempty"`
	ErrorStatistics   *ErrorStats `json:"ErrorStatistics,omitempty"`
	FaultStatistics   *FaultStats `json:"FaultStatistics,omitempty"`
	TotalCount        int64       `json:"TotalCount,omitempty"`
	TotalResponseTime float64     `json:"TotalResponseTime,omitempty"`
}

// ErrorStats represents error statistics.
type ErrorStats struct {
	ThrottleCount int64 `json:"ThrottleCount,omitempty"`
	OtherCount    int64 `json:"OtherCount,omitempty"`
	TotalCount    int64 `json:"TotalCount,omitempty"`
}

// FaultStats represents fault statistics.
type FaultStats struct {
	OtherCount int64 `json:"OtherCount,omitempty"`
	TotalCount int64 `json:"TotalCount,omitempty"`
}

// HistogramEntry represents a histogram entry.
type HistogramEntry struct {
	Value float64 `json:"Value,omitempty"`
	Count int32   `json:"Count,omitempty"`
}

// Alias represents an alias.
type Alias struct {
	Name  string   `json:"Name,omitempty"`
	Names []string `json:"Names,omitempty"`
	Type  string   `json:"Type,omitempty"`
}

// PutTraceSegmentsInput is the request for PutTraceSegments.
type PutTraceSegmentsInput struct {
	TraceSegmentDocuments []string `json:"TraceSegmentDocuments"`
}

// PutTraceSegmentsOutput is the response for PutTraceSegments.
type PutTraceSegmentsOutput struct {
	UnprocessedTraceSegments []UnprocessedTraceSegment `json:"UnprocessedTraceSegments,omitempty"`
}

// UnprocessedTraceSegment represents an unprocessed segment.
type UnprocessedTraceSegment struct {
	ID        string `json:"Id,omitempty"`
	ErrorCode string `json:"ErrorCode,omitempty"`
	Message   string `json:"Message,omitempty"`
}

// GetTraceSummariesInput is the request for GetTraceSummaries.
type GetTraceSummariesInput struct {
	StartTime        *AWSTimestamp `json:"StartTime"`
	EndTime          *AWSTimestamp `json:"EndTime"`
	TimeRangeType    string        `json:"TimeRangeType,omitempty"`
	Sampling         bool          `json:"Sampling,omitempty"`
	SamplingStrategy string        `json:"SamplingStrategy,omitempty"`
	FilterExpression string        `json:"FilterExpression,omitempty"`
	NextToken        string        `json:"NextToken,omitempty"`
}

// GetTraceSummariesOutput is the response for GetTraceSummaries.
type GetTraceSummariesOutput struct {
	TraceSummaries       []TraceSummaryResponse `json:"TraceSummaries,omitempty"`
	ApproximateTime      *time.Time             `json:"ApproximateTime,omitempty"`
	TracesProcessedCount int64                  `json:"TracesProcessedCount,omitempty"`
	NextToken            string                 `json:"NextToken,omitempty"`
}

// TraceSummaryResponse represents a trace summary in API responses.
type TraceSummaryResponse struct {
	ID                string                           `json:"Id,omitempty"`
	Duration          float64                          `json:"Duration,omitempty"`
	ResponseTime      float64                          `json:"ResponseTime,omitempty"`
	HasFault          bool                             `json:"HasFault,omitempty"`
	HasError          bool                             `json:"HasError,omitempty"`
	HasThrottle       bool                             `json:"HasThrottle,omitempty"`
	IsPartial         bool                             `json:"IsPartial,omitempty"`
	HTTP              *HTTPInfo                        `json:"Http,omitempty"`
	Annotations       map[string][]ValueWithAnnotation `json:"Annotations,omitempty"`
	Users             []TraceUser                      `json:"Users,omitempty"`
	ServiceIDs        []ServiceID                      `json:"ServiceIds,omitempty"`
	EntryPoint        *ServiceID                       `json:"EntryPoint,omitempty"`
	FaultRootCauses   []FaultRootCause                 `json:"FaultRootCauses,omitempty"`
	ErrorRootCauses   []ErrorRootCause                 `json:"ErrorRootCauses,omitempty"`
	AvailabilityZones []AvailabilityZone               `json:"AvailabilityZones,omitempty"`
	InstanceIDs       []InstanceID                     `json:"InstanceIds,omitempty"`
	ResourceARNs      []ResourceARN                    `json:"ResourceARNs,omitempty"`
	MatchedEventTime  *time.Time                       `json:"MatchedEventTime,omitempty"`
	Revision          int32                            `json:"Revision,omitempty"`
}

// BatchGetTracesInput is the request for BatchGetTraces.
type BatchGetTracesInput struct {
	TraceIDs  []string `json:"TraceIds"`
	NextToken string   `json:"NextToken,omitempty"`
}

// BatchGetTracesOutput is the response for BatchGetTraces.
type BatchGetTracesOutput struct {
	Traces              []TraceResponse `json:"Traces,omitempty"`
	UnprocessedTraceIDs []string        `json:"UnprocessedTraceIds,omitempty"`
	NextToken           string          `json:"NextToken,omitempty"`
}

// TraceResponse represents a trace in API responses.
type TraceResponse struct {
	ID            string            `json:"Id,omitempty"`
	Duration      float64           `json:"Duration,omitempty"`
	LimitExceeded bool              `json:"LimitExceeded,omitempty"`
	Segments      []SegmentResponse `json:"Segments,omitempty"`
}

// SegmentResponse represents a segment in API responses.
type SegmentResponse struct {
	ID       string `json:"Id,omitempty"`
	Document string `json:"Document,omitempty"`
}

// GetServiceGraphInput is the request for GetServiceGraph.
type GetServiceGraphInput struct {
	StartTime *AWSTimestamp `json:"StartTime"`
	EndTime   *AWSTimestamp `json:"EndTime"`
	GroupName string        `json:"GroupName,omitempty"`
	GroupARN  string        `json:"GroupARN,omitempty"`
	NextToken string        `json:"NextToken,omitempty"`
}

// GetServiceGraphOutput is the response for GetServiceGraph.
type GetServiceGraphOutput struct {
	StartTime                *AWSTimestamp `json:"StartTime,omitempty"`
	EndTime                  *AWSTimestamp `json:"EndTime,omitempty"`
	Services                 []ServiceNode `json:"Services,omitempty"`
	ContainsOldGroupVersions bool          `json:"ContainsOldGroupVersions,omitempty"`
	NextToken                string        `json:"NextToken,omitempty"`
}

// CreateGroupInput is the request for CreateGroup.
type CreateGroupInput struct {
	GroupName             string                 `json:"GroupName"`
	FilterExpression      string                 `json:"FilterExpression,omitempty"`
	InsightsConfiguration *InsightsConfiguration `json:"InsightsConfiguration,omitempty"`
	Tags                  []Tag                  `json:"Tags,omitempty"`
}

// CreateGroupOutput is the response for CreateGroup.
type CreateGroupOutput struct {
	Group *GroupResponse `json:"Group,omitempty"`
}

// GroupResponse represents a group in API responses.
type GroupResponse struct {
	GroupName             string                 `json:"GroupName,omitempty"`
	GroupARN              string                 `json:"GroupARN,omitempty"`
	FilterExpression      string                 `json:"FilterExpression,omitempty"`
	InsightsConfiguration *InsightsConfiguration `json:"InsightsConfiguration,omitempty"`
}

// DeleteGroupInput is the request for DeleteGroup.
type DeleteGroupInput struct {
	GroupName string `json:"GroupName,omitempty"`
	GroupARN  string `json:"GroupARN,omitempty"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// ErrorResponse represents an X-Ray error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error represents an X-Ray error.
type Error = service.CodedError
