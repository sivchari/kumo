package secretsmanager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetResourcePolicy_ExistingSecret(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("http://localhost:4566")
	svc := New(store, "http://localhost:4566")

	if _, err := store.CreateSecret(t.Context(), &CreateSecretRequest{
		Name:         "policy-existing",
		SecretString: "value",
	}); err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"SecretId":"policy-existing"}`))
	req.Header.Set("X-Amz-Target", "secretsmanager.GetResourcePolicy")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", w.Code, w.Body.String())
	}

	var resp getResourcePolicyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Name != "policy-existing" {
		t.Errorf("Name: got %q, want %q", resp.Name, "policy-existing")
	}

	if resp.ARN == "" {
		t.Error("ARN: empty (terraform reads ARN from refresh response)")
	}

	if resp.ResourcePolicy != "" {
		t.Errorf("ResourcePolicy: got %q, want empty (no policy modeled)", resp.ResourcePolicy)
	}
}

func TestGetResourcePolicy_MissingSecret(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage("http://localhost:4566"), "http://localhost:4566")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"SecretId":"does-not-exist"}`))
	req.Header.Set("X-Amz-Target", "secretsmanager.GetResourcePolicy")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404, body=%s", w.Code, w.Body.String())
	}
}

func TestPutAndDeleteResourcePolicy_NoOp(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage("http://localhost:4566"), "http://localhost:4566")

	for _, action := range []string{"PutResourcePolicy", "DeleteResourcePolicy"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/",
				strings.NewReader(`{"SecretId":"x","ResourcePolicy":"{}"}`))
			req.Header.Set("X-Amz-Target", "secretsmanager."+action)

			w := httptest.NewRecorder()
			svc.DispatchAction(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status: got %d, body=%s", w.Code, w.Body.String())
			}
		})
	}
}
