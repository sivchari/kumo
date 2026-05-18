package iam

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDispatchAction_JSONBody covers the path where the AWS Query dispatcher
// has converted the form-encoded request to JSON before calling DispatchAction.
//
// terraform-provider-aws and other clients that target the unified `/`
// endpoint hit this code path; the IAM-direct `/iam/` route keeps the form
// payload as-is. The handlers must accept both shapes.
func TestDispatchAction_JSONBody(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	body := strings.NewReader(`{"RoleName":"json-role","AssumeRolePolicyDocument":"{\"Version\":\"2012-10-17\"}","Description":"from json"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", "iam.CreateRole")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("DispatchAction CreateRole (JSON): got %d, body=%s", w.Code, w.Body.String())
	}

	var resp CreateRoleResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("xml unmarshal: %v", err)
	}

	if got, want := resp.CreateRoleResult.Role.RoleName, "json-role"; got != want {
		t.Fatalf("RoleName: got %q, want %q", got, want)
	}

	if got, want := resp.CreateRoleResult.Role.Description, "from json"; got != want {
		t.Fatalf("Description: got %q, want %q (body restoration broken — multiple getJSONValue calls couldn't re-read the request body)", got, want)
	}
}

// TestQueryProtocolService confirms the IAM service surfaces the marker
// methods the unified Query dispatcher uses to register a service.
func TestQueryProtocolService(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	if got := svc.ServiceIdentifier(); got != "iam" {
		t.Fatalf("ServiceIdentifier: got %q, want %q", got, "iam")
	}

	if got := svc.TargetPrefix(); got == "" {
		t.Fatal("TargetPrefix: must be non-empty so the dispatcher can register a key")
	}

	actions := svc.Actions()
	if len(actions) == 0 {
		t.Fatal("Actions: expected at least one action")
	}

	has := func(name string) bool {
		for _, a := range actions {
			if a == name {
				return true
			}
		}

		return false
	}

	for _, want := range []string{"CreateRole", "GetRole", "DeleteRole", "UpdateRole"} {
		if !has(want) {
			t.Errorf("Actions missing %q", want)
		}
	}

	// QueryProtocol is a marker — calling it must not panic.
	svc.QueryProtocol()
}

// TestGetJSONValue_BodyRestoration locks in the contract that getJSONValue
// can be called repeatedly on the same request without consuming the body.
//
// IAM handlers call getFormValue once per parameter (RoleName,
// AssumeRolePolicyDocument, Path, Description, MaxSessionDuration, Tags ...),
// so the body must be readable across all of those calls.
func TestGetJSONValue_BodyRestoration(t *testing.T) {
	t.Parallel()

	body := io.NopCloser(strings.NewReader(`{"a":"first","b":"second"}`))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")

	if got := getFormValue(req, "a"); got != "first" {
		t.Fatalf("first read: got %q, want %q", got, "first")
	}

	if got := getFormValue(req, "b"); got != "second" {
		t.Fatalf("second read (body must be restored): got %q, want %q", got, "second")
	}

	if got := getFormValue(req, "missing"); got != "" {
		t.Fatalf("missing key: got %q, want empty", got)
	}
}
