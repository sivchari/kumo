// Package dlm provides Data Lifecycle Manager service emulation for kumo.
package dlm

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

// Service implements the DLM service.
type Service struct {
	storage Storage
}

// New creates a new DLM service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "dlm"
}

// RegisterRoutes registers the service routes.
func (s *Service) RegisterRoutes(r service.Router) {
	r.HandleFunc("POST", "/dlm/policies", s.CreateLifecyclePolicy)
	r.HandleFunc("GET", "/dlm/policies", s.GetLifecyclePolicies)
	r.HandleFunc("GET", "/dlm/policies/{policyId}", s.GetLifecyclePolicy)
	r.HandleFunc("PATCH", "/dlm/policies/{policyId}", s.UpdateLifecyclePolicy)
	r.HandleFunc("DELETE", "/dlm/policies/{policyId}", s.DeleteLifecyclePolicy)
}

// Ensure Service implements service.Service.
var _ service.Service = (*Service)(nil)

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
		Display:     "DLM",
		Category:    "Other Services",
		Description: "Data lifecycle manager",
	}
}
