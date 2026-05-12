// Package sqs provides SQS service emulation for kumo.
package sqs

import (
	"time"
)

// Queue represents an SQS queue.
type Queue struct {
	Name                      string
	URL                       string
	ARN                       string
	Tags                      map[string]string
	CreatedTimestamp          time.Time
	LastModifiedTimestamp     time.Time
	VisibilityTimeout         int
	MessageRetentionPeriod    int
	DelaySeconds              int
	MaxMessageSize            int
	ReceiveWaitTimeSeconds    int
	FifoQueue                 bool
	ContentBasedDeduplication bool
	Policy                    string // JSON string: IAM policy document
	RedrivePolicy             string // JSON string: {"deadLetterTargetArn":"...","maxReceiveCount":"N"}
	MaxReceiveCount           int    // Parsed from RedrivePolicy
	DeadLetterTargetArn       string // Parsed from RedrivePolicy
}

// Message represents an SQS message.
type Message struct {
	MessageID              string
	ReceiptHandle          string
	Body                   string
	MD5OfBody              string
	Attributes             map[string]string
	MessageAttributes      map[string]MessageAttributeValue
	SentTimestamp          time.Time
	VisibleAt              time.Time
	ReceiveCount           int
	MessageGroupID         string
	MessageDeduplicationID string
	SequenceNumber         string
}

// MessageAttributeValue represents a message attribute.
type MessageAttributeValue struct {
	DataType    string `json:"DataType"`
	StringValue string `json:"StringValue,omitempty"`
	BinaryValue []byte `json:"BinaryValue,omitempty"`
}

// JSON Request/Response Types for AWS JSON 1.0 Protocol
// AWS SQS API uses PascalCase for JSON field names.

// CreateQueueRequest is the request for CreateQueue.
type CreateQueueRequest struct {
	QueueName  string            `json:"QueueName"`
	Attributes map[string]string `json:"Attributes,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// CreateQueueResponse is the response for CreateQueue.
type CreateQueueResponse struct {
	QueueURL string `json:"QueueUrl"`
}

// DeleteQueueRequest is the request for DeleteQueue.
type DeleteQueueRequest struct {
	QueueURL string `json:"QueueUrl"`
}

// ListQueuesRequest is the request for ListQueues.
type ListQueuesRequest struct {
	QueueNamePrefix string `json:"QueueNamePrefix,omitempty"`
	MaxResults      int    `json:"MaxResults,omitempty"`
	NextToken       string `json:"NextToken,omitempty"`
}

// ListQueuesResponse is the response for ListQueues.
type ListQueuesResponse struct {
	QueueUrls []string `json:"QueueUrls,omitempty"`
	NextToken string   `json:"NextToken,omitempty"`
}

// GetQueueURLRequest is the request for GetQueueUrl.
type GetQueueURLRequest struct {
	QueueName              string `json:"QueueName"`
	QueueOwnerAWSAccountID string `json:"QueueOwnerAWSAccountId,omitempty"`
}

// GetQueueURLResponse is the response for GetQueueUrl.
type GetQueueURLResponse struct {
	QueueURL string `json:"QueueUrl"`
}

// ListQueueTagsRequest is the request for ListQueueTags.
type ListQueueTagsRequest struct {
	QueueURL string `json:"QueueUrl"`
}

// ListQueueTagsResponse is the response for ListQueueTags.
type ListQueueTagsResponse struct {
	Tags map[string]string `json:"Tags,omitempty"`
}

// TagQueueRequest is the request for TagQueue.
type TagQueueRequest struct {
	QueueURL string            `json:"QueueUrl"`
	Tags     map[string]string `json:"Tags"`
}

// UntagQueueRequest is the request for UntagQueue.
type UntagQueueRequest struct {
	QueueURL string   `json:"QueueUrl"`
	TagKeys  []string `json:"TagKeys"`
}

// SendMessageRequest is the request for SendMessage.
type SendMessageRequest struct {
	QueueURL               string                           `json:"QueueUrl"`
	MessageBody            string                           `json:"MessageBody"`
	DelaySeconds           int                              `json:"DelaySeconds,omitempty"`
	MessageAttributes      map[string]MessageAttributeValue `json:"MessageAttributes,omitempty"`
	MessageDeduplicationID string                           `json:"MessageDeduplicationId,omitempty"`
	MessageGroupID         string                           `json:"MessageGroupId,omitempty"`
}

// SendMessageResponse is the response for SendMessage.
type SendMessageResponse struct {
	MessageID                    string `json:"MessageId"`
	MD5OfMessageBody             string `json:"MD5OfMessageBody"`
	MD5OfMessageAttributes       string `json:"MD5OfMessageAttributes,omitempty"`
	MD5OfMessageSystemAttributes string `json:"MD5OfMessageSystemAttributes,omitempty"`
	SequenceNumber               string `json:"SequenceNumber,omitempty"`
}

// SendMessageBatchRequest is the request for SendMessageBatch.
type SendMessageBatchRequest struct {
	QueueURL string                         `json:"QueueUrl"`
	Entries  []SendMessageBatchRequestEntry `json:"Entries"`
}

// SendMessageBatchRequestEntry is an individual entry in a SendMessageBatch request.
type SendMessageBatchRequestEntry struct {
	ID                     string                           `json:"Id"`
	MessageBody            string                           `json:"MessageBody"`
	DelaySeconds           int                              `json:"DelaySeconds,omitempty"`
	MessageAttributes      map[string]MessageAttributeValue `json:"MessageAttributes,omitempty"`
	MessageDeduplicationID string                           `json:"MessageDeduplicationId,omitempty"`
	MessageGroupID         string                           `json:"MessageGroupId,omitempty"`
}

// SendMessageBatchResponse is the response for SendMessageBatch.
type SendMessageBatchResponse struct {
	Successful []SendMessageBatchResultEntry `json:"Successful,omitempty"`
	Failed     []BatchResultErrorEntry       `json:"Failed,omitempty"`
}

// SendMessageBatchResultEntry is a successful entry in a SendMessageBatch response.
type SendMessageBatchResultEntry struct {
	ID                     string `json:"Id"`
	MessageID              string `json:"MessageId"`
	MD5OfMessageBody       string `json:"MD5OfMessageBody"`
	MD5OfMessageAttributes string `json:"MD5OfMessageAttributes,omitempty"`
	SequenceNumber         string `json:"SequenceNumber,omitempty"`
}

// BatchResultErrorEntry is a failed entry in a batch response.
type BatchResultErrorEntry struct {
	ID          string `json:"Id"`
	SenderFault bool   `json:"SenderFault"`
	Code        string `json:"Code"`
	Message     string `json:"Message,omitempty"`
}

// ReceiveMessageRequest is the request for ReceiveMessage.
type ReceiveMessageRequest struct {
	QueueURL                string   `json:"QueueUrl"`
	AttributeNames          []string `json:"AttributeNames,omitempty"`
	MaxNumberOfMessages     int      `json:"MaxNumberOfMessages,omitempty"`
	MessageAttributeNames   []string `json:"MessageAttributeNames,omitempty"`
	ReceiveRequestAttemptID string   `json:"ReceiveRequestAttemptId,omitempty"`
	VisibilityTimeout       int      `json:"VisibilityTimeout,omitempty"`
	WaitTimeSeconds         int      `json:"WaitTimeSeconds,omitempty"`
}

// ReceiveMessageResponse is the response for ReceiveMessage.
type ReceiveMessageResponse struct {
	Messages []MessageResponse `json:"Messages,omitempty"`
}

// MessageResponse represents a message in response.
type MessageResponse struct {
	MessageID              string                           `json:"MessageId"`
	ReceiptHandle          string                           `json:"ReceiptHandle"`
	MD5OfBody              string                           `json:"MD5OfBody"`
	Body                   string                           `json:"Body"`
	Attributes             map[string]string                `json:"Attributes,omitempty"`
	MD5OfMessageAttributes string                           `json:"MD5OfMessageAttributes,omitempty"`
	MessageAttributes      map[string]MessageAttributeValue `json:"MessageAttributes,omitempty"`
	SequenceNumber         string                           `json:"SequenceNumber,omitempty"`
}

// DeleteMessageRequest is the request for DeleteMessage.
type DeleteMessageRequest struct {
	QueueURL      string `json:"QueueUrl"`
	ReceiptHandle string `json:"ReceiptHandle"`
}

// DeleteMessageBatchRequest is the request for DeleteMessageBatch.
type DeleteMessageBatchRequest struct {
	QueueURL string                           `json:"QueueUrl"`
	Entries  []DeleteMessageBatchRequestEntry `json:"Entries"`
}

// DeleteMessageBatchRequestEntry is an individual entry in a DeleteMessageBatch request.
type DeleteMessageBatchRequestEntry struct {
	ID            string `json:"Id"`
	ReceiptHandle string `json:"ReceiptHandle"`
}

// DeleteMessageBatchResponse is the response for DeleteMessageBatch.
type DeleteMessageBatchResponse struct {
	Successful []DeleteMessageBatchResultEntry `json:"Successful,omitempty"`
	Failed     []BatchResultErrorEntry         `json:"Failed,omitempty"`
}

// DeleteMessageBatchResultEntry is a successful entry in a DeleteMessageBatch response.
type DeleteMessageBatchResultEntry struct {
	ID string `json:"Id"`
}

// PurgeQueueRequest is the request for PurgeQueue.
type PurgeQueueRequest struct {
	QueueURL string `json:"QueueUrl"`
}

// GetQueueAttributesRequest is the request for GetQueueAttributes.
type GetQueueAttributesRequest struct {
	QueueURL       string   `json:"QueueUrl"`
	AttributeNames []string `json:"AttributeNames,omitempty"`
}

// GetQueueAttributesResponse is the response for GetQueueAttributes.
type GetQueueAttributesResponse struct {
	Attributes map[string]string `json:"Attributes,omitempty"`
}

// SetQueueAttributesRequest is the request for SetQueueAttributes.
type SetQueueAttributesRequest struct {
	QueueURL   string            `json:"QueueUrl"`
	Attributes map[string]string `json:"Attributes"`
}

// ErrorResponse represents an SQS error response in JSON format.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}
