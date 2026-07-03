package appsync

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

// Service implements the AppSync service.
type Service struct {
	storage Storage
}

// New creates a new AppSync service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "appsync"
}

// RegisterRoutes registers the AppSync routes.
// AppSync uses REST API protocol with /v1 prefix.
// Note: Routes use /appsync prefix for internal routing to avoid S3 conflicts.
func (s *Service) RegisterRoutes(r service.Router) {
	// GraphQL API operations.
	r.HandleFunc("POST", "/appsync/v1/apis", s.CreateGraphqlAPI)
	r.HandleFunc("DELETE", "/appsync/v1/apis/{apiId}", s.DeleteGraphqlAPI)
	r.HandleFunc("GET", "/appsync/v1/apis/{apiId}", s.GetGraphqlAPI)
	r.HandleFunc("GET", "/appsync/v1/apis", s.ListGraphqlAPIs)

	// Data source operations.
	r.HandleFunc("POST", "/appsync/v1/apis/{apiId}/datasources", s.CreateDataSource)

	// Resolver operations.
	r.HandleFunc("POST", "/appsync/v1/apis/{apiId}/types/{typeName}/resolvers", s.CreateResolver)

	// Schema operations.
	r.HandleFunc("POST", "/appsync/v1/apis/{apiId}/schemacreation", s.StartSchemaCreation)
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
		Display:     "AppSync",
		Category:    "Application Integration",
		Description: "GraphQL API",
	}
}
