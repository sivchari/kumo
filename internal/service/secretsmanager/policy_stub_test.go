package secretsmanager

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetResourcePolicy_MissingSecret(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage("http://localhost:4566"), "http://localhost:4566")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"SecretId":"does-not-exist"}`))
	req.Header.Set("X-Amz-Target", "secretsmanager.GetResourcePolicy")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404, body=%s", w.Code, w.Body.String())
	}
}

func TestPutResourcePolicy_MissingSecret(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage("http://localhost:4566"), "http://localhost:4566")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/",
		strings.NewReader(`{"SecretId":"does-not-exist","ResourcePolicy":"{}"}`))
	req.Header.Set("X-Amz-Target", "secretsmanager.PutResourcePolicy")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404, body=%s", w.Code, w.Body.String())
	}
}

func TestDeleteResourcePolicy_MissingSecret(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage("http://localhost:4566"), "http://localhost:4566")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/",
		strings.NewReader(`{"SecretId":"does-not-exist"}`))
	req.Header.Set("X-Amz-Target", "secretsmanager.DeleteResourcePolicy")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404, body=%s", w.Code, w.Body.String())
	}
}
