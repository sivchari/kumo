package cloudfront

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // Test helper for CloudFront RSA-SHA1 signatures.
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	testPolicyValue = "abc"
	testRemoteAddr  = "127.0.0.1:12345"
)

// testKeyPair generates a fresh RSA key pair and returns the private
// key plus the PEM-encoded public key string suitable for
// PublicKeyConfig.EncodedKey.
func testKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	return priv, string(pubPEM)
}

// cfSign produces a CloudFront-Base64-encoded RSA-SHA1 signature.
func cfSign(t *testing.T, priv *rsa.PrivateKey, message []byte) string {
	t.Helper()

	//nolint:gosec // CloudFront protocol mandates SHA1.
	h := sha1.Sum(message)

	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA1, h[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	return cfBase64Encode(sig)
}

// cfBase64Encode encodes to CloudFront's modified Base64.
func cfBase64Encode(data []byte) string {
	s := base64.StdEncoding.EncodeToString(data)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "=", "_")
	s = strings.ReplaceAll(s, "/", "~")

	return s
}

// newTestRequest builds an *http.Request with a background context
// for unit tests. Satisfies the noctx linter.
func newTestRequest(t *testing.T, target string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	return req
}

func TestExtractSignedCredentials_Cookies(t *testing.T) {
	t.Parallel()

	req := newTestRequest(t, "/kumo/cdn/E123/file.txt")
	req.AddCookie(&http.Cookie{Name: "CloudFront-Policy", Value: testPolicyValue})
	req.AddCookie(&http.Cookie{Name: "CloudFront-Signature", Value: "sig"})
	req.AddCookie(&http.Cookie{Name: "CloudFront-Key-Pair-Id", Value: "KID"})

	creds := extractSignedCredentials(req)
	if creds == nil {
		t.Fatal("expected credentials from cookies")
	}

	if creds.Policy != testPolicyValue {
		t.Errorf("Policy = %q, want %q", creds.Policy, testPolicyValue)
	}

	if creds.Signature != "sig" {
		t.Errorf("Signature = %q, want %q", creds.Signature, "sig")
	}

	if creds.KeyPairID != "KID" {
		t.Errorf("KeyPairID = %q, want %q", creds.KeyPairID, "KID")
	}
}

func TestExtractSignedCredentials_QueryCanned(t *testing.T) {
	t.Parallel()

	req := newTestRequest(t,
		"/kumo/cdn/E123/file.txt?Expires=1700000000&Signature=sig&Key-Pair-Id=KID")

	creds := extractSignedCredentials(req)
	if creds == nil {
		t.Fatal("expected credentials from query")
	}

	if creds.Expires != 1700000000 {
		t.Errorf("Expires = %d, want 1700000000", creds.Expires)
	}

	if creds.Policy != "" {
		t.Errorf("Policy = %q, want empty", creds.Policy)
	}
}

func TestExtractSignedCredentials_QueryCustom(t *testing.T) {
	t.Parallel()

	req := newTestRequest(t,
		"/kumo/cdn/E123/file.txt?Policy=abc&Signature=sig&Key-Pair-Id=KID")

	creds := extractSignedCredentials(req)
	if creds == nil {
		t.Fatal("expected credentials from query")
	}

	if creds.Policy != testPolicyValue {
		t.Errorf("Policy = %q, want %q", creds.Policy, testPolicyValue)
	}
}

func TestExtractSignedCredentials_None(t *testing.T) {
	t.Parallel()

	req := newTestRequest(t, "/kumo/cdn/E123/file.txt")

	creds := extractSignedCredentials(req)
	if creds != nil {
		t.Fatalf("expected nil, got %+v", creds)
	}
}

func TestCFBase64RoundTrip(t *testing.T) {
	t.Parallel()

	original := []byte(`{"Statement":[{"Resource":"*"}]}`)
	encoded := cfBase64Encode(original)

	if strings.ContainsAny(encoded, "+=/") {
		t.Errorf("encoded contains forbidden chars: %q", encoded)
	}

	decoded, err := cfBase64Decode(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !bytes.Equal(decoded, original) {
		t.Errorf("round-trip mismatch: got %q, want %q", decoded, original)
	}
}

func TestVerifyRSASHA1(t *testing.T) {
	t.Parallel()

	priv, pubPEM := testKeyPair(t)

	pub, err := parseRSAPublicKey(pubPEM)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}

	message := []byte("hello world")
	sig := cfSign(t, priv, message)

	if err := verifyRSASHA1(pub, message, sig); err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	// Tampered message should fail.
	if err := verifyRSASHA1(pub, []byte("tampered"), sig); err == nil {
		t.Fatal("expected error for tampered message")
	}
}

func TestEvaluatePolicy_Valid(t *testing.T) {
	t.Parallel()

	now := time.Unix(1700000000, 0)
	policy := &cfPolicy{
		Statement: []cfStatement{{
			Resource: "*",
			Condition: cfCondition{
				DateLessThan: &cfEpoch{EpochTime: 1700001000},
			},
		}},
	}

	req := newTestRequest(t, "/file.txt")
	req.RemoteAddr = testRemoteAddr

	if err := evaluatePolicy(policy, req, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvaluatePolicy_Expired(t *testing.T) {
	t.Parallel()

	now := time.Unix(1700002000, 0)
	policy := &cfPolicy{
		Statement: []cfStatement{{
			Resource: "*",
			Condition: cfCondition{
				DateLessThan: &cfEpoch{EpochTime: 1700001000},
			},
		}},
	}

	req := newTestRequest(t, "/file.txt")
	req.RemoteAddr = testRemoteAddr

	err := evaluatePolicy(policy, req, now)
	if err == nil {
		t.Fatal("expected error for expired policy")
	}

	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error = %q, want 'expired' substring", err.Error())
	}
}

func TestEvaluatePolicy_NotYetValid(t *testing.T) {
	t.Parallel()

	now := time.Unix(1699999000, 0)
	policy := &cfPolicy{
		Statement: []cfStatement{{
			Resource: "*",
			Condition: cfCondition{
				DateLessThan:    &cfEpoch{EpochTime: 1700010000},
				DateGreaterThan: &cfEpoch{EpochTime: 1700000000},
			},
		}},
	}

	req := newTestRequest(t, "/file.txt")
	req.RemoteAddr = testRemoteAddr

	err := evaluatePolicy(policy, req, now)
	if err == nil {
		t.Fatal("expected error for not-yet-valid policy")
	}

	if !strings.Contains(err.Error(), "not yet valid") {
		t.Errorf("error = %q, want 'not yet valid' substring", err.Error())
	}
}

func TestEvaluatePolicy_IPAllowed(t *testing.T) {
	t.Parallel()

	now := time.Unix(1700000000, 0)
	policy := &cfPolicy{
		Statement: []cfStatement{{
			Resource: "*",
			Condition: cfCondition{
				DateLessThan: &cfEpoch{EpochTime: 1700010000},
				IPAddress:    &cfIPAddr{SourceIP: "192.168.1.0/24"},
			},
		}},
	}

	req := newTestRequest(t, "/file.txt")
	req.RemoteAddr = "192.168.1.42:12345"

	if err := evaluatePolicy(policy, req, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvaluatePolicy_IPDenied(t *testing.T) {
	t.Parallel()

	now := time.Unix(1700000000, 0)
	policy := &cfPolicy{
		Statement: []cfStatement{{
			Resource: "*",
			Condition: cfCondition{
				DateLessThan: &cfEpoch{EpochTime: 1700010000},
				IPAddress:    &cfIPAddr{SourceIP: "10.0.0.0/8"},
			},
		}},
	}

	req := newTestRequest(t, "/file.txt")
	req.RemoteAddr = "192.168.1.42:12345"

	err := evaluatePolicy(policy, req, now)
	if err == nil {
		t.Fatal("expected error for denied IP")
	}

	if !strings.Contains(err.Error(), "not in allowed range") {
		t.Errorf("error = %q, want 'not in allowed range' substring", err.Error())
	}
}

func TestRequiresSigning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dist *Distribution
		want bool
	}{
		{
			name: "nil distribution",
			dist: nil,
			want: false,
		},
		{
			name: "no trusted key groups",
			dist: &Distribution{
				DistributionConfig: &DistributionConfig{
					DefaultCacheBehavior: &DefaultCacheBehavior{},
				},
			},
			want: false,
		},
		{
			name: "trusted key groups disabled",
			dist: &Distribution{
				DistributionConfig: &DistributionConfig{
					DefaultCacheBehavior: &DefaultCacheBehavior{
						TrustedKeyGroups: &TrustedKeyGroups{Enabled: false},
					},
				},
			},
			want: false,
		},
		{
			name: "trusted key groups enabled",
			dist: &Distribution{
				DistributionConfig: &DistributionConfig{
					DefaultCacheBehavior: &DefaultCacheBehavior{
						TrustedKeyGroups: &TrustedKeyGroups{Enabled: true, Quantity: 1, Items: []string{"kg1"}},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := requiresSigning(tt.dist)
			if got != tt.want {
				t.Errorf("requiresSigning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRSAPublicKey(t *testing.T) {
	t.Parallel()

	_, pubPEM := testKeyPair(t)

	pub, err := parseRSAPublicKey(pubPEM)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if pub == nil {
		t.Fatal("expected non-nil public key")
	}
}

func TestParseRSAPublicKey_Invalid(t *testing.T) {
	t.Parallel()

	_, err := parseRSAPublicKey("not a pem")
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestRequestResourceURL(t *testing.T) {
	t.Parallel()

	req := newTestRequest(t,
		"http://localhost:4566/kumo/cdn/E123/path/file.txt?Expires=123&Signature=sig&Key-Pair-Id=KID&color=red")

	got := requestResourceURL(req)
	want := "http://localhost:4566/kumo/cdn/E123/path/file.txt?color=red"

	if got != want {
		t.Errorf("requestResourceURL() = %q, want %q", got, want)
	}
}

func TestRequestResourceURL_NoExtraQuery(t *testing.T) {
	t.Parallel()

	req := newTestRequest(t,
		"http://localhost:4566/kumo/cdn/E123/file.txt?Expires=123&Signature=sig&Key-Pair-Id=KID")

	got := requestResourceURL(req)
	want := "http://localhost:4566/kumo/cdn/E123/file.txt"

	if got != want {
		t.Errorf("requestResourceURL() = %q, want %q", got, want)
	}
}

// signCanned signs a canned policy for the given URL and expiry.
// Used by integration-level tests in this package.
func signCanned(t *testing.T, priv *rsa.PrivateKey, resource string, expires int64) string {
	t.Helper()

	//nolint:gocritic // Canned policy must match the exact JSON the verifier reconstructs.
	policy := fmt.Sprintf(
		`{"Statement":[{"Resource":"%s","Condition":{"DateLessThan":{"AWS:EpochTime":%d}}}]}`,
		resource, expires,
	)

	return cfSign(t, priv, []byte(policy))
}

// signCustom signs a custom policy JSON document.
// Used by integration-level tests in this package.
func signCustom(t *testing.T, priv *rsa.PrivateKey, policyJSON []byte) string {
	t.Helper()

	return cfSign(t, priv, policyJSON)
}

// TestSignHelpers ensures signCanned and signCustom produce verifiable signatures.
func TestSignHelpers(t *testing.T) {
	t.Parallel()

	priv, pubPEM := testKeyPair(t)

	pub, err := parseRSAPublicKey(pubPEM)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	t.Run("canned", func(t *testing.T) {
		t.Parallel()

		resource := "http://localhost:4566/kumo/cdn/E1/file.txt"
		expires := int64(1700010000)
		sig := signCanned(t, priv, resource, expires)

		//nolint:gocritic // Must match the exact canned policy layout.
		policy := fmt.Sprintf(
			`{"Statement":[{"Resource":"%s","Condition":{"DateLessThan":{"AWS:EpochTime":%d}}}]}`,
			resource, expires,
		)

		if err := verifyRSASHA1(pub, []byte(policy), sig); err != nil {
			t.Fatalf("canned signature verification failed: %v", err)
		}
	})

	t.Run("custom", func(t *testing.T) {
		t.Parallel()

		policy := []byte(`{"Statement":[{"Resource":"*","Condition":{"DateLessThan":{"AWS:EpochTime":9999999999}}}]}`)
		sig := signCustom(t, priv, policy)

		if err := verifyRSASHA1(pub, policy, sig); err != nil {
			t.Fatalf("custom signature verification failed: %v", err)
		}
	})
}

// TestCheckEdgeSigning_NoTrustedKeyGroups verifies that requests pass
// through when the distribution does not require signing.
func TestCheckEdgeSigning_NoTrustedKeyGroups(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())
	dist := &Distribution{
		DistributionConfig: &DistributionConfig{
			DefaultCacheBehavior: &DefaultCacheBehavior{},
		},
	}

	rec := httptest.NewRecorder()
	req := newTestRequest(t, "/kumo/cdn/E1/file.txt")

	if !svc.checkEdgeSigning(rec, req, dist) {
		t.Fatal("expected pass-through when signing is not required")
	}
}

// TestCheckEdgeSigning_MissingCredentials verifies that a 403 is
// returned when signed credentials are absent.
func TestCheckEdgeSigning_MissingCredentials(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())
	dist := &Distribution{
		DistributionConfig: &DistributionConfig{
			DefaultCacheBehavior: &DefaultCacheBehavior{
				TrustedKeyGroups: &TrustedKeyGroups{Enabled: true, Quantity: 1, Items: []string{"kg1"}},
			},
		},
	}

	rec := httptest.NewRecorder()
	req := newTestRequest(t, "/kumo/cdn/E1/file.txt")

	if svc.checkEdgeSigning(rec, req, dist) {
		t.Fatal("expected rejection when credentials are missing")
	}

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
