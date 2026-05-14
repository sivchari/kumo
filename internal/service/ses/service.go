package ses

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

// Service implements the SES v1 service.
type Service struct {
	storage Storage
}

// New creates a new SES v1 service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "ses"
}

// RegisterRoutes registers the SES v1 routes.
// SES v1 uses Query protocol, so most routes are handled via DispatchAction.
// The mailbox endpoint is registered here as a kumo-specific REST endpoint.
func (s *Service) RegisterRoutes(r service.Router) {
	// kumo-specific endpoint for local mailbox testing.
	r.HandleFunc("GET", "/_aws/ses", s.GetMailbox)
}

// TargetPrefix returns the target prefix for SES v1.
func (s *Service) TargetPrefix() string {
	return "SimpleEmailService"
}

// Actions returns the list of action names this service handles.
func (s *Service) Actions() []string {
	return []string{
		"VerifyEmailIdentity",
		"SendEmail",
		"SendRawEmail",
		"ListIdentities",
		"DeleteIdentity",
		"GetIdentityVerificationAttributes",
	}
}

// ServiceIdentifier returns the SDK service identifier for User-Agent disambiguation.
func (s *Service) ServiceIdentifier() string {
	return "ses"
}

// QueryProtocol is a marker method that indicates SES v1 uses AWS Query protocol.
func (s *Service) QueryProtocol() {}

// Close saves the storage state if persistence is enabled.
func (s *Service) Close() error {
	if c, ok := s.storage.(io.Closer); ok {
		if err := c.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
	}

	return nil
}
