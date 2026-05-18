package iam

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	updateRoleName    = "update-test-role"
	originalAssume    = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`
	replacementAssume = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`
)

// TestUpdateRole_Description exercises the storage layer: changing
// Description and MaxSessionDuration via UpdateRole. Both arguments
// are optional in AWS — only fields that are sent on the wire are
// updated.
func TestUpdateRole_Description(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()

	if _, err := store.CreateRole(ctx, &CreateRoleRequest{
		RoleName:                 updateRoleName,
		AssumeRolePolicyDocument: originalAssume,
		Description:              "before",
		MaxSessionDuration:       3600,
	}); err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	desc := "after"

	maxSession := int(7200)

	if err := store.UpdateRole(ctx, updateRoleName, &desc, &maxSession); err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}

	got, err := store.GetRole(ctx, updateRoleName)
	if err != nil {
		t.Fatalf("GetRole: %v", err)
	}

	if got.Description != "after" {
		t.Fatalf("Description: got %q, want %q", got.Description, "after")
	}

	if got.MaxSessionDuration != 7200 {
		t.Fatalf("MaxSessionDuration: got %d, want 7200", got.MaxSessionDuration)
	}
}

// TestUpdateRole_PartialUpdate verifies that nil arguments are treated
// as "leave unchanged" rather than "set to zero".
func TestUpdateRole_PartialUpdate(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()

	_, _ = store.CreateRole(ctx, &CreateRoleRequest{
		RoleName:                 "partial-role",
		AssumeRolePolicyDocument: originalAssume,
		Description:              "keep this",
		MaxSessionDuration:       1800,
	})

	maxSession := int(7200)
	if err := store.UpdateRole(ctx, "partial-role", nil, &maxSession); err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}

	got, _ := store.GetRole(ctx, "partial-role")

	if got.Description != "keep this" {
		t.Fatalf("Description should be unchanged, got %q", got.Description)
	}

	if got.MaxSessionDuration != 7200 {
		t.Fatalf("MaxSessionDuration: got %d, want 7200", got.MaxSessionDuration)
	}
}

// TestUpdateAssumeRolePolicy replaces the trust policy on an existing
// role. Audit consumers care about the policy text being current — a
// stub that accepts the call but doesn't persist would mask drift.
func TestUpdateAssumeRolePolicy(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()

	_, _ = store.CreateRole(ctx, &CreateRoleRequest{
		RoleName:                 "trust-role",
		AssumeRolePolicyDocument: originalAssume,
	})

	if err := store.UpdateAssumeRolePolicy(ctx, "trust-role", replacementAssume); err != nil {
		t.Fatalf("UpdateAssumeRolePolicy: %v", err)
	}

	got, _ := store.GetRole(ctx, "trust-role")
	if got.AssumeRolePolicyDocument != replacementAssume {
		t.Fatalf("AssumeRolePolicyDocument:\ngot:  %s\nwant: %s",
			got.AssumeRolePolicyDocument, replacementAssume)
	}
}

// TestTagRole_UpsertByKey verifies merge-by-key tagging semantics:
// existing keys overwrite, new keys append. terraform/pulumi/alchemy
// all expect this — they tag a role with provider-specific keys
// (alchemy_stage, alchemy_resource, Name) on every apply.
func TestTagRole_UpsertByKey(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()

	_, _ = store.CreateRole(ctx, &CreateRoleRequest{
		RoleName:                 "tag-role",
		AssumeRolePolicyDocument: originalAssume,
		Tags: []Tag{
			{Key: "Name", Value: "first"},
			{Key: "Owner", Value: "team-a"},
		},
	})

	if err := store.TagRole(ctx, "tag-role", []Tag{
		{Key: "Name", Value: "second"},
		{Key: "alchemy_stage", Value: "dev"},
	}); err != nil {
		t.Fatalf("TagRole: %v", err)
	}

	got, _ := store.GetRole(ctx, "tag-role")

	want := map[string]string{
		"Name":          "second", // overwritten
		"Owner":         "team-a", // preserved
		"alchemy_stage": "dev",    // appended
	}
	if len(got.Tags) != len(want) {
		t.Fatalf("len(Tags): got %d, want %d (%+v)", len(got.Tags), len(want), got.Tags)
	}

	for _, tag := range got.Tags {
		if w, ok := want[tag.Key]; !ok || w != tag.Value {
			t.Fatalf("tag mismatch: got %s=%s, want %s=%s", tag.Key, tag.Value, tag.Key, w)
		}
	}
}

// TestUpdateRole_NotFound returns AWS-style NoSuchEntity for both
// updates when the role doesn't exist.
func TestUpdateRole_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewMemoryStorage()

	desc := "x"
	if err := store.UpdateRole(ctx, "no-such-role", &desc, nil); err == nil {
		t.Fatal("expected error from UpdateRole on missing role")
	} else {
		assertIAMErrorCode(t, err, errNoSuchEntity)
	}

	if err := store.UpdateAssumeRolePolicy(ctx, "no-such-role", "{}"); err == nil {
		t.Fatal("expected error from UpdateAssumeRolePolicy on missing role")
	} else {
		assertIAMErrorCode(t, err, errNoSuchEntity)
	}
}

// TestUpdateRole_HTTP exercises the AWS Query protocol surface. Real
// callers (terraform, alchemy, AWS SDK) hit /iam with
// Action=UpdateRole&RoleName=...&Description=...
func TestUpdateRole_HTTP(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()
	svc := New(store)

	_, _ = store.CreateRole(ctx, &CreateRoleRequest{
		RoleName:                 "http-update-role",
		AssumeRolePolicyDocument: originalAssume,
		Description:              "old",
		MaxSessionDuration:       3600,
	})

	t.Run("UpdateRole sets Description", func(t *testing.T) {
		body := strings.NewReader(
			"Action=UpdateRole&RoleName=http-update-role&Description=new&Version=2010-05-08")
		req := httptest.NewRequest(http.MethodPost, "/iam", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		svc.UpdateRole(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("UpdateRole HTTP: got %d, body=%s", w.Code, w.Body.String())
		}

		var resp UpdateRoleResponse
		if err := xml.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("xml unmarshal: %v", err)
		}

		got, _ := store.GetRole(ctx, "http-update-role")
		if got.Description != "new" {
			t.Fatalf("Description not persisted: got %q", got.Description)
		}
	})

	t.Run("UpdateAssumeRolePolicy replaces trust policy", func(t *testing.T) {
		body := strings.NewReader(
			"Action=UpdateAssumeRolePolicy&RoleName=http-update-role&PolicyDocument=" +
				url(replacementAssume) + "&Version=2010-05-08")
		req := httptest.NewRequest(http.MethodPost, "/iam", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		svc.UpdateAssumeRolePolicy(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("UpdateAssumeRolePolicy HTTP: got %d, body=%s", w.Code, w.Body.String())
		}

		got, _ := store.GetRole(ctx, "http-update-role")
		if got.AssumeRolePolicyDocument != replacementAssume {
			t.Fatalf("AssumeRolePolicyDocument not persisted")
		}
	})
}

// url is a tiny URL-encoder for the test bodies — the JSON document
// has braces and quotes that the form parser would otherwise trip on.
func url(s string) string {
	return strings.NewReplacer("{", "%7B", "}", "%7D", `"`, "%22", " ", "+").Replace(s)
}

// assertIAMErrorCode unwraps to *Error and checks Code.
func assertIAMErrorCode(t *testing.T, err error, want string) {
	t.Helper()

	var iamErr *Error
	if !errors.As(err, &iamErr) || iamErr.Code != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
