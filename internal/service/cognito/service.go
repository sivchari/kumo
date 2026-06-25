// Package cognito provides AWS Cognito Identity Provider service emulation.
package cognito

import (
	"fmt"
	"io"
	"net/http"
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
//
// Cognito's operations use the AWS JSON protocol over POST / (dispatched by
// DispatchAction). The only REST route is the per-pool JWKS endpoint. Its path
// formally overlaps S3's wildcard /{bucket}/{key...}, but Go 1.22+ ServeMux
// prefers the more specific pattern (the literal .well-known/jwks.json suffix),
// so JWKS wins; router_test.go guards this.
func (s *Service) RegisterRoutes(r service.Router) {
	r.HandleFunc(http.MethodGet, "/{userPoolId}/.well-known/jwks.json", s.GetJWKS)
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
