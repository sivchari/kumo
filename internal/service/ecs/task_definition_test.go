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

	storage := NewMemoryStorage()
	service := New(storage)

	taskDefinition, err := storage.RegisterTaskDefinition(context.Background(), &RegisterTaskDefinitionRequest{
		Family: "kumo-local",
		CPU:    "512",
		Memory: "1024",
		ContainerDefinitions: []ContainerDefinition{
			{
				Name:                   "kumo",
				Image:                  "ghcr.io/sivchari/kumo:latest",
				Essential:              true,
				User:                   "10000:10000",
				ReadonlyRootFilesystem: true,
				MountPoints:            []MountPoint{},
				LinuxParameters: &LinuxParameters{
					InitProcessEnabled: true,
					Capabilities: &KernelCapabilities{
						Drop: []string{"ALL"},
						Add:  []string{},
					},
				},
				LogConfiguration: &LogConfiguration{
					LogDriver: "awslogs",
					Options: map[string]string{
						"awslogs-group": "/ecs/kumo-local/kumo",
					},
				},
			},
		},
		EphemeralStorage: &EphemeralStorage{SizeInGiB: 21},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"taskDefinition":"`+taskDefinition.TaskDefinitionArn+`"}`))
	rec := httptest.NewRecorder()

	service.DescribeTaskDefinition(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, want := range []string{
		`"taskDefinitionArn":"` + taskDefinition.TaskDefinitionArn + `"`,
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
