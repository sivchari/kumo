// Package cloudcontrol provides AWS Cloud Control API emulation. Cloud
// Control is AWS's unified CRUD interface that exposes any
// CloudFormation-modeled resource type through the same six operations
// (Create / Read / Update / Delete / List / status polling). Implementing
// it lets clients that target Cloud Control — most notably the
// terraform-provider-awscc — drive a kumo-modeled resource without
// per-service handler implementations on the kumo side.
package cloudcontrol

import (
	"io"
	"net/http"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	service.Register(New(defaultRegistry()))
}

// Service implements the Cloud Control API service. It dispatches each
// operation to a per-resource-type Handler registered in the type registry.
type Service struct {
	registry *Registry
	progress *progressTracker
}

// New creates a new Cloud Control service backed by the given registry.
func New(reg *Registry) *Service {
	return &Service{registry: reg, progress: newProgressTracker()}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "cloudcontrol"
}

// RegisterRoutes is a no-op — Cloud Control uses AWS JSON 1.0 over a single
// POST endpoint, dispatched via the X-Amz-Target header.
func (s *Service) RegisterRoutes(_ service.Router) {}

// TargetPrefix returns the X-Amz-Target prefix the SDK uses for Cloud
// Control: every operation's target is "CloudApiService.<Action>".
func (s *Service) TargetPrefix() string {
	return "CloudApiService"
}

// JSONProtocol marks this service as using AWS JSON 1.0.
func (s *Service) JSONProtocol() {}

// Close is a no-op — the registry holds no closable state of its own.
func (s *Service) Close() error { return nil }

// DispatchAction is invoked by the JSON protocol dispatcher after it
// confirms the X-Amz-Target prefix matches "CloudApiService". The action
// name is the part after the dot.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")

	action := target
	if idx := strings.LastIndex(target, "."); idx >= 0 {
		action = target[idx+1:]
	}

	switch action {
	case "CreateResource":
		s.CreateResource(w, r)
	case "GetResource":
		s.GetResource(w, r)
	case "UpdateResource":
		s.UpdateResource(w, r)
	case "DeleteResource":
		s.DeleteResource(w, r)
	case "ListResources":
		s.ListResources(w, r)
	case "GetResourceRequestStatus":
		s.GetResourceRequestStatus(w, r)
	default:
		writeError(w, "InvalidAction", "Action "+action+" is not supported")
	}
}
