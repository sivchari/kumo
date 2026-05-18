package kinesis

import "time"

// StreamStatus represents the status of a Kinesis stream.
type StreamStatus string

// Stream status constants.
const (
	StreamStatusCreating StreamStatus = "CREATING"
	StreamStatusDeleting StreamStatus = "DELETING"
	StreamStatusActive   StreamStatus = "ACTIVE"
	StreamStatusUpdating StreamStatus = "UPDATING"
)

// ShardIteratorType represents the type of shard iterator.
type ShardIteratorType string

// Shard iterator type constants.
const (
	ShardIteratorTypeAtSequenceNumber    ShardIteratorType = "AT_SEQUENCE_NUMBER"
	ShardIteratorTypeAfterSequenceNumber ShardIteratorType = "AFTER_SEQUENCE_NUMBER"
	ShardIteratorTypeTrimHorizon         ShardIteratorType = "TRIM_HORIZON"
	ShardIteratorTypeLatest              ShardIteratorType = "LATEST"
	ShardIteratorTypeAtTimestamp         ShardIteratorType = "AT_TIMESTAMP"
)

// Stream represents a Kinesis stream.
type Stream struct {
	StreamName              string
	StreamARN               string
	StreamStatus            StreamStatus
	ShardCount              int32
	RetentionPeriodHours    int32
	StreamCreationTimestamp time.Time
	EnhancedMonitoring      []EnhancedMetrics
	EncryptionType          string
	KeyID                   string
	StreamModeDetails       *StreamModeDetails
	HasMoreShards           bool
	OpenShardCount          int32
	ConsumerCount           int32
}

// StreamModeDetails contains details about the stream mode.
type StreamModeDetails struct {
	StreamMode string `json:"StreamMode"`
}

// EnhancedMetrics represents enhanced monitoring metrics.
type EnhancedMetrics struct {
	ShardLevelMetrics []string `json:"ShardLevelMetrics"`
}

// Shard represents a Kinesis shard.
type Shard struct {
	ShardID               string
	ParentShardID         string
	AdjacentParentShardID string
	HashKeyRange          HashKeyRange
	SequenceNumberRange   SequenceNumberRange
}

// HashKeyRange represents the range of hash keys for a shard.
type HashKeyRange struct {
	StartingHashKey string `json:"StartingHashKey"`
	EndingHashKey   string `json:"EndingHashKey"`
}

// SequenceNumberRange represents the range of sequence numbers for a shard.
type SequenceNumberRange struct {
	StartingSequenceNumber string  `json:"StartingSequenceNumber"`
	EndingSequenceNumber   *string `json:"EndingSequenceNumber,omitempty"`
}

// Record represents a Kinesis record.
type Record struct {
	Data                        []byte
	PartitionKey                string
	SequenceNumber              string
	ApproximateArrivalTimestamp time.Time
	EncryptionType              string
}

// CreateStreamRequest is the request for CreateStream.
type CreateStreamRequest struct {
	StreamName        string             `json:"StreamName"`
	ShardCount        *int32             `json:"ShardCount,omitempty"`
	StreamModeDetails *StreamModeDetails `json:"StreamModeDetails,omitempty"`
}

// CreateStreamResponse is the response for CreateStream.
type CreateStreamResponse struct{}

// DeleteStreamRequest is the request for DeleteStream.
type DeleteStreamRequest struct {
	StreamName              string `json:"StreamName"`
	StreamARN               string `json:"StreamARN,omitempty"`
	EnforceConsumerDeletion bool   `json:"EnforceConsumerDeletion,omitempty"`
}

// DeleteStreamResponse is the response for DeleteStream.
type DeleteStreamResponse struct{}

// DescribeStreamRequest is the request for DescribeStream.
type DescribeStreamRequest struct {
	StreamName            string `json:"StreamName,omitempty"`
	StreamARN             string `json:"StreamARN,omitempty"`
	Limit                 int32  `json:"Limit,omitempty"`
	ExclusiveStartShardID string `json:"ExclusiveStartShardId,omitempty"`
}

// DescribeStreamResponse is the response for DescribeStream.
type DescribeStreamResponse struct {
	StreamDescription StreamDescription `json:"StreamDescription"`
}

// DescribeStreamSummaryRequest is the request for DescribeStreamSummary.
type DescribeStreamSummaryRequest struct {
	StreamName string `json:"StreamName,omitempty"`
	StreamARN  string `json:"StreamARN,omitempty"`
}

// DescribeStreamSummaryResponse is the response for DescribeStreamSummary.
type DescribeStreamSummaryResponse struct {
	StreamDescriptionSummary StreamDescriptionSummary `json:"StreamDescriptionSummary"`
}

// StreamDescriptionSummary is the summary description of a stream.
type StreamDescriptionSummary struct {
	StreamName              string             `json:"StreamName"`
	StreamARN               string             `json:"StreamARN"`
	StreamStatus            string             `json:"StreamStatus"`
	StreamModeDetails       *StreamModeDetails `json:"StreamModeDetails,omitempty"`
	RetentionPeriodHours    int32              `json:"RetentionPeriodHours"`
	StreamCreationTimestamp float64            `json:"StreamCreationTimestamp"`
	EnhancedMonitoring      []EnhancedMetrics  `json:"EnhancedMonitoring"`
	EncryptionType          string             `json:"EncryptionType,omitempty"`
	KeyID                   string             `json:"KeyId,omitempty"`
	OpenShardCount          int32              `json:"OpenShardCount"`
	ConsumerCount           int32              `json:"ConsumerCount"`
}

// StreamDescription contains stream details.
type StreamDescription struct {
	StreamName              string             `json:"StreamName"`
	StreamARN               string             `json:"StreamARN"`
	StreamStatus            string             `json:"StreamStatus"`
	StreamModeDetails       *StreamModeDetails `json:"StreamModeDetails,omitempty"`
	Shards                  []ShardOutput      `json:"Shards"`
	HasMoreShards           bool               `json:"HasMoreShards"`
	RetentionPeriodHours    int32              `json:"RetentionPeriodHours"`
	StreamCreationTimestamp float64            `json:"StreamCreationTimestamp"`
	EnhancedMonitoring      []EnhancedMetrics  `json:"EnhancedMonitoring"`
	EncryptionType          string             `json:"EncryptionType,omitempty"`
	KeyID                   string             `json:"KeyId,omitempty"`
	OpenShardCount          int32              `json:"OpenShardCount,omitempty"`
	ConsumerCount           int32              `json:"ConsumerCount,omitempty"`
}

// ShardOutput is the output representation of a shard.
type ShardOutput struct {
	ShardID               string              `json:"ShardId"`
	ParentShardID         string              `json:"ParentShardId,omitempty"`
	AdjacentParentShardID string              `json:"AdjacentParentShardId,omitempty"`
	HashKeyRange          HashKeyRange        `json:"HashKeyRange"`
	SequenceNumberRange   SequenceNumberRange `json:"SequenceNumberRange"`
}

// ListStreamsRequest is the request for ListStreams.
type ListStreamsRequest struct {
	ExclusiveStartStreamName string `json:"ExclusiveStartStreamName,omitempty"`
	Limit                    int32  `json:"Limit,omitempty"`
	NextToken                string `json:"NextToken,omitempty"`
}

// ListStreamsResponse is the response for ListStreams.
type ListStreamsResponse struct {
	StreamNames     []string        `json:"StreamNames"`
	HasMoreStreams  bool            `json:"HasMoreStreams"`
	NextToken       string          `json:"NextToken,omitempty"`
	StreamSummaries []StreamSummary `json:"StreamSummaries,omitempty"`
}

// StreamSummary contains a summary of a stream.
type StreamSummary struct {
	StreamName              string             `json:"StreamName"`
	StreamARN               string             `json:"StreamARN"`
	StreamStatus            string             `json:"StreamStatus"`
	StreamModeDetails       *StreamModeDetails `json:"StreamModeDetails,omitempty"`
	StreamCreationTimestamp float64            `json:"StreamCreationTimestamp"`
}

// ListShardsRequest is the request for ListShards.
type ListShardsRequest struct {
	StreamName              string       `json:"StreamName,omitempty"`
	StreamARN               string       `json:"StreamARN,omitempty"`
	NextToken               string       `json:"NextToken,omitempty"`
	ExclusiveStartShardID   string       `json:"ExclusiveStartShardId,omitempty"`
	MaxResults              int32        `json:"MaxResults,omitempty"`
	StreamCreationTimestamp float64      `json:"StreamCreationTimestamp,omitempty"`
	ShardFilter             *ShardFilter `json:"ShardFilter,omitempty"`
}

// ShardFilter contains filtering options for listing shards.
type ShardFilter struct {
	Type      string  `json:"Type"`
	ShardID   string  `json:"ShardId,omitempty"`
	Timestamp float64 `json:"Timestamp,omitempty"`
}

// ListShardsResponse is the response for ListShards.
type ListShardsResponse struct {
	Shards    []ShardOutput `json:"Shards"`
	NextToken string        `json:"NextToken,omitempty"`
}

// PutRecordRequest is the request for PutRecord.
type PutRecordRequest struct {
	StreamName                string `json:"StreamName,omitempty"`
	StreamARN                 string `json:"StreamARN,omitempty"`
	Data                      []byte `json:"Data"`
	PartitionKey              string `json:"PartitionKey"`
	ExplicitHashKey           string `json:"ExplicitHashKey,omitempty"`
	SequenceNumberForOrdering string `json:"SequenceNumberForOrdering,omitempty"`
}

// PutRecordResponse is the response for PutRecord.
type PutRecordResponse struct {
	ShardID        string `json:"ShardId"`
	SequenceNumber string `json:"SequenceNumber"`
	EncryptionType string `json:"EncryptionType,omitempty"`
}

// PutRecordsRequest is the request for PutRecords.
type PutRecordsRequest struct {
	StreamName string                   `json:"StreamName,omitempty"`
	StreamARN  string                   `json:"StreamARN,omitempty"`
	Records    []PutRecordsRequestEntry `json:"Records"`
}

// PutRecordsRequestEntry is a single record in PutRecords request.
type PutRecordsRequestEntry struct {
	Data            []byte `json:"Data"`
	PartitionKey    string `json:"PartitionKey"`
	ExplicitHashKey string `json:"ExplicitHashKey,omitempty"`
}

// PutRecordsResponse is the response for PutRecords.
type PutRecordsResponse struct {
	FailedRecordCount int32                   `json:"FailedRecordCount"`
	Records           []PutRecordsResultEntry `json:"Records"`
	EncryptionType    string                  `json:"EncryptionType,omitempty"`
}

// PutRecordsResultEntry is a single record result in PutRecords response.
type PutRecordsResultEntry struct {
	ShardID        string `json:"ShardId,omitempty"`
	SequenceNumber string `json:"SequenceNumber,omitempty"`
	ErrorCode      string `json:"ErrorCode,omitempty"`
	ErrorMessage   string `json:"ErrorMessage,omitempty"`
}

// GetShardIteratorRequest is the request for GetShardIterator.
type GetShardIteratorRequest struct {
	StreamName             string  `json:"StreamName,omitempty"`
	StreamARN              string  `json:"StreamARN,omitempty"`
	ShardID                string  `json:"ShardId"`
	ShardIteratorType      string  `json:"ShardIteratorType"`
	StartingSequenceNumber string  `json:"StartingSequenceNumber,omitempty"`
	Timestamp              float64 `json:"Timestamp,omitempty"`
}

// GetShardIteratorResponse is the response for GetShardIterator.
type GetShardIteratorResponse struct {
	ShardIterator string `json:"ShardIterator"`
}

// GetRecordsRequest is the request for GetRecords.
type GetRecordsRequest struct {
	ShardIterator string `json:"ShardIterator"`
	Limit         int32  `json:"Limit,omitempty"`
	StreamARN     string `json:"StreamARN,omitempty"`
}

// GetRecordsResponse is the response for GetRecords.
type GetRecordsResponse struct {
	Records            []RecordOutput `json:"Records"`
	NextShardIterator  string         `json:"NextShardIterator,omitempty"`
	MillisBehindLatest int64          `json:"MillisBehindLatest"`
	ChildShards        []ChildShard   `json:"ChildShards,omitempty"`
}

// RecordOutput is the output representation of a record.
type RecordOutput struct {
	Data                        string  `json:"Data"`
	PartitionKey                string  `json:"PartitionKey"`
	SequenceNumber              string  `json:"SequenceNumber"`
	ApproximateArrivalTimestamp float64 `json:"ApproximateArrivalTimestamp"`
	EncryptionType              string  `json:"EncryptionType,omitempty"`
}

// ChildShard represents a child shard.
type ChildShard struct {
	ShardID      string       `json:"ShardId"`
	ParentShards []string     `json:"ParentShards"`
	HashKeyRange HashKeyRange `json:"HashKeyRange"`
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
