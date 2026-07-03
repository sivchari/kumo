// Package sagemaker provides SageMaker service emulation for kumo.
package sagemaker

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

// Service implements the SageMaker service.
type Service struct {
	storage Storage
}

// New creates a new SageMaker service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "sagemaker"
}

// RegisterRoutes registers the SageMaker routes.
// Note: SageMaker uses AWS JSON 1.1 protocol via the JSONProtocolService interface,
// so no direct routes are registered here.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - SageMaker uses JSON protocol dispatcher
}

// TargetPrefix returns the X-Amz-Target header prefix for SageMaker.
func (s *Service) TargetPrefix() string {
	return "SageMaker"
}

// JSONProtocol is a marker method that indicates SageMaker uses AWS JSON 1.1 protocol.
func (s *Service) JSONProtocol() {}

// Ensure Service implements service.Service and service.JSONProtocolService.
var (
	_ service.Service             = (*Service)(nil)
	_ service.JSONProtocolService = (*Service)(nil)
)

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
		Display:     "SageMaker",
		Category:    "Analytics & ML",
		Description: "Machine learning",
	}
}
