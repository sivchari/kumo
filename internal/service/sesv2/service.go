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

// RegisterRoutes registers the SES v2 routes.
// SES v2 uses REST API with path-based routing.
func (s *Service) RegisterRoutes(r service.Router) {
	// Email Identity routes.
	r.HandleFunc("POST", "/ses/v2/email/identities", s.CreateEmailIdentity)
	r.HandleFunc("GET", "/ses/v2/email/identities", s.ListEmailIdentities)
	r.HandleFunc("GET", "/ses/v2/email/identities/{emailIdentity}", s.GetEmailIdentity)
	r.HandleFunc("DELETE", "/ses/v2/email/identities/{emailIdentity}", s.DeleteEmailIdentity)

	// Configuration Set routes.
	r.HandleFunc("POST", "/ses/v2/email/configuration-sets", s.CreateConfigurationSet)
	r.HandleFunc("GET", "/ses/v2/email/configuration-sets", s.ListConfigurationSets)
	r.HandleFunc("GET", "/ses/v2/email/configuration-sets/{configurationSetName}", s.GetConfigurationSet)
	r.HandleFunc("DELETE", "/ses/v2/email/configuration-sets/{configurationSetName}", s.DeleteConfigurationSet)

	// Email Template routes.
	r.HandleFunc("POST", "/ses/v2/email/templates", s.CreateEmailTemplate)
	r.HandleFunc("GET", "/ses/v2/email/templates", s.ListEmailTemplates)
	r.HandleFunc("GET", "/ses/v2/email/templates/{templateName}", s.GetEmailTemplate)
	r.HandleFunc("PUT", "/ses/v2/email/templates/{templateName}", s.UpdateEmailTemplate)
	r.HandleFunc("DELETE", "/ses/v2/email/templates/{templateName}", s.DeleteEmailTemplate)

	// Send Email routes.
	r.HandleFunc("POST", "/ses/v2/email/outbound-emails", s.SendEmail)
	r.HandleFunc("POST", "/ses/v2/email/outbound-bulk-emails", s.SendBulkEmail)

	// kumo-specific endpoint for testing.
	r.HandleFunc("GET", "/kumo/ses/v2/sent-emails", s.GetSentEmails)
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
