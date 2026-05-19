package pinpointsmsvoicev2

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errInvalidParameter = "ValidationException"
)

// Storage defines the interface for Pinpoint SMS Voice v2 storage operations.
type Storage interface {
	SendTextMessage(ctx context.Context, req *SendTextMessageInput) (string, error)
	GetSentTextMessages(ctx context.Context) ([]*SentTextMessage, error)
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

// MemoryStorage implements Storage with in-memory data structures.
type MemoryStorage struct {
	mu               sync.RWMutex       `json:"-"`
	SentTextMessages []*SentTextMessage `json:"sentTextMessages"`
	dataDir          string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		SentTextMessages: make([]*SentTextMessage, 0),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "pinpointsmsvoicev2", s)
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

	if s.SentTextMessages == nil {
		s.SentTextMessages = make([]*SentTextMessage, 0)
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

	_ = storage.SaveBytes(s.dataDir, "pinpointsmsvoicev2", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "pinpointsmsvoicev2", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// SendTextMessage sends a text message (stores it for testing).
func (s *MemoryStorage) SendTextMessage(_ context.Context, req *SendTextMessageInput) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.DestinationPhoneNumber == "" {
		return "", &Error{
			Code:    errInvalidParameter,
			Message: "DestinationPhoneNumber is required",
		}
	}

	messageID := uuid.New().String()

	msg := &SentTextMessage{
		MessageID:              messageID,
		DestinationPhoneNumber: req.DestinationPhoneNumber,
		OriginationIdentity:    req.OriginationIdentity,
		MessageBody:            req.MessageBody,
		MessageType:            req.MessageType,
		ConfigurationSetName:   req.ConfigurationSetName,
		SentAt:                 time.Now(),
	}

	s.SentTextMessages = append(s.SentTextMessages, msg)

	s.saveLocked()

	return messageID, nil
}

// GetSentTextMessages returns all sent text messages.
func (s *MemoryStorage) GetSentTextMessages(_ context.Context) ([]*SentTextMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.SentTextMessages, nil
}
