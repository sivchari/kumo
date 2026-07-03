package dataexchange

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

// Service implements the AWS Data Exchange service.
type Service struct {
	storage Storage
}

// New creates a new Data Exchange service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "dataexchange"
}

// Prefix returns the URL prefix for Data Exchange.
func (s *Service) Prefix() string {
	return "/v1"
}

// RegisterRoutes registers routes with the router.
func (s *Service) RegisterRoutes(r service.Router) {
	// Data set operations.
	r.Handle("POST", "/v1/data-sets", s.CreateDataSet)
	r.Handle("GET", "/v1/data-sets", s.ListDataSets)
	r.Handle("GET", "/v1/data-sets/{dataSetId}", s.GetDataSet)
	r.Handle("PATCH", "/v1/data-sets/{dataSetId}", s.UpdateDataSet)
	r.Handle("DELETE", "/v1/data-sets/{dataSetId}", s.DeleteDataSet)

	// Revision operations.
	r.Handle("POST", "/v1/data-sets/{dataSetId}/revisions", s.CreateRevision)
	r.Handle("GET", "/v1/data-sets/{dataSetId}/revisions", s.ListRevisions)
	r.Handle("GET", "/v1/data-sets/{dataSetId}/revisions/{revisionId}", s.GetRevision)
	r.Handle("PATCH", "/v1/data-sets/{dataSetId}/revisions/{revisionId}", s.UpdateRevision)
	r.Handle("DELETE", "/v1/data-sets/{dataSetId}/revisions/{revisionId}", s.DeleteRevision)

	// Job operations.
	r.Handle("POST", "/v1/jobs", s.CreateJob)
	r.Handle("GET", "/v1/jobs", s.ListJobs)
	r.Handle("GET", "/v1/jobs/{jobId}", s.GetJob)
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
		Display:     "Data Exchange",
		Category:    "Analytics & ML",
		Description: "Data marketplace",
	}
}
