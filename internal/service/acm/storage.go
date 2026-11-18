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

// resourceRecordTypeCNAME is the type of DNS validation records.
const resourceRecordTypeCNAME = "CNAME"

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

// keyAlgorithmRSA2048 is the KeyAlgorithm a certificate request defaults to.
const keyAlgorithmRSA2048 = "RSA_2048"

// renewalIneligible is the RenewalEligibility of every certificate, since
// kumo does not renew certificates.
const renewalIneligible = "INELIGIBLE"

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

// WithDNSValidationDelay keeps a DNS-validated certificate in
// PENDING_VALIDATION for delay after it is requested, and then issues it.
// A delay of zero, the default, issues it as soon as it is requested.
func WithDNSValidationDelay(delay time.Duration) Option {
	return func(s *MemoryStorage) {
		s.dnsValidationDelay = delay
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data structures.
//
// Operations return copies of the stored certificates, made while holding
// mu, because a stored certificate changes after it is returned: it is
// issued once its DNS validation delay passes, and its material is
// generated on the first GetCertificate.
type MemoryStorage struct {
	mu           sync.RWMutex            `json:"-"`
	Certificates map[string]*Certificate `json:"certificates"`
	dataDir      string
	region       string
	// dnsValidationDelay is how long a DNS-validated certificate stays
	// PENDING_VALIDATION after it is requested (see WithDNSValidationDelay).
	dnsValidationDelay time.Duration
	// now returns the current time; tests replace it to move past the DNS
	// validation delay.
	now func() time.Time
	// caMu guards ca. Certificate material is generated without holding
	// mu, so the CA has a lock of its own.
	caMu sync.Mutex
	ca   *localCA
	// newKey generates the private key of an issued certificate; tests
	// replace it to check when and where it runs.
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
		now:          time.Now,
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

// UnmarshalJSON restores the storage state from JSON, normalizing what
// older snapshots hold. A snapshot saved before kumo checked tags can hold
// a key twice on a certificate; it is stored once, with its last value, as
// a tagging call stores it. DNS-validated certificates whose validation
// delay has passed are issued on load: a snapshot saved before kumo issued
// DNS-validated certificates holds them in PENDING_VALIDATION, where they
// would otherwise stay.
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

	for _, cert := range s.Certificates {
		cert.Tags = applyTags(nil, cert.Tags)

		if _, err := s.issueIfDueLocked(cert); err != nil {
			return err
		}
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

// buildDomainValidations creates the domain validation options of a newly
// requested certificate, each PENDING_VALIDATION until the certificate is
// issued (see markIssued).
func buildDomainValidations(req *RequestCertificateInput, validationMethod string) []DomainValidation {
	domains := make([]string, 0, 1+len(req.SubjectAlternativeNames))
	domains = append(domains, req.DomainName)
	domains = append(domains, req.SubjectAlternativeNames...)

	validations := make([]DomainValidation, 0, len(domains))

	for _, domain := range domains {
		dv := DomainValidation{
			DomainName:       domain,
			ValidationDomain: domain,
			ValidationStatus: statusPendingValidation,
			ValidationMethod: validationMethod,
		}

		if validationMethod == validationMethodDNS {
			dv.ResourceRecord = validationRecord(domain)
		}

		validations = append(validations, dv)
	}

	return validations
}

// validationRecordLabelLength is the length of the lowercase label between
// the token and acm-validations.aws in a validation record's value.
const validationRecordLabelLength = 10

// validationRecord returns the DNS validation CNAME for domain. ACM creates
// the record for the domain and the account, so the same name and value are
// returned for every certificate covering the domain, and a wildcard domain
// shares the record of its base domain. The value has the shape of the ones
// ACM returns, _<token>.<label>.acm-validations.aws.
// https://docs.aws.amazon.com/acm/latest/userguide/dns-validation.html
func validationRecord(domain string) *ResourceRecord {
	base := strings.ToLower(strings.TrimPrefix(domain, "*."))

	digest := func(kind string) [sha256.Size]byte {
		return sha256.Sum256([]byte(kind + "/" + accountID + "/" + base))
	}

	name := digest("name")
	value := digest("value")
	labelDigest := digest("label")

	label := make([]byte, validationRecordLabelLength)
	for i, b := range labelDigest[:validationRecordLabelLength] {
		label[i] = 'a' + b%26
	}

	return &ResourceRecord{
		Name:  fmt.Sprintf("_%s.%s.", hex.EncodeToString(name[:16]), base),
		Type:  resourceRecordTypeCNAME,
		Value: fmt.Sprintf("_%s.%s.acm-validations.aws.", hex.EncodeToString(value[:16]), label),
	}
}

// RequestCertificate requests a new certificate. kumo cannot observe DNS,
// so a DNS-validated certificate is issued once the storage's DNS
// validation delay has passed instead of waiting for its validation CNAME
// records; with the default delay of zero it is issued right away, which
// clients such as the Terraform AWS provider's aws_acm_certificate_validation
// wait for. Email validation stays pending. The key and the certificate
// itself are generated on the first GetCertificate.
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

	cert := &Certificate{
		CertificateArn:          s.certificateARN(uuid.New().String()),
		DomainName:              req.DomainName,
		SubjectAlternativeNames: req.SubjectAlternativeNames,
		Status:                  statusPendingValidation,
		Type:                    certificateTypeAmazonIssued,
		KeyAlgorithm:            keyAlgorithm,
		Serial:                  hex.EncodeToString(serialBytes),
		Subject:                 fmt.Sprintf("CN=%s", req.DomainName),
		CreatedAt:               s.now(),
		DomainValidationOptions: buildDomainValidations(req, validationMethod),
		RenewalEligibility:      renewalIneligible,
		Options:                 req.Options,
		Tags:                    tags,
	}

	if _, err := s.issueIfDueLocked(cert); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Certificates[cert.CertificateArn] = cert

	s.saveLocked()

	return cert.clone(), nil
}

// issueIfDueLocked issues cert if it is a DNS-validated certificate whose
// validation delay has passed, as of the moment the delay ended, and
// reports whether it did. The caller must hold s.mu for writing, or cert
// must not be visible to other goroutines yet.
func (s *MemoryStorage) issueIfDueLocked(cert *Certificate) (bool, error) {
	if cert.Type != certificateTypeAmazonIssued || cert.Status != statusPendingValidation || !validatedByDNS(cert) {
		return false, nil
	}

	issuedAt := cert.CreatedAt.Add(s.dnsValidationDelay)
	if s.now().Before(issuedAt) {
		return false, nil
	}

	if err := markIssued(cert, issuedAt); err != nil {
		return false, err
	}

	return true, nil
}

// validatedByDNS reports whether every domain of cert is validated by DNS.
func validatedByDNS(cert *Certificate) bool {
	return len(cert.DomainValidationOptions) > 0 &&
		!slices.ContainsFunc(cert.DomainValidationOptions, func(dv DomainValidation) bool {
			return dv.ValidationMethod != validationMethodDNS
		})
}

// clone returns a copy of c that a caller can read after s.mu is released.
// The copy shares c's slices, which the storage replaces rather than
// modifies.
func (c *Certificate) clone() *Certificate {
	cp := *c

	return &cp
}

// lookupLocked returns the certificate stored under arn, first issuing it
// if its DNS validation delay has passed. The caller must hold s.mu for
// writing.
func (s *MemoryStorage) lookupLocked(arn string) (*Certificate, error) {
	cert, exists := s.Certificates[arn]
	if !exists {
		return nil, certificateNotFound(arn)
	}

	issued, err := s.issueIfDueLocked(cert)
	if err != nil {
		return nil, err
	}

	if issued {
		s.saveLocked()
	}

	return cert, nil
}

// certificateARN returns the ARN of the certificate certID in the storage's
// region.
func (s *MemoryStorage) certificateARN(certID string) string {
	return fmt.Sprintf("arn:aws:acm:%s:%s:certificate/%s", s.region, accountID, certID)
}

// DescribeCertificate retrieves certificate details.
func (s *MemoryStorage) DescribeCertificate(_ context.Context, arn string) (*Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cert, err := s.lookupLocked(arn)
	if err != nil {
		return nil, err
	}

	return cert.clone(), nil
}

// ListCertificates lists certificates with optional filtering.
func (s *MemoryStorage) ListCertificates(_ context.Context, statuses []string, maxItems int32, _ string) ([]*Certificate, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if maxItems <= 0 {
		maxItems = 100
	}

	certs := make([]*Certificate, 0, len(s.Certificates))
	saved := false

	for _, cert := range s.Certificates {
		issued, err := s.issueIfDueLocked(cert)
		if err != nil {
			return nil, "", err
		}

		if issued && !saved {
			s.saveLocked()

			saved = true
		}

		if len(statuses) > 0 && !slices.Contains(statuses, cert.Status) {
			continue
		}

		certs = append(certs, cert.clone())
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

// GetCertificate retrieves certificate and chain. The material of an
// issued certificate is generated on the first call, without holding the
// storage lock, and stored, so later calls return the same material.
func (s *MemoryStorage) GetCertificate(_ context.Context, arn string) (*Certificate, error) {
	cert, err := s.retrievable(arn)
	if err != nil || cert.CertificateBody != "" {
		return cert, err
	}

	generated, err := s.certificateMaterial(cert)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	stored, exists := s.Certificates[arn]
	if !exists {
		return nil, certificateNotFound(arn)
	}

	// A concurrent call may have stored its material first, or the ARN may
	// have been imported over; keep what is stored then.
	if stored.CertificateBody == "" && stored.Serial == cert.Serial {
		stored.CertificateBody = generated.certificate
		stored.CertificateChain = generated.chain
		stored.PrivateKey = generated.privateKey

		s.saveLocked()
	}

	return stored.clone(), nil
}

// retrievable returns a copy of the certificate stored under arn if
// GetCertificate can return it: an imported certificate, or an issued one.
func (s *MemoryStorage) retrievable(arn string) (*Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cert, err := s.lookupLocked(arn)
	if err != nil {
		return nil, err
	}

	if cert.Status != statusIssued && cert.Type != certificateTypeImported {
		return nil, &Error{
			Code:    errRequestPending,
			Message: fmt.Sprintf("Certificate with arn %s is not yet issued", arn),
		}
	}

	return cert.clone(), nil
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
		arn = s.certificateARN(uuid.New().String())
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

	return cert.clone(), nil
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
// certificate may carry, so a key given twice counts once. existing is
// applied the same way first, so a key that a snapshot saved before kumo
// checked tags holds twice ends up once, with its last value.
func mergeTags(existing, tags []Tag) ([]Tag, error) {
	if err := validateTags(tags); err != nil {
		return nil, err
	}

	merged := applyTags(applyTags(nil, existing), tags)

	if len(merged) > maxTagsPerCertificate {
		return nil, tooManyTagsError()
	}

	return merged, nil
}

// applyTags returns a copy of dst with tags applied in order: a key that is
// already present takes the new value, and any other key is appended.
func applyTags(dst, tags []Tag) []Tag {
	merged := slices.Clone(dst)

	for _, tag := range tags {
		idx := slices.IndexFunc(merged, func(t Tag) bool { return t.Key == tag.Key })
		if idx >= 0 {
			merged[idx].Value = tag.Value
		} else {
			merged = append(merged, tag)
		}
	}

	return merged
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
