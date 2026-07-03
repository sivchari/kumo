package finspace

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check to ensure Service implements service.Service.
var (
	_ service.Service = (*Service)(nil)
	_ io.Closer       = (*Service)(nil)
)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Service implements the FinSpace service.
type Service struct {
	storage Storage
}

// New creates a new FinSpace service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "finspace"
}

// RegisterRoutes registers the FinSpace routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// KxEnvironment operations
	r.Handle("POST", "/kx/environments", s.CreateKxEnvironment)
	r.Handle("GET", "/kx/environments/{environmentId}", s.GetKxEnvironment)
	r.Handle("DELETE", "/kx/environments/{environmentId}", s.DeleteKxEnvironment)
	r.Handle("GET", "/kx/environments", s.ListKxEnvironments)
	r.Handle("PUT", "/kx/environments/{environmentId}", s.UpdateKxEnvironment)

	// KxDatabase operations
	r.Handle("POST", "/kx/environments/{environmentId}/databases", s.CreateKxDatabase)
	r.Handle("GET", "/kx/environments/{environmentId}/databases/{databaseName}", s.GetKxDatabase)
	r.Handle("DELETE", "/kx/environments/{environmentId}/databases/{databaseName}", s.DeleteKxDatabase)
	r.Handle("GET", "/kx/environments/{environmentId}/databases", s.ListKxDatabases)
	r.Handle("PUT", "/kx/environments/{environmentId}/databases/{databaseName}", s.UpdateKxDatabase)

	// KxUser operations
	r.Handle("POST", "/kx/environments/{environmentId}/users", s.CreateKxUser)
	r.Handle("GET", "/kx/environments/{environmentId}/users/{userName}", s.GetKxUser)
	r.Handle("DELETE", "/kx/environments/{environmentId}/users/{userName}", s.DeleteKxUser)
	r.Handle("GET", "/kx/environments/{environmentId}/users", s.ListKxUsers)
	r.Handle("PUT", "/kx/environments/{environmentId}/users/{userName}", s.UpdateKxUser)

	// Tag operations
	r.Handle("POST", "/tags", s.TagResource)
	r.Handle("DELETE", "/tags", s.UntagResource)
	r.Handle("GET", "/tags", s.ListTagsForResource)
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

// Prefix returns the URL prefix for FinSpace.
func (s *Service) Prefix() string {
	return "/finspace"
}

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "FinSpace",
		Category:    "Other Services",
		Description: "Financial data management",
	}
}
