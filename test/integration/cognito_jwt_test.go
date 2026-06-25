//go:build integration

package integration

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

const kumoBaseURL = "http://localhost:4566"

// TestCognito_AdminInitiateAuth_JWTAndJWKS exercises the end-to-end path an IdP
// Lambda Authorizer relies on: AdminInitiateAuth must return an RS256-signed ID
// token, and the published JWKS must verify that token's signature and expiry.
func TestCognito_AdminInitiateAuth_JWTAndJWKS(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	const username, password, email = "jwtuser", "Password123!", "jwtuser@example.com"

	poolOut, err := client.CreateUserPool(ctx, &cognitoidentityprovider.CreateUserPoolInput{
		PoolName: aws.String("jwt-pool"),
	})
	if err != nil {
		t.Fatal(err)
	}

	userPoolID := *poolOut.UserPool.Id

	// Clean up the pool so it does not pollute the shared, stateful
	// TestCognito_ListUserPools golden (which lists every pool).
	t.Cleanup(func() {
		_, _ = client.DeleteUserPool(context.Background(), &cognitoidentityprovider.DeleteUserPoolInput{
			UserPoolId: aws.String(userPoolID),
		})
	})

	clientOut, err := client.CreateUserPoolClient(ctx, &cognitoidentityprovider.CreateUserPoolClientInput{
		UserPoolId: aws.String(userPoolID),
		ClientName: aws.String("jwt-client"),
		ExplicitAuthFlows: []types.ExplicitAuthFlowsType{
			types.ExplicitAuthFlowsTypeAllowAdminUserPasswordAuth,
			types.ExplicitAuthFlowsTypeAllowUserPasswordAuth,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	clientID := *clientOut.UserPoolClient.ClientId

	signUpAndConfirm(t, client, clientID, username, password, email)

	authOut, err := client.AdminInitiateAuth(ctx, &cognitoidentityprovider.AdminInitiateAuthInput{
		UserPoolId:     aws.String(userPoolID),
		ClientId:       aws.String(clientID),
		AuthFlow:       types.AuthFlowTypeAdminUserPasswordAuth,
		AuthParameters: map[string]string{"USERNAME": username, "PASSWORD": password},
	})
	if err != nil {
		t.Fatal(err)
	}

	if authOut.AuthenticationResult == nil || authOut.AuthenticationResult.IdToken == nil {
		t.Fatal("AdminInitiateAuth returned no IdToken")
	}

	idToken := *authOut.AuthenticationResult.IdToken
	if strings.Count(idToken, ".") != 2 {
		t.Fatalf("IdToken is not a 3-part JWT: %q", idToken)
	}

	// Authorizer-equivalent verification against the published JWKS.
	pub, kid := fetchJWKS(t, userPoolID)

	header := decodeJWTSegment(t, idToken, 0)
	if header["kid"] != kid {
		t.Errorf("token kid %v does not match JWKS kid %v", header["kid"], kid)
	}

	verifyJWTSignature(t, idToken, pub)

	claims := decodeJWTSegment(t, idToken, 1)
	if exp, ok := claims["exp"].(float64); !ok || int64(exp) <= time.Now().Unix() {
		t.Errorf("exp is not in the future: %v", claims["exp"])
	}

	if sub, ok := claims["sub"].(string); !ok || sub == "" {
		t.Errorf("sub claim is missing or empty: %v", claims["sub"])
	}
}

// signUpAndConfirm registers a user and confirms it so it can authenticate.
func signUpAndConfirm(t *testing.T, client *cognitoidentityprovider.Client, clientID, username, password, email string) {
	t.Helper()

	ctx := t.Context()

	if _, err := client.SignUp(ctx, &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(clientID),
		Username: aws.String(username),
		Password: aws.String(password),
		UserAttributes: []types.AttributeType{
			{Name: aws.String("email"), Value: aws.String(email)},
		},
	}); err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if _, err := client.ConfirmSignUp(ctx, &cognitoidentityprovider.ConfirmSignUpInput{
		ClientId:         aws.String(clientID),
		Username:         aws.String(username),
		ConfirmationCode: aws.String("123456"),
	}); err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}
}

// fetchJWKS GETs the User Pool JWKS and reconstructs the RSA public key + kid.
func fetchJWKS(t *testing.T, userPoolID string) (*rsa.PublicKey, string) {
	t.Helper()

	url := kumoBaseURL + "/" + userPoolID + "/.well-known/jwks.json"

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("build jwks request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("fetch jwks: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("jwks status: got %d, want 200", resp.StatusCode)
	}

	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		t.Fatalf("decode jwks: %v", err)
	}

	if len(set.Keys) != 1 {
		t.Fatalf("jwks keys: got %d, want 1", len(set.Keys))
	}

	k := set.Keys[0]

	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		t.Fatalf("decode jwk n: %v", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		t.Fatalf("decode jwk e: %v", err)
	}

	pub := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}

	return pub, k.Kid
}

// verifyJWTSignature asserts the compact JWS verifies under pub (RS256).
func verifyJWTSignature(t *testing.T, token string, pub *rsa.PublicKey) {
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
		t.Fatalf("signature verification failed: %v", err)
	}
}

// decodeJWTSegment decodes the base64url JSON of a token segment into a map.
func decodeJWTSegment(t *testing.T, token string, idx int) map[string]any {
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
