package dynamodb

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

const defaultBaseURL = "http://localhost:4566"

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(defaultBaseURL, opts...)))
}

// Service implements the DynamoDB service.
type Service struct {
	storage Storage
}

// New creates a new DynamoDB service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "dynamodb"
}

// RegisterRoutes registers the DynamoDB routes.
// Note: DynamoDB uses AWS JSON 1.0 protocol via the JSONProtocolService interface,
// so no direct routes are registered here.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - DynamoDB uses JSON protocol dispatcher
}

// TargetPrefix returns the X-Amz-Target header prefix for DynamoDB.
func (s *Service) TargetPrefix() string {
	return "DynamoDB_20120810"
}

// JSONProtocol is a marker method that indicates DynamoDB uses AWS JSON 1.0 protocol.
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
		Display:     "DynamoDB",
		Category:    "Storage",
		Description: "NoSQL database",
	}
}
