package documentdb

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

// Service implements the DocumentDB service.
type Service struct {
	storage Storage
}

// New creates a new DocumentDB service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "docdb"
}

// RegisterRoutes registers routes with the router.
// DocumentDB uses Query protocol, so routes are registered via DispatchAction.
func (s *Service) RegisterRoutes(_ service.Router) {
	// DocumentDB uses Query protocol, routing is handled by DispatchAction.
}

// TargetPrefix returns the X-Amz-Target header prefix for DocumentDB.
// DocumentDB uses the same target prefix as RDS.
func (s *Service) TargetPrefix() string {
	return "AmazonRDSv19"
}

// Actions returns the list of action names this service handles.
func (s *Service) Actions() []string {
	return []string{
		"CreateDBCluster",
		"DeleteDBCluster",
		"DescribeDBClusters",
		"ModifyDBCluster",
		"CreateDBInstance",
		"DeleteDBInstance",
		"DescribeDBInstances",
	}
}

// ServiceIdentifier returns the SDK service identifier for User-Agent disambiguation.
func (s *Service) ServiceIdentifier() string {
	return "docdb"
}

// QueryProtocol is a marker method that indicates DocumentDB uses AWS Query protocol.
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
		Display:     "DocumentDB",
		Category:    "Database",
		Description: "MongoDB-compatible database",
	}
}
