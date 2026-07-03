package elbv2

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateListenerParsesDefaultActionsFromQueryForm(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	service := New(storage)

	lb, err := storage.CreateLoadBalancer(nil, &CreateLoadBalancerRequest{
		Name:           "kumo-local",
		Type:           "application",
		Scheme:         "internal",
		SecurityGroups: []string{"sg-kumo"},
		Subnets:        []string{"subnet-kumo-a"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tg, err := storage.CreateTargetGroup(nil, &CreateTargetGroupRequest{
		Name:       "kumo-local-kumo",
		Port:       4566,
		Protocol:   "HTTP",
		TargetType: "ip",
		VpcID:      "vpc-kumo",
	})
	if err != nil {
		t.Fatal(err)
	}

	body := "Action=CreateListener" +
		"&LoadBalancerArn=" + lb.LoadBalancerArn +
		"&Port=4566" +
		"&Protocol=HTTP" +
		"&DefaultActions.member.1.Type=forward" +
		"&DefaultActions.member.1.TargetGroupArn=" + tg.TargetGroupArn
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	service.CreateListener(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "<TargetGroupArn>"+tg.TargetGroupArn+"</TargetGroupArn>") {
		t.Fatalf("expected listener response to include default target group, got %s", rec.Body.String())
	}
}

func TestCreateTargetGroupParsesMatcherFromQueryForm(t *testing.T) {
	t.Parallel()

	service := New(NewMemoryStorage())
	body := "Action=CreateTargetGroup" +
		"&Name=kumo-local-kumo" +
		"&Port=4566" +
		"&Protocol=HTTP" +
		"&TargetType=ip" +
		"&VpcId=vpc-kumo" +
		"&Matcher.HttpCode=200"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	service.CreateTargetGroup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "<Matcher><HttpCode>200</HttpCode></Matcher>") {
		t.Fatalf("expected target group response to include matcher, got %s", rec.Body.String())
	}
}
