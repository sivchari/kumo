// Package ecr provides a mock implementation of AWS Elastic Container Registry.
package ecr

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

// Service is the ECR service.
type Service struct {
	storage Storage
}

// New creates a new ECR service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "ecr"
}

// TargetPrefix returns the X-Amz-Target prefix.
func (s *Service) TargetPrefix() string {
	return "AmazonEC2ContainerRegistry_V20150921"
}

// JSONProtocol marks this service as using AWS JSON protocol.
func (s *Service) JSONProtocol() {}

// RegisterRoutes registers routes for the service.
func (s *Service) RegisterRoutes(_ service.Router) {
	// ECR uses AWS JSON 1.1 protocol with X-Amz-Target header.
	// Routes are dispatched by the server based on X-Amz-Target.
}

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
		Display:     "ECR",
		Category:    "Container",
		Description: "Container registry",
	}
}
