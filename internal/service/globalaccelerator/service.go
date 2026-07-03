// Package globalaccelerator provides AWS Global Accelerator service emulation.
package globalaccelerator

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

// Service implements the Global Accelerator service.
type Service struct {
	storage Storage
}

// New creates a new Global Accelerator service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "globalaccelerator"
}

// TargetPrefix returns the AWS JSON target prefix.
func (s *Service) TargetPrefix() string {
	return "GlobalAccelerator_V20180706"
}

// JSONProtocol marks this service as using AWS JSON 1.1 protocol.
func (s *Service) JSONProtocol() {}

// RegisterRoutes registers routes for REST-based operations.
func (s *Service) RegisterRoutes(_ service.Router) {
	// Global Accelerator uses AWS JSON protocol with X-Amz-Target header.
	// Routes are handled by DispatchAction.
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
		Display:     "Global Accelerator",
		Category:    "Networking & Content Delivery",
		Description: "Network acceleration",
	}
}
