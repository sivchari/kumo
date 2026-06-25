package cloudtrail

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errTrailNotFound      = "TrailNotFoundException"
	errTrailAlreadyExists = "TrailAlreadyExistsException"
	errValidationError    = "ValidationException"
)

// Default values.
const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "123456789012"
)

// Storage defines the CloudTrail storage interface.
type Storage interface {
	CreateTrail(ctx context.Context, req *CreateTrailRequest) (*Trail, error)
	DeleteTrail(ctx context.Context, name string) error
	GetTrail(ctx context.Context, name string) (*Trail, error)
	DescribeTrails(ctx context.Context, names []string) ([]*Trail, error)
	StartLogging(ctx context.Context, name string) error
	StopLogging(ctx context.Context, name string) error
	LookupEvents(ctx context.Context, req *LookupEventsRequest) ([]*Event, string, error)
	GetTrailStatus(ctx context.Context, name string) (*Trail, error)
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
	mu        sync.RWMutex      `json:"-"`
	Trails    map[string]*Trail `json:"trails"`
	region    string
	accountID string
	dataDir   string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Trails:    make(map[string]*Trail),
		region:    region,
		accountID: defaultAccountID,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "cloudtrail", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (m *MemoryStorage) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(m)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (m *MemoryStorage) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(m)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if m.Trails == nil {
		m.Trails = make(map[string]*Trail)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "cloudtrail", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "cloudtrail", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateTrail creates a new trail.
func (m *MemoryStorage) CreateTrail(_ context.Context, req *CreateTrailRequest) (*Trail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, &Error{Code: errValidationError, Message: "Trail name is required"}
	}

	if req.S3BucketName == "" {
		return nil, &Error{Code: errValidationError, Message: "S3 bucket name is required"}
	}

	key := normalizeTrailName(req.Name)

	if _, exists := m.Trails[key]; exists {
		return nil, &Error{Code: errTrailAlreadyExists, Message: "Trail already exists"}
	}

	trail := &Trail{
		Name:                       key,
		TrailARN:                   generateTrailARN(m.region, m.accountID, key),
		S3BucketName:               req.S3BucketName,
		S3KeyPrefix:                req.S3KeyPrefix,
		IncludeGlobalServiceEvents: true,
		IsMultiRegionTrail:         false,
		HomeRegion:                 m.region,
		IsLogging:                  false,
		LogFileValidationEnabled:   false,
		CloudWatchLogsLogGroupArn:  req.CloudWatchLogsLogGroupArn,
		CloudWatchLogsRoleArn:      req.CloudWatchLogsRoleArn,
		KMSKeyID:                   req.KMSKeyID,
		HasCustomEventSelectors:    false,
		HasInsightSelectors:        false,
		IsOrganizationTrail:        false,
		CreationTime:               time.Now(),
	}

	if req.IncludeGlobalServiceEvents != nil {
		trail.IncludeGlobalServiceEvents = *req.IncludeGlobalServiceEvents
	}

	if req.IsMultiRegionTrail != nil {
		trail.IsMultiRegionTrail = *req.IsMultiRegionTrail
	}

	if req.EnableLogFileValidation != nil {
		trail.LogFileValidationEnabled = *req.EnableLogFileValidation
	}

	if req.IsOrganizationTrail != nil {
		trail.IsOrganizationTrail = *req.IsOrganizationTrail
	}

	m.Trails[key] = trail

	m.saveLocked()

	return trail, nil
}

// DeleteTrail deletes a trail.
func (m *MemoryStorage) DeleteTrail(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = normalizeTrailName(name)

	if _, exists := m.Trails[name]; !exists {
		return &Error{Code: errTrailNotFound, Message: "Trail not found"}
	}

	delete(m.Trails, name)

	m.saveLocked()

	return nil
}

// GetTrail gets a trail by name.
func (m *MemoryStorage) GetTrail(_ context.Context, name string) (*Trail, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name = normalizeTrailName(name)

	trail, exists := m.Trails[name]
	if !exists {
		return nil, &Error{Code: errTrailNotFound, Message: "Trail not found"}
	}

	return trail, nil
}

// DescribeTrails describes trails.
func (m *MemoryStorage) DescribeTrails(_ context.Context, names []string) ([]*Trail, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(names) == 0 {
		// Return all trails.
		result := make([]*Trail, 0, len(m.Trails))
		for _, trail := range m.Trails {
			result = append(result, trail)
		}

		return result, nil
	}

	// Return specified trails.
	result := make([]*Trail, 0, len(names))

	for _, name := range names {
		if trail, exists := m.Trails[normalizeTrailName(name)]; exists {
			result = append(result, trail)
		}
	}

	return result, nil
}

// StartLogging starts logging for a trail.
func (m *MemoryStorage) StartLogging(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = normalizeTrailName(name)

	trail, exists := m.Trails[name]
	if !exists {
		return &Error{Code: errTrailNotFound, Message: "Trail not found"}
	}

	trail.IsLogging = true

	m.saveLocked()

	return nil
}

// StopLogging stops logging for a trail.
func (m *MemoryStorage) StopLogging(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = normalizeTrailName(name)

	trail, exists := m.Trails[name]
	if !exists {
		return &Error{Code: errTrailNotFound, Message: "Trail not found"}
	}

	trail.IsLogging = false

	m.saveLocked()

	return nil
}

// LookupEvents looks up events.
// For MVP, this returns an empty list as we don't capture actual events.
func (m *MemoryStorage) LookupEvents(_ context.Context, _ *LookupEventsRequest) ([]*Event, string, error) {
	// Return empty events list for MVP.
	return []*Event{}, "", nil
}

// GetTrailStatus gets the status of a trail.
func (m *MemoryStorage) GetTrailStatus(_ context.Context, name string) (*Trail, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name = normalizeTrailName(name)

	trail, exists := m.Trails[name]
	if !exists {
		return nil, &Error{Code: errTrailNotFound, Message: "Trail not found"}
	}

	return trail, nil
}

// Helper functions.

// normalizeTrailName normalizes both a short name and an ARN to the short name.
// If the input is in the form "arn:aws:cloudtrail:<region>:<account>:trail/<name>"
// it returns the trailing <name>; otherwise it returns the input as-is.
// Real CloudTrail accepts either form for Name, so kumo treats them as aliases.
func normalizeTrailName(name string) string {
	if i := strings.LastIndex(name, ":trail/"); i >= 0 {
		return name[i+len(":trail/"):]
	}

	return name
}

func generateTrailARN(region, accountID, trailName string) string {
	return "arn:aws:cloudtrail:" + region + ":" + accountID + ":trail/" + trailName
}
