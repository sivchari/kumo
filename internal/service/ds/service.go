package ds

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	svc := &Service{
		storage: NewMemoryStorage(opts...),
	}

	service.Register(svc)
}

// Service implements the Directory Service.
type Service struct {
	storage Storage
}

// NewService creates a new Directory Service.
func NewService() *Service {
	return &Service{
		storage: NewMemoryStorage(),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "ds"
}

// RegisterRoutes registers the Directory Service routes.
// Directory Service uses AWS JSON 1.1 protocol with X-Amz-Target header.
func (s *Service) RegisterRoutes(_ service.Router) {
	// Directory Service uses POST / with X-Amz-Target header
	// Routes are handled by the JSON protocol dispatcher.
}

// TargetPrefix returns the X-Amz-Target prefix for this service.
func (s *Service) TargetPrefix() string {
	return "DirectoryService_20150416"
}

// DispatchAction dispatches the action to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := r.Header.Get("X-Amz-Target")

	switch action {
	case "DirectoryService_20150416.CreateDirectory":
		s.CreateDirectory(w, r)
	case "DirectoryService_20150416.DescribeDirectories":
		s.DescribeDirectories(w, r)
	case "DirectoryService_20150416.DeleteDirectory":
		s.DeleteDirectory(w, r)
	case "DirectoryService_20150416.CreateSnapshot":
		s.CreateSnapshot(w, r)
	case "DirectoryService_20150416.DescribeSnapshots":
		s.DescribeSnapshots(w, r)
	case "DirectoryService_20150416.DeleteSnapshot":
		s.DeleteSnapshot(w, r)
	default:
		writeDSError(w, ErrUnsupportedOperation, "Unsupported operation: "+action, http.StatusBadRequest)
	}
}

// JSONProtocol is a marker method for JSON protocol services.
func (s *Service) JSONProtocol() {}

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
		Display:     "Directory Service",
		Category:    "Other Services",
		Description: "Microsoft AD",
	}
}
