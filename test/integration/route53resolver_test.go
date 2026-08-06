//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver/types"
	"github.com/sivchari/golden"
)

func newRoute53ResolverClient(t *testing.T) *route53resolver.Client {
	t.Helper()

	return route53resolver.NewFromConfig(awsConfig(t), func(o *route53resolver.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestRoute53Resolver_ResolverEndpoint(t *testing.T) {
	client := newRoute53ResolverClient(t)
	ctx := t.Context()

	// Create resolver endpoint.
	createOutput, err := client.CreateResolverEndpoint(ctx, &route53resolver.CreateResolverEndpointInput{
		CreatorRequestId: aws.String("test-request-id-1"),
		Name:             aws.String("test-endpoint"),
		SecurityGroupIds: []string{"sg-12345678"},
		Direction:        types.ResolverEndpointDirectionInbound,
		IpAddresses: []types.IpAddressRequest{
			{
				SubnetId: aws.String("subnet-12345678"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	endpointID := *createOutput.ResolverEndpoint.Id

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "Id", "Arn", "CreationTime", "ModificationTime", "HostVPCId"),
	)
	g.Assert("create", createOutput)

	// Get resolver endpoint.
	getOutput, err := client.GetResolverEndpoint(ctx, &route53resolver.GetResolverEndpointInput{
		ResolverEndpointId: aws.String(endpointID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "Id", "Arn", "CreationTime", "ModificationTime", "HostVPCId"),
	)
	g2.Assert("get", getOutput)

	// List resolver endpoints.
	listOutput, err := client.ListResolverEndpoints(ctx, &route53resolver.ListResolverEndpointsInput{})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "Id", "Arn", "CreationTime", "ModificationTime", "HostVPCId"),
	)
	g3.Assert("list", listOutput)

	// Delete resolver endpoint.
	_, err = client.DeleteResolverEndpoint(ctx, &route53resolver.DeleteResolverEndpointInput{
		ResolverEndpointId: aws.String(endpointID),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's deleted - should return error.
	_, err = client.GetResolverEndpoint(ctx, &route53resolver.GetResolverEndpointInput{
		ResolverEndpointId: aws.String(endpointID),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRoute53Resolver_ResolverRule(t *testing.T) {
	client := newRoute53ResolverClient(t)
	ctx := t.Context()

	// Create resolver rule.
	createOutput, err := client.CreateResolverRule(ctx, &route53resolver.CreateResolverRuleInput{
		CreatorRequestId: aws.String("test-rule-request-id"),
		Name:             aws.String("test-rule"),
		RuleType:         types.RuleTypeOptionForward,
		DomainName:       aws.String("example.com."),
		TargetIps: []types.TargetAddress{
			{
				Ip:   aws.String("10.0.0.1"),
				Port: aws.Int32(53),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ruleID := *createOutput.ResolverRule.Id

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "Id", "Arn", "CreationTime", "ModificationTime", "OwnerId"),
	)
	g.Assert("create", createOutput)

	// Get resolver rule.
	getOutput, err := client.GetResolverRule(ctx, &route53resolver.GetResolverRuleInput{
		ResolverRuleId: aws.String(ruleID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "Id", "Arn", "CreationTime", "ModificationTime", "OwnerId"),
	)
	g2.Assert("get", getOutput)

	// Associate resolver rule with VPC.
	assocOutput, err := client.AssociateResolverRule(ctx, &route53resolver.AssociateResolverRuleInput{
		ResolverRuleId: aws.String(ruleID),
		VPCId:          aws.String("vpc-12345678"),
		Name:           aws.String("test-association"),
	})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "Id", "ResolverRuleId"),
	)
	g3.Assert("associate", assocOutput)

	// List resolver rule associations.
	listAssocOutput, err := client.ListResolverRuleAssociations(ctx, &route53resolver.ListResolverRuleAssociationsInput{})
	if err != nil {
		t.Fatal(err)
	}

	g4 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "Id", "ResolverRuleId"),
	)
	g4.Assert("list_associations", listAssocOutput)

	// Disassociate resolver rule.
	_, err = client.DisassociateResolverRule(ctx, &route53resolver.DisassociateResolverRuleInput{
		ResolverRuleId: aws.String(ruleID),
		VPCId:          aws.String("vpc-12345678"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete resolver rule.
	_, err = client.DeleteResolverRule(ctx, &route53resolver.DeleteResolverRuleInput{
		ResolverRuleId: aws.String(ruleID),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's deleted - should return error.
	_, err = client.GetResolverRule(ctx, &route53resolver.GetResolverRuleInput{
		ResolverRuleId: aws.String(ruleID),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
