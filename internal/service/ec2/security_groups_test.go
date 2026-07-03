package ec2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDescribeSecurityGroupsReturnsCreatedGroupWithRulesAndTags(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	service := New(storage)
	ctx := context.Background()

	sg, err := storage.CreateSecurityGroup(ctx, &CreateSecurityGroupRequest{
		GroupName:        "kumo-tasks",
		GroupDescription: "kumo ECS tasks",
		VpcID:            "vpc-kumo",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := storage.AuthorizeSecurityGroupIngress(ctx, sg.GroupID, "", []IPPermission{
		{
			IPProtocol: "tcp",
			FromPort:   4566,
			ToPort:     4566,
			IPRanges: []IPRange{
				{CidrIP: "10.0.0.0/8"},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := storage.CreateTags(ctx, []string{sg.GroupID}, []Tag{
		{Key: "Name", Value: "kumo-tasks"},
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"GroupIds":["`+sg.GroupID+`"]}`))
	rec := httptest.NewRecorder()

	service.DescribeSecurityGroups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, want := range []string{
		"<DescribeSecurityGroupsResponse",
		"<groupId>" + sg.GroupID + "</groupId>",
		"<groupName>kumo-tasks</groupName>",
		"<groupDescription>kumo ECS tasks</groupDescription>",
		"<vpcId>vpc-kumo</vpcId>",
		"<ipProtocol>tcp</ipProtocol>",
		"<fromPort>4566</fromPort>",
		"<toPort>4566</toPort>",
		"<cidrIp>10.0.0.0/8</cidrIp>",
		"<key>Name</key>",
		"<value>kumo-tasks</value>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got %s", want, body)
		}
	}
}

func TestRevokeSecurityGroupEgressRemovesMatchingRule(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	service := New(storage)
	ctx := context.Background()

	sg, err := storage.CreateSecurityGroup(ctx, &CreateSecurityGroupRequest{
		GroupName:        "kumo-alb",
		GroupDescription: "kumo ALB",
		VpcID:            "vpc-kumo",
	})
	if err != nil {
		t.Fatal(err)
	}

	permission := IPPermission{
		IPProtocol: "-1",
		FromPort:   0,
		ToPort:     0,
		IPRanges: []IPRange{
			{CidrIP: "0.0.0.0/0"},
		},
	}
	if err := storage.AuthorizeSecurityGroupEgress(ctx, sg.GroupID, []IPPermission{permission}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{
		"GroupId":"`+sg.GroupID+`",
		"IpPermissions":[{"IpProtocol":"-1","FromPort":0,"ToPort":0,"IpRanges":[{"CidrIp":"0.0.0.0/0"}]}]
	}`))
	rec := httptest.NewRecorder()

	service.RevokeSecurityGroupEgress(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	groups, err := storage.DescribeSecurityGroups(ctx, []string{sg.GroupID}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(groups[0].EgressRules) != 0 {
		t.Fatalf("expected egress rules to be removed, got %#v", groups[0].EgressRules)
	}
}
