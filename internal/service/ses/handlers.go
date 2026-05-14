package ses

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"strings"

	"github.com/google/uuid"
)

const sesXMLNS = "http://ses.amazonaws.com/doc/2010-12-01/"

// Error codes for SES.
const (
	errInvalidParameter     = "InvalidParameterValue"
	errInternalServiceError = "InternalError"
	errInvalidAction        = "InvalidAction"
)

// VerifyEmailIdentity handles the VerifyEmailIdentity action.
func (s *Service) VerifyEmailIdentity(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailIdentityRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.EmailAddress == "" {
		writeError(w, errInvalidParameter, "EmailAddress is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.VerifyEmailIdentity(r.Context(), req.EmailAddress); err != nil {
		writeError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLVerifyEmailIdentityResponse{
		Xmlns: sesXMLNS,
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// SendEmail handles the SendEmail action.
func (s *Service) SendEmail(w http.ResponseWriter, r *http.Request) {
	var req SendEmailRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	destinations := make([]string, 0, len(req.DestinationToAddresses)+len(req.DestinationCcAddresses)+len(req.DestinationBccAddresses))
	destinations = append(destinations, req.DestinationToAddresses...)
	destinations = append(destinations, req.DestinationCcAddresses...)
	destinations = append(destinations, req.DestinationBccAddresses...)

	email := &SentEmail{
		Source:      req.Source,
		Subject:     req.MessageSubjectData,
		Body:        req.MessageBodyTextData,
		HTMLBody:    req.MessageBodyHTMLData,
		Destination: destinations,
	}

	messageID, err := s.storage.SendEmail(r.Context(), email)
	if err != nil {
		writeError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLSendEmailResponse{
		Xmlns: sesXMLNS,
		SendEmailResult: XMLSendEmailResult{
			MessageID: messageID,
		},
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// SendRawEmail handles the SendRawEmail action.
func (s *Service) SendRawEmail(w http.ResponseWriter, r *http.Request) {
	var req SendRawEmailRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	// Decode base64-encoded raw message data.
	rawBytes, err := base64.StdEncoding.DecodeString(req.RawMessageData)
	if err != nil {
		// Try the raw data as-is if base64 decoding fails.
		rawBytes = []byte(req.RawMessageData)
	}

	rawData := string(rawBytes)

	// Parse the raw email to extract subject and body.
	subject, body, htmlBody := extractRawEmailContent(rawBytes)

	// Use destinations from the request or extract from headers.
	destinations := req.Destinations
	if len(destinations) == 0 {
		destinations = extractRawEmailRecipients(rawBytes)
	}

	source := req.Source
	if source == "" {
		source = extractRawEmailSender(rawBytes)
	}

	email := &SentEmail{
		Source:      source,
		Subject:     subject,
		Body:        body,
		HTMLBody:    htmlBody,
		RawData:     rawData,
		Destination: destinations,
	}

	messageID, err := s.storage.SendEmail(r.Context(), email)
	if err != nil {
		writeError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLSendRawEmailResponse{
		Xmlns: sesXMLNS,
		SendRawEmailResult: XMLSendRawEmailResult{
			MessageID: messageID,
		},
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// ListIdentities handles the ListIdentities action.
func (s *Service) ListIdentities(w http.ResponseWriter, r *http.Request) {
	identities, err := s.storage.ListIdentities(r.Context())
	if err != nil {
		writeError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLListIdentitiesResponse{
		Xmlns: sesXMLNS,
		ListIdentitiesResult: XMLListIdentitiesResult{
			Identities: XMLIdentities{
				Member: identities,
			},
		},
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// DeleteIdentity handles the DeleteIdentity action.
func (s *Service) DeleteIdentity(w http.ResponseWriter, r *http.Request) {
	var req DeleteIdentityRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteIdentity(r.Context(), req.Identity); err != nil {
		writeError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLDeleteIdentityResponse{
		Xmlns: sesXMLNS,
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// GetIdentityVerificationAttributes handles the GetIdentityVerificationAttributes action.
func (s *Service) GetIdentityVerificationAttributes(w http.ResponseWriter, r *http.Request) {
	var req GetIdentityVerificationAttributesRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	attrs, err := s.storage.GetIdentityVerificationAttributes(r.Context(), req.Identities)
	if err != nil {
		writeError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	entries := make([]XMLVerificationEntry, 0, len(attrs))
	for identity, status := range attrs {
		entries = append(entries, XMLVerificationEntry{
			Key: identity,
			Value: XMLVerificationAttributeValue{
				VerificationStatus: status,
			},
		})
	}

	writeXMLResponse(w, XMLGetIdentityVerificationAttributesResponse{
		Xmlns: sesXMLNS,
		GetIdentityVerificationAttributesResult: XMLGetIdentityVerificationAttributesResult{
			VerificationAttributes: XMLVerificationAttributes{
				Entry: entries,
			},
		},
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// GetMailbox handles the kumo-specific mailbox endpoint.
// This returns all sent emails for a given sender, exposed at /_aws/ses?email=...
func (s *Service) GetMailbox(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "email query parameter is required",
		})

		return
	}

	emails, err := s.storage.GetMailbox(r.Context(), email)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to get mailbox",
		})

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(emails)
}

// actionHandlers returns a map of action names to handler functions.
func (s *Service) actionHandlers() map[string]func(http.ResponseWriter, *http.Request) {
	return map[string]func(http.ResponseWriter, *http.Request){
		"VerifyEmailIdentity":               s.VerifyEmailIdentity,
		"SendEmail":                         s.SendEmail,
		"SendRawEmail":                      s.SendRawEmail,
		"ListIdentities":                    s.ListIdentities,
		"DeleteIdentity":                    s.DeleteIdentity,
		"GetIdentityVerificationAttributes": s.GetIdentityVerificationAttributes,
	}
}

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, s.TargetPrefix()+".")

	handlers := s.actionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, errInvalidAction, "The action "+action+" is not valid", http.StatusBadRequest)
}

// Helper functions.

// readJSONRequest reads and decodes JSON request body.
func readJSONRequest(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// writeXMLResponse writes an XML response with HTTP 200 OK.
func writeXMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

// writeError writes an SES error response in XML format.
func writeError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(XMLErrorResponse{
		Xmlns: sesXMLNS,
		Error: XMLErrorDetail{
			Type:    "Sender",
			Code:    code,
			Message: message,
		},
		RequestID: requestID,
	})
}

// extractRawEmailContent parses an RFC 2822 MIME message and extracts subject, text body, and HTML body.
func extractRawEmailContent(data []byte) (subject, textBody, htmlBody string) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return "", "", ""
	}

	subject = msg.Header.Get("Subject")

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		// Not multipart; read body directly.
		b, readErr := io.ReadAll(msg.Body)
		if readErr != nil {
			return subject, "", ""
		}

		content := string(b)
		if strings.HasPrefix(mediaType, "text/html") {
			return subject, "", content
		}

		return subject, content, ""
	}

	// Multipart message: extract both text/plain and text/html parts.
	reader := multipart.NewReader(msg.Body, params["boundary"])

	for {
		part, partErr := reader.NextPart()
		if partErr != nil {
			break
		}

		partType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))

		b, readErr := io.ReadAll(part)
		if readErr != nil {
			continue
		}

		switch partType {
		case "text/plain":
			textBody = string(b)
		case "text/html":
			htmlBody = string(b)
		}
	}

	return subject, textBody, htmlBody
}

// extractRawEmailRecipients extracts recipients from raw email headers.
func extractRawEmailRecipients(data []byte) []string {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	var recipients []string

	for _, header := range []string{"To", "Cc", "Bcc"} {
		if val := msg.Header.Get(header); val != "" {
			addrs, err := mail.ParseAddressList(val)
			if err == nil {
				for _, a := range addrs {
					recipients = append(recipients, a.Address)
				}
			}
		}
	}

	return recipients
}

// extractRawEmailSender extracts the sender from raw email headers.
func extractRawEmailSender(data []byte) string {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return ""
	}

	from := msg.Header.Get("From")
	if from == "" {
		return ""
	}

	addr, err := mail.ParseAddress(from)
	if err != nil {
		return from
	}

	return addr.Address
}
