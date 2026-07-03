package ecs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDescribeServicesReturnsCreatedServiceAsRunning(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	service := New(storage)
	ctx := context.Background()

	if _, err := storage.CreateCluster(ctx, &CreateClusterRequest{ClusterName: "kumo-local"}); err != nil {
		t.Fatal(err)
	}

	taskDefinition, err := storage.RegisterTaskDefinition(ctx, &RegisterTaskDefinitionRequest{
		Family: "kumo-local",
		ContainerDefinitions: []ContainerDefinition{
			{Name: "kumo", Image: "ghcr.io/sivchari/kumo:latest", Essential: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	created, err := storage.CreateService(ctx, &CreateServiceRequest{
		Cluster:                     "kumo-local",
		ServiceName:                 "kumo-local",
		TaskDefinition:              taskDefinition.TaskDefinitionArn,
		DesiredCount:                1,
		LaunchType:                  "FARGATE",
		SchedulingStrategy:          "REPLICA",
		AvailabilityZoneRebalancing: "DISABLED",
		LoadBalancers: []ServiceLoadBalancer{
			{
				TargetGroupArn: "arn:aws:elasticloadbalancing:us-east-1:000000000000:targetgroup/kumo-local-kumo/123",
				ContainerName:  "kumo",
				ContainerPort:  4566,
			},
		},
		NetworkConfiguration: &NetworkConfiguration{
			AwsvpcConfiguration: &AwsVpcConfiguration{
				Subnets:        []string{"subnet-kumo-a", "subnet-kumo-b"},
				SecurityGroups: []string{"sg-kumo"},
				AssignPublicIP: "DISABLED",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"cluster":"kumo-local","services":["kumo-local"]}`))
	rec := httptest.NewRecorder()

	service.DescribeServices(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, want := range []string{
		`"serviceArn":"` + created.ServiceArn + `"`,
		`"serviceName":"kumo-local"`,
		`"runningCount":1`,
		`"pendingCount":0`,
		`"status":"ACTIVE"`,
		`"schedulingStrategy":"REPLICA"`,
		`"availabilityZoneRebalancing":"DISABLED"`,
		`"networkConfiguration"`,
		`"awsvpcConfiguration"`,
		`"loadBalancers"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got %s", want, body)
		}
	}
}
