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
	"math/big"
	"slices"
	"time"
)

// localCAName is the common name of the CA that signs the certificates kumo
// issues.
const localCAName = "kumo local CA"

// issuedValidity is how long a certificate issued by kumo stays valid.
const issuedValidity = 365 * 24 * time.Hour

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

	return &localCA{cert: cert, key: key, certPEM: encodePEM("CERTIFICATE", der)}, nil
}

// caLocked returns the storage's CA, generating it on first use. The caller
// must hold s.mu for writing.
func (s *MemoryStorage) caLocked(now time.Time) (*localCA, error) {
	if s.ca == nil {
		ca, err := newLocalCA(now)
		if err != nil {
			return nil, err
		}

		s.ca = ca
	}

	return s.ca, nil
}

// issueLocked generates the certificate, chain and private key of an issued
// cert, signed by the storage's CA, and fills in the describe fields derived
// from them. A certificate that is not issued is left as is. The caller must
// hold s.mu for writing.
func (s *MemoryStorage) issueLocked(cert *Certificate, now time.Time) error {
	if cert.Status != statusIssued {
		return nil
	}

	ca, err := s.caLocked(now)
	if err != nil {
		return err
	}

	key, err := generateKey(cert.KeyAlgorithm)
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

	cert.CertificateBody = encodePEM("CERTIFICATE", der)
	cert.CertificateChain = ca.certPEM
	cert.PrivateKey = encodePEM("PRIVATE KEY", keyDER)
	cert.Serial = hex.EncodeToString(serial.Bytes())
	cert.Issuer = localCAName
	cert.SignatureAlgorithm = "SHA256WITHECDSA"
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

// generateKey generates a private key for an ACM key algorithm, defaulting
// to RSA 2048 like RequestCertificate.
func generateKey(keyAlgorithm string) (crypto.Signer, error) {
	var (
		key crypto.Signer
		err error
	)

	switch keyAlgorithm {
	case "RSA_1024":
		key, err = rsa.GenerateKey(rand.Reader, 1024) //nolint:gosec // requested key size
	case "RSA_3072":
		key, err = rsa.GenerateKey(rand.Reader, 3072)
	case "RSA_4096":
		key, err = rsa.GenerateKey(rand.Reader, 4096)
	case "EC_prime256v1":
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "EC_secp384r1":
		key, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "EC_secp521r1":
		key, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		key, err = rsa.GenerateKey(rand.Reader, 2048)
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
