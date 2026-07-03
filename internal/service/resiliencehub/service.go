package resiliencehub

import (
	"fmt"
	"io"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check to ensure Service implements service.Service.
var _ service.Service = (*Service)(nil)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Service implements the Resilience Hub service.
type Service struct {
	storage Storage
}

// New creates a new Resilience Hub service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "resiliencehub"
}

// RegisterRoutes registers the Resilience Hub routes.
// Resilience Hub uses REST API with POST methods for all operations.
// Some operations also support GET for SDK compatibility.
func (s *Service) RegisterRoutes(r service.Router) {
	// App operations
	r.Handle("POST", "/create-app", s.CreateApp)
	r.Handle("POST", "/describe-app", s.DescribeApp)
	r.Handle("POST", "/update-app", s.UpdateApp)
	r.Handle("POST", "/delete-app", s.DeleteApp)
	r.Handle("POST", "/list-apps", s.ListApps)
	r.Handle("GET", "/list-apps", s.ListApps)

	// ResiliencyPolicy operations
	r.Handle("POST", "/create-resiliency-policy", s.CreateResiliencyPolicy)
	r.Handle("POST", "/describe-resiliency-policy", s.DescribeResiliencyPolicy)
	r.Handle("POST", "/update-resiliency-policy", s.UpdateResiliencyPolicy)
	r.Handle("POST", "/delete-resiliency-policy", s.DeleteResiliencyPolicy)
	r.Handle("POST", "/list-resiliency-policies", s.ListResiliencyPolicies)
	r.Handle("GET", "/list-resiliency-policies", s.ListResiliencyPolicies)

	// Assessment operations
	r.Handle("POST", "/start-app-assessment", s.StartAppAssessment)
	r.Handle("POST", "/describe-app-assessment", s.DescribeAppAssessment)
	r.Handle("POST", "/delete-app-assessment", s.DeleteAppAssessment)
	r.Handle("POST", "/list-app-assessments", s.ListAppAssessments)
	r.Handle("GET", "/list-app-assessments", s.ListAppAssessments)

	// Tag operations
	r.Handle("POST", "/tag-resource", s.TagResource)
	r.Handle("POST", "/untag-resource", s.UntagResource)
	r.Handle("POST", "/list-tags-for-resource", s.ListTagsForResource)
	r.Handle("GET", "/list-tags-for-resource", s.ListTagsForResource)
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
		Display:     "Resilience Hub",
		Category:    "Other Services",
		Description: "Application resilience",
	}
}
