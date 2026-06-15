package service

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// jsonErrorBody is the AWS JSON-protocol error envelope.
//
//nolint:tagliatelle // AWS JSON error envelope uses the literal key "__type".
type jsonErrorBody struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// WriteJSONError writes an AWS JSON-protocol error response: the given
// Content-Type and HTTP status, an x-amzn-RequestId header, and a body of
// {"__type": code, "message": message}. It is the shared implementation behind
// the per-service writeXxxError helpers for JSON-protocol services.
func WriteJSONError(w http.ResponseWriter, contentType, code, message string, status int) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Amzn-Requestid", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(jsonErrorBody{Type: code, Message: message})
}
