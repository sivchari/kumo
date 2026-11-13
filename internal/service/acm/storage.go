package acm

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errNotFound         = "ResourceNotFoundException"
	errInvalidParameter = "ValidationException"
	errInvalidTag       = "InvalidTagException"
	errTooManyTags      = "TooManyTagsException"
	errRequestPending   = "RequestInProgressException"
)

// maxTagsPerCertificate is the ACM quota on tags per certificate.
// https://docs.aws.amazon.com/acm/latest/userguide/tags-restrictions.html
const maxTagsPerCertificate = 50

// Length limits, in characters, of a tag key and value. The API reference's
// Tag data type allows keys of 1 to 128 characters and values of up to 256;
// the user guide's tag restrictions give 127 and 255, and kumo uses the
// API reference's limits so it never rejects a tag the API reference allows.
// https://docs.aws.amazon.com/acm/latest/APIReference/API_Tag.html
const (
	maxTagKeyLength   = 128
	maxTagValueLength = 256
)

// reservedTagPrefix marks tag keys reserved for AWS use, which callers cannot
// add, edit or delete.
const reservedTagPrefix = "aws:"

// Certificate and domain validation statuses.
const (
	statusPendingValidation = "PENDING_VALIDATION"
	statusIssued            = "ISSUED"
	validationStatusSuccess = "SUCCESS"
	validationMethodDNS     = "DNS"
)

// accountID is the AWS account ID kumo reports for every certificate.
const accountID = "000000000000"

// defaultRegion is the region of certificate ARNs when AWS_DEFAULT_REGION
// is not set.
const defaultRegion = "us-east-1"

// CertificateType values: a certificate imported via ImportCertificate, or
// one issued by ACM itself.
const (
	certificateTypeImported     = "IMPORTED"
	certificateTypeAmazonIssued = "AMAZON_ISSUED"
)

// renewalIneligible is the RenewalEligibility of every certificate, since
// kumo does not renew certificates.
const renewalIneligible = "INELIGIBLE"

// resourceRecordTypeCNAME is the type of DNS validation records.
const resourceRecordTypeCNAME = "CNAME"

// Storage defines the interface for ACM storage operations.
type Storage interface {
	RequestCertificate(ctx context.Context, req *RequestCertificateInput) (*Certificate, error)
	DescribeCertificate(ctx context.Context, arn string) (*Certificate, error)
	ListCertificates(ctx context.Context, statuses []string, maxItems int32, nextToken string) ([]*Certificate, string, error)
	DeleteCertificate(ctx context.Context, arn string) error
	GetCertificate(ctx context.Context, arn string) (*Certificate, error)
	ImportCertificate(ctx context.Context, req *ImportCertificateInput) (*Certificate, error)
	ListTagsForCertificate(ctx context.Context, arn string) ([]Tag, error)
	AddTagsToCertificate(ctx context.Context, arn string, tags []Tag) error
	RemoveTagsFromCertificate(ctx context.Context, arn string, tags []TagToRemove) error
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
	region       string
	// caMu guards ca. Certificates are issued without holding mu, so the CA
	// has a lock of its own.
	caMu sync.Mutex
	ca   *localCA
	// newKey generates the private key of an issued certificate; tests
	// replace it to check that it runs without mu held.
	newKey func(keyAlgorithm string) (crypto.Signer, error)
}

// NewMemoryStorage creates a new in-memory storage. Certificate ARNs carry
// the region from AWS_DEFAULT_REGION, or defaultRegion when it is not set.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Certificates: make(map[string]*Certificate),
		region:       region,
		newKey:       generateKey,
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

	storage.ScheduleSave(s.dataDir, "acm", s.MarshalJSON)
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
// DNS validation is reported as already succeeded (see initialStatus).
func buildDomainValidations(req *RequestCertificateInput, validationMethod string) []DomainValidation {
	domains := make([]string, 0, 1+len(req.SubjectAlternativeNames))
	domains = append(domains, req.DomainName)
	domains = append(domains, req.SubjectAlternativeNames...)

	validationStatus := statusPendingValidation
	if validationMethod == validationMethodDNS {
		validationStatus = validationStatusSuccess
	}

	validations := make([]DomainValidation, 0, len(domains))

	for _, domain := range domains {
		dv := DomainValidation{
			DomainName:       domain,
			ValidationDomain: domain,
			ValidationStatus: validationStatus,
			ValidationMethod: validationMethod,
		}

		if validationMethod == validationMethodDNS {
			dv.ResourceRecord = validationRecord(domain)
		}

		validations = append(validations, dv)
	}

	return validations
}

// validationRecord returns the DNS validation CNAME for domain. ACM creates
// the record for the domain and the account, so the same name and value are
// returned for every certificate covering the domain, and a wildcard domain
// shares the record of its base domain.
// https://docs.aws.amazon.com/acm/latest/userguide/dns-validation.html
func validationRecord(domain string) *ResourceRecord {
	base := strings.ToLower(strings.TrimPrefix(domain, "*."))

	token := func(kind string) string {
		sum := sha256.Sum256([]byte(kind + "/" + accountID + "/" + base))

		return hex.EncodeToString(sum[:16])
	}

	return &ResourceRecord{
		Name:  fmt.Sprintf("_%s.%s.", token("name"), base),
		Type:  resourceRecordTypeCNAME,
		Value: fmt.Sprintf("_%s.acm-validations.aws.", token("value")),
	}
}

// RequestCertificate requests a new certificate. An issued certificate gets
// its key and certificate before the storage is locked, so generating them
// does not block the other operations, and it is stored together with that
// material, so it is never visible as ISSUED without it.
func (s *MemoryStorage) RequestCertificate(_ context.Context, req *RequestCertificateInput) (*Certificate, error) {
	if req.DomainName == "" {
		return nil, &Error{
			Code:    errInvalidParameter,
			Message: "DomainName is required",
		}
	}

	keyAlgorithm, err := requestKeyAlgorithm(req.KeyAlgorithm)
	if err != nil {
		return nil, err
	}

	tags, err := mergeTags(nil, req.Tags)
	if err != nil {
		return nil, err
	}

	serialBytes := make([]byte, 16)
	if _, err := rand.Read(serialBytes); err != nil {
		return nil, fmt.Errorf("failed to generate serial: %w", err)
	}

	validationMethod := req.ValidationMethod
	if validationMethod == "" {
		validationMethod = validationMethodDNS
	}

	now := time.Now()
	status, issuedAt := initialStatus(validationMethod, now)

	cert := &Certificate{
		CertificateArn:          s.newCertificateARN(),
		DomainName:              req.DomainName,
		SubjectAlternativeNames: req.SubjectAlternativeNames,
		Status:                  status,
		Type:                    certificateTypeAmazonIssued,
		KeyAlgorithm:            keyAlgorithm,
		Serial:                  hex.EncodeToString(serialBytes),
		Subject:                 fmt.Sprintf("CN=%s", req.DomainName),
		CreatedAt:               now,
		IssuedAt:                issuedAt,
		DomainValidationOptions: buildDomainValidations(req, validationMethod),
		RenewalEligibility:      renewalIneligible,
		Options:                 req.Options,
		Tags:                    tags,
	}

	if err := s.issue(cert, now); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Certificates[cert.CertificateArn] = cert

	s.saveLocked()

	return cert, nil
}

// newCertificateARN generates the ARN of a new certificate in the storage's
// region.
func (s *MemoryStorage) newCertificateARN() string {
	return fmt.Sprintf("arn:aws:acm:%s:%s:certificate/%s", s.region, accountID, uuid.New().String())
}

// initialStatus returns the status and issue time of a newly requested
// certificate. kumo cannot observe DNS, so a DNS-validated certificate is
// issued immediately instead of waiting for its validation CNAME records:
// clients such as the Terraform AWS provider's aws_acm_certificate_validation
// block until the certificate is ISSUED. Email validation stays pending.
func initialStatus(validationMethod string, now time.Time) (string, *time.Time) {
	if validationMethod == validationMethodDNS {
		return statusIssued, &now
	}

	return statusPendingValidation, nil
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
		if len(statuses) > 0 && !slices.Contains(statuses, cert.Status) {
			continue
		}

		certs = append(certs, cert)
	}

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

	// Only imported certificates and issued certificates with their material
	// can be retrieved; RequestCertificate generates the material when it
	// issues a certificate.
	issued := cert.Status == statusIssued && cert.CertificateBody != ""
	if !issued && cert.Type != certificateTypeImported {
		return nil, &Error{
			Code:    errRequestPending,
			Message: fmt.Sprintf("Certificate with arn %s is not yet issued", arn),
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

	tags, err := mergeTags(nil, req.Tags)
	if err != nil {
		return nil, err
	}

	arn := req.CertificateArn
	if arn == "" {
		arn = s.newCertificateARN()
	}

	serialBytes := make([]byte, 16)
	if _, err := rand.Read(serialBytes); err != nil {
		return nil, fmt.Errorf("failed to generate serial: %w", err)
	}

	now := time.Now()

	// Parse certificate to extract domain (simplified - just use a placeholder).
	domainName := "imported.example.com"

	cert := &Certificate{
		CertificateArn:     arn,
		DomainName:         domainName,
		Status:             statusIssued,
		Type:               certificateTypeImported,
		KeyAlgorithm:       keyAlgorithmRSA2048,
		Serial:             hex.EncodeToString(serialBytes),
		Subject:            fmt.Sprintf("CN=%s", domainName),
		CreatedAt:          now,
		ImportedAt:         &now,
		IssuedAt:           &now,
		CertificateBody:    string(req.Certificate),
		CertificateChain:   string(req.CertificateChain),
		PrivateKey:         string(req.PrivateKey),
		RenewalEligibility: renewalIneligible,
		InUseBy:            []string{},
		Tags:               tags,
	}

	s.Certificates[arn] = cert

	s.saveLocked()

	return cert, nil
}

// ListTagsForCertificate returns the tags applied to a certificate.
func (s *MemoryStorage) ListTagsForCertificate(_ context.Context, arn string) ([]Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cert, exists := s.Certificates[arn]
	if !exists {
		return nil, certificateNotFound(arn)
	}

	return slices.Clone(cert.Tags), nil
}

// AddTagsToCertificate adds tags to a certificate, overwriting the value of
// any key that is already present.
func (s *MemoryStorage) AddTagsToCertificate(_ context.Context, arn string, tags []Tag) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cert, exists := s.Certificates[arn]
	if !exists {
		return certificateNotFound(arn)
	}

	merged, err := mergeTags(cert.Tags, tags)
	if err != nil {
		return err
	}

	cert.Tags = merged

	s.saveLocked()

	return nil
}

// RemoveTagsFromCertificate removes tags from a certificate. A tag given
// without a value is removed regardless of its value; a tag given with a
// value is removed only when the value matches.
func (s *MemoryStorage) RemoveTagsFromCertificate(_ context.Context, arn string, tags []TagToRemove) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cert, exists := s.Certificates[arn]
	if !exists {
		return certificateNotFound(arn)
	}

	// Validate every tag before removing any, so an invalid call changes
	// nothing.
	for _, tag := range tags {
		var value string
		if tag.Value != nil {
			value = *tag.Value
		}

		if err := validateTag(tag.Key, value); err != nil {
			return err
		}
	}

	cert.Tags = slices.DeleteFunc(slices.Clone(cert.Tags), func(t Tag) bool {
		return slices.ContainsFunc(tags, func(r TagToRemove) bool {
			return r.Key == t.Key && (r.Value == nil || *r.Value == t.Value)
		})
	})

	s.saveLocked()

	return nil
}

// mergeTags returns existing with tags applied in order, the way every
// tagging call stores tags: a key that is already present, in existing or
// earlier in tags, takes the new value, and any other key is appended. It
// rejects tags that validateTags refuses and a result with more tags than a
// certificate may carry, so a key given twice counts once.
func mergeTags(existing, tags []Tag) ([]Tag, error) {
	if err := validateTags(tags); err != nil {
		return nil, err
	}

	merged := slices.Clone(existing)

	for _, tag := range tags {
		idx := slices.IndexFunc(merged, func(t Tag) bool { return t.Key == tag.Key })
		if idx >= 0 {
			merged[idx].Value = tag.Value
		} else {
			merged = append(merged, tag)
		}
	}

	if len(merged) > maxTagsPerCertificate {
		return nil, tooManyTagsError()
	}

	return merged, nil
}

// validateTags rejects tags that ACM refuses on any tagging call: more
// entries than the Tags parameter takes (at most 50, the same as the
// quota), or a tag that validateTag refuses.
func validateTags(tags []Tag) error {
	if len(tags) > maxTagsPerCertificate {
		return tooManyTagsError()
	}

	for _, tag := range tags {
		if err := validateTag(tag.Key, tag.Value); err != nil {
			return err
		}
	}

	return nil
}

// validateTag rejects a tag whose key or value is outside the Tag type's
// length limits, or whose key has the prefix reserved for AWS. Every
// tagging operation, RemoveTagsFromCertificate included, takes tags of
// this type.
func validateTag(key, value string) error {
	if n := utf8.RuneCountInString(key); n == 0 || n > maxTagKeyLength {
		return &Error{
			Code:    errInvalidParameter,
			Message: fmt.Sprintf("tag key %q must be 1 to %d characters long", key, maxTagKeyLength),
		}
	}

	if strings.HasPrefix(key, reservedTagPrefix) {
		return reservedTagError(key)
	}

	if utf8.RuneCountInString(value) > maxTagValueLength {
		return &Error{
			Code:    errInvalidParameter,
			Message: fmt.Sprintf("tag value for key %q must be at most %d characters long", key, maxTagValueLength),
		}
	}

	return nil
}

// tooManyTagsError reports a certificate exceeding maxTagsPerCertificate.
func tooManyTagsError() error {
	return &Error{
		Code:    errTooManyTags,
		Message: fmt.Sprintf("a certificate can have at most %d tags", maxTagsPerCertificate),
	}
}

// reservedTagError reports a tag key that uses the prefix reserved for AWS.
func reservedTagError(key string) error {
	return &Error{
		Code:    errInvalidTag,
		Message: fmt.Sprintf("tag key %q uses the reserved prefix %q", key, reservedTagPrefix),
	}
}

// certificateNotFound reports an ARN that does not refer to a stored
// certificate.
func certificateNotFound(arn string) error {
	return &Error{
		Code:    errNotFound,
		Message: fmt.Sprintf("Certificate with arn %s not found", arn),
	}
}
