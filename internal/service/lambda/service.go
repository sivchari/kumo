package lambda

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/service/execapi"
)

const defaultBaseURL = "http://localhost:4566"

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(defaultBaseURL, opts...), defaultBaseURL))
}

// Service implements the Lambda service.
type Service struct {
	storage Storage
	baseURL string
	// invokeBaseURL is the kumo endpoint used for in-process self-calls
	// (function URL -> Invoke); it follows KUMO_HOST / KUMO_PORT like execapi.
	invokeBaseURL string
	broker        *runtimeBroker
	async         *asyncDispatcher
}

// New creates a new Lambda service.
func New(storage Storage, baseURL string) *Service {
	return &Service{
		storage:       storage,
		baseURL:       baseURL,
		invokeBaseURL: execapi.ResolveBaseURL(),
		broker:        newRuntimeBroker(),
		async:         newAsyncDispatcher(),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "lambda"
}

// BaseURL returns the kumo server base URL the service self-calls against.
func (s *Service) BaseURL() string { return s.baseURL }

// RegisterRoutes registers the AWS-faithful Lambda routes and the
// kumo-native Runtime API.
func (s *Service) RegisterRoutes(r service.Router) {
	s.registerAPIRoutes(r)

	// kumo-native Lambda Runtime API. A handler built with lambda.Start
	// connects here with AWS_LAMBDA_RUNTIME_API=<host>/_runtime/{functionName},
	// so an unmodified binary runs against kumo without external RIE.
	r.Handle("GET", "/_runtime/{functionName}/2018-06-01/runtime/invocation/next", s.RuntimeNext)
	r.Handle("POST", "/_runtime/{functionName}/2018-06-01/runtime/invocation/{requestId}/response", s.RuntimeResponse)
	r.Handle("POST", "/_runtime/{functionName}/2018-06-01/runtime/invocation/{requestId}/error", s.RuntimeError)
	r.Handle("POST", "/_runtime/{functionName}/2018-06-01/runtime/init/error", s.RuntimeInitError)
}

// SigningName returns the SigV4 credential scope service name for Lambda.
func (s *Service) SigningName() string {
	return "lambda"
}

// registerAPIRoutes registers the Lambda API routes.
func (s *Service) registerAPIRoutes(r service.Router) {
	r.Handle("POST", "/2015-03-31/functions", s.CreateFunction)
	r.Handle("GET", "/2015-03-31/functions", s.ListFunctions)
	r.Handle("GET", "/2015-03-31/functions/{functionName}", s.GetFunction)
	r.Handle("DELETE", "/2015-03-31/functions/{functionName}", s.DeleteFunction)
	r.Handle("PUT", "/2015-03-31/functions/{functionName}/code", s.UpdateFunctionCode)
	r.Handle("GET", "/2015-03-31/functions/{functionName}/configuration", s.GetFunctionConfiguration)
	r.Handle("PUT", "/2015-03-31/functions/{functionName}/configuration", s.UpdateFunctionConfiguration)
	r.Handle("POST", "/2015-03-31/functions/{functionName}/invocations", s.Invoke)
	r.Handle("POST", "/2015-03-31/event-source-mappings", s.CreateEventSourceMapping)
	r.Handle("GET", "/2015-03-31/event-source-mappings", s.ListEventSourceMappings)
	r.Handle("GET", "/2015-03-31/event-source-mappings/{uuid}", s.GetEventSourceMapping)
	r.Handle("PUT", "/2015-03-31/event-source-mappings/{uuid}", s.UpdateEventSourceMapping)
	r.Handle("DELETE", "/2015-03-31/event-source-mappings/{uuid}", s.DeleteEventSourceMapping)

	// terraform-provider-aws refresh endpoints. Required after
	// CreateFunction; without these the apply errors immediately on
	// the post-create read.
	r.Handle("GET", "/2015-03-31/functions/{functionName}/versions", s.ListVersionsByFunction)
	r.Handle("GET", "/2015-03-31/functions/{functionName}/aliases", s.ListAliases)
	r.Handle("GET", "/2015-03-31/functions/{functionName}/policy", s.GetPolicy)
	r.Handle("POST", "/2015-03-31/functions/{functionName}/policy", s.AddPermission)
	r.Handle("DELETE", "/2015-03-31/functions/{functionName}/policy/{statementId}", s.RemovePermission)
	r.Handle("GET", "/2020-06-30/functions/{functionName}/code-signing-config", s.GetFunctionCodeSigningConfig)
	r.Handle("GET", "/2019-09-25/functions/{functionName}/event-invoke-config/list", s.ListFunctionEventInvokeConfigs)
	r.Handle("GET", "/2017-03-31/tags/{arn...}", s.ListTags)
	r.Handle("POST", "/2017-03-31/tags/{arn...}", s.TagResource)
	r.Handle("DELETE", "/2017-03-31/tags/{arn...}", s.UntagResource)

	s.registerFunctionURLRoutes(r)
}

// Close stops the async dispatcher and saves the storage state if
// persistence is enabled.
func (s *Service) Close() error {
	s.async.close()

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
		Display:     "Lambda",
		Category:    "Compute",
		Description: "Serverless functions",
	}
}
