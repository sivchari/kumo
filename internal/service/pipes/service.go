package pipes

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

// Service implements the EventBridge Pipes service.
type Service struct {
	storage Storage
}

// New creates a new Pipes service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "pipes"
}

// Storage returns the underlying storage. Used by cross-service wiring
// (wirePipesToEventBridge) to install the EventBridge publish target.
func (s *Service) Storage() Storage {
	return s.storage
}

// RegisterRoutes registers the Pipes routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Pipe CRUD operations.
	r.Handle("POST", "/v1/pipes/{name}", s.CreatePipe)
	r.Handle("GET", "/v1/pipes/{name}", s.DescribePipe)
	r.Handle("PUT", "/v1/pipes/{name}", s.UpdatePipe)
	r.Handle("DELETE", "/v1/pipes/{name}", s.DeletePipe)

	// List pipes.
	r.Handle("GET", "/v1/pipes", s.ListPipes)

	// Pipe control operations.
	r.Handle("POST", "/v1/pipes/{name}/start", s.StartPipe)
	r.Handle("POST", "/v1/pipes/{name}/stop", s.StopPipe)

	// Tag operations.
	r.Handle("POST", "/tags/{arn...}", s.TagResource)
	r.Handle("DELETE", "/tags/{arn...}", s.UntagResource)
	r.Handle("GET", "/tags/{arn...}", s.ListTagsForResource)
}

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
		Display:     "Pipes",
		Category:    "Messaging & Integration",
		Description: "Event-driven integration",
	}
}
