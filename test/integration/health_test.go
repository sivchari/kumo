//go:build integration

// Package integration provides integration tests for kumo.
package integration

import (
	"io"
	"net/http"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	resp, err := http.Get(testEndpoint() + "/health")
	if err != nil {
		t.Fatalf("failed to get health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	expected := `{"status":"healthy"}`
	if string(body) != expected {
		t.Errorf("expected body %q, got %q", expected, string(body))
	}
}
