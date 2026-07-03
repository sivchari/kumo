package secretsmanager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSecretValue = "value"

func TestGetResourcePolicy_ExistingSecret(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("http://localhost:4566")
	svc := New(store, "http://localhost:4566")

	if _, err := store.CreateSecret(t.Context(), &CreateSecretRequest{
		Name:         "policy-existing",
		SecretString: testSecretValue,
	}); err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"SecretId":"policy-existing"}`))
	req.Header.Set("X-Amz-Target", "secretsmanager.GetResourcePolicy")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", w.Code, w.Body.String())
	}

	var resp GetResourcePolicyResponse
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
		t.Errorf("ResourcePolicy: got %q, want empty (no policy attached yet)", resp.ResourcePolicy)
	}
}

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

func TestPutResourcePolicy(t *testing.T) {
	t.Parallel()

	const secretName = "policy-put"

	store := NewMemoryStorage("http://localhost:4566")
	svc := New(store, "http://localhost:4566")

	if _, err := store.CreateSecret(t.Context(), &CreateSecretRequest{
		Name:         secretName,
		SecretString: testSecretValue,
	}); err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	policy := `{"Version":"2012-10-17","Statement":[]}`

	body, err := json.Marshal(PutResourcePolicyRequest{SecretID: secretName, ResourcePolicy: policy})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(string(body)))
	req.Header.Set("X-Amz-Target", "secretsmanager.PutResourcePolicy")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", w.Code, w.Body.String())
	}

	var resp PutResourcePolicyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Name != secretName {
		t.Errorf("Name: got %q, want %q", resp.Name, secretName)
	}

	if resp.ARN == "" {
		t.Error("ARN: empty")
	}

	// Verify the policy is persisted via GetResourcePolicy.
	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"SecretId":"`+secretName+`"}`))
	getReq.Header.Set("X-Amz-Target", "secretsmanager.GetResourcePolicy")

	gw := httptest.NewRecorder()
	svc.DispatchAction(gw, getReq)

	var getResp GetResourcePolicyResponse
	if err := json.Unmarshal(gw.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if getResp.ResourcePolicy != policy {
		t.Errorf("ResourcePolicy: got %q, want %q", getResp.ResourcePolicy, policy)
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

func TestDeleteResourcePolicy(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("http://localhost:4566")
	svc := New(store, "http://localhost:4566")

	if _, err := store.CreateSecret(t.Context(), &CreateSecretRequest{
		Name:         "policy-delete",
		SecretString: testSecretValue,
	}); err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	// Put a policy first.
	if _, err := store.PutResourcePolicy(t.Context(), "policy-delete", `{"Version":"2012-10-17"}`); err != nil {
		t.Fatalf("PutResourcePolicy: %v", err)
	}

	// Delete the policy.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/",
		strings.NewReader(`{"SecretId":"policy-delete"}`))
	req.Header.Set("X-Amz-Target", "secretsmanager.DeleteResourcePolicy")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", w.Code, w.Body.String())
	}

	var resp DeleteResourcePolicyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Name != "policy-delete" {
		t.Errorf("Name: got %q, want %q", resp.Name, "policy-delete")
	}

	if resp.ARN == "" {
		t.Error("ARN: empty")
	}

	// Verify the policy is removed.
	getReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"SecretId":"policy-delete"}`))
	getReq.Header.Set("X-Amz-Target", "secretsmanager.GetResourcePolicy")

	gw := httptest.NewRecorder()
	svc.DispatchAction(gw, getReq)

	var getResp GetResourcePolicyResponse
	if err := json.Unmarshal(gw.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if getResp.ResourcePolicy != "" {
		t.Errorf("ResourcePolicy: got %q, want empty after delete", getResp.ResourcePolicy)
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
