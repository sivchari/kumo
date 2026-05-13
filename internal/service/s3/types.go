// Package s3 provides S3 service emulation for kumo.
package s3

import (
	"encoding/xml"
	"time"
)

// Bucket represents an S3 bucket.
type Bucket struct {
	Name         string
	CreationDate time.Time
}

// BucketLoggingStatus is the wire shape of {Put,Get}BucketLogging.
// LoggingEnabled is omitted (nil pointer) when logging is disabled —
// AWS sends `<BucketLoggingStatus/>` in that case.
type BucketLoggingStatus struct {
	XMLName        xml.Name              `xml:"BucketLoggingStatus"`
	Xmlns          string                `xml:"xmlns,attr,omitempty"`
	LoggingEnabled *LoggingEnabledStatus `xml:"LoggingEnabled,omitempty"`
}

// LoggingEnabledStatus is the body of LoggingEnabled. TargetGrants
// isn't modelled; terraform's aws_s3_bucket_logging only sets target
// + prefix.
type LoggingEnabledStatus struct {
	TargetBucket string `xml:"TargetBucket"`
	TargetPrefix string `xml:"TargetPrefix,omitempty"`
}

// Object represents an S3 object.
type Object struct {
	Key                  string
	Body                 []byte
	ETag                 string
	Size                 int64
	LastModified         time.Time
	ContentType          string
	Metadata             map[string]string
	Tags                 map[string]string
	VersionID            string
	IsDeleteMarker       bool
	ServerSideEncryption string
	SSEKMSKeyID          string
}

// Tagging represents the XML structure for S3 object tagging.
type Tagging struct {
	XMLName xml.Name `xml:"Tagging"`
	TagSet  TagSet   `xml:"TagSet"`
}

// TagSet represents a set of tags.
type TagSet struct {
	Tags []Tag `xml:"Tag"`
}

// Tag represents a single tag key-value pair.
type Tag struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// XML Response Types

// ListAllMyBucketsResult is the response for ListBuckets.
type ListAllMyBucketsResult struct {
	XMLName xml.Name `xml:"ListAllMyBucketsResult"`
	Xmlns   string   `xml:"xmlns,attr"`
	Buckets Buckets  `xml:"Buckets"`
	Owner   Owner    `xml:"Owner"`
}

// Owner represents the bucket owner.
type Owner struct {
	ID string `xml:"ID"`
}

// Buckets is a list of buckets.
type Buckets struct {
	Bucket []BucketInfo `xml:"Bucket"`
}

// BucketInfo represents bucket information in XML response.
type BucketInfo struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate"`
	BucketArn    string `xml:"BucketArn"`
}

// ListBucketResult is the response for ListObjectsV2.
type ListBucketResult struct {
	XMLName               xml.Name       `xml:"ListBucketResult"`
	Xmlns                 string         `xml:"xmlns,attr"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix"`
	KeyCount              int            `xml:"KeyCount"`
	MaxKeys               int            `xml:"MaxKeys"`
	IsTruncated           bool           `xml:"IsTruncated"`
	Contents              []ObjectInfo   `xml:"Contents"`
	ContinuationToken     string         `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string         `xml:"NextContinuationToken,omitempty"`
	StartAfter            string         `xml:"StartAfter,omitempty"`
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes,omitempty"`
}

// ObjectInfo represents object information in XML response.
type ObjectInfo struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

// CommonPrefix represents a common prefix in ListObjects response.
type CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

// ListBucketResultV1 is the response for the legacy ListObjects (V1)
// API. Same XML root name as V2, but the pagination fields are
// `Marker` / `NextMarker` instead of `ContinuationToken` /
// `NextContinuationToken`, and there is no `KeyCount`.
//
// The V1 API is still emitted by awscli's `aws s3 ls` (in some
// configurations), older AWS SDKs, and a handful of non-AWS S3
// clients (Go AWS SDK v1, some Java tooling), so it cannot be
// retired purely by emulating V2.
type ListBucketResultV1 struct {
	XMLName        xml.Name       `xml:"ListBucketResult"`
	Xmlns          string         `xml:"xmlns,attr"`
	Name           string         `xml:"Name"`
	Prefix         string         `xml:"Prefix"`
	Marker         string         `xml:"Marker"`
	NextMarker     string         `xml:"NextMarker,omitempty"`
	MaxKeys        int            `xml:"MaxKeys"`
	Delimiter      string         `xml:"Delimiter,omitempty"`
	IsTruncated    bool           `xml:"IsTruncated"`
	Contents       []ObjectInfo   `xml:"Contents"`
	CommonPrefixes []CommonPrefix `xml:"CommonPrefixes,omitempty"`
}

// ErrorResponse represents an S3 error response.
type ErrorResponse struct {
	XMLName    xml.Name `xml:"Error"`
	Code       string   `xml:"Code"`
	Message    string   `xml:"Message"`
	Resource   string   `xml:"Resource,omitempty"`
	RequestID  string   `xml:"RequestId"`
	BucketName string   `xml:"BucketName,omitempty"`
	Key        string   `xml:"Key,omitempty"`
}

// Versioning Types

// VersioningConfiguration represents bucket versioning configuration.
type VersioningConfiguration struct {
	XMLName xml.Name `xml:"VersioningConfiguration"`
	Xmlns   string   `xml:"xmlns,attr,omitempty"`
	Status  string   `xml:"Status,omitempty"`
}

// ListVersionsResult is the response for ListObjectVersions.
type ListVersionsResult struct {
	XMLName             xml.Name            `xml:"ListVersionsResult"`
	Xmlns               string              `xml:"xmlns,attr"`
	Name                string              `xml:"Name"`
	Prefix              string              `xml:"Prefix,omitempty"`
	KeyMarker           string              `xml:"KeyMarker,omitempty"`
	VersionIDMarker     string              `xml:"VersionIdMarker,omitempty"`
	NextKeyMarker       string              `xml:"NextKeyMarker,omitempty"`
	NextVersionIDMarker string              `xml:"NextVersionIdMarker,omitempty"`
	MaxKeys             int                 `xml:"MaxKeys"`
	IsTruncated         bool                `xml:"IsTruncated"`
	Versions            []ObjectVersionInfo `xml:"Version,omitempty"`
	DeleteMarkers       []DeleteMarkerInfo  `xml:"DeleteMarker,omitempty"`
	CommonPrefixes      []CommonPrefix      `xml:"CommonPrefixes,omitempty"`
}

// ObjectVersionInfo represents an object version in ListVersionsResult.
type ObjectVersionInfo struct {
	Key          string `xml:"Key"`
	VersionID    string `xml:"VersionId"`
	IsLatest     bool   `xml:"IsLatest"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
	Owner        Owner  `xml:"Owner"`
}

// DeleteMarkerInfo represents a delete marker in ListVersionsResult.
type DeleteMarkerInfo struct {
	Key          string `xml:"Key"`
	VersionID    string `xml:"VersionId"`
	IsLatest     bool   `xml:"IsLatest"`
	LastModified string `xml:"LastModified"`
	Owner        Owner  `xml:"Owner"`
}

// DeleteObjects Types

// DeleteRequest is the request body for DeleteObjects.
type DeleteRequest struct {
	XMLName xml.Name            `xml:"Delete"`
	Objects []DeleteObjectEntry `xml:"Object"`
	Quiet   bool                `xml:"Quiet"`
}

// DeleteObjectEntry represents an object to delete in a DeleteObjects request.
type DeleteObjectEntry struct {
	Key       string `xml:"Key"`
	VersionID string `xml:"VersionId,omitempty"`
}

// DeleteResult is the response for DeleteObjects.
type DeleteResult struct {
	XMLName xml.Name            `xml:"DeleteResult"`
	Xmlns   string              `xml:"xmlns,attr"`
	Deleted []DeletedObject     `xml:"Deleted,omitempty"`
	Errors  []DeleteObjectError `xml:"Error,omitempty"`
}

// DeletedObject represents a successfully deleted object.
type DeletedObject struct {
	Key                   string `xml:"Key"`
	VersionID             string `xml:"VersionId,omitempty"`
	DeleteMarker          bool   `xml:"DeleteMarker,omitempty"`
	DeleteMarkerVersionID string `xml:"DeleteMarkerVersionId,omitempty"`
}

// DeleteObjectError represents an error deleting an object.
type DeleteObjectError struct {
	Key       string `xml:"Key"`
	Code      string `xml:"Code"`
	Message   string `xml:"Message"`
	VersionID string `xml:"VersionId,omitempty"`
}

// Multipart Upload Types

// MultipartUpload represents an in-progress multipart upload.
type MultipartUpload struct {
	Bucket    string
	Key       string
	UploadID  string
	Initiated time.Time
	Parts     map[int]*Part // partNumber -> Part
}

// Part represents a part in a multipart upload.
type Part struct {
	PartNumber   int
	ETag         string
	Size         int64
	LastModified time.Time
	Body         []byte
}

// InitiateMultipartUploadResult is the response for CreateMultipartUpload.
type InitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadID string   `xml:"UploadId"`
}

// CompleteMultipartUploadRequest is the request body for CompleteMultipartUpload.
type CompleteMultipartUploadRequest struct {
	XMLName xml.Name      `xml:"CompleteMultipartUpload"`
	Parts   []PartRequest `xml:"Part"`
}

// PartRequest represents a part in the complete request.
type PartRequest struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

// CompleteMultipartUploadResult is the response for CompleteMultipartUpload.
type CompleteMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

// ListMultipartUploadsResult is the response for ListMultipartUploads.
type ListMultipartUploadsResult struct {
	XMLName            xml.Name       `xml:"ListMultipartUploadsResult"`
	Xmlns              string         `xml:"xmlns,attr"`
	Bucket             string         `xml:"Bucket"`
	KeyMarker          string         `xml:"KeyMarker,omitempty"`
	UploadIDMarker     string         `xml:"UploadIdMarker,omitempty"`
	NextKeyMarker      string         `xml:"NextKeyMarker,omitempty"`
	NextUploadIDMarker string         `xml:"NextUploadIdMarker,omitempty"`
	MaxUploads         int            `xml:"MaxUploads"`
	IsTruncated        bool           `xml:"IsTruncated"`
	Uploads            []UploadInfo   `xml:"Upload"`
	CommonPrefixes     []CommonPrefix `xml:"CommonPrefixes,omitempty"`
}

// UploadInfo represents a multipart upload in the list response.
type UploadInfo struct {
	Key       string `xml:"Key"`
	UploadID  string `xml:"UploadId"`
	Initiated string `xml:"Initiated"`
}

// ListPartsResult is the response for ListParts.
type ListPartsResult struct {
	XMLName              xml.Name   `xml:"ListPartsResult"`
	Xmlns                string     `xml:"xmlns,attr"`
	Bucket               string     `xml:"Bucket"`
	Key                  string     `xml:"Key"`
	UploadID             string     `xml:"UploadId"`
	PartNumberMarker     int        `xml:"PartNumberMarker"`
	NextPartNumberMarker int        `xml:"NextPartNumberMarker"`
	MaxParts             int        `xml:"MaxParts"`
	IsTruncated          bool       `xml:"IsTruncated"`
	Parts                []PartInfo `xml:"Part"`
}

// PartInfo represents a part in the list parts response.
type PartInfo struct {
	PartNumber   int    `xml:"PartNumber"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
}

// CopyObjectResult is the response for CopyObject.
type CopyObjectResult struct {
	XMLName      xml.Name `xml:"CopyObjectResult"`
	ETag         string   `xml:"ETag"`
	LastModified string   `xml:"LastModified"`
}

// CopyPartResult is the response for UploadPartCopy.
type CopyPartResult struct {
	XMLName      xml.Name `xml:"CopyPartResult"`
	Xmlns        string   `xml:"xmlns,attr"`
	LastModified string   `xml:"LastModified"`
	ETag         string   `xml:"ETag"`
}

// CopyRange is the resolved x-amz-copy-source-range — START..END inclusive.
// nil means "copy the whole source object".
type CopyRange struct {
	Start int64
	End   int64
}

// NotificationConfiguration represents S3 bucket notification configuration.
type NotificationConfiguration struct {
	XMLName           xml.Name           `xml:"NotificationConfiguration"`
	EventBridgeConfig *EventBridgeConfig `xml:"EventBridgeConfiguration,omitempty"`
}

// EventBridgeConfig represents EventBridge notification configuration.
type EventBridgeConfig struct{}

// CORSConfiguration represents S3 bucket CORS configuration (XML request body).
type CORSConfiguration struct {
	XMLName   xml.Name   `xml:"CORSConfiguration"`
	CORSRules []CORSRule `xml:"CORSRule"`
}

// CORSRule represents a single CORS rule.
type CORSRule struct {
	AllowedHeaders []string `json:"allowedHeaders,omitempty" xml:"AllowedHeader"`
	AllowedMethods []string `json:"allowedMethods"           xml:"AllowedMethod"`
	AllowedOrigins []string `json:"allowedOrigins"           xml:"AllowedOrigin"`
	ExposeHeaders  []string `json:"exposeHeaders,omitempty"  xml:"ExposeHeader"`
	MaxAgeSeconds  int      `json:"maxAgeSeconds,omitempty"  xml:"MaxAgeSeconds"`
}

// PublicAccessBlockConfiguration is the request/response body for the
// public access block APIs.
type PublicAccessBlockConfiguration struct {
	XMLName               xml.Name `xml:"PublicAccessBlockConfiguration"`
	Xmlns                 string   `xml:"xmlns,attr,omitempty"`
	BlockPublicAcls       bool     `xml:"BlockPublicAcls"`
	IgnorePublicAcls      bool     `xml:"IgnorePublicAcls"`
	BlockPublicPolicy     bool     `xml:"BlockPublicPolicy"`
	RestrictPublicBuckets bool     `xml:"RestrictPublicBuckets"`
}

// ServerSideEncryptionConfiguration is the request/response body for the
// bucket-encryption APIs.
type ServerSideEncryptionConfiguration struct {
	XMLName xml.Name                    `xml:"ServerSideEncryptionConfiguration"`
	Xmlns   string                      `xml:"xmlns,attr,omitempty"`
	Rules   []ServerSideEncryptionRuleX `xml:"Rule"`
}

// ServerSideEncryptionRuleX is the XML representation of a single SSE rule.
// (Suffixed with X to avoid colliding with storage's ServerSideEncryptionRule.)
type ServerSideEncryptionRuleX struct {
	ApplyServerSideEncryptionByDefault *ApplyServerSideEncryptionByDefault `xml:"ApplyServerSideEncryptionByDefault,omitempty"`
	BucketKeyEnabled                   bool                                `xml:"BucketKeyEnabled,omitempty"`
}

// ApplyServerSideEncryptionByDefault holds the default SSE algorithm and
// optional KMS key id.
type ApplyServerSideEncryptionByDefault struct {
	SSEAlgorithm   string `xml:"SSEAlgorithm"`
	KMSMasterKeyID string `xml:"KMSMasterKeyID,omitempty"`
}
