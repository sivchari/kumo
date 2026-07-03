package ecs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDescribeTaskDefinitionReturnsRegisteredDefinitionByArn(t *testing.T) {
	t.Parallel()

	service, taskDefinition := registeredKumoTaskDefinitionFixture(t)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"taskDefinition":"`+taskDefinition.TaskDefinitionArn+`"}`))
	rec := httptest.NewRecorder()

	service.DescribeTaskDefinition(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	assertTaskDefinitionBody(t, rec.Body.String(), taskDefinition.TaskDefinitionArn)
}

func registeredKumoTaskDefinitionFixture(t *testing.T) (*Service, *TaskDefinition) {
	t.Helper()

	storage := NewMemoryStorage()
	service := New(storage)

	taskDefinition, err := storage.RegisterTaskDefinition(context.Background(), kumoTaskDefinitionRequest())
	if err != nil {
		t.Fatal(err)
	}

	return service, taskDefinition
}

func kumoTaskDefinitionRequest() *RegisterTaskDefinitionRequest {
	return &RegisterTaskDefinitionRequest{
		Family:               "kumo-local",
		CPU:                  "512",
		Memory:               "1024",
		ContainerDefinitions: []ContainerDefinition{kumoContainerDefinition()},
		EphemeralStorage:     &EphemeralStorage{SizeInGiB: 21},
	}
}

func kumoContainerDefinition() ContainerDefinition {
	return ContainerDefinition{
		Name:                   "kumo",
		Image:                  "ghcr.io/sivchari/kumo:latest",
		Essential:              true,
		User:                   "10000:10000",
		ReadonlyRootFilesystem: true,
		MountPoints:            []MountPoint{},
		LinuxParameters: &LinuxParameters{
			InitProcessEnabled: true,
			Capabilities:       &KernelCapabilities{Drop: []string{"ALL"}, Add: []string{}},
		},
		LogConfiguration: &LogConfiguration{
			LogDriver: "awslogs",
			Options:   map[string]string{"awslogs-group": "/ecs/kumo-local/kumo"},
		},
	}
}

func assertTaskDefinitionBody(t *testing.T, body, taskDefinitionArn string) {
	t.Helper()

	for _, want := range []string{
		`"taskDefinitionArn":"` + taskDefinitionArn + `"`,
		`"family":"kumo-local"`,
		`"revision":1`,
		`"status":"ACTIVE"`,
		`"readonlyRootFilesystem":true`,
		`"user":"10000:10000"`,
		`"mountPoints":[]`,
		`"linuxParameters"`,
		`"logConfiguration"`,
		`"ephemeralStorage":{"sizeInGiB":21}`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got %s", want, body)
		}
	}
}
