package amp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// backendEnvVar is the env var pointing at the real local Prometheus
// (or compatible TSDB) that data-plane requests are forwarded to.
const backendEnvVar = "KUMO_AMP_BACKEND"

// Compile-time check.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...), os.Getenv(backendEnvVar)))
}

// Service implements the AMP service.
type Service struct {
	storage        Storage
	backend        string // host:port of the real Prometheus to forward data-plane traffic to
	backendErr     error
	dataPlaneProxy *httputil.ReverseProxy
}

// New creates a service. Pass an empty backend to disable the data
// plane (control plane still works).
func New(storage Storage, backend string) *Service {
	s := &Service{storage: storage, backend: backend}
	if backend == "" {
		return s
	}

	target, err := backendURL(backend)
	if err != nil {
		s.backendErr = err

		return s
	}

	s.dataPlaneProxy = newDataPlaneProxy(target)

	return s
}

// Name is the kumo-internal service name.
func (s *Service) Name() string { return "amp" }

// RegisterRoutes wires the AMP REST routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// Control plane.
	r.HandleFunc("POST", "/workspaces", s.CreateWorkspace)
	r.HandleFunc("GET", "/workspaces", s.ListWorkspaces)
	r.HandleFunc("GET", "/workspaces/{workspaceId}", s.DescribeWorkspace)
	r.HandleFunc("DELETE", "/workspaces/{workspaceId}", s.DeleteWorkspace)

	// Data plane - proxied to KUMO_AMP_BACKEND when set.
	r.HandleFunc("POST", "/workspaces/{workspaceId}/api/v1/remote_write", s.RemoteWrite)
	r.HandleFunc("GET", "/workspaces/{workspaceId}/api/v1/query", s.QueryInstant)
	r.HandleFunc("POST", "/workspaces/{workspaceId}/api/v1/query", s.QueryInstant)
	r.HandleFunc("GET", "/workspaces/{workspaceId}/api/v1/query_range", s.QueryRange)
	r.HandleFunc("POST", "/workspaces/{workspaceId}/api/v1/query_range", s.QueryRange)
}

// Close persists state.
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
		Display:     "Amazon Managed Service for Prometheus",
		Category:    "Monitoring & Logging",
		Description: "Managed Prometheus workspaces with data-plane passthrough",
	}
}

type backendPathContextKey struct{}

func withBackendPath(r *http.Request, backendPath string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), backendPathContextKey{}, backendPath))
}

func backendURL(backend string) (*url.URL, error) {
	if strings.Contains(backend, "://") {
		return nil, fmt.Errorf("%s must be host:port without scheme", backendEnvVar)
	}

	target, err := url.Parse("http://" + backend)
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid host:port: %w", backendEnvVar, err)
	}

	if target.Host == "" {
		return nil, fmt.Errorf("%s must be host:port without scheme", backendEnvVar)
	}

	if target.Path != "" && target.Path != "/" {
		return nil, fmt.Errorf("%s must not include a path", backendEnvVar)
	}

	return target, nil
}

func newDataPlaneProxy(target *url.URL) *httputil.ReverseProxy {
	rp := httputil.NewSingleHostReverseProxy(target)
	originalDirector := rp.Director

	rp.Director = func(req *http.Request) {
		originalDirector(req)

		backendPath, _ := req.Context().Value(backendPathContextKey{}).(string)
		if backendPath != "" {
			req.URL.Path = strings.TrimRight(target.Path, "/") + backendPath
		}

		req.Host = target.Host
	}

	return rp
}
