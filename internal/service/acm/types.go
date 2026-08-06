// Package acm provides ACM service emulation for kumo.
package acm

import (
	"encoding/json"
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// AWSTimestamp is a time.Time that marshals to Unix timestamp (float64).
// AWS APIs use Unix timestamps in JSON responses.
type AWSTimestamp struct {
	time.Time
}

// MarshalJSON implements json.Marshaler.
func (t AWSTimestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return json.Marshal(nil) //nolint:wrapcheck // MarshalJSON interface requirement
	}

	return json.Marshal(float64(t.Unix()) + float64(t.Nanosecond())/1e9) //nolint:wrapcheck // MarshalJSON interface requirement
}

// Ptr returns a pointer to the AWSTimestamp.
func (t AWSTimestamp) Ptr() *AWSTimestamp {
	if t.IsZero() {
		return nil
	}

	return &t
}

// ToAWSTimestamp converts time.Time to AWSTimestamp.
func ToAWSTimestamp(t time.Time) AWSTimestamp {
	return AWSTimestamp{Time: t}
}

// ToAWSTimestampPtr converts *time.Time to *AWSTimestamp.
func ToAWSTimestampPtr(t *time.Time) *AWSTimestamp {
	if t == nil {
		return nil
	}

	return &AWSTimestamp{Time: *t}
}

// Certificate represents an ACM certificate.
type Certificate struct {
	CertificateArn          string
	DomainName              string
	SubjectAlternativeNames []string
	Status                  string // PENDING_VALIDATION, ISSUED, INACTIVE, EXPIRED, VALIDATION_TIMED_OUT, REVOKED, FAILED
	Type                    string // IMPORTED, AMAZON_ISSUED, PRIVATE
	KeyAlgorithm            string
	SignatureAlgorithm      string
	Issuer                  string
	Serial                  string
	Subject                 string
	NotBefore               *time.Time
	NotAfter                *time.Time
	CreatedAt               time.Time
	ImportedAt              *time.Time
	IssuedAt                *time.Time
	CertificateChain        string
	CertificateBody         string
	PrivateKey              string
	DomainValidationOptions []DomainValidation
	RenewalEligibility      string
	InUseBy                 []string
	FailureReason           string
	ExtendedKeyUsages       []ExtendedKeyUsage
	KeyUsages               []KeyUsage
	Options                 *CertificateOptions
	Tags                    []Tag
}

// DomainValidation represents domain validation information.
type DomainValidation struct {
	DomainName       string          `json:"DomainName,omitempty"`
	ValidationDomain string          `json:"ValidationDomain,omitempty"`
	ValidationStatus string          `json:"ValidationStatus,omitempty"`
	ResourceRecord   *ResourceRecord `json:"ResourceRecord,omitempty"`
	ValidationMethod string          `json:"ValidationMethod,omitempty"`
	ValidationEmails []string        `json:"ValidationEmails,omitempty"`
}

// ResourceRecord represents a DNS resource record.
type ResourceRecord struct {
	Name  string `json:"Name,omitempty"`
	Type  string `json:"Type,omitempty"`
	Value string `json:"Value,omitempty"`
}

// ExtendedKeyUsage represents extended key usage.
type ExtendedKeyUsage struct {
	Name string `json:"Name,omitempty"`
	OID  string `json:"OID,omitempty"`
}

// KeyUsage represents key usage.
type KeyUsage struct {
	Name string `json:"Name,omitempty"`
}

// CertificateOptions represents certificate options.
type CertificateOptions struct {
	CertificateTransparencyLoggingPreference string `json:"CertificateTransparencyLoggingPreference,omitempty"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// RequestCertificateInput is the request for RequestCertificate.
type RequestCertificateInput struct {
	DomainName              string              `json:"DomainName"`
	SubjectAlternativeNames []string            `json:"SubjectAlternativeNames,omitempty"`
	IdempotencyToken        string              `json:"IdempotencyToken,omitempty"`
	DomainValidationOptions []DomainValidation  `json:"DomainValidationOptions,omitempty"`
	Options                 *CertificateOptions `json:"Options,omitempty"`
	ValidationMethod        string              `json:"ValidationMethod,omitempty"`
	KeyAlgorithm            string              `json:"KeyAlgorithm,omitempty"`
	Tags                    []Tag               `json:"Tags,omitempty"`
}

// RequestCertificateOutput is the response for RequestCertificate.
type RequestCertificateOutput struct {
	CertificateArn string `json:"CertificateArn,omitempty"`
}

// DescribeCertificateInput is the request for DescribeCertificate.
type DescribeCertificateInput struct {
	CertificateArn string `json:"CertificateArn"`
}

// DescribeCertificateOutput is the response for DescribeCertificate.
type DescribeCertificateOutput struct {
	Certificate *CertificateDetail `json:"Certificate,omitempty"`
}

// CertificateDetail represents detailed certificate information.
type CertificateDetail struct {
	CertificateArn          string              `json:"CertificateArn,omitempty"`
	DomainName              string              `json:"DomainName,omitempty"`
	SubjectAlternativeNames []string            `json:"SubjectAlternativeNames,omitempty"`
	DomainValidationOptions []DomainValidation  `json:"DomainValidationOptions,omitempty"`
	Serial                  string              `json:"Serial,omitempty"`
	Subject                 string              `json:"Subject,omitempty"`
	Issuer                  string              `json:"Issuer,omitempty"`
	CreatedAt               *AWSTimestamp       `json:"CreatedAt,omitempty"`
	IssuedAt                *AWSTimestamp       `json:"IssuedAt,omitempty"`
	ImportedAt              *AWSTimestamp       `json:"ImportedAt,omitempty"`
	Status                  string              `json:"Status,omitempty"`
	NotBefore               *AWSTimestamp       `json:"NotBefore,omitempty"`
	NotAfter                *AWSTimestamp       `json:"NotAfter,omitempty"`
	KeyAlgorithm            string              `json:"KeyAlgorithm,omitempty"`
	SignatureAlgorithm      string              `json:"SignatureAlgorithm,omitempty"`
	InUseBy                 []string            `json:"InUseBy,omitempty"`
	FailureReason           string              `json:"FailureReason,omitempty"`
	Type                    string              `json:"Type,omitempty"`
	RenewalSummary          *RenewalSummary     `json:"RenewalSummary,omitempty"`
	KeyUsages               []KeyUsage          `json:"KeyUsages,omitempty"`
	ExtendedKeyUsages       []ExtendedKeyUsage  `json:"ExtendedKeyUsages,omitempty"`
	RenewalEligibility      string              `json:"RenewalEligibility,omitempty"`
	Options                 *CertificateOptions `json:"Options,omitempty"`
}

// RenewalSummary represents renewal summary.
type RenewalSummary struct {
	RenewalStatus           string             `json:"RenewalStatus,omitempty"`
	DomainValidationOptions []DomainValidation `json:"DomainValidationOptions,omitempty"`
	RenewalStatusReason     string             `json:"RenewalStatusReason,omitempty"`
	UpdatedAt               *AWSTimestamp      `json:"UpdatedAt,omitempty"`
}

// ListCertificatesInput is the request for ListCertificates.
type ListCertificatesInput struct {
	CertificateStatuses []string `json:"CertificateStatuses,omitempty"`
	MaxItems            int32    `json:"MaxItems,omitempty"`
	NextToken           string   `json:"NextToken,omitempty"`
	SortBy              string   `json:"SortBy,omitempty"`
	SortOrder           string   `json:"SortOrder,omitempty"`
}

// ListCertificatesOutput is the response for ListCertificates.
type ListCertificatesOutput struct {
	CertificateSummaryList []CertificateSummary `json:"CertificateSummaryList,omitempty"`
	NextToken              string               `json:"NextToken,omitempty"`
}

// CertificateSummary represents a certificate summary.
type CertificateSummary struct {
	CertificateArn          string        `json:"CertificateArn,omitempty"`
	DomainName              string        `json:"DomainName,omitempty"`
	SubjectAlternativeNames []string      `json:"SubjectAlternativeNameSummaries,omitempty"`
	Status                  string        `json:"Status,omitempty"`
	Type                    string        `json:"Type,omitempty"`
	KeyAlgorithm            string        `json:"KeyAlgorithm,omitempty"`
	CreatedAt               *AWSTimestamp `json:"CreatedAt,omitempty"`
	IssuedAt                *AWSTimestamp `json:"IssuedAt,omitempty"`
	ImportedAt              *AWSTimestamp `json:"ImportedAt,omitempty"`
	NotBefore               *AWSTimestamp `json:"NotBefore,omitempty"`
	NotAfter                *AWSTimestamp `json:"NotAfter,omitempty"`
	RenewalEligibility      string        `json:"RenewalEligibility,omitempty"`
	Exported                bool          `json:"Exported,omitempty"`
	InUse                   bool          `json:"InUse,omitempty"`
	KeyUsages               []string      `json:"KeyUsages,omitempty"`
	ExtendedKeyUsages       []string      `json:"ExtendedKeyUsages,omitempty"`
}

// DeleteCertificateInput is the request for DeleteCertificate.
type DeleteCertificateInput struct {
	CertificateArn string `json:"CertificateArn"`
}

// GetCertificateInput is the request for GetCertificate.
type GetCertificateInput struct {
	CertificateArn string `json:"CertificateArn"`
}

// GetCertificateOutput is the response for GetCertificate.
type GetCertificateOutput struct {
	Certificate      string `json:"Certificate,omitempty"`
	CertificateChain string `json:"CertificateChain,omitempty"`
}

// ImportCertificateInput is the request for ImportCertificate.
type ImportCertificateInput struct {
	Certificate      []byte `json:"Certificate"`
	PrivateKey       []byte `json:"PrivateKey"`
	CertificateChain []byte `json:"CertificateChain,omitempty"`
	CertificateArn   string `json:"CertificateArn,omitempty"`
	Tags             []Tag  `json:"Tags,omitempty"`
}

// ImportCertificateOutput is the response for ImportCertificate.
type ImportCertificateOutput struct {
	CertificateArn string `json:"CertificateArn,omitempty"`
}

// ErrorResponse represents an ACM error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error represents an ACM error.
type Error = service.CodedError
