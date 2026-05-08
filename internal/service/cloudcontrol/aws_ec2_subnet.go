package cloudcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sivchari/kumo/internal/service/ec2"
)

// awsEC2Subnet adapts AWS::EC2::Subnet to the EC2 storage. Like
// awsEC2VPC, only the fields kumo's storage actually persists are
// honoured today; the rest can be added without changing the wire shape.
type awsEC2Subnet struct{}

func init() {
	registerDefaultHandler(&awsEC2Subnet{})
}

// subnetProperties is the JSON shape AWS::EC2::Subnet uses on the wire.
type subnetProperties struct {
	SubnetID            string `json:"SubnetId,omitempty"`
	VpcID               string `json:"VpcId,omitempty"`
	CidrBlock           string `json:"CidrBlock,omitempty"`
	AvailabilityZone    string `json:"AvailabilityZone,omitempty"`
	MapPublicIpOnLaunch bool   `json:"MapPublicIpOnLaunch,omitempty"`
}

func (*awsEC2Subnet) TypeName() string { return "AWS::EC2::Subnet" }

func (*awsEC2Subnet) storage() (ec2.Storage, error) {
	return lookupStorage[ec2.Storage]("ec2")
}

func (h *awsEC2Subnet) Create(ctx context.Context, desired []byte) (string, []byte, error) {
	var props subnetProperties
	if err := json.Unmarshal(desired, &props); err != nil {
		return "", nil, fmt.Errorf("invalid AWS::EC2::Subnet properties: %w", err)
	}

	if props.VpcID == "" {
		return "", nil, errors.New("VpcId is required")
	}

	if props.CidrBlock == "" {
		return "", nil, errors.New("CidrBlock is required")
	}

	storage, err := h.storage()
	if err != nil {
		return "", nil, err
	}

	subnet, err := storage.CreateSubnet(ctx, &ec2.CreateSubnetRequest{
		VpcID:            props.VpcID,
		CidrBlock:        props.CidrBlock,
		AvailabilityZone: props.AvailabilityZone,
	})
	if err != nil {
		return "", nil, err
	}

	state, err := subnetStateJSON(subnet)
	if err != nil {
		return "", nil, err
	}

	return subnet.SubnetID, state, nil
}

func (h *awsEC2Subnet) Read(ctx context.Context, identifier string) ([]byte, error) {
	storage, err := h.storage()
	if err != nil {
		return nil, err
	}

	subnets, err := storage.DescribeSubnets(ctx, []string{identifier}, nil)
	if err != nil {
		return nil, err
	}

	if len(subnets) == 0 {
		return nil, &NotFoundError{Message: "subnet " + identifier + " does not exist"}
	}

	return subnetStateJSON(subnets[0])
}

func (h *awsEC2Subnet) Update(ctx context.Context, identifier string, _ []byte) ([]byte, error) {
	return h.Read(ctx, identifier)
}

func (h *awsEC2Subnet) Delete(ctx context.Context, identifier string) error {
	storage, err := h.storage()
	if err != nil {
		return err
	}

	subnets, err := storage.DescribeSubnets(ctx, []string{identifier}, nil)
	if err != nil {
		return err
	}

	if len(subnets) == 0 {
		return &NotFoundError{Message: "subnet " + identifier + " does not exist"}
	}

	return storage.DeleteSubnet(ctx, identifier)
}

func (h *awsEC2Subnet) List(ctx context.Context) ([]ResourceDescription, error) {
	storage, err := h.storage()
	if err != nil {
		return nil, err
	}

	subnets, err := storage.DescribeSubnets(ctx, nil, nil)
	if err != nil {
		return nil, err
	}

	out := make([]ResourceDescription, 0, len(subnets))

	for _, s := range subnets {
		props, err := subnetStateJSON(s)
		if err != nil {
			return nil, err
		}

		out = append(out, ResourceDescription{Identifier: s.SubnetID, Properties: props})
	}

	return out, nil
}

// subnetStateJSON serialises a Subnet for read responses. The full
// CloudFormation schema is emitted (with null / empty defaults for what
// kumo doesn't model) because terraform-provider-awscc requires every
// Computed property to be resolved after apply.
func subnetStateJSON(s *ec2.Subnet) ([]byte, error) {
	state := map[string]any{
		"SubnetId":                      s.SubnetID,
		"VpcId":                         s.VpcID,
		"CidrBlock":                     s.CidrBlock,
		"AvailabilityZone":              s.AvailabilityZone,
		"AvailabilityZoneId":            "",
		"AvailableIpAddressCount":       s.AvailableIPAddressCount,
		"AssignIpv6AddressOnCreation":   false,
		"EnableDns64":                   false,
		"Ipv4IpamPoolId":                nil,
		"Ipv4NetmaskLength":             nil,
		"Ipv6CidrBlock":                 nil,
		"Ipv6CidrBlocks":                []any{},
		"Ipv6IpamPoolId":                nil,
		"Ipv6Native":                    false,
		"Ipv6NetmaskLength":             nil,
		"MapPublicIpOnLaunch":           s.MapPublicIPOnLaunch,
		"NetworkAclAssociationId":       "",
		"OutpostArn":                    nil,
		"PrivateDnsNameOptionsOnLaunch": nil,
		"Tags":                          []any{},
	}

	return json.Marshal(state)
}
