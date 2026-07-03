// Package scheduler provides EventBridge Scheduler service emulation for kumo.
package scheduler

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

// Service implements the EventBridge Scheduler service.
type Service struct {
	storage Storage
}

// New creates a new Scheduler service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "scheduler"
}

// RegisterRoutes registers the service routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Schedule operations
	r.HandleFunc("POST", "/scheduler/schedules/{name}", s.CreateSchedule)
	r.HandleFunc("GET", "/scheduler/schedules/{name}", s.GetSchedule)
	r.HandleFunc("PUT", "/scheduler/schedules/{name}", s.UpdateSchedule)
	r.HandleFunc("DELETE", "/scheduler/schedules/{name}", s.DeleteSchedule)
	r.HandleFunc("GET", "/scheduler/schedules", s.ListSchedules)

	// Schedule group operations
	r.HandleFunc("POST", "/scheduler/schedule-groups/{name}", s.CreateScheduleGroup)
	r.HandleFunc("GET", "/scheduler/schedule-groups/{name}", s.GetScheduleGroup)
	r.HandleFunc("DELETE", "/scheduler/schedule-groups/{name}", s.DeleteScheduleGroup)
	r.HandleFunc("GET", "/scheduler/schedule-groups", s.ListScheduleGroups)
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
		Display:     "Scheduler",
		Category:    "Application Integration",
		Description: "Task scheduling",
	}
}
