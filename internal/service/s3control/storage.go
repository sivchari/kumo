package s3control

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/sivchari/kumo/internal/storage"
)

// Default values.
const defaultRegion = "us-east-1"

// Storage is the interface for S3 Control storage operations.
type Storage interface {
	// Public Access Block operations
	GetPublicAccessBlock(ctx context.Context, accountID string) (*PublicAccessBlockConfiguration, error)
	PutPublicAccessBlock(ctx context.Context, accountID string, config *PublicAccessBlockConfiguration) error
	DeletePublicAccessBlock(ctx context.Context, accountID string) error

	// Access Point operations
	CreateAccessPoint(ctx context.Context, accountID string, ap *AccessPoint) (*AccessPoint, error)
	GetAccessPoint(ctx context.Context, accountID, name string) (*AccessPoint, error)
	DeleteAccessPoint(ctx context.Context, accountID, name string) error
	ListAccessPoints(ctx context.Context, accountID, bucket string, maxResults int, nextToken string) ([]*AccessPoint, string, error)
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

// MemoryStorage implements in-memory storage for S3 Control.
type MemoryStorage struct {
	mu                 sync.RWMutex                               `json:"-"`
	PublicAccessBlocks map[string]*PublicAccessBlockConfiguration `json:"publicAccessBlocks"` // key: accountID
	AccessPoints       map[string]map[string]*AccessPoint         `json:"accessPoints"`       // key: accountID -> name -> AccessPoint
	region             string
	dataDir            string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		PublicAccessBlocks: make(map[string]*PublicAccessBlockConfiguration),
		AccessPoints:       make(map[string]map[string]*AccessPoint),
		region:             region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "s3control", s)
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

	if s.PublicAccessBlocks == nil {
		s.PublicAccessBlocks = make(map[string]*PublicAccessBlockConfiguration)
	}

	if s.AccessPoints == nil {
		s.AccessPoints = make(map[string]map[string]*AccessPoint)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(s)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(s.dataDir, "s3control", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "s3control", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// GetPublicAccessBlock retrieves the public access block configuration for an account.
func (s *MemoryStorage) GetPublicAccessBlock(_ context.Context, accountID string) (*PublicAccessBlockConfiguration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.PublicAccessBlocks[accountID]
	if !exists {
		return nil, &Error{
			Code:    ErrNoSuchPublicAccessBlockConfiguration,
			Message: "The public access block configuration was not found",
		}
	}

	return config, nil
}

// PutPublicAccessBlock sets the public access block configuration for an account.
func (s *MemoryStorage) PutPublicAccessBlock(_ context.Context, accountID string, config *PublicAccessBlockConfiguration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.PublicAccessBlocks[accountID] = config

	s.saveLocked()

	return nil
}

// DeletePublicAccessBlock removes the public access block configuration for an account.
func (s *MemoryStorage) DeletePublicAccessBlock(_ context.Context, accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.PublicAccessBlocks, accountID)

	s.saveLocked()

	return nil
}

// CreateAccessPoint creates a new access point.
func (s *MemoryStorage) CreateAccessPoint(_ context.Context, accountID string, ap *AccessPoint) (*AccessPoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.AccessPoints[accountID]; !exists {
		s.AccessPoints[accountID] = make(map[string]*AccessPoint)
	}

	if _, exists := s.AccessPoints[accountID][ap.Name]; exists {
		return nil, &Error{
			Code:    ErrAccessPointAlreadyOwnedByYou,
			Message: fmt.Sprintf("Access point %s already exists", ap.Name),
		}
	}

	// Generate ARN and alias
	ap.AccountID = accountID
	ap.AccessPointArn = fmt.Sprintf("arn:aws:s3:%s:%s:accesspoint/%s", s.region, accountID, ap.Name)
	ap.Alias = fmt.Sprintf("%s-%s-s3alias", ap.Name, accountID[:12])

	if ap.VpcConfiguration != nil {
		ap.NetworkOrigin = "VPC"
	} else {
		ap.NetworkOrigin = "Internet"
	}

	ap.Endpoints = map[string]string{
		"https": fmt.Sprintf("https://%s-%s.s3-accesspoint.%s.amazonaws.com", ap.Name, accountID, s.region),
	}

	s.AccessPoints[accountID][ap.Name] = ap

	s.saveLocked()

	return ap, nil
}

// GetAccessPoint retrieves an access point.
func (s *MemoryStorage) GetAccessPoint(_ context.Context, accountID, name string) (*AccessPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accountAPs, exists := s.AccessPoints[accountID]
	if !exists {
		return nil, &Error{
			Code:    ErrNoSuchAccessPoint,
			Message: fmt.Sprintf("Access point %s does not exist", name),
		}
	}

	ap, exists := accountAPs[name]
	if !exists {
		return nil, &Error{
			Code:    ErrNoSuchAccessPoint,
			Message: fmt.Sprintf("Access point %s does not exist", name),
		}
	}

	return ap, nil
}

// DeleteAccessPoint deletes an access point.
func (s *MemoryStorage) DeleteAccessPoint(_ context.Context, accountID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	accountAPs, exists := s.AccessPoints[accountID]
	if !exists {
		return &Error{
			Code:    ErrNoSuchAccessPoint,
			Message: fmt.Sprintf("Access point %s does not exist", name),
		}
	}

	if _, exists := accountAPs[name]; !exists {
		return &Error{
			Code:    ErrNoSuchAccessPoint,
			Message: fmt.Sprintf("Access point %s does not exist", name),
		}
	}

	delete(accountAPs, name)

	s.saveLocked()

	return nil
}

// ListAccessPoints lists access points for an account.
func (s *MemoryStorage) ListAccessPoints(_ context.Context, accountID, bucket string, maxResults int, nextToken string) ([]*AccessPoint, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults == 0 {
		maxResults = 1000
	}

	accountAPs, exists := s.AccessPoints[accountID]
	if !exists {
		return []*AccessPoint{}, "", nil
	}

	var accessPoints []*AccessPoint

	for _, ap := range accountAPs {
		if bucket != "" && ap.Bucket != bucket {
			continue
		}

		accessPoints = append(accessPoints, ap)
	}

	// Simple pagination (no sorting for simplicity)
	start := 0

	if nextToken != "" {
		for i, ap := range accessPoints {
			if ap.Name == nextToken {
				start = i

				break
			}
		}
	}

	end := min(start+maxResults, len(accessPoints))

	result := accessPoints[start:end]
	newNextToken := ""

	if end < len(accessPoints) {
		newNextToken = accessPoints[end].Name
	}

	return result, newNextToken, nil
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}
