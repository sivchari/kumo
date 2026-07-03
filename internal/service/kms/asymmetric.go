package kms

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	_ "crypto/sha256" // register SHA-256 for crypto.Hash.New
	_ "crypto/sha512" // register SHA-384/512 for crypto.Hash.New
	"crypto/x509"
	"fmt"
	"strings"
)

// isAsymmetricSpec reports whether the key spec is an asymmetric (RSA/ECC) spec.
func isAsymmetricSpec(spec KeySpec) bool {
	switch spec { //nolint:exhaustive // only RSA/ECC specs are asymmetric; the rest default to false
	case KeySpecRSA2048, KeySpecRSA3072, KeySpecRSA4096,
		KeySpecECCNistP256, KeySpecECCNistP384, KeySpecECCNistP521:
		return true
	default:
		return false
	}
}

// isRSASpec reports whether the key spec is an RSA spec.
func isRSASpec(spec KeySpec) bool {
	switch spec { //nolint:exhaustive // only RSA specs match; the rest default to false
	case KeySpecRSA2048, KeySpecRSA3072, KeySpecRSA4096:
		return true
	default:
		return false
	}
}

// generateAsymmetricKey generates a private key for the given asymmetric spec
// and returns it PKCS#8 DER-encoded.
func generateAsymmetricKey(spec KeySpec) ([]byte, error) {
	var priv crypto.PrivateKey

	switch spec { //nolint:exhaustive // non-asymmetric specs are rejected by the default case
	case KeySpecRSA2048:
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate RSA 2048 key: %w", err)
		}

		priv = k
	case KeySpecRSA3072:
		k, err := rsa.GenerateKey(rand.Reader, 3072)
		if err != nil {
			return nil, fmt.Errorf("generate RSA 3072 key: %w", err)
		}

		priv = k
	case KeySpecRSA4096:
		k, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, fmt.Errorf("generate RSA 4096 key: %w", err)
		}

		priv = k
	case KeySpecECCNistP256:
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ECC P256 key: %w", err)
		}

		priv = k
	case KeySpecECCNistP384:
		k, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ECC P384 key: %w", err)
		}

		priv = k
	case KeySpecECCNistP521:
		k, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("generate ECC P521 key: %w", err)
		}

		priv = k
	default:
		return nil, fmt.Errorf("unsupported key spec %q", spec)
	}

	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}

	return der, nil
}

// hashForAlgorithm returns the hash function implied by a signing algorithm.
func hashForAlgorithm(algo SigningAlgorithm) (crypto.Hash, error) {
	switch {
	case strings.HasSuffix(string(algo), "SHA_256"):
		return crypto.SHA256, nil
	case strings.HasSuffix(string(algo), "SHA_384"):
		return crypto.SHA384, nil
	case strings.HasSuffix(string(algo), "SHA_512"):
		return crypto.SHA512, nil
	default:
		return 0, &ServiceError{
			Code:    errInvalidKeyUsage,
			Message: fmt.Sprintf("Unsupported signing algorithm %q", algo),
		}
	}
}

// digestMessage returns the digest to sign/verify. When messageType is DIGEST
// the message is already hashed and used as-is; otherwise it is hashed with the
// algorithm's hash function.
func digestMessage(message []byte, h crypto.Hash, messageType MessageType) []byte {
	if messageType == MessageTypeDigest {
		return message
	}

	hasher := h.New()
	hasher.Write(message)

	return hasher.Sum(nil)
}

// signDigest signs a message with the PKCS#8 DER private key using the algorithm.
func signDigest(der, message []byte, algo SigningAlgorithm, messageType MessageType) ([]byte, error) {
	h, err := hashForAlgorithm(algo)
	if err != nil {
		return nil, err
	}

	priv, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	digest := digestMessage(message, h, messageType)
	isRSA := strings.HasPrefix(string(algo), "RSASSA")

	switch key := priv.(type) {
	case *rsa.PrivateKey:
		if !isRSA {
			return nil, &ServiceError{Code: errInvalidKeyUsage, Message: "Signing algorithm is incompatible with the RSA key."}
		}

		if strings.HasPrefix(string(algo), "RSASSA_PSS") {
			sig, err := rsa.SignPSS(rand.Reader, key, h, digest, &rsa.PSSOptions{
				SaltLength: rsa.PSSSaltLengthEqualsHash,
				Hash:       h,
			})
			if err != nil {
				return nil, fmt.Errorf("sign RSA-PSS: %w", err)
			}

			return sig, nil
		}

		sig, err := rsa.SignPKCS1v15(rand.Reader, key, h, digest)
		if err != nil {
			return nil, fmt.Errorf("sign RSA-PKCS1v15: %w", err)
		}

		return sig, nil
	case *ecdsa.PrivateKey:
		if isRSA {
			return nil, &ServiceError{Code: errInvalidKeyUsage, Message: "Signing algorithm is incompatible with the ECC key."}
		}

		sig, err := ecdsa.SignASN1(rand.Reader, key, digest)
		if err != nil {
			return nil, fmt.Errorf("sign ECDSA: %w", err)
		}

		return sig, nil
	default:
		return nil, &ServiceError{Code: errInvalidKeyUsage, Message: "Key type does not support signing."}
	}
}

// verifyDigest verifies a signature against a message using the PKCS#8 DER private key.
func verifyDigest(der, message, signature []byte, algo SigningAlgorithm, messageType MessageType) (bool, error) {
	h, err := hashForAlgorithm(algo)
	if err != nil {
		return false, err
	}

	priv, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return false, fmt.Errorf("parse private key: %w", err)
	}

	digest := digestMessage(message, h, messageType)
	isRSA := strings.HasPrefix(string(algo), "RSASSA")

	switch key := priv.(type) {
	case *rsa.PrivateKey:
		if !isRSA {
			return false, &ServiceError{Code: errInvalidKeyUsage, Message: "Signing algorithm is incompatible with the RSA key."}
		}

		if strings.HasPrefix(string(algo), "RSASSA_PSS") {
			err = rsa.VerifyPSS(&key.PublicKey, h, digest, signature, &rsa.PSSOptions{
				SaltLength: rsa.PSSSaltLengthAuto,
				Hash:       h,
			})
		} else {
			err = rsa.VerifyPKCS1v15(&key.PublicKey, h, digest, signature)
		}

		return err == nil, nil
	case *ecdsa.PrivateKey:
		if isRSA {
			return false, &ServiceError{Code: errInvalidKeyUsage, Message: "Signing algorithm is incompatible with the ECC key."}
		}

		return ecdsa.VerifyASN1(&key.PublicKey, digest, signature), nil
	default:
		return false, &ServiceError{Code: errInvalidKeyUsage, Message: "Key type does not support verification."}
	}
}

// publicKeyDER returns the DER-encoded SubjectPublicKeyInfo for the key's public part.
func publicKeyDER(der []byte) ([]byte, error) {
	priv, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	signer, ok := priv.(crypto.Signer)
	if !ok {
		return nil, &ServiceError{Code: errInvalidKeyUsage, Message: "Key does not expose a public key."}
	}

	pub, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}

	return pub, nil
}

// signingAlgorithmsForSpec returns the signing algorithms supported by a key spec.
func signingAlgorithmsForSpec(spec KeySpec) []string {
	switch spec { //nolint:exhaustive // only asymmetric signing specs have signing algorithms; others return nil
	case KeySpecRSA2048, KeySpecRSA3072, KeySpecRSA4096:
		return []string{
			string(SigningAlgorithmRSASSAPSSSha256), string(SigningAlgorithmRSASSAPSSSha384), string(SigningAlgorithmRSASSAPSSSha512),
			string(SigningAlgorithmRSASSAPKCS1V15Sha256), string(SigningAlgorithmRSASSAPKCS1V15Sha384), string(SigningAlgorithmRSASSAPKCS1V15Sha512),
		}
	case KeySpecECCNistP256:
		return []string{string(SigningAlgorithmECDSASha256)}
	case KeySpecECCNistP384:
		return []string{string(SigningAlgorithmECDSASha384)}
	case KeySpecECCNistP521:
		return []string{string(SigningAlgorithmECDSASha512)}
	default:
		return nil
	}
}

// encryptionAlgorithmsForSpec returns the encryption algorithms supported by a key spec.
func encryptionAlgorithmsForSpec(spec KeySpec) []string {
	if isRSASpec(spec) {
		return []string{string(EncryptionAlgorithmRSAESOAEPSha1), string(EncryptionAlgorithmRSAESOAEPSha256)}
	}

	return nil
}
