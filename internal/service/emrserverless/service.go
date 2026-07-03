package emrserverless

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

// Service implements the EMR Serverless service.
type Service struct {
	storage Storage
}

// New creates a new EMR Serverless service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
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

// Name returns the service name.
func (s *Service) Name() string {
	return "emrserverless"
}

// RegisterRoutes registers the service routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Application operations.
	r.Handle("POST", "/applications", s.CreateApplication)
	r.Handle("GET", "/applications/{applicationId}", s.GetApplication)
	r.Handle("GET", "/applications", s.ListApplications)
	r.Handle("PATCH", "/applications/{applicationId}", s.UpdateApplication)
	r.Handle("DELETE", "/applications/{applicationId}", s.DeleteApplication)
	r.Handle("POST", "/applications/{applicationId}/start", s.StartApplication)
	r.Handle("POST", "/applications/{applicationId}/stop", s.StopApplication)

	// Job run operations.
	r.Handle("POST", "/applications/{applicationId}/jobruns", s.StartJobRun)
	r.Handle("GET", "/applications/{applicationId}/jobruns/{jobRunId}", s.GetJobRun)
	r.Handle("GET", "/applications/{applicationId}/jobruns", s.ListJobRuns)
	r.Handle("DELETE", "/applications/{applicationId}/jobruns/{jobRunId}", s.CancelJobRun)
}

// Ensure Service implements service.Service.
var _ service.Service = (*Service)(nil)

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "EMR Serverless",
		Category:    "Other Services",
		Description: "Big data processing",
	}
}
