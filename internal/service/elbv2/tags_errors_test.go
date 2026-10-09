package elbv2

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sivchari/kumo/internal/server"
)

const (
	testAccountARN = "arn:aws:elasticloadbalancing:us-east-1:000000000000:"
	testSubnet     = "subnet-a"
	testVPC        = "vpc-a"
	testMatcher    = "200-299"
	jsonContent    = "application/x-amz-json-1.0"

	keyResourceArn1 = "ResourceArns.member.1"
	keyPort         = "Port"
	keyProtocol     = "Protocol"
	keyName         = "Name"
)

// newQueryDispatcher wires the service the way the server does, so requests
// reach the handlers as the form-to-JSON converted body.
func newQueryDispatcher(svc *Service) *server.QueryProtocolDispatcher {
	d := server.NewQueryProtocolDispatcher()
	for _, action := range svc.Actions() {
		d.RegisterAction(action, svc.TargetPrefix(), svc.ServiceIdentifier(), svc.DispatchAction)
	}

	return d
}

func postQuery(t *testing.T, h http.Handler, action string, values url.Values) *httptest.ResponseRecorder {
	t.Helper()

	values.Set("Action", action)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "aws-sdk-go-v2 api/elasticloadbalancingv2#1.0")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// mustPostQuery posts a request that must succeed.
func mustPostQuery(t *testing.T, h http.Handler, action string, values url.Values) string {
	t.Helper()

	rec := postQuery(t, h, action, values)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: status %d: %s", action, rec.Code, rec.Body.String())
	}

	return rec.Body.String()
}

// withTags adds Tags.member.N entries in the order given.
func withTags(values url.Values, pairs ...string) url.Values {
	for i := 0; i+1 < len(pairs); i += 2 {
		n := i/2 + 1
		values.Set(fmt.Sprintf("Tags.member.%d.Key", n), pairs[i])
		values.Set(fmt.Sprintf("Tags.member.%d.Value", n), pairs[i+1])
	}

	return values
}

func newTestLoadBalancer(t *testing.T, storage *MemoryStorage) *LoadBalancer {
	t.Helper()

	lb, err := storage.CreateLoadBalancer(context.Background(), &CreateLoadBalancerRequest{
		Name:    "lb",
		Type:    lbTypeApplication,
		Subnets: []string{testSubnet},
	})
	if err != nil {
		t.Fatal(err)
	}

	return lb
}

func TestTagActions_UnknownResourceReturnsSpecificNotFound(t *testing.T) {
	t.Parallel()

	resources := []struct {
		name     string
		arn      string
		wantCode string
	}{
		{"load balancer", testAccountARN + "loadbalancer/app/missing/0123456789abcdef", "LoadBalancerNotFound"},
		{"target group", testAccountARN + "targetgroup/missing/0123456789abcdef", "TargetGroupNotFound"},
		{"listener", testAccountARN + "listener/app/missing/0123456789abcdef/0123456789abcdef", "ListenerNotFound"},
		{"rule", testAccountARN + "listener-rule/app/missing/0123456789abcdef/0123456789abcdef/0123456789abcdef", "RuleNotFound"},
	}

	actions := map[string]func(url.Values){
		"AddTags":      func(v url.Values) { withTags(v, "k", "v") },
		"RemoveTags":   func(v url.Values) { v.Set("TagKeys.member.1", "k") },
		"DescribeTags": func(url.Values) {},
	}

	for action, fill := range actions {
		for _, res := range resources {
			t.Run(action+"/"+res.name, func(t *testing.T) {
				t.Parallel()

				values := url.Values{keyResourceArn1: {res.arn}}
				fill(values)

				rec := postQuery(t, newQueryDispatcher(New(NewMemoryStorage())), action, values)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
				}

				if want := "<Code>" + res.wantCode + "</Code>"; !strings.Contains(rec.Body.String(), want) {
					t.Fatalf("expected %s, got %s", want, rec.Body.String())
				}
			})
		}
	}
}

func TestAddTags_DuplicateKeysRejected(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	lb := newTestLoadBalancer(t, storage)

	values := withTags(url.Values{keyResourceArn1: {lb.LoadBalancerArn}}, "k", "a", "k", "b")

	rec := postQuery(t, newQueryDispatcher(New(storage)), "AddTags", values)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "<Code>DuplicateTagKeys</Code>") {
		t.Fatalf("expected 400 DuplicateTagKeys, got %d: %s", rec.Code, rec.Body.String())
	}

	assertNoStoredTag(t, storage, lb.LoadBalancerArn, "k")
}

func TestAddTags_TooManyTagsRejectedAtomically(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	ctx := context.Background()
	lb := newTestLoadBalancer(t, storage)

	full, err := storage.CreateTargetGroup(ctx, &CreateTargetGroupRequest{Name: "tg", Protocol: protocolHTTP, Port: 80, VpcID: testVPC})
	if err != nil {
		t.Fatal(err)
	}

	existing := make(map[string]string, maxTagsPerResource)
	for i := range maxTagsPerResource {
		existing[fmt.Sprintf("existing-%d", i)] = "v"
	}

	if err := storage.AddTags(ctx, []string{full.TargetGroupArn}, existing); err != nil {
		t.Fatalf("filling to the limit must succeed: %v", err)
	}

	// Overwriting an existing key adds no tag, so it stays within the limit.
	if err := storage.AddTags(ctx, []string{full.TargetGroupArn}, map[string]string{"existing-0": "new"}); err != nil {
		t.Fatalf("updating an existing key at the limit must succeed: %v", err)
	}

	values := withTags(url.Values{keyResourceArn1: {lb.LoadBalancerArn}}, "one-more", "v")
	values.Set("ResourceArns.member.2", full.TargetGroupArn)

	rec := postQuery(t, newQueryDispatcher(New(storage)), "AddTags", values)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "<Code>TooManyTags</Code>") {
		t.Fatalf("expected 400 TooManyTags, got %d: %s", rec.Code, rec.Body.String())
	}

	assertNoStoredTag(t, storage, lb.LoadBalancerArn, "one-more")
}

func TestTags_ListenerAndRuleLifecycle(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	h := newQueryDispatcher(New(storage))
	ctx := context.Background()
	lb := newTestLoadBalancer(t, storage)

	mustPostQuery(t, h, "CreateListener", withTags(url.Values{
		"LoadBalancerArn":              {lb.LoadBalancerArn},
		keyPort:                        {"80"},
		keyProtocol:                    {protocolHTTP},
		"DefaultActions.member.1.Type": {"fixed-response"},
	}, "scope", "2024"))

	listeners, err := storage.DescribeListeners(ctx, nil, lb.LoadBalancerArn)
	if err != nil {
		t.Fatal(err)
	}

	listenerArn := listeners[0].ListenerArn
	assertStoredTag(t, storage, listenerArn, "scope", "2024")

	mustPostQuery(t, h, "CreateRule", withTags(url.Values{
		"ListenerArn":                         {listenerArn},
		"Priority":                            {"10"},
		"Conditions.member.1.Field":           {"path-pattern"},
		"Conditions.member.1.Values.member.1": {"/x"},
		"Actions.member.1.Type":               {"fixed-response"},
	}, "created", "yes"))

	rules, err := storage.DescribeRules(ctx, listenerArn, nil)
	if err != nil {
		t.Fatal(err)
	}

	var ruleArn string

	for _, rule := range rules {
		if !rule.IsDefault {
			ruleArn = rule.RuleArn
		}
	}

	if ruleArn == "" {
		t.Fatal("expected a non-default rule")
	}

	assertStoredTag(t, storage, ruleArn, "created", "yes")

	mustPostQuery(t, h, "AddTags", withTags(url.Values{keyResourceArn1: {ruleArn}}, "extra", "1"))
	mustPostQuery(t, h, "RemoveTags", url.Values{keyResourceArn1: {ruleArn}, "TagKeys.member.1": {"created"}})

	assertStoredTag(t, storage, ruleArn, "extra", "1")
	assertNoStoredTag(t, storage, ruleArn, "created")
}

func TestCreateViaDispatcher_StoresTags(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	h := newQueryDispatcher(New(storage))
	ctx := context.Background()

	mustPostQuery(t, h, "CreateLoadBalancer", withTags(url.Values{
		keyName:            {"lb"},
		"Subnets.member.1": {testSubnet},
	}, "Project", testResourceName, "Build", "007", "Flag", "true"))

	lbs, err := storage.DescribeLoadBalancers(ctx, nil, []string{"lb"})
	if err != nil {
		t.Fatal(err)
	}

	// Numeric-looking and boolean-looking values must survive as strings.
	assertStoredTag(t, storage, lbs[0].LoadBalancerArn, "Project", testResourceName)
	assertStoredTag(t, storage, lbs[0].LoadBalancerArn, "Build", "007")
	assertStoredTag(t, storage, lbs[0].LoadBalancerArn, "Flag", "true")

	mustPostQuery(t, h, "CreateTargetGroup", withTags(url.Values{
		keyName:            {"tg"},
		keyProtocol:        {protocolHTTP},
		keyPort:            {"80"},
		"VpcId":            {testVPC},
		"Matcher.HttpCode": {testMatcher},
	}, "Project", testResourceName))

	tgs, err := storage.DescribeTargetGroups(ctx, nil, []string{"tg"}, "")
	if err != nil {
		t.Fatal(err)
	}

	assertStoredTag(t, storage, tgs[0].TargetGroupArn, "Project", testResourceName)

	if tgs[0].Matcher != testMatcher {
		t.Fatalf("expected matcher %s, got %q", testMatcher, tgs[0].Matcher)
	}
}

func TestCreate_JSONBodyStillReadable(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	svc := New(storage)
	ctx := context.Background()

	post := func(handler http.HandlerFunc, body string) {
		t.Helper()

		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", jsonContent)

		rec := httptest.NewRecorder()
		handler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
	}

	post(svc.CreateLoadBalancer, `{"Name":"json-lb","Subnets":["subnet-a"],"Type":"application"}`)
	post(svc.CreateTargetGroup, `{"Name":"json-tg","Protocol":"HTTP","Port":80,"VpcId":"vpc-a"}`)

	lbs, err := storage.DescribeLoadBalancers(ctx, nil, []string{"json-lb"})
	if err != nil || len(lbs) != 1 || len(lbs[0].AvailabilityZones) == 0 {
		t.Fatalf("expected json-lb with its subnet as availability zone, got %v (err %v)", lbs, err)
	}

	tgs, err := storage.DescribeTargetGroups(ctx, nil, []string{"json-tg"}, "")
	if err != nil || len(tgs) != 1 || tgs[0].Port != 80 {
		t.Fatalf("expected json-tg on port 80, got %v (err %v)", tgs, err)
	}
}

func TestCreateTargetGroup_MatcherAndHealthCheckDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		req         CreateTargetGroupRequest
		wantMatcher string
		wantEnabled bool
	}{
		{"http default matcher", CreateTargetGroupRequest{Name: "a", Protocol: protocolHTTP, Port: 80}, "200", true},
		{"https default matcher", CreateTargetGroupRequest{Name: "b", Protocol: protocolHTTPS, Port: 443}, "200", true},
		{"explicit matcher kept", CreateTargetGroupRequest{Name: "c", Protocol: protocolHTTP, Port: 80, Matcher: &Matcher{HTTPCode: testMatcher}}, testMatcher, true},
		{"tcp has no matcher", CreateTargetGroupRequest{Name: "d", Protocol: "TCP", Port: 80}, "", true},
		{"lambda disabled by default", CreateTargetGroupRequest{Name: "e", TargetType: "lambda"}, "", false},
		{"lambda explicitly enabled", CreateTargetGroupRequest{Name: "f", TargetType: "lambda", HealthCheckEnabled: true}, "", true},
		{"ip cannot be disabled", CreateTargetGroupRequest{Name: "g", Protocol: protocolHTTP, Port: 80, TargetType: "ip"}, "200", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tg, err := NewMemoryStorage().CreateTargetGroup(context.Background(), &tt.req)
			if err != nil {
				t.Fatal(err)
			}

			if tg.Matcher != tt.wantMatcher {
				t.Errorf("matcher: want %q, got %q", tt.wantMatcher, tg.Matcher)
			}

			if tg.HealthCheckEnabled != tt.wantEnabled {
				t.Errorf("HealthCheckEnabled: want %v, got %v", tt.wantEnabled, tg.HealthCheckEnabled)
			}
		})
	}
}

func TestCreateTargetGroup_FormHealthCheckEnabledForLambda(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		form string
		want bool
	}{
		{"omitted", "", false},
		{"explicit true", "&HealthCheckEnabled=true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storage := NewMemoryStorage()
			body := "Action=CreateTargetGroup&Name=fn&TargetType=lambda" + tt.form
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			rec := httptest.NewRecorder()
			New(storage).CreateTargetGroup(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}

			tgs, err := storage.DescribeTargetGroups(context.Background(), nil, []string{"fn"}, "")
			if err != nil {
				t.Fatal(err)
			}

			if tgs[0].HealthCheckEnabled != tt.want {
				t.Errorf("HealthCheckEnabled: want %v, got %v", tt.want, tgs[0].HealthCheckEnabled)
			}
		})
	}
}

func TestDescribeTargetGroups_ReturnsDefaultMatcher(t *testing.T) {
	t.Parallel()

	h := newQueryDispatcher(New(NewMemoryStorage()))

	mustPostQuery(t, h, "CreateTargetGroup", url.Values{
		keyName: {"tg"}, keyProtocol: {protocolHTTP}, "Port": {"80"}, "VpcId": {testVPC},
	})

	body := mustPostQuery(t, h, "DescribeTargetGroups", url.Values{"Names.member.1": {"tg"}})
	if !strings.Contains(body, "<Matcher><HttpCode>200</HttpCode></Matcher>") {
		t.Fatalf("expected default matcher in response, got %s", body)
	}
}

func assertNoStoredTag(t *testing.T, storage *MemoryStorage, arn, key string) {
	t.Helper()

	tagsByARN, err := storage.DescribeTags(context.Background(), []string{arn})
	if err != nil {
		t.Fatal(err)
	}

	if got, ok := tagsByARN[arn][key]; ok {
		t.Fatalf("expected tag %s to be absent, got %q", key, got)
	}
}
