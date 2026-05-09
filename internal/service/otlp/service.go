package otlp

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Service is the OTLP/HTTP ingest endpoint.
type Service struct {
	storage Storage
}

// New constructs a Service backed by the given storage.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service identifier used by kumo's logger.
func (s *Service) Name() string { return "otlp" }

// RegisterRoutes wires the three OTLP/HTTP signal endpoints plus a
// kumo-private readback under /kumo/otlp.
func (s *Service) RegisterRoutes(r service.Router) {
	// OTLP/HTTP standard ingest paths.
	r.HandleFunc("POST", "/v1/traces", s.PutTraces)
	r.HandleFunc("POST", "/v1/metrics", s.PutMetrics)
	r.HandleFunc("POST", "/v1/logs", s.PutLogs)

	// Test-facing readback. Lives under /_kumo/ because real AWS /
	// real OTel backends don't have this — it's specifically for
	// integration tests asserting "my app emitted the signals I
	// expected".
	r.HandleFunc("GET", "/kumo/otlp/{signal}", s.ListReceived)
	r.HandleFunc("DELETE", "/kumo/otlp", s.ResetReceived)
}

// Close persists the storage if configured.
func (s *Service) Close() error {
	if c, ok := s.storage.(io.Closer); ok {
		if err := c.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
	}

	return nil
}
