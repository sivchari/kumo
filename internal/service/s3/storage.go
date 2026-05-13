package s3

import (
	"context"
	"crypto/md5" //nolint:gosec // MD5 is required for S3 ETag calculation per AWS specification
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// Versioning status constants.
const (
	VersioningEnabled   = "Enabled"
	VersioningSuspended = "Suspended"
	VersionIDNull       = "null"
)

// Storage defines the S3 storage interface.
type Storage interface {
	// Bucket operations
	CreateBucket(ctx context.Context, name string) error
	DeleteBucket(ctx context.Context, name string) error
	ListBuckets(ctx context.Context) ([]Bucket, error)
	BucketExists(ctx context.Context, name string) (bool, error)

	// Object operations
	PutObject(ctx context.Context, bucket, key string, body io.Reader, metadata map[string]string) (*Object, error)
	GetObject(ctx context.Context, bucket, key string) (*Object, error)
	GetObjectVersion(ctx context.Context, bucket, key, versionID string) (*Object, error)
	DeleteObject(ctx context.Context, bucket, key string) (*Object, error)
	DeleteObjectVersion(ctx context.Context, bucket, key, versionID string) (*Object, error)
	HeadObject(ctx context.Context, bucket, key string) (*Object, error)
	ListObjects(ctx context.Context, bucket, prefix, delimiter string, maxKeys int) ([]Object, []string, error)

	// Versioning operations
	PutBucketVersioning(ctx context.Context, bucket, status string) error
	GetBucketVersioning(ctx context.Context, bucket string) (string, error)
	ListObjectVersions(ctx context.Context, bucket, prefix, delimiter string, maxKeys int) ([]Object, []string, error)

	// Multipart upload operations
	CreateMultipartUpload(ctx context.Context, bucket, key string) (*MultipartUpload, error)
	UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, body io.Reader) (*Part, error)
	CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []PartRequest) (*Object, error)
	AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error
	ListMultipartUploads(ctx context.Context, bucket, prefix string, maxUploads int) ([]*MultipartUpload, error)
	ListParts(ctx context.Context, bucket, key, uploadID string, maxParts int) ([]*Part, error)
	UploadPartCopy(ctx context.Context, dstBucket, dstKey, uploadID string, partNumber int, srcBucket, srcKey string, copyRange *CopyRange) (*Part, error)

	// Object tagging
	PutObjectTagging(ctx context.Context, bucket, key string, tags map[string]string) error
	GetObjectTagging(ctx context.Context, bucket, key string) (map[string]string, error)

	// Object ACL
	PutObjectACL(ctx context.Context, bucket, key string, acl *ObjectACL) error
	GetObjectACL(ctx context.Context, bucket, key string) (*ObjectACL, error)

	// Notification and CORS
	SetEventBridgeNotification(ctx context.Context, bucket string, enabled bool)
	IsEventBridgeEnabled(ctx context.Context, bucket string) bool
	SetCORSConfiguration(ctx context.Context, bucket string, rules []CORSRule)
	GetCORSRules(ctx context.Context, bucket string) []CORSRule

	PutPublicAccessBlock(ctx context.Context, bucket string, cfg PublicAccessBlockConfig) error
	GetPublicAccessBlock(ctx context.Context, bucket string) (*PublicAccessBlockConfig, error)
	DeletePublicAccessBlock(ctx context.Context, bucket string) error

	PutBucketEncryption(ctx context.Context, bucket string, cfg ServerSideEncryptionConfig) error
	GetBucketEncryption(ctx context.Context, bucket string) (*ServerSideEncryptionConfig, error)
	PutBucketPolicy(ctx context.Context, bucket, document string) error
	GetBucketPolicy(ctx context.Context, bucket string) (string, error)
	DeleteBucketPolicy(ctx context.Context, bucket string) error
	DeleteBucketEncryption(ctx context.Context, bucket string) error

	PutBucketLogging(ctx context.Context, bucket string, cfg BucketLoggingConfig) error
	GetBucketLogging(ctx context.Context, bucket string) (*BucketLoggingConfig, error)

	// Bucket website / lifecycle / object restore.
	PutBucketWebsite(ctx context.Context, bucket string, cfg *WebsiteConfiguration) error
	GetBucketWebsite(ctx context.Context, bucket string) (*WebsiteConfiguration, error)
	DeleteBucketWebsite(ctx context.Context, bucket string) error

	PutBucketLifecycle(ctx context.Context, bucket string, cfg *LifecycleConfiguration) error
	GetBucketLifecycle(ctx context.Context, bucket string) (*LifecycleConfiguration, error)
	DeleteBucketLifecycle(ctx context.Context, bucket string) error

	PutObjectRestore(ctx context.Context, bucket, key string, state *RestoreState) (bool, error)
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu      sync.RWMutex             `json:"-"`
	Buckets map[string]*MemoryBucket `json:"buckets"`
	dataDir string
}

// MemoryBucket holds the data for a single S3 bucket.
type MemoryBucket struct {
	Name               string                      `json:"name"`
	CreationDate       time.Time                   `json:"creationDate"`
	Objects            map[string]*Object          `json:"objects"`                     // current/latest version per key
	Versions           map[string][]*Object        `json:"versions"`                    // all versions per key (newest first)
	VersioningStatus   string                      `json:"versioningStatus"`            // "", "Enabled", "Suspended"
	VersionIDCounter   uint64                      `json:"versionIdcounter"`            // counter for generating version IDs
	MultipartUploads   map[string]*MultipartUpload `json:"-"`                           // uploadID -> MultipartUpload
	EventBridgeEnabled bool                        `json:"eventBridgeEnabled"`          // EventBridge notification
	CORSRules          []CORSRule                  `json:"corsRules,omitempty"`         // CORS configuration
	PublicAccessBlock  *PublicAccessBlockConfig    `json:"publicAccessBlock,omitempty"` // public access block configuration
	Encryption         *ServerSideEncryptionConfig `json:"encryption,omitempty"`        // server-side encryption configuration
	Policy             string                      `json:"policy,omitempty"`            // bucket policy JSON document (empty == not configured)
	Logging            *BucketLoggingConfig        `json:"logging,omitempty"`           // server access logging target (nil == disabled)
	ObjectACLs         map[string]*ObjectACL       `json:"objectAcls,omitempty"`        // per-object ACL (key -> ACL)
	Website            *WebsiteConfiguration       `json:"website,omitempty"`           // static-site-hosting configuration
	Lifecycle          *LifecycleConfiguration     `json:"lifecycle,omitempty"`         // expiration / transition rules
	ObjectRestores     map[string]*RestoreState    `json:"objectRestores,omitempty"`    // per-object restore state (key -> state)
}

// BucketLoggingConfig stores the destination for server access logs.
// AWS lets logging be opted out by sending an empty BucketLoggingStatus
// (no LoggingEnabled element); we represent that as a nil pointer on
// the bucket. TargetBucket == "" never lands in storage — the handler
// only persists configs with a target.
type BucketLoggingConfig struct {
	TargetBucket string `json:"targetBucket"`
	TargetPrefix string `json:"targetPrefix,omitempty"`
}

// PublicAccessBlockConfig stores the four PAB flags.
type PublicAccessBlockConfig struct {
	BlockPublicAcls       bool `json:"blockPublicAcls"`
	IgnorePublicAcls      bool `json:"ignorePublicAcls"`
	BlockPublicPolicy     bool `json:"blockPublicPolicy"`
	RestrictPublicBuckets bool `json:"restrictPublicBuckets"`
}

// ServerSideEncryptionConfig stores the bucket SSE configuration.
type ServerSideEncryptionConfig struct {
	Rules []ServerSideEncryptionRule `json:"rules"`
}

// ServerSideEncryptionRule represents a single SSE rule.
type ServerSideEncryptionRule struct {
	SSEAlgorithm     string `json:"sseAlgorithm"`               // AES256 / aws:kms / aws:kms:dsse
	KMSMasterKeyID   string `json:"kmsMasterKeyId,omitempty"`   // optional KMS key
	BucketKeyEnabled bool   `json:"bucketKeyEnabled,omitempty"` // S3 Bucket Keys
}

// NewMemoryStorage creates a new in-memory S3 storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Buckets: make(map[string]*MemoryBucket),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "s3", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(s)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Buckets == nil {
		s.Buckets = make(map[string]*MemoryBucket)
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "s3", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateBucket creates a new bucket.
func (s *MemoryStorage) CreateBucket(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Buckets[name]; exists {
		return &BucketError{Code: "BucketAlreadyOwnedByYou", Message: "Your previous request to create the named bucket succeeded and you already own it.", BucketName: name}
	}

	s.Buckets[name] = &MemoryBucket{
		Name:             name,
		CreationDate:     time.Now(),
		Objects:          make(map[string]*Object),
		Versions:         make(map[string][]*Object),
		VersioningStatus: "",
		MultipartUploads: make(map[string]*MultipartUpload),
	}

	return nil
}

// DeleteBucket deletes a bucket.
func (s *MemoryStorage) DeleteBucket(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bucket, exists := s.Buckets[name]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: name}
	}

	if len(bucket.Objects) > 0 {
		return &BucketError{Code: "BucketNotEmpty", Message: "The bucket you tried to delete is not empty", BucketName: name}
	}

	delete(s.Buckets, name)

	return nil
}

// ListBuckets returns all buckets.
func (s *MemoryStorage) ListBuckets(_ context.Context) ([]Bucket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	buckets := make([]Bucket, 0, len(s.Buckets))
	for _, b := range s.Buckets {
		buckets = append(buckets, Bucket{
			Name:         b.Name,
			CreationDate: b.CreationDate,
		})
	}

	// Sort by name for consistent ordering
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Name < buckets[j].Name
	})

	return buckets, nil
}

// BucketExists checks if a bucket exists.
func (s *MemoryStorage) BucketExists(_ context.Context, name string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.Buckets[name]

	return exists, nil
}

// PutObject stores an object.
func (s *MemoryStorage) PutObject(_ context.Context, bucket, key string, body io.Reader, metadata map[string]string) (*Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	hash := md5.Sum(data) //nolint:gosec // MD5 is required for S3 ETag calculation per AWS specification
	etag := hex.EncodeToString(hash[:])
	obj := &Object{
		Key:          key,
		Body:         data,
		ETag:         fmt.Sprintf("%q", etag),
		Size:         int64(len(data)),
		LastModified: time.Now(),
		Metadata:     metadata,
	}

	if metadata != nil {
		if ct, ok := metadata["Content-Type"]; ok {
			obj.ContentType = ct
		}

		applySSEMetadata(obj, metadata)
	}

	if obj.ContentType == "" {
		obj.ContentType = "application/octet-stream"
	}

	// Handle versioning
	switch b.VersioningStatus {
	case VersioningEnabled:
		// Generate version ID
		b.VersionIDCounter++
		obj.VersionID = fmt.Sprintf("v%d", b.VersionIDCounter)

		// Prepend to versions list (newest first)
		b.Versions[key] = append([]*Object{obj}, b.Versions[key]...)
	case VersioningSuspended:
		// For suspended versioning, use "null" version ID
		obj.VersionID = VersionIDNull

		// Remove any existing "null" version
		versions := b.Versions[key]
		newVersions := make([]*Object, 0, len(versions))

		for _, v := range versions {
			if v.VersionID != VersionIDNull {
				newVersions = append(newVersions, v)
			}
		}

		b.Versions[key] = append([]*Object{obj}, newVersions...)
	}

	// Always update current object
	b.Objects[key] = obj

	return obj, nil
}

// applySSEMetadata extracts SSE headers from metadata and sets them on the object.
func applySSEMetadata(obj *Object, metadata map[string]string) {
	obj.ServerSideEncryption = metadata["x-amz-server-side-encryption"]
	delete(metadata, "x-amz-server-side-encryption")

	obj.SSEKMSKeyID = metadata["x-amz-server-side-encryption-aws-kms-key-id"]
	delete(metadata, "x-amz-server-side-encryption-aws-kms-key-id")
}

// GetObject retrieves an object.
func (s *MemoryStorage) GetObject(_ context.Context, bucket, key string) (*Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	obj, exists := b.Objects[key]
	if !exists {
		return nil, &ObjectError{Code: "NoSuchKey", Message: "The specified key does not exist.", Key: key}
	}

	// Check if current version is a delete marker
	if obj.IsDeleteMarker {
		return nil, &ObjectError{Code: "NoSuchKey", Message: "The specified key does not exist.", Key: key}
	}

	return obj, nil
}

// GetObjectVersion retrieves a specific version of an object.
func (s *MemoryStorage) GetObjectVersion(_ context.Context, bucket, key, versionID string) (*Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	versions := b.Versions[key]
	for _, obj := range versions {
		if obj.VersionID == versionID {
			if obj.IsDeleteMarker {
				return nil, &ObjectError{Code: "MethodNotAllowed", Message: "The specified method is not allowed against this resource.", Key: key}
			}

			return obj, nil
		}
	}

	return nil, &ObjectError{Code: "NoSuchVersion", Message: "The specified version does not exist.", Key: key}
}

// PutObjectTagging sets tags on an object.
func (s *MemoryStorage) PutObjectTagging(_ context.Context, bucket, key string, tags map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bd, exists := s.Buckets[bucket]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist"}
	}

	obj, exists := bd.Objects[key]
	if !exists {
		return &BucketError{Code: "NoSuchKey", Message: "The specified key does not exist."}
	}

	obj.Tags = tags
	bd.Objects[key] = obj

	return nil
}

// GetObjectTagging retrieves tags from an object.
func (s *MemoryStorage) GetObjectTagging(_ context.Context, bucket, key string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bd, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist"}
	}

	obj, exists := bd.Objects[key]
	if !exists {
		return nil, &BucketError{Code: "NoSuchKey", Message: "The specified key does not exist."}
	}

	if obj.Tags == nil {
		return map[string]string{}, nil
	}

	return obj.Tags, nil
}

// DeleteObject deletes an object.
// Returns the deleted object (or delete marker for versioned buckets), or nil if non-versioned delete.
func (s *MemoryStorage) DeleteObject(_ context.Context, bucket, key string) (*Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	// Handle versioning - create delete marker for enabled buckets
	if b.VersioningStatus == VersioningEnabled {
		b.VersionIDCounter++
		deleteMarker := &Object{
			Key:            key,
			VersionID:      fmt.Sprintf("v%d", b.VersionIDCounter),
			IsDeleteMarker: true,
			LastModified:   time.Now(),
		}

		// Prepend delete marker to versions
		b.Versions[key] = append([]*Object{deleteMarker}, b.Versions[key]...)
		b.Objects[key] = deleteMarker

		return deleteMarker, nil
	}

	// For non-versioned or suspended buckets, just delete
	delete(b.Objects, key)

	// For suspended buckets, also remove "null" version
	if b.VersioningStatus == VersioningSuspended {
		versions := b.Versions[key]
		newVersions := make([]*Object, 0, len(versions))

		for _, v := range versions {
			if v.VersionID != VersionIDNull {
				newVersions = append(newVersions, v)
			}
		}

		if len(newVersions) == 0 {
			delete(b.Versions, key)
		} else {
			b.Versions[key] = newVersions
		}
	}

	// Return empty object for non-versioned delete (S3 returns 204 with no body)
	return &Object{Key: key}, nil
}

// DeleteObjectVersion deletes a specific version of an object.
func (s *MemoryStorage) DeleteObjectVersion(_ context.Context, bucket, key, versionID string) (*Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	versions := b.Versions[key]
	deletedObj, newVersions := filterOutVersion(versions, versionID)

	// S3 doesn't return error if version doesn't exist, returns empty object
	if deletedObj == nil {
		return &Object{Key: key, VersionID: versionID}, nil
	}

	if len(newVersions) == 0 {
		delete(b.Versions, key)
		delete(b.Objects, key)
	} else {
		b.Versions[key] = newVersions
		// Update current object to the newest version
		b.Objects[key] = newVersions[0]
	}

	return deletedObj, nil
}

// filterOutVersion removes a specific version from the versions list.
func filterOutVersion(versions []*Object, versionID string) (*Object, []*Object) {
	var deletedObj *Object

	newVersions := make([]*Object, 0, len(versions))

	for _, v := range versions {
		if v.VersionID == versionID {
			deletedObj = v
		} else {
			newVersions = append(newVersions, v)
		}
	}

	return deletedObj, newVersions
}

// HeadObject retrieves object metadata without body.
func (s *MemoryStorage) HeadObject(_ context.Context, bucket, key string) (*Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	obj, exists := b.Objects[key]
	if !exists {
		return nil, &ObjectError{Code: "NoSuchKey", Message: "The specified key does not exist.", Key: key}
	}

	// Return metadata only (no body)
	return &Object{
		Key:                  obj.Key,
		ContentType:          obj.ContentType,
		ETag:                 obj.ETag,
		Size:                 obj.Size,
		LastModified:         obj.LastModified,
		Metadata:             obj.Metadata,
		VersionID:            obj.VersionID,
		ServerSideEncryption: obj.ServerSideEncryption,
		SSEKMSKeyID:          obj.SSEKMSKeyID,
	}, nil
}

// ListObjects lists objects in a bucket.
func (s *MemoryStorage) ListObjects(_ context.Context, bucket, prefix, delimiter string, maxKeys int) ([]Object, []string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if maxKeys <= 0 {
		maxKeys = 1000
	}

	objects := make([]Object, 0)
	commonPrefixes := make(map[string]bool)

	// Collect all matching keys.
	keys := make([]string, 0, len(b.Objects))

	for key := range b.Objects {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}

	// Sort keys for consistent ordering
	sort.Strings(keys)

	for _, key := range keys {
		obj := b.Objects[key]

		// Handle delimiter
		if delimiter != "" {
			// Find the part after prefix
			remainder := strings.TrimPrefix(key, prefix)
			if idx := strings.Index(remainder, delimiter); idx >= 0 {
				// This is a common prefix
				commonPrefix := prefix + remainder[:idx+len(delimiter)]
				commonPrefixes[commonPrefix] = true

				continue
			}
		}

		objects = append(objects, Object{
			Key:          obj.Key,
			ETag:         obj.ETag,
			Size:         obj.Size,
			LastModified: obj.LastModified,
		})

		if len(objects) >= maxKeys {
			break
		}
	}

	// Convert common prefixes to sorted slice
	prefixList := make([]string, 0, len(commonPrefixes))
	for p := range commonPrefixes {
		prefixList = append(prefixList, p)
	}

	sort.Strings(prefixList)

	return objects, prefixList, nil
}

// PutBucketVersioning sets the versioning status of a bucket.
func (s *MemoryStorage) PutBucketVersioning(_ context.Context, bucket, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if status != VersioningEnabled && status != VersioningSuspended && status != "" {
		return &BucketError{Code: "MalformedXML", Message: "Invalid versioning status", BucketName: bucket}
	}

	b.VersioningStatus = status

	return nil
}

// GetBucketVersioning returns the versioning status of a bucket.
func (s *MemoryStorage) GetBucketVersioning(_ context.Context, bucket string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return "", &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	return b.VersioningStatus, nil
}

// ListObjectVersions lists all versions of objects in a bucket.
func (s *MemoryStorage) ListObjectVersions(_ context.Context, bucket, prefix, delimiter string, maxKeys int) ([]Object, []string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if maxKeys <= 0 {
		maxKeys = 1000
	}

	keys := collectVersionKeys(b, prefix)
	sort.Strings(keys)

	objects, commonPrefixes := processVersionKeys(b, keys, prefix, delimiter, maxKeys)
	prefixList := sortedPrefixList(commonPrefixes)

	return objects, prefixList, nil
}

// collectVersionKeys collects all keys that match the prefix from both versions and objects maps.
func collectVersionKeys(b *MemoryBucket, prefix string) []string {
	keySet := make(map[string]bool)

	for key := range b.Versions {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			keySet[key] = true
		}
	}

	for key := range b.Objects {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			keySet[key] = true
		}
	}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}

	return keys
}

// processVersionKeys processes keys and returns objects and common prefixes.
func processVersionKeys(b *MemoryBucket, keys []string, prefix, delimiter string, maxKeys int) ([]Object, map[string]bool) {
	objects := make([]Object, 0)
	commonPrefixes := make(map[string]bool)
	count := 0

	for _, key := range keys {
		if count >= maxKeys {
			break
		}

		// Handle delimiter for common prefixes
		if delimiter != "" {
			if cp := extractCommonPrefix(key, prefix, delimiter); cp != "" {
				commonPrefixes[cp] = true

				continue
			}
		}

		// Add versions for this key
		added := addKeyVersions(b, key, &objects, maxKeys-count)
		count += added
	}

	return objects, commonPrefixes
}

// extractCommonPrefix extracts common prefix if delimiter is found.
func extractCommonPrefix(key, prefix, delimiter string) string {
	remainder := strings.TrimPrefix(key, prefix)
	if idx := strings.Index(remainder, delimiter); idx >= 0 {
		return prefix + remainder[:idx+len(delimiter)]
	}

	return ""
}

// addKeyVersions adds all versions of a key to the objects slice.
func addKeyVersions(b *MemoryBucket, key string, objects *[]Object, limit int) int {
	versions := b.Versions[key]
	if len(versions) == 0 {
		// No versioning history, include current object if exists
		if obj, exists := b.Objects[key]; exists {
			*objects = append(*objects, objectToVersionInfo(obj))

			return 1
		}

		return 0
	}

	count := 0

	for _, obj := range versions {
		if count >= limit {
			break
		}

		*objects = append(*objects, objectToVersionInfo(obj))
		count++
	}

	return count
}

// objectToVersionInfo converts an Object to version info format.
func objectToVersionInfo(obj *Object) Object {
	return Object{
		Key:            obj.Key,
		VersionID:      obj.VersionID,
		ETag:           obj.ETag,
		Size:           obj.Size,
		LastModified:   obj.LastModified,
		IsDeleteMarker: obj.IsDeleteMarker,
	}
}

// sortedPrefixList converts a map of prefixes to a sorted slice.
func sortedPrefixList(prefixes map[string]bool) []string {
	list := make([]string, 0, len(prefixes))
	for p := range prefixes {
		list = append(list, p)
	}

	sort.Strings(list)

	return list
}

// BucketError represents an S3 bucket error.
type BucketError struct {
	Code       string
	Message    string
	BucketName string
}

func (e *BucketError) Error() string {
	return fmt.Sprintf("%s: %s (bucket: %s)", e.Code, e.Message, e.BucketName)
}

// ObjectError represents an S3 object error.
type ObjectError struct {
	Code    string
	Message string
	Key     string
}

func (e *ObjectError) Error() string {
	return fmt.Sprintf("%s: %s (key: %s)", e.Code, e.Message, e.Key)
}

// MultipartError represents an S3 multipart upload error.
type MultipartError struct {
	Code     string
	Message  string
	UploadID string
}

func (e *MultipartError) Error() string {
	return fmt.Sprintf("%s: %s (uploadId: %s)", e.Code, e.Message, e.UploadID)
}

// CreateMultipartUpload creates a new multipart upload.
func (s *MemoryStorage) CreateMultipartUpload(_ context.Context, bucket, key string) (*MultipartUpload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	uploadID := generateUploadID()
	upload := &MultipartUpload{
		Bucket:    bucket,
		Key:       key,
		UploadID:  uploadID,
		Initiated: time.Now(),
		Parts:     make(map[int]*Part),
	}

	b.MultipartUploads[uploadID] = upload

	return upload, nil
}

// UploadPart uploads a part of a multipart upload.
func (s *MemoryStorage) UploadPart(_ context.Context, bucket, key, uploadID string, partNumber int, body io.Reader) (*Part, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	upload, exists := b.MultipartUploads[uploadID]
	if !exists {
		return nil, &MultipartError{Code: "NoSuchUpload", Message: "The specified upload does not exist", UploadID: uploadID}
	}

	if upload.Key != key {
		return nil, &MultipartError{Code: "NoSuchUpload", Message: "The specified upload does not exist", UploadID: uploadID}
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	hash := md5.Sum(data) //nolint:gosec // MD5 is required for S3 ETag calculation per AWS specification
	etag := hex.EncodeToString(hash[:])

	part := &Part{
		PartNumber:   partNumber,
		ETag:         fmt.Sprintf("%q", etag),
		Size:         int64(len(data)),
		LastModified: time.Now(),
		Body:         data,
	}

	upload.Parts[partNumber] = part

	return part, nil
}

// UploadPartCopy copies bytes from an existing object into a part of an
// in-progress multipart upload. RFC-equivalent: AWS S3 UploadPartCopy.
// `copyRange` may be nil to copy the entire source object.
func (s *MemoryStorage) UploadPartCopy(_ context.Context, dstBucket, dstKey, uploadID string, partNumber int, srcBucket, srcKey string, copyRange *CopyRange) (*Part, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	srcB, exists := s.Buckets[srcBucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: srcBucket}
	}

	srcObj, exists := srcB.Objects[srcKey]
	if !exists {
		return nil, &ObjectError{Code: "NoSuchKey", Message: "The specified key does not exist.", Key: srcKey}
	}

	dstB, exists := s.Buckets[dstBucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: dstBucket}
	}

	upload, exists := dstB.MultipartUploads[uploadID]
	if !exists || upload.Key != dstKey {
		return nil, &MultipartError{Code: "NoSuchUpload", Message: "The specified upload does not exist", UploadID: uploadID}
	}

	data := srcObj.Body
	if copyRange != nil {
		size := int64(len(data))
		if copyRange.Start < 0 || copyRange.End >= size || copyRange.Start > copyRange.End {
			return nil, &MultipartError{Code: "InvalidArgument", Message: "Range out of source object bounds", UploadID: uploadID}
		}

		data = data[copyRange.Start : copyRange.End+1]
	}

	hash := md5.Sum(data) //nolint:gosec // MD5 is required for S3 ETag calculation per AWS specification
	etag := hex.EncodeToString(hash[:])

	part := &Part{
		PartNumber:   partNumber,
		ETag:         fmt.Sprintf("%q", etag),
		Size:         int64(len(data)),
		LastModified: time.Now(),
		Body:         append([]byte(nil), data...),
	}

	upload.Parts[partNumber] = part

	return part, nil
}

// CompleteMultipartUpload completes a multipart upload by assembling parts.
func (s *MemoryStorage) CompleteMultipartUpload(_ context.Context, bucket, key, uploadID string, parts []PartRequest) (*Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	upload, exists := b.MultipartUploads[uploadID]
	if !exists {
		return nil, &MultipartError{Code: "NoSuchUpload", Message: "The specified upload does not exist", UploadID: uploadID}
	}

	if upload.Key != key {
		return nil, &MultipartError{Code: "NoSuchUpload", Message: "The specified upload does not exist", UploadID: uploadID}
	}

	// Validate and assemble parts
	var combinedBody []byte

	for _, pr := range parts {
		part, ok := upload.Parts[pr.PartNumber]
		if !ok {
			return nil, &MultipartError{Code: "InvalidPart", Message: "One or more of the specified parts could not be found", UploadID: uploadID}
		}

		// Verify ETag matches
		if part.ETag != pr.ETag && part.ETag != fmt.Sprintf("%q", strings.Trim(pr.ETag, "\"")) {
			return nil, &MultipartError{Code: "InvalidPart", Message: "One or more of the specified parts could not be found", UploadID: uploadID}
		}

		combinedBody = append(combinedBody, part.Body...)
	}

	// Calculate final ETag (MD5 of MD5s + "-" + number of parts)
	etag := calculateMultipartETag(parts, upload.Parts)

	obj := &Object{
		Key:          key,
		Body:         combinedBody,
		ETag:         etag,
		Size:         int64(len(combinedBody)),
		LastModified: time.Now(),
		ContentType:  "application/octet-stream",
	}

	b.Objects[key] = obj
	delete(b.MultipartUploads, uploadID)

	return obj, nil
}

// AbortMultipartUpload aborts a multipart upload.
func (s *MemoryStorage) AbortMultipartUpload(_ context.Context, bucket, key, uploadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	upload, exists := b.MultipartUploads[uploadID]
	if !exists {
		return &MultipartError{Code: "NoSuchUpload", Message: "The specified upload does not exist", UploadID: uploadID}
	}

	if upload.Key != key {
		return &MultipartError{Code: "NoSuchUpload", Message: "The specified upload does not exist", UploadID: uploadID}
	}

	delete(b.MultipartUploads, uploadID)

	return nil
}

// ListMultipartUploads lists in-progress multipart uploads.
func (s *MemoryStorage) ListMultipartUploads(_ context.Context, bucket, prefix string, maxUploads int) ([]*MultipartUpload, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if maxUploads <= 0 {
		maxUploads = 1000
	}

	uploads := make([]*MultipartUpload, 0)

	for _, upload := range b.MultipartUploads {
		if prefix == "" || strings.HasPrefix(upload.Key, prefix) {
			uploads = append(uploads, upload)
		}

		if len(uploads) >= maxUploads {
			break
		}
	}

	// Sort by key and then by upload ID for consistent ordering
	sort.Slice(uploads, func(i, j int) bool {
		if uploads[i].Key != uploads[j].Key {
			return uploads[i].Key < uploads[j].Key
		}

		return uploads[i].UploadID < uploads[j].UploadID
	})

	return uploads, nil
}

// ListParts lists the parts that have been uploaded for a multipart upload.
func (s *MemoryStorage) ListParts(_ context.Context, bucket, key, uploadID string, maxParts int) ([]*Part, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	upload, exists := b.MultipartUploads[uploadID]
	if !exists {
		return nil, &MultipartError{Code: "NoSuchUpload", Message: "The specified upload does not exist", UploadID: uploadID}
	}

	if upload.Key != key {
		return nil, &MultipartError{Code: "NoSuchUpload", Message: "The specified upload does not exist", UploadID: uploadID}
	}

	if maxParts <= 0 {
		maxParts = 1000
	}

	parts := make([]*Part, 0, len(upload.Parts))
	for _, part := range upload.Parts {
		parts = append(parts, part)
	}

	// Sort by part number
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	// Limit to maxParts
	if len(parts) > maxParts {
		parts = parts[:maxParts]
	}

	return parts, nil
}

// generateUploadID generates a unique upload ID.
func generateUploadID() string {
	// Generate a UUID-based upload ID similar to AWS format
	return strings.ReplaceAll(fmt.Sprintf("%s%s", randomHex(8), randomHex(8)), "-", "")
}

// randomHex generates a random hex string.
func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() & 0xff)
		time.Sleep(time.Nanosecond)
	}

	return hex.EncodeToString(b)
}

// calculateMultipartETag calculates the ETag for a completed multipart upload.
// Format: "MD5-of-MD5s-N" where N is the number of parts.
func calculateMultipartETag(partRequests []PartRequest, parts map[int]*Part) string {
	const md5Size = 16 // MD5 produces 16 bytes

	// Concatenate all part ETags (raw MD5 values)
	md5Concat := make([]byte, 0, len(partRequests)*md5Size)

	for _, pr := range partRequests {
		part := parts[pr.PartNumber]
		// Extract raw MD5 from ETag (remove quotes)
		etag := strings.Trim(part.ETag, "\"")

		md5Bytes, _ := hex.DecodeString(etag)
		md5Concat = append(md5Concat, md5Bytes...)
	}

	// Calculate MD5 of concatenated MD5s
	finalHash := md5.Sum(md5Concat) //nolint:gosec // MD5 is required for S3 ETag calculation per AWS specification

	return fmt.Sprintf("%q", fmt.Sprintf("%s-%d", hex.EncodeToString(finalHash[:]), len(partRequests)))
}

// SetEventBridgeNotification enables or disables EventBridge notification for a bucket.
func (s *MemoryStorage) SetEventBridgeNotification(_ context.Context, bucket string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if b, exists := s.Buckets[bucket]; exists {
		b.EventBridgeEnabled = enabled
	}
}

// IsEventBridgeEnabled returns whether EventBridge notification is enabled for a bucket.
func (s *MemoryStorage) IsEventBridgeEnabled(_ context.Context, bucket string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if b, exists := s.Buckets[bucket]; exists {
		return b.EventBridgeEnabled
	}

	return false
}

// SetCORSConfiguration sets the CORS configuration for a bucket.
func (s *MemoryStorage) SetCORSConfiguration(_ context.Context, bucket string, rules []CORSRule) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if b, exists := s.Buckets[bucket]; exists {
		b.CORSRules = rules
	}
}

// GetCORSRules returns the CORS rules for a bucket.
func (s *MemoryStorage) GetCORSRules(_ context.Context, bucket string) []CORSRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if b, exists := s.Buckets[bucket]; exists {
		return b.CORSRules
	}

	return nil
}

// PutPublicAccessBlock stores a bucket's public access block configuration.
func (s *MemoryStorage) PutPublicAccessBlock(_ context.Context, bucket string, cfg PublicAccessBlockConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	c := cfg
	b.PublicAccessBlock = &c

	return nil
}

// GetPublicAccessBlock returns a bucket's public access block configuration.
func (s *MemoryStorage) GetPublicAccessBlock(_ context.Context, bucket string) (*PublicAccessBlockConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if b.PublicAccessBlock == nil {
		return nil, &BucketError{
			Code:       "NoSuchPublicAccessBlockConfiguration",
			Message:    "The public access block configuration was not found",
			BucketName: bucket,
		}
	}

	c := *b.PublicAccessBlock

	return &c, nil
}

// DeletePublicAccessBlock removes a bucket's public access block configuration.
func (s *MemoryStorage) DeletePublicAccessBlock(_ context.Context, bucket string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	b.PublicAccessBlock = nil

	return nil
}

// PutBucketEncryption stores a bucket's server-side encryption configuration.
func (s *MemoryStorage) PutBucketEncryption(_ context.Context, bucket string, cfg ServerSideEncryptionConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	c := ServerSideEncryptionConfig{Rules: append([]ServerSideEncryptionRule(nil), cfg.Rules...)}
	b.Encryption = &c

	return nil
}

// GetBucketEncryption returns a bucket's server-side encryption configuration.
func (s *MemoryStorage) GetBucketEncryption(_ context.Context, bucket string) (*ServerSideEncryptionConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if b.Encryption == nil {
		return nil, &BucketError{
			Code:       "ServerSideEncryptionConfigurationNotFoundError",
			Message:    "The server side encryption configuration was not found",
			BucketName: bucket,
		}
	}

	c := ServerSideEncryptionConfig{Rules: append([]ServerSideEncryptionRule(nil), b.Encryption.Rules...)}

	return &c, nil
}

// DeleteBucketEncryption removes a bucket's server-side encryption configuration.
func (s *MemoryStorage) DeleteBucketEncryption(_ context.Context, bucket string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	b.Encryption = nil

	return nil
}

// PutBucketPolicy stores a bucket's policy document. AWS treats the
// document as opaque JSON for the purposes of Put/Get, so we just
// persist the bytes — IAM-layer evaluation is out of scope.
func (s *MemoryStorage) PutBucketPolicy(_ context.Context, bucket, document string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	b.Policy = document

	return nil
}

// GetBucketPolicy returns the configured policy document. Returns
// NoSuchBucketPolicy when the bucket exists but has no policy set.
func (s *MemoryStorage) GetBucketPolicy(_ context.Context, bucket string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return "", &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if b.Policy == "" {
		return "", &BucketError{
			Code:       "NoSuchBucketPolicy",
			Message:    "The bucket policy does not exist",
			BucketName: bucket,
		}
	}

	return b.Policy, nil
}

// DeleteBucketPolicy clears the bucket's policy. AWS allows this on a
// bucket without a policy (returns 204), so a missing policy is not
// an error here either.
func (s *MemoryStorage) DeleteBucketPolicy(_ context.Context, bucket string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	b.Policy = ""

	return nil
}

// PutBucketLogging stores the server-access-log destination for a
// bucket. An empty TargetBucket means the caller is opting out, so we
// clear the config (matches AWS semantics where an empty
// BucketLoggingStatus body disables logging).
func (s *MemoryStorage) PutBucketLogging(_ context.Context, bucket string, cfg BucketLoggingConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if cfg.TargetBucket == "" {
		b.Logging = nil

		return nil
	}

	c := cfg
	b.Logging = &c

	return nil
}

// GetBucketLogging returns the configured destination, or nil when
// logging is disabled. Real AWS GET on an unconfigured bucket returns
// an empty <BucketLoggingStatus/> rather than NoSuch*; the handler
// renders that case from a nil result.
func (s *MemoryStorage) GetBucketLogging(_ context.Context, bucket string) (*BucketLoggingConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, exists := s.Buckets[bucket]
	if !exists {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if b.Logging == nil {
		return nil, nil //nolint:nilnil // documented contract: nil cfg + nil err == disabled
	}

	c := *b.Logging

	return &c, nil
}
