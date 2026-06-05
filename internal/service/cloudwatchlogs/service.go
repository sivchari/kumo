package cloudwatchlogs

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

const defaultBaseURL = "http://localhost:4566"

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(defaultBaseURL, opts...), defaultBaseURL))
}

// Service implements the CloudWatch Logs service.
type Service struct {
	storage Storage
	baseURL string
}

// New creates a new CloudWatch Logs service.
func New(storage Storage, baseURL string) *Service {
	return &Service{
		storage: storage,
		baseURL: baseURL,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "logs"
}

// RegisterRoutes registers the CloudWatch Logs routes.
// Note: CloudWatch Logs uses AWS JSON 1.1 protocol via the JSONProtocolService interface,
// so no direct routes are registered here.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - CloudWatch Logs uses JSON protocol dispatcher
}

// TargetPrefix returns the X-Amz-Target header prefix for CloudWatch Logs.
func (s *Service) TargetPrefix() string {
	return "Logs_20140328"
}

// JSONProtocol is a marker method that indicates CloudWatch Logs uses AWS JSON 1.1 protocol.
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
		Display:     "CloudWatch Logs",
		Category:    "Monitoring & Logging",
		Description: "Log management",
	}
}
