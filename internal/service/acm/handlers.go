package acm

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// DispatchAction routes ACM requests based on the X-Amz-Target header.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	if target == "" {
		writeError(w, errInvalidParameter, "Missing X-Amz-Target header", http.StatusBadRequest)

		return
	}

	// Extract operation from target (e.g., "CertificateManager.RequestCertificate")
	parts := strings.Split(target, ".")
	if len(parts) != 2 {
		writeError(w, errInvalidParameter, "Invalid X-Amz-Target header", http.StatusBadRequest)

		return
	}

	operation := parts[1]

	switch operation {
	case "RequestCertificate":
		s.RequestCertificate(w, r)
	case "DescribeCertificate":
		s.DescribeCertificate(w, r)
	case "ListCertificates":
		s.ListCertificates(w, r)
	case "DeleteCertificate":
		s.DeleteCertificate(w, r)
	case "GetCertificate":
		s.GetCertificate(w, r)
	case "ImportCertificate":
		s.ImportCertificate(w, r)
	default:
		writeError(w, errInvalidParameter, fmt.Sprintf("Unknown operation: %s", operation), http.StatusBadRequest)
	}
}

// RequestCertificate handles the RequestCertificate operation.
func (s *Service) RequestCertificate(w http.ResponseWriter, r *http.Request) {
	var req RequestCertificateInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	cert, err := s.storage.RequestCertificate(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, RequestCertificateOutput{
		CertificateArn: cert.CertificateArn,
	})
}

// DescribeCertificate handles the DescribeCertificate operation.
func (s *Service) DescribeCertificate(w http.ResponseWriter, r *http.Request) {
	var req DescribeCertificateInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.CertificateArn == "" {
		writeError(w, errInvalidParameter, "CertificateArn is required", http.StatusBadRequest)

		return
	}

	cert, err := s.storage.DescribeCertificate(r.Context(), req.CertificateArn)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	detail := &CertificateDetail{
		CertificateArn:          cert.CertificateArn,
		DomainName:              cert.DomainName,
		SubjectAlternativeNames: cert.SubjectAlternativeNames,
		DomainValidationOptions: cert.DomainValidationOptions,
		Serial:                  cert.Serial,
		Subject:                 cert.Subject,
		Issuer:                  cert.Issuer,
		CreatedAt:               ToAWSTimestamp(cert.CreatedAt).Ptr(),
		IssuedAt:                ToAWSTimestampPtr(cert.IssuedAt),
		ImportedAt:              ToAWSTimestampPtr(cert.ImportedAt),
		Status:                  cert.Status,
		NotBefore:               ToAWSTimestampPtr(cert.NotBefore),
		NotAfter:                ToAWSTimestampPtr(cert.NotAfter),
		KeyAlgorithm:            cert.KeyAlgorithm,
		SignatureAlgorithm:      cert.SignatureAlgorithm,
		InUseBy:                 cert.InUseBy,
		FailureReason:           cert.FailureReason,
		Type:                    cert.Type,
		KeyUsages:               cert.KeyUsages,
		ExtendedKeyUsages:       cert.ExtendedKeyUsages,
		RenewalEligibility:      cert.RenewalEligibility,
		Options:                 cert.Options,
	}

	writeJSONResponse(w, DescribeCertificateOutput{
		Certificate: detail,
	})
}

// ListCertificates handles the ListCertificates operation.
func (s *Service) ListCertificates(w http.ResponseWriter, r *http.Request) {
	var req ListCertificatesInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	certs, nextToken, err := s.storage.ListCertificates(r.Context(), req.CertificateStatuses, req.MaxItems, req.NextToken)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	summaries := make([]CertificateSummary, 0, len(certs))

	for _, cert := range certs {
		// Convert KeyUsages to string slice
		keyUsages := make([]string, 0, len(cert.KeyUsages))
		for _, ku := range cert.KeyUsages {
			keyUsages = append(keyUsages, ku.Name)
		}

		// Convert ExtendedKeyUsages to string slice
		extendedKeyUsages := make([]string, 0, len(cert.ExtendedKeyUsages))
		for _, eku := range cert.ExtendedKeyUsages {
			extendedKeyUsages = append(extendedKeyUsages, eku.Name)
		}

		summary := CertificateSummary{
			CertificateArn:          cert.CertificateArn,
			DomainName:              cert.DomainName,
			SubjectAlternativeNames: cert.SubjectAlternativeNames,
			Status:                  cert.Status,
			Type:                    cert.Type,
			KeyAlgorithm:            cert.KeyAlgorithm,
			CreatedAt:               ToAWSTimestamp(cert.CreatedAt).Ptr(),
			IssuedAt:                ToAWSTimestampPtr(cert.IssuedAt),
			ImportedAt:              ToAWSTimestampPtr(cert.ImportedAt),
			NotBefore:               ToAWSTimestampPtr(cert.NotBefore),
			NotAfter:                ToAWSTimestampPtr(cert.NotAfter),
			RenewalEligibility:      cert.RenewalEligibility,
			Exported:                cert.Type == "IMPORTED",
			InUse:                   len(cert.InUseBy) > 0,
			KeyUsages:               keyUsages,
			ExtendedKeyUsages:       extendedKeyUsages,
		}

		summaries = append(summaries, summary)
	}

	writeJSONResponse(w, ListCertificatesOutput{
		CertificateSummaryList: summaries,
		NextToken:              nextToken,
	})
}

// DeleteCertificate handles the DeleteCertificate operation.
func (s *Service) DeleteCertificate(w http.ResponseWriter, r *http.Request) {
	var req DeleteCertificateInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.CertificateArn == "" {
		writeError(w, errInvalidParameter, "CertificateArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteCertificate(r.Context(), req.CertificateArn); err != nil {
		handleStorageError(w, err)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
}

// GetCertificate handles the GetCertificate operation.
func (s *Service) GetCertificate(w http.ResponseWriter, r *http.Request) {
	var req GetCertificateInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.CertificateArn == "" {
		writeError(w, errInvalidParameter, "CertificateArn is required", http.StatusBadRequest)

		return
	}

	cert, err := s.storage.GetCertificate(r.Context(), req.CertificateArn)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, GetCertificateOutput{
		Certificate:      cert.CertificateBody,
		CertificateChain: cert.CertificateChain,
	})
}

// ImportCertificate handles the ImportCertificate operation.
func (s *Service) ImportCertificate(w http.ResponseWriter, r *http.Request) {
	var req ImportCertificateInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	cert, err := s.storage.ImportCertificate(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, ImportCertificateOutput{
		CertificateArn: cert.CertificateArn,
	})
}

// Helper functions.

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeJSON, v)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeJSON, code, message, status)
}

// handleStorageError handles storage errors and writes appropriate response.
func handleStorageError(w http.ResponseWriter, err error) {
	var acmErr *Error
	if errors.As(err, &acmErr) {
		status := http.StatusBadRequest
		if acmErr.Code == errNotFound {
			status = http.StatusNotFound
		}

		writeError(w, acmErr.Code, acmErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)
}
