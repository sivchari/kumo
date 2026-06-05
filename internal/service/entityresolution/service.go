// Package entityresolution provides an AWS Entity Resolution service emulator.
package entityresolution

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

// Service implements the Entity Resolution service.
type Service struct {
	storage Storage
}

// New creates a new Entity Resolution service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
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
	return "entityresolution"
}

// RegisterRoutes registers the Entity Resolution routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Schema mapping routes.
	r.Handle("POST", "/schemas", s.CreateSchemaMapping)
	r.Handle("GET", "/schemas", s.ListSchemaMappings)
	r.Handle("GET", "/schemas/{schemaName}", s.GetSchemaMapping)
	r.Handle("DELETE", "/schemas/{schemaName}", s.DeleteSchemaMapping)

	// Matching workflow routes.
	r.Handle("POST", "/matchingworkflows", s.CreateMatchingWorkflow)
	r.Handle("GET", "/matchingworkflows", s.ListMatchingWorkflows)
	r.Handle("GET", "/matchingworkflows/{workflowName}", s.GetMatchingWorkflow)
	r.Handle("DELETE", "/matchingworkflows/{workflowName}", s.DeleteMatchingWorkflow)

	// ID mapping workflow routes.
	r.Handle("POST", "/idmappingworkflows", s.CreateIDMappingWorkflow)
	r.Handle("GET", "/idmappingworkflows", s.ListIDMappingWorkflows)
	r.Handle("GET", "/idmappingworkflows/{workflowName}", s.GetIDMappingWorkflow)
	r.Handle("DELETE", "/idmappingworkflows/{workflowName}", s.DeleteIDMappingWorkflow)

	// Provider service routes.
	r.Handle("GET", "/providerservices", s.ListProviderServices)
}

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "Entity Resolution",
		Category:    "Analytics & ML",
		Description: "Entity matching",
	}
}
