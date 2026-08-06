//nolint:tagliatelle // AWS Pinpoint SMS Voice v2 API uses PascalCase for JSON tags
package pinpointsmsvoicev2

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// SentTextMessage represents a sent text message for debugging purposes.
type SentTextMessage struct {
	MessageID              string    `json:"MessageId"`
	DestinationPhoneNumber string    `json:"DestinationPhoneNumber"`
	OriginationIdentity    string    `json:"OriginationIdentity,omitempty"`
	MessageBody            string    `json:"MessageBody,omitempty"`
	MessageType            string    `json:"MessageType,omitempty"`
	ConfigurationSetName   string    `json:"ConfigurationSetName,omitempty"`
	SentAt                 time.Time `json:"SentAt"`
}

// SendTextMessageInput is the request for SendTextMessage.
type SendTextMessageInput struct {
	DestinationPhoneNumber string `json:"DestinationPhoneNumber"`
	OriginationIdentity    string `json:"OriginationIdentity,omitempty"`
	MessageBody            string `json:"MessageBody,omitempty"`
	MessageType            string `json:"MessageType,omitempty"`
	ConfigurationSetName   string `json:"ConfigurationSetName,omitempty"`
}

// SendTextMessageOutput is the response for SendTextMessage.
type SendTextMessageOutput struct {
	MessageID string `json:"MessageId,omitempty"`
}

// GetSentTextMessagesResponse is the response for GetSentTextMessages.
type GetSentTextMessagesResponse struct {
	SentTextMessages []*SentTextMessage `json:"SentTextMessages"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error represents a service error.
type Error = service.CodedError
