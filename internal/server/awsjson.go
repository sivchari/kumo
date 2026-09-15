package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Error response field names, shared by the JSON, Query, and CBOR protocol
// error writers (awsjson.go/awsquery.go/cbor_dispatcher.go).
const (
	errFieldType    = "__type"
	errFieldMessage = "message"
)

// JSONServiceHandler handles JSON protocol requests for a specific service.
type JSONServiceHandler func(w http.ResponseWriter, r *http.Request)

// JSONProtocolDispatcher routes AWS JSON 1.0 protocol requests to the appropriate service
// based on the X-Amz-Target header prefix.
type JSONProtocolDispatcher struct {
	// handlers maps target prefix to service handler
	// e.g., "AmazonSQS" -> SQS handler, "DynamoDB_20120810" -> DynamoDB handler
	handlers map[string]JSONServiceHandler
}

// NewJSONProtocolDispatcher creates a new JSON protocol dispatcher.
func NewJSONProtocolDispatcher() *JSONProtocolDispatcher {
	return &JSONProtocolDispatcher{
		handlers: make(map[string]JSONServiceHandler),
	}
}

// Register registers a service handler for the given target prefix.
// The prefix is the part before the dot in X-Amz-Target header,
// e.g., "AmazonSQS" for "AmazonSQS.CreateQueue" or
// "DynamoDB_20120810" for "DynamoDB_20120810.CreateTable".
func (d *JSONProtocolDispatcher) Register(prefix string, handler JSONServiceHandler) {
	d.handlers[prefix] = handler
}

// ServeHTTP implements http.Handler and dispatches to the appropriate service.
func (d *JSONProtocolDispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	if target == "" {
		writeJSONError(w, "MissingTargetHeader", "X-Amz-Target header is required", http.StatusBadRequest)

		return
	}

	// Extract prefix from target (e.g., "AmazonSQS" from "AmazonSQS.CreateQueue")
	parts := strings.SplitN(target, ".", 2)
	if len(parts) < 2 {
		writeJSONError(w, "InvalidTargetHeader", "X-Amz-Target header format is invalid", http.StatusBadRequest)

		return
	}

	prefix := parts[0]
	handler, ok := d.handlers[prefix]

	if !ok {
		writeJSONError(w, "UnknownService", "Unknown service: "+prefix, http.StatusBadRequest)

		return
	}

	handler(w, r)
}

// writeJSONError writes an AWS JSON error response.
func writeJSONError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		errFieldType:    code,
		errFieldMessage: message,
	})
}
