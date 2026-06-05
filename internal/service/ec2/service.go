package ec2

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

// Service implements the EC2 service.
type Service struct {
	storage Storage
}

// New creates a new EC2 service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "ec2"
}

// Storage exposes the underlying storage so other services that need to
// operate on the same EC2 store (notably the cloudcontrol service, which
// proxies AWS::EC2::* through the existing EC2 storage) can read and
// mutate it without going back through HTTP.
func (s *Service) Storage() Storage {
	return s.storage
}

// Prefix returns the URL prefix for the service.

// RegisterRoutes registers routes with the router.
// EC2 uses Query protocol, so routes are registered via DispatchAction.
func (s *Service) RegisterRoutes(_ service.Router) {
	// EC2 uses Query protocol, routing is handled by DispatchAction.
}

// TargetPrefix returns the X-Amz-Target header prefix for EC2.
// EC2 uses Action parameter instead of X-Amz-Target header.
func (s *Service) TargetPrefix() string {
	return "AmazonEC2"
}

// Actions returns the list of action names this service handles.
func (s *Service) Actions() []string {
	return []string{
		// Instance operations
		"RunInstances",
		"TerminateInstances",
		"DescribeInstances",
		"StartInstances",
		"StopInstances",
		// Security Group operations
		"CreateSecurityGroup",
		"DeleteSecurityGroup",
		"AuthorizeSecurityGroupIngress",
		"AuthorizeSecurityGroupEgress",
		"DescribeSecurityGroups",
		"RevokeSecurityGroupIngress",
		"RevokeSecurityGroupEgress",
		// Key Pair operations
		"CreateKeyPair",
		"DeleteKeyPair",
		"DescribeKeyPairs",
		// VPC operations
		"CreateVpc",
		"DeleteVpc",
		"DescribeVpcs",
		// Subnet operations
		"CreateSubnet",
		"DeleteSubnet",
		"DescribeSubnets",
		// Internet Gateway operations
		"CreateInternetGateway",
		"AttachInternetGateway",
		"DetachInternetGateway",
		"DeleteInternetGateway",
		"DescribeInternetGateways",
		// Network interface — destroy-time stub
		"DescribeNetworkInterfaces",
		// Route Table operations
		"CreateRouteTable",
		"CreateRoute",
		"AssociateRouteTable",
		"DescribeRouteTables",
		// NAT Gateway operations
		"CreateNatGateway",
		"DescribeNatGateways",
		// Tag operations
		"CreateTags",
		"DeleteTags",
		"DescribeTags",
		// VPC / Subnet attribute mutation
		"ModifyVpcAttribute",
		"DescribeVpcAttribute",
		"ModifySubnetAttribute",
	}
}

// ServiceIdentifier returns the SDK service identifier for User-Agent disambiguation.
func (s *Service) ServiceIdentifier() string {
	return "ec2"
}

// QueryProtocol is a marker method that indicates EC2 uses AWS Query protocol.
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
		Display:     "EC2",
		Category:    "Compute",
		Description: "Virtual machines",
	}
}
