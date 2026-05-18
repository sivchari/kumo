// Package dynamodb provides DynamoDB service emulation for kumo.
package dynamodb

import (
	"encoding/json"
	"fmt"
	"time"
)

// ReturnValues constants for DynamoDB operations.
const (
	ReturnValuesAllOld     = "ALL_OLD"
	ReturnValuesAllNew     = "ALL_NEW"
	ReturnValuesUpdatedOld = "UPDATED_OLD"
	ReturnValuesUpdatedNew = "UPDATED_NEW"
)

// Error code constants.
const (
	ErrCodeConditionalCheckFailed = "ConditionalCheckFailedException"
)

// AttributeValue represents a DynamoDB attribute value.
// Custom MarshalJSON/UnmarshalJSON handle serialization to preserve empty
// collections (e.g. "L": [], "SS": []) and omit nil fields.
type AttributeValue struct {
	S    *string
	N    *string
	B    []byte
	SS   []string
	NS   []string
	BS   [][]byte
	M    map[string]*AttributeValue
	L    []*AttributeValue
	NULL *bool
	BOOL *bool
}

// MarshalJSON serializes AttributeValue, preserving empty slices/maps.
//
//nolint:gocritic // hugeParam: value receiver is required so json.Marshal works on map[string]AttributeValue (Item type).
func (av AttributeValue) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)

	if av.S != nil {
		m["S"] = av.S
	}

	if av.N != nil {
		m["N"] = av.N
	}

	if av.B != nil {
		m["B"] = av.B
	}

	if av.SS != nil {
		m["SS"] = av.SS
	}

	if av.NS != nil {
		m["NS"] = av.NS
	}

	if av.BS != nil {
		m["BS"] = av.BS
	}

	if av.M != nil {
		m["M"] = av.M
	}

	if av.L != nil {
		m["L"] = av.L
	}

	if av.NULL != nil {
		m["NULL"] = av.NULL
	}

	if av.BOOL != nil {
		m["BOOL"] = av.BOOL
	}

	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attribute value: %w", err)
	}

	return data, nil
}

// UnmarshalJSON deserializes AttributeValue.
func (av *AttributeValue) UnmarshalJSON(data []byte) error {
	var raw struct {
		S    *string                    `json:"S"`
		N    *string                    `json:"N"`
		B    []byte                     `json:"B"`
		SS   []string                   `json:"SS"`
		NS   []string                   `json:"NS"`
		BS   [][]byte                   `json:"BS"`
		M    map[string]*AttributeValue `json:"M"`
		L    []*AttributeValue          `json:"L"`
		NULL *bool                      `json:"NULL"`
		BOOL *bool                      `json:"BOOL"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err //nolint:wrapcheck // internal deserialization
	}

	av.S = raw.S
	av.N = raw.N
	av.B = raw.B
	av.SS = raw.SS
	av.NS = raw.NS
	av.BS = raw.BS
	av.M = raw.M
	av.L = raw.L
	av.NULL = raw.NULL
	av.BOOL = raw.BOOL

	return nil
}

// KeySchemaElement represents a key schema element.
type KeySchemaElement struct {
	AttributeName string `json:"AttributeName"`
	KeyType       string `json:"KeyType"` // HASH or RANGE
}

// AttributeDefinition represents an attribute definition.
type AttributeDefinition struct {
	AttributeName string `json:"AttributeName"`
	AttributeType string `json:"AttributeType"` // S, N, or B
}

// ProvisionedThroughput represents provisioned throughput settings.
type ProvisionedThroughput struct {
	ReadCapacityUnits  int64 `json:"ReadCapacityUnits"`
	WriteCapacityUnits int64 `json:"WriteCapacityUnits"`
}

// ProvisionedThroughputDescription represents provisioned throughput description.
type ProvisionedThroughputDescription struct {
	ReadCapacityUnits      int64  `json:"ReadCapacityUnits"`
	WriteCapacityUnits     int64  `json:"WriteCapacityUnits"`
	LastIncreaseDateTime   *int64 `json:"LastIncreaseDateTime,omitempty"`
	LastDecreaseDateTime   *int64 `json:"LastDecreaseDateTime,omitempty"`
	NumberOfDecreasesToday int64  `json:"NumberOfDecreasesToday"`
}

// Projection represents a GSI projection.
type Projection struct {
	ProjectionType   string   `json:"ProjectionType"`
	NonKeyAttributes []string `json:"NonKeyAttributes,omitempty"`
}

// GlobalSecondaryIndex represents a GSI definition in CreateTable requests.
type GlobalSecondaryIndex struct {
	IndexName             string                 `json:"IndexName"`
	KeySchema             []KeySchemaElement     `json:"KeySchema"`
	Projection            Projection             `json:"Projection"`
	ProvisionedThroughput *ProvisionedThroughput `json:"ProvisionedThroughput,omitempty"`
}

// LocalSecondaryIndex represents an LSI definition in CreateTable requests.
type LocalSecondaryIndex struct {
	IndexName  string             `json:"IndexName"`
	KeySchema  []KeySchemaElement `json:"KeySchema"`
	Projection Projection         `json:"Projection"`
}

// LocalSecondaryIndexDescription represents an LSI in DescribeTable responses.
type LocalSecondaryIndexDescription struct {
	IndexName      string             `json:"IndexName"`
	KeySchema      []KeySchemaElement `json:"KeySchema"`
	Projection     Projection         `json:"Projection"`
	IndexArn       string             `json:"IndexArn"`
	ItemCount      int64              `json:"ItemCount"`
	IndexSizeBytes int64              `json:"IndexSizeBytes"`
}

// GlobalSecondaryIndexDescription represents a GSI in DescribeTable responses.
type GlobalSecondaryIndexDescription struct {
	IndexName             string                            `json:"IndexName"`
	KeySchema             []KeySchemaElement                `json:"KeySchema"`
	Projection            Projection                        `json:"Projection"`
	IndexStatus           string                            `json:"IndexStatus"`
	IndexArn              string                            `json:"IndexArn"`
	ItemCount             int64                             `json:"ItemCount"`
	IndexSizeBytes        int64                             `json:"IndexSizeBytes"`
	ProvisionedThroughput *ProvisionedThroughputDescription `json:"ProvisionedThroughput,omitempty"`
}

// Table represents a DynamoDB table.
type Table struct {
	Name                   string
	KeySchema              []KeySchemaElement
	AttributeDefinitions   []AttributeDefinition
	ProvisionedThroughput  *ProvisionedThroughput
	GlobalSecondaryIndexes []GlobalSecondaryIndex
	LocalSecondaryIndexes  []LocalSecondaryIndex
	CreationDateTime       time.Time
	TableStatus            string
	ItemCount              int64
	TableSizeBytes         int64
	TableARN               string
	BillingMode            string
	DeletionProtection     bool
	TTLAttributeName       string
	TTLEnabled             bool
	StreamEnabled          bool
	StreamViewType         string
	LatestStreamArn        string
}

// StreamSpecification represents DynamoDB stream settings.
type StreamSpecification struct {
	StreamEnabled  bool   `json:"StreamEnabled"`
	StreamViewType string `json:"StreamViewType,omitempty"`
}

// TableDescription represents a table description in responses.
type TableDescription struct {
	TableName                 string                            `json:"TableName"`
	TableStatus               string                            `json:"TableStatus"`
	TableARN                  string                            `json:"TableArn"`
	TableID                   string                            `json:"TableId,omitempty"`
	CreationDateTime          float64                           `json:"CreationDateTime"`
	KeySchema                 []KeySchemaElement                `json:"KeySchema"`
	AttributeDefinitions      []AttributeDefinition             `json:"AttributeDefinitions"`
	ProvisionedThroughput     *ProvisionedThroughputDescription `json:"ProvisionedThroughput,omitempty"`
	GlobalSecondaryIndexes    []GlobalSecondaryIndexDescription `json:"GlobalSecondaryIndexes,omitempty"`
	LocalSecondaryIndexes     []LocalSecondaryIndexDescription  `json:"LocalSecondaryIndexes,omitempty"`
	ItemCount                 int64                             `json:"ItemCount"`
	TableSizeBytes            int64                             `json:"TableSizeBytes"`
	BillingModeSummary        *BillingModeSummary               `json:"BillingModeSummary,omitempty"`
	StreamSpecification       *StreamSpecification              `json:"StreamSpecification,omitempty"`
	LatestStreamArn           string                            `json:"LatestStreamArn,omitempty"`
	DeletionProtectionEnabled bool                              `json:"DeletionProtectionEnabled"`
}

// BillingModeSummary represents billing mode summary.
type BillingModeSummary struct {
	BillingMode                       string `json:"BillingMode"`
	LastUpdateToPayPerRequestDateTime *int64 `json:"LastUpdateToPayPerRequestDateTime,omitempty"`
}

// Item represents a DynamoDB item.
type Item map[string]AttributeValue

// JSON Request/Response Types for AWS JSON 1.0 Protocol.

// CreateTableRequest is the request for CreateTable.
type CreateTableRequest struct {
	TableName                 string                 `json:"TableName"`
	KeySchema                 []KeySchemaElement     `json:"KeySchema"`
	AttributeDefinitions      []AttributeDefinition  `json:"AttributeDefinitions"`
	ProvisionedThroughput     *ProvisionedThroughput `json:"ProvisionedThroughput,omitempty"`
	GlobalSecondaryIndexes    []GlobalSecondaryIndex `json:"GlobalSecondaryIndexes,omitempty"`
	LocalSecondaryIndexes     []LocalSecondaryIndex  `json:"LocalSecondaryIndexes,omitempty"`
	BillingMode               string                 `json:"BillingMode,omitempty"`
	DeletionProtectionEnabled bool                   `json:"DeletionProtectionEnabled,omitempty"`
	StreamSpecification       *StreamSpecification   `json:"StreamSpecification,omitempty"`
}

// CreateTableResponse is the response for CreateTable.
type CreateTableResponse struct {
	TableDescription TableDescription `json:"TableDescription"`
}

// DeleteTableRequest is the request for DeleteTable.
type DeleteTableRequest struct {
	TableName string `json:"TableName"`
}

// DeleteTableResponse is the response for DeleteTable.
type DeleteTableResponse struct {
	TableDescription TableDescription `json:"TableDescription"`
}

// ListTablesRequest is the request for ListTables.
type ListTablesRequest struct {
	ExclusiveStartTableName string `json:"ExclusiveStartTableName,omitempty"`
	Limit                   int    `json:"Limit,omitempty"`
}

// ListTablesResponse is the response for ListTables.
type ListTablesResponse struct {
	TableNames             []string `json:"TableNames"`
	LastEvaluatedTableName string   `json:"LastEvaluatedTableName,omitempty"`
}

// DescribeTableRequest is the request for DescribeTable.
type DescribeTableRequest struct {
	TableName string `json:"TableName"`
}

// DescribeTableResponse is the response for DescribeTable.
type DescribeTableResponse struct {
	Table TableDescription `json:"Table"`
}

// PutItemRequest is the request for PutItem.
type PutItemRequest struct {
	TableName                 string                    `json:"TableName"`
	Item                      Item                      `json:"Item"`
	ConditionExpression       string                    `json:"ConditionExpression,omitempty"`
	ExpressionAttributeNames  map[string]string         `json:"ExpressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]AttributeValue `json:"ExpressionAttributeValues,omitempty"`
	ReturnValues              string                    `json:"ReturnValues,omitempty"`
}

// PutItemResponse is the response for PutItem.
type PutItemResponse struct {
	Attributes Item `json:"Attributes,omitempty"`
}

// GetItemRequest is the request for GetItem.
type GetItemRequest struct {
	TableName                string            `json:"TableName"`
	Key                      Item              `json:"Key"`
	ProjectionExpression     string            `json:"ProjectionExpression,omitempty"`
	ExpressionAttributeNames map[string]string `json:"ExpressionAttributeNames,omitempty"`
	ConsistentRead           bool              `json:"ConsistentRead,omitempty"`
}

// GetItemResponse is the response for GetItem.
type GetItemResponse struct {
	Item Item `json:"Item,omitempty"`
}

// DeleteItemRequest is the request for DeleteItem.
type DeleteItemRequest struct {
	TableName                 string                    `json:"TableName"`
	Key                       Item                      `json:"Key"`
	ConditionExpression       string                    `json:"ConditionExpression,omitempty"`
	ExpressionAttributeNames  map[string]string         `json:"ExpressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]AttributeValue `json:"ExpressionAttributeValues,omitempty"`
	ReturnValues              string                    `json:"ReturnValues,omitempty"`
}

// DeleteItemResponse is the response for DeleteItem.
type DeleteItemResponse struct {
	Attributes Item `json:"Attributes,omitempty"`
}

// AttributeValueUpdate represents a legacy AttributeUpdates entry.
type AttributeValueUpdate struct {
	Action string         `json:"Action"` // PUT, DELETE, ADD
	Value  AttributeValue `json:"Value"`
}

// UpdateItemRequest is the request for UpdateItem.
type UpdateItemRequest struct {
	TableName                 string                          `json:"TableName"`
	Key                       Item                            `json:"Key"`
	UpdateExpression          string                          `json:"UpdateExpression,omitempty"`
	ConditionExpression       string                          `json:"ConditionExpression,omitempty"`
	ExpressionAttributeNames  map[string]string               `json:"ExpressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]AttributeValue       `json:"ExpressionAttributeValues,omitempty"`
	AttributeUpdates          map[string]AttributeValueUpdate `json:"AttributeUpdates,omitempty"`
	ReturnValues              string                          `json:"ReturnValues,omitempty"`
}

// UpdateItemResponse is the response for UpdateItem.
type UpdateItemResponse struct {
	Attributes Item `json:"Attributes,omitempty"`
}

// QueryRequest is the request for Query.
type QueryRequest struct {
	TableName                 string                    `json:"TableName"`
	IndexName                 string                    `json:"IndexName,omitempty"`
	KeyConditionExpression    string                    `json:"KeyConditionExpression,omitempty"`
	FilterExpression          string                    `json:"FilterExpression,omitempty"`
	ProjectionExpression      string                    `json:"ProjectionExpression,omitempty"`
	ExpressionAttributeNames  map[string]string         `json:"ExpressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]AttributeValue `json:"ExpressionAttributeValues,omitempty"`
	Limit                     int                       `json:"Limit,omitempty"`
	ExclusiveStartKey         Item                      `json:"ExclusiveStartKey,omitempty"`
	ScanIndexForward          *bool                     `json:"ScanIndexForward,omitempty"`
	ConsistentRead            bool                      `json:"ConsistentRead,omitempty"`
	Select                    string                    `json:"Select,omitempty"`
}

// QueryResponse is the response for Query.
type QueryResponse struct {
	Items            []Item `json:"Items"`
	Count            int    `json:"Count"`
	ScannedCount     int    `json:"ScannedCount"`
	LastEvaluatedKey Item   `json:"LastEvaluatedKey,omitempty"`
}

// ScanRequest is the request for Scan.
type ScanRequest struct {
	TableName                 string                    `json:"TableName"`
	FilterExpression          string                    `json:"FilterExpression,omitempty"`
	ProjectionExpression      string                    `json:"ProjectionExpression,omitempty"`
	ExpressionAttributeNames  map[string]string         `json:"ExpressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]AttributeValue `json:"ExpressionAttributeValues,omitempty"`
	Limit                     int                       `json:"Limit,omitempty"`
	ExclusiveStartKey         Item                      `json:"ExclusiveStartKey,omitempty"`
	ConsistentRead            bool                      `json:"ConsistentRead,omitempty"`
	Select                    string                    `json:"Select,omitempty"`
	Segment                   *int                      `json:"Segment,omitempty"`
	TotalSegments             *int                      `json:"TotalSegments,omitempty"`
}

// ScanResponse is the response for Scan.
type ScanResponse struct {
	Items            []Item `json:"Items"`
	Count            int    `json:"Count"`
	ScannedCount     int    `json:"ScannedCount"`
	LastEvaluatedKey Item   `json:"LastEvaluatedKey,omitempty"`
}

// TimeToLiveSpecification represents a TTL specification for UpdateTimeToLive.
type TimeToLiveSpecification struct {
	AttributeName string `json:"AttributeName"`
	Enabled       bool   `json:"Enabled"`
}

// TimeToLiveDescription represents a TTL description in responses.
type TimeToLiveDescription struct {
	AttributeName    string `json:"AttributeName,omitempty"`
	TimeToLiveStatus string `json:"TimeToLiveStatus"`
}

// UpdateTimeToLiveRequest is the request for UpdateTimeToLive.
type UpdateTimeToLiveRequest struct {
	TableName               string                  `json:"TableName"`
	TimeToLiveSpecification TimeToLiveSpecification `json:"TimeToLiveSpecification"`
}

// UpdateTimeToLiveResponse is the response for UpdateTimeToLive.
type UpdateTimeToLiveResponse struct {
	TimeToLiveSpecification TimeToLiveSpecification `json:"TimeToLiveSpecification"`
}

// DescribeTimeToLiveRequest is the request for DescribeTimeToLive.
type DescribeTimeToLiveRequest struct {
	TableName string `json:"TableName"`
}

// DescribeTimeToLiveResponse is the response for DescribeTimeToLive.
type DescribeTimeToLiveResponse struct {
	TimeToLiveDescription TimeToLiveDescription `json:"TimeToLiveDescription"`
}

// TransactWriteItemsRequest is the request for TransactWriteItems.
type TransactWriteItemsRequest struct {
	TransactItems               []TransactWriteItem `json:"TransactItems"`
	ClientRequestToken          string              `json:"ClientRequestToken,omitempty"`
	ReturnConsumedCapacity      string              `json:"ReturnConsumedCapacity,omitempty"`
	ReturnItemCollectionMetrics string              `json:"ReturnItemCollectionMetrics,omitempty"`
}

// TransactWriteItem represents a single item in a TransactWriteItems request.
type TransactWriteItem struct {
	ConditionCheck *TransactConditionCheck `json:"ConditionCheck,omitempty"`
	Delete         *TransactDelete         `json:"Delete,omitempty"`
	Put            *TransactPut            `json:"Put,omitempty"`
	Update         *TransactUpdate         `json:"Update,omitempty"`
}

// TransactConditionCheck represents a ConditionCheck action in a transaction.
type TransactConditionCheck struct {
	TableName                 string                    `json:"TableName"`
	Key                       Item                      `json:"Key"`
	ConditionExpression       string                    `json:"ConditionExpression"`
	ExpressionAttributeNames  map[string]string         `json:"ExpressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]AttributeValue `json:"ExpressionAttributeValues,omitempty"`
}

// TransactDelete represents a Delete action in a transaction.
type TransactDelete struct {
	TableName                 string                    `json:"TableName"`
	Key                       Item                      `json:"Key"`
	ConditionExpression       string                    `json:"ConditionExpression,omitempty"`
	ExpressionAttributeNames  map[string]string         `json:"ExpressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]AttributeValue `json:"ExpressionAttributeValues,omitempty"`
}

// TransactPut represents a Put action in a transaction.
type TransactPut struct {
	TableName                 string                    `json:"TableName"`
	Item                      Item                      `json:"Item"`
	ConditionExpression       string                    `json:"ConditionExpression,omitempty"`
	ExpressionAttributeNames  map[string]string         `json:"ExpressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]AttributeValue `json:"ExpressionAttributeValues,omitempty"`
}

// TransactUpdate represents an Update action in a transaction.
type TransactUpdate struct {
	TableName                 string                    `json:"TableName"`
	Key                       Item                      `json:"Key"`
	UpdateExpression          string                    `json:"UpdateExpression"`
	ConditionExpression       string                    `json:"ConditionExpression,omitempty"`
	ExpressionAttributeNames  map[string]string         `json:"ExpressionAttributeNames,omitempty"`
	ExpressionAttributeValues map[string]AttributeValue `json:"ExpressionAttributeValues,omitempty"`
}

// TransactWriteItemsResponse is the response for TransactWriteItems.
type TransactWriteItemsResponse struct {
	// Empty on success - DynamoDB returns minimal response.
}

// TransactGetItemsRequest is the request for TransactGetItems.
type TransactGetItemsRequest struct {
	TransactItems          []TransactGetItem `json:"TransactItems"`
	ReturnConsumedCapacity string            `json:"ReturnConsumedCapacity,omitempty"`
}

// TransactGetItem represents a single item in a TransactGetItems request.
type TransactGetItem struct {
	Get *TransactGet `json:"Get"`
}

// TransactGet represents a Get action in a transaction.
type TransactGet struct {
	TableName                string            `json:"TableName"`
	Key                      Item              `json:"Key"`
	ProjectionExpression     string            `json:"ProjectionExpression,omitempty"`
	ExpressionAttributeNames map[string]string `json:"ExpressionAttributeNames,omitempty"`
}

// TransactGetItemsResponse is the response for TransactGetItems.
type TransactGetItemsResponse struct {
	Responses []TransactGetItemResponse `json:"Responses"`
}

// TransactGetItemResponse wraps a single item in the TransactGetItems response.
type TransactGetItemResponse struct {
	Item Item `json:"Item,omitempty"`
}

// CancellationReason represents a reason for transaction cancellation.
type CancellationReason struct {
	Code    string `json:"Code"`
	Message string `json:"Message,omitempty"`
}

// TransactionCanceledResponse represents the error response for canceled transactions.
type TransactionCanceledResponse struct {
	Type                string               `json:"__type"`
	Message             string               `json:"message"`
	CancellationReasons []CancellationReason `json:"CancellationReasons"`
}

// BatchWriteItemRequest is the request for BatchWriteItem.
type BatchWriteItemRequest struct {
	RequestItems map[string][]WriteRequest `json:"RequestItems"`
}

// WriteRequest represents a single write request in a batch.
type WriteRequest struct {
	PutRequest    *BatchPutRequest    `json:"PutRequest,omitempty"`
	DeleteRequest *BatchDeleteRequest `json:"DeleteRequest,omitempty"`
}

// BatchPutRequest represents a put request in a batch write.
type BatchPutRequest struct {
	Item Item `json:"Item"`
}

// BatchDeleteRequest represents a delete request in a batch write.
type BatchDeleteRequest struct {
	Key Item `json:"Key"`
}

// BatchWriteItemResponse is the response for BatchWriteItem.
type BatchWriteItemResponse struct {
	UnprocessedItems map[string][]WriteRequest `json:"UnprocessedItems,omitempty"`
}

// BatchGetItemRequest is the request for BatchGetItem.
type BatchGetItemRequest struct {
	RequestItems map[string]KeysAndAttributes `json:"RequestItems"`
}

// KeysAndAttributes represents keys and optional projection for batch get.
type KeysAndAttributes struct {
	Keys                     []Item            `json:"Keys"`
	ProjectionExpression     string            `json:"ProjectionExpression,omitempty"`
	ExpressionAttributeNames map[string]string `json:"ExpressionAttributeNames,omitempty"`
	ConsistentRead           bool              `json:"ConsistentRead,omitempty"`
}

// BatchGetItemResponse is the response for BatchGetItem.
type BatchGetItemResponse struct {
	Responses       map[string][]Item            `json:"Responses,omitempty"`
	UnprocessedKeys map[string]KeysAndAttributes `json:"UnprocessedKeys,omitempty"`
}

// ErrorResponse represents a DynamoDB error response in JSON format.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// TableError represents a DynamoDB table error.
type TableError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *TableError) Error() string {
	return e.Message
}

// Tag represents a key-value tag for a DynamoDB resource.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// UpdateTableRequest is the request for UpdateTable.
type UpdateTableRequest struct {
	TableName string `json:"TableName"`
}

// ListTagsOfResourceRequest is the request for ListTagsOfResource.
type ListTagsOfResourceRequest struct {
	ResourceArn string `json:"ResourceArn"`
	NextToken   string `json:"NextToken,omitempty"`
}

// ListTagsOfResourceResponse is the response for ListTagsOfResource.
//
// AWS returns at minimum an empty array; clients (notably terraform-provider-aws)
// require the field to be present even when no tags exist.
type ListTagsOfResourceResponse struct {
	Tags      []Tag  `json:"Tags"`
	NextToken string `json:"NextToken,omitempty"`
}

// TagResourceRequest is the request for TagResource.
type TagResourceRequest struct {
	ResourceArn string `json:"ResourceArn"`
	Tags        []Tag  `json:"Tags"`
}

// UntagResourceRequest is the request for UntagResource.
type UntagResourceRequest struct {
	ResourceArn string   `json:"ResourceArn"`
	TagKeys     []string `json:"TagKeys"`
}

// DescribeContinuousBackupsRequest is the request for DescribeContinuousBackups.
type DescribeContinuousBackupsRequest struct {
	TableName string `json:"TableName"`
}

// DescribeContinuousBackupsResponse is the response for DescribeContinuousBackups.
type DescribeContinuousBackupsResponse struct {
	ContinuousBackupsDescription ContinuousBackupsDescription `json:"ContinuousBackupsDescription"`
}

// ContinuousBackupsDescription describes the continuous backups status.
type ContinuousBackupsDescription struct {
	ContinuousBackupsStatus        string                         `json:"ContinuousBackupsStatus"`
	PointInTimeRecoveryDescription PointInTimeRecoveryDescription `json:"PointInTimeRecoveryDescription"`
}

// PointInTimeRecoveryDescription describes the PITR status.
type PointInTimeRecoveryDescription struct {
	PointInTimeRecoveryStatus string `json:"PointInTimeRecoveryStatus"`
}
