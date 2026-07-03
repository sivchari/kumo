// Package kafka provides an MSK (Managed Streaming for Apache Kafka) service emulator.
package kafka

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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

// Service implements the MSK service.
type Service struct {
	storage Storage
}

// New creates a new MSK service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "kafka"
}

// RegisterRoutes registers the service routes.
func (s *Service) RegisterRoutes(r service.Router) {
	r.Handle("POST", "/kafka/v1/clusters", s.CreateCluster)
	r.Handle("GET", "/kafka/v1/clusters", s.ListClusters)
	// Cluster ARN contains slashes, so we use a catch-all and dispatch manually.
	r.Handle("GET", "/kafka/v1/clusters/{rest...}", s.handleGetCluster)
	r.Handle("DELETE", "/kafka/v1/clusters/{rest...}", s.DeleteCluster)
	r.Handle("PUT", "/kafka/v1/clusters/{rest...}", s.UpdateClusterConfiguration)
}

// handleGetCluster dispatches GET requests based on the path suffix.
func (s *Service) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasSuffix(path, "/bootstrap-brokers") {
		s.GetBootstrapBrokers(w, r)

		return
	}

	s.DescribeCluster(w, r)
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
		Display:     "MSK (Kafka)",
		Category:    "Messaging & Integration",
		Description: "Managed streaming for Kafka",
	}
}
