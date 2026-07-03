package rekognition

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check to ensure Service implements service.Service.
var _ service.Service = (*Service)(nil)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Service implements the AWS Rekognition service.
type Service struct {
	storage Storage
}

// New creates a new Rekognition service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "rekognition"
}

// TargetPrefix returns the X-Amz-Target prefix for JSON protocol dispatch.
func (s *Service) TargetPrefix() string {
	return "RekognitionService"
}

// JSONProtocol marks this service as using JSON protocol.
func (s *Service) JSONProtocol() {}

// RegisterRoutes registers the routes for this service.
func (s *Service) RegisterRoutes(_ service.Router) {
	// JSON protocol services use DispatchAction for routing
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
		Display:     "Rekognition",
		Category:    "Analytics & ML",
		Description: "Image/video analysis",
	}
}
