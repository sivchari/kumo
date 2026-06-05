package batch

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

// Service implements the Batch service.
type Service struct {
	storage Storage
}

// New creates a new Batch service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "batch"
}

// RegisterRoutes registers the Batch routes.
// Batch uses REST JSON protocol with paths like /v1/createcomputeenvironment.
func (s *Service) RegisterRoutes(r service.Router) {
	// Compute Environment operations
	r.Handle("POST", "/v1/createcomputeenvironment", s.CreateComputeEnvironment)
	r.Handle("POST", "/v1/deletecomputeenvironment", s.DeleteComputeEnvironment)
	r.Handle("POST", "/v1/describecomputeenvironments", s.DescribeComputeEnvironments)

	// Job Queue operations
	r.Handle("POST", "/v1/createjobqueue", s.CreateJobQueue)
	r.Handle("POST", "/v1/deletejobqueue", s.DeleteJobQueue)
	r.Handle("POST", "/v1/describejobqueues", s.DescribeJobQueues)

	// Job Definition operations
	r.Handle("POST", "/v1/registerjobdefinition", s.RegisterJobDefinition)

	// Job operations
	r.Handle("POST", "/v1/submitjob", s.SubmitJob)
	r.Handle("POST", "/v1/describejobs", s.DescribeJobs)
	r.Handle("POST", "/v1/terminatejob", s.TerminateJob)
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
		Display:     "Batch",
		Category:    "Compute",
		Description: "Batch computing",
	}
}
