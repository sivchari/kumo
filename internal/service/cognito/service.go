// Package cognito provides AWS Cognito Identity Provider service emulation.
package cognito

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Service implements the Cognito Identity Provider service.
type Service struct {
	storage Storage
}

// New creates a new Cognito service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "cognito-idp"
}

// TargetPrefix returns the AWS JSON target prefix.
func (s *Service) TargetPrefix() string {
	return "AWSCognitoIdentityProviderService"
}

// JSONProtocol marks this service as using AWS JSON 1.1 protocol.
func (s *Service) JSONProtocol() {}

// RegisterRoutes registers routes for REST-based operations.
func (s *Service) RegisterRoutes(_ service.Router) {
	// Cognito uses AWS JSON protocol with X-Amz-Target header.
	// Routes are handled by DispatchAction.
}

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

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
		Display:     "Cognito",
		Category:    "Security & Identity",
		Description: "User authentication",
	}
}
