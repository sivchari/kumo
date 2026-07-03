// Package forecast provides Amazon Forecast service emulation.
package forecast

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Service implements the Forecast service.
type Service struct {
	storage Storage
}

// New creates a new Forecast service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "forecast"
}

// TargetPrefix returns the target prefix for AWS JSON protocol.
func (s *Service) TargetPrefix() string {
	return "AmazonForecast"
}

// JSONProtocol indicates this service uses AWS JSON protocol.
func (s *Service) JSONProtocol() {}

// RegisterRoutes registers HTTP routes for the service.
func (s *Service) RegisterRoutes(_ service.Router) {}

// Close saves the storage state if persistence is enabled.
func (s *Service) Close() error {
	if c, ok := s.storage.(io.Closer); ok {
		if err := c.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
	}

	return nil
}

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

var (
	_ service.Service             = (*Service)(nil)
	_ service.JSONProtocolService = (*Service)(nil)
	_ io.Closer                   = (*Service)(nil)
)

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "Forecast",
		Category:    "Analytics & ML",
		Description: "Time-series forecasting",
	}
}
