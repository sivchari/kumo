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

func TestPublicKey_CreateGetListDelete(t *testing.T) {
	t.Parallel()

	svc := newSigningTestService()

	created := createPublicKeyForTest(t, svc, "ref-1", "key-one")

	if !strings.HasPrefix(created.ID, "K") {
		t.Fatalf("PublicKey.ID should start with K, got %q", created.ID)
	}

	// Get
	getReq := httptest.NewRequest(http.MethodGet, "/2020-05-31/public-key/"+created.ID, http.NoBody)
	getReq.SetPathValue("id", created.ID)

	getW := httptest.NewRecorder()
	svc.GetPublicKey(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("GetPublicKey status: %d body=%s", getW.Code, getW.Body.String())
	}

	// List
	listW := httptest.NewRecorder()
	svc.ListPublicKeys(listW, httptest.NewRequest(http.MethodGet, "/2020-05-31/public-key", http.NoBody))

	if listW.Code != http.StatusOK {
		t.Fatalf("ListPublicKeys status: %d", listW.Code)
	}

	var list PublicKeyListXML
	if err := xml.Unmarshal(listW.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v body=%s", err, listW.Body.String())
	}

	if list.Quantity != 1 || len(list.Items) != 1 {
		t.Fatalf("ListPublicKeys: quantity=%d items=%d, want 1/1", list.Quantity, len(list.Items))
	}

	// Delete
	delReq := httptest.NewRequest(http.MethodDelete, "/2020-05-31/public-key/"+created.ID, http.NoBody)
	delReq.SetPathValue("id", created.ID)

	delW := httptest.NewRecorder()
	svc.DeletePublicKey(delW, delReq)

	if delW.Code != http.StatusNoContent {
		t.Fatalf("DeletePublicKey status: %d body=%s", delW.Code, delW.Body.String())
	}

	// Get-after-delete is 404.
	getReq2 := httptest.NewRequest(http.MethodGet, "/2020-05-31/public-key/"+created.ID, http.NoBody)
	getReq2.SetPathValue("id", created.ID)

	getW2 := httptest.NewRecorder()
	svc.GetPublicKey(getW2, getReq2)

	if getW2.Code != http.StatusNotFound {
		t.Fatalf("GetPublicKey after delete: status %d, want 404", getW2.Code)
	}
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

func TestKeyGroup_CreateGetListAndPublicKeyInUseBlock(t *testing.T) {
	t.Parallel()

	svc := newSigningTestService()
	pk := createPublicKeyForTest(t, svc, "ref-x", "kx")

	body := KeyGroupConfigXML{Name: "g-prod", Items: []string{pk.ID}, Comment: "prod"}
	raw, _ := xml.Marshal(body)

	w := httptest.NewRecorder()
	svc.CreateKeyGroup(w, httptest.NewRequest(http.MethodPost, "/2020-05-31/key-group", strings.NewReader(string(raw))))

	if w.Code != http.StatusCreated {
		t.Fatalf("CreateKeyGroup status: %d body=%s", w.Code, w.Body.String())
	}

	var group KeyGroupResultXML
	if err := xml.Unmarshal(w.Body.Bytes(), &group); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}

	if len(group.KeyGroupConfig.Items) != 1 || group.KeyGroupConfig.Items[0] != pk.ID {
		t.Fatalf("KeyGroupConfig.Items: got %v, want [%s]", group.KeyGroupConfig.Items, pk.ID)
	}

	// Deleting the underlying PublicKey should fail with 409 PublicKeyInUse.
	delReq := httptest.NewRequest(http.MethodDelete, "/2020-05-31/public-key/"+pk.ID, http.NoBody)
	delReq.SetPathValue("id", pk.ID)

	delW := httptest.NewRecorder()
	svc.DeletePublicKey(delW, delReq)

	if delW.Code != http.StatusConflict {
		t.Fatalf("DeletePublicKey while referenced: status %d, want 409", delW.Code)
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
