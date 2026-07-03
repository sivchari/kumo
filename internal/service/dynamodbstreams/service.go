package dynamodbstreams

import (
	"fmt"
	"io"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/streams"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	service.Register(New(NewMemoryStorage(streams.Global)))
}

// Service implements the DynamoDB Streams service.
type Service struct {
	storage Storage
}

// New creates a new DynamoDB Streams service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "dynamodbstreams"
}

// RegisterRoutes registers routes for the service.
// DynamoDB Streams uses AWS JSON 1.0 protocol, so no direct routes are registered.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - uses JSON protocol dispatcher.
}

// TargetPrefix returns the X-Amz-Target header prefix for DynamoDB Streams.
func (s *Service) TargetPrefix() string {
	return "DynamoDBStreams_20120810"
}

// JSONProtocol is a marker method that indicates DynamoDB Streams uses AWS JSON 1.0 protocol.
func (s *Service) JSONProtocol() {}

// Close releases resources held by the service.
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
		Display:     "DynamoDB Streams",
		Category:    "Storage",
		Description: "DynamoDB change data capture",
	}
}
