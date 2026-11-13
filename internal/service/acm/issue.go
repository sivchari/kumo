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
// issued certificate stores its own chain, so a CA regenerated after a
// restart does not invalidate certificates issued before it.
type localCA struct {
	cert    *x509.Certificate
	key     crypto.Signer
	certPEM string
}

// newLocalCA generates a self-signed CA certificate.
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
		NotBefore:             now.Add(-time.Hour),
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
func (s *MemoryStorage) issuingCA(now time.Time) (*localCA, error) {
	s.caMu.Lock()
	defer s.caMu.Unlock()

	if s.ca == nil {
		ca, err := newLocalCA(now)
		if err != nil {
			return nil, err
		}

		s.ca = ca
	}

	return s.ca, nil
}

// issue generates the certificate, chain and private key of an issued cert,
// signed by the storage's CA, and fills in the describe fields derived from
// them. A certificate that is not issued is left as is. cert must not be
// stored yet, and the caller must not hold s.mu: generating a key takes long
// enough to stall every operation waiting for the lock.
func (s *MemoryStorage) issue(cert *Certificate, now time.Time) error {
	if cert.Status != statusIssued {
		return nil
	}

	ca, err := s.issuingCA(now)
	if err != nil {
		return err
	}

	key, err := s.newKey(cert.KeyAlgorithm)
	if err != nil {
		return err
	}

	serial, err := randomSerial()
	if err != nil {
		return err
	}

	template := leafTemplate(cert, key, serial, now.Truncate(time.Second))

	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, key.Public(), ca.key)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	cert.CertificateBody = encodePEM(pemTypeCertificate, der)
	cert.CertificateChain = ca.certPEM
	cert.PrivateKey = encodePEM(pemTypePrivateKey, keyDER)
	cert.Serial = hex.EncodeToString(serial.Bytes())
	cert.Issuer = localCAName
	cert.SignatureAlgorithm = signatureAlgorithmECDSAWithSHA256
	cert.NotBefore = &template.NotBefore
	cert.NotAfter = &template.NotAfter

	return nil
}

// leafTemplate returns the template of the certificate issued for cert,
// covering its domain name and subject alternative names.
func leafTemplate(cert *Certificate, key crypto.Signer, serial *big.Int, notBefore time.Time) *x509.Certificate {
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
		NotBefore:    notBefore,
		NotAfter:     notBefore.Add(issuedValidity),
		KeyUsage:     keyUsage,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
}

// Key algorithms that RequestCertificate issues certificates with, as
// KeyAlgorithm values.
const (
	keyAlgorithmRSA2048      = "RSA_2048"
	keyAlgorithmECPrime256v1 = "EC_prime256v1"
	keyAlgorithmECSecp384r1  = "EC_secp384r1"
)

// keySpec describes the private key of a key algorithm: an RSA key of
// rsaBits bits, or an ECDSA key on curve.
type keySpec struct {
	rsaBits int
	curve   elliptic.Curve
}

// requestKeySpecs holds the key algorithms RequestCertificate issues
// certificates with. KeyAlgorithm also takes RSA_1024, RSA_3072, RSA_4096
// and EC_secp521r1, but the API reference supports only these three for a
// certificate request: "Other listed algorithms are for imported
// certificates only."
// https://docs.aws.amazon.com/acm/latest/APIReference/API_RequestCertificate.html
var requestKeySpecs = map[string]keySpec{
	keyAlgorithmRSA2048:      {rsaBits: 2048},
	keyAlgorithmECPrime256v1: {curve: elliptic.P256()},
	keyAlgorithmECSecp384r1:  {curve: elliptic.P384()},
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

// generateKey generates a private key for a key algorithm in
// requestKeySpecs.
func generateKey(keyAlgorithm string) (crypto.Signer, error) {
	spec, ok := requestKeySpecs[keyAlgorithm]
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
