package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestObjectACL_RoundTripWithCannedHeader — PUT with `x-amz-acl: private`,
// GET back, expect a single FULL_CONTROL grant for the owner.
func TestObjectACL_RoundTripWithCannedHeader(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	_ = store.CreateBucket(ctx, "ab")
	_, _ = store.PutObject(ctx, "ab", "k", strings.NewReader("x"), nil)

	putReq := httptest.NewRequest(http.MethodPut, "/ab/k?acl", http.NoBody)
	putReq.SetPathValue("bucket", "ab")
	putReq.SetPathValue("key", "k")
	putReq.Header.Set("X-Amz-Acl", "private")

	putW := httptest.NewRecorder()
	svc.handleObjectPut(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("PUT acl status: got %d, want 200 (body=%s)", putW.Code, putW.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/ab/k?acl", http.NoBody)
	getReq.SetPathValue("bucket", "ab")
	getReq.SetPathValue("key", "k")

	getW := httptest.NewRecorder()
	svc.handleObjectGet(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("GET acl status: got %d, want 200", getW.Code)
	}

	var got AccessControlPolicy
	if err := xml.Unmarshal(getW.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, getW.Body.String())
	}

	if len(got.ACL.Grants) != 1 {
		t.Fatalf("grants: got %d, want 1", len(got.ACL.Grants))
	}

	if got.ACL.Grants[0].Permission != "FULL_CONTROL" {
		t.Fatalf("permission: got %q, want FULL_CONTROL", got.ACL.Grants[0].Permission)
	}
}

// TestObjectACL_RoundTripWithBody — PUT a real grant list via XML body,
// GET back, expect the same grants.
func TestObjectACL_RoundTripWithBody(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	ctx := context.Background()

	_ = store.CreateBucket(ctx, "ab")
	_, _ = store.PutObject(ctx, "ab", "k", strings.NewReader("x"), nil)

	body := AccessControlPolicy{
		Owner: AccessControlOwner{ID: "user-1", DisplayName: "Alice"},
		ACL: AccessControlListBody{Grants: []AccessControlGrant{
			{Grantee: AccessControlGrantee{Type: "CanonicalUser", ID: "user-1", DisplayName: "Alice"}, Permission: "FULL_CONTROL"},
			{Grantee: AccessControlGrantee{Type: "Group", URI: "http://acs.amazonaws.com/groups/global/AllUsers"}, Permission: "READ"},
		}},
	}
	raw, _ := xml.Marshal(body)

	putReq := httptest.NewRequest(http.MethodPut, "/ab/k?acl", strings.NewReader(string(raw)))
	putReq.SetPathValue("bucket", "ab")
	putReq.SetPathValue("key", "k")

	putW := httptest.NewRecorder()
	svc.handleObjectPut(putW, putReq)

	if putW.Code != http.StatusOK {
		t.Fatalf("PUT status: got %d, want 200 (body=%s)", putW.Code, putW.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/ab/k?acl", http.NoBody)
	getReq.SetPathValue("bucket", "ab")
	getReq.SetPathValue("key", "k")

	getW := httptest.NewRecorder()
	svc.handleObjectGet(getW, getReq)

	var got AccessControlPolicy
	if err := xml.Unmarshal(getW.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, getW.Body.String())
	}

	if got.Owner.ID != "user-1" {
		t.Fatalf("owner.ID: got %q, want user-1", got.Owner.ID)
	}

	if len(got.ACL.Grants) != 2 {
		t.Fatalf("grants: got %d, want 2", len(got.ACL.Grants))
	}

	if got.ACL.Grants[1].Grantee.URI != "http://acs.amazonaws.com/groups/global/AllUsers" {
		t.Fatalf("Group grantee URI: got %q", got.ACL.Grants[1].Grantee.URI)
	}
}

// TestObjectACL_GetOnNonExistentObjectIs404 — GetObjectAcl on a key
// that doesn't exist returns NoSuchKey, not the default ACL.
func TestObjectACL_GetOnNonExistentObjectIs404(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store, "")
	_ = store.CreateBucket(context.Background(), "ab")

	req := httptest.NewRequest(http.MethodGet, "/ab/missing?acl", http.NoBody)
	req.SetPathValue("bucket", "ab")
	req.SetPathValue("key", "missing")

	w := httptest.NewRecorder()
	svc.handleObjectGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", w.Code)
	}
}
