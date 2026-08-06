// Package s3tables provides S3 Tables service emulation for kumo.
package s3tables

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// TableBucket represents an S3 table bucket.
type TableBucket struct {
	Arn       string    `json:"arn"`
	ID        string    `json:"tableBucketId"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	OwnerID   string    `json:"ownerAccountId"`
	CreatedAt time.Time `json:"createdAt"`
}

// Table represents an S3 table.
type Table struct {
	Arn               string    `json:"tableARN"` //nolint:tagliatelle // AWS API uses tableARN
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	TableBucketArn    string    `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	Type              string    `json:"type"`
	Format            string    `json:"format"`
	VersionToken      string    `json:"versionToken"`
	MetadataLocation  string    `json:"metadataLocation,omitempty"`
	WarehouseLocation string    `json:"warehouseLocation,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	CreatedBy         string    `json:"createdBy"`
	ModifiedAt        time.Time `json:"modifiedAt"`
	ModifiedBy        string    `json:"modifiedBy"`
	OwnerID           string    `json:"ownerAccountId"`
}

// Namespace represents an S3 Tables namespace.
type Namespace struct {
	Namespace      []string  `json:"namespace"`
	TableBucketArn string    `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	OwnerID        string    `json:"ownerAccountId"`
	CreatedAt      time.Time `json:"createdAt"`
	CreatedBy      string    `json:"createdBy"`
}

// CreateTableBucketRequest represents a CreateTableBucket request.
type CreateTableBucketRequest struct {
	Name string `json:"name"`
}

// CreateTableBucketResponse represents a CreateTableBucket response.
type CreateTableBucketResponse struct {
	Arn string `json:"arn"`
}

// DeleteTableBucketRequest represents a DeleteTableBucket request.
type DeleteTableBucketRequest struct {
	TableBucketArn string `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
}

// GetTableBucketRequest represents a GetTableBucket request.
type GetTableBucketRequest struct {
	TableBucketArn string `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
}

// GetTableBucketResponse represents a GetTableBucket response.
type GetTableBucketResponse struct {
	Arn       string    `json:"arn"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"ownerAccountId"`
	CreatedAt time.Time `json:"createdAt"`
}

// ListTableBucketsRequest represents a ListTableBuckets request.
type ListTableBucketsRequest struct {
	ContinuationToken string `json:"continuationToken,omitempty"`
	MaxBuckets        int    `json:"maxBuckets,omitempty"`
	Prefix            string `json:"prefix,omitempty"`
}

// ListTableBucketsResponse represents a ListTableBuckets response.
type ListTableBucketsResponse struct {
	TableBuckets      []TableBucketSummary `json:"tableBuckets"`
	ContinuationToken string               `json:"continuationToken,omitempty"`
}

// TableBucketSummary represents a summary of a table bucket.
type TableBucketSummary struct {
	Arn       string    `json:"arn"`
	ID        string    `json:"tableBucketId,omitempty"`
	Name      string    `json:"name"`
	Type      string    `json:"type,omitempty"`
	OwnerID   string    `json:"ownerAccountId"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreateNamespaceRequest represents a CreateNamespace request.
type CreateNamespaceRequest struct {
	TableBucketArn string   `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	Namespace      []string `json:"namespace"`
}

// CreateNamespaceResponse represents a CreateNamespace response.
type CreateNamespaceResponse struct {
	Namespace      []string `json:"namespace"`
	TableBucketArn string   `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
}

// DeleteNamespaceRequest represents a DeleteNamespace request.
type DeleteNamespaceRequest struct {
	TableBucketArn string `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	Namespace      string `json:"namespace"`
}

// GetNamespaceRequest represents a GetNamespace request.
type GetNamespaceRequest struct {
	TableBucketArn string `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	Namespace      string `json:"namespace"`
}

// GetNamespaceResponse represents a GetNamespace response.
type GetNamespaceResponse struct {
	Namespace      []string  `json:"namespace"`
	TableBucketArn string    `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	OwnerID        string    `json:"ownerAccountId"`
	CreatedAt      time.Time `json:"createdAt"`
	CreatedBy      string    `json:"createdBy"`
}

// ListNamespacesRequest represents a ListNamespaces request.
type ListNamespacesRequest struct {
	TableBucketArn    string `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	ContinuationToken string `json:"continuationToken,omitempty"`
	MaxNamespaces     int    `json:"maxNamespaces,omitempty"`
	Prefix            string `json:"prefix,omitempty"`
}

// ListNamespacesResponse represents a ListNamespaces response.
type ListNamespacesResponse struct {
	Namespaces        []NamespaceSummary `json:"namespaces"`
	ContinuationToken string             `json:"continuationToken,omitempty"`
}

// NamespaceSummary represents a summary of a namespace.
type NamespaceSummary struct {
	Namespace []string  `json:"namespace"`
	CreatedAt time.Time `json:"createdAt"`
	CreatedBy string    `json:"createdBy"`
	OwnerID   string    `json:"ownerAccountId"`
}

// CreateTableRequest represents a CreateTable request.
type CreateTableRequest struct {
	TableBucketArn string `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	Namespace      string `json:"namespace"`
	Name           string `json:"name"`
	Format         string `json:"format"`
}

// CreateTableResponse represents a CreateTable response.
type CreateTableResponse struct {
	TableArn     string `json:"tableARN"` //nolint:tagliatelle // AWS API uses tableARN
	VersionToken string `json:"versionToken"`
}

// DeleteTableRequest represents a DeleteTable request.
type DeleteTableRequest struct {
	TableBucketArn string `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	Namespace      string `json:"namespace"`
	Name           string `json:"name"`
	VersionToken   string `json:"versionToken,omitempty"`
}

// GetTableRequest represents a GetTable request.
type GetTableRequest struct {
	TableBucketArn string `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	Namespace      string `json:"namespace"`
	Name           string `json:"name"`
}

// GetTableResponse represents a GetTable response.
type GetTableResponse struct {
	Arn               string    `json:"tableARN"` //nolint:tagliatelle // AWS API uses tableARN
	Name              string    `json:"name"`
	Namespace         []string  `json:"namespace"`
	TableBucketArn    string    `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	Type              string    `json:"type"`
	Format            string    `json:"format"`
	VersionToken      string    `json:"versionToken"`
	MetadataLocation  string    `json:"metadataLocation,omitempty"`
	WarehouseLocation string    `json:"warehouseLocation,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	CreatedBy         string    `json:"createdBy"`
	ModifiedAt        time.Time `json:"modifiedAt"`
	ModifiedBy        string    `json:"modifiedBy"`
	OwnerID           string    `json:"ownerAccountId"`
}

// ListTablesRequest represents a ListTables request.
type ListTablesRequest struct {
	TableBucketArn    string `json:"tableBucketARN"` //nolint:tagliatelle // AWS API uses tableBucketARN
	Namespace         string `json:"namespace,omitempty"`
	ContinuationToken string `json:"continuationToken,omitempty"`
	MaxTables         int    `json:"maxTables,omitempty"`
	Prefix            string `json:"prefix,omitempty"`
}

// ListTablesResponse represents a ListTables response.
type ListTablesResponse struct {
	Tables            []TableSummary `json:"tables"`
	ContinuationToken string         `json:"continuationToken,omitempty"`
}

// TableSummary represents a summary of a table.
type TableSummary struct {
	Arn        string    `json:"tableARN"` //nolint:tagliatelle // AWS API uses tableARN
	Name       string    `json:"name"`
	Namespace  []string  `json:"namespace"`
	Type       string    `json:"type"`
	CreatedAt  time.Time `json:"createdAt"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

// Error represents an S3 Tables error.
type Error = service.CodedError

// Error codes.
const (
	errNotFound      = "NotFoundException"
	errConflict      = "ConflictException"
	errBadRequest    = "BadRequestException"
	errInternalError = "InternalServerException"
)
