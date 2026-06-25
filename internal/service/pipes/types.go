// Package pipes implements the AWS EventBridge Pipes service emulation.
package pipes

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// AWSTimestamp represents a timestamp in AWS format (Unix epoch seconds as float64).
type AWSTimestamp struct {
	time.Time
}

// MarshalJSON marshals the timestamp to Unix epoch seconds.
func (t AWSTimestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		data, err := json.Marshal(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal nil timestamp: %w", err)
		}

		return data, nil
	}

	seconds := float64(t.UnixNano()) / float64(time.Second)
	// Round to 3 decimal places for millisecond precision.
	seconds = math.Round(seconds*1000) / 1000

	data, err := json.Marshal(seconds)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal timestamp: %w", err)
	}

	return data, nil
}

// UnmarshalJSON unmarshals Unix epoch seconds to a timestamp.
func (t *AWSTimestamp) UnmarshalJSON(data []byte) error {
	var seconds float64
	if err := json.Unmarshal(data, &seconds); err != nil {
		return fmt.Errorf("failed to unmarshal timestamp: %w", err)
	}

	nanos := int64(seconds * float64(time.Second))
	t.Time = time.Unix(0, nanos)

	return nil
}

// Pipe represents an EventBridge Pipe.
type Pipe struct {
	Arn                  string                `json:"Arn,omitempty"`
	Name                 string                `json:"Name,omitempty"`
	Source               string                `json:"Source,omitempty"`
	Target               string                `json:"Target,omitempty"`
	RoleArn              string                `json:"RoleArn,omitempty"`
	Description          string                `json:"Description,omitempty"`
	DesiredState         string                `json:"DesiredState,omitempty"`
	CurrentState         string                `json:"CurrentState,omitempty"`
	StateReason          string                `json:"StateReason,omitempty"`
	Enrichment           string                `json:"Enrichment,omitempty"`
	EnrichmentParameters *EnrichmentParameters `json:"EnrichmentParameters,omitempty"`
	SourceParameters     *SourceParameters     `json:"SourceParameters,omitempty"`
	TargetParameters     *TargetParameters     `json:"TargetParameters,omitempty"`
	LogConfiguration     *LogConfiguration     `json:"LogConfiguration,omitempty"`
	KmsKeyIdentifier     string                `json:"KmsKeyIdentifier,omitempty"`
	Tags                 map[string]string     `json:"Tags,omitempty"`
	CreationTime         AWSTimestamp          `json:"CreationTime,omitempty"`
	LastModifiedTime     AWSTimestamp          `json:"LastModifiedTime,omitempty"`
}

// PipeSummary represents a pipe summary for list operations.
type PipeSummary struct {
	Arn              string       `json:"Arn,omitempty"`
	Name             string       `json:"Name,omitempty"`
	Source           string       `json:"Source,omitempty"`
	Target           string       `json:"Target,omitempty"`
	DesiredState     string       `json:"DesiredState,omitempty"`
	CurrentState     string       `json:"CurrentState,omitempty"`
	StateReason      string       `json:"StateReason,omitempty"`
	Enrichment       string       `json:"Enrichment,omitempty"`
	CreationTime     AWSTimestamp `json:"CreationTime,omitempty"`
	LastModifiedTime AWSTimestamp `json:"LastModifiedTime,omitempty"`
}

// EnrichmentParameters contains enrichment configuration.
type EnrichmentParameters struct {
	HttpParameters *EnrichmentHttpParameters `json:"HttpParameters,omitempty"`
	InputTemplate  string                    `json:"InputTemplate,omitempty"`
}

// EnrichmentHttpParameters contains HTTP parameters for enrichment.
type EnrichmentHttpParameters struct {
	HeaderParameters      map[string]string `json:"HeaderParameters,omitempty"`
	PathParameterValues   []string          `json:"PathParameterValues,omitempty"`
	QueryStringParameters map[string]string `json:"QueryStringParameters,omitempty"`
}

// SourceParameters contains source configuration.
// This is simplified - AWS has many source types with different parameters.
type SourceParameters struct {
	FilterCriteria           *FilterCriteria                 `json:"FilterCriteria,omitempty"`
	DynamoDBStreamParameters *DynamoDBStreamSourceParameters `json:"DynamoDBStreamParameters,omitempty"`
	KinesisStreamParameters  *KinesisStreamSourceParameters  `json:"KinesisStreamParameters,omitempty"`
	SqsQueueParameters       *SqsQueueSourceParameters       `json:"SqsQueueParameters,omitempty"`
}

// FilterCriteria contains event filter criteria.
type FilterCriteria struct {
	Filters []Filter `json:"Filters,omitempty"`
}

// Filter represents an event filter.
type Filter struct {
	Pattern string `json:"Pattern,omitempty"`
}

// DynamoDBStreamSourceParameters contains DynamoDB stream source parameters.
type DynamoDBStreamSourceParameters struct {
	BatchSize                      int32             `json:"BatchSize,omitempty"`
	DeadLetterConfig               *DeadLetterConfig `json:"DeadLetterConfig,omitempty"`
	MaximumBatchingWindowInSeconds int32             `json:"MaximumBatchingWindowInSeconds,omitempty"`
	MaximumRecordAgeInSeconds      int32             `json:"MaximumRecordAgeInSeconds,omitempty"`
	MaximumRetryAttempts           int32             `json:"MaximumRetryAttempts,omitempty"`
	OnPartialBatchItemFailure      string            `json:"OnPartialBatchItemFailure,omitempty"`
	ParallelizationFactor          int32             `json:"ParallelizationFactor,omitempty"`
	StartingPosition               string            `json:"StartingPosition,omitempty"`
}

// KinesisStreamSourceParameters contains Kinesis stream source parameters.
type KinesisStreamSourceParameters struct {
	BatchSize                      int32             `json:"BatchSize,omitempty"`
	DeadLetterConfig               *DeadLetterConfig `json:"DeadLetterConfig,omitempty"`
	MaximumBatchingWindowInSeconds int32             `json:"MaximumBatchingWindowInSeconds,omitempty"`
	MaximumRecordAgeInSeconds      int32             `json:"MaximumRecordAgeInSeconds,omitempty"`
	MaximumRetryAttempts           int32             `json:"MaximumRetryAttempts,omitempty"`
	OnPartialBatchItemFailure      string            `json:"OnPartialBatchItemFailure,omitempty"`
	ParallelizationFactor          int32             `json:"ParallelizationFactor,omitempty"`
	StartingPosition               string            `json:"StartingPosition,omitempty"`
	StartingPositionTimestamp      AWSTimestamp      `json:"StartingPositionTimestamp,omitempty"`
}

// SqsQueueSourceParameters contains SQS queue source parameters.
type SqsQueueSourceParameters struct {
	BatchSize                      int32 `json:"BatchSize,omitempty"`
	MaximumBatchingWindowInSeconds int32 `json:"MaximumBatchingWindowInSeconds,omitempty"`
}

// DeadLetterConfig contains dead letter queue configuration.
type DeadLetterConfig struct {
	Arn string `json:"Arn,omitempty"`
}

// TargetParameters contains target configuration.
// This is simplified - AWS has many target types with different parameters.
type TargetParameters struct {
	InputTemplate                 string                       `json:"InputTemplate,omitempty"`
	LambdaFunctionParameters      *LambdaTargetParameters      `json:"LambdaFunctionParameters,omitempty"`
	SqsQueueParameters            *SqsTargetParameters         `json:"SqsQueueParameters,omitempty"`
	EcsTaskParameters             *EcsTargetParameters         `json:"EcsTaskParameters,omitempty"`
	EventBridgeEventBusParameters *EventBridgeTargetParameters `json:"EventBridgeEventBusParameters,omitempty"`
}

// EventBridgeTargetParameters contains parameters for an EventBridge event bus
// target. Source/DetailType are stamped onto the events the pipe publishes so
// downstream rules can match on them.
type EventBridgeTargetParameters struct {
	DetailType string   `json:"DetailType,omitempty"`
	Source     string   `json:"Source,omitempty"`
	Resources  []string `json:"Resources,omitempty"`
}

// LambdaTargetParameters contains Lambda target parameters.
type LambdaTargetParameters struct {
	InvocationType string `json:"InvocationType,omitempty"`
}

// SqsTargetParameters contains SQS target parameters.
type SqsTargetParameters struct {
	MessageDeduplicationId string `json:"MessageDeduplicationId,omitempty"`
	MessageGroupId         string `json:"MessageGroupId,omitempty"`
}

// EcsTargetParameters contains ECS target parameters.
type EcsTargetParameters struct {
	TaskDefinitionArn    string                `json:"TaskDefinitionArn,omitempty"`
	TaskCount            int32                 `json:"TaskCount,omitempty"`
	LaunchType           string                `json:"LaunchType,omitempty"`
	NetworkConfiguration *NetworkConfiguration `json:"NetworkConfiguration,omitempty"`
	Overrides            *EcsTaskOverride      `json:"Overrides,omitempty"`
}

// NetworkConfiguration contains network configuration for ECS tasks.
type NetworkConfiguration struct {
	AwsVpcConfiguration *AwsVpcConfiguration `json:"AwsvpcConfiguration,omitempty"`
}

// AwsVpcConfiguration contains VPC configuration.
type AwsVpcConfiguration struct {
	AssignPublicIp string   `json:"AssignPublicIp,omitempty"`
	SecurityGroups []string `json:"SecurityGroups,omitempty"`
	Subnets        []string `json:"Subnets,omitempty"`
}

// EcsTaskOverride contains ECS task overrides.
type EcsTaskOverride struct {
	ContainerOverrides []ContainerOverride `json:"ContainerOverrides,omitempty"`
	Cpu                string              `json:"Cpu,omitempty"`
	Memory             string              `json:"Memory,omitempty"`
	TaskRoleArn        string              `json:"TaskRoleArn,omitempty"`
}

// ContainerOverride contains container overrides.
type ContainerOverride struct {
	Name    string   `json:"Name,omitempty"`
	Command []string `json:"Command,omitempty"`
}

// LogConfiguration contains logging configuration.
type LogConfiguration struct {
	Level                        string                        `json:"Level,omitempty"`
	CloudwatchLogsLogDestination *CloudwatchLogsLogDestination `json:"CloudwatchLogsLogDestination,omitempty"`
	FirehoseLogDestination       *FirehoseLogDestination       `json:"FirehoseLogDestination,omitempty"`
	S3LogDestination             *S3LogDestination             `json:"S3LogDestination,omitempty"`
	IncludeExecutionData         []string                      `json:"IncludeExecutionData,omitempty"`
}

// CloudwatchLogsLogDestination contains CloudWatch Logs destination configuration.
type CloudwatchLogsLogDestination struct {
	LogGroupArn string `json:"LogGroupArn,omitempty"`
}

// FirehoseLogDestination contains Firehose destination configuration.
type FirehoseLogDestination struct {
	DeliveryStreamArn string `json:"DeliveryStreamArn,omitempty"`
}

// S3LogDestination contains S3 destination configuration.
type S3LogDestination struct {
	BucketName   string `json:"BucketName,omitempty"`
	BucketOwner  string `json:"BucketOwner,omitempty"`
	OutputFormat string `json:"OutputFormat,omitempty"`
	Prefix       string `json:"Prefix,omitempty"`
}

// Request types.

// CreatePipeInput represents the input for CreatePipe.
type CreatePipeInput struct {
	Name                 string                `json:"Name,omitempty"`
	Source               string                `json:"Source,omitempty"`
	Target               string                `json:"Target,omitempty"`
	RoleArn              string                `json:"RoleArn,omitempty"`
	Description          string                `json:"Description,omitempty"`
	DesiredState         string                `json:"DesiredState,omitempty"`
	Enrichment           string                `json:"Enrichment,omitempty"`
	EnrichmentParameters *EnrichmentParameters `json:"EnrichmentParameters,omitempty"`
	SourceParameters     *SourceParameters     `json:"SourceParameters,omitempty"`
	TargetParameters     *TargetParameters     `json:"TargetParameters,omitempty"`
	LogConfiguration     *LogConfiguration     `json:"LogConfiguration,omitempty"`
	KmsKeyIdentifier     string                `json:"KmsKeyIdentifier,omitempty"`
	Tags                 map[string]string     `json:"Tags,omitempty"`
}

// UpdatePipeInput represents the input for UpdatePipe.
type UpdatePipeInput struct {
	Name                 string                `json:"Name,omitempty"`
	RoleArn              string                `json:"RoleArn,omitempty"`
	Description          string                `json:"Description,omitempty"`
	DesiredState         string                `json:"DesiredState,omitempty"`
	Source               string                `json:"Source,omitempty"`
	Target               string                `json:"Target,omitempty"`
	Enrichment           string                `json:"Enrichment,omitempty"`
	EnrichmentParameters *EnrichmentParameters `json:"EnrichmentParameters,omitempty"`
	SourceParameters     *SourceParameters     `json:"SourceParameters,omitempty"`
	TargetParameters     *TargetParameters     `json:"TargetParameters,omitempty"`
	LogConfiguration     *LogConfiguration     `json:"LogConfiguration,omitempty"`
	KmsKeyIdentifier     string                `json:"KmsKeyIdentifier,omitempty"`
}

// ListPipesInput represents the input for ListPipes.
type ListPipesInput struct {
	CurrentState string `json:"CurrentState,omitempty"`
	DesiredState string `json:"DesiredState,omitempty"`
	Limit        int32  `json:"Limit,omitempty"`
	NamePrefix   string `json:"NamePrefix,omitempty"`
	NextToken    string `json:"NextToken,omitempty"`
	SourcePrefix string `json:"SourcePrefix,omitempty"`
	TargetPrefix string `json:"TargetPrefix,omitempty"`
}

// TagResourceInput represents the input for TagResource.
type TagResourceInput struct {
	ResourceArn string            `json:"resourceArn,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// UntagResourceInput represents the input for UntagResource.
type UntagResourceInput struct {
	ResourceArn string   `json:"resourceArn,omitempty"`
	TagKeys     []string `json:"tagKeys,omitempty"`
}

// Response types.

// CreatePipeOutput represents the output for CreatePipe.
type CreatePipeOutput struct {
	Arn              string       `json:"Arn,omitempty"`
	Name             string       `json:"Name,omitempty"`
	DesiredState     string       `json:"DesiredState,omitempty"`
	CurrentState     string       `json:"CurrentState,omitempty"`
	CreationTime     AWSTimestamp `json:"CreationTime,omitempty"`
	LastModifiedTime AWSTimestamp `json:"LastModifiedTime,omitempty"`
}

// UpdatePipeOutput represents the output for UpdatePipe.
type UpdatePipeOutput struct {
	Arn              string       `json:"Arn,omitempty"`
	Name             string       `json:"Name,omitempty"`
	DesiredState     string       `json:"DesiredState,omitempty"`
	CurrentState     string       `json:"CurrentState,omitempty"`
	CreationTime     AWSTimestamp `json:"CreationTime,omitempty"`
	LastModifiedTime AWSTimestamp `json:"LastModifiedTime,omitempty"`
}

// DescribePipeOutput represents the output for DescribePipe.
type DescribePipeOutput struct {
	Arn                  string                `json:"Arn,omitempty"`
	Name                 string                `json:"Name,omitempty"`
	Source               string                `json:"Source,omitempty"`
	Target               string                `json:"Target,omitempty"`
	RoleArn              string                `json:"RoleArn,omitempty"`
	Description          string                `json:"Description,omitempty"`
	DesiredState         string                `json:"DesiredState,omitempty"`
	CurrentState         string                `json:"CurrentState,omitempty"`
	StateReason          string                `json:"StateReason,omitempty"`
	Enrichment           string                `json:"Enrichment,omitempty"`
	EnrichmentParameters *EnrichmentParameters `json:"EnrichmentParameters,omitempty"`
	SourceParameters     *SourceParameters     `json:"SourceParameters,omitempty"`
	TargetParameters     *TargetParameters     `json:"TargetParameters,omitempty"`
	LogConfiguration     *LogConfiguration     `json:"LogConfiguration,omitempty"`
	KmsKeyIdentifier     string                `json:"KmsKeyIdentifier,omitempty"`
	Tags                 map[string]string     `json:"Tags,omitempty"`
	CreationTime         AWSTimestamp          `json:"CreationTime,omitempty"`
	LastModifiedTime     AWSTimestamp          `json:"LastModifiedTime,omitempty"`
}

// DeletePipeOutput represents the output for DeletePipe.
type DeletePipeOutput struct {
	Arn              string       `json:"Arn,omitempty"`
	Name             string       `json:"Name,omitempty"`
	DesiredState     string       `json:"DesiredState,omitempty"`
	CurrentState     string       `json:"CurrentState,omitempty"`
	CreationTime     AWSTimestamp `json:"CreationTime,omitempty"`
	LastModifiedTime AWSTimestamp `json:"LastModifiedTime,omitempty"`
}

// ListPipesOutput represents the output for ListPipes.
type ListPipesOutput struct {
	Pipes     []*PipeSummary `json:"Pipes,omitempty"`
	NextToken string         `json:"NextToken,omitempty"`
}

// StartPipeOutput represents the output for StartPipe.
type StartPipeOutput struct {
	Arn              string       `json:"Arn,omitempty"`
	Name             string       `json:"Name,omitempty"`
	DesiredState     string       `json:"DesiredState,omitempty"`
	CurrentState     string       `json:"CurrentState,omitempty"`
	CreationTime     AWSTimestamp `json:"CreationTime,omitempty"`
	LastModifiedTime AWSTimestamp `json:"LastModifiedTime,omitempty"`
}

// StopPipeOutput represents the output for StopPipe.
type StopPipeOutput struct {
	Arn              string       `json:"Arn,omitempty"`
	Name             string       `json:"Name,omitempty"`
	DesiredState     string       `json:"DesiredState,omitempty"`
	CurrentState     string       `json:"CurrentState,omitempty"`
	CreationTime     AWSTimestamp `json:"CreationTime,omitempty"`
	LastModifiedTime AWSTimestamp `json:"LastModifiedTime,omitempty"`
}

// ListTagsForResourceOutput represents the output for ListTagsForResource.
type ListTagsForResourceOutput struct {
	Tags map[string]string `json:"tags,omitempty"`
}

// Error types.

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Message string `json:"message,omitempty"`
}

// Pipe state constants.
const (
	// Desired states.
	DesiredStateRunning = "RUNNING"
	DesiredStateStopped = "STOPPED"

	// Current states.
	CurrentStateRunning  = "RUNNING"
	CurrentStateStopped  = "STOPPED"
	CurrentStateCreating = "CREATING"
	CurrentStateUpdating = "UPDATING"
	CurrentStateDeleting = "DELETING"
	CurrentStateStarting = "STARTING"
	CurrentStateStopping = "STOPPING"

	// Failed states.
	CurrentStateCreateFailed         = "CREATE_FAILED"
	CurrentStateUpdateFailed         = "UPDATE_FAILED"
	CurrentStateStartFailed          = "START_FAILED"
	CurrentStateStopFailed           = "STOP_FAILED"
	CurrentStateDeleteFailed         = "DELETE_FAILED"
	CurrentStateCreateRollbackFailed = "CREATE_ROLLBACK_FAILED"
	CurrentStateDeleteRollbackFailed = "DELETE_ROLLBACK_FAILED"
	CurrentStateUpdateRollbackFailed = "UPDATE_ROLLBACK_FAILED"
)

// Error code constants.
const (
	errConflictException   = "ConflictException"
	errNotFoundException   = "NotFoundException"
	errValidationException = "ValidationException"
	errInternalException   = "InternalException"
)

// Pagination constants.
const (
	defaultPageLimit = 100
	maxPageLimit     = 100
)
