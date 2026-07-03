package elbv2

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAddTagsAreReturnedByDescribeTags(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	service := New(storage)

	lb, err := storage.CreateLoadBalancer(nil, &CreateLoadBalancerRequest{
		Name:    "kumo-local",
		Type:    "application",
		Scheme:  "internal",
		Subnets: []string{"subnet-kumo-a"},
	})
	if err != nil {
		t.Fatal(err)
	}

	addBody := "Action=AddTags" +
		"&ResourceArns.member.1=" + lb.LoadBalancerArn +
		"&Tags.member.1.Key=Project" +
		"&Tags.member.1.Value=kumo-local"
	addReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(addBody))
	addReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addRec := httptest.NewRecorder()

	service.AddTags(addRec, addReq)

	if addRec.Code != http.StatusOK {
		t.Fatalf("expected AddTags status 200, got %d: %s", addRec.Code, addRec.Body.String())
	}

	describeBody := "Action=DescribeTags&ResourceArns.member.1=" + lb.LoadBalancerArn
	describeReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(describeBody))
	describeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	describeRec := httptest.NewRecorder()

	service.DescribeTags(describeRec, describeReq)

	if describeRec.Code != http.StatusOK {
		t.Fatalf("expected DescribeTags status 200, got %d: %s", describeRec.Code, describeRec.Body.String())
	}

	body := describeRec.Body.String()
	for _, want := range []string{
		"<ResourceArn>" + lb.LoadBalancerArn + "</ResourceArn>",
		"<Key>Project</Key>",
		"<Value>kumo-local</Value>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got %s", want, body)
		}
	}
}

func TestCreateLoadBalancerStoresQueryTags(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	service := New(storage)
	body := "Action=CreateLoadBalancer" +
		"&Name=kumo-local" +
		"&Type=application" +
		"&Scheme=internal" +
		"&Subnets.member.1=subnet-kumo-a" +
		"&Tags.member.1.Key=Project" +
		"&Tags.member.1.Value=kumo-local"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	service.CreateLoadBalancer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	lbs, err := storage.DescribeLoadBalancers(nil, nil, []string{"kumo-local"})
	if err != nil {
		t.Fatal(err)
	}

	assertStoredTag(t, storage, lbs[0].LoadBalancerArn, "Project", "kumo-local")
}

func TestCreateTargetGroupStoresQueryTags(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	service := New(storage)
	body := "Action=CreateTargetGroup" +
		"&Name=kumo-local-kumo" +
		"&Port=4566" +
		"&Protocol=HTTP" +
		"&TargetType=ip" +
		"&VpcId=vpc-kumo" +
		"&Tags.member.1.Key=Project" +
		"&Tags.member.1.Value=kumo-local"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	service.CreateTargetGroup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	tgs, err := storage.DescribeTargetGroups(nil, nil, []string{"kumo-local-kumo"}, "")
	if err != nil {
		t.Fatal(err)
	}

	assertStoredTag(t, storage, tgs[0].TargetGroupArn, "Project", "kumo-local")
}

func assertStoredTag(t *testing.T, storage *MemoryStorage, arn, key, want string) {
	t.Helper()

	tagsByARN, err := storage.DescribeTags(nil, []string{arn})
	if err != nil {
		t.Fatal(err)
	}

	if got := tagsByARN[arn][key]; got != want {
		t.Fatalf("expected tag %s=%q, got %q", key, want, got)
	}
}
