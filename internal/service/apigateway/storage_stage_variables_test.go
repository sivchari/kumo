package apigateway

import "testing"

func TestMemoryStorage_CreateStage_Variables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		variables map[string]string
	}{
		{name: "populated variables round-trip", variables: map[string]string{"env": "dev"}},
		{name: "nil variables round-trip", variables: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			storage := NewMemoryStorage()

			api, err := storage.CreateRestAPI(t.Context(), &CreateRestAPIRequest{Name: "test-api"})
			if err != nil {
				t.Fatalf("CreateRestAPI() error = %v", err)
			}

			deployment, err := storage.CreateDeployment(t.Context(), api.ID, &CreateDeploymentRequest{})
			if err != nil {
				t.Fatalf("CreateDeployment() error = %v", err)
			}

			created, err := storage.CreateStage(t.Context(), api.ID, &CreateStageRequest{
				StageName:    "dev",
				DeploymentID: deployment.ID,
				Variables:    tt.variables,
			})
			if err != nil {
				t.Fatalf("CreateStage() error = %v", err)
			}

			if !mapsEqualV1(created.Variables, tt.variables) {
				t.Errorf("CreateStage() Variables = %v, want %v", created.Variables, tt.variables)
			}

			got, err := storage.GetStage(t.Context(), api.ID, "dev")
			if err != nil {
				t.Fatalf("GetStage() error = %v", err)
			}

			if !mapsEqualV1(got.Variables, tt.variables) {
				t.Errorf("GetStage() Variables = %v, want %v", got.Variables, tt.variables)
			}

			resp := toStageResponse(got)
			if !mapsEqualV1(resp.Variables, tt.variables) {
				t.Errorf("toStageResponse() Variables = %v, want %v", resp.Variables, tt.variables)
			}
		})
	}
}

func mapsEqualV1(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if b[k] != v {
			return false
		}
	}

	return true
}
