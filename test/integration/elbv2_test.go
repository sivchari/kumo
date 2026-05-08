//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/sivchari/golden"
)

func newELBv2Client(t *testing.T) *elasticloadbalancingv2.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return elasticloadbalancingv2.NewFromConfig(cfg, func(o *elasticloadbalancingv2.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestELBv2_CreateAndDeleteLoadBalancer(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	lbName := "test-load-balancer"

	// Create load balancer
	createResult, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String(lbName),
		Subnets: []string{"subnet-12345678", "subnet-87654321"},
		Type:    types.LoadBalancerTypeEnumApplication,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("LoadBalancerArn", "DNSName", "CanonicalHostedZoneId", "CreatedTime", "VpcId", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	lb := createResult.LoadBalancers[0]

	t.Cleanup(func() {
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lb.LoadBalancerArn,
		})
	})

	// Delete load balancer
	_, err = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
		LoadBalancerArn: lb.LoadBalancerArn,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestELBv2_DescribeLoadBalancers(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	lbName := "test-describe-lb"

	// Create load balancer
	createResult, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String(lbName),
		Subnets: []string{"subnet-12345678"},
	})
	if err != nil {
		t.Fatal(err)
	}

	lbArn := createResult.LoadBalancers[0].LoadBalancerArn

	t.Cleanup(func() {
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	// Describe load balancers by ARN
	descResult, err := client.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []string{*lbArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("LoadBalancerArn", "DNSName", "CanonicalHostedZoneId", "CreatedTime", "VpcId", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestELBv2_CreateAndDeleteTargetGroup(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	tgName := "test-target-group"

	// Create target group
	createResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String(tgName),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumInstance,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("TargetGroupArn", "LoadBalancerArns", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	tg := createResult.TargetGroups[0]

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tg.TargetGroupArn,
		})
	})

	// Delete target group
	_, err = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
		TargetGroupArn: tg.TargetGroupArn,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestELBv2_DescribeTargetGroups(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	tgName := "test-describe-tg"

	// Create target group
	createResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String(tgName),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumInstance,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgArn := createResult.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
	})

	// Describe target groups
	descResult, err := client.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		TargetGroupArns: []string{*tgArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("TargetGroupArn", "LoadBalancerArns", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestELBv2_RegisterAndDeregisterTargets(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	tgName := "test-register-targets"

	// Create target group
	createResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String(tgName),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumInstance,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgArn := createResult.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
	})

	// Register targets
	_, err = client.RegisterTargets(ctx, &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: tgArn,
		Targets: []types.TargetDescription{
			{Id: aws.String("i-12345678"), Port: aws.Int32(80)},
			{Id: aws.String("i-87654321"), Port: aws.Int32(80)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Deregister targets
	_, err = client.DeregisterTargets(ctx, &elasticloadbalancingv2.DeregisterTargetsInput{
		TargetGroupArn: tgArn,
		Targets: []types.TargetDescription{
			{Id: aws.String("i-12345678")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestELBv2_CreateAndDeleteListener(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	lbName := "test-listener-lb"
	tgName := "test-listener-tg"

	// Create load balancer
	lbResult, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String(lbName),
		Subnets: []string{"subnet-12345678"},
	})
	if err != nil {
		t.Fatal(err)
	}

	lbArn := lbResult.LoadBalancers[0].LoadBalancerArn

	// Create target group
	tgResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String(tgName),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumInstance,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgArn := tgResult.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	// Create listener
	listenerResult, err := client.CreateListener(ctx, &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: lbArn,
		Port:            aws.Int32(80),
		Protocol:        types.ProtocolEnumHttp,
		DefaultActions: []types.Action{
			{
				Type:           types.ActionTypeEnumForward,
				TargetGroupArn: tgArn,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ListenerArn", "LoadBalancerArn", "TargetGroupArn", "ResultMetadata")).Assert(t.Name()+"_create", listenerResult)

	listenerArn := listenerResult.Listeners[0].ListenerArn

	// Delete listener
	_, err = client.DeleteListener(context.Background(), &elasticloadbalancingv2.DeleteListenerInput{
		ListenerArn: listenerArn,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestELBv2_LoadBalancerWithTargetGroupAndListener(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()

	// Create load balancer
	lbResult, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String("test-full-lb"),
		Subnets: []string{"subnet-12345678", "subnet-87654321"},
		Type:    types.LoadBalancerTypeEnumApplication,
	})
	if err != nil {
		t.Fatal(err)
	}

	lbArn := lbResult.LoadBalancers[0].LoadBalancerArn

	// Create target group
	tgResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String("test-full-tg"),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumInstance,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgArn := tgResult.TargetGroups[0].TargetGroupArn

	// Register targets
	_, err = client.RegisterTargets(ctx, &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: tgArn,
		Targets: []types.TargetDescription{
			{Id: aws.String("i-12345678"), Port: aws.Int32(80)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create listener
	listenerResult, err := client.CreateListener(ctx, &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: lbArn,
		Port:            aws.Int32(80),
		Protocol:        types.ProtocolEnumHttp,
		DefaultActions: []types.Action{
			{
				Type:           types.ActionTypeEnumForward,
				TargetGroupArn: tgArn,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	listenerArn := listenerResult.Listeners[0].ListenerArn

	// Cleanup in reverse order
	t.Cleanup(func() {
		_, _ = client.DeleteListener(context.Background(), &elasticloadbalancingv2.DeleteListenerInput{
			ListenerArn: listenerArn,
		})
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	// Verify everything is created
	descLbResult, err := client.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []string{*lbArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("LoadBalancerArn", "DNSName", "CanonicalHostedZoneId", "CreatedTime", "VpcId", "SubnetId", "ResultMetadata")).Assert(t.Name()+"_describe_lb", descLbResult)

	descTgResult, err := client.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		TargetGroupArns: []string{*tgArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("TargetGroupArn", "LoadBalancerArns", "ResultMetadata")).Assert(t.Name()+"_describe_tg", descTgResult)
}

func TestELBv2_ListenerRuleLifecycle(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()

	lbRes, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String("rule-test-lb"),
		Subnets: []string{"subnet-aaaa1111", "subnet-bbbb2222"},
	})
	if err != nil {
		t.Fatalf("CreateLoadBalancer: %v", err)
	}

	lbArn := lbRes.LoadBalancers[0].LoadBalancerArn

	t.Cleanup(func() {
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	tgRes, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:     aws.String("rule-test-tg"),
		Protocol: types.ProtocolEnumHttp,
		Port:     aws.Int32(80),
		VpcId:    aws.String("vpc-xxxx"),
	})
	if err != nil {
		t.Fatalf("CreateTargetGroup: %v", err)
	}

	tgArn := tgRes.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
	})

	listenerRes, err := client.CreateListener(ctx, &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: lbArn,
		Protocol:        types.ProtocolEnumHttp,
		Port:            aws.Int32(80),
		DefaultActions: []types.Action{
			{Type: types.ActionTypeEnumForward, TargetGroupArn: tgArn},
		},
	})
	if err != nil {
		t.Fatalf("CreateListener: %v", err)
	}

	listenerArn := listenerRes.Listeners[0].ListenerArn

	t.Cleanup(func() {
		_, _ = client.DeleteListener(context.Background(), &elasticloadbalancingv2.DeleteListenerInput{
			ListenerArn: listenerArn,
		})
	})

	createRuleRes, err := client.CreateRule(ctx, &elasticloadbalancingv2.CreateRuleInput{
		ListenerArn: listenerArn,
		Priority:    aws.Int32(100),
		Conditions: []types.RuleCondition{
			{
				Field: aws.String("path-pattern"),
				PathPatternConfig: &types.PathPatternConditionConfig{
					Values: []string{"/api/*"},
				},
			},
		},
		Actions: []types.Action{
			{Type: types.ActionTypeEnumForward, TargetGroupArn: tgArn},
		},
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	ruleArn := createRuleRes.Rules[0].RuleArn

	if got := len(createRuleRes.Rules[0].Conditions); got != 1 {
		t.Errorf("created rule has %d conditions, want 1", got)
	}

	if got := len(createRuleRes.Rules[0].Conditions[0].Values); got != 1 || createRuleRes.Rules[0].Conditions[0].Values[0] != "/api/*" {
		t.Errorf("rule.Conditions[0].Values = %v, want [/api/*]", createRuleRes.Rules[0].Conditions[0].Values)
	}

	descRes, err := client.DescribeRules(ctx, &elasticloadbalancingv2.DescribeRulesInput{
		ListenerArn: listenerArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Default rule + the rule we created.
	if got := len(descRes.Rules); got != 2 {
		t.Errorf("DescribeRules returned %d rules, want 2 (default + created)", got)
	}

	if _, err := client.DeleteRule(ctx, &elasticloadbalancingv2.DeleteRuleInput{
		RuleArn: ruleArn,
	}); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}

	descAfter, err := client.DescribeRules(ctx, &elasticloadbalancingv2.DescribeRulesInput{
		ListenerArn: listenerArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := len(descAfter.Rules); got != 1 {
		t.Errorf("after delete, DescribeRules returned %d rules, want 1 (default only)", got)
	}
}

func TestELBv2_LoadBalancerAttributes(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()

	lbRes, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String("attr-test-lb"),
		Subnets: []string{"subnet-aaaa1111", "subnet-bbbb2222"},
	})
	if err != nil {
		t.Fatalf("CreateLoadBalancer: %v", err)
	}

	lbArn := lbRes.LoadBalancers[0].LoadBalancerArn

	t.Cleanup(func() {
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	descBefore, err := client.DescribeLoadBalancerAttributes(ctx, &elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{
		LoadBalancerArn: lbArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	if found := lookupLBAttr(descBefore.Attributes, "deletion_protection.enabled"); found != "false" {
		t.Errorf("default deletion_protection.enabled = %q, want %q", found, "false")
	}

	if found := lookupLBAttr(descBefore.Attributes, "idle_timeout.timeout_seconds"); found != "60" {
		t.Errorf("default idle_timeout.timeout_seconds = %q, want %q", found, "60")
	}

	if _, err := client.ModifyLoadBalancerAttributes(ctx, &elasticloadbalancingv2.ModifyLoadBalancerAttributesInput{
		LoadBalancerArn: lbArn,
		Attributes: []types.LoadBalancerAttribute{
			{Key: aws.String("deletion_protection.enabled"), Value: aws.String("true")},
			{Key: aws.String("idle_timeout.timeout_seconds"), Value: aws.String("120")},
		},
	}); err != nil {
		t.Fatalf("ModifyLoadBalancerAttributes: %v", err)
	}

	descAfter, err := client.DescribeLoadBalancerAttributes(ctx, &elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{
		LoadBalancerArn: lbArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	if found := lookupLBAttr(descAfter.Attributes, "deletion_protection.enabled"); found != "true" {
		t.Errorf("after Modify, deletion_protection.enabled = %q, want %q", found, "true")
	}

	if found := lookupLBAttr(descAfter.Attributes, "idle_timeout.timeout_seconds"); found != "120" {
		t.Errorf("after Modify, idle_timeout.timeout_seconds = %q, want %q", found, "120")
	}
}

func TestELBv2_TargetGroupAttributes(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()

	tgRes, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:     aws.String("attr-test-tg"),
		Protocol: types.ProtocolEnumHttp,
		Port:     aws.Int32(80),
		VpcId:    aws.String("vpc-xxxx"),
	})
	if err != nil {
		t.Fatalf("CreateTargetGroup: %v", err)
	}

	tgArn := tgRes.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
	})

	descBefore, err := client.DescribeTargetGroupAttributes(ctx, &elasticloadbalancingv2.DescribeTargetGroupAttributesInput{
		TargetGroupArn: tgArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	if found := lookupTGAttr(descBefore.Attributes, "deregistration_delay.timeout_seconds"); found != "300" {
		t.Errorf("default deregistration_delay.timeout_seconds = %q, want %q", found, "300")
	}

	if _, err := client.ModifyTargetGroupAttributes(ctx, &elasticloadbalancingv2.ModifyTargetGroupAttributesInput{
		TargetGroupArn: tgArn,
		Attributes: []types.TargetGroupAttribute{
			{Key: aws.String("deregistration_delay.timeout_seconds"), Value: aws.String("60")},
		},
	}); err != nil {
		t.Fatalf("ModifyTargetGroupAttributes: %v", err)
	}

	descAfter, err := client.DescribeTargetGroupAttributes(ctx, &elasticloadbalancingv2.DescribeTargetGroupAttributesInput{
		TargetGroupArn: tgArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	if found := lookupTGAttr(descAfter.Attributes, "deregistration_delay.timeout_seconds"); found != "60" {
		t.Errorf("after Modify, deregistration_delay.timeout_seconds = %q, want %q", found, "60")
	}
}

func lookupLBAttr(attrs []types.LoadBalancerAttribute, key string) string {
	for _, a := range attrs {
		if a.Key != nil && *a.Key == key && a.Value != nil {
			return *a.Value
		}
	}

	return ""
}

func lookupTGAttr(attrs []types.TargetGroupAttribute, key string) string {
	for _, a := range attrs {
		if a.Key != nil && *a.Key == key && a.Value != nil {
			return *a.Value
		}
	}

	return ""
}

func TestELBv2_DescribeListenersAndModify(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()

	lbRes, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String("listener-mod-lb"),
		Subnets: []string{"subnet-aaaa1111"},
	})
	if err != nil {
		t.Fatalf("CreateLoadBalancer: %v", err)
	}

	lbArn := lbRes.LoadBalancers[0].LoadBalancerArn

	t.Cleanup(func() {
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	tgRes, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:     aws.String("listener-mod-tg"),
		Protocol: types.ProtocolEnumHttp,
		Port:     aws.Int32(80),
		VpcId:    aws.String("vpc-x"),
	})
	if err != nil {
		t.Fatalf("CreateTargetGroup: %v", err)
	}

	tgArn := tgRes.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
	})

	listenerRes, err := client.CreateListener(ctx, &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: lbArn,
		Protocol:        types.ProtocolEnumHttp,
		Port:            aws.Int32(80),
		DefaultActions: []types.Action{
			{Type: types.ActionTypeEnumForward, TargetGroupArn: tgArn},
		},
	})
	if err != nil {
		t.Fatalf("CreateListener: %v", err)
	}

	listenerArn := listenerRes.Listeners[0].ListenerArn

	t.Cleanup(func() {
		_, _ = client.DeleteListener(context.Background(), &elasticloadbalancingv2.DeleteListenerInput{
			ListenerArn: listenerArn,
		})
	})

	descRes, err := client.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{
		LoadBalancerArn: lbArn,
	})
	if err != nil {
		t.Fatalf("DescribeListeners: %v", err)
	}

	if got := len(descRes.Listeners); got != 1 {
		t.Errorf("DescribeListeners returned %d listeners, want 1", got)
	}

	if _, err := client.ModifyListener(ctx, &elasticloadbalancingv2.ModifyListenerInput{
		ListenerArn: listenerArn,
		Port:        aws.Int32(8080),
	}); err != nil {
		t.Fatalf("ModifyListener: %v", err)
	}

	descAfter, err := client.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{
		ListenerArns: []string{*listenerArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := *descAfter.Listeners[0].Port; got != 8080 {
		t.Errorf("after Modify, listener.Port = %d, want 8080", got)
	}
}

func TestELBv2_DescribeTargetHealth(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()

	tgRes, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:     aws.String("health-test-tg"),
		Protocol: types.ProtocolEnumHttp,
		Port:     aws.Int32(80),
		VpcId:    aws.String("vpc-x"),
	})
	if err != nil {
		t.Fatalf("CreateTargetGroup: %v", err)
	}

	tgArn := tgRes.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
	})

	// On a freshly-created target group with no registered targets, the
	// unfiltered call returns an empty list.
	healthRes, err := client.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: tgArn,
	})
	if err != nil {
		t.Fatalf("DescribeTargetHealth: %v", err)
	}

	if got := len(healthRes.TargetHealthDescriptions); got != 0 {
		t.Errorf("on empty target group, DescribeTargetHealth returned %d entries, want 0", got)
	}

	// A request with explicit Targets returns one entry per requested target,
	// reporting "unused" for any target that is not currently registered. This
	// matches AWS behavior for the read-side query.
	healthFiltered, err := client.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: tgArn,
		Targets: []types.TargetDescription{
			{Id: aws.String("10.0.0.99"), Port: aws.Int32(80)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := len(healthFiltered.TargetHealthDescriptions); got != 1 {
		t.Fatalf("filtered DescribeTargetHealth returned %d entries, want 1", got)
	}

	if got := string(healthFiltered.TargetHealthDescriptions[0].TargetHealth.State); got != "unused" {
		t.Errorf("non-registered target state = %q, want unused", got)
	}
}
