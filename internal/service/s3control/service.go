package s3control

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

// Service implements the S3 Control service.
type Service struct {
	storage Storage
}

// New creates a new S3 Control service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "s3control"
}

// RegisterRoutes registers the S3 Control routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Public Access Block operations
	r.Handle("GET", "/v20180820/configuration/publicAccessBlock", s.GetPublicAccessBlock)
	r.Handle("PUT", "/v20180820/configuration/publicAccessBlock", s.PutPublicAccessBlock)
	r.Handle("DELETE", "/v20180820/configuration/publicAccessBlock", s.DeletePublicAccessBlock)

	// Access Point operations
	r.Handle("PUT", "/v20180820/accesspoint/{name}", s.CreateAccessPoint)
	r.Handle("GET", "/v20180820/accesspoint/{name}", s.GetAccessPoint)
	r.Handle("DELETE", "/v20180820/accesspoint/{name}", s.DeleteAccessPoint)
	r.Handle("GET", "/v20180820/accesspoint", s.ListAccessPoints)
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
		Display:     "S3 Control",
		Category:    "Storage",
		Description: "S3 account-level operations",
	}
}
