//go:build integration

package integration

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // CloudFront uses RSA-SHA1 for signed URL/cookie verification.
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfapi "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/sivchari/golden"
)

// generateTestKeyPair creates a 2048-bit RSA key pair for testing.
func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
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

// cfBase64 encodes bytes using CloudFront's modified Base64.
func cfBase64(data []byte) string {
	s := base64.StdEncoding.EncodeToString(data)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "=", "_")
	s = strings.ReplaceAll(s, "/", "~")

	return s
}

// signPolicy signs a policy JSON with RSA-SHA1 and returns a
// CloudFront-Base64-encoded signature.
func signPolicy(t *testing.T, priv *rsa.PrivateKey, policy []byte) string {
	t.Helper()

	//nolint:gosec // CloudFront mandates SHA1.
	h := sha1.Sum(policy)

	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA1, h[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	return cfBase64(sig)
}

func TestCloudFront_PublicKeyAndKeyGroup(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	_, pubPEM := generateTestKeyPair(t)

	// Create PublicKey.
	pkResult, err := client.CreatePublicKey(ctx, &cfapi.CreatePublicKeyInput{
		PublicKeyConfig: &types.PublicKeyConfig{
			CallerReference: aws.String("test-pk-signing"),
			Name:            aws.String("test-signing-key"),
			EncodedKey:      aws.String(pubPEM),
			Comment:         aws.String("Integration test key"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeletePublicKey(context.Background(), &cfapi.DeletePublicKeyInput{
			Id:      pkResult.PublicKey.Id,
			IfMatch: pkResult.ETag,
		})
	})

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"CreatedTime",
		"ETag",
		"EncodedKey",
		"ResultMetadata",
	)).Assert(t.Name()+"_create_public_key", pkResult)

	// Create KeyGroup referencing the PublicKey.
	kgResult, err := client.CreateKeyGroup(ctx, &cfapi.CreateKeyGroupInput{
		KeyGroupConfig: &types.KeyGroupConfig{
			Name:    aws.String("test-key-group"),
			Items:   []string{*pkResult.PublicKey.Id},
			Comment: aws.String("Integration test key group"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteKeyGroup(context.Background(), &cfapi.DeleteKeyGroupInput{
			Id:      kgResult.KeyGroup.Id,
			IfMatch: kgResult.ETag,
		})
	})

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"LastModifiedTime",
		"ETag",
		"Items",
		"ResultMetadata",
	)).Assert(t.Name()+"_create_key_group", kgResult)
}

func TestCloudFront_EdgeSignedCookie(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	priv, pubPEM := generateTestKeyPair(t)

	// 1. Create PublicKey.
	pkResult, err := client.CreatePublicKey(ctx, &cfapi.CreatePublicKeyInput{
		PublicKeyConfig: &types.PublicKeyConfig{
			CallerReference: aws.String("test-edge-signed-cookie"),
			Name:            aws.String("edge-signing-key"),
			EncodedKey:      aws.String(pubPEM),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	pkID := *pkResult.PublicKey.Id

	t.Cleanup(func() {
		_, _ = client.DeletePublicKey(context.Background(), &cfapi.DeletePublicKeyInput{
			Id:      pkResult.PublicKey.Id,
			IfMatch: pkResult.ETag,
		})
	})

	// 2. Create KeyGroup.
	kgResult, err := client.CreateKeyGroup(ctx, &cfapi.CreateKeyGroupInput{
		KeyGroupConfig: &types.KeyGroupConfig{
			Name:  aws.String("edge-key-group"),
			Items: []string{pkID},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	kgID := *kgResult.KeyGroup.Id

	t.Cleanup(func() {
		_, _ = client.DeleteKeyGroup(context.Background(), &cfapi.DeleteKeyGroupInput{
			Id:      kgResult.KeyGroup.Id,
			IfMatch: kgResult.ETag,
		})
	})

	// 3. Start a test origin that always returns 200.
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "origin-ok")
	}))
	t.Cleanup(origin.Close)

	// Parse origin host:port for the Distribution config.
	originAddr := strings.TrimPrefix(origin.URL, "http://")
	originHost, originPort := originAddr, 80

	if parts := strings.SplitN(originAddr, ":", 2); len(parts) == 2 {
		originHost = parts[0]
		fmt.Sscanf(parts[1], "%d", &originPort)
	}

	// 4. Create Distribution with TrustedKeyGroups and the test origin.
	distResult, err := client.CreateDistribution(ctx, &cfapi.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-edge-signed-cookie-dist"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{{
					Id:         aws.String("test-origin"),
					DomainName: aws.String(originHost),
					CustomOriginConfig: &types.CustomOriginConfig{
						HTTPPort:             aws.Int32(int32(originPort)),
						HTTPSPort:            aws.Int32(443),
						OriginProtocolPolicy: types.OriginProtocolPolicyHttpOnly,
					},
				}},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("test-origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				TrustedKeyGroups: &types.TrustedKeyGroups{
					Enabled:  aws.Bool(true),
					Quantity: aws.Int32(1),
					Items:    []string{kgID},
				},
				MinTTL:     aws.Int64(0),
				DefaultTTL: aws.Int64(0),
				MaxTTL:     aws.Int64(0),
				ForwardedValues: &types.ForwardedValues{
					QueryString: aws.Bool(false),
					Cookies:     &types.CookiePreference{Forward: types.ItemSelectionNone},
				},
			},
			Comment: aws.String("Signed cookie test"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	distID := *distResult.Distribution.Id

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cfapi.DeleteDistributionInput{
			Id:      distResult.Distribution.Id,
			IfMatch: distResult.ETag,
		})
	})

	edgeURL := fmt.Sprintf("http://localhost:4566/kumo/cdn/%s/test.txt", distID)

	t.Run("no_credentials_returns_403", func(t *testing.T) {
		t.Parallel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, edgeURL, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("valid_signed_cookie_passes", func(t *testing.T) {
		t.Parallel()

		// Build a custom policy with a far-future expiry.
		policy := []byte(`{"Statement":[{"Resource":"*","Condition":{"DateLessThan":{"AWS:EpochTime":9999999999}}}]}`)
		sig := signPolicy(t, priv, policy)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, edgeURL, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}

		req.AddCookie(&http.Cookie{Name: "CloudFront-Policy", Value: cfBase64(policy)})
		req.AddCookie(&http.Cookie{Name: "CloudFront-Signature", Value: sig})
		req.AddCookie(&http.Cookie{Name: "CloudFront-Key-Pair-Id", Value: pkID})

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()

		// The origin (kumo's root) returns 200.
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d, want %d; body = %s", resp.StatusCode, http.StatusOK, body)
		}
	})

	t.Run("bad_signature_returns_403", func(t *testing.T) {
		t.Parallel()

		policy := []byte(`{"Statement":[{"Resource":"*","Condition":{"DateLessThan":{"AWS:EpochTime":9999999999}}}]}`)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, edgeURL, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}

		req.AddCookie(&http.Cookie{Name: "CloudFront-Policy", Value: cfBase64(policy)})
		req.AddCookie(&http.Cookie{Name: "CloudFront-Signature", Value: "invalid-signature"})
		req.AddCookie(&http.Cookie{Name: "CloudFront-Key-Pair-Id", Value: pkID})

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
	})

	t.Run("expired_policy_returns_403", func(t *testing.T) {
		t.Parallel()

		// Policy with epoch 0 = expired.
		policy := []byte(`{"Statement":[{"Resource":"*","Condition":{"DateLessThan":{"AWS:EpochTime":0}}}]}`)
		sig := signPolicy(t, priv, policy)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, edgeURL, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}

		req.AddCookie(&http.Cookie{Name: "CloudFront-Policy", Value: cfBase64(policy)})
		req.AddCookie(&http.Cookie{Name: "CloudFront-Signature", Value: sig})
		req.AddCookie(&http.Cookie{Name: "CloudFront-Key-Pair-Id", Value: pkID})

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
	})
}
