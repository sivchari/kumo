// Package amp provides emulation for Amazon Managed Service for
// Prometheus. The AWS SDK package and CLI command are named amp; the
// AWS endpoint and ARN namespace still use aps.
//
// kumo handles the *control plane* (CreateWorkspace etc) entirely in
// memory. The *data plane* (remote_write ingest, PromQL query) is
// proxied to a real local Prometheus configured via env var. That
// matches the "kumo can't reasonably emulate this protocol, but you
// already have a real one running locally" pattern from #579 (RDS
// endpoint passthrough).
package amp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const defaultRegion = "us-east-1"

// Storage is the in-memory contract.
type Storage interface {
	CreateWorkspace(ctx context.Context, alias string, tags map[string]string) (*Workspace, error)
	DescribeWorkspace(ctx context.Context, id string) (*Workspace, error)
	ListWorkspaces(ctx context.Context, alias string) ([]Workspace, error)
	DeleteWorkspace(ctx context.Context, id string) error
}

// Option configures MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// MemoryStorage is the in-memory implementation.
type MemoryStorage struct {
	mu         sync.RWMutex          `json:"-"`
	Workspaces map[string]*Workspace `json:"workspaces"`
	dataDir    string
}

// NewMemoryStorage creates an empty in-memory store.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{Workspaces: make(map[string]*Workspace)}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "amp", s)
	}

	return s
}

var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MarshalJSON serialises under lock.
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

// UnmarshalJSON restores under lock.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}
	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Workspaces == nil {
		s.Workspaces = make(map[string]*Workspace)
	}

	return nil
}

// Close persists state to disk.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "amp", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateWorkspace allocates a new workspace ID and stores it.
// PrometheusEndpoint is the AWS-shaped URL the SDK callers expect to
// see — the actual data-plane proxying happens at the kumo HTTP
// router layer based on the workspace ID in the URL.
func (s *MemoryStorage) CreateWorkspace(_ context.Context, alias string, tags map[string]string) (*Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := "ws-" + uuid.New().String()
	ws := &Workspace{
		WorkspaceID:        id,
		Alias:              alias,
		Arn:                fmt.Sprintf("arn:aws:aps:%s:000000000000:workspace/%s", defaultRegion, id),
		Status:             Status{StatusCode: "ACTIVE"},
		PrometheusEndpoint: fmt.Sprintf("https://aps-workspaces.%s.amazonaws.com/workspaces/%s/", defaultRegion, id),
		CreatedAt:          time.Now().UTC(),
		Tags:               tags,
	}

	s.Workspaces[id] = ws

	return ws, nil
}

// DescribeWorkspace returns one workspace or NotFound.
func (s *MemoryStorage) DescribeWorkspace(_ context.Context, id string) (*Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ws, ok := s.Workspaces[id]
	if !ok {
		return nil, &Error{Code: "ResourceNotFoundException", Message: fmt.Sprintf("workspace %s not found", id)}
	}

	return ws, nil
}

// ListWorkspaces lists all workspaces, optionally filtered by alias.
func (s *MemoryStorage) ListWorkspaces(_ context.Context, alias string) ([]Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Workspace, 0, len(s.Workspaces))

	for _, ws := range s.Workspaces {
		if alias != "" && ws.Alias != alias {
			continue
		}

		out = append(out, *ws)
	}

	return out, nil
}

// DeleteWorkspace removes a workspace.
func (s *MemoryStorage) DeleteWorkspace(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Workspaces[id]; !ok {
		return &Error{Code: "ResourceNotFoundException", Message: fmt.Sprintf("workspace %s not found", id)}
	}

	delete(s.Workspaces, id)

	return nil
}
