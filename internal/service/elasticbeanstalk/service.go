package elasticbeanstalk

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

// Service implements the AWS Elastic Beanstalk service.
type Service struct {
	storage Storage
}

// New creates a new Elastic Beanstalk service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "elasticbeanstalk"
}

// TargetPrefix returns the X-Amz-Target header prefix.
func (s *Service) TargetPrefix() string {
	return "ElasticBeanstalk"
}

// Actions returns the list of action names this service handles.
func (s *Service) Actions() []string {
	return []string{
		"CreateApplication",
		"DescribeApplications",
		"UpdateApplication",
		"DeleteApplication",
		"CreateEnvironment",
		"DescribeEnvironments",
		"TerminateEnvironment",
	}
}

// ServiceIdentifier returns the SDK service identifier for User-Agent disambiguation.
func (s *Service) ServiceIdentifier() string {
	return "elasticbeanstalk"
}

// QueryProtocol marks this service as using AWS Query protocol.
func (s *Service) QueryProtocol() {}

// RegisterRoutes registers routes with the router.
func (s *Service) RegisterRoutes(_ service.Router) {
	// Query protocol services use DispatchAction for routing.
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

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "Elastic Beanstalk",
		Category:    "Compute",
		Description: "Application deployment",
	}
}
