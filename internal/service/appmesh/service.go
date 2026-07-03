package appmesh

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Service implements the App Mesh service.
type Service struct {
	storage Storage
}

// New creates a new App Mesh service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Name returns the service name.
func (s *Service) Name() string {
	return "appmesh"
}

// RegisterRoutes registers the service routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Mesh operations.
	r.Handle("PUT", "/v20190125/meshes", s.CreateMesh)
	r.Handle("GET", "/v20190125/meshes/{meshName}", s.DescribeMesh)
	r.Handle("GET", "/v20190125/meshes", s.ListMeshes)
	r.Handle("PUT", "/v20190125/meshes/{meshName}", s.UpdateMesh)
	r.Handle("DELETE", "/v20190125/meshes/{meshName}", s.DeleteMesh)

	// Virtual Node operations.
	r.Handle("PUT", "/v20190125/meshes/{meshName}/virtualNodes", s.CreateVirtualNode)
	r.Handle("GET", "/v20190125/meshes/{meshName}/virtualNodes/{virtualNodeName}", s.DescribeVirtualNode)
	r.Handle("GET", "/v20190125/meshes/{meshName}/virtualNodes", s.ListVirtualNodes)
	r.Handle("PUT", "/v20190125/meshes/{meshName}/virtualNodes/{virtualNodeName}", s.UpdateVirtualNode)
	r.Handle("DELETE", "/v20190125/meshes/{meshName}/virtualNodes/{virtualNodeName}", s.DeleteVirtualNode)

	// Virtual Service operations.
	r.Handle("PUT", "/v20190125/meshes/{meshName}/virtualServices", s.CreateVirtualService)
	r.Handle("GET", "/v20190125/meshes/{meshName}/virtualServices/{virtualServiceName}", s.DescribeVirtualService)
	r.Handle("GET", "/v20190125/meshes/{meshName}/virtualServices", s.ListVirtualServices)
	r.Handle("PUT", "/v20190125/meshes/{meshName}/virtualServices/{virtualServiceName}", s.UpdateVirtualService)
	r.Handle("DELETE", "/v20190125/meshes/{meshName}/virtualServices/{virtualServiceName}", s.DeleteVirtualService)

	// Virtual Router operations.
	r.Handle("PUT", "/v20190125/meshes/{meshName}/virtualRouters", s.CreateVirtualRouter)
	r.Handle("GET", "/v20190125/meshes/{meshName}/virtualRouters/{virtualRouterName}", s.DescribeVirtualRouter)
	r.Handle("GET", "/v20190125/meshes/{meshName}/virtualRouters", s.ListVirtualRouters)
	r.Handle("PUT", "/v20190125/meshes/{meshName}/virtualRouters/{virtualRouterName}", s.UpdateVirtualRouter)
	r.Handle("DELETE", "/v20190125/meshes/{meshName}/virtualRouters/{virtualRouterName}", s.DeleteVirtualRouter)

	// Route operations.
	r.Handle("PUT", "/v20190125/meshes/{meshName}/virtualRouter/{virtualRouterName}/routes", s.CreateRoute)
	r.Handle("GET", "/v20190125/meshes/{meshName}/virtualRouter/{virtualRouterName}/routes/{routeName}", s.DescribeRoute)
	r.Handle("GET", "/v20190125/meshes/{meshName}/virtualRouter/{virtualRouterName}/routes", s.ListRoutes)
	r.Handle("PUT", "/v20190125/meshes/{meshName}/virtualRouter/{virtualRouterName}/routes/{routeName}", s.UpdateRoute)
	r.Handle("DELETE", "/v20190125/meshes/{meshName}/virtualRouter/{virtualRouterName}/routes/{routeName}", s.DeleteRoute)
}

// Compile-time interface check.
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
		Display:     "App Mesh",
		Category:    "Networking & Content Delivery",
		Description: "Service mesh",
	}
}
