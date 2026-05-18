package route53

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

// Service is the Route 53 service.
type Service struct {
	storage Storage
}

// New creates a new Route 53 service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "route53"
}

// RegisterRoutes registers the service routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Hosted Zones
	r.Handle("POST", "/2013-04-01/hostedzone", s.CreateHostedZone)
	r.Handle("GET", "/2013-04-01/hostedzone", s.ListHostedZones)
	r.Handle("GET", "/2013-04-01/hostedzonesbyname", s.ListHostedZonesByName)
	r.Handle("GET", "/2013-04-01/hostedzone/{id}", s.GetHostedZone)
	r.Handle("DELETE", "/2013-04-01/hostedzone/{id}", s.DeleteHostedZone)

	// Resource Record Sets
	r.Handle("POST", "/2013-04-01/hostedzone/{id}/rrset", s.ChangeResourceRecordSets)
	r.Handle("GET", "/2013-04-01/hostedzone/{id}/rrset", s.ListResourceRecordSets)

	// Changes
	r.Handle("GET", "/2013-04-01/change/{id}", s.GetChange)

	// Tags
	r.Handle("GET", "/2013-04-01/tags/{type}/{id}", s.ListTagsForResource)
	r.Handle("POST", "/2013-04-01/tags/{type}/{id}", s.ChangeTagsForResource)
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
