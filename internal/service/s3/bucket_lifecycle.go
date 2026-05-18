package s3

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

// LifecycleConfiguration is the XML body of PUT/GET
// /{bucket}?lifecycle, matching S3's `LifecycleConfiguration` schema.
//
// kumo doesn't actually evaluate the rules at object-write time (no
// background expiration / transition job) — it just roundtrips the
// configuration so terraform / cdk / pulumi resources like
// `aws_s3_bucket_lifecycle_configuration` see consistent state.
type LifecycleConfiguration struct {
	XMLName xml.Name        `xml:"LifecycleConfiguration"`
	Xmlns   string          `xml:"xmlns,attr,omitempty"`
	Rules   []LifecycleRule `xml:"Rule"`
}

// LifecycleRule is one rule in a LifecycleConfiguration.
type LifecycleRule struct {
	XMLName                        xml.Name                        `xml:"Rule"`
	ID                             string                          `xml:"ID,omitempty"`
	Status                         string                          `xml:"Status"`
	Filter                         *LifecycleFilter                `xml:"Filter,omitempty"`
	Prefix                         string                          `xml:"Prefix,omitempty"` // legacy, pre-Filter
	Expiration                     *LifecycleExpiration            `xml:"Expiration,omitempty"`
	NoncurrentVersionExpiration    *NoncurrentVersionExpiration    `xml:"NoncurrentVersionExpiration,omitempty"`
	Transitions                    []LifecycleTransition           `xml:"Transition,omitempty"`
	NoncurrentVersionTransitions   []NoncurrentVersionTransition   `xml:"NoncurrentVersionTransition,omitempty"`
	AbortIncompleteMultipartUpload *AbortIncompleteMultipartUpload `xml:"AbortIncompleteMultipartUpload,omitempty"`
}

// LifecycleFilter scopes a rule to a subset of the bucket.
type LifecycleFilter struct {
	Prefix                string              `xml:"Prefix,omitempty"`
	Tag                   *Tag                `xml:"Tag,omitempty"`
	ObjectSizeGreaterThan int64               `xml:"ObjectSizeGreaterThan,omitempty"`
	ObjectSizeLessThan    int64               `xml:"ObjectSizeLessThan,omitempty"`
	And                   *LifecycleFilterAnd `xml:"And,omitempty"`
}

// LifecycleFilterAnd combines multiple filter conditions (S3's AND).
type LifecycleFilterAnd struct {
	Prefix                string `xml:"Prefix,omitempty"`
	Tags                  []Tag  `xml:"Tag,omitempty"`
	ObjectSizeGreaterThan int64  `xml:"ObjectSizeGreaterThan,omitempty"`
	ObjectSizeLessThan    int64  `xml:"ObjectSizeLessThan,omitempty"`
}

// LifecycleExpiration controls when current-version objects are
// deleted by the lifecycle engine.
type LifecycleExpiration struct {
	Days                      int    `xml:"Days,omitempty"`
	Date                      string `xml:"Date,omitempty"`
	ExpiredObjectDeleteMarker bool   `xml:"ExpiredObjectDeleteMarker,omitempty"`
}

// NoncurrentVersionExpiration controls when noncurrent versions
// (only meaningful with versioning enabled) are deleted.
type NoncurrentVersionExpiration struct {
	NoncurrentDays          int `xml:"NoncurrentDays,omitempty"`
	NewerNoncurrentVersions int `xml:"NewerNoncurrentVersions,omitempty"`
}

// LifecycleTransition controls when current-version objects move to
// a colder storage class.
type LifecycleTransition struct {
	Days         int    `xml:"Days,omitempty"`
	Date         string `xml:"Date,omitempty"`
	StorageClass string `xml:"StorageClass"`
}

// NoncurrentVersionTransition is the noncurrent-version analogue of
// LifecycleTransition.
type NoncurrentVersionTransition struct {
	NoncurrentDays          int    `xml:"NoncurrentDays,omitempty"`
	NewerNoncurrentVersions int    `xml:"NewerNoncurrentVersions,omitempty"`
	StorageClass            string `xml:"StorageClass"`
}

// AbortIncompleteMultipartUpload — abort uploads stuck in progress.
type AbortIncompleteMultipartUpload struct {
	DaysAfterInitiation int `xml:"DaysAfterInitiation,omitempty"`
}

// PutBucketLifecycleConfiguration handles PUT /{bucket}?lifecycle.
func (s *Service) PutBucketLifecycleConfiguration(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeS3Error(w, r, "InvalidRequest", "Failed to read request body", http.StatusBadRequest)

		return
	}

	var cfg LifecycleConfiguration
	if err := xml.Unmarshal(body, &cfg); err != nil {
		writeS3Error(w, r, "MalformedXML", fmt.Sprintf("LifecycleConfiguration XML: %v", err), http.StatusBadRequest)

		return
	}

	if err := s.storage.PutBucketLifecycle(r.Context(), bucket, &cfg); err != nil {
		handleBucketLevelError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketLifecycleConfiguration handles GET /{bucket}?lifecycle.
func (s *Service) GetBucketLifecycleConfiguration(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	cfg, err := s.storage.GetBucketLifecycle(r.Context(), bucket)
	if err != nil {
		handleBucketLevelError(w, r, err)

		return
	}

	cfg.Xmlns = s3Namespace
	writeXMLResponse(w, cfg)
}

// DeleteBucketLifecycle handles DELETE /{bucket}?lifecycle.
func (s *Service) DeleteBucketLifecycle(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	if err := s.storage.DeleteBucketLifecycle(r.Context(), bucket); err != nil {
		handleBucketLevelError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// MemoryStorage hooks for bucket lifecycle.

// PutBucketLifecycle stores the lifecycle configuration on the bucket.
func (s *MemoryStorage) PutBucketLifecycle(_ context.Context, bucket string, cfg *LifecycleConfiguration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	b.Lifecycle = cfg

	return nil
}

// GetBucketLifecycle returns the stored lifecycle configuration, or
// NoSuchLifecycleConfiguration if none has been set.
func (s *MemoryStorage) GetBucketLifecycle(_ context.Context, bucket string) (*LifecycleConfiguration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if b.Lifecycle == nil {
		return nil, &BucketError{Code: "NoSuchLifecycleConfiguration", Message: "The lifecycle configuration does not exist", BucketName: bucket}
	}

	return b.Lifecycle, nil
}

// DeleteBucketLifecycle removes the lifecycle configuration from the
// bucket. Idempotent.
func (s *MemoryStorage) DeleteBucketLifecycle(_ context.Context, bucket string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	b.Lifecycle = nil

	return nil
}
