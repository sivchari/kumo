package glacier

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

// Service implements the AWS Glacier service.
type Service struct {
	storage Storage
}

// New creates a new Glacier service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "glacier"
}

// RegisterRoutes registers the Glacier routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Vault operations.
	r.Handle("PUT", "/-/vaults/{vaultName}", s.CreateVault)
	r.Handle("GET", "/-/vaults/{vaultName}", s.DescribeVault)
	r.Handle("DELETE", "/-/vaults/{vaultName}", s.DeleteVault)
	r.Handle("GET", "/-/vaults", s.ListVaults)
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

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "Glacier",
		Category:    "Storage",
		Description: "Archive storage",
	}
}
