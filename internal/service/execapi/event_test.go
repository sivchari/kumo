package execapi

import (
	"encoding/json"
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
			checkBuildEventV1StageVariables(t, tt.stageVariables, tt.wantVars)
		})
	}
}

func checkBuildEventV1StageVariables(t *testing.T, stageVariables, wantVars map[string]string) {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), "GET", "/dev/items", nil)
	req := &Request{
		APIID:          "api1",
		Stage:          "dev",
		ResourcePath:   "/items",
		StageVariables: stageVariables,
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

	if !mapsEqual(decoded.StageVariables, wantVars) {
		t.Errorf("stageVariables = %v, want %v", decoded.StageVariables, wantVars)
	}

	if stageVariables == nil && !hasNullField(data, "stageVariables") {
		t.Error("expected stageVariables to be present as null in the v1 event")
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

	if !mapsEqual(typed.StageVariables, wantVars) {
		t.Errorf("stageVariables = %v, want %v", typed.StageVariables, wantVars)
	}
}

func mapsEqual(a, b map[string]string) bool {
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
