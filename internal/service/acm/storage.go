package acm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errNotFound         = "ResourceNotFoundException"
	errInvalidParameter = "ValidationException"
)

// Storage defines the interface for ACM storage operations.
type Storage interface {
	RequestCertificate(ctx context.Context, req *RequestCertificateInput) (*Certificate, error)
	DescribeCertificate(ctx context.Context, arn string) (*Certificate, error)
	ListCertificates(ctx context.Context, statuses []string, maxItems int32, nextToken string) ([]*Certificate, string, error)
	DeleteCertificate(ctx context.Context, arn string) error
	GetCertificate(ctx context.Context, arn string) (*Certificate, error)
	ImportCertificate(ctx context.Context, req *ImportCertificateInput) (*Certificate, error)
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data structures.
type MemoryStorage struct {
	mu           sync.RWMutex            `json:"-"`
	Certificates map[string]*Certificate `json:"certificates"`
	dataDir      string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Certificates: make(map[string]*Certificate),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "acm", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(s)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Certificates == nil {
		s.Certificates = make(map[string]*Certificate)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(s)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(s.dataDir, "acm", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "acm", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// buildDomainValidations creates domain validation options for the certificate.
func buildDomainValidations(req *RequestCertificateInput, certID string) []DomainValidation {
	domains := make([]string, 0, 1+len(req.SubjectAlternativeNames))
	domains = append(domains, req.DomainName)
	domains = append(domains, req.SubjectAlternativeNames...)

	validationMethod := req.ValidationMethod
	if validationMethod == "" {
		validationMethod = "DNS"
	}

	validations := make([]DomainValidation, 0, len(domains))

	for _, domain := range domains {
		dv := DomainValidation{
			DomainName:       domain,
			ValidationDomain: domain,
			ValidationStatus: "PENDING_VALIDATION",
			ValidationMethod: validationMethod,
		}

		if validationMethod == "DNS" {
			dv.ResourceRecord = &ResourceRecord{
				Name:  fmt.Sprintf("_acme-challenge.%s", domain),
				Type:  "CNAME",
				Value: fmt.Sprintf("_%s.acm-validations.aws.", certID[:8]),
			}
		}

		validations = append(validations, dv)
	}

	return validations
}

// RequestCertificate requests a new certificate.
func (s *MemoryStorage) RequestCertificate(_ context.Context, req *RequestCertificateInput) (*Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.DomainName == "" {
		return nil, &Error{
			Code:    errInvalidParameter,
			Message: "DomainName is required",
		}
	}

	certID := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:acm:us-east-1:000000000000:certificate/%s", certID)

	serialBytes := make([]byte, 16)
	if _, err := rand.Read(serialBytes); err != nil {
		return nil, fmt.Errorf("failed to generate serial: %w", err)
	}

	keyAlgorithm := req.KeyAlgorithm
	if keyAlgorithm == "" {
		keyAlgorithm = "RSA_2048"
	}

	now := time.Now()
	cert := &Certificate{
		CertificateArn:          arn,
		DomainName:              req.DomainName,
		SubjectAlternativeNames: req.SubjectAlternativeNames,
		Status:                  "PENDING_VALIDATION",
		Type:                    "AMAZON_ISSUED",
		KeyAlgorithm:            keyAlgorithm,
		Serial:                  hex.EncodeToString(serialBytes),
		Subject:                 fmt.Sprintf("CN=%s", req.DomainName),
		CreatedAt:               now,
		DomainValidationOptions: buildDomainValidations(req, certID),
		RenewalEligibility:      "INELIGIBLE",
		Options:                 req.Options,
		Tags:                    req.Tags,
	}

	s.Certificates[arn] = cert

	s.saveLocked()

	return cert, nil
}

// DescribeCertificate retrieves certificate details.
func (s *MemoryStorage) DescribeCertificate(_ context.Context, arn string) (*Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cert, exists := s.Certificates[arn]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Certificate with arn %s not found", arn),
		}
	}

	return cert, nil
}

// ListCertificates lists certificates with optional filtering.
func (s *MemoryStorage) ListCertificates(_ context.Context, statuses []string, maxItems int32, _ string) ([]*Certificate, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxItems <= 0 {
		maxItems = 100
	}

	certs := make([]*Certificate, 0, len(s.Certificates))

	for _, cert := range s.Certificates {
		// Filter by status if specified.
		if len(statuses) > 0 && !slices.Contains(statuses, cert.Status) {
			continue
		}

		certs = append(certs, cert)
	}

	// Apply pagination.
	if len(certs) > int(maxItems) {
		certs = certs[:maxItems]
	}

	return certs, "", nil
}

// DeleteCertificate deletes a certificate.
func (s *MemoryStorage) DeleteCertificate(_ context.Context, arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Certificates[arn]; !exists {
		return &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Certificate with arn %s not found", arn),
		}
	}

	delete(s.Certificates, arn)

	s.saveLocked()

	return nil
}

// GetCertificate retrieves certificate and chain.
func (s *MemoryStorage) GetCertificate(_ context.Context, arn string) (*Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cert, exists := s.Certificates[arn]
	if !exists {
		return nil, &Error{
			Code:    errNotFound,
			Message: fmt.Sprintf("Certificate with arn %s not found", arn),
		}
	}

	// Only issued or imported certificates can be retrieved.
	if cert.Status != "ISSUED" && cert.Type != "IMPORTED" {
		return nil, &Error{
			Code:    errNotFound,
			Message: "Certificate is not issued yet",
		}
	}

	return cert, nil
}

// ImportCertificate imports a certificate.
func (s *MemoryStorage) ImportCertificate(_ context.Context, req *ImportCertificateInput) (*Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.Certificate) == 0 {
		return nil, &Error{
			Code:    errInvalidParameter,
			Message: "Certificate is required",
		}
	}

	if len(req.PrivateKey) == 0 {
		return nil, &Error{
			Code:    errInvalidParameter,
			Message: "PrivateKey is required",
		}
	}

	arn := req.CertificateArn
	if arn == "" {
		// Generate new ARN.
		certID := uuid.New().String()
		arn = fmt.Sprintf("arn:aws:acm:us-east-1:000000000000:certificate/%s", certID)
	}

	// Generate serial number.
	serialBytes := make([]byte, 16)
	if _, err := rand.Read(serialBytes); err != nil {
		return nil, fmt.Errorf("failed to generate serial: %w", err)
	}

	serial := hex.EncodeToString(serialBytes)
	now := time.Now()

	// Parse certificate to extract domain (simplified - just use a placeholder).
	domainName := "imported.example.com"

	cert := &Certificate{
		CertificateArn:     arn,
		DomainName:         domainName,
		Status:             "ISSUED",
		Type:               "IMPORTED",
		KeyAlgorithm:       "RSA_2048",
		Serial:             serial,
		Subject:            fmt.Sprintf("CN=%s", domainName),
		CreatedAt:          now,
		ImportedAt:         &now,
		IssuedAt:           &now,
		CertificateBody:    string(req.Certificate),
		CertificateChain:   string(req.CertificateChain),
		PrivateKey:         string(req.PrivateKey),
		RenewalEligibility: "INELIGIBLE",
		InUseBy:            []string{},
		Tags:               req.Tags,
	}

	s.Certificates[arn] = cert

	s.saveLocked()

	return cert, nil
}
