// Package macie2 provides a mock implementation of Amazon Macie2.
package macie2

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check to ensure Service implements service.Service.
var _ service.Service = (*Service)(nil)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Service implements the Amazon Macie2 service.
type Service struct {
	storage Storage
}

// New creates a new Macie2 service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "macie2"
}

// RegisterRoutes registers the Macie2 service routes.
// Macie2 uses REST JSON API with various HTTP methods.
func (s *Service) RegisterRoutes(r service.Router) {
	// Macie session operations
	r.Handle("POST", "/macie", s.EnableMacie)
	r.Handle("GET", "/macie", s.GetMacieSession)
	r.Handle("PATCH", "/macie", s.UpdateMacieSession)
	r.Handle("DELETE", "/macie", s.DisableMacie)

	// Allow list operations
	r.Handle("POST", "/allow-lists", s.CreateAllowList)
	r.Handle("GET", "/allow-lists", s.ListAllowLists)
	r.Handle("GET", "/allow-lists/{id}", s.GetAllowList)
	r.Handle("PUT", "/allow-lists/{id}", s.UpdateAllowList)
	r.Handle("DELETE", "/allow-lists/{id}", s.DeleteAllowList)

	// Classification job operations
	r.Handle("POST", "/jobs", s.CreateClassificationJob)
	r.Handle("GET", "/jobs/{jobId}", s.DescribeClassificationJob)
	r.Handle("POST", "/jobs/list", s.ListClassificationJobs)
	r.Handle("PATCH", "/jobs/{jobId}", s.UpdateClassificationJob)

	// Custom data identifier operations
	r.Handle("POST", "/custom-data-identifiers", s.CreateCustomDataIdentifier)
	r.Handle("GET", "/custom-data-identifiers/{id}", s.GetCustomDataIdentifier)
	r.Handle("DELETE", "/custom-data-identifiers/{id}", s.DeleteCustomDataIdentifier)
	r.Handle("POST", "/custom-data-identifiers/list", s.ListCustomDataIdentifiers)

	// Findings filter operations
	r.Handle("POST", "/findingsfilters", s.CreateFindingsFilter)
	r.Handle("GET", "/findingsfilters", s.ListFindingsFilters)
	r.Handle("GET", "/findingsfilters/{id}", s.GetFindingsFilter)
	r.Handle("PATCH", "/findingsfilters/{id}", s.UpdateFindingsFilter)
	r.Handle("DELETE", "/findingsfilters/{id}", s.DeleteFindingsFilter)

	// Findings operations
	r.Handle("POST", "/findings/describe", s.GetFindings)
	r.Handle("POST", "/findings", s.ListFindings)
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
		Display:     "Macie",
		Category:    "Security & Identity",
		Description: "Data security and privacy",
	}
}
