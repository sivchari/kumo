package cloudcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sivchari/kumo/internal/service/ec2"
)

// awsEC2VPC adapts the AWS::EC2::VPC Cloud Control type to kumo's EC2
// storage. Properties model the CloudFormation surface, but only the
// fields kumo's storage actually persists are honoured today —
// additional CFN properties (Ipv6CidrBlock, EnableDnsHostnames, …) can
// be wired through ModifyVpcAttribute calls in a follow-up without
// changing the wire shape.
type awsEC2VPC struct{}

func init() {
	registerDefaultHandler(&awsEC2VPC{})
}

// vpcProperties is the JSON shape AWS::EC2::VPC uses on the wire. JSON
// tags are PascalCase to match AWS CloudFormation; the Go field names
// stay idiomatic (DNS upper-cased) even though the JSON spelling uses
// "Dns".
type vpcProperties struct {
	VpcID              string `json:"VpcId,omitempty"`
	CidrBlock          string `json:"CidrBlock,omitempty"`
	InstanceTenancy    string `json:"InstanceTenancy,omitempty"`
	EnableDNSHostnames bool   `json:"EnableDnsHostnames,omitempty"`
	EnableDNSSupport   bool   `json:"EnableDnsSupport,omitempty"`
}

func (*awsEC2VPC) TypeName() string { return "AWS::EC2::VPC" }

func (*awsEC2VPC) storage() (ec2.Storage, error) {
	return lookupStorage[ec2.Storage]("ec2")
}

func (h *awsEC2VPC) Create(ctx context.Context, desired []byte) (string, []byte, error) {
	var props vpcProperties
	if err := json.Unmarshal(desired, &props); err != nil {
		return "", nil, fmt.Errorf("invalid AWS::EC2::VPC properties: %w", err)
	}

	if props.CidrBlock == "" {
		return "", nil, errors.New("CidrBlock is required")
	}

	storage, err := h.storage()
	if err != nil {
		return "", nil, err
	}

	vpc, err := storage.CreateVpc(ctx, &ec2.CreateVpcRequest{
		CidrBlock:       props.CidrBlock,
		InstanceTenancy: props.InstanceTenancy,
	})
	if err != nil {
		return "", nil, err
	}

	state, err := vpcStateJSON(vpc)
	if err != nil {
		return "", nil, err
	}

	return vpc.VpcID, state, nil
}

func (h *awsEC2VPC) Read(ctx context.Context, identifier string) ([]byte, error) {
	storage, err := h.storage()
	if err != nil {
		return nil, err
	}

	vpcs, err := storage.DescribeVpcs(ctx, []string{identifier})
	if err != nil {
		return nil, err
	}

	if len(vpcs) == 0 {
		return nil, &NotFoundError{Message: "vpc " + identifier + " does not exist"}
	}

	return vpcStateJSON(vpcs[0])
}

// Update is currently read-back-only. The mutable VPC attributes
// (EnableDnsHostnames, EnableDnsSupport) ride a separate AWS API
// (ModifyVpcAttribute); wiring those through Cloud Control's PatchDocument
// flow is a follow-up.
func (h *awsEC2VPC) Update(ctx context.Context, identifier string, _ []byte) ([]byte, error) {
	return h.Read(ctx, identifier)
}

func (h *awsEC2VPC) Delete(ctx context.Context, identifier string) error {
	storage, err := h.storage()
	if err != nil {
		return err
	}

	vpcs, err := storage.DescribeVpcs(ctx, []string{identifier})
	if err != nil {
		return err
	}

	if len(vpcs) == 0 {
		return &NotFoundError{Message: "vpc " + identifier + " does not exist"}
	}

	return storage.DeleteVpc(ctx, identifier)
}

func (h *awsEC2VPC) List(ctx context.Context) ([]ResourceDescription, error) {
	storage, err := h.storage()
	if err != nil {
		return nil, err
	}

	vpcs, err := storage.DescribeVpcs(ctx, nil)
	if err != nil {
		return nil, err
	}

	out := make([]ResourceDescription, 0, len(vpcs))

	for _, v := range vpcs {
		props, err := vpcStateJSON(v)
		if err != nil {
			return nil, err
		}

		out = append(out, ResourceDescription{Identifier: v.VpcID, Properties: props})
	}

	return out, nil
}

// vpcStateJSON is the shared serialiser for read paths so Create / Read /
// List all produce the same wire shape. The full CloudFormation schema
// is emitted (with null / empty defaults for what kumo doesn't model)
// because terraform-provider-awscc treats every Computed property as
// "must be known after apply".
func vpcStateJSON(v *ec2.Vpc) ([]byte, error) {
	state := map[string]any{
		"VpcId":                 v.VpcID,
		"CidrBlock":             v.CidrBlock,
		"InstanceTenancy":       v.InstanceTenancy,
		"EnableDnsHostnames":    v.EnableDNSHostnames,
		"EnableDnsSupport":      v.EnableDNSSupport,
		"CidrBlockAssociations": []any{},
		"DefaultNetworkAcl":     "",
		"DefaultSecurityGroup":  "",
		"Ipv6CidrBlocks":        []any{},
		"Tags":                  []any{},
	}

	return json.Marshal(state)
}
