package sesv2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errNotFound         = "NotFoundException"
	errAlreadyExists    = "AlreadyExistsException"
	errInvalidParameter = "ValidationException"
	errBadRequest       = "BadRequestException"
)

// Storage defines the interface for SES v2 storage operations.
type Storage interface {
	// Email Identity operations.
	CreateEmailIdentity(ctx context.Context, req *CreateEmailIdentityRequest) (*EmailIdentity, error)
	GetEmailIdentity(ctx context.Context, emailIdentity string) (*EmailIdentity, error)
	ListEmailIdentities(ctx context.Context, nextToken string, pageSize int32) ([]*EmailIdentity, string, error)
	DeleteEmailIdentity(ctx context.Context, emailIdentity string) error

	// Configuration Set operations.
	CreateConfigurationSet(ctx context.Context, req *CreateConfigurationSetRequest) (*ConfigurationSet, error)
	GetConfigurationSet(ctx context.Context, name string) (*ConfigurationSet, error)
	ListConfigurationSets(ctx context.Context, nextToken string, pageSize int32) ([]string, string, error)
	DeleteConfigurationSet(ctx context.Context, name string) error

	// Email Template operations.
	CreateEmailTemplate(ctx context.Context, req *CreateEmailTemplateRequest) (*EmailTemplate, error)
	GetEmailTemplate(ctx context.Context, name string) (*EmailTemplate, error)
	UpdateEmailTemplate(ctx context.Context, name string, req *UpdateEmailTemplateRequest) (*EmailTemplate, error)
	DeleteEmailTemplate(ctx context.Context, name string) error
	ListEmailTemplates(ctx context.Context, nextToken string, pageSize int32) ([]*EmailTemplate, string, error)

	// Send Email.
	SendEmail(ctx context.Context, req *SendEmailRequest) (string, error)

	// Send Bulk Email.
	SendBulkEmail(ctx context.Context, req *SendBulkEmailRequest) (*SendBulkEmailResponse, error)

	// Get sent emails (for testing purposes).
	GetSentEmails(ctx context.Context) ([]*SentEmail, error)
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
	mu                sync.RWMutex                 `json:"-"`
	EmailIdentities   map[string]*EmailIdentity    `json:"emailIdentities"`
	ConfigurationSets map[string]*ConfigurationSet `json:"configurationSets"`
	EmailTemplates    map[string]*EmailTemplate    `json:"emailTemplates"`
	SentEmails        []*SentEmail                 `json:"sentEmails"`
	dataDir           string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		EmailIdentities:   make(map[string]*EmailIdentity),
		ConfigurationSets: make(map[string]*ConfigurationSet),
		EmailTemplates:    make(map[string]*EmailTemplate),
		SentEmails:        make([]*SentEmail, 0),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "sesv2", s)
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

	if s.EmailIdentities == nil {
		s.EmailIdentities = make(map[string]*EmailIdentity)
	}

	if s.ConfigurationSets == nil {
		s.ConfigurationSets = make(map[string]*ConfigurationSet)
	}

	if s.EmailTemplates == nil {
		s.EmailTemplates = make(map[string]*EmailTemplate)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "sesv2", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "sesv2", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateEmailIdentity creates a new email identity.
func (s *MemoryStorage) CreateEmailIdentity(_ context.Context, req *CreateEmailIdentityRequest) (*EmailIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.EmailIdentity == "" {
		return nil, &IdentityError{
			Code:    errInvalidParameter,
			Message: "EmailIdentity is required",
		}
	}

	if _, exists := s.EmailIdentities[req.EmailIdentity]; exists {
		return nil, &IdentityError{
			Code:    errAlreadyExists,
			Message: "The email identity already exists",
		}
	}

	identityType := "EMAIL_ADDRESS"
	if !strings.Contains(req.EmailIdentity, "@") {
		identityType = "DOMAIN"
	}

	identity := &EmailIdentity{
		IdentityName:             req.EmailIdentity,
		IdentityType:             identityType,
		VerifiedForSendingStatus: true, // Auto-verify for testing.
		DkimAttributes: &DkimAttributes{
			SigningEnabled:          true,
			Status:                  "SUCCESS",
			SigningAttributesOrigin: "AWS_SES",
			Tokens:                  []string{uuid.New().String()[:20]},
		},
		CreatedAt: time.Now(),
	}

	s.EmailIdentities[req.EmailIdentity] = identity

	s.saveLocked()

	return identity, nil
}

// GetEmailIdentity retrieves an email identity.
func (s *MemoryStorage) GetEmailIdentity(_ context.Context, emailIdentity string) (*EmailIdentity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	identity, exists := s.EmailIdentities[emailIdentity]
	if !exists {
		return nil, &IdentityError{
			Code:    errNotFound,
			Message: "The email identity does not exist",
		}
	}

	return identity, nil
}

// ListEmailIdentities lists all email identities.
func (s *MemoryStorage) ListEmailIdentities(_ context.Context, _ string, pageSize int32) ([]*EmailIdentity, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pageSize <= 0 {
		pageSize = 100
	}

	identities := make([]*EmailIdentity, 0, len(s.EmailIdentities))
	for _, identity := range s.EmailIdentities {
		identities = append(identities, identity)
	}

	// Simple pagination (no actual cursor).
	if len(identities) > int(pageSize) {
		identities = identities[:pageSize]
	}

	return identities, "", nil
}

// DeleteEmailIdentity deletes an email identity.
func (s *MemoryStorage) DeleteEmailIdentity(_ context.Context, emailIdentity string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.EmailIdentities[emailIdentity]; !exists {
		return &IdentityError{
			Code:    errNotFound,
			Message: "The email identity does not exist",
		}
	}

	delete(s.EmailIdentities, emailIdentity)

	s.saveLocked()

	return nil
}

// CreateConfigurationSet creates a new configuration set.
func (s *MemoryStorage) CreateConfigurationSet(_ context.Context, req *CreateConfigurationSetRequest) (*ConfigurationSet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.ConfigurationSetName == "" {
		return nil, &IdentityError{
			Code:    errInvalidParameter,
			Message: "ConfigurationSetName is required",
		}
	}

	if _, exists := s.ConfigurationSets[req.ConfigurationSetName]; exists {
		return nil, &IdentityError{
			Code:    errAlreadyExists,
			Message: "The configuration set already exists",
		}
	}

	configSet := &ConfigurationSet{
		Name:              req.ConfigurationSetName,
		DeliveryOptions:   req.DeliveryOptions,
		ReputationOptions: req.ReputationOptions,
		SendingOptions:    req.SendingOptions,
		TrackingOptions:   req.TrackingOptions,
		Tags:              req.Tags,
	}

	// Set defaults if not provided.
	if configSet.SendingOptions == nil {
		configSet.SendingOptions = &SendingOptions{SendingEnabled: true}
	}

	s.ConfigurationSets[req.ConfigurationSetName] = configSet

	s.saveLocked()

	return configSet, nil
}

// GetConfigurationSet retrieves a configuration set.
func (s *MemoryStorage) GetConfigurationSet(_ context.Context, name string) (*ConfigurationSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configSet, exists := s.ConfigurationSets[name]
	if !exists {
		return nil, &IdentityError{
			Code:    errNotFound,
			Message: "The configuration set does not exist",
		}
	}

	return configSet, nil
}

// ListConfigurationSets lists all configuration sets.
func (s *MemoryStorage) ListConfigurationSets(_ context.Context, _ string, pageSize int32) ([]string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pageSize <= 0 {
		pageSize = 100
	}

	names := make([]string, 0, len(s.ConfigurationSets))
	for name := range s.ConfigurationSets {
		names = append(names, name)
	}

	if len(names) > int(pageSize) {
		names = names[:pageSize]
	}

	return names, "", nil
}

// DeleteConfigurationSet deletes a configuration set.
func (s *MemoryStorage) DeleteConfigurationSet(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.ConfigurationSets[name]; !exists {
		return &IdentityError{
			Code:    errNotFound,
			Message: "The configuration set does not exist",
		}
	}

	delete(s.ConfigurationSets, name)

	s.saveLocked()

	return nil
}

// CreateEmailTemplate creates a new email template.
func (s *MemoryStorage) CreateEmailTemplate(_ context.Context, req *CreateEmailTemplateRequest) (*EmailTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.TemplateName == "" {
		return nil, &IdentityError{
			Code:    errInvalidParameter,
			Message: "TemplateName is required",
		}
	}

	if req.TemplateContent == nil {
		return nil, &IdentityError{
			Code:    errInvalidParameter,
			Message: "TemplateContent is required",
		}
	}

	if _, exists := s.EmailTemplates[req.TemplateName]; exists {
		return nil, &IdentityError{
			Code:    errAlreadyExists,
			Message: "The email template already exists",
		}
	}

	tmpl := &EmailTemplate{
		Name:             req.TemplateName,
		TemplateContent:  cloneTemplateContent(req.TemplateContent),
		CreatedTimestamp: time.Now(),
	}

	s.EmailTemplates[req.TemplateName] = tmpl

	return tmpl, nil
}

// GetEmailTemplate retrieves an email template.
func (s *MemoryStorage) GetEmailTemplate(_ context.Context, name string) (*EmailTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tmpl, exists := s.EmailTemplates[name]
	if !exists {
		return nil, &IdentityError{
			Code:    errNotFound,
			Message: "The email template does not exist",
		}
	}

	return tmpl, nil
}

// UpdateEmailTemplate replaces the content of an existing email template.
func (s *MemoryStorage) UpdateEmailTemplate(_ context.Context, name string, req *UpdateEmailTemplateRequest) (*EmailTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.TemplateContent == nil {
		return nil, &IdentityError{
			Code:    errInvalidParameter,
			Message: "TemplateContent is required",
		}
	}

	tmpl, exists := s.EmailTemplates[name]
	if !exists {
		return nil, &IdentityError{
			Code:    errNotFound,
			Message: "The email template does not exist",
		}
	}

	tmpl.TemplateContent = cloneTemplateContent(req.TemplateContent)

	return tmpl, nil
}

// DeleteEmailTemplate deletes an email template.
func (s *MemoryStorage) DeleteEmailTemplate(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.EmailTemplates[name]; !exists {
		return &IdentityError{
			Code:    errNotFound,
			Message: "The email template does not exist",
		}
	}

	delete(s.EmailTemplates, name)

	return nil
}

// ListEmailTemplates lists all email templates.
func (s *MemoryStorage) ListEmailTemplates(_ context.Context, _ string, pageSize int32) ([]*EmailTemplate, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pageSize <= 0 {
		pageSize = 100
	}

	templates := make([]*EmailTemplate, 0, len(s.EmailTemplates))
	for _, tmpl := range s.EmailTemplates {
		templates = append(templates, tmpl)
	}

	if len(templates) > int(pageSize) {
		templates = templates[:pageSize]
	}

	return templates, "", nil
}

// cloneTemplateContent returns a deep copy of the template content to avoid aliasing.
func cloneTemplateContent(src *EmailTemplateContent) *EmailTemplateContent {
	if src == nil {
		return nil
	}

	clone := *src

	return &clone
}

// SendEmail sends an email (stores it for testing).
func (s *MemoryStorage) SendEmail(_ context.Context, req *SendEmailRequest) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Content == nil {
		return "", &IdentityError{
			Code:    errBadRequest,
			Message: "Content is required",
		}
	}

	// Basic validation.
	// Destination is not required when Content.Raw is set,
	// because recipients can be extracted from MIME headers.
	hasDestination := req.Destination != nil &&
		(len(req.Destination.ToAddresses) > 0 ||
			len(req.Destination.CcAddresses) > 0 ||
			len(req.Destination.BccAddresses) > 0)

	if !hasDestination && req.Content.Raw == nil {
		return "", &IdentityError{
			Code:    errBadRequest,
			Message: "Destination is required",
		}
	}

	// Generate message ID.
	messageID := uuid.New().String()

	// Extract content based on email type.
	var (
		subject, body, htmlBody string
		rawData                 []byte
		destination             = req.Destination
	)

	switch {
	case req.Content.Raw != nil:
		rawData = req.Content.Raw.Data
		subject, body, htmlBody = extractRawEmailContent(rawData)

		if !hasDestination {
			destination = extractRawEmailDestination(rawData)
		}
	case req.Content.Simple != nil:
		subject, body, htmlBody = extractSimpleEmailContent(req.Content.Simple)
	}

	// Store the sent email.
	sentEmail := &SentEmail{
		MessageID:            messageID,
		FromEmailAddress:     req.FromEmailAddress,
		Destination:          destination,
		Subject:              subject,
		Body:                 body,
		HTMLBody:             htmlBody,
		RawData:              rawData,
		ConfigurationSetName: req.ConfigurationSetName,
		SentAt:               time.Now(),
	}

	s.SentEmails = append(s.SentEmails, sentEmail)

	s.saveLocked()

	return messageID, nil
}

// SendBulkEmail sends one email per BulkEmailEntry, recording each as a SentEmail.
//
// AWS SES v2 returns HTTP 200 with per-entry status even when individual entries
// fail validation. Errors at the request level (missing entries, missing template,
// unknown template) result in HTTP 400.
func (s *MemoryStorage) SendBulkEmail(_ context.Context, req *SendBulkEmailRequest) (*SendBulkEmailResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmpl, defaultTemplate, err := s.validateBulkEmailRequest(req)
	if err != nil {
		return nil, err
	}

	results := make([]BulkEmailEntryResult, 0, len(req.BulkEmailEntries))

	for _, entry := range req.BulkEmailEntries {
		sent, result := buildBulkEmailEntry(req, &entry, defaultTemplate, tmpl)
		if sent != nil {
			s.SentEmails = append(s.SentEmails, sent)
		}

		results = append(results, result)
	}

	return &SendBulkEmailResponse{BulkEmailEntryResults: results}, nil
}

// validateBulkEmailRequest checks the top-level request and resolves the template.
// Must be called with s.mu held.
func (s *MemoryStorage) validateBulkEmailRequest(req *SendBulkEmailRequest) (*EmailTemplate, *Template, error) {
	if len(req.BulkEmailEntries) == 0 {
		return nil, nil, &IdentityError{
			Code:    errBadRequest,
			Message: "BulkEmailEntries is required",
		}
	}

	if req.DefaultContent == nil || req.DefaultContent.Template == nil {
		return nil, nil, &IdentityError{
			Code:    errBadRequest,
			Message: "DefaultContent.Template is required",
		}
	}

	defaultTemplate := req.DefaultContent.Template
	if defaultTemplate.TemplateName == "" {
		return nil, nil, &IdentityError{
			Code:    errBadRequest,
			Message: "DefaultContent.Template.TemplateName is required",
		}
	}

	tmpl, exists := s.EmailTemplates[defaultTemplate.TemplateName]
	if !exists {
		return nil, nil, &IdentityError{
			Code:    errNotFound,
			Message: "The email template does not exist",
		}
	}

	return tmpl, defaultTemplate, nil
}

// buildBulkEmailEntry computes the recorded SentEmail (nil on per-entry failure)
// and the per-entry result for a single BulkEmailEntry.
func buildBulkEmailEntry(req *SendBulkEmailRequest, entry *BulkEmailEntry, defaultTemplate *Template, tmpl *EmailTemplate) (*SentEmail, BulkEmailEntryResult) {
	messageID := uuid.New().String()

	hasDestination := entry.Destination != nil &&
		(len(entry.Destination.ToAddresses) > 0 ||
			len(entry.Destination.CcAddresses) > 0 ||
			len(entry.Destination.BccAddresses) > 0)

	if !hasDestination {
		return nil, BulkEmailEntryResult{
			Status:    "FAILED",
			Error:     "Destination is required",
			MessageID: messageID,
		}
	}

	templateData := defaultTemplate.TemplateData
	if entry.ReplacementEmailContent != nil &&
		entry.ReplacementEmailContent.ReplacementTemplate != nil &&
		entry.ReplacementEmailContent.ReplacementTemplate.ReplacementTemplateData != "" {
		templateData = entry.ReplacementEmailContent.ReplacementTemplate.ReplacementTemplateData
	}

	sent := &SentEmail{
		MessageID:            messageID,
		FromEmailAddress:     req.FromEmailAddress,
		Destination:          cloneDestination(entry.Destination),
		TemplateName:         defaultTemplate.TemplateName,
		TemplateData:         templateData,
		ConfigurationSetName: req.ConfigurationSetName,
		SentAt:               time.Now(),
	}

	if tmpl.TemplateContent != nil {
		sent.Subject = tmpl.TemplateContent.Subject
		sent.Body = tmpl.TemplateContent.Text
		sent.HTMLBody = tmpl.TemplateContent.HTML
	}

	return sent, BulkEmailEntryResult{
		Status:    "SUCCESS",
		MessageID: messageID,
	}
}

// cloneDestination returns a deep copy of a destination to avoid aliasing.
func cloneDestination(src *Destination) *Destination {
	if src == nil {
		return nil
	}

	clone := &Destination{}

	if len(src.ToAddresses) > 0 {
		clone.ToAddresses = append([]string{}, src.ToAddresses...)
	}

	if len(src.CcAddresses) > 0 {
		clone.CcAddresses = append([]string{}, src.CcAddresses...)
	}

	if len(src.BccAddresses) > 0 {
		clone.BccAddresses = append([]string{}, src.BccAddresses...)
	}

	return clone
}

// GetSentEmails returns all sent emails (for testing).
func (s *MemoryStorage) GetSentEmails(_ context.Context) ([]*SentEmail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.SentEmails, nil
}

// extractRawEmailDestination parses an RFC 2822 MIME message and extracts destination addresses.
func extractRawEmailDestination(data []byte) *Destination {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	dest := &Destination{}

	if to := msg.Header.Get("To"); to != "" {
		addrs, err := mail.ParseAddressList(to)
		if err == nil {
			for _, a := range addrs {
				dest.ToAddresses = append(dest.ToAddresses, a.Address)
			}
		}
	}

	if cc := msg.Header.Get("Cc"); cc != "" {
		addrs, err := mail.ParseAddressList(cc)
		if err == nil {
			for _, a := range addrs {
				dest.CcAddresses = append(dest.CcAddresses, a.Address)
			}
		}
	}

	if bcc := msg.Header.Get("Bcc"); bcc != "" {
		addrs, err := mail.ParseAddressList(bcc)
		if err == nil {
			for _, a := range addrs {
				dest.BccAddresses = append(dest.BccAddresses, a.Address)
			}
		}
	}

	return dest
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
		b, err := io.ReadAll(msg.Body)
		if err != nil {
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
		part, err := reader.NextPart()
		if err != nil {
			break
		}

		partType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))

		b, err := io.ReadAll(part)
		if err != nil {
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

// extractSimpleEmailContent extracts subject, text body, and HTML body from a SimpleEmail.
func extractSimpleEmailContent(simple *SimpleEmail) (subject, textBody, htmlBody string) {
	if simple == nil {
		return "", "", ""
	}

	if simple.Subject != nil {
		subject = simple.Subject.Data
	}

	if simple.Body == nil {
		return subject, "", ""
	}

	if simple.Body.Text != nil {
		textBody = simple.Body.Text.Data
	}

	if simple.Body.HTML != nil {
		htmlBody = simple.Body.HTML.Data
	}

	return subject, textBody, htmlBody
}
