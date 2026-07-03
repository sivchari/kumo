package redshift

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time interface checks.
var (
	_ io.Closer                    = (*Service)(nil)
	_ service.QueryProtocolService = (*Service)(nil)
)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	storage := NewMemoryStorage(opts...)
	service.Register(New(storage))
}

// Service implements the Redshift service.
type Service struct {
	storage Storage
}

// New creates a new Redshift service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "redshift"
}

// RegisterRoutes registers routes with the router.
// Redshift uses Query protocol, so routes are registered via DispatchAction.
func (s *Service) RegisterRoutes(_ service.Router) {
	// Redshift uses Query protocol, routing is handled by DispatchAction.
}

// TargetPrefix returns the X-Amz-Target header prefix for Redshift.
// Redshift uses Action parameter instead of X-Amz-Target header.
func (s *Service) TargetPrefix() string {
	return "RedshiftServiceVersion20121201"
}

// Actions returns the list of action names this service handles.
func (s *Service) Actions() []string {
	return []string{
		"CreateCluster",
		"DeleteCluster",
		"DescribeClusters",
		"ModifyCluster",
		"CreateClusterSnapshot",
		"DeleteClusterSnapshot",
		"DescribeClusterSnapshots",
	}
}

// ServiceIdentifier returns the SDK service identifier for User-Agent disambiguation.
func (s *Service) ServiceIdentifier() string {
	return "redshift"
}

// QueryProtocol is a marker method that indicates Redshift uses AWS Query protocol.
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

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "Redshift",
		Category:    "Database",
		Description: "Data warehousing",
	}
}
