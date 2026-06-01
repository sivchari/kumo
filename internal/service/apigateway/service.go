// Package apigateway provides API Gateway service emulation for kumo.
package apigateway

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/service/execapi"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	svc := New(NewMemoryStorage(opts...))
	svc.baseURL = execapi.ResolveBaseURL()

	service.Register(svc)
}

// Service implements the API Gateway service.
type Service struct {
	storage Storage
	baseURL string
}

// New creates a new API Gateway service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "apigateway"
}

// RegisterRoutes registers the API Gateway routes.
//
// Routes are registered under both the /apigateway/... prefix (legacy
// per-service BaseEndpoint) and the bare /restapis/... prefix that
// terraform-provider-aws and aws-sdk-go-v2 use against the unified
// endpoint.
func (s *Service) RegisterRoutes(r service.Router) {
	for _, prefix := range []string{"/apigateway", ""} {
		// REST API routes.
		r.HandleFunc("POST", prefix+"/restapis", s.CreateRestAPI)
		r.HandleFunc("GET", prefix+"/restapis", s.GetRestAPIs)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}", s.GetRestAPI)
		r.HandleFunc("DELETE", prefix+"/restapis/{restApiId}", s.DeleteRestAPI)

		// Resource routes.
		r.HandleFunc("POST", prefix+"/restapis/{restApiId}/resources/{parentId}", s.CreateResource)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}/resources", s.GetResources)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}/resources/{resourceId}", s.GetResource)
		r.HandleFunc("DELETE", prefix+"/restapis/{restApiId}/resources/{resourceId}", s.DeleteResource)

		// Method routes.
		r.HandleFunc("PUT", prefix+"/restapis/{restApiId}/resources/{resourceId}/methods/{httpMethod}", s.PutMethod)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}/resources/{resourceId}/methods/{httpMethod}", s.GetMethod)
		r.HandleFunc("DELETE", prefix+"/restapis/{restApiId}/resources/{resourceId}/methods/{httpMethod}", s.DeleteMethod)

		// Integration routes.
		r.HandleFunc("PUT", prefix+"/restapis/{restApiId}/resources/{resourceId}/methods/{httpMethod}/integration", s.PutIntegration)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}/resources/{resourceId}/methods/{httpMethod}/integration", s.GetIntegration)

		// Deployment routes.
		r.HandleFunc("POST", prefix+"/restapis/{restApiId}/deployments", s.CreateDeployment)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}/deployments", s.GetDeployments)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}/deployments/{deploymentId}", s.GetDeployment)
		r.HandleFunc("DELETE", prefix+"/restapis/{restApiId}/deployments/{deploymentId}", s.DeleteDeployment)

		// Stage routes.
		r.HandleFunc("POST", prefix+"/restapis/{restApiId}/stages", s.CreateStage)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}/stages", s.GetStages)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}/stages/{stageName}", s.GetStage)
		r.HandleFunc("DELETE", prefix+"/restapis/{restApiId}/stages/{stageName}", s.DeleteStage)
	}
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
