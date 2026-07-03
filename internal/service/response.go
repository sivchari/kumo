package service

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// Content types used by the AWS JSON protocols.
const (
	ContentTypeAmzJSON10 = "application/x-amz-json-1.0"
	ContentTypeAmzJSON11 = "application/x-amz-json-1.1"
	ContentTypeJSON      = "application/json"
)

// WriteJSONResponse writes v as a JSON body with HTTP 200 OK, the given
// Content-Type, and a generated x-amzn-RequestId header. This is the shared
// implementation behind the per-service writeJSONResponse helpers.
func WriteJSONResponse(w http.ResponseWriter, contentType string, v any) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Amzn-Requestid", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}
