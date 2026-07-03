// Package dynamodbstreams provides a mock implementation of AWS DynamoDB Streams.
package dynamodbstreams

// DescribeStreamRequest is the request for DescribeStream.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type DescribeStreamRequest struct {
	StreamArn             string `json:"StreamArn"`
	Limit                 int32  `json:"Limit,omitempty"`
	ExclusiveStartShardID string `json:"ExclusiveStartShardId,omitempty"`
}

// DescribeStreamResponse is the response for DescribeStream.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type DescribeStreamResponse struct {
	StreamDescription StreamDescription `json:"StreamDescription"`
}

// StreamDescription contains stream details.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type StreamDescription struct {
	StreamArn               string      `json:"StreamArn"`
	StreamLabel             string      `json:"StreamLabel"`
	StreamStatus            string      `json:"StreamStatus"`
	StreamViewType          string      `json:"StreamViewType"`
	TableName               string      `json:"TableName"`
	KeySchema               []KeySchema `json:"KeySchema"`
	Shards                  []Shard     `json:"Shards"`
	CreationRequestDateTime float64     `json:"CreationRequestDateTime"`
	LastEvaluatedShardID    string      `json:"LastEvaluatedShardId,omitempty"`
}

// KeySchema represents a key schema element.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type KeySchema struct {
	AttributeName string `json:"AttributeName"`
	KeyType       string `json:"KeyType"`
}

// Shard represents a shard in a DynamoDB stream.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type Shard struct {
	ShardID             string              `json:"ShardId"`
	ParentShardID       string              `json:"ParentShardId,omitempty"`
	SequenceNumberRange SequenceNumberRange `json:"SequenceNumberRange"`
}

// SequenceNumberRange represents the range of sequence numbers for a shard.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type SequenceNumberRange struct {
	StartingSequenceNumber string  `json:"StartingSequenceNumber"`
	EndingSequenceNumber   *string `json:"EndingSequenceNumber,omitempty"`
}

// GetShardIteratorRequest is the request for GetShardIterator.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type GetShardIteratorRequest struct {
	StreamArn         string `json:"StreamArn"`
	ShardID           string `json:"ShardId"`
	ShardIteratorType string `json:"ShardIteratorType"`
	SequenceNumber    string `json:"SequenceNumber,omitempty"`
}

// GetShardIteratorResponse is the response for GetShardIterator.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type GetShardIteratorResponse struct {
	ShardIterator string `json:"ShardIterator,omitempty"`
}

// GetRecordsRequest is the request for GetRecords.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type GetRecordsRequest struct {
	ShardIterator string `json:"ShardIterator"`
	Limit         int32  `json:"Limit,omitempty"`
}

// GetRecordsResponse is the response for GetRecords.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type GetRecordsResponse struct {
	Records           []RecordOutput `json:"Records"`
	NextShardIterator *string        `json:"NextShardIterator,omitempty"`
}

// RecordOutput is the output representation of a stream record.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type RecordOutput struct {
	EventID      string              `json:"eventID"`
	EventName    string              `json:"eventName"`
	EventVersion string              `json:"eventVersion"`
	EventSource  string              `json:"eventSource"`
	AwsRegion    string              `json:"awsRegion"`
	Dynamodb     *StreamRecordOutput `json:"dynamodb"`
}

// StreamRecordOutput represents the DynamoDB-specific portion of a stream record.
//
//nolint:tagliatelle // AWS JSON protocol uses PascalCase field names.
type StreamRecordOutput struct {
	ApproximateCreationDateTime float64                    `json:"ApproximateCreationDateTime"`
	Keys                        map[string]AttributeOutput `json:"Keys"`
	NewImage                    map[string]AttributeOutput `json:"NewImage,omitempty"`
	OldImage                    map[string]AttributeOutput `json:"OldImage,omitempty"`
	SequenceNumber              string                     `json:"SequenceNumber"`
	SizeBytes                   int64                      `json:"SizeBytes"`
	StreamViewType              string                     `json:"StreamViewType"`
}

// AttributeOutput is the JSON representation of a DynamoDB attribute value.
// It wraps the streams.AttributeValue for JSON serialization.
type AttributeOutput map[string]any

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Type    string `json:"__type"` //nolint:tagliatelle // AWS error format
	Message string `json:"message"`
}
