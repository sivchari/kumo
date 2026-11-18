package acm

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
)

// opGetCertificate is the GetCertificate operation name.
const opGetCertificate = "GetCertificate"

// dispatch sends an ACM request with body to svc and returns the response.
func dispatch(t *testing.T, svc *Service, operation, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-Amz-Target", "CertificateManager."+operation)
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	return w
}

func parsePEMCertificate(t *testing.T, data string) *x509.Certificate {
	t.Helper()

	block, rest := pem.Decode([]byte(data))
	if block == nil || block.Type != pemTypeCertificate {
		t.Fatalf("no CERTIFICATE PEM block in %q", data)
	}

	if len(rest) != 0 {
		t.Fatalf("unexpected data after the PEM block: %q", rest)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	return cert
}

// assertDescribeMatchesLeaf checks that the describe fields of an issued
// certificate match its certificate material.
func assertDescribeMatchesLeaf(t *testing.T, got *Certificate, leaf, ca *x509.Certificate) {
	t.Helper()

	if got.Serial != hex.EncodeToString(leaf.SerialNumber.Bytes()) {
		t.Errorf("Serial: got %s, want %x", got.Serial, leaf.SerialNumber.Bytes())
	}

	if got.NotBefore == nil || !got.NotBefore.Equal(leaf.NotBefore) || got.NotAfter == nil || !got.NotAfter.Equal(leaf.NotAfter) {
		t.Errorf("validity: got %v..%v, want %v..%v", got.NotBefore, got.NotAfter, leaf.NotBefore, leaf.NotAfter)
	}

	if got.Issuer != ca.Subject.CommonName {
		t.Errorf("Issuer: got %q, want %q", got.Issuer, ca.Subject.CommonName)
	}

	block, _ := pem.Decode([]byte(got.PrivateKey))
	if block == nil {
		t.Fatal("no private key PEM block")
	}

	if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
		t.Errorf("parse private key: %v", err)
	}
}

func TestGetCertificate_IssuedCertificateMaterial(t *testing.T) {
	t.Parallel()

	const domain = "material.example.com"

	store := NewMemoryStorage()
	ctx := t.Context()

	requested, err := store.RequestCertificate(ctx, &RequestCertificateInput{
		DomainName:              domain,
		SubjectAlternativeNames: []string{"*." + domain, domain},
	})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	got, err := store.GetCertificate(ctx, requested.CertificateArn)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}

	leaf := parsePEMCertificate(t, got.CertificateBody)
	ca := parsePEMCertificate(t, got.CertificateChain)

	if want := []string{domain, "*." + domain}; !slices.Equal(leaf.DNSNames, want) {
		t.Errorf("DNSNames: got %v, want %v", leaf.DNSNames, want)
	}

	if leaf.Subject.CommonName != domain {
		t.Errorf("CommonName: got %q, want %s", leaf.Subject.CommonName, domain)
	}

	if _, ok := leaf.PublicKey.(*rsa.PublicKey); !ok {
		t.Errorf("public key: got %T, want RSA for the default key algorithm", leaf.PublicKey)
	}

	roots := x509.NewCertPool()
	roots.AddCert(ca)

	if _, err := leaf.Verify(x509.VerifyOptions{
		DNSName:     "www." + domain,
		Roots:       roots,
		CurrentTime: leaf.NotBefore,
	}); err != nil {
		t.Errorf("verify leaf against chain: %v", err)
	}

	assertDescribeMatchesLeaf(t, got, leaf, ca)
}

func TestGetCertificate_ECKeyAlgorithm(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := t.Context()

	requested, err := store.RequestCertificate(ctx, &RequestCertificateInput{
		DomainName:   "ec.example.com",
		KeyAlgorithm: keyAlgorithmECPrime256v1,
	})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	got, err := store.GetCertificate(ctx, requested.CertificateArn)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}

	if _, ok := parsePEMCertificate(t, got.CertificateBody).PublicKey.(*ecdsa.PublicKey); !ok {
		t.Error("public key: want ECDSA for EC_prime256v1")
	}
}

func TestGetCertificate_PendingValidation(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := t.Context()

	requested, err := store.RequestCertificate(ctx, &RequestCertificateInput{
		DomainName:       "pending.example.com",
		ValidationMethod: validationEmail,
	})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	if requested.CertificateBody != "" {
		t.Errorf("pending certificate has material: %q", requested.CertificateBody)
	}

	_, err = store.GetCertificate(ctx, requested.CertificateArn)
	requireCode(t, err, errRequestPending)
}

func TestGetCertificate_PersistedState(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	store := NewMemoryStorage()

	requested, err := store.RequestCertificate(ctx, &RequestCertificateInput{DomainName: "persisted.example.com"})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	first, err := store.GetCertificate(ctx, requested.CertificateArn)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}

	data, err := json.Marshal(store)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	restored := NewMemoryStorage()
	if err := json.Unmarshal(data, restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got, err := restored.GetCertificate(ctx, requested.CertificateArn)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}

	if got.CertificateBody != first.CertificateBody || got.CertificateChain != first.CertificateChain {
		t.Error("certificate material changed across a save and load")
	}
}

// keyCounter replaces store.newKey with one that counts the keys generated
// and records whether any was generated while the storage lock was held.
type keyCounter struct {
	mu       sync.Mutex
	keys     int
	lockHeld bool
}

func countKeys(store *MemoryStorage) *keyCounter {
	c := &keyCounter{}

	store.newKey = func(keyAlgorithm string) (crypto.Signer, error) {
		c.mu.Lock()
		defer c.mu.Unlock()

		c.keys++

		if store.mu.TryLock() {
			store.mu.Unlock()
		} else {
			c.lockHeld = true
		}

		return generateKey(keyAlgorithm)
	}

	return c
}

// TestRequestCertificate_GeneratesNoKey checks that an issued certificate
// reports its validity and issuer on request, but has no key or
// certificate until GetCertificate is called.
func TestRequestCertificate_GeneratesNoKey(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	counter := countKeys(store)

	requested, err := store.RequestCertificate(t.Context(), &RequestCertificateInput{DomainName: "lazy.example.com"})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	if requested.Status != statusIssued || requested.NotBefore == nil || requested.NotAfter == nil || requested.Issuer != localCAName {
		t.Errorf("issued certificate: Status %s validity %v..%v Issuer %q, want ISSUED with validity and issuer",
			requested.Status, requested.NotBefore, requested.NotAfter, requested.Issuer)
	}

	if counter.keys != 0 || requested.CertificateBody != "" {
		t.Errorf("RequestCertificate generated %d keys, material %q; want none", counter.keys, requested.CertificateBody)
	}
}

// TestGetCertificate_GeneratesMaterialOnce checks that the first
// GetCertificate generates the key and the certificate, matching the
// serial number and validity reported on request, without holding the
// storage lock, and stores them, so a later call returns the same material
// without generating another key.
func TestGetCertificate_GeneratesMaterialOnce(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	counter := countKeys(store)
	ctx := t.Context()

	requested, err := store.RequestCertificate(ctx, &RequestCertificateInput{DomainName: "lazy-get.example.com"})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	first, err := store.GetCertificate(ctx, requested.CertificateArn)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}

	if counter.lockHeld {
		t.Error("GetCertificate generated the key while holding the storage lock")
	}

	leaf := parsePEMCertificate(t, first.CertificateBody)
	if !leaf.NotBefore.Equal(*requested.NotBefore) || !leaf.NotAfter.Equal(*requested.NotAfter) || first.Serial != requested.Serial {
		t.Errorf("certificate serial %s validity %v..%v, want what RequestCertificate reported: %s %v..%v",
			first.Serial, leaf.NotBefore, leaf.NotAfter, requested.Serial, requested.NotBefore, requested.NotAfter)
	}

	second, err := store.GetCertificate(ctx, requested.CertificateArn)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}

	if counter.keys != 1 || second.CertificateBody != first.CertificateBody || second.PrivateKey != first.PrivateKey {
		t.Errorf("second GetCertificate: %d keys generated in total, same material=%v; want 1 and true",
			counter.keys, second.CertificateBody == first.CertificateBody)
	}
}

// TestGetCertificate_ConcurrentFirstCalls runs the first GetCertificate of a
// certificate from several goroutines: every call returns the material that
// was stored, whichever call generated it.
func TestGetCertificate_ConcurrentFirstCalls(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := t.Context()

	requested, err := store.RequestCertificate(ctx, &RequestCertificateInput{
		DomainName:   "concurrent-get.example.com",
		KeyAlgorithm: keyAlgorithmECPrime256v1,
	})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	bodies := make([]string, 8)

	var wg sync.WaitGroup

	for i := range bodies {
		wg.Go(func() {
			got, err := store.GetCertificate(ctx, requested.CertificateArn)
			if err != nil {
				t.Errorf("GetCertificate: %v", err)

				return
			}

			bodies[i] = got.CertificateBody
		})
	}

	wg.Wait()

	stored := store.Certificates[requested.CertificateArn].CertificateBody
	for i, body := range bodies {
		if body != stored {
			t.Errorf("call %d returned material other than the stored one", i)
		}
	}
}

// TestCertificates_ConcurrentAccess runs requests and reads side by side so
// that go test -race reports any write to a certificate after it is stored.
func TestCertificates_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	var wg sync.WaitGroup

	for i := range 8 {
		wg.Go(func() {
			w := dispatch(t, svc, "RequestCertificate", fmt.Sprintf(`{"DomainName":"concurrent-%d.example.com","KeyAlgorithm":"EC_prime256v1"}`, i))
			if w.Code != http.StatusOK {
				t.Errorf("RequestCertificate: status %d body=%s", w.Code, w.Body.String())

				return
			}

			var requested RequestCertificateOutput
			if err := json.Unmarshal(w.Body.Bytes(), &requested); err != nil {
				t.Errorf("unmarshal RequestCertificate: %v", err)

				return
			}

			body := `{"CertificateArn":"` + requested.CertificateArn + `"}`
			for _, operation := range []string{opGetCertificate, "DescribeCertificate", "ListCertificates", opGetCertificate} {
				if w := dispatch(t, svc, operation, body); w.Code != http.StatusOK {
					t.Errorf("%s: status %d body=%s", operation, w.Code, w.Body.String())
				}
			}
		})
	}

	wg.Wait()
}
