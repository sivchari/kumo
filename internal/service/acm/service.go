package acm

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// dnsValidationDelayEnvVar is the env var holding how long a DNS-validated
// certificate stays PENDING_VALIDATION before it is issued, as a Go
// duration such as "30s". Unset, DNS-validated certificates are issued as
// soon as they are requested.
const dnsValidationDelayEnvVar = "KUMO_ACM_DNS_VALIDATION_DELAY"

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	if delay, ok := dnsValidationDelay(os.Getenv(dnsValidationDelayEnvVar)); ok {
		opts = append(opts, WithDNSValidationDelay(delay))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// dnsValidationDelay parses the value of dnsValidationDelayEnvVar. An empty
// value is not set; a value that is not a non-negative duration is ignored
// with a warning, keeping the default of issuing right away.
func dnsValidationDelay(value string) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}

	delay, err := time.ParseDuration(value)
	if err != nil || delay < 0 {
		slog.Warn("ignoring invalid ACM DNS validation delay; DNS-validated certificates are issued immediately",
			"env", dnsValidationDelayEnvVar, "value", value)

		return 0, false
	}

	return delay, true
}

// Service implements the ACM service.
type Service struct {
	storage Storage
}

// New creates a new ACM service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "acm"
}

// RegisterRoutes registers the ACM routes.
// Note: ACM uses AWS JSON 1.1 protocol via the JSONProtocolService interface,
// so no direct routes are registered here.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - ACM uses JSON protocol dispatcher
}

// TargetPrefix returns the X-Amz-Target header prefix for ACM.
func (s *Service) TargetPrefix() string {
	return "CertificateManager"
}

// JSONProtocol is a marker method that indicates ACM uses AWS JSON 1.1 protocol.
func (s *Service) JSONProtocol() {}

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
		Display:     "ACM",
		Category:    "Security & Identity",
		Description: "Certificate management",
	}
}
