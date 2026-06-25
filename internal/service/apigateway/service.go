// Package apigateway provides API Gateway service emulation for kumo.
package apigateway

import (
	"context"
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

// LambdaInvoker is the subset of the Lambda service the API Gateway needs to
// run a REQUEST authorizer: a synchronous, in-process function invoke. The
// server injects an implementation after registration via SetLambdaInvoker (see
// internal/server/apigateway_lambda_wiring.go), avoiding a direct dependency on
// the lambda package and the import cycle it would create.
type LambdaInvoker interface {
	InvokeSync(ctx context.Context, fn string, payload []byte) ([]byte, error)
}

// Service implements the API Gateway service.
type Service struct {
	storage Storage
	baseURL string
	invoker LambdaInvoker
}

// New creates a new API Gateway service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// SetLambdaInvoker injects the Lambda invoker used by REQUEST authorizers. It
// is called once during server wiring, before any request is served.
func (s *Service) SetLambdaInvoker(invoker LambdaInvoker) {
	s.invoker = invoker
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

		// Authorizer routes.
		r.HandleFunc("POST", prefix+"/restapis/{restApiId}/authorizers", s.CreateAuthorizer)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}/authorizers", s.GetAuthorizers)
		r.HandleFunc("GET", prefix+"/restapis/{restApiId}/authorizers/{authorizerId}", s.GetAuthorizer)
		r.HandleFunc("DELETE", prefix+"/restapis/{restApiId}/authorizers/{authorizerId}", s.DeleteAuthorizer)
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

// Meta returns the service's documentation metadata.
func (s *Service) Meta() service.Meta {
	return service.Meta{
		Display:     "API Gateway",
		Category:    "Networking & Content Delivery",
		Description: "API management (REST API)",
	}
}
