// Package gamelift provides a mock implementation of AWS GameLift.
package gamelift

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Service is the GameLift service.
type Service struct {
	storage Storage
}

// New creates a new GameLift service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "gamelift"
}

// TargetPrefix returns the X-Amz-Target prefix.
func (s *Service) TargetPrefix() string {
	return "GameLift"
}

// JSONProtocol marks this service as using AWS JSON protocol.
func (s *Service) JSONProtocol() {}

// RegisterRoutes registers routes for the service.
func (s *Service) RegisterRoutes(_ service.Router) {
	// GameLift uses AWS JSON 1.0 protocol with X-Amz-Target header.
	// Routes are dispatched by the server based on X-Amz-Target.
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

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Ensure Service implements required interfaces.
var (
	_ service.Service             = (*Service)(nil)
	_ service.JSONProtocolService = (*Service)(nil)
	_ io.Closer                   = (*Service)(nil)
)

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "GameLift",
		Category:    "Other Services",
		Description: "Game server hosting",
	}
}
