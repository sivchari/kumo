// Package cognito provides AWS Cognito Identity Provider service emulation.
package cognito

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// JWT/JWKS constants.
const (
	signingAlg     = "RS256"
	idTokenUse     = "id"
	accessTokenUse = "access"
	accessScope    = "aws.cognito.signin.user.admin"
	// tokenValidityUnitSeconds converts the stored token validity (minutes)
	// to seconds. kumo interprets IDTokenValidity/AccessTokenValidity as
	// minutes, matching the default of 60 (= 1 hour, ExpiresIn 3600).
	tokenValidityUnitSeconds = 60
	// rsaKeyBits is the modulus size for User Pool signing keys.
	rsaKeyBits = 2048
)

// signingKey is a User Pool's RSA key pair plus its key id (kid). It is stored
// on UserPool and persisted as PKCS#8 DER so the published JWKS stays stable
// across restarts when KUMO_DATA_DIR is set. signingKeyJSON (its on-disk form)
// lives in types.go alongside the other wire/persistence structs.
type signingKey struct {
	KeyID      string
	PrivateKey *rsa.PrivateKey
}

// newSigningKey generates a fresh RSA key pair and key id.
func newSigningKey() (*signingKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}

	return &signingKey{KeyID: uuid.New().String(), PrivateKey: priv}, nil
}

// MarshalJSON serializes the key id and the PKCS#8 DER of the private key.
func (k *signingKey) MarshalJSON() ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(k.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("marshal signing key: %w", err)
	}

	data, err := json.Marshal(signingKeyJSON{KeyID: k.KeyID, DER: der})
	if err != nil {
		return nil, fmt.Errorf("marshal signing key json: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the key id and parses the PKCS#8 DER private key.
func (k *signingKey) UnmarshalJSON(data []byte) error {
	var aux signingKeyJSON

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("unmarshal signing key json: %w", err)
	}

	parsed, err := x509.ParsePKCS8PrivateKey(aux.DER)
	if err != nil {
		return fmt.Errorf("parse signing key: %w", err)
	}

	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("signing key is not RSA: %T", parsed)
	}

	k.KeyID = aux.KeyID
	k.PrivateKey = rsaKey

	return nil
}

// signRS256 builds an RS256 JWS compact serialization (RSASSA-PKCS1-v1_5 +
// SHA-256) from the already-marshalled header and claims.
func signRS256(headerJSON, claimsJSON []byte, key *rsa.PrivateKey) (string, error) {
	enc := base64.RawURLEncoding
	signingInput := enc.EncodeToString(headerJSON) + "." + enc.EncodeToString(claimsJSON)

	sum := sha256.Sum256([]byte(signingInput))

	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	return signingInput + "." + enc.EncodeToString(sig), nil
}

// issueTokens builds and signs the RS256 ID and access tokens for a successful
// authentication. expiresIn is the access token lifetime in seconds.
func issueTokens(
	pool *UserPool,
	client *UserPoolClient,
	user *User,
	issuer string,
	now time.Time,
) (idToken, accessToken string, expiresIn int32, err error) {
	header, err := json.Marshal(map[string]string{"kid": pool.SigningKey.KeyID, "alg": signingAlg})
	if err != nil {
		return "", "", 0, fmt.Errorf("marshal jwt header: %w", err)
	}

	iat := now.Unix()

	idClaims, err := json.Marshal(map[string]any{
		"sub":              user.Sub,
		"aud":              client.ClientID,
		"iss":              issuer,
		"token_use":        idTokenUse,
		"cognito:username": user.Username,
		"email":            userEmail(user),
		"auth_time":        iat,
		"iat":              iat,
		"exp":              iat + int64(client.IDTokenValidity)*tokenValidityUnitSeconds,
		"jti":              uuid.New().String(),
	})
	if err != nil {
		return "", "", 0, fmt.Errorf("marshal id claims: %w", err)
	}

	accessClaims, err := json.Marshal(map[string]any{
		"sub":       user.Sub,
		"iss":       issuer,
		"token_use": accessTokenUse,
		"client_id": client.ClientID,
		"username":  user.Username,
		"scope":     accessScope,
		"iat":       iat,
		"exp":       iat + int64(client.AccessTokenValidity)*tokenValidityUnitSeconds,
		"jti":       uuid.New().String(),
	})
	if err != nil {
		return "", "", 0, fmt.Errorf("marshal access claims: %w", err)
	}

	idToken, err = signRS256(header, idClaims, pool.SigningKey.PrivateKey)
	if err != nil {
		return "", "", 0, err
	}

	accessToken, err = signRS256(header, accessClaims, pool.SigningKey.PrivateKey)
	if err != nil {
		return "", "", 0, err
	}

	return idToken, accessToken, client.AccessTokenValidity * tokenValidityUnitSeconds, nil
}

// userEmail returns the user's email attribute, or empty if unset.
func userEmail(user *User) string {
	for _, attr := range user.Attributes {
		if attr.Name == "email" {
			return attr.Value
		}
	}

	return ""
}

// buildJWKS derives the JWK Set for a public key and its key id. n is the
// modulus as big-endian bytes, e is the public exponent as its minimal
// big-endian bytes, both base64url-encoded.
func buildJWKS(pub *rsa.PublicKey, kid string) jwkSet {
	return jwkSet{
		Keys: []jwk{
			{
				Kty: "RSA",
				Use: "sig",
				Alg: signingAlg,
				Kid: kid,
				N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			},
		},
	}
}

// buildIssuer derives the token issuer from the request host so it matches the
// host the Authorizer uses to fetch JWKS: iss = http://{host}/{userPoolID}.
func buildIssuer(r *http.Request, poolID string) string {
	return "http://" + r.Host + "/" + poolID
}
