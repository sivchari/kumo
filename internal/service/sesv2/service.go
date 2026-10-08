package sesv2

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

// Service implements the SES v2 service.
type Service struct {
	storage Storage
}

// New creates a new SES v2 service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "sesv2"
}

// SigningName returns the SigV4 credential scope service name for SES.
func (s *Service) SigningName() string {
	return "ses"
}

// RegisterRoutes registers the AWS-faithful SES v2 routes and the
// kumo-specific test endpoint.
func (s *Service) RegisterRoutes(r service.Router) {
	r.HandleFunc("GET", "/kumo/ses/v2/sent-emails", s.GetSentEmails)

	// Email Identity routes.
	r.HandleFunc("POST", "/v2/email/identities", s.CreateEmailIdentity)
	r.HandleFunc("GET", "/v2/email/identities", s.ListEmailIdentities)
	r.HandleFunc("GET", "/v2/email/identities/{emailIdentity}", s.GetEmailIdentity)
	r.HandleFunc("DELETE", "/v2/email/identities/{emailIdentity}", s.DeleteEmailIdentity)

	// Configuration Set routes.
	r.HandleFunc("POST", "/v2/email/configuration-sets", s.CreateConfigurationSet)
	r.HandleFunc("GET", "/v2/email/configuration-sets", s.ListConfigurationSets)
	r.HandleFunc("GET", "/v2/email/configuration-sets/{configurationSetName}", s.GetConfigurationSet)
	r.HandleFunc("DELETE", "/v2/email/configuration-sets/{configurationSetName}", s.DeleteConfigurationSet)

	// Email Template routes.
	r.HandleFunc("POST", "/v2/email/templates", s.CreateEmailTemplate)
	r.HandleFunc("GET", "/v2/email/templates", s.ListEmailTemplates)
	r.HandleFunc("GET", "/v2/email/templates/{templateName}", s.GetEmailTemplate)
	r.HandleFunc("PUT", "/v2/email/templates/{templateName}", s.UpdateEmailTemplate)
	r.HandleFunc("DELETE", "/v2/email/templates/{templateName}", s.DeleteEmailTemplate)

	// Send Email routes.
	r.HandleFunc("POST", "/v2/email/outbound-emails", s.SendEmail)
	r.HandleFunc("POST", "/v2/email/outbound-bulk-emails", s.SendBulkEmail)
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
		Display:     "SES v2",
		Category:    "Application Integration",
		Description: "Email service (v2 API)",
	}
}
