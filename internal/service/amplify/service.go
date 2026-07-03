package amplify

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

// Service implements the Amplify service.
type Service struct {
	storage Storage
}

// New creates a new Amplify service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "amplify"
}

// Prefix returns the URL prefix for Amplify.
func (s *Service) Prefix() string {
	return "/apps"
}

// RegisterRoutes registers routes with the router.
func (s *Service) RegisterRoutes(r service.Router) {
	// App operations.
	r.Handle("POST", "/apps", s.CreateApp)
	r.Handle("GET", "/apps", s.ListApps)
	r.Handle("GET", "/apps/{appId}", s.GetApp)
	r.Handle("POST", "/apps/{appId}", s.UpdateApp)
	r.Handle("DELETE", "/apps/{appId}", s.DeleteApp)

	// Branch operations.
	r.Handle("POST", "/apps/{appId}/branches", s.CreateBranch)
	r.Handle("GET", "/apps/{appId}/branches", s.ListBranches)
	r.Handle("GET", "/apps/{appId}/branches/{branchName}", s.GetBranch)
	r.Handle("DELETE", "/apps/{appId}/branches/{branchName}", s.DeleteBranch)
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
		Display:     "Amplify",
		Category:    "Application Integration",
		Description: "Full-stack application hosting",
	}
}
