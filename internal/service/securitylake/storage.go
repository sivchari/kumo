package securitylake

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	errResourceNotFound = "ResourceNotFoundException"
	errConflict         = "ConflictException"
	errValidation       = "ValidationException"

	defaultRegion   = "us-east-1"
	statusCompleted = "COMPLETED"
	statusActive    = "ACTIVE"
)

// Storage is the interface for Security Lake storage operations.
type Storage interface {
	// Data Lake operations
	CreateDataLake(ctx context.Context, req *CreateDataLakeRequest) ([]*DataLake, error)
	DeleteDataLake(ctx context.Context, regions []string) error
	ListDataLakes(ctx context.Context, regions []string) ([]*DataLake, error)
	UpdateDataLake(ctx context.Context, req *UpdateDataLakeRequest) ([]*DataLake, error)

	// Subscriber operations
	CreateSubscriber(ctx context.Context, req *CreateSubscriberRequest) (*Subscriber, error)
	GetSubscriber(ctx context.Context, subscriberID string) (*Subscriber, error)
	DeleteSubscriber(ctx context.Context, subscriberID string) error
	UpdateSubscriber(ctx context.Context, req *UpdateSubscriberRequest) (*Subscriber, error)
	ListSubscribers(ctx context.Context, maxResults int, nextToken string) ([]*Subscriber, string, error)

	// Log Source operations
	CreateAwsLogSource(ctx context.Context, req *CreateAwsLogSourceRequest) ([]string, error)
	DeleteAwsLogSource(ctx context.Context, req *DeleteAwsLogSourceRequest) ([]string, error)
	ListLogSources(ctx context.Context, req *ListLogSourcesRequest) ([]*LogSource, string, error)

	// Tag operations
	TagResource(ctx context.Context, resourceARN string, tags []*Tag) error
	UntagResource(ctx context.Context, resourceARN string, tagKeys []string) error
	ListTagsForResource(ctx context.Context, resourceARN string) ([]*Tag, error)
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

// MemoryStorage implements in-memory storage for Security Lake.
type MemoryStorage struct {
	mu          sync.RWMutex           `json:"-"`
	DataLakes   map[string]*DataLake   `json:"dataLakes"`
	Subscribers map[string]*Subscriber `json:"subscribers"`
	LogSources  map[string]*LogSource  `json:"logSources"`
	Tags        map[string][]*Tag      `json:"tags"`
	accountID   string
	region      string
	dataDir     string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		DataLakes:   make(map[string]*DataLake),
		Subscribers: make(map[string]*Subscriber),
		LogSources:  make(map[string]*LogSource),
		Tags:        make(map[string][]*Tag),
		accountID:   "123456789012",
		region:      region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "securitylake", s)
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

	if s.DataLakes == nil {
		s.DataLakes = make(map[string]*DataLake)
	}

	if s.Subscribers == nil {
		s.Subscribers = make(map[string]*Subscriber)
	}

	if s.LogSources == nil {
		s.LogSources = make(map[string]*LogSource)
	}

	if s.Tags == nil {
		s.Tags = make(map[string][]*Tag)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "securitylake", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "securitylake", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateDataLake creates new data lakes.
func (s *MemoryStorage) CreateDataLake(_ context.Context, req *CreateDataLakeRequest) ([]*DataLake, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dataLakes := make([]*DataLake, 0, len(req.Configurations))

	for _, config := range req.Configurations {
		region := config.Region
		if region == "" {
			region = s.region
		}

		// Check if data lake already exists for this region
		if _, exists := s.DataLakes[region]; exists {
			return nil, &Error{
				Code:    errConflict,
				Message: fmt.Sprintf("Data lake already exists in region %s", region),
			}
		}

		bucketName := fmt.Sprintf("aws-security-data-lake-%s-%s-%s", s.region, s.accountID, uuid.New().String()[:8])
		arn := fmt.Sprintf("arn:aws:securitylake:%s:%s:data-lake/default", region, s.accountID)

		dataLake := &DataLake{
			ARN:                      arn,
			CreateStatus:             statusCompleted,
			EncryptionConfiguration:  config.EncryptionConfiguration,
			LifecycleConfiguration:   config.LifecycleConfiguration,
			Region:                   region,
			ReplicationConfiguration: config.ReplicationConfiguration,
			S3BucketARN:              fmt.Sprintf("arn:aws:s3:::%s", bucketName),
		}

		s.DataLakes[region] = dataLake

		if len(req.Tags) > 0 {
			s.Tags[arn] = req.Tags
		}

		dataLakes = append(dataLakes, dataLake)
	}

	s.saveLocked()

	return dataLakes, nil
}

// DeleteDataLake deletes data lakes.
func (s *MemoryStorage) DeleteDataLake(_ context.Context, regions []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, region := range regions {
		dataLake, exists := s.DataLakes[region]
		if !exists {
			return &Error{
				Code:    errResourceNotFound,
				Message: fmt.Sprintf("Data lake not found in region %s", region),
			}
		}

		delete(s.Tags, dataLake.ARN)
		delete(s.DataLakes, region)
	}

	s.saveLocked()

	return nil
}

// ListDataLakes lists data lakes.
func (s *MemoryStorage) ListDataLakes(_ context.Context, regions []string) ([]*DataLake, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dataLakes := make([]*DataLake, 0)

	if len(regions) == 0 {
		for _, dataLake := range s.DataLakes {
			dataLakes = append(dataLakes, dataLake)
		}
	} else {
		for _, region := range regions {
			if dataLake, exists := s.DataLakes[region]; exists {
				dataLakes = append(dataLakes, dataLake)
			}
		}
	}

	return dataLakes, nil
}

// UpdateDataLake updates data lakes.
func (s *MemoryStorage) UpdateDataLake(_ context.Context, req *UpdateDataLakeRequest) ([]*DataLake, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dataLakes := make([]*DataLake, 0, len(req.Configurations))

	for _, config := range req.Configurations {
		region := config.Region
		if region == "" {
			region = s.region
		}

		dataLake, exists := s.DataLakes[region]
		if !exists {
			return nil, &Error{
				Code:    errResourceNotFound,
				Message: fmt.Sprintf("Data lake not found in region %s", region),
			}
		}

		if config.EncryptionConfiguration != nil {
			dataLake.EncryptionConfiguration = config.EncryptionConfiguration
		}

		if config.LifecycleConfiguration != nil {
			dataLake.LifecycleConfiguration = config.LifecycleConfiguration
		}

		if config.ReplicationConfiguration != nil {
			dataLake.ReplicationConfiguration = config.ReplicationConfiguration
		}

		dataLakes = append(dataLakes, dataLake)
	}

	s.saveLocked()

	return dataLakes, nil
}

// CreateSubscriber creates a new subscriber.
func (s *MemoryStorage) CreateSubscriber(_ context.Context, req *CreateSubscriberRequest) (*Subscriber, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate subscriber name
	for _, sub := range s.Subscribers {
		if sub.SubscriberName == req.SubscriberName {
			return nil, &Error{
				Code:    errConflict,
				Message: fmt.Sprintf("Subscriber with name %s already exists", req.SubscriberName),
			}
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	subscriberID := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:securitylake:%s:%s:subscriber/%s", s.region, s.accountID, subscriberID)

	subscriber := &Subscriber{
		AccessTypes:           req.AccessTypes,
		CreatedAt:             now,
		Sources:               req.Sources,
		SubscriberARN:         arn,
		SubscriberDescription: req.SubscriberDescription,
		SubscriberID:          subscriberID,
		SubscriberIdentity:    req.SubscriberIdentity,
		SubscriberName:        req.SubscriberName,
		SubscriberStatus:      statusActive,
		UpdatedAt:             now,
	}

	s.Subscribers[subscriberID] = subscriber

	if len(req.Tags) > 0 {
		s.Tags[arn] = req.Tags
	}

	s.saveLocked()

	return subscriber, nil
}

// GetSubscriber retrieves a subscriber by ID.
func (s *MemoryStorage) GetSubscriber(_ context.Context, subscriberID string) (*Subscriber, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	subscriber, exists := s.Subscribers[subscriberID]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Subscriber with ID %s not found", subscriberID),
		}
	}

	return subscriber, nil
}

// DeleteSubscriber deletes a subscriber.
func (s *MemoryStorage) DeleteSubscriber(_ context.Context, subscriberID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	subscriber, exists := s.Subscribers[subscriberID]
	if !exists {
		return &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Subscriber with ID %s not found", subscriberID),
		}
	}

	delete(s.Tags, subscriber.SubscriberARN)
	delete(s.Subscribers, subscriberID)

	s.saveLocked()

	return nil
}

// UpdateSubscriber updates a subscriber.
func (s *MemoryStorage) UpdateSubscriber(_ context.Context, req *UpdateSubscriberRequest) (*Subscriber, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subscriber, exists := s.Subscribers[req.SubscriberID]
	if !exists {
		return nil, &Error{
			Code:    errResourceNotFound,
			Message: fmt.Sprintf("Subscriber with ID %s not found", req.SubscriberID),
		}
	}

	if req.SubscriberName != "" {
		subscriber.SubscriberName = req.SubscriberName
	}

	if req.SubscriberDescription != "" {
		subscriber.SubscriberDescription = req.SubscriberDescription
	}

	if req.SubscriberIdentity != nil {
		subscriber.SubscriberIdentity = req.SubscriberIdentity
	}

	if len(req.Sources) > 0 {
		subscriber.Sources = req.Sources
	}

	subscriber.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	s.saveLocked()

	return subscriber, nil
}

// ListSubscribers lists subscribers.
func (s *MemoryStorage) ListSubscribers(_ context.Context, maxResults int, _ string) ([]*Subscriber, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults == 0 {
		maxResults = 10
	}

	subscribers := make([]*Subscriber, 0, len(s.Subscribers))

	for _, subscriber := range s.Subscribers {
		subscribers = append(subscribers, subscriber)

		if len(subscribers) >= maxResults {
			break
		}
	}

	return subscribers, "", nil
}

// CreateAwsLogSource creates AWS log sources.
func (s *MemoryStorage) CreateAwsLogSource(_ context.Context, req *CreateAwsLogSourceRequest) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	failed := make([]string, 0)

	for _, source := range req.Sources {
		for _, region := range source.Regions {
			key := fmt.Sprintf("%s-%s-%s", source.SourceName, region, s.accountID)

			logSource := &LogSource{
				Account: s.accountID,
				Region:  region,
				Sources: []*LogSourceResource{
					{
						AwsLogSource: &AwsLogSourceResource{
							SourceName:    source.SourceName,
							SourceVersion: source.SourceVersion,
						},
					},
				},
			}

			s.LogSources[key] = logSource
		}
	}

	s.saveLocked()

	return failed, nil
}

// DeleteAwsLogSource deletes AWS log sources.
func (s *MemoryStorage) DeleteAwsLogSource(_ context.Context, req *DeleteAwsLogSourceRequest) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	failed := make([]string, 0)

	for _, source := range req.Sources {
		for _, region := range source.Regions {
			key := fmt.Sprintf("%s-%s-%s", source.SourceName, region, s.accountID)

			delete(s.LogSources, key)
		}
	}

	s.saveLocked()

	return failed, nil
}

// ListLogSources lists log sources.
func (s *MemoryStorage) ListLogSources(_ context.Context, req *ListLogSourcesRequest) ([]*LogSource, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	maxResults := req.MaxResults
	if maxResults == 0 {
		maxResults = 10
	}

	sources := make([]*LogSource, 0, len(s.LogSources))

	for _, source := range s.LogSources {
		// Filter by regions if specified
		if len(req.Regions) > 0 && !slices.Contains(req.Regions, source.Region) {
			continue
		}

		// Filter by accounts if specified
		if len(req.Accounts) > 0 && !slices.Contains(req.Accounts, source.Account) {
			continue
		}

		sources = append(sources, source)

		if len(sources) >= maxResults {
			break
		}
	}

	return sources, "", nil
}

// TagResource adds tags to a resource.
func (s *MemoryStorage) TagResource(_ context.Context, resourceARN string, tags []*Tag) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existingTags := s.Tags[resourceARN]
	tagMap := make(map[string]string)

	for _, tag := range existingTags {
		tagMap[tag.Key] = tag.Value
	}

	for _, tag := range tags {
		tagMap[tag.Key] = tag.Value
	}

	newTags := make([]*Tag, 0, len(tagMap))

	for k, v := range tagMap {
		newTags = append(newTags, &Tag{Key: k, Value: v})
	}

	s.Tags[resourceARN] = newTags

	s.saveLocked()

	return nil
}

// UntagResource removes tags from a resource.
func (s *MemoryStorage) UntagResource(_ context.Context, resourceARN string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existingTags := s.Tags[resourceARN]
	keySet := make(map[string]bool)

	for _, key := range tagKeys {
		keySet[key] = true
	}

	newTags := make([]*Tag, 0)

	for _, tag := range existingTags {
		if !keySet[tag.Key] {
			newTags = append(newTags, tag)
		}
	}

	s.Tags[resourceARN] = newTags

	s.saveLocked()

	return nil
}

// ListTagsForResource lists tags for a resource.
func (s *MemoryStorage) ListTagsForResource(_ context.Context, resourceARN string) ([]*Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := s.Tags[resourceARN]
	if tags == nil {
		tags = make([]*Tag, 0)
	}

	return tags, nil
}
