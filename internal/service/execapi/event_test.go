package execapi

import (
	"encoding/json"
	"maps"
	"net/http/httptest"
	"testing"
)

func TestBuildEventV1_StageVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		stageVariables map[string]string
		wantVars       map[string]string
	}{
		{
			name:           "populated stage variables are included",
			stageVariables: map[string]string{"env": "dev"},
			wantVars:       map[string]string{"env": "dev"},
		},
		{
			name:           "nil stage variables encode as null",
			stageVariables: nil,
			wantVars:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequestWithContext(t.Context(), "GET", "/dev/items", nil)
			req := &Request{
				APIID:          "api1",
				Stage:          "dev",
				ResourcePath:   "/items",
				StageVariables: tt.stageVariables,
			}

			data, err := buildEventV1(r, req, nil)
			if err != nil {
				t.Fatalf("buildEventV1() error = %v", err)
			}

			var decoded struct {
				StageVariables map[string]string `json:"stageVariables"`
			}

			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal event: %v", err)
			}

			if !maps.Equal(decoded.StageVariables, tt.wantVars) {
				t.Errorf("stageVariables = %v, want %v", decoded.StageVariables, tt.wantVars)
			}

			if tt.stageVariables == nil && !hasNullField(data, "stageVariables") {
				t.Error("expected stageVariables to be present as null in the v1 event")
			}
		})
	}
}

func TestBuildEventV2_StageVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		stageVariables map[string]string
		wantVars       map[string]string
		wantOmitted    bool
	}{
		{
			name:           "populated stage variables are included",
			stageVariables: map[string]string{"env": "dev"},
			wantVars:       map[string]string{"env": "dev"},
		},
		{
			name:           "nil stage variables are omitted",
			stageVariables: nil,
			wantVars:       nil,
			wantOmitted:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checkBuildEventV2StageVariables(t, tt.stageVariables, tt.wantVars, tt.wantOmitted)
		})
	}
}

func checkBuildEventV2StageVariables(t *testing.T, stageVariables, wantVars map[string]string, wantOmitted bool) {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), "GET", "/items", nil)
	req := &Request{
		APIID:          "api1",
		Stage:          "$default",
		ResourcePath:   "/items",
		RouteKey:       "GET /items",
		StageVariables: stageVariables,
	}

	data, err := buildEventV2(r, req, nil)
	if err != nil {
		t.Fatalf("buildEventV2() error = %v", err)
	}

	var decoded map[string]any

	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	_, present := decoded["stageVariables"]
	if wantOmitted && present {
		t.Error("expected stageVariables to be omitted from the v2 event")
	}

	if wantOmitted {
		return
	}

	var typed struct {
		StageVariables map[string]string `json:"stageVariables"`
	}

	if err := json.Unmarshal(data, &typed); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	if !maps.Equal(typed.StageVariables, wantVars) {
		t.Errorf("stageVariables = %v, want %v", typed.StageVariables, wantVars)
	}
}

func TestBuildEventV2_StagePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stage    string
		wantPath string
	}{
		{
			name:     "$default stage is omitted from the path",
			stage:    "$default",
			wantPath: "/items",
		},
		{
			name:     "named stage is retained in the path",
			stage:    "prod",
			wantPath: "/prod/items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checkBuildEventV2StagePath(t, tt.stage, tt.wantPath)
		})
	}
}

func checkBuildEventV2StagePath(t *testing.T, stage, wantPath string) {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), "GET", wantPath, nil)
	req := &Request{
		APIID:        "api1",
		Stage:        stage,
		ResourcePath: "/items",
		RouteKey:     "GET /items",
	}

	data, err := buildEventV2(r, req, nil)
	if err != nil {
		t.Fatalf("buildEventV2() error = %v", err)
	}

	var decoded struct {
		RawPath        string `json:"rawPath"`
		RequestContext struct {
			Stage string `json:"stage"`
			HTTP  struct {
				Path string `json:"path"`
			} `json:"http"`
		} `json:"requestContext"`
	}

	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	if decoded.RawPath != wantPath {
		t.Errorf("rawPath = %q, want %q", decoded.RawPath, wantPath)
	}

	if decoded.RequestContext.HTTP.Path != wantPath {
		t.Errorf("requestContext.http.path = %q, want %q", decoded.RequestContext.HTTP.Path, wantPath)
	}

	if decoded.RequestContext.Stage != stage {
		t.Errorf("requestContext.stage = %q, want %q", decoded.RequestContext.Stage, stage)
	}
}

func TestBuildEventV1_StagePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stage    string
		wantPath string
	}{
		{
			name:     "$default stage is omitted from the path",
			stage:    "$default",
			wantPath: "/items",
		},
		{
			name:     "named stage is retained in the path",
			stage:    "dev",
			wantPath: "/dev/items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequestWithContext(t.Context(), "GET", tt.wantPath, nil)
			req := &Request{
				APIID:        "api1",
				Stage:        tt.stage,
				ResourcePath: "/items",
			}

			data, err := buildEventV1(r, req, nil)
			if err != nil {
				t.Fatalf("buildEventV1() error = %v", err)
			}

			var decoded struct {
				RequestContext struct {
					Path  string `json:"path"`
					Stage string `json:"stage"`
				} `json:"requestContext"`
			}

			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("unmarshal event: %v", err)
			}

			if decoded.RequestContext.Path != tt.wantPath {
				t.Errorf("requestContext.path = %q, want %q", decoded.RequestContext.Path, tt.wantPath)
			}

			if decoded.RequestContext.Stage != tt.stage {
				t.Errorf("requestContext.stage = %q, want %q", decoded.RequestContext.Stage, tt.stage)
			}
		})
	}
}

// hasNullField reports whether the given top-level JSON field is present and
// set to null in the raw payload.
func hasNullField(data []byte, field string) bool {
	var raw map[string]json.RawMessage

	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}

	v, ok := raw[field]

	return ok && string(v) == "null"
}
