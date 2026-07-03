// Package sesv2 provides SES v2 service emulation for kumo.
package sesv2

import (
	"strconv"
	"time"
)

// epochSeconds is a time.Time-compatible value that JSON-marshals as a
// Unix-epoch number (with optional fractional seconds). The AWS SES v2
// rest-json protocol uses this representation for Timestamp shapes; the
// SDK's deserializer rejects ISO 8601 strings with
// "expected Timestamp to be a JSON Number, got string instead".
type epochSeconds time.Time

// MarshalJSON implements json.Marshaler.
func (t epochSeconds) MarshalJSON() ([]byte, error) {
	secs := float64(time.Time(t).UnixNano()) / 1e9

	return []byte(strconv.FormatFloat(secs, 'f', -1, 64)), nil
}

// EmailIdentity represents an email identity (email address or domain).
type EmailIdentity struct {
	IdentityName             string
	IdentityType             string // EMAIL_ADDRESS or DOMAIN
	VerifiedForSendingStatus bool
	DkimAttributes           *DkimAttributes
	CreatedAt                time.Time
}

// DkimAttributes represents DKIM configuration for an identity.
type DkimAttributes struct {
	SigningEnabled          bool
	Status                  string // PENDING, SUCCESS, FAILED, NOT_STARTED, TEMPORARY_FAILURE
	SigningAttributesOrigin string
	Tokens                  []string
}

// ConfigurationSet represents an SES configuration set.
type ConfigurationSet struct {
	Name              string
	DeliveryOptions   *DeliveryOptions
	ReputationOptions *ReputationOptions
	SendingOptions    *SendingOptions
	TrackingOptions   *TrackingOptions
	Tags              []Tag
}

// DeliveryOptions represents delivery options for a configuration set.
type DeliveryOptions struct {
	TLSPolicy       string
	SendingPoolName string
}

// ReputationOptions represents reputation options for a configuration set.
type ReputationOptions struct {
	ReputationMetricsEnabled bool
	LastFreshStart           *time.Time
}

// SendingOptions represents sending options for a configuration set.
type SendingOptions struct {
	SendingEnabled bool
}

// TrackingOptions represents tracking options for a configuration set.
type TrackingOptions struct {
	CustomRedirectDomain string
}

// SentEmail represents a sent email for debugging purposes.
type SentEmail struct {
	MessageID            string       `json:"MessageId"`
	FromEmailAddress     string       `json:"FromEmailAddress"`
	Destination          *Destination `json:"Destination,omitempty"`
	Subject              string       `json:"Subject,omitempty"`
	Body                 string       `json:"Body,omitempty"`
	HTMLBody             string       `json:"HTMLBody,omitempty"`
	RawData              []byte       `json:"RawData,omitempty"`
	TemplateName         string       `json:"TemplateName,omitempty"`
	TemplateData         string       `json:"TemplateData,omitempty"`
	ConfigurationSetName string       `json:"ConfigurationSetName,omitempty"`
	SentAt               time.Time    `json:"SentAt"`
}

// EmailTemplate represents an SES v2 email template.
type EmailTemplate struct {
	Name             string
	TemplateContent  *EmailTemplateContent
	CreatedTimestamp time.Time
}

// EmailTemplateContent represents the renderable parts of an email template.
type EmailTemplateContent struct {
	Subject string `json:"Subject,omitempty"`
	Text    string `json:"Text,omitempty"`
	HTML    string `json:"Html,omitempty"`
}

// EmailTemplateMetadata describes a template entry returned by ListEmailTemplates.
type EmailTemplateMetadata struct {
	TemplateName     string       `json:"TemplateName,omitempty"`
	CreatedTimestamp epochSeconds `json:"CreatedTimestamp,omitempty"`
}

// Destination represents email destinations.
type Destination struct {
	ToAddresses  []string `json:"ToAddresses,omitempty"`
	CcAddresses  []string `json:"CcAddresses,omitempty"`
	BccAddresses []string `json:"BccAddresses,omitempty"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// CreateEmailIdentityRequest is the request for CreateEmailIdentity.
type CreateEmailIdentityRequest struct {
	EmailIdentity         string            `json:"EmailIdentity"`
	ConfigurationSetName  string            `json:"ConfigurationSetName,omitempty"`
	DkimSigningAttributes *DkimSigningAttrs `json:"DkimSigningAttributes,omitempty"`
	Tags                  []Tag             `json:"Tags,omitempty"`
}

// DkimSigningAttrs represents DKIM signing attributes in request.
type DkimSigningAttrs struct {
	DomainSigningPrivateKey string `json:"DomainSigningPrivateKey,omitempty"`
	DomainSigningSelector   string `json:"DomainSigningSelector,omitempty"`
	NextSigningKeyLength    string `json:"NextSigningKeyLength,omitempty"`
}

// CreateEmailIdentityResponse is the response for CreateEmailIdentity.
type CreateEmailIdentityResponse struct {
	IdentityType             string          `json:"IdentityType,omitempty"`
	VerifiedForSendingStatus bool            `json:"VerifiedForSendingStatus"`
	DkimAttributes           *DkimAttributes `json:"DkimAttributes,omitempty"`
}

// GetEmailIdentityResponse is the response for GetEmailIdentity.
type GetEmailIdentityResponse struct {
	IdentityType             string          `json:"IdentityType,omitempty"`
	FeedbackForwardingStatus bool            `json:"FeedbackForwardingStatus"`
	VerifiedForSendingStatus bool            `json:"VerifiedForSendingStatus"`
	DkimAttributes           *DkimAttributes `json:"DkimAttributes,omitempty"`
	ConfigurationSetName     string          `json:"ConfigurationSetName,omitempty"`
	Tags                     []Tag           `json:"Tags,omitempty"`
}

// ListEmailIdentitiesRequest is the request for ListEmailIdentities.
type ListEmailIdentitiesRequest struct {
	NextToken string `json:"NextToken,omitempty"`
	PageSize  int32  `json:"PageSize,omitempty"`
}

// ListEmailIdentitiesResponse is the response for ListEmailIdentities.
type ListEmailIdentitiesResponse struct {
	EmailIdentities []EmailIdentitySummary `json:"EmailIdentities,omitempty"`
	NextToken       string                 `json:"NextToken,omitempty"`
}

// EmailIdentitySummary represents an email identity summary.
type EmailIdentitySummary struct {
	IdentityName   string `json:"IdentityName,omitempty"`
	IdentityType   string `json:"IdentityType,omitempty"`
	SendingEnabled bool   `json:"SendingEnabled"`
}

// CreateConfigurationSetRequest is the request for CreateConfigurationSet.
type CreateConfigurationSetRequest struct {
	ConfigurationSetName string             `json:"ConfigurationSetName"`
	DeliveryOptions      *DeliveryOptions   `json:"DeliveryOptions,omitempty"`
	ReputationOptions    *ReputationOptions `json:"ReputationOptions,omitempty"`
	SendingOptions       *SendingOptions    `json:"SendingOptions,omitempty"`
	TrackingOptions      *TrackingOptions   `json:"TrackingOptions,omitempty"`
	Tags                 []Tag              `json:"Tags,omitempty"`
}

// GetConfigurationSetResponse is the response for GetConfigurationSet.
type GetConfigurationSetResponse struct {
	ConfigurationSetName string             `json:"ConfigurationSetName,omitempty"`
	DeliveryOptions      *DeliveryOptions   `json:"DeliveryOptions,omitempty"`
	ReputationOptions    *ReputationOptions `json:"ReputationOptions,omitempty"`
	SendingOptions       *SendingOptions    `json:"SendingOptions,omitempty"`
	TrackingOptions      *TrackingOptions   `json:"TrackingOptions,omitempty"`
	Tags                 []Tag              `json:"Tags,omitempty"`
}

// ListConfigurationSetsRequest is the request for ListConfigurationSets.
type ListConfigurationSetsRequest struct {
	NextToken string `json:"NextToken,omitempty"`
	PageSize  int32  `json:"PageSize,omitempty"`
}

// ListConfigurationSetsResponse is the response for ListConfigurationSets.
type ListConfigurationSetsResponse struct {
	ConfigurationSets []string `json:"ConfigurationSets,omitempty"`
	NextToken         string   `json:"NextToken,omitempty"`
}

// SendEmailRequest is the request for SendEmail.
type SendEmailRequest struct {
	FromEmailAddress               string        `json:"FromEmailAddress,omitempty"`
	FromEmailAddressIdentityArn    string        `json:"FromEmailAddressIdentityArn,omitempty"`
	Destination                    *Destination  `json:"Destination,omitempty"`
	ReplyToAddresses               []string      `json:"ReplyToAddresses,omitempty"`
	FeedbackForwardingEmailAddress string        `json:"FeedbackForwardingEmailAddress,omitempty"`
	Content                        *EmailContent `json:"Content,omitempty"`
	EmailTags                      []MessageTag  `json:"EmailTags,omitempty"`
	ConfigurationSetName           string        `json:"ConfigurationSetName,omitempty"`
}

// EmailContent represents the content of an email.
type EmailContent struct {
	Simple   *SimpleEmail `json:"Simple,omitempty"`
	Raw      *RawEmail    `json:"Raw,omitempty"`
	Template *Template    `json:"Template,omitempty"`
}

// SimpleEmail represents a simple email.
type SimpleEmail struct {
	Subject *Content `json:"Subject,omitempty"`
	Body    *Body    `json:"Body,omitempty"`
}

// RawEmail represents a raw email.
type RawEmail struct {
	Data []byte `json:"Data,omitempty"`
}

// Template represents a template email.
type Template struct {
	TemplateName string `json:"TemplateName,omitempty"`
	TemplateArn  string `json:"TemplateArn,omitempty"`
	TemplateData string `json:"TemplateData,omitempty"`
}

// Body represents the body of an email.
type Body struct {
	Text *Content `json:"Text,omitempty"`
	HTML *Content `json:"Html,omitempty"`
}

// Content represents text content.
type Content struct {
	Data    string `json:"Data,omitempty"`
	Charset string `json:"Charset,omitempty"`
}

// MessageTag represents a message tag.
type MessageTag struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

// SendEmailResponse is the response for SendEmail.
type SendEmailResponse struct {
	MessageID string `json:"MessageId,omitempty"`
}

// CreateEmailTemplateRequest is the request for CreateEmailTemplate.
type CreateEmailTemplateRequest struct {
	TemplateName    string                `json:"TemplateName"`
	TemplateContent *EmailTemplateContent `json:"TemplateContent,omitempty"`
}

// UpdateEmailTemplateRequest is the request for UpdateEmailTemplate.
type UpdateEmailTemplateRequest struct {
	TemplateContent *EmailTemplateContent `json:"TemplateContent,omitempty"`
}

// GetEmailTemplateResponse is the response for GetEmailTemplate.
type GetEmailTemplateResponse struct {
	TemplateName    string                `json:"TemplateName,omitempty"`
	TemplateContent *EmailTemplateContent `json:"TemplateContent,omitempty"`
}

// ListEmailTemplatesRequest is the request for ListEmailTemplates.
type ListEmailTemplatesRequest struct {
	NextToken string `json:"NextToken,omitempty"`
	PageSize  int32  `json:"PageSize,omitempty"`
}

// ListEmailTemplatesResponse is the response for ListEmailTemplates.
type ListEmailTemplatesResponse struct {
	TemplatesMetadata []EmailTemplateMetadata `json:"TemplatesMetadata,omitempty"`
	NextToken         string                  `json:"NextToken,omitempty"`
}

// SendBulkEmailRequest is the request for SendBulkEmail.
type SendBulkEmailRequest struct {
	FromEmailAddress               string            `json:"FromEmailAddress,omitempty"`
	FromEmailAddressIdentityArn    string            `json:"FromEmailAddressIdentityArn,omitempty"`
	ReplyToAddresses               []string          `json:"ReplyToAddresses,omitempty"`
	FeedbackForwardingEmailAddress string            `json:"FeedbackForwardingEmailAddress,omitempty"`
	DefaultContent                 *BulkEmailContent `json:"DefaultContent,omitempty"`
	BulkEmailEntries               []BulkEmailEntry  `json:"BulkEmailEntries,omitempty"`
	DefaultEmailTags               []MessageTag      `json:"DefaultEmailTags,omitempty"`
	ConfigurationSetName           string            `json:"ConfigurationSetName,omitempty"`
}

// BulkEmailContent represents the default content shared across all bulk entries.
type BulkEmailContent struct {
	Template *Template `json:"Template,omitempty"`
}

// BulkEmailEntry represents a single bulk-email destination entry.
type BulkEmailEntry struct {
	Destination             *Destination             `json:"Destination,omitempty"`
	ReplacementEmailContent *ReplacementEmailContent `json:"ReplacementEmailContent,omitempty"`
	ReplacementTags         []MessageTag             `json:"ReplacementTags,omitempty"`
}

// ReplacementEmailContent carries per-entry template overrides.
type ReplacementEmailContent struct {
	ReplacementTemplate *ReplacementTemplate `json:"ReplacementTemplate,omitempty"`
}

// ReplacementTemplate carries per-entry replacement template data.
type ReplacementTemplate struct {
	ReplacementTemplateData string `json:"ReplacementTemplateData,omitempty"`
}

// SendBulkEmailResponse is the response for SendBulkEmail.
type SendBulkEmailResponse struct {
	BulkEmailEntryResults []BulkEmailEntryResult `json:"BulkEmailEntryResults,omitempty"`
}

// BulkEmailEntryResult represents the result of a single bulk-email entry.
type BulkEmailEntryResult struct {
	Status    string `json:"Status,omitempty"`
	Error     string `json:"Error,omitempty"`
	MessageID string `json:"MessageId,omitempty"`
}

// GetSentEmailsResponse is the response for GetSentEmails.
type GetSentEmailsResponse struct {
	SentEmails []*SentEmail `json:"SentEmails"`
}

// ErrorResponse represents an SES error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// IdentityError represents an SES error.
type IdentityError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *IdentityError) Error() string {
	return e.Message
}
