// Package verifiedpermissions provides Amazon Verified Permissions emulation.
// It evaluates Cedar policies via github.com/cedar-policy/cedar-go and speaks
// the AWS JSON 1.0 protocol (X-Amz-Target: VerifiedPermissions.<Operation>).
package verifiedpermissions

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

// Service implements the Verified Permissions service.
type Service struct {
	storage *MemoryStorage
}

// New creates a new Verified Permissions service backed by the given storage.
func New(storage *MemoryStorage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "verifiedpermissions"
}

// RegisterRoutes registers no routes; Verified Permissions uses the JSON
// protocol dispatcher.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - Verified Permissions uses the JSON protocol dispatcher.
}

// TargetPrefix returns the X-Amz-Target prefix for Verified Permissions.
func (s *Service) TargetPrefix() string {
	return "VerifiedPermissions"
}

// JSONProtocol marks the service as using the AWS JSON protocol.
func (s *Service) JSONProtocol() {}

// Close persists the storage state if persistence is enabled.
func (s *Service) Close() error {
	if err := s.storage.Close(); err != nil {
		return fmt.Errorf("failed to close storage: %w", err)
	}

	return nil
}

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "Verified Permissions",
		Category:    "Security & Identity",
		Description: "Fine-grained authorization (Cedar)",
	}
}
