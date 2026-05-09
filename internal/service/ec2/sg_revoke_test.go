package ec2

import (
	"context"
	"errors"
	"testing"
)

// TestRevokeSecurityGroupIngress_RemovesMatchingCIDR exercises the
// per-CIDR revoke semantics AWS implements: revoking
// `{tcp, 22, 22, [0.0.0.0/0]}` from a rule that authorized
// `{tcp, 22, 22, [0.0.0.0/0, 10.0.0.0/8]}` leaves only the
// `10.0.0.0/8` range. terraform aws_security_group relies on this when
// it re-applies a changed cidr_blocks list.
func TestRevokeSecurityGroupIngress_RemovesMatchingCIDR(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()

	vpc, err := store.CreateVpc(ctx, &CreateVpcRequest{CidrBlock: "10.0.0.0/16"})
	if err != nil {
		t.Fatalf("CreateVpc: %v", err)
	}

	sg, err := store.CreateSecurityGroup(ctx, &CreateSecurityGroupRequest{
		GroupName:        "revoke-test",
		GroupDescription: "revoke-test",
		VpcID:            vpc.VpcID,
	})
	if err != nil {
		t.Fatalf("CreateSecurityGroup: %v", err)
	}

	if err := store.AuthorizeSecurityGroupIngress(ctx, sg.GroupID, "", []IPPermission{{
		IPProtocol: "tcp",
		FromPort:   22,
		ToPort:     22,
		IPRanges: []IPRange{
			{CidrIP: "0.0.0.0/0"},
			{CidrIP: "10.0.0.0/8"},
		},
	}}); err != nil {
		t.Fatalf("AuthorizeSecurityGroupIngress: %v", err)
	}

	if err := store.RevokeSecurityGroupIngress(ctx, sg.GroupID, "", []IPPermission{{
		IPProtocol: "tcp",
		FromPort:   22,
		ToPort:     22,
		IPRanges:   []IPRange{{CidrIP: "0.0.0.0/0"}},
	}}); err != nil {
		t.Fatalf("RevokeSecurityGroupIngress: %v", err)
	}

	got, err := store.DescribeSecurityGroups(ctx, []string{sg.GroupID}, nil)
	if err != nil || len(got) != 1 {
		t.Fatalf("DescribeSecurityGroups: err=%v len=%d", err, len(got))
	}

	if len(got[0].IngressRules) != 1 {
		t.Fatalf("expected 1 surviving rule, got %d: %+v", len(got[0].IngressRules), got[0].IngressRules)
	}

	if len(got[0].IngressRules[0].IPRanges) != 1 || got[0].IngressRules[0].IPRanges[0].CidrIP != "10.0.0.0/8" {
		t.Fatalf("expected only 10.0.0.0/8 to survive, got %+v", got[0].IngressRules[0].IPRanges)
	}
}

// TestRevokeSecurityGroupIngress_DropsRuleWhenAllCIDRsRemoved verifies
// that revoking the only CIDR on a rule deletes the whole rule entry,
// not just empties its IPRanges. AWS shows the SG with no entries for
// that proto/port combo afterwards.
func TestRevokeSecurityGroupIngress_DropsRuleWhenAllCIDRsRemoved(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewMemoryStorage()

	vpc, _ := store.CreateVpc(ctx, &CreateVpcRequest{CidrBlock: "10.0.0.0/16"})
	sg, _ := store.CreateSecurityGroup(ctx, &CreateSecurityGroupRequest{
		GroupName:        "drop-test",
		GroupDescription: "drop-test",
		VpcID:            vpc.VpcID,
	})

	_ = store.AuthorizeSecurityGroupIngress(ctx, sg.GroupID, "", []IPPermission{{
		IPProtocol: "tcp", FromPort: 22, ToPort: 22,
		IPRanges: []IPRange{{CidrIP: "0.0.0.0/0"}},
	}})

	if err := store.RevokeSecurityGroupIngress(ctx, sg.GroupID, "", []IPPermission{{
		IPProtocol: "tcp", FromPort: 22, ToPort: 22,
		IPRanges: []IPRange{{CidrIP: "0.0.0.0/0"}},
	}}); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	got, _ := store.DescribeSecurityGroups(ctx, []string{sg.GroupID}, nil)
	if len(got[0].IngressRules) != 0 {
		t.Fatalf("expected 0 rules after revoking only CIDR, got %d: %+v",
			len(got[0].IngressRules), got[0].IngressRules)
	}
}

// TestRevokeSecurityGroupEgress mirrors the ingress drop case.
func TestRevokeSecurityGroupEgress(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewMemoryStorage()

	vpc, _ := store.CreateVpc(ctx, &CreateVpcRequest{CidrBlock: "10.0.0.0/16"})
	sg, _ := store.CreateSecurityGroup(ctx, &CreateSecurityGroupRequest{
		GroupName:        "egress-test",
		GroupDescription: "egress-test",
		VpcID:            vpc.VpcID,
	})

	_ = store.AuthorizeSecurityGroupEgress(ctx, sg.GroupID, []IPPermission{{
		IPProtocol: "-1", FromPort: 0, ToPort: 0,
		IPRanges: []IPRange{{CidrIP: "0.0.0.0/0"}},
	}})

	if err := store.RevokeSecurityGroupEgress(ctx, sg.GroupID, []IPPermission{{
		IPProtocol: "-1", FromPort: 0, ToPort: 0,
		IPRanges: []IPRange{{CidrIP: "0.0.0.0/0"}},
	}}); err != nil {
		t.Fatalf("revoke egress: %v", err)
	}

	got, _ := store.DescribeSecurityGroups(ctx, []string{sg.GroupID}, nil)
	if len(got[0].EgressRules) != 0 {
		t.Fatalf("expected 0 egress rules after revoke, got %d", len(got[0].EgressRules))
	}
}

// TestRevokeSecurityGroup_NotFound returns the AWS-style error code
// when the SG doesn't exist. terraform doesn't currently rely on the
// distinction (it surfaces any 4xx), but staying close to AWS keeps
// the door open for stricter consumers.
func TestRevokeSecurityGroup_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewMemoryStorage()

	err := store.RevokeSecurityGroupIngress(ctx, "sg-doesnotexist", "", nil)
	if err == nil {
		t.Fatal("expected error revoking from missing SG")
	}

	var ec2Err *Error
	if !errors.As(err, &ec2Err) || ec2Err.Code != "InvalidGroup.NotFound" {
		t.Fatalf("expected InvalidGroup.NotFound, got %v", err)
	}
}
