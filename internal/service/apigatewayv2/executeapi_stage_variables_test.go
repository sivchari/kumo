package apigatewayv2

import (
	"maps"
	"net/http/httptest"
	"testing"
)

func TestResolveStage_ReturnsStageVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stageName  string
		invokePath string
		wantStage  string
		wantVars   map[string]string
	}{
		{
			name:       "named stage carries its variables",
			stageName:  "dev",
			invokePath: "/dev/items",
			wantStage:  "dev",
			wantVars:   map[string]string{"env": "dev"},
		},
		{
			name:       "$default fallback carries its variables",
			stageName:  defaultStageName,
			invokePath: "/items",
			wantStage:  defaultStageName,
			wantVars:   map[string]string{"env": "prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checkResolveStageReturnsVariables(t, tt.stageName, tt.invokePath, tt.wantStage, tt.wantVars)
		})
	}
}

func checkResolveStageReturnsVariables(t *testing.T, stageName, invokePath, wantStage string, wantVars map[string]string) {
	t.Helper()

	storage := NewMemoryStorage()
	svc := New(storage)

	api, err := storage.CreateAPI(t.Context(), &CreateAPIRequest{Name: "test-api", ProtocolType: "HTTP"})
	if err != nil {
		t.Fatalf("CreateAPI() error = %v", err)
	}

	_, err = storage.CreateStage(t.Context(), api.APIID, &CreateStageRequest{
		StageName:      stageName,
		StageVariables: wantVars,
	})
	if err != nil {
		t.Fatalf("CreateStage() error = %v", err)
	}

	r := httptest.NewRequestWithContext(t.Context(), "GET", invokePath, nil)

	stage, stageObj, routePath := svc.resolveStage(r, api.APIID, invokePath)

	if stage != wantStage {
		t.Errorf("resolveStage() stage = %q, want %q", stage, wantStage)
	}

	if routePath == "" {
		t.Error("resolveStage() routePath is empty, want a resolved path")
	}

	if stageObj == nil {
		t.Fatal("resolveStage() stageObj is nil, want a resolved *Stage")
	}

	if !maps.Equal(stageObj.StageVariables, wantVars) {
		t.Errorf("resolveStage() stageObj.StageVariables = %v, want %v", stageObj.StageVariables, wantVars)
	}
}

func TestResolveStage_UnknownStage(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()
	svc := New(storage)

	api, err := storage.CreateAPI(t.Context(), &CreateAPIRequest{Name: "test-api", ProtocolType: "HTTP"})
	if err != nil {
		t.Fatalf("CreateAPI() error = %v", err)
	}

	r := httptest.NewRequestWithContext(t.Context(), "GET", "/missing/items", nil)

	stage, stageObj, routePath := svc.resolveStage(r, api.APIID, "/missing/items")

	if stage != "" || stageObj != nil || routePath != "" {
		t.Errorf("resolveStage() = (%q, %v, %q), want (\"\", nil, \"\")", stage, stageObj, routePath)
	}
}
