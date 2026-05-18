package lambda

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
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
}

// New creates a new Lambda service.
func New(storage Storage, baseURL string) *Service {
	return &Service{
		storage: storage,
		baseURL: baseURL,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "lambda"
}

// RegisterRoutes registers the Lambda routes.
// Routes are registered under both /lambda/... (for SDK BaseEndpoint) and /2015-03-31/... (for CLI).
func (s *Service) RegisterRoutes(r service.Router) {
	for _, prefix := range []string{"/lambda", ""} {
		r.Handle("POST", prefix+"/2015-03-31/functions", s.CreateFunction)
		r.Handle("GET", prefix+"/2015-03-31/functions", s.ListFunctions)
		r.Handle("GET", prefix+"/2015-03-31/functions/{functionName}", s.GetFunction)
		r.Handle("DELETE", prefix+"/2015-03-31/functions/{functionName}", s.DeleteFunction)
		r.Handle("PUT", prefix+"/2015-03-31/functions/{functionName}/code", s.UpdateFunctionCode)
		r.Handle("GET", prefix+"/2015-03-31/functions/{functionName}/configuration", s.GetFunctionConfiguration)
		r.Handle("PUT", prefix+"/2015-03-31/functions/{functionName}/configuration", s.UpdateFunctionConfiguration)
		r.Handle("POST", prefix+"/2015-03-31/functions/{functionName}/invocations", s.Invoke)
		r.Handle("POST", prefix+"/2015-03-31/event-source-mappings", s.CreateEventSourceMapping)
		r.Handle("GET", prefix+"/2015-03-31/event-source-mappings", s.ListEventSourceMappings)
		r.Handle("GET", prefix+"/2015-03-31/event-source-mappings/{uuid}", s.GetEventSourceMapping)
		r.Handle("PUT", prefix+"/2015-03-31/event-source-mappings/{uuid}", s.UpdateEventSourceMapping)
		r.Handle("DELETE", prefix+"/2015-03-31/event-source-mappings/{uuid}", s.DeleteEventSourceMapping)

		// terraform-provider-aws refresh endpoints. Required after
		// CreateFunction; without these the apply errors immediately on
		// the post-create read.
		r.Handle("GET", prefix+"/2015-03-31/functions/{functionName}/versions", s.ListVersionsByFunction)
		r.Handle("GET", prefix+"/2015-03-31/functions/{functionName}/aliases", s.ListAliases)
		r.Handle("GET", prefix+"/2015-03-31/functions/{functionName}/policy", s.GetPolicy)
		r.Handle("POST", prefix+"/2015-03-31/functions/{functionName}/policy", s.AddPermission)
		r.Handle("DELETE", prefix+"/2015-03-31/functions/{functionName}/policy/{statementId}", s.RemovePermission)
		r.Handle("GET", prefix+"/2020-06-30/functions/{functionName}/code-signing-config", s.GetFunctionCodeSigningConfig)
		r.Handle("GET", prefix+"/2019-09-25/functions/{functionName}/event-invoke-config/list", s.ListFunctionEventInvokeConfigs)
		r.Handle("GET", prefix+"/2017-03-31/tags/{arn...}", s.ListTags)
		r.Handle("POST", prefix+"/2017-03-31/tags/{arn...}", s.TagResource)
		r.Handle("DELETE", prefix+"/2017-03-31/tags/{arn...}", s.UntagResource)
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
