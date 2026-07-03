package cloudfront

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

// Service implements the CloudFront service.
type Service struct {
	storage   Storage
	edgeCache *edgeCache
}

// New creates a new CloudFront service.
func New(storage Storage) *Service {
	return &Service{
		storage:   storage,
		edgeCache: newEdgeCache(),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "cloudfront"
}

// RegisterRoutes registers the CloudFront routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Distribution operations.
	r.Handle("POST", "/2020-05-31/distribution", s.CreateDistribution)
	r.Handle("GET", "/2020-05-31/distribution", s.ListDistributions)
	r.Handle("GET", "/2020-05-31/distribution/{id}", s.GetDistribution)
	r.Handle("GET", "/2020-05-31/distribution/{id}/config", s.GetDistributionConfig)
	r.Handle("PUT", "/2020-05-31/distribution/{id}/config", s.UpdateDistribution)
	r.Handle("DELETE", "/2020-05-31/distribution/{id}", s.DeleteDistribution)

	// Invalidation operations.
	r.Handle("POST", "/2020-05-31/distribution/{id}/invalidation", s.CreateInvalidation)
	r.Handle("GET", "/2020-05-31/distribution/{id}/invalidation/{invalidationId}", s.GetInvalidation)

	// PublicKey + KeyGroup — building blocks for signed URL / signed cookie verification.
	r.Handle("POST", "/2020-05-31/public-key", s.CreatePublicKey)
	r.Handle("GET", "/2020-05-31/public-key", s.ListPublicKeys)
	r.Handle("GET", "/2020-05-31/public-key/{id}", s.GetPublicKey)
	r.Handle("DELETE", "/2020-05-31/public-key/{id}", s.DeletePublicKey)

	r.Handle("POST", "/2020-05-31/key-group", s.CreateKeyGroup)
	r.Handle("GET", "/2020-05-31/key-group", s.ListKeyGroups)
	r.Handle("GET", "/2020-05-31/key-group/{id}", s.GetKeyGroup)
	r.Handle("DELETE", "/2020-05-31/key-group/{id}", s.DeleteKeyGroup)

	// Edge — proxies real requests through the cache layer.
	for _, method := range []string{"GET", "HEAD", "PUT", "POST", "DELETE", "PATCH"} {
		r.Handle(method, "/kumo/cdn/{distributionId}/{path...}", s.Edge)
	}
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
		Display:     "CloudFront",
		Category:    "Networking & Content Delivery",
		Description: "CDN",
	}
}
