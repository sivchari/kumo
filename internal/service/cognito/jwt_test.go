package cognito

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"
)

// Shared fixtures for the token tests.
const (
	testPoolID   = "us-east-1_pool"
	testClientID = "client123"
	testSub      = "sub-uuid-123"
	testUsername = "alice"
	testEmail    = "alice@example.com"
	testIssuer   = "http://localhost:4566/us-east-1_pool"
	testIDValid  = 60
	testACValid  = 30
	testNow      = 1_700_000_000
)

// newAuthFixtures builds a pool (with a freshly generated signing key), a
// client, and a user for token tests.
func newAuthFixtures(t *testing.T) (*UserPool, *UserPoolClient, *User) {
	t.Helper()

	key, err := newSigningKey()
	if err != nil {
		t.Fatalf("newSigningKey: %v", err)
	}

	pool := &UserPool{ID: testPoolID, SigningKey: key}
	client := &UserPoolClient{
		ClientID:            testClientID,
		IDTokenValidity:     testIDValid,
		AccessTokenValidity: testACValid,
	}
	user := &User{
		Username:   testUsername,
		Sub:        testSub,
		Attributes: []UserAttribute{{Name: "email", Value: testEmail}},
	}

	return pool, client, user
}

func TestIssueTokens(t *testing.T) {
	t.Parallel()

	pool, client, user := newAuthFixtures(t)

	idToken, accessToken, expiresIn, err := issueTokens(pool, client, user, testIssuer, time.Unix(testNow, 0))
	if err != nil {
		t.Fatalf("issueTokens: %v", err)
	}

	if want := int32(testACValid * tokenValidityUnitSeconds); expiresIn != want {
		t.Errorf("expiresIn: got %d, want %d", expiresIn, want)
	}

	pub := &pool.SigningKey.PrivateKey.PublicKey
	verifyRS256(t, idToken, pub)
	verifyRS256(t, accessToken, pub)

	header := decodeSegment(t, idToken, 0)
	if header["kid"] != pool.SigningKey.KeyID {
		t.Errorf("kid: got %v, want %v", header["kid"], pool.SigningKey.KeyID)
	}

	if header["alg"] != signingAlg {
		t.Errorf("alg: got %v, want %v", header["alg"], signingAlg)
	}

	idClaims := decodeSegment(t, idToken, 1)
	if got := claimInt(t, idClaims, "exp") - claimInt(t, idClaims, "iat"); got != testIDValid*tokenValidityUnitSeconds {
		t.Errorf("id exp-iat: got %d, want %d", got, testIDValid*tokenValidityUnitSeconds)
	}

	accessClaims := decodeSegment(t, accessToken, 1)
	if got := claimInt(t, accessClaims, "exp") - claimInt(t, accessClaims, "iat"); got != testACValid*tokenValidityUnitSeconds {
		t.Errorf("access exp-iat: got %d, want %d", got, testACValid*tokenValidityUnitSeconds)
	}
}

func TestIssueTokens_Claims(t *testing.T) {
	t.Parallel()

	pool, client, user := newAuthFixtures(t)

	idToken, accessToken, _, err := issueTokens(pool, client, user, testIssuer, time.Unix(testNow, 0))
	if err != nil {
		t.Fatalf("issueTokens: %v", err)
	}

	idClaims := decodeSegment(t, idToken, 1)
	accessClaims := decodeSegment(t, accessToken, 1)

	cases := []struct {
		name   string
		claims map[string]any
		key    string
		want   string
	}{
		{"id sub", idClaims, "sub", testSub},
		{"id aud", idClaims, "aud", testClientID},
		{"id iss", idClaims, "iss", testIssuer},
		{"id token_use", idClaims, "token_use", idTokenUse},
		{"id username", idClaims, "cognito:username", testUsername},
		{"id email", idClaims, "email", testEmail},
		{"access sub", accessClaims, "sub", testSub},
		{"access iss", accessClaims, "iss", testIssuer},
		{"access token_use", accessClaims, "token_use", accessTokenUse},
		{"access client_id", accessClaims, "client_id", testClientID},
		{"access username", accessClaims, "username", testUsername},
		{"access scope", accessClaims, "scope", accessScope},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkClaim(t, tc.claims, tc.key, tc.want)
		})
	}
}

func TestBuildJWKS(t *testing.T) {
	t.Parallel()

	key, err := newSigningKey()
	if err != nil {
		t.Fatalf("newSigningKey: %v", err)
	}

	pub := &key.PrivateKey.PublicKey
	set := buildJWKS(pub, key.KeyID)

	if len(set.Keys) != 1 {
		t.Fatalf("keys: got %d, want 1", len(set.Keys))
	}

	entry := set.Keys[0]
	if entry.Kty != "RSA" || entry.Use != "sig" || entry.Alg != signingAlg {
		t.Errorf("jwk header fields: %+v", entry)
	}

	if entry.Kid != key.KeyID {
		t.Errorf("kid: got %q, want %q", entry.Kid, key.KeyID)
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(entry.N)
	if err != nil {
		t.Fatalf("decode n: %v", err)
	}

	if got := new(big.Int).SetBytes(nBytes); got.Cmp(pub.N) != 0 {
		t.Errorf("modulus mismatch")
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(entry.E)
	if err != nil {
		t.Fatalf("decode e: %v", err)
	}

	if got := int(new(big.Int).SetBytes(eBytes).Int64()); got != pub.E {
		t.Errorf("exponent: got %d, want %d", got, pub.E)
	}
}

func TestSigningKey_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	key, err := newSigningKey()
	if err != nil {
		t.Fatalf("newSigningKey: %v", err)
	}

	data, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored signingKey
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.KeyID != key.KeyID {
		t.Errorf("kid: got %q, want %q", restored.KeyID, key.KeyID)
	}

	if restored.PrivateKey.N.Cmp(key.PrivateKey.N) != 0 {
		t.Errorf("modulus mismatch after round-trip")
	}
}

// verifyRS256 asserts the compact JWS verifies under pub.
func verifyRS256(t *testing.T, token string, pub *rsa.PublicKey) {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token is not 3 dot-separated parts: %q", token)
	}

	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}

	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
		t.Fatalf("signature verify failed: %v", err)
	}
}

// decodeSegment decodes the base64url JSON of a token segment into a map.
func decodeSegment(t *testing.T, token string, idx int) map[string]any {
	t.Helper()

	parts := strings.Split(token, ".")

	raw, err := base64.RawURLEncoding.DecodeString(parts[idx])
	if err != nil {
		t.Fatalf("decode segment %d: %v", idx, err)
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal segment %d: %v", idx, err)
	}

	return m
}

// checkClaim asserts a string claim equals want.
func checkClaim(t *testing.T, claims map[string]any, key, want string) {
	t.Helper()

	got, ok := claims[key].(string)
	if !ok {
		t.Errorf("claim %q: not a string (%T)", key, claims[key])

		return
	}

	if got != want {
		t.Errorf("claim %q: got %q, want %q", key, got, want)
	}
}

// claimInt returns a numeric claim as int64.
func claimInt(t *testing.T, claims map[string]any, key string) int64 {
	t.Helper()

	f, ok := claims[key].(float64)
	if !ok {
		t.Fatalf("claim %q: not a number (%T)", key, claims[key])
	}

	return int64(f)
}
