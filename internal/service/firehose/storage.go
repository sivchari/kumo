package firehose

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// S3Putter is the minimal S3 write surface the delivery loop needs. It is
// satisfied by an adapter installed via SetS3Putter in cross-service wiring,
// avoiding a direct dependency on the s3 package (and an import cycle).
type S3Putter interface {
	PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error
}

// Storage defines the interface for Firehose storage operations.
type Storage interface {
	CreateDeliveryStream(ctx context.Context, input *CreateDeliveryStreamInput) (*DeliveryStream, error)
	DeleteDeliveryStream(ctx context.Context, name string, allowForceDelete bool) error
	DescribeDeliveryStream(ctx context.Context, name string, limit int32, exclusiveStartDestinationID string) (*DeliveryStream, error)
	ListDeliveryStreams(ctx context.Context, streamType, exclusiveStartName string, limit int32) ([]string, bool, error)
	PutRecord(ctx context.Context, streamName string, record Record) (string, error)
	PutRecordBatch(ctx context.Context, streamName string, records []Record) ([]PutRecordBatchResponseEntry, int32, error)
	UpdateDestination(ctx context.Context, input *UpdateDestinationInput) error
	ListTagsForDeliveryStream(ctx context.Context, name string) ([]Tag, error)
	TagDeliveryStream(ctx context.Context, name string, tags []Tag) error
	UntagDeliveryStream(ctx context.Context, name string, tagKeys []string) error
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
	mu      sync.RWMutex           `json:"-"`
	Streams map[string]*StreamData `json:"streams"`
	dataDir string

	// s3Putter is the installed S3 delivery target (nil until wiring runs).
	s3Putter S3Putter
	// flushCtx scopes the background flush loop; cancelled by Close.
	flushCtx    context.Context //nolint:containedctx // intentional: scopes the background flush goroutine to storage lifetime
	flushCancel context.CancelFunc
	flushOnce   sync.Once
}

// StreamData holds a delivery stream and its records.
type StreamData struct {
	Stream  *DeliveryStream   `json:"stream"`
	Records []StoredRecord    `json:"records"`
	Tags    map[string]string `json:"tags,omitempty"`
	// DeliveredCount is the number of leading Records already written to S3
	// (a high-water mark). Persisted so a restart does not re-deliver.
	DeliveredCount int `json:"deliveredCount,omitempty"`
	// DeliveryVersion is the 1-based object version embedded in the S3 key,
	// incremented per delivered object.
	DeliveryVersion int `json:"deliveryVersion,omitempty"`
}

// StoredRecord holds a stored record.
type StoredRecord struct {
	RecordID string    `json:"recordId"`
	Data     []byte    `json:"data"`
	Received time.Time `json:"received"`
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	ctx, cancel := context.WithCancel(context.Background())

	s := &MemoryStorage{
		Streams:     make(map[string]*StreamData),
		flushCtx:    ctx,
		flushCancel: cancel,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "firehose", s)
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

	if s.Streams == nil {
		s.Streams = make(map[string]*StreamData)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "firehose", s.MarshalJSON)
}

// Close stops the background flush loop, performs a final synchronous flush of
// any buffered records to S3, then saves the storage state to disk if
// persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.flushCancel != nil {
		s.flushCancel()
	}

	s.flushAll(true)

	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "firehose", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateDeliveryStream creates a new delivery stream.
func (s *MemoryStorage) CreateDeliveryStream(_ context.Context, input *CreateDeliveryStreamInput) (*DeliveryStream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Streams[input.DeliveryStreamName]; exists {
		return nil, &Error{
			Code:    errResourceInUse,
			Message: fmt.Sprintf("Delivery stream %s already exists", input.DeliveryStreamName),
		}
	}

	stream := s.buildDeliveryStream(input)
	s.Streams[input.DeliveryStreamName] = &StreamData{
		Stream:  stream,
		Records: make([]StoredRecord, 0),
		Tags:    tagsToMap(input.Tags),
	}

	s.saveLocked()

	return stream, nil
}

// tagsToMap converts a slice of tags to a key-value map.
func tagsToMap(tags []Tag) map[string]string {
	if len(tags) == 0 {
		return nil
	}

	m := make(map[string]string, len(tags))
	for _, t := range tags {
		m[t.Key] = t.Value
	}

	return m
}

func (s *MemoryStorage) buildDeliveryStream(input *CreateDeliveryStreamInput) *DeliveryStream {
	now := time.Now()

	streamType := DeliveryStreamTypeDirectPut
	if input.DeliveryStreamType != "" {
		streamType = DeliveryStreamType(input.DeliveryStreamType)
	}

	arn := fmt.Sprintf("arn:aws:firehose:us-east-1:000000000000:deliverystream/%s", input.DeliveryStreamName)

	stream := &DeliveryStream{
		DeliveryStreamName:   input.DeliveryStreamName,
		DeliveryStreamARN:    arn,
		DeliveryStreamStatus: DeliveryStreamStatusActive,
		DeliveryStreamType:   streamType,
		CreateTimestamp:      now,
		LastUpdateTimestamp:  now,
		VersionID:            "1",
		HasMoreDestinations:  false,
	}

	stream.Destinations = s.buildDestinations(input)

	if input.KinesisStreamSourceConfiguration != nil {
		stream.Source = &SourceDescription{
			KinesisStreamSourceDescription: &KinesisStreamSourceDescription{
				KinesisStreamARN:       input.KinesisStreamSourceConfiguration.KinesisStreamARN,
				RoleARN:                input.KinesisStreamSourceConfiguration.RoleARN,
				DeliveryStartTimestamp: now,
			},
		}
	}

	return stream
}

func (s *MemoryStorage) buildDestinations(input *CreateDeliveryStreamInput) []DestinationDescription {
	destID := uuid.New().String()
	destinations := make([]DestinationDescription, 0)

	if input.S3DestinationConfiguration != nil {
		dest := DestinationDescription{
			DestinationID: destID,
			S3DestinationDescription: &S3DestinationDescription{
				BucketARN:         input.S3DestinationConfiguration.BucketARN,
				Prefix:            input.S3DestinationConfiguration.Prefix,
				ErrorOutputPrefix: input.S3DestinationConfiguration.ErrorOutputPrefix,
				RoleARN:           input.S3DestinationConfiguration.RoleARN,
				BufferingHints:    input.S3DestinationConfiguration.BufferingHints,
				CompressionFormat: input.S3DestinationConfiguration.CompressionFormat,
				CloudWatchLogging: input.S3DestinationConfiguration.CloudWatchLogging,
			},
		}

		destinations = append(destinations, dest)
	}

	if input.ExtendedS3DestinationConfiguration != nil {
		dest := DestinationDescription{
			DestinationID: destID,
			ExtendedS3DestinationDescription: &ExtendedS3DestinationDescription{
				BucketARN:         input.ExtendedS3DestinationConfiguration.BucketARN,
				Prefix:            input.ExtendedS3DestinationConfiguration.Prefix,
				ErrorOutputPrefix: input.ExtendedS3DestinationConfiguration.ErrorOutputPrefix,
				RoleARN:           input.ExtendedS3DestinationConfiguration.RoleARN,
				BufferingHints:    input.ExtendedS3DestinationConfiguration.BufferingHints,
				CompressionFormat: input.ExtendedS3DestinationConfiguration.CompressionFormat,
				CloudWatchLogging: input.ExtendedS3DestinationConfiguration.CloudWatchLogging,
				ProcessingConfig:  input.ExtendedS3DestinationConfiguration.ProcessingConfig,
				S3BackupMode:      input.ExtendedS3DestinationConfiguration.S3BackupMode,
			},
		}

		destinations = append(destinations, dest)
	}

	// If no destination is provided, create a default.
	if len(destinations) == 0 {
		destinations = append(destinations, DestinationDescription{
			DestinationID: destID,
		})
	}

	return destinations
}

// DeleteDeliveryStream deletes a delivery stream.
func (s *MemoryStorage) DeleteDeliveryStream(_ context.Context, name string, _ bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Streams[name]; !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Delivery stream %s not found", name),
		}
	}

	delete(s.Streams, name)

	s.saveLocked()

	return nil
}

// DescribeDeliveryStream describes a delivery stream.
func (s *MemoryStorage) DescribeDeliveryStream(_ context.Context, name string, _ int32, _ string) (*DeliveryStream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.Streams[name]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Delivery stream %s not found", name),
		}
	}

	return data.Stream, nil
}

// ListDeliveryStreams lists delivery streams.
func (s *MemoryStorage) ListDeliveryStreams(_ context.Context, streamType, exclusiveStartName string, limit int32) ([]string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit == 0 {
		limit = 10
	}

	if limit > 10000 {
		limit = 10000
	}

	names := s.collectStreamNames(streamType)

	sort.Strings(names)

	startIdx := s.findStartIndex(names, exclusiveStartName)
	if startIdx >= len(names) {
		return []string{}, false, nil
	}

	endIdx := startIdx + int(limit)
	hasMore := false

	if endIdx > len(names) {
		endIdx = len(names)
	} else {
		hasMore = endIdx < len(names)
	}

	return names[startIdx:endIdx], hasMore, nil
}

func (s *MemoryStorage) collectStreamNames(streamType string) []string {
	names := make([]string, 0, len(s.Streams))

	for name, data := range s.Streams {
		if streamType != "" && string(data.Stream.DeliveryStreamType) != streamType {
			continue
		}

		names = append(names, name)
	}

	return names
}

func (s *MemoryStorage) findStartIndex(names []string, exclusiveStartName string) int {
	if exclusiveStartName == "" {
		return 0
	}

	for i, name := range names {
		if name == exclusiveStartName {
			return i + 1
		}
	}

	return 0
}

// PutRecord puts a record to a delivery stream.
func (s *MemoryStorage) PutRecord(_ context.Context, streamName string, record Record) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.Streams[streamName]
	if !exists {
		return "", &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Delivery stream %s not found", streamName),
		}
	}

	recordID := uuid.New().String()

	data.Records = append(data.Records, StoredRecord{
		RecordID: recordID,
		Data:     record.Data,
		Received: time.Now(),
	})

	s.saveLocked()

	return recordID, nil
}

// PutRecordBatch puts multiple records to a delivery stream.
func (s *MemoryStorage) PutRecordBatch(_ context.Context, streamName string, records []Record) ([]PutRecordBatchResponseEntry, int32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.Streams[streamName]
	if !exists {
		return nil, 0, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Delivery stream %s not found", streamName),
		}
	}

	responses := make([]PutRecordBatchResponseEntry, len(records))
	now := time.Now()

	for i, record := range records {
		recordID := uuid.New().String()

		data.Records = append(data.Records, StoredRecord{
			RecordID: recordID,
			Data:     record.Data,
			Received: now,
		})

		responses[i] = PutRecordBatchResponseEntry{
			RecordID: recordID,
		}
	}

	s.saveLocked()

	return responses, 0, nil
}

// updateDestinationLocked finds and updates the matching destination in the stream.
// Must be called under lock.
func (s *MemoryStorage) updateDestinationLocked(data *StreamData, input *UpdateDestinationInput) error {
	for i, dest := range data.Stream.Destinations {
		if dest.DestinationID != input.DestinationID {
			continue
		}

		if input.S3DestinationUpdate != nil {
			if dest.S3DestinationDescription == nil {
				dest.S3DestinationDescription = &S3DestinationDescription{}
			}

			s.applyS3Update(dest.S3DestinationDescription, input.S3DestinationUpdate)
		}

		if input.ExtendedS3DestinationUpdate != nil {
			if dest.ExtendedS3DestinationDescription == nil {
				dest.ExtendedS3DestinationDescription = &ExtendedS3DestinationDescription{}
			}

			s.applyExtendedS3Update(dest.ExtendedS3DestinationDescription, input.ExtendedS3DestinationUpdate)
		}

		data.Stream.Destinations[i] = dest

		return nil
	}

	return &Error{
		Code:    errInvalidArgument,
		Message: fmt.Sprintf("Destination %s not found", input.DestinationID),
	}
}

// UpdateDestination updates a destination.
func (s *MemoryStorage) UpdateDestination(_ context.Context, input *UpdateDestinationInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.Streams[input.DeliveryStreamName]
	if !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Delivery stream %s not found", input.DeliveryStreamName),
		}
	}

	if data.Stream.VersionID != input.CurrentDeliveryStreamVersionID {
		return &Error{
			Code:    errInvalidArgument,
			Message: "Invalid version ID",
		}
	}

	if err := s.updateDestinationLocked(data, input); err != nil {
		return err
	}

	versionNum, _ := strconv.Atoi(data.Stream.VersionID)
	data.Stream.VersionID = strconv.Itoa(versionNum + 1)
	data.Stream.LastUpdateTimestamp = time.Now()

	s.saveLocked()

	return nil
}

func (s *MemoryStorage) applyS3Update(desc *S3DestinationDescription, update *S3DestinationUpdate) {
	if update.BucketARN != "" {
		desc.BucketARN = update.BucketARN
	}

	if update.Prefix != "" {
		desc.Prefix = update.Prefix
	}

	if update.ErrorOutputPrefix != "" {
		desc.ErrorOutputPrefix = update.ErrorOutputPrefix
	}

	if update.RoleARN != "" {
		desc.RoleARN = update.RoleARN
	}

	if update.BufferingHints != nil {
		desc.BufferingHints = update.BufferingHints
	}

	if update.CompressionFormat != "" {
		desc.CompressionFormat = update.CompressionFormat
	}

	if update.CloudWatchLogging != nil {
		desc.CloudWatchLogging = update.CloudWatchLogging
	}
}

func (s *MemoryStorage) applyExtendedS3Update(desc *ExtendedS3DestinationDescription, update *ExtendedS3DestinationUpdate) {
	if update.BucketARN != "" {
		desc.BucketARN = update.BucketARN
	}

	if update.Prefix != "" {
		desc.Prefix = update.Prefix
	}

	if update.ErrorOutputPrefix != "" {
		desc.ErrorOutputPrefix = update.ErrorOutputPrefix
	}

	if update.RoleARN != "" {
		desc.RoleARN = update.RoleARN
	}

	if update.BufferingHints != nil {
		desc.BufferingHints = update.BufferingHints
	}

	if update.CompressionFormat != "" {
		desc.CompressionFormat = update.CompressionFormat
	}

	if update.CloudWatchLogging != nil {
		desc.CloudWatchLogging = update.CloudWatchLogging
	}

	if update.ProcessingConfig != nil {
		desc.ProcessingConfig = update.ProcessingConfig
	}

	if update.S3BackupMode != "" {
		desc.S3BackupMode = update.S3BackupMode
	}
}

// ListTagsForDeliveryStream returns the tags of a delivery stream, sorted by key.
func (s *MemoryStorage) ListTagsForDeliveryStream(_ context.Context, name string) ([]Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.Streams[name]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Delivery stream %s not found", name),
		}
	}

	keys := make([]string, 0, len(data.Tags))
	for k := range data.Tags {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	tags := make([]Tag, 0, len(keys))
	for _, k := range keys {
		tags = append(tags, Tag{Key: k, Value: data.Tags[k]})
	}

	return tags, nil
}

// TagDeliveryStream adds or overwrites tags on a delivery stream by key.
func (s *MemoryStorage) TagDeliveryStream(_ context.Context, name string, tags []Tag) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.Streams[name]
	if !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Delivery stream %s not found", name),
		}
	}

	if data.Tags == nil {
		data.Tags = make(map[string]string, len(tags))
	}

	for _, t := range tags {
		data.Tags[t.Key] = t.Value
	}

	s.saveLocked()

	return nil
}

// UntagDeliveryStream removes tags from a delivery stream by key.
func (s *MemoryStorage) UntagDeliveryStream(_ context.Context, name string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.Streams[name]
	if !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Delivery stream %s not found", name),
		}
	}

	for _, k := range tagKeys {
		delete(data.Tags, k)
	}

	s.saveLocked()

	return nil
}
