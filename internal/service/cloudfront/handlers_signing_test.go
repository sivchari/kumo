package cloudfront

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const samplePEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA1234567890abcdef==
-----END PUBLIC KEY-----`

func newSigningTestService() *Service {
	return New(NewMemoryStorage())
}

func createPublicKeyForTest(t *testing.T, svc *Service, callerRef, name string) *PublicKeyResultXML {
	t.Helper()

	body := PublicKeyConfigXML{
		CallerReference: callerRef,
		Name:            name,
		EncodedKey:      samplePEM,
		Comment:         "test",
	}
	raw, _ := xml.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/2020-05-31/public-key", strings.NewReader(string(raw)))
	w := httptest.NewRecorder()
	svc.CreatePublicKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("CreatePublicKey: status %d body=%s", w.Code, w.Body.String())
	}

	var out PublicKeyResultXML
	if err := xml.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}

	return &out
}

func TestPublicKey_DuplicateCallerReferenceConflicts(t *testing.T) {
	t.Parallel()

	svc := newSigningTestService()
	createPublicKeyForTest(t, svc, "dup-ref", "k1")

	body := PublicKeyConfigXML{CallerReference: "dup-ref", Name: "k2", EncodedKey: samplePEM}
	raw, _ := xml.Marshal(body)

	w := httptest.NewRecorder()
	svc.CreatePublicKey(w, httptest.NewRequest(http.MethodPost, "/2020-05-31/public-key", strings.NewReader(string(raw))))

	if w.Code != http.StatusConflict {
		t.Fatalf("status: got %d, want 409 (duplicate caller ref)", w.Code)
	}
}

func TestKeyGroup_CreateRejectsUnknownPublicKey(t *testing.T) {
	t.Parallel()

	svc := newSigningTestService()

	body := KeyGroupConfigXML{Name: "g1", Items: []string{"K-doesnotexist"}}
	raw, _ := xml.Marshal(body)

	w := httptest.NewRecorder()
	svc.CreateKeyGroup(w, httptest.NewRequest(http.MethodPost, "/2020-05-31/key-group", strings.NewReader(string(raw))))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404 (NoSuchPublicKey)", w.Code)
	}
}

func TestKeyGroup_DeleteWhileReferencedByDistributionFails(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	svc := New(store)
	ctx := context.Background()

	pk, err := store.CreatePublicKey(ctx, &PublicKeyConfig{CallerReference: "r", Name: "k", EncodedKey: samplePEM})
	if err != nil {
		t.Fatalf("CreatePublicKey: %v", err)
	}

	group, err := store.CreateKeyGroup(ctx, &KeyGroupConfig{Name: "g", Items: []string{pk.ID}})
	if err != nil {
		t.Fatalf("CreateKeyGroup: %v", err)
	}

	// Stand up a distribution whose default cache behavior trusts our key group.
	store.Distributions["DIST123"] = &Distribution{
		ID: "DIST123",
		DistributionConfig: &DistributionConfig{
			DefaultCacheBehavior: &DefaultCacheBehavior{
				TrustedKeyGroups: &TrustedKeyGroups{Enabled: true, Quantity: 1, Items: []string{group.ID}},
			},
		},
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/2020-05-31/key-group/"+group.ID, http.NoBody)
	delReq.SetPathValue("id", group.ID)

	delW := httptest.NewRecorder()
	svc.DeleteKeyGroup(delW, delReq)

	if delW.Code != http.StatusConflict {
		t.Fatalf("DeleteKeyGroup while referenced: status %d, want 409", delW.Code)
	}
}

func TestSigningReadMethodsDoNotInitializeNilMaps(t *testing.T) {
	t.Parallel()

	store := &MemoryStorage{}
	ctx := context.Background()

	if got := store.ListPublicKeys(ctx); len(got) != 0 {
		t.Fatalf("ListPublicKeys len = %d, want 0", len(got))
	}

	if store.signing.PublicKeys != nil {
		t.Fatalf("ListPublicKeys initialized PublicKeys under read lock")
	}

	if _, err := store.GetPublicKey(ctx, "missing"); err == nil {
		t.Fatalf("GetPublicKey missing key error = nil")
	}

	if store.signing.PublicKeys != nil {
		t.Fatalf("GetPublicKey initialized PublicKeys under read lock")
	}

	if got := store.ListKeyGroups(ctx); len(got) != 0 {
		t.Fatalf("ListKeyGroups len = %d, want 0", len(got))
	}

	if store.signing.KeyGroups != nil {
		t.Fatalf("ListKeyGroups initialized KeyGroups under read lock")
	}

	if _, err := store.GetKeyGroup(ctx, "missing"); err == nil {
		t.Fatalf("GetKeyGroup missing key error = nil")
	}

	if store.signing.KeyGroups != nil {
		t.Fatalf("GetKeyGroup initialized KeyGroups under read lock")
	}
}
