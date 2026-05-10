package sqs

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

// Service implements the SQS service.
type Service struct {
	storage Storage
	baseURL string
}

// New creates a new SQS service.
func New(storage Storage, baseURL string) *Service {
	return &Service{
		storage: storage,
		baseURL: baseURL,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "sqs"
}

// RegisterRoutes registers the SQS routes.
// Note: SQS uses AWS JSON 1.0 protocol via the JSONProtocolService interface,
// so no direct routes are registered here.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - SQS uses JSON protocol dispatcher
}

// TargetPrefix returns the X-Amz-Target header prefix for SQS.
func (s *Service) TargetPrefix() string {
	return "AmazonSQS"
}

// JSONProtocol is a marker method that indicates SQS uses AWS JSON 1.0 protocol.
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

// Storage returns the SQS storage so other services (e.g. SNS) can hand
// messages directly to a queue without going through the public SDK
// surface.
func (s *Service) Storage() Storage {
	return s.storage
}

// BaseURL returns the configured base URL used when generating queue URLs.
// SNS uses this to translate an SQS ARN (which is what subscribers send)
// into the queue URL the storage layer keys queues by.
func (s *Service) BaseURL() string {
	return s.baseURL
}
