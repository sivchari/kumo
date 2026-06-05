package codeconnections

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Service implements the AWS CodeConnections service.
type Service struct {
	storage Storage
}

// New creates a new CodeConnections service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "codeconnections"
}

// TargetPrefix returns the X-Amz-Target prefix for JSON protocol dispatch.
func (s *Service) TargetPrefix() string {
	return "CodeConnections_20231201"
}

// JSONProtocol marks this service as using JSON protocol.
func (s *Service) JSONProtocol() {}

// RegisterRoutes registers the routes for this service.
func (s *Service) RegisterRoutes(_ service.Router) {
	// JSON protocol services use DispatchAction for routing
}

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
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
		Display:     "CodeConnections",
		Category:    "Management & Configuration",
		Description: "Source code connections",
	}
}
