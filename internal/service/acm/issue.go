package acm

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strings"
	"time"
)

// localCAName is the common name of the CA that signs the certificates kumo
// issues.
const localCAName = "kumo local CA"

// issuedValidity is how long a certificate issued by kumo stays valid.
const issuedValidity = 365 * 24 * time.Hour

// signatureAlgorithmECDSAWithSHA256 is the SignatureAlgorithm of the
// certificates kumo issues: the CA key is ECDSA P-256, which x509 signs with
// SHA-256.
const signatureAlgorithmECDSAWithSHA256 = "SHA256WITHECDSA"

// PEM block types of the issued material.
const (
	pemTypeCertificate = "CERTIFICATE"
	pemTypePrivateKey  = "PRIVATE KEY"
)

// localCA signs the certificates kumo issues. It lives only in memory: every
// certificate stores its own chain once its material is generated, so a CA
// regenerated after a restart does not invalidate that material.
type localCA struct {
	cert    *x509.Certificate
	key     crypto.Signer
	certPEM string
}

// newLocalCA generates a self-signed CA certificate. Its validity starts
// issuedValidity before now, so it covers every certificate that has not
// expired yet, including one issued before a restart whose material is
// generated after it.
func newLocalCA(now time.Time) (*localCA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: localCAName},
		NotBefore:             now.Add(-issuedValidity),
		NotAfter:              now.Add(10 * issuedValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	return &localCA{cert: cert, key: key, certPEM: encodePEM(pemTypeCertificate, der)}, nil
}

// issuingCA returns the storage's CA, generating it on first use.
func (s *MemoryStorage) issuingCA() (*localCA, error) {
	s.caMu.Lock()
	defer s.caMu.Unlock()

	if s.ca == nil {
		ca, err := newLocalCA(s.now())
		if err != nil {
			return nil, err
		}

		s.ca = ca
	}

	return s.ca, nil
}

// markIssued moves cert to ISSUED as of issuedAt, marks each domain's
// validation as succeeded, and fixes the describe fields of the certificate
// it will be issued as: serial number, issuer, signature algorithm and
// validity. The key and the certificate itself are generated later, on the
// first GetCertificate (see certificateMaterial), from these fields.
//
// cert must not be visible to other goroutines, or the caller must hold
// s.mu for writing. Slices are replaced rather than modified, so copies
// returned earlier keep their contents.
func markIssued(cert *Certificate, issuedAt time.Time) error {
	serial, err := randomSerial()
	if err != nil {
		return err
	}

	notBefore := issuedAt.Truncate(time.Second)
	notAfter := notBefore.Add(issuedValidity)

	validations := slices.Clone(cert.DomainValidationOptions)
	for i := range validations {
		validations[i].ValidationStatus = validationStatusSuccess
	}

	cert.Status = statusIssued
	cert.IssuedAt = &issuedAt
	cert.DomainValidationOptions = validations
	cert.Serial = hex.EncodeToString(serial.Bytes())
	cert.Issuer = localCAName
	cert.SignatureAlgorithm = signatureAlgorithmECDSAWithSHA256
	cert.NotBefore = &notBefore
	cert.NotAfter = &notAfter

	return nil
}

// material is the PEM-encoded certificate, chain and private key of an
// issued certificate.
type material struct {
	certificate string
	chain       string
	privateKey  string
}

// certificateMaterial generates the key of cert and the certificate signed
// by the storage's CA, matching the serial number and validity markIssued
// fixed. The caller must not hold s.mu: generating a key takes long enough
// to stall every operation waiting for the lock.
func (s *MemoryStorage) certificateMaterial(cert *Certificate) (*material, error) {
	ca, err := s.issuingCA()
	if err != nil {
		return nil, err
	}

	key, err := s.newKey(cert.KeyAlgorithm)
	if err != nil {
		return nil, err
	}

	template, err := leafTemplate(cert, key)
	if err != nil {
		return nil, err
	}

	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, key.Public(), ca.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	return &material{
		certificate: encodePEM(pemTypeCertificate, der),
		chain:       ca.certPEM,
		privateKey:  encodePEM(pemTypePrivateKey, keyDER),
	}, nil
}

// leafTemplate returns the template of the certificate issued for cert,
// covering its domain name and subject alternative names, with the serial
// number and validity markIssued fixed.
func leafTemplate(cert *Certificate, key crypto.Signer) (*x509.Certificate, error) {
	serial, ok := new(big.Int).SetString(cert.Serial, 16)
	if !ok || cert.NotBefore == nil || cert.NotAfter == nil {
		return nil, fmt.Errorf("certificate %s has no issued serial number and validity", cert.CertificateArn)
	}

	dnsNames := []string{cert.DomainName}
	for _, san := range cert.SubjectAlternativeNames {
		if !slices.Contains(dnsNames, san) {
			dnsNames = append(dnsNames, san)
		}
	}

	keyUsage := x509.KeyUsageDigitalSignature
	if _, ok := key.(*rsa.PrivateKey); ok {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}

	return &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cert.DomainName},
		DNSNames:     dnsNames,
		NotBefore:    *cert.NotBefore,
		NotAfter:     *cert.NotAfter,
		KeyUsage:     keyUsage,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}, nil
}

// KeyAlgorithm values besides keyAlgorithmRSA2048.
const (
	keyAlgorithmRSA1024      = "RSA_1024"
	keyAlgorithmRSA3072      = "RSA_3072"
	keyAlgorithmRSA4096      = "RSA_4096"
	keyAlgorithmECPrime256v1 = "EC_prime256v1"
	keyAlgorithmECSecp384r1  = "EC_secp384r1"
	keyAlgorithmECSecp521r1  = "EC_secp521r1"
)

// keySpec describes the private key of a key algorithm: an RSA key of
// rsaBits bits, or an ECDSA key on curve.
type keySpec struct {
	rsaBits int
	curve   elliptic.Curve
}

// keySpecs holds every KeyAlgorithm value, so material can be generated for
// any stored certificate, including one a snapshot written by an older kumo
// holds with an algorithm RequestCertificate no longer accepts.
// https://docs.aws.amazon.com/acm/latest/APIReference/API_CertificateDetail.html
var keySpecs = map[string]keySpec{
	keyAlgorithmRSA1024:      {rsaBits: 1024},
	keyAlgorithmRSA2048:      {rsaBits: 2048},
	keyAlgorithmRSA3072:      {rsaBits: 3072},
	keyAlgorithmRSA4096:      {rsaBits: 4096},
	keyAlgorithmECPrime256v1: {curve: elliptic.P256()},
	keyAlgorithmECSecp384r1:  {curve: elliptic.P384()},
	keyAlgorithmECSecp521r1:  {curve: elliptic.P521()},
}

// requestKeySpecs holds the key algorithms RequestCertificate issues
// certificates with, a subset of keySpecs. The API reference supports only
// these three for a certificate request: "Other listed algorithms are for
// imported certificates only."
// https://docs.aws.amazon.com/acm/latest/APIReference/API_RequestCertificate.html
var requestKeySpecs = map[string]struct{}{
	keyAlgorithmRSA2048:      {},
	keyAlgorithmECPrime256v1: {},
	keyAlgorithmECSecp384r1:  {},
}

// requestKeyAlgorithm returns the key algorithm a requested certificate is
// issued with: keyAlgorithm, or RSA_2048 when it is empty, the documented
// default. Any other value, whether it is not a KeyAlgorithm value or is one
// for imported certificates only, is rejected rather than issued with a key
// that differs from the algorithm the certificate reports.
func requestKeyAlgorithm(keyAlgorithm string) (string, error) {
	if keyAlgorithm == "" {
		return keyAlgorithmRSA2048, nil
	}

	if _, ok := requestKeySpecs[keyAlgorithm]; !ok {
		return "", &Error{
			Code: errInvalidParameter,
			Message: fmt.Sprintf("KeyAlgorithm %q is not supported for a certificate request; supported values are %s",
				keyAlgorithm, strings.Join(slices.Sorted(maps.Keys(requestKeySpecs)), ", ")),
		}
	}

	return keyAlgorithm, nil
}

// generateKey generates a private key for any KeyAlgorithm value.
func generateKey(keyAlgorithm string) (crypto.Signer, error) {
	spec, ok := keySpecs[keyAlgorithm]
	if !ok {
		return nil, fmt.Errorf("unsupported key algorithm %q", keyAlgorithm)
	}

	var (
		key crypto.Signer
		err error
	)

	if spec.curve != nil {
		key, err = ecdsa.GenerateKey(spec.curve, rand.Reader)
	} else {
		key, err = rsa.GenerateKey(rand.Reader, spec.rsaBits)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate %s key: %w", keyAlgorithm, err)
	}

	return key, nil
}

// randomSerial returns a positive random 128-bit certificate serial number.
func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial: %w", err)
	}

	return serial.Add(serial, big.NewInt(1)), nil
}

// encodePEM encodes der as a PEM block of the given type.
func encodePEM(blockType string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}))
}
