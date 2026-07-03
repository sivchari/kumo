// Package s3tables provides S3 Tables service emulation for kumo.
package s3tables

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

// Service implements the S3 Tables service.
type Service struct {
	storage Storage
}

// New creates a new S3 Tables service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "s3tables"
}

// RegisterRoutes registers the service routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Table bucket operations
	// SDK uses PUT for Create, GET for List/Get, DELETE for Delete
	r.HandleFunc("PUT", "/buckets", s.CreateTableBucket)
	r.HandleFunc("DELETE", "/buckets/{tableBucketARN}", s.DeleteTableBucket)
	r.HandleFunc("GET", "/buckets/{tableBucketARN}", s.GetTableBucket)
	r.HandleFunc("GET", "/buckets", s.ListTableBuckets)

	// Namespace operations
	r.HandleFunc("PUT", "/namespaces/{tableBucketARN}", s.CreateNamespace)
	r.HandleFunc("DELETE", "/namespaces/{tableBucketARN}/{namespace}", s.DeleteNamespace)
	r.HandleFunc("GET", "/namespaces/{tableBucketARN}/{namespace}", s.GetNamespace)
	r.HandleFunc("GET", "/namespaces/{tableBucketARN}", s.ListNamespaces)

	// Table operations
	r.HandleFunc("PUT", "/tables/{tableBucketARN}/{namespace}", s.CreateTable)
	r.HandleFunc("DELETE", "/tables/{tableBucketARN}/{namespace}/{tableName}", s.DeleteTable)
	r.HandleFunc("GET", "/get-table", s.GetTable)
	r.HandleFunc("GET", "/tables/{tableBucketARN}/{namespace}", s.ListTables)
	r.HandleFunc("GET", "/tables/{tableBucketARN}", s.ListTables)
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
		Display:     "S3 Tables",
		Category:    "Storage",
		Description: "S3 table buckets",
	}
}
