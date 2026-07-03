// Package athena provides Athena service emulation for kumo.
package athena

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

// Service implements the Athena service.
type Service struct {
	storage Storage
}

// New creates a new Athena service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "athena"
}

// RegisterRoutes registers the Athena routes.
// Note: Athena uses AWS JSON 1.1 protocol via the JSONProtocolService interface,
// so no direct routes are registered here.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - Athena uses JSON protocol dispatcher
}

// TargetPrefix returns the X-Amz-Target header prefix for Athena.
func (s *Service) TargetPrefix() string {
	return "AmazonAthena"
}

// JSONProtocol is a marker method that indicates Athena uses AWS JSON 1.1 protocol.
func (s *Service) JSONProtocol() {}

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
		Display:     "Athena",
		Category:    "Analytics & ML",
		Description: "SQL query service",
	}
}
