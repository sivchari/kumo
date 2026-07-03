package ssm

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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

// Service implements the SSM Parameter Store service.
type Service struct {
	storage Storage
}

// New creates a new SSM service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "ssm"
}

// RegisterRoutes registers the SSM routes.
// Note: SSM uses AWS JSON 1.1 protocol via the JSONProtocolService interface,
// so no direct routes are registered here.
func (s *Service) RegisterRoutes(_ service.Router) {
	// No routes to register - SSM uses JSON protocol dispatcher
}

// TargetPrefix returns the X-Amz-Target header prefix for SSM.
func (s *Service) TargetPrefix() string {
	return "AmazonSSM"
}

// JSONProtocol is a marker method that indicates SSM uses AWS JSON 1.1 protocol.
func (s *Service) JSONProtocol() {}

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
// This method implements the JSONProtocolService interface.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AmazonSSM.")

	switch action {
	case "PutParameter":
		s.PutParameter(w, r)
	case "GetParameter":
		s.GetParameter(w, r)
	case "GetParameters":
		s.GetParameters(w, r)
	case "GetParametersByPath":
		s.GetParametersByPath(w, r)
	case "DeleteParameter":
		s.DeleteParameter(w, r)
	case "DeleteParameters":
		s.DeleteParameters(w, r)
	case "DescribeParameters":
		s.DescribeParameters(w, r)
	case "ListTagsForResource":
		s.ListTagsForResource(w, r)
	case "AddTagsToResource":
		s.AddTagsToResource(w, r)
	case "RemoveTagsFromResource":
		s.RemoveTagsFromResource(w, r)
	default:
		writeSSMError(w, ErrInvalidParameterValue, "The action "+action+" is not valid", http.StatusBadRequest)
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
		Display:     "SSM",
		Category:    "Management & Configuration",
		Description: "Systems Manager",
	}
}
