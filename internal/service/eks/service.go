// Package eks provides an EKS service emulator.
package eks

import (
	"fmt"
	"io"
	"net/http"
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

// Service implements the EKS service.
type Service struct {
	storage Storage
}

// New creates a new EKS service.
func New(storage Storage) *Service {
	return &Service{storage: storage}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "eks"
}

// RegisterRoutes registers the service routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Cluster operations
	r.HandleFunc("POST", "/eks/clusters", s.CreateCluster)
	r.HandleFunc("DELETE", "/eks/clusters/{name}", s.DeleteCluster)
	r.HandleFunc("GET", "/eks/clusters/{name}", s.handleClusterGet)
	r.HandleFunc("GET", "/eks/clusters", s.ListClusters)

	// Nodegroup operations
	r.HandleFunc("POST", "/eks/clusters/{name}/node-groups", s.CreateNodegroup)
	r.HandleFunc("DELETE", "/eks/clusters/{name}/node-groups/{nodegroupName}", s.DeleteNodegroup)
	r.HandleFunc("GET", "/eks/clusters/{name}/node-groups/{nodegroupName}", s.DescribeNodegroup)
	r.HandleFunc("GET", "/eks/clusters/{name}/node-groups", s.ListNodegroups)
}

// handleClusterGet handles GET requests to /clusters/{name}.
// This is needed because the router might match both DescribeCluster and ListNodegroups.
func (s *Service) handleClusterGet(w http.ResponseWriter, r *http.Request) {
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
		Display:     "EKS",
		Category:    "Container",
		Description: "Kubernetes service",
	}
}
