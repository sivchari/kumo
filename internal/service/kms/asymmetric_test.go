package kms

import (
	"bytes"
	"errors"
	"testing"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec KeySpec
		algo SigningAlgorithm
	}{
		{"RSA2048 PKCS1 SHA256", KeySpecRSA2048, SigningAlgorithmRSASSAPKCS1V15Sha256},
		{"RSA2048 PKCS1 SHA384", KeySpecRSA2048, SigningAlgorithmRSASSAPKCS1V15Sha384},
		{"RSA2048 PKCS1 SHA512", KeySpecRSA2048, SigningAlgorithmRSASSAPKCS1V15Sha512},
		{"RSA2048 PSS SHA256", KeySpecRSA2048, SigningAlgorithmRSASSAPSSSha256},
		{"RSA3072 PSS SHA384", KeySpecRSA3072, SigningAlgorithmRSASSAPSSSha384},
		{"ECC P256", KeySpecECCNistP256, SigningAlgorithmECDSASha256},
		{"ECC P384", KeySpecECCNistP384, SigningAlgorithmECDSASha384},
		{"ECC P521", KeySpecECCNistP521, SigningAlgorithmECDSASha512},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			der, err := generateAsymmetricKey(tt.spec)
			if err != nil {
				t.Fatalf("generateAsymmetricKey: %v", err)
			}

			message := []byte("the quick brown fox")

			sig, err := signDigest(der, message, tt.algo, MessageTypeRaw)
			if err != nil {
				t.Fatalf("signDigest: %v", err)
			}

			valid, err := verifyDigest(der, message, sig, tt.algo, MessageTypeRaw)
			if err != nil {
				t.Fatalf("verifyDigest: %v", err)
			}

			if !valid {
				t.Error("expected signature to verify")
			}

			// A tampered message must not verify.
			tampered, err := verifyDigest(der, []byte("tampered"), sig, tt.algo, MessageTypeRaw)
			if err != nil {
				t.Fatalf("verifyDigest tampered: %v", err)
			}

			if tampered {
				t.Error("expected tampered message to fail verification")
			}
		})
	}
}

func TestSignDigest_AlgorithmKeyMismatch(t *testing.T) {
	t.Parallel()

	rsaKey, err := generateAsymmetricKey(KeySpecRSA2048)
	if err != nil {
		t.Fatal(err)
	}

	eccKey, err := generateAsymmetricKey(KeySpecECCNistP256)
	if err != nil {
		t.Fatal(err)
	}

	// RSA algorithm against an ECC key must be rejected.
	if _, err := signDigest(eccKey, []byte("msg"), SigningAlgorithmRSASSAPSSSha256, MessageTypeRaw); err == nil {
		t.Error("expected error signing ECC key with RSA algorithm")
	}

	// ECDSA algorithm against an RSA key must be rejected.
	if _, err := signDigest(rsaKey, []byte("msg"), SigningAlgorithmECDSASha256, MessageTypeRaw); err == nil {
		t.Error("expected error signing RSA key with ECDSA algorithm")
	}
}

func TestHashForAlgorithm_Unsupported(t *testing.T) {
	t.Parallel()

	_, err := hashForAlgorithm(SigningAlgorithm("BOGUS_ALGO"))

	var svcErr *ServiceError
	if !errors.As(err, &svcErr) || svcErr.Code != errInvalidKeyUsage {
		t.Errorf("expected InvalidKeyUsage ServiceError, got %v", err)
	}
}

func TestDigestMessage_DigestPassthrough(t *testing.T) {
	t.Parallel()

	// When MessageType is DIGEST, the message is returned unchanged.
	digest := []byte("already-a-digest")

	got := digestMessage(digest, 0, MessageTypeDigest)
	if !bytes.Equal(got, digest) {
		t.Errorf("expected passthrough, got %q", got)
	}
}
