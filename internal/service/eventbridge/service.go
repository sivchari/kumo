// Package eventbridge provides AWS EventBridge service emulation.
package eventbridge

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

const defaultBaseURL = "http://localhost:4566"

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

// Service implements the EventBridge service.
type Service struct {
	storage Storage
}

// New creates a new EventBridge service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "events"
}

// Storage returns the underlying storage. Used by cross-service wiring
// (wirePipesToEventBridge) to publish events in-process.
func (s *Service) Storage() Storage {
	return s.storage
}

// TargetPrefix returns the X-Amz-Target prefix.
func (s *Service) TargetPrefix() string {
	return "AWSEvents"
}

// JSONProtocol marks this service as using JSON protocol.
func (s *Service) JSONProtocol() {}

// RegisterRoutes registers the service routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// kumo-specific endpoint for testing.
	r.HandleFunc("GET", "/kumo/eventbridge/delivered-events", s.GetDeliveredEvents)
}

func init() {
	baseURL := defaultBaseURL

	if host := os.Getenv("KUMO_HOST"); host != "" {
		port := os.Getenv("KUMO_PORT")
		if port == "" {
			port = "4566"
		}

		baseURL = fmt.Sprintf("http://%s:%s", host, port)
	} else if port := os.Getenv("KUMO_PORT"); port != "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}

	var opts []Option

	opts = append(opts, WithBaseURL(baseURL))

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
		Display:     "EventBridge",
		Category:    "Messaging & Integration",
		Description: "Event bus",
	}
}
