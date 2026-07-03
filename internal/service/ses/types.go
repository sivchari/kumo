// Package ses provides SES v1 service emulation for kumo.
package ses

import (
	"encoding/xml"
	"time"
)

// Identity represents a verified email identity.
type Identity struct {
	Email      string
	VerifiedAt time.Time
}

// SentEmail represents a sent email stored in the local mailbox.
type SentEmail struct {
	MessageID   string    `json:"messageId"`
	Source      string    `json:"source"`
	Subject     string    `json:"subject"`
	Body        string    `json:"body"`
	HTMLBody    string    `json:"htmlBody,omitempty"`
	RawData     string    `json:"rawData,omitempty"`
	Destination []string  `json:"destination"`
	SentAt      time.Time `json:"sentAt"`
}

// VerifyEmailIdentityRequest is the request for VerifyEmailIdentity.
//
//nolint:tagliatelle // AWS Query protocol uses PascalCase
type VerifyEmailIdentityRequest struct {
	EmailAddress string `json:"EmailAddress"`
}

// SendEmailRequest is the request for SendEmail.
// After formToJSON, nested Query keys become flat dot-separated keys:
//
//	"Destination.ToAddresses.member.1" -> "Destination.ToAddresses": [...]
//	"Message.Subject.Data" -> "Message.Subject.Data": "..."
//
//nolint:tagliatelle // AWS SES uses PascalCase JSON
type SendEmailRequest struct {
	Source                  string   `json:"Source"`
	DestinationToAddresses  []string `json:"Destination.ToAddresses"`
	DestinationCcAddresses  []string `json:"Destination.CcAddresses"`
	DestinationBccAddresses []string `json:"Destination.BccAddresses"`
	MessageSubjectData      string   `json:"Message.Subject.Data"`
	MessageSubjectCharset   string   `json:"Message.Subject.Charset,omitempty"`
	MessageBodyTextData     string   `json:"Message.Body.Text.Data,omitempty"`
	MessageBodyTextCharset  string   `json:"Message.Body.Text.Charset,omitempty"`
	MessageBodyHTMLData     string   `json:"Message.Body.Html.Data,omitempty"`
	MessageBodyHTMLCharset  string   `json:"Message.Body.Html.Charset,omitempty"`
}

// SendRawEmailRequest is the request for SendRawEmail.
// After formToJSON, "RawMessage.Data" stays flat and "Destinations.member.N" -> "Destinations".
//
//nolint:tagliatelle // AWS SES uses PascalCase JSON
type SendRawEmailRequest struct {
	Source         string   `json:"Source"`
	Destinations   []string `json:"Destinations"`
	RawMessageData string   `json:"RawMessage.Data"`
}

// ListIdentitiesRequest is the request for ListIdentities.
//
//nolint:tagliatelle // AWS Query protocol uses PascalCase
type ListIdentitiesRequest struct {
	IdentityType string `json:"IdentityType,omitempty"`
	NextToken    string `json:"NextToken,omitempty"`
	MaxItems     int    `json:"MaxItems,omitempty"`
}

// DeleteIdentityRequest is the request for DeleteIdentity.
//
//nolint:tagliatelle // AWS Query protocol uses PascalCase
type DeleteIdentityRequest struct {
	Identity string `json:"Identity"`
}

// GetIdentityVerificationAttributesRequest is the request for GetIdentityVerificationAttributes.
// After formToJSON, "Identities.member.N" -> "Identities".
//
//nolint:tagliatelle // AWS Query protocol uses PascalCase
type GetIdentityVerificationAttributesRequest struct {
	Identities []string `json:"Identities"`
}

// XML response types for Query protocol.

// XMLVerifyEmailIdentityResponse is the XML response for VerifyEmailIdentity.
type XMLVerifyEmailIdentityResponse struct {
	XMLName                   xml.Name         `xml:"VerifyEmailIdentityResponse"`
	Xmlns                     string           `xml:"xmlns,attr"`
	VerifyEmailIdentityResult struct{}         `xml:"VerifyEmailIdentityResult"`
	ResponseMetadata          ResponseMetadata `xml:"ResponseMetadata"`
}

// XMLSendEmailResponse is the XML response for SendEmail.
type XMLSendEmailResponse struct {
	XMLName          xml.Name           `xml:"SendEmailResponse"`
	Xmlns            string             `xml:"xmlns,attr"`
	SendEmailResult  XMLSendEmailResult `xml:"SendEmailResult"`
	ResponseMetadata ResponseMetadata   `xml:"ResponseMetadata"`
}

// XMLSendEmailResult contains the SendEmail result.
type XMLSendEmailResult struct {
	MessageID string `xml:"MessageId"`
}

// XMLSendRawEmailResponse is the XML response for SendRawEmail.
type XMLSendRawEmailResponse struct {
	XMLName            xml.Name              `xml:"SendRawEmailResponse"`
	Xmlns              string                `xml:"xmlns,attr"`
	SendRawEmailResult XMLSendRawEmailResult `xml:"SendRawEmailResult"`
	ResponseMetadata   ResponseMetadata      `xml:"ResponseMetadata"`
}

// XMLSendRawEmailResult contains the SendRawEmail result.
type XMLSendRawEmailResult struct {
	MessageID string `xml:"MessageId"`
}

// XMLListIdentitiesResponse is the XML response for ListIdentities.
type XMLListIdentitiesResponse struct {
	XMLName              xml.Name                `xml:"ListIdentitiesResponse"`
	Xmlns                string                  `xml:"xmlns,attr"`
	ListIdentitiesResult XMLListIdentitiesResult `xml:"ListIdentitiesResult"`
	ResponseMetadata     ResponseMetadata        `xml:"ResponseMetadata"`
}

// XMLListIdentitiesResult contains the ListIdentities result.
type XMLListIdentitiesResult struct {
	Identities XMLIdentities `xml:"Identities"`
}

// XMLIdentities is a wrapper for identity members.
type XMLIdentities struct {
	Member []string `xml:"member"`
}

// XMLDeleteIdentityResponse is the XML response for DeleteIdentity.
type XMLDeleteIdentityResponse struct {
	XMLName              xml.Name         `xml:"DeleteIdentityResponse"`
	Xmlns                string           `xml:"xmlns,attr"`
	DeleteIdentityResult struct{}         `xml:"DeleteIdentityResult"`
	ResponseMetadata     ResponseMetadata `xml:"ResponseMetadata"`
}

// XMLGetIdentityVerificationAttributesResponse is the XML response for GetIdentityVerificationAttributes.
type XMLGetIdentityVerificationAttributesResponse struct {
	XMLName                                 xml.Name                                   `xml:"GetIdentityVerificationAttributesResponse"`
	Xmlns                                   string                                     `xml:"xmlns,attr"`
	GetIdentityVerificationAttributesResult XMLGetIdentityVerificationAttributesResult `xml:"GetIdentityVerificationAttributesResult"`
	ResponseMetadata                        ResponseMetadata                           `xml:"ResponseMetadata"`
}

// XMLGetIdentityVerificationAttributesResult contains the result.
type XMLGetIdentityVerificationAttributesResult struct {
	VerificationAttributes XMLVerificationAttributes `xml:"VerificationAttributes"`
}

// XMLVerificationAttributes wraps the verification attribute entries.
type XMLVerificationAttributes struct {
	Entry []XMLVerificationEntry `xml:"entry"`
}

// XMLVerificationEntry is a single identity verification entry.
type XMLVerificationEntry struct {
	Key   string                        `xml:"key"`
	Value XMLVerificationAttributeValue `xml:"value"`
}

// XMLVerificationAttributeValue contains the verification status.
type XMLVerificationAttributeValue struct {
	VerificationStatus string `xml:"VerificationStatus"`
}

// ResponseMetadata contains the response metadata.
type ResponseMetadata struct {
	RequestID string `xml:"RequestId"`
}

// XMLErrorResponse is the XML error response for SES.
type XMLErrorResponse struct {
	XMLName   xml.Name       `xml:"ErrorResponse"`
	Xmlns     string         `xml:"xmlns,attr"`
	Error     XMLErrorDetail `xml:"Error"`
	RequestID string         `xml:"RequestId"`
}

// XMLErrorDetail contains error details.
type XMLErrorDetail struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// Error represents an SES error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}
