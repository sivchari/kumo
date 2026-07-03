package route53resolver

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check to ensure Service implements JSONProtocolService.
var _ service.JSONProtocolService = (*Service)(nil)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Service implements the Route 53 Resolver service.
type Service struct {
	storage Storage
}

// New creates a new Route 53 Resolver service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "route53resolver"
}

// RegisterRoutes registers the Route 53 Resolver routes.
// Route 53 Resolver uses AWS JSON 1.1 protocol via the JSONProtocolService interface.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - Route 53 Resolver uses JSON protocol dispatcher
}

// TargetPrefix returns the X-Amz-Target header prefix for Route 53 Resolver.
func (s *Service) TargetPrefix() string {
	return "Route53Resolver"
}

// JSONProtocol is a marker method that indicates Route 53 Resolver uses AWS JSON 1.1 protocol.
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
		Display:     "Route 53 Resolver",
		Category:    "Networking & Content Delivery",
		Description: "DNS resolver",
	}
}
