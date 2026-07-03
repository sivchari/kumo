package rds

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

	storage := NewMemoryStorage(opts...)
	service.Register(New(storage))
}

// Service implements the RDS service.
type Service struct {
	storage Storage
}

// New creates a new RDS service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "rds"
}

// RegisterRoutes registers routes with the router.
// RDS uses Query protocol, so routes are registered via DispatchAction.
func (s *Service) RegisterRoutes(_ service.Router) {
	// RDS uses Query protocol, routing is handled by DispatchAction.
}

// TargetPrefix returns the X-Amz-Target header prefix for RDS.
// RDS uses Action parameter instead of X-Amz-Target header.
func (s *Service) TargetPrefix() string {
	return "AmazonRDSv19"
}

// Actions returns the list of action names this service handles.
func (s *Service) Actions() []string {
	return []string{
		"CreateDBInstance",
		"DeleteDBInstance",
		"DescribeDBInstances",
		"ModifyDBInstance",
		"StartDBInstance",
		"StopDBInstance",
		"CreateDBCluster",
		"DeleteDBCluster",
		"DescribeDBClusters",
		"ModifyDBCluster",
		"CreateDBSnapshot",
		"DeleteDBSnapshot",
	}
}

// ServiceIdentifier returns the SDK service identifier for User-Agent disambiguation.
func (s *Service) ServiceIdentifier() string {
	return "rds"
}

// QueryProtocol is a marker method that indicates RDS uses AWS Query protocol.
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
		Display:     "RDS",
		Category:    "Database",
		Description: "Relational database service",
	}
}
