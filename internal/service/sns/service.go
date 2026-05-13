package sns

import (
	"fmt"
	"io"
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

	storage := NewMemoryStorage("", opts...)
	service.Register(New(storage))
}

// Service implements the SNS service.
type Service struct {
	storage Storage
}

// New creates a new SNS service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "sns"
}

// RegisterRoutes registers routes with the router.
// SNS uses Query protocol, so routes are registered via DispatchAction.
func (s *Service) RegisterRoutes(_ service.Router) {
	// SNS uses Query protocol, routing is handled by DispatchAction.
}

// TargetPrefix returns the X-Amz-Target header prefix for SNS.
func (s *Service) TargetPrefix() string {
	return "AmazonSimpleNotificationService"
}

// Actions returns the list of action names this service handles.
func (s *Service) Actions() []string {
	return []string{
		"CreateTopic",
		"DeleteTopic",
		"ListTopics",
		"Subscribe",
		"Unsubscribe",
		"Publish",
		"ListSubscriptions",
		"ListSubscriptionsByTopic",
		"GetSubscriptionAttributes",
		"GetTopicAttributes",
		"SetTopicAttributes",
		"ListTagsForResource",
		"TagResource",
		"UntagResource",
	}
}

// ServiceIdentifier returns the SDK service identifier for User-Agent disambiguation.
func (s *Service) ServiceIdentifier() string {
	return "sns"
}

// QueryProtocol is a marker method that indicates SNS uses AWS Query protocol.
func (s *Service) QueryProtocol() {}

// Storage returns the SNS storage.
// This can be used to set up cross-service integration (e.g., SNS to SQS).
func (s *Service) Storage() Storage {
	return s.storage
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
