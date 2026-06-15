package sesv2

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// CreateEmailIdentity handles the CreateEmailIdentity operation.
func (s *Service) CreateEmailIdentity(w http.ResponseWriter, r *http.Request) {
	var req CreateEmailIdentityRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	identity, err := s.storage.CreateEmailIdentity(r.Context(), &req)
	if err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errAlreadyExists {
				status = http.StatusConflict
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, CreateEmailIdentityResponse{
		IdentityType:             identity.IdentityType,
		VerifiedForSendingStatus: identity.VerifiedForSendingStatus,
		DkimAttributes:           identity.DkimAttributes,
	})
}

// GetEmailIdentity handles the GetEmailIdentity operation.
func (s *Service) GetEmailIdentity(w http.ResponseWriter, r *http.Request) {
	emailIdentity := extractPathParam(r.URL.Path, "/ses/v2/email/identities/")
	if emailIdentity == "" {
		writeError(w, errInvalidParameter, "EmailIdentity is required", http.StatusBadRequest)

		return
	}

	identity, err := s.storage.GetEmailIdentity(r.Context(), emailIdentity)
	if err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, GetEmailIdentityResponse{
		IdentityType:             identity.IdentityType,
		FeedbackForwardingStatus: true,
		VerifiedForSendingStatus: identity.VerifiedForSendingStatus,
		DkimAttributes:           identity.DkimAttributes,
	})
}

// ListEmailIdentities handles the ListEmailIdentities operation.
func (s *Service) ListEmailIdentities(w http.ResponseWriter, r *http.Request) {
	nextToken := r.URL.Query().Get("NextToken")
	pageSize := parsePageSize(r.URL.Query().Get("PageSize"))

	identities, nextTokenOut, err := s.storage.ListEmailIdentities(r.Context(), nextToken, pageSize)
	if err != nil {
		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	summaries := make([]EmailIdentitySummary, 0, len(identities))
	for _, identity := range identities {
		summaries = append(summaries, EmailIdentitySummary{
			IdentityName:   identity.IdentityName,
			IdentityType:   identity.IdentityType,
			SendingEnabled: identity.VerifiedForSendingStatus,
		})
	}

	writeJSONResponse(w, ListEmailIdentitiesResponse{
		EmailIdentities: summaries,
		NextToken:       nextTokenOut,
	})
}

// DeleteEmailIdentity handles the DeleteEmailIdentity operation.
func (s *Service) DeleteEmailIdentity(w http.ResponseWriter, r *http.Request) {
	emailIdentity := extractPathParam(r.URL.Path, "/ses/v2/email/identities/")
	if emailIdentity == "" {
		writeError(w, errInvalidParameter, "EmailIdentity is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteEmailIdentity(r.Context(), emailIdentity); err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreateConfigurationSet handles the CreateConfigurationSet operation.
func (s *Service) CreateConfigurationSet(w http.ResponseWriter, r *http.Request) {
	var req CreateConfigurationSetRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	_, err := s.storage.CreateConfigurationSet(r.Context(), &req)
	if err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errAlreadyExists {
				status = http.StatusConflict
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetConfigurationSet handles the GetConfigurationSet operation.
func (s *Service) GetConfigurationSet(w http.ResponseWriter, r *http.Request) {
	name := extractPathParam(r.URL.Path, "/ses/v2/email/configuration-sets/")
	if name == "" {
		writeError(w, errInvalidParameter, "ConfigurationSetName is required", http.StatusBadRequest)

		return
	}

	configSet, err := s.storage.GetConfigurationSet(r.Context(), name)
	if err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, GetConfigurationSetResponse{
		ConfigurationSetName: configSet.Name,
		DeliveryOptions:      configSet.DeliveryOptions,
		ReputationOptions:    configSet.ReputationOptions,
		SendingOptions:       configSet.SendingOptions,
		TrackingOptions:      configSet.TrackingOptions,
		Tags:                 configSet.Tags,
	})
}

// ListConfigurationSets handles the ListConfigurationSets operation.
func (s *Service) ListConfigurationSets(w http.ResponseWriter, r *http.Request) {
	nextToken := r.URL.Query().Get("NextToken")
	pageSize := parsePageSize(r.URL.Query().Get("PageSize"))

	names, nextTokenOut, err := s.storage.ListConfigurationSets(r.Context(), nextToken, pageSize)
	if err != nil {
		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, ListConfigurationSetsResponse{
		ConfigurationSets: names,
		NextToken:         nextTokenOut,
	})
}

// DeleteConfigurationSet handles the DeleteConfigurationSet operation.
func (s *Service) DeleteConfigurationSet(w http.ResponseWriter, r *http.Request) {
	name := extractPathParam(r.URL.Path, "/ses/v2/email/configuration-sets/")
	if name == "" {
		writeError(w, errInvalidParameter, "ConfigurationSetName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteConfigurationSet(r.Context(), name); err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreateEmailTemplate handles the CreateEmailTemplate operation.
func (s *Service) CreateEmailTemplate(w http.ResponseWriter, r *http.Request) {
	var req CreateEmailTemplateRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	_, err := s.storage.CreateEmailTemplate(r.Context(), &req)
	if err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errAlreadyExists {
				status = http.StatusConflict
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetEmailTemplate handles the GetEmailTemplate operation.
func (s *Service) GetEmailTemplate(w http.ResponseWriter, r *http.Request) {
	name := extractPathParam(r.URL.Path, "/ses/v2/email/templates/")
	if name == "" {
		writeError(w, errInvalidParameter, "TemplateName is required", http.StatusBadRequest)

		return
	}

	tmpl, err := s.storage.GetEmailTemplate(r.Context(), name)
	if err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, GetEmailTemplateResponse{
		TemplateName:    tmpl.Name,
		TemplateContent: tmpl.TemplateContent,
	})
}

// UpdateEmailTemplate handles the UpdateEmailTemplate operation.
func (s *Service) UpdateEmailTemplate(w http.ResponseWriter, r *http.Request) {
	name := extractPathParam(r.URL.Path, "/ses/v2/email/templates/")
	if name == "" {
		writeError(w, errInvalidParameter, "TemplateName is required", http.StatusBadRequest)

		return
	}

	var req UpdateEmailTemplateRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	_, err := s.storage.UpdateEmailTemplate(r.Context(), name, &req)
	if err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteEmailTemplate handles the DeleteEmailTemplate operation.
func (s *Service) DeleteEmailTemplate(w http.ResponseWriter, r *http.Request) {
	name := extractPathParam(r.URL.Path, "/ses/v2/email/templates/")
	if name == "" {
		writeError(w, errInvalidParameter, "TemplateName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteEmailTemplate(r.Context(), name); err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// ListEmailTemplates handles the ListEmailTemplates operation.
func (s *Service) ListEmailTemplates(w http.ResponseWriter, r *http.Request) {
	nextToken := r.URL.Query().Get("NextToken")
	pageSize := parsePageSize(r.URL.Query().Get("PageSize"))

	templates, nextTokenOut, err := s.storage.ListEmailTemplates(r.Context(), nextToken, pageSize)
	if err != nil {
		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	metadata := make([]EmailTemplateMetadata, 0, len(templates))
	for _, tmpl := range templates {
		metadata = append(metadata, EmailTemplateMetadata{
			TemplateName:     tmpl.Name,
			CreatedTimestamp: epochSeconds(tmpl.CreatedTimestamp),
		})
	}

	writeJSONResponse(w, ListEmailTemplatesResponse{
		TemplatesMetadata: metadata,
		NextToken:         nextTokenOut,
	})
}

// SendBulkEmail handles the SendBulkEmail operation.
func (s *Service) SendBulkEmail(w http.ResponseWriter, r *http.Request) {
	var req SendBulkEmailRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.SendBulkEmail(r.Context(), &req)
	if err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, resp)
}

// SendEmail handles the SendEmail operation.
func (s *Service) SendEmail(w http.ResponseWriter, r *http.Request) {
	var req SendEmailRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	messageID, err := s.storage.SendEmail(r.Context(), &req)
	if err != nil {
		var sErr *IdentityError
		if errors.As(err, &sErr) {
			writeError(w, sErr.Code, sErr.Message, http.StatusBadRequest)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, SendEmailResponse{
		MessageID: messageID,
	})
}

// GetSentEmails handles the GetSentEmails operation.
func (s *Service) GetSentEmails(w http.ResponseWriter, r *http.Request) {
	emails, err := s.storage.GetSentEmails(r.Context())
	if err != nil {
		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, GetSentEmailsResponse{
		SentEmails: emails,
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

// extractPathParam extracts a path parameter from the URL.
func extractPathParam(path, prefix string) string {
	param, found := strings.CutPrefix(path, prefix)
	if !found {
		return ""
	}

	// Remove any trailing path segments.
	if idx := strings.Index(param, "/"); idx != -1 {
		param = param[:idx]
	}

	return param
}

// parsePageSize parses the page size from a string, returning 100 as default.
func parsePageSize(s string) int32 {
	const (
		defaultPageSize = 100
		maxPageSize     = 1000
	)

	if s == "" {
		return defaultPageSize
	}

	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n <= 0 || n > maxPageSize {
		return defaultPageSize
	}

	return int32(n)
}
