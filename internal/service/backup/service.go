package backup

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

// Service implements the AWS Backup service.
type Service struct {
	storage Storage
}

// New creates a new Backup service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "backup"
}

// Prefix returns the URL prefix for Backup.
func (s *Service) Prefix() string {
	return "/backup-vaults"
}

// RegisterRoutes registers routes with the router.
func (s *Service) RegisterRoutes(r service.Router) {
	// Backup vault operations.
	r.Handle("PUT", "/backup-vaults/{backupVaultName}", s.CreateBackupVault)
	r.Handle("GET", "/backup-vaults/{backupVaultName}", s.DescribeBackupVault)
	r.Handle("GET", "/backup-vaults", s.ListBackupVaults)
	r.Handle("DELETE", "/backup-vaults/{backupVaultName}", s.DeleteBackupVault)

	// Backup plan operations.
	r.Handle("PUT", "/backup/plans", s.CreateBackupPlan)
	r.Handle("GET", "/backup/plans/{backupPlanId}", s.GetBackupPlan)
	r.Handle("GET", "/backup/plans", s.ListBackupPlans)
	r.Handle("DELETE", "/backup/plans/{backupPlanId}", s.DeleteBackupPlan)

	// Backup selection operations.
	r.Handle("PUT", "/backup/plans/{backupPlanId}/selections", s.CreateBackupSelection)
	r.Handle("GET", "/backup/plans/{backupPlanId}/selections/{selectionId}", s.GetBackupSelection)
	r.Handle("GET", "/backup/plans/{backupPlanId}/selections", s.ListBackupSelections)
	r.Handle("DELETE", "/backup/plans/{backupPlanId}/selections/{selectionId}", s.DeleteBackupSelection)
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
		Display:     "Backup",
		Category:    "Management & Configuration",
		Description: "Centralized backup service",
	}
}
