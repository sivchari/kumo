// Package apigatewayv2 provides API Gateway v2 (HTTP API) service emulation for kumo.
package apigatewayv2

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/service/execapi"
)

const (
	// defaultRouteSelectionExpression is the default routeSelectionExpression
	// for HTTP APIs when the caller does not provide one.
	defaultRouteSelectionExpression = "$request.method $request.path"

	// deploymentStatusDeployed is the status reported for a created deployment.
	deploymentStatusDeployed = "DEPLOYED"
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

// Service implements the API Gateway v2 service.
type Service struct {
	storage Storage
	baseURL string
}

// New creates a new API Gateway v2 service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "apigatewayv2"
}

// RegisterRoutes registers the API Gateway v2 routes.
//
// Routes are registered under both the /apigatewayv2/... prefix (legacy
// per-service BaseEndpoint) and the bare /v2/... prefix that
// terraform-provider-aws and aws-sdk-go-v2 use against the unified endpoint.
func (s *Service) RegisterRoutes(r service.Router) {
	for _, prefix := range []string{"/apigatewayv2", ""} {
		// API routes.
		r.HandleFunc("POST", prefix+"/v2/apis", s.CreateAPI)
		r.HandleFunc("GET", prefix+"/v2/apis", s.GetAPIs)
		r.HandleFunc("GET", prefix+"/v2/apis/{apiId}", s.GetAPI)
		r.HandleFunc("PATCH", prefix+"/v2/apis/{apiId}", s.UpdateAPI)
		r.HandleFunc("DELETE", prefix+"/v2/apis/{apiId}", s.DeleteAPI)

		// Route routes.
		r.HandleFunc("POST", prefix+"/v2/apis/{apiId}/routes", s.CreateRoute)
		r.HandleFunc("GET", prefix+"/v2/apis/{apiId}/routes", s.GetRoutes)
		r.HandleFunc("GET", prefix+"/v2/apis/{apiId}/routes/{routeId}", s.GetRoute)
		r.HandleFunc("PATCH", prefix+"/v2/apis/{apiId}/routes/{routeId}", s.UpdateRoute)
		r.HandleFunc("DELETE", prefix+"/v2/apis/{apiId}/routes/{routeId}", s.DeleteRoute)

		// Integration routes.
		r.HandleFunc("POST", prefix+"/v2/apis/{apiId}/integrations", s.CreateIntegration)
		r.HandleFunc("GET", prefix+"/v2/apis/{apiId}/integrations", s.GetIntegrations)
		r.HandleFunc("GET", prefix+"/v2/apis/{apiId}/integrations/{integrationId}", s.GetIntegration)
		r.HandleFunc("PATCH", prefix+"/v2/apis/{apiId}/integrations/{integrationId}", s.UpdateIntegration)
		r.HandleFunc("DELETE", prefix+"/v2/apis/{apiId}/integrations/{integrationId}", s.DeleteIntegration)

		// Stage routes.
		r.HandleFunc("POST", prefix+"/v2/apis/{apiId}/stages", s.CreateStage)
		r.HandleFunc("GET", prefix+"/v2/apis/{apiId}/stages", s.GetStages)
		r.HandleFunc("GET", prefix+"/v2/apis/{apiId}/stages/{stageName}", s.GetStage)
		r.HandleFunc("PATCH", prefix+"/v2/apis/{apiId}/stages/{stageName}", s.UpdateStage)
		r.HandleFunc("DELETE", prefix+"/v2/apis/{apiId}/stages/{stageName}", s.DeleteStage)

		// Deployment routes.
		r.HandleFunc("POST", prefix+"/v2/apis/{apiId}/deployments", s.CreateDeployment)
		r.HandleFunc("GET", prefix+"/v2/apis/{apiId}/deployments", s.GetDeployments)
		r.HandleFunc("GET", prefix+"/v2/apis/{apiId}/deployments/{deploymentId}", s.GetDeployment)
		r.HandleFunc("DELETE", prefix+"/v2/apis/{apiId}/deployments/{deploymentId}", s.DeleteDeployment)

		// Tag routes. The resource ARN is captured as a trailing wildcard
		// because it contains ':' and '/' characters.
		r.HandleFunc("GET", prefix+"/v2/tags/{arn...}", s.GetTags)
		r.HandleFunc("POST", prefix+"/v2/tags/{arn...}", s.TagResource)
		r.HandleFunc("DELETE", prefix+"/v2/tags/{arn...}", s.UntagResource)
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
		Display:     "API Gateway v2",
		Category:    "Networking & Content Delivery",
		Description: "API management (HTTP/WebSocket API)",
	}
}
