// Package glue provides AWS Glue service emulation for kumo.
package glue

import (
	"encoding/json"
	"time"
)

// AWSTimestamp is a time.Time that marshals to Unix timestamp (float64).
// AWS APIs use Unix timestamps in JSON responses.
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

// Ptr returns a pointer to the AWSTimestamp.
func (t AWSTimestamp) Ptr() *AWSTimestamp {
	if t.IsZero() {
		return nil
	}

	return &t
}

// ToAWSTimestamp converts time.Time to AWSTimestamp.
func ToAWSTimestamp(t time.Time) AWSTimestamp {
	return AWSTimestamp{Time: t}
}

// ToAWSTimestampPtr converts *time.Time to *AWSTimestamp.
func ToAWSTimestampPtr(t *time.Time) *AWSTimestamp {
	if t == nil {
		return nil
	}

	return &AWSTimestamp{Time: *t}
}

// Database represents a Glue database.
type Database struct {
	Name            string
	Description     string
	LocationURI     string
	Parameters      map[string]string
	CreateTime      time.Time
	CatalogID       string
	CreateTableMode string
}

// DatabaseInput represents input for creating/updating a database.
type DatabaseInput struct {
	Name            string            `json:"Name"`
	Description     string            `json:"Description,omitempty"`
	LocationURI     string            `json:"LocationUri,omitempty"`
	Parameters      map[string]string `json:"Parameters,omitempty"`
	CreateTableMode string            `json:"CreateTableDefaultPermissions,omitempty"`
}

// Table represents a Glue table.
type Table struct {
	Name              string
	DatabaseName      string
	Description       string
	Owner             string
	CreateTime        time.Time
	UpdateTime        time.Time
	LastAccessTime    *time.Time
	LastAnalyzedTime  *time.Time
	Retention         int32
	StorageDescriptor *StorageDescriptor
	PartitionKeys     []Column
	ViewOriginalText  string
	ViewExpandedText  string
	TableType         string
	Parameters        map[string]string
	CatalogID         string
}

// TableInput represents input for creating/updating a table.
type TableInput struct {
	Name              string             `json:"Name"`
	Description       string             `json:"Description,omitempty"`
	Owner             string             `json:"Owner,omitempty"`
	Retention         int32              `json:"Retention,omitempty"`
	StorageDescriptor *StorageDescriptor `json:"StorageDescriptor,omitempty"`
	PartitionKeys     []Column           `json:"PartitionKeys,omitempty"`
	ViewOriginalText  string             `json:"ViewOriginalText,omitempty"`
	ViewExpandedText  string             `json:"ViewExpandedText,omitempty"`
	TableType         string             `json:"TableType,omitempty"`
	Parameters        map[string]string  `json:"Parameters,omitempty"`
}

// StorageDescriptor describes the physical storage of table data.
type StorageDescriptor struct {
	Columns                []Column          `json:"Columns,omitempty"`
	Location               string            `json:"Location,omitempty"`
	InputFormat            string            `json:"InputFormat,omitempty"`
	OutputFormat           string            `json:"OutputFormat,omitempty"`
	Compressed             bool              `json:"Compressed,omitempty"`
	NumberOfBuckets        int32             `json:"NumberOfBuckets,omitempty"`
	SerdeInfo              *SerDeInfo        `json:"SerdeInfo,omitempty"`
	BucketColumns          []string          `json:"BucketColumns,omitempty"`
	SortColumns            []SortColumn      `json:"SortColumns,omitempty"`
	Parameters             map[string]string `json:"Parameters,omitempty"`
	StoredAsSubDirectories bool              `json:"StoredAsSubDirectories,omitempty"`
}

// Column represents a column in a table.
type Column struct {
	Name       string            `json:"Name"`
	Type       string            `json:"Type,omitempty"`
	Comment    string            `json:"Comment,omitempty"`
	Parameters map[string]string `json:"Parameters,omitempty"`
}

// SerDeInfo contains serialization/deserialization information.
type SerDeInfo struct {
	Name                 string            `json:"Name,omitempty"`
	SerializationLibrary string            `json:"SerializationLibrary,omitempty"`
	Parameters           map[string]string `json:"Parameters,omitempty"`
}

// SortColumn specifies a column for sorting.
type SortColumn struct {
	Column    string `json:"Column"`
	SortOrder int32  `json:"SortOrder"`
}

// Job represents a Glue job.
type Job struct {
	Name                    string
	Description             string
	Role                    string
	Command                 *JobCommand
	DefaultArguments        map[string]string
	NonOverridableArguments map[string]string
	MaxRetries              int32
	AllocatedCapacity       int32
	Timeout                 int32
	MaxCapacity             float64
	WorkerType              string
	NumberOfWorkers         int32
	GlueVersion             string
	CreatedOn               time.Time
	LastModifiedOn          time.Time
	ExecutionProperty       *ExecutionProperty
}

// JobCommand specifies the job command.
type JobCommand struct {
	Name           string `json:"Name,omitempty"`
	ScriptLocation string `json:"ScriptLocation,omitempty"`
	PythonVersion  string `json:"PythonVersion,omitempty"`
	Runtime        string `json:"Runtime,omitempty"`
}

// ExecutionProperty specifies execution properties.
type ExecutionProperty struct {
	MaxConcurrentRuns int32 `json:"MaxConcurrentRuns,omitempty"`
}

// JobRun represents a job run.
type JobRun struct {
	ID                string
	Attempt           int32
	PreviousRunID     string
	TriggerName       string
	JobName           string
	StartedOn         time.Time
	LastModifiedOn    time.Time
	CompletedOn       *time.Time
	JobRunState       string
	Arguments         map[string]string
	ErrorMessage      string
	PredecessorRuns   []Predecessor
	AllocatedCapacity int32
	ExecutionTime     int32
	Timeout           int32
	MaxCapacity       float64
	WorkerType        string
	NumberOfWorkers   int32
	GlueVersion       string
}

// Predecessor represents a predecessor job run.
type Predecessor struct {
	JobName string `json:"JobName,omitempty"`
	RunID   string `json:"RunId,omitempty"`
}

// CreateDatabaseInput is the request for CreateDatabase.
type CreateDatabaseInput struct {
	CatalogID     string         `json:"CatalogId,omitempty"`
	DatabaseInput *DatabaseInput `json:"DatabaseInput"`
}

// GetDatabaseInput is the request for GetDatabase.
type GetDatabaseInput struct {
	CatalogID string `json:"CatalogId,omitempty"`
	Name      string `json:"Name"`
}

// GetDatabaseOutput is the response for GetDatabase.
type GetDatabaseOutput struct {
	Database *DatabaseResponse `json:"Database,omitempty"`
}

// DatabaseResponse represents a database in API responses.
type DatabaseResponse struct {
	Name            string            `json:"Name,omitempty"`
	Description     string            `json:"Description,omitempty"`
	LocationURI     string            `json:"LocationUri,omitempty"`
	Parameters      map[string]string `json:"Parameters,omitempty"`
	CreateTime      *AWSTimestamp     `json:"CreateTime,omitempty"`
	CatalogID       string            `json:"CatalogId,omitempty"`
	CreateTableMode string            `json:"CreateTableDefaultPermissions,omitempty"`
}

// GetDatabasesInput is the request for GetDatabases.
type GetDatabasesInput struct {
	CatalogID  string `json:"CatalogId,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
	MaxResults int32  `json:"MaxResults,omitempty"`
}

// GetDatabasesOutput is the response for GetDatabases.
type GetDatabasesOutput struct {
	DatabaseList []*DatabaseResponse `json:"DatabaseList,omitempty"`
	NextToken    string              `json:"NextToken,omitempty"`
}

// DeleteDatabaseInput is the request for DeleteDatabase.
type DeleteDatabaseInput struct {
	CatalogID string `json:"CatalogId,omitempty"`
	Name      string `json:"Name"`
}

// CreateTableInput is the request for CreateTable.
type CreateTableInput struct {
	CatalogID    string      `json:"CatalogId,omitempty"`
	DatabaseName string      `json:"DatabaseName"`
	TableInput   *TableInput `json:"TableInput"`
}

// GetTableInput is the request for GetTable.
type GetTableInput struct {
	CatalogID    string `json:"CatalogId,omitempty"`
	DatabaseName string `json:"DatabaseName"`
	Name         string `json:"Name"`
}

// GetTableOutput is the response for GetTable.
type GetTableOutput struct {
	Table *TableResponse `json:"Table,omitempty"`
}

// TableResponse represents a table in API responses.
type TableResponse struct {
	Name              string             `json:"Name,omitempty"`
	DatabaseName      string             `json:"DatabaseName,omitempty"`
	Description       string             `json:"Description,omitempty"`
	Owner             string             `json:"Owner,omitempty"`
	CreateTime        *AWSTimestamp      `json:"CreateTime,omitempty"`
	UpdateTime        *AWSTimestamp      `json:"UpdateTime,omitempty"`
	LastAccessTime    *AWSTimestamp      `json:"LastAccessTime,omitempty"`
	LastAnalyzedTime  *AWSTimestamp      `json:"LastAnalyzedTime,omitempty"`
	Retention         int32              `json:"Retention,omitempty"`
	StorageDescriptor *StorageDescriptor `json:"StorageDescriptor,omitempty"`
	PartitionKeys     []Column           `json:"PartitionKeys,omitempty"`
	ViewOriginalText  string             `json:"ViewOriginalText,omitempty"`
	ViewExpandedText  string             `json:"ViewExpandedText,omitempty"`
	TableType         string             `json:"TableType,omitempty"`
	Parameters        map[string]string  `json:"Parameters,omitempty"`
	CatalogID         string             `json:"CatalogId,omitempty"`
}

// GetTablesInput is the request for GetTables.
type GetTablesInput struct {
	CatalogID    string `json:"CatalogId,omitempty"`
	DatabaseName string `json:"DatabaseName"`
	Expression   string `json:"Expression,omitempty"`
	NextToken    string `json:"NextToken,omitempty"`
	MaxResults   int32  `json:"MaxResults,omitempty"`
}

// GetTablesOutput is the response for GetTables.
type GetTablesOutput struct {
	TableList []*TableResponse `json:"TableList,omitempty"`
	NextToken string           `json:"NextToken,omitempty"`
}

// DeleteTableInput is the request for DeleteTable.
type DeleteTableInput struct {
	CatalogID    string `json:"CatalogId,omitempty"`
	DatabaseName string `json:"DatabaseName"`
	Name         string `json:"Name"`
}

// CreateJobInput is the request for CreateJob.
type CreateJobInput struct {
	Name                    string             `json:"Name"`
	Description             string             `json:"Description,omitempty"`
	Role                    string             `json:"Role"`
	Command                 *JobCommand        `json:"Command"`
	DefaultArguments        map[string]string  `json:"DefaultArguments,omitempty"`
	NonOverridableArguments map[string]string  `json:"NonOverridableArguments,omitempty"`
	MaxRetries              int32              `json:"MaxRetries,omitempty"`
	AllocatedCapacity       int32              `json:"AllocatedCapacity,omitempty"`
	Timeout                 int32              `json:"Timeout,omitempty"`
	MaxCapacity             float64            `json:"MaxCapacity,omitempty"`
	WorkerType              string             `json:"WorkerType,omitempty"`
	NumberOfWorkers         int32              `json:"NumberOfWorkers,omitempty"`
	GlueVersion             string             `json:"GlueVersion,omitempty"`
	ExecutionProperty       *ExecutionProperty `json:"ExecutionProperty,omitempty"`
}

// CreateJobOutput is the response for CreateJob.
type CreateJobOutput struct {
	Name string `json:"Name,omitempty"`
}

// DeleteJobInput is the request for DeleteJob.
type DeleteJobInput struct {
	JobName string `json:"JobName"`
}

// DeleteJobOutput is the response for DeleteJob.
type DeleteJobOutput struct {
	JobName string `json:"JobName,omitempty"`
}

// StartJobRunInput is the request for StartJobRun.
type StartJobRunInput struct {
	JobName              string            `json:"JobName"`
	JobRunID             string            `json:"JobRunId,omitempty"`
	Arguments            map[string]string `json:"Arguments,omitempty"`
	AllocatedCapacity    int32             `json:"AllocatedCapacity,omitempty"`
	Timeout              int32             `json:"Timeout,omitempty"`
	MaxCapacity          float64           `json:"MaxCapacity,omitempty"`
	WorkerType           string            `json:"WorkerType,omitempty"`
	NumberOfWorkers      int32             `json:"NumberOfWorkers,omitempty"`
	NotificationProperty *NotificationProp `json:"NotificationProperty,omitempty"`
}

// NotificationProp specifies notification properties.
type NotificationProp struct {
	NotifyDelayAfter int32 `json:"NotifyDelayAfter,omitempty"`
}

// StartJobRunOutput is the response for StartJobRun.
type StartJobRunOutput struct {
	JobRunID string `json:"JobRunId,omitempty"`
}

// ErrorResponse represents a Glue error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error represents a Glue error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}

// GetTagsInput is the request for GetTags.
type GetTagsInput struct {
	ResourceArn string `json:"ResourceArn"`
}

// GetTagsOutput is the response for GetTags.
type GetTagsOutput struct {
	Tags map[string]string `json:"Tags"`
}

// TagResourceInput is the request for TagResource.
type TagResourceInput struct {
	ResourceArn string            `json:"ResourceArn"`
	TagsToAdd   map[string]string `json:"TagsToAdd"`
}

// UntagResourceInput is the request for UntagResource.
type UntagResourceInput struct {
	ResourceArn  string   `json:"ResourceArn"`
	TagsToRemove []string `json:"TagsToRemove"`
}
