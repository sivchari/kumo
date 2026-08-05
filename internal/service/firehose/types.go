// Package firehose provides a mock implementation of Amazon Data Firehose.
package firehose

import "time"

// DeliveryStreamStatus represents the status of a delivery stream.
type DeliveryStreamStatus string

// Delivery stream status constants.
const (
	DeliveryStreamStatusCreating       DeliveryStreamStatus = "CREATING"
	DeliveryStreamStatusCreatingFailed DeliveryStreamStatus = "CREATING_FAILED"
	DeliveryStreamStatusDeleting       DeliveryStreamStatus = "DELETING"
	DeliveryStreamStatusDeletingFailed DeliveryStreamStatus = "DELETING_FAILED"
	DeliveryStreamStatusActive         DeliveryStreamStatus = "ACTIVE"
	DeliveryStreamStatusSuspended      DeliveryStreamStatus = "SUSPENDED"
)

// DeliveryStreamType represents the type of delivery stream.
type DeliveryStreamType string

// Delivery stream type constants.
const (
	DeliveryStreamTypeDirectPut               DeliveryStreamType = "DirectPut"
	DeliveryStreamTypeKinesisStreamAsSource   DeliveryStreamType = "KinesisStreamAsSource"
	DeliveryStreamTypeMSKAsSource             DeliveryStreamType = "MSKAsSource"
	DeliveryStreamTypeDatabaseAsSource        DeliveryStreamType = "DatabaseAsSource"
	DeliveryStreamTypeDirectPutFromMSKConnect DeliveryStreamType = "DirectPutFromMSKConnect"
)

// DestinationType represents the type of destination.
type DestinationType string

// Destination type constants.
const (
	DestinationTypeS3            DestinationType = "S3"
	DestinationTypeExtendedS3    DestinationType = "ExtendedS3"
	DestinationTypeRedshift      DestinationType = "Redshift"
	DestinationTypeElasticsearch DestinationType = "Elasticsearch"
	DestinationTypeSplunk        DestinationType = "Splunk"
	DestinationTypeHTTPEndpoint  DestinationType = "HttpEndpoint"
	DestinationTypeSnowflake     DestinationType = "Snowflake"
	DestinationTypeIceberg       DestinationType = "Iceberg"
)

// DeliveryStream represents a Firehose delivery stream.
type DeliveryStream struct {
	DeliveryStreamName   string
	DeliveryStreamARN    string
	DeliveryStreamStatus DeliveryStreamStatus
	DeliveryStreamType   DeliveryStreamType
	CreateTimestamp      time.Time
	LastUpdateTimestamp  time.Time
	VersionID            string
	Destinations         []DestinationDescription
	HasMoreDestinations  bool
	Source               *SourceDescription
}

// DestinationDescription describes a destination.
type DestinationDescription struct {
	DestinationID                    string                            `json:"DestinationId"`
	S3DestinationDescription         *S3DestinationDescription         `json:"S3DestinationDescription,omitempty"`
	ExtendedS3DestinationDescription *ExtendedS3DestinationDescription `json:"ExtendedS3DestinationDescription,omitempty"`
}

// S3DestinationDescription describes an S3 destination.
type S3DestinationDescription struct {
	BucketARN         string             `json:"BucketARN"`
	Prefix            string             `json:"Prefix,omitempty"`
	ErrorOutputPrefix string             `json:"ErrorOutputPrefix,omitempty"`
	RoleARN           string             `json:"RoleARN"`
	BufferingHints    *BufferingHints    `json:"BufferingHints,omitempty"`
	CompressionFormat string             `json:"CompressionFormat,omitempty"`
	CloudWatchLogging *CloudWatchLogging `json:"CloudWatchLoggingOptions,omitempty"`
}

// ExtendedS3DestinationDescription describes an extended S3 destination.
type ExtendedS3DestinationDescription struct {
	BucketARN         string             `json:"BucketARN"`
	Prefix            string             `json:"Prefix,omitempty"`
	ErrorOutputPrefix string             `json:"ErrorOutputPrefix,omitempty"`
	RoleARN           string             `json:"RoleARN"`
	BufferingHints    *BufferingHints    `json:"BufferingHints,omitempty"`
	CompressionFormat string             `json:"CompressionFormat,omitempty"`
	CloudWatchLogging *CloudWatchLogging `json:"CloudWatchLoggingOptions,omitempty"`
	ProcessingConfig  *ProcessingConfig  `json:"ProcessingConfiguration,omitempty"`
	S3BackupMode      string             `json:"S3BackupMode,omitempty"`
}

// BufferingHints contains buffering configuration.
type BufferingHints struct {
	SizeInMBs         int32 `json:"SizeInMBs,omitempty"`
	IntervalInSeconds int32 `json:"IntervalInSeconds,omitempty"`
}

// CloudWatchLogging contains CloudWatch logging configuration.
type CloudWatchLogging struct {
	Enabled       bool   `json:"Enabled"`
	LogGroupName  string `json:"LogGroupName,omitempty"`
	LogStreamName string `json:"LogStreamName,omitempty"`
}

// ProcessingConfig contains processing configuration.
type ProcessingConfig struct {
	Enabled    bool        `json:"Enabled"`
	Processors []Processor `json:"Processors,omitempty"`
}

// Processor represents a data processor.
type Processor struct {
	Type       string               `json:"Type"`
	Parameters []ProcessorParameter `json:"Parameters,omitempty"`
}

// ProcessorParameter represents a processor parameter.
type ProcessorParameter struct {
	ParameterName  string `json:"ParameterName"`
	ParameterValue string `json:"ParameterValue"`
}

// SourceDescription describes the source of a delivery stream.
type SourceDescription struct {
	KinesisStreamSourceDescription *KinesisStreamSourceDescription `json:"KinesisStreamSourceDescription,omitempty"`
}

// KinesisStreamSourceDescription describes a Kinesis stream source.
type KinesisStreamSourceDescription struct {
	KinesisStreamARN       string    `json:"KinesisStreamARN"`
	RoleARN                string    `json:"RoleARN"`
	DeliveryStartTimestamp time.Time `json:"DeliveryStartTimestamp"`
}

// Record represents a record to put.
type Record struct {
	Data []byte `json:"Data"`
}

// CreateDeliveryStreamInput is the input for CreateDeliveryStream.
type CreateDeliveryStreamInput struct {
	DeliveryStreamName                 string                              `json:"DeliveryStreamName"`
	DeliveryStreamType                 string                              `json:"DeliveryStreamType,omitempty"`
	S3DestinationConfiguration         *S3DestinationConfiguration         `json:"S3DestinationConfiguration,omitempty"`
	ExtendedS3DestinationConfiguration *ExtendedS3DestinationConfiguration `json:"ExtendedS3DestinationConfiguration,omitempty"`
	KinesisStreamSourceConfiguration   *KinesisStreamSourceConfiguration   `json:"KinesisStreamSourceConfiguration,omitempty"`
	Tags                               []Tag                               `json:"Tags,omitempty"`
}

// S3DestinationConfiguration is the S3 destination configuration.
type S3DestinationConfiguration struct {
	BucketARN         string             `json:"BucketARN"`
	Prefix            string             `json:"Prefix,omitempty"`
	ErrorOutputPrefix string             `json:"ErrorOutputPrefix,omitempty"`
	RoleARN           string             `json:"RoleARN"`
	BufferingHints    *BufferingHints    `json:"BufferingHints,omitempty"`
	CompressionFormat string             `json:"CompressionFormat,omitempty"`
	CloudWatchLogging *CloudWatchLogging `json:"CloudWatchLoggingOptions,omitempty"`
}

// ExtendedS3DestinationConfiguration is the extended S3 destination configuration.
type ExtendedS3DestinationConfiguration struct {
	BucketARN         string             `json:"BucketARN"`
	Prefix            string             `json:"Prefix,omitempty"`
	ErrorOutputPrefix string             `json:"ErrorOutputPrefix,omitempty"`
	RoleARN           string             `json:"RoleARN"`
	BufferingHints    *BufferingHints    `json:"BufferingHints,omitempty"`
	CompressionFormat string             `json:"CompressionFormat,omitempty"`
	CloudWatchLogging *CloudWatchLogging `json:"CloudWatchLoggingOptions,omitempty"`
	ProcessingConfig  *ProcessingConfig  `json:"ProcessingConfiguration,omitempty"`
	S3BackupMode      string             `json:"S3BackupMode,omitempty"`
}

// KinesisStreamSourceConfiguration is the Kinesis stream source configuration.
type KinesisStreamSourceConfiguration struct {
	KinesisStreamARN string `json:"KinesisStreamARN"`
	RoleARN          string `json:"RoleARN"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value,omitempty"`
}

// CreateDeliveryStreamOutput is the output for CreateDeliveryStream.
type CreateDeliveryStreamOutput struct {
	DeliveryStreamARN string `json:"DeliveryStreamARN"`
}

// DeleteDeliveryStreamInput is the input for DeleteDeliveryStream.
type DeleteDeliveryStreamInput struct {
	DeliveryStreamName string `json:"DeliveryStreamName"`
	AllowForceDelete   bool   `json:"AllowForceDelete,omitempty"`
}

// DeleteDeliveryStreamOutput is the output for DeleteDeliveryStream.
type DeleteDeliveryStreamOutput struct{}

// DescribeDeliveryStreamInput is the input for DescribeDeliveryStream.
type DescribeDeliveryStreamInput struct {
	DeliveryStreamName          string `json:"DeliveryStreamName"`
	Limit                       int32  `json:"Limit,omitempty"`
	ExclusiveStartDestinationID string `json:"ExclusiveStartDestinationId,omitempty"`
}

// DescribeDeliveryStreamOutput is the output for DescribeDeliveryStream.
type DescribeDeliveryStreamOutput struct {
	DeliveryStreamDescription DeliveryStreamDescription `json:"DeliveryStreamDescription"`
}

// DeliveryStreamDescription describes a delivery stream.
type DeliveryStreamDescription struct {
	DeliveryStreamName   string                   `json:"DeliveryStreamName"`
	DeliveryStreamARN    string                   `json:"DeliveryStreamARN"`
	DeliveryStreamStatus string                   `json:"DeliveryStreamStatus"`
	DeliveryStreamType   string                   `json:"DeliveryStreamType"`
	CreateTimestamp      float64                  `json:"CreateTimestamp"`
	LastUpdateTimestamp  float64                  `json:"LastUpdateTimestamp,omitempty"`
	VersionID            string                   `json:"VersionId"`
	Destinations         []DestinationDescription `json:"Destinations"`
	HasMoreDestinations  bool                     `json:"HasMoreDestinations"`
	Source               *SourceDescription       `json:"Source,omitempty"`
}

// ListDeliveryStreamsInput is the input for ListDeliveryStreams.
type ListDeliveryStreamsInput struct {
	DeliveryStreamType               string `json:"DeliveryStreamType,omitempty"`
	ExclusiveStartDeliveryStreamName string `json:"ExclusiveStartDeliveryStreamName,omitempty"`
	Limit                            int32  `json:"Limit,omitempty"`
}

// ListDeliveryStreamsOutput is the output for ListDeliveryStreams.
type ListDeliveryStreamsOutput struct {
	DeliveryStreamNames    []string `json:"DeliveryStreamNames"`
	HasMoreDeliveryStreams bool     `json:"HasMoreDeliveryStreams"`
}

// PutRecordInput is the input for PutRecord.
type PutRecordInput struct {
	DeliveryStreamName string `json:"DeliveryStreamName"`
	Record             Record `json:"Record"`
}

// PutRecordOutput is the output for PutRecord.
type PutRecordOutput struct {
	RecordID  string `json:"RecordId"`
	Encrypted bool   `json:"Encrypted,omitempty"`
}

// PutRecordBatchInput is the input for PutRecordBatch.
type PutRecordBatchInput struct {
	DeliveryStreamName string   `json:"DeliveryStreamName"`
	Records            []Record `json:"Records"`
}

// PutRecordBatchOutput is the output for PutRecordBatch.
type PutRecordBatchOutput struct {
	FailedPutCount   int32                         `json:"FailedPutCount"`
	Encrypted        bool                          `json:"Encrypted,omitempty"`
	RequestResponses []PutRecordBatchResponseEntry `json:"RequestResponses"`
}

// PutRecordBatchResponseEntry is a response entry for PutRecordBatch.
type PutRecordBatchResponseEntry struct {
	RecordID     string `json:"RecordId,omitempty"`
	ErrorCode    string `json:"ErrorCode,omitempty"`
	ErrorMessage string `json:"ErrorMessage,omitempty"`
}

// UpdateDestinationInput is the input for UpdateDestination.
type UpdateDestinationInput struct {
	DeliveryStreamName             string                       `json:"DeliveryStreamName"`
	CurrentDeliveryStreamVersionID string                       `json:"CurrentDeliveryStreamVersionId"`
	DestinationID                  string                       `json:"DestinationId"`
	S3DestinationUpdate            *S3DestinationUpdate         `json:"S3DestinationUpdate,omitempty"`
	ExtendedS3DestinationUpdate    *ExtendedS3DestinationUpdate `json:"ExtendedS3DestinationUpdate,omitempty"`
}

// S3DestinationUpdate is the S3 destination update configuration.
type S3DestinationUpdate struct {
	BucketARN         string             `json:"BucketARN,omitempty"`
	Prefix            string             `json:"Prefix,omitempty"`
	ErrorOutputPrefix string             `json:"ErrorOutputPrefix,omitempty"`
	RoleARN           string             `json:"RoleARN,omitempty"`
	BufferingHints    *BufferingHints    `json:"BufferingHints,omitempty"`
	CompressionFormat string             `json:"CompressionFormat,omitempty"`
	CloudWatchLogging *CloudWatchLogging `json:"CloudWatchLoggingOptions,omitempty"`
}

// ExtendedS3DestinationUpdate is the extended S3 destination update configuration.
type ExtendedS3DestinationUpdate struct {
	BucketARN         string             `json:"BucketARN,omitempty"`
	Prefix            string             `json:"Prefix,omitempty"`
	ErrorOutputPrefix string             `json:"ErrorOutputPrefix,omitempty"`
	RoleARN           string             `json:"RoleARN,omitempty"`
	BufferingHints    *BufferingHints    `json:"BufferingHints,omitempty"`
	CompressionFormat string             `json:"CompressionFormat,omitempty"`
	CloudWatchLogging *CloudWatchLogging `json:"CloudWatchLoggingOptions,omitempty"`
	ProcessingConfig  *ProcessingConfig  `json:"ProcessingConfiguration,omitempty"`
	S3BackupMode      string             `json:"S3BackupMode,omitempty"`
}

// UpdateDestinationOutput is the output for UpdateDestination.
type UpdateDestinationOutput struct{}

// ListTagsForDeliveryStreamInput is the input for ListTagsForDeliveryStream.
type ListTagsForDeliveryStreamInput struct {
	DeliveryStreamName   string `json:"DeliveryStreamName"`
	ExclusiveStartTagKey string `json:"ExclusiveStartTagKey,omitempty"`
	Limit                int32  `json:"Limit,omitempty"`
}

// ListTagsForDeliveryStreamOutput is the output for ListTagsForDeliveryStream.
type ListTagsForDeliveryStreamOutput struct {
	Tags        []Tag `json:"Tags"`
	HasMoreTags bool  `json:"HasMoreTags"`
}

// TagDeliveryStreamInput is the input for TagDeliveryStream.
type TagDeliveryStreamInput struct {
	DeliveryStreamName string `json:"DeliveryStreamName"`
	Tags               []Tag  `json:"Tags"`
}

// TagDeliveryStreamOutput is the output for TagDeliveryStream.
type TagDeliveryStreamOutput struct{}

// UntagDeliveryStreamInput is the input for UntagDeliveryStream.
type UntagDeliveryStreamInput struct {
	DeliveryStreamName string   `json:"DeliveryStreamName"`
	TagKeys            []string `json:"TagKeys"`
}

// UntagDeliveryStreamOutput is the output for UntagDeliveryStream.
type UntagDeliveryStreamOutput struct{}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error represents a service error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}

// Error codes.
const (
	errResourceNotFound   = "ResourceNotFoundException"
	errResourceInUse      = "ResourceInUseException"
	errInvalidArgument    = "InvalidArgumentException"
	errLimitExceeded      = "LimitExceededException"
	errServiceUnavailable = "ServiceUnavailableException"
)
