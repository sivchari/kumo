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

	if got.CertificateBody != requested.CertificateBody || got.CertificateChain != requested.CertificateChain {
		t.Error("certificate material changed across a save and load")
	}
}

func TestGetCertificate_IssuedWithoutMaterial(t *testing.T) {
	t.Parallel()

	const arn = "arn:aws:acm:us-east-1:000000000000:certificate/no-material"

	store := NewMemoryStorage()
	store.Certificates[arn] = &Certificate{
		CertificateArn: arn,
		DomainName:     "no-material.example.com",
		Status:         statusIssued,
		Type:           certificateTypeAmazonIssued,
	}

	_, err := store.GetCertificate(t.Context(), arn)
	requireCode(t, err, errRequestPending)

	if store.Certificates[arn].CertificateBody != "" {
		t.Error("GetCertificate generated material")
	}
}

// TestRequestCertificate_IssuesOutsideStorageLock checks that the key of an
// issued certificate is generated while the storage lock is free, so a slow
// key does not block the other operations.
func TestRequestCertificate_IssuesOutsideStorageLock(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()

	var lockHeld bool

	store.newKey = func(keyAlgorithm string) (crypto.Signer, error) {
		if store.mu.TryLock() {
			store.mu.Unlock()
		} else {
			lockHeld = true
		}

		return generateKey(keyAlgorithm)
	}

	requested, err := store.RequestCertificate(t.Context(), &RequestCertificateInput{DomainName: "unlocked.example.com"})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	if lockHeld {
		t.Error("RequestCertificate generated the key while holding the storage lock")
	}

	if requested.CertificateBody == "" {
		t.Error("issued certificate has no material")
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
