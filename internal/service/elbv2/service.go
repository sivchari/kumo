package elbv2

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

	service.Register(New(NewMemoryStorage(opts...)))
}

// Service implements the ELB v2 service.
type Service struct {
	storage Storage
}

// New creates a new ELB v2 service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "elasticloadbalancingv2"
}

// RegisterRoutes registers routes with the router.
// ELB uses Query protocol, so routes are registered via DispatchAction.
func (s *Service) RegisterRoutes(_ service.Router) {
	// ELB uses Query protocol, routing is handled by DispatchAction.
}

// TargetPrefix returns the X-Amz-Target header prefix for ELB.
func (s *Service) TargetPrefix() string {
	return "ElasticLoadBalancing"
}

// Actions returns the list of action names this service handles.
func (s *Service) Actions() []string {
	return []string{
		"CreateLoadBalancer",
		"DeleteLoadBalancer",
		"DescribeLoadBalancers",
		"CreateTargetGroup",
		"DeleteTargetGroup",
		"DescribeTargetGroups",
		"RegisterTargets",
		"DeregisterTargets",
		"CreateListener",
		"DeleteListener",
		"CreateRule",
		"DescribeRules",
		"ModifyRule",
		"DeleteRule",
		"SetRulePriorities",
		"ModifyLoadBalancerAttributes",
		"DescribeLoadBalancerAttributes",
		"ModifyTargetGroupAttributes",
		"DescribeTargetGroupAttributes",
		"DescribeTargetHealth",
		"ModifyListener",
		"DescribeListeners",
		"DescribeTags",
		"AddTags",
		"RemoveTags",
		"DescribeCapacityReservation",
		"DescribeListenerAttributes",
		"ModifyListenerAttributes",
	}
}

// ServiceIdentifier returns the SDK service identifier for User-Agent disambiguation.
func (s *Service) ServiceIdentifier() string {
	return "elasticloadbalancingv2"
}

// QueryProtocol is a marker method that indicates ELB uses AWS Query protocol.
func (s *Service) QueryProtocol() {}

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
		Display:     "ELBv2",
		Category:    "Networking & Content Delivery",
		Description: "Load balancing",
	}
}
