// Package otlp provides an OTLP/HTTP ingest endpoint for kumo.
//
// The goal is operational testing — "did my application send signals
// in the shape it would in production?" — not a production-grade OTel
// backend. So we accept OTLP/JSON, store the raw payloads in memory,
// and expose them for tests to assert on.
//
// OTLP/protobuf is intentionally out of scope for the first iteration:
// every OTel SDK supports OTEL_EXPORTER_OTLP_PROTOCOL=http/json, and
// JSON keeps kumo dependency-free (no protoc, no opentelemetry-proto
// vendoring).
package otlp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/storage"
)

// Signal identifies which OTLP signal kind a payload carries.
type Signal string

// The three OTLP signal kinds. Match the OTel HTTP path suffixes
// (/v1/traces, /v1/metrics, /v1/logs).
const (
	SignalTraces  Signal = "traces"
	SignalMetrics Signal = "metrics"
	SignalLogs    Signal = "logs"
)

// Payload is one received OTLP/JSON request body kept verbatim.
// We don't re-parse OTLP into a typed model — tests should reach in
// with `jq` or a per-test parser. That keeps the storage independent
// of OTel proto schema drift.
type Payload struct {
	Signal      Signal    `json:"signal"`
	ReceivedAt  time.Time `json:"receivedAt"`
	ContentType string    `json:"contentType"`
	Body        string    `json:"body"` // raw JSON document as received
}

// Storage is the interface the handlers talk to.
type Storage interface {
	Append(ctx context.Context, p Payload) error
	List(ctx context.Context, signal Signal) []Payload
	Reset(ctx context.Context)
}

// Option configures MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the given directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// MemoryStorage holds OTLP payloads in memory, optionally persisted.
type MemoryStorage struct {
	mu       sync.RWMutex `json:"-"`
	Payloads []Payload    `json:"payloads"`
	dataDir  string
}

// NewMemoryStorage creates a fresh in-memory store.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{Payloads: make([]Payload, 0)}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "otlp", s)
	}

	return s
}

// MarshalJSON / UnmarshalJSON are needed for the storage round-trip.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MarshalJSON serialises the payload list under lock.
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

// UnmarshalJSON restores the payload list under lock.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}
	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Payloads == nil {
		s.Payloads = make([]Payload, 0)
	}

	return nil
}

// Close persists state to disk if a data dir was configured.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "otlp", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// Append records one received payload.
func (s *MemoryStorage) Append(_ context.Context, p Payload) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Payloads = append(s.Payloads, p)

	return nil
}

// List returns the payloads received for the given signal kind, in
// arrival order. Returns a copy so callers can iterate safely.
func (s *MemoryStorage) List(_ context.Context, signal Signal) []Payload {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Payload, 0, len(s.Payloads))

	for _, p := range s.Payloads {
		if p.Signal == signal {
			out = append(out, p)
		}
	}

	return out
}

// Reset clears all stored payloads. Used by tests between runs.
func (s *MemoryStorage) Reset(_ context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Payloads = s.Payloads[:0]
}
