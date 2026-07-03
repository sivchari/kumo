package ses

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Storage defines the interface for SES v1 storage operations.
type Storage interface {
	VerifyEmailIdentity(ctx context.Context, email string) error
	ListIdentities(ctx context.Context) ([]string, error)
	DeleteIdentity(ctx context.Context, identity string) error
	GetIdentityVerificationAttributes(ctx context.Context, identities []string) (map[string]string, error)
	SendEmail(ctx context.Context, email *SentEmail) (string, error)
	GetMailbox(ctx context.Context, email string) ([]*SentEmail, error)
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
	mu         sync.RWMutex         `json:"-"`
	Identities map[string]*Identity `json:"identities"`
	Emails     []*SentEmail         `json:"emails"`
	dataDir    string
}

// NewMemoryStorage creates a new in-memory SES storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Identities: make(map[string]*Identity),
		Emails:     make([]*SentEmail, 0),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "ses", s)
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

	if m.Identities == nil {
		m.Identities = make(map[string]*Identity)
	}

	if m.Emails == nil {
		m.Emails = make([]*SentEmail, 0)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "ses", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "ses", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// VerifyEmailIdentity stores an email address as verified.
func (m *MemoryStorage) VerifyEmailIdentity(_ context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Identities[email] = &Identity{
		Email:      email,
		VerifiedAt: time.Now(),
	}

	m.saveLocked()

	return nil
}

// ListIdentities returns all verified identities sorted alphabetically.
func (m *MemoryStorage) ListIdentities(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	identities := make([]string, 0, len(m.Identities))
	for email := range m.Identities {
		identities = append(identities, email)
	}

	sort.Strings(identities)

	return identities, nil
}

// DeleteIdentity removes a verified identity.
func (m *MemoryStorage) DeleteIdentity(_ context.Context, identity string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.Identities, identity)

	m.saveLocked()

	return nil
}

// GetIdentityVerificationAttributes returns verification status for the given identities.
func (m *MemoryStorage) GetIdentityVerificationAttributes(_ context.Context, identities []string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(identities))

	for _, identity := range identities {
		if _, exists := m.Identities[identity]; exists {
			result[identity] = "Success"
		}
	}

	return result, nil
}

// SendEmail stores an email and returns the message ID.
func (m *MemoryStorage) SendEmail(_ context.Context, email *SentEmail) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	email.MessageID = uuid.New().String()
	email.SentAt = time.Now()
	m.Emails = append(m.Emails, email)

	m.saveLocked()

	return email.MessageID, nil
}

// GetMailbox returns all sent emails for the given sender email address.
func (m *MemoryStorage) GetMailbox(_ context.Context, email string) ([]*SentEmail, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*SentEmail

	for _, e := range m.Emails {
		if e.Source == email {
			result = append(result, e)
		}
	}

	return result, nil
}
