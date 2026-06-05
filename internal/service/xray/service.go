package xray

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

// Service implements the X-Ray service.
type Service struct {
	storage Storage
}

// New creates a new X-Ray service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "xray"
}

// RegisterRoutes registers the X-Ray routes.
// X-Ray uses REST JSON protocol.
func (s *Service) RegisterRoutes(r service.Router) {
	r.HandleFunc("POST", "/TraceSegments", s.PutTraceSegments)
	r.HandleFunc("POST", "/TraceSummaries", s.GetTraceSummaries)
	r.HandleFunc("POST", "/Traces", s.BatchGetTraces)
	r.HandleFunc("POST", "/ServiceGraph", s.GetServiceGraph)
	r.HandleFunc("POST", "/CreateGroup", s.CreateGroup)
	r.HandleFunc("POST", "/DeleteGroup", s.DeleteGroup)
}

// Close saves the storage state if persistence is enabled.
func (s *Service) Close() error {
	if c, ok := s.storage.(io.Closer); ok {
		if err := c.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
	}

	return nil
}

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "X-Ray",
		Category:    "Monitoring & Logging",
		Description: "Distributed tracing",
	}
}
