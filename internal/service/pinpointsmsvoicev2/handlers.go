package pinpointsmsvoicev2

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// DispatchAction routes requests based on the X-Amz-Target header.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	if target == "" {
		writeError(w, errInvalidParameter, "Missing X-Amz-Target header", http.StatusBadRequest)

		return
	}

	parts := strings.Split(target, ".")
	if len(parts) != 2 {
		writeError(w, errInvalidParameter, "Invalid X-Amz-Target header", http.StatusBadRequest)

		return
	}

	operation := parts[1]

	switch operation {
	case "SendTextMessage":
		s.SendTextMessage(w, r)
	default:
		writeError(w, errInvalidParameter, fmt.Sprintf("Unknown operation: %s", operation), http.StatusBadRequest)
	}
}

// SendTextMessage handles the SendTextMessage operation.
func (s *Service) SendTextMessage(w http.ResponseWriter, r *http.Request) {
	var req SendTextMessageInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	messageID, err := s.storage.SendTextMessage(r.Context(), &req)
	if err != nil {
		var sErr *Error
		if errors.As(err, &sErr) {
			writeError(w, sErr.Code, sErr.Message, http.StatusBadRequest)

			return
		}

		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, SendTextMessageOutput{
		MessageID: messageID,
	})
}

// GetSentTextMessages handles the GetSentTextMessages operation.
func (s *Service) GetSentTextMessages(w http.ResponseWriter, r *http.Request) {
	messages, err := s.storage.GetSentTextMessages(r.Context())
	if err != nil {
		writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)

		return
	}

	writeJSONResponse(w, GetSentTextMessagesResponse{
		SentTextMessages: messages,
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
