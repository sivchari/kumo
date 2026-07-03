package ebs

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

// Service implements the AWS EBS direct API service.
type Service struct {
	storage Storage
}

// New creates a new EBS service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "ebs"
}

// RegisterRoutes registers the EBS routes.
func (s *Service) RegisterRoutes(r service.Router) {
	r.Handle("POST", "/snapshots", s.StartSnapshotHandler)
	r.Handle("POST", "/snapshots/completion/{snapshotId}", s.CompleteSnapshotHandler)
	r.Handle("GET", "/snapshots/{snapshotId}/blocks", s.ListSnapshotBlocksHandler)
	r.Handle("PUT", "/snapshots/{snapshotId}/blocks/{blockIndex}", s.PutSnapshotBlockHandler)
	r.Handle("GET", "/snapshots/{snapshotId}/blocks/{blockIndex}", s.GetSnapshotBlockHandler)
	r.Handle("GET", "/snapshots/{secondSnapshotId}/changedblocks", s.ListChangedBlocksHandler)
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

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "EBS",
		Category:    "Storage",
		Description: "Block storage",
	}
}
