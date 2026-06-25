package s3

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

// ObjectLockConfiguration is the XML body of PUT/GET /{bucket}?object-lock,
// matching S3's `ObjectLockConfiguration` schema.
//
// kumo doesn't actually enforce WORM retention at object-write/delete time —
// it just roundtrips the bucket's default-retention configuration so terraform
// resources like `aws_s3_bucket_object_lock_configuration` see consistent
// state. Object-level retention (PutObjectRetention) and legal hold
// (PutObjectLegalHold) are out of scope.
type ObjectLockConfiguration struct {
	XMLName           xml.Name        `xml:"ObjectLockConfiguration"`
	Xmlns             string          `xml:"xmlns,attr,omitempty"`
	ObjectLockEnabled string          `xml:"ObjectLockEnabled,omitempty"`
	Rule              *ObjectLockRule `xml:"Rule,omitempty"`
}

// ObjectLockRule wraps the bucket default retention rule.
type ObjectLockRule struct {
	DefaultRetention *DefaultRetention `xml:"DefaultRetention,omitempty"`
}

// DefaultRetention is the bucket's default retention period. Days and Years
// are mutually exclusive, so both are pointers — only the one supplied by the
// caller is serialized back.
type DefaultRetention struct {
	Mode  string `xml:"Mode,omitempty"`
	Days  *int   `xml:"Days,omitempty"`
	Years *int   `xml:"Years,omitempty"`
}

// PutObjectLockConfiguration handles PUT /{bucket}?object-lock.
//
// kumo does not require the bucket to have been created with object lock
// enabled (real S3 returns InvalidBucketState otherwise); the emulator stays
// permissive so terraform apply succeeds against an already-versioned bucket.
func (s *Service) PutObjectLockConfiguration(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeS3Error(w, r, "InvalidRequest", "Failed to read request body", http.StatusBadRequest)

		return
	}

	var cfg ObjectLockConfiguration
	if err = xml.Unmarshal(body, &cfg); err != nil {
		writeS3Error(w, r, "MalformedXML", fmt.Sprintf("ObjectLockConfiguration XML: %v", err), http.StatusBadRequest)

		return
	}

	if err := s.storage.PutBucketObjectLockConfiguration(r.Context(), bucket, &cfg); err != nil {
		handleBucketLevelError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetObjectLockConfiguration handles GET /{bucket}?object-lock.
func (s *Service) GetObjectLockConfiguration(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	cfg, err := s.storage.GetBucketObjectLockConfiguration(r.Context(), bucket)
	if err != nil {
		handleBucketLevelError(w, r, err)

		return
	}

	cfg.Xmlns = s3Namespace
	writeXMLResponse(w, cfg)
}

// MemoryStorage hooks for bucket object lock configuration.

// PutBucketObjectLockConfiguration stores the object lock configuration on the
// bucket.
func (s *MemoryStorage) PutBucketObjectLockConfiguration(_ context.Context, bucket string, cfg *ObjectLockConfiguration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	b.ObjectLockConfiguration = cfg

	s.saveLocked()

	return nil
}

// GetBucketObjectLockConfiguration returns the stored object lock
// configuration, or ObjectLockConfigurationNotFoundError if none has been set.
func (s *MemoryStorage) GetBucketObjectLockConfiguration(_ context.Context, bucket string) (*ObjectLockConfiguration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if b.ObjectLockConfiguration == nil {
		return nil, &BucketError{Code: "ObjectLockConfigurationNotFoundError", Message: "Object Lock configuration does not exist for this bucket", BucketName: bucket}
	}

	return b.ObjectLockConfiguration, nil
}
