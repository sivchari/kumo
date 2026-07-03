package codeguruprofiler

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

// Service implements the AWS CodeGuru Profiler service.
type Service struct {
	storage Storage
}

// New creates a new CodeGuru Profiler service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "codeguru-profiler"
}

// Prefix returns the URL prefix for CodeGuru Profiler.
func (s *Service) Prefix() string {
	return "/profilingGroups"
}

// RegisterRoutes registers routes with the router.
func (s *Service) RegisterRoutes(r service.Router) {
	r.Handle("POST", "/profilingGroups", s.CreateProfilingGroup)
	r.Handle("GET", "/profilingGroups", s.ListProfilingGroups)
	r.Handle("GET", "/profilingGroups/{profilingGroupName}", s.DescribeProfilingGroup)
	r.Handle("PUT", "/profilingGroups/{profilingGroupName}", s.UpdateProfilingGroup)
	r.Handle("DELETE", "/profilingGroups/{profilingGroupName}", s.DeleteProfilingGroup)
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
		Display:     "CodeGuru Profiler",
		Category:    "Developer Tools",
		Description: "Application profiling",
	}
}
