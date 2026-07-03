// Package organizations provides AWS Organizations service emulation.
package organizations

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Service implements the Organizations service.
type Service struct {
	storage Storage
}

// New creates a new Organizations service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "organizations"
}

// TargetPrefix returns the target prefix for AWS JSON protocol.
func (s *Service) TargetPrefix() string {
	return "AWSOrganizationsV20161128"
}

// JSONProtocol indicates this service uses AWS JSON protocol.
func (s *Service) JSONProtocol() {}

// RegisterRoutes registers HTTP routes for the service.
func (s *Service) RegisterRoutes(_ service.Router) {}

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

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
		Display:     "Organizations",
		Category:    "Management & Configuration",
		Description: "Multi-account management",
	}
}
