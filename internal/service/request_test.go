package service_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sivchari/kumo/internal/service"
)

func TestReadJSONRequest(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{name: "valid JSON decodes into v", body: `{"name":"alice"}`, want: "alice"},
		{name: "empty body is a no-op", body: "", want: ""},
		{name: "invalid JSON returns an error", body: `{not json`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), "POST", "/", strings.NewReader(tt.body))

			var p payload

			err := service.ReadJSONRequest(req, &p)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadJSONRequest() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && p.Name != tt.want {
				t.Errorf("ReadJSONRequest() decoded name = %q, want %q", p.Name, tt.want)
			}
		})
	}
}
