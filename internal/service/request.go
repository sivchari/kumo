package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ReadJSONRequest reads the HTTP request body and decodes it as JSON into v.
// An empty body is treated as a no-op (v is left unchanged), matching the
// behavior shared across the JSON-protocol service handlers.
func ReadJSONRequest(r *http.Request, v any) error {
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

type queryProtocolKey struct{}

// WithQueryProtocol marks r as having arrived over the AWS Query protocol,
// for services that also accept AWS JSON on the same handlers.
func WithQueryProtocol(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), queryProtocolKey{}, true))
}

// IsQueryProtocol reports whether r was marked by WithQueryProtocol.
func IsQueryProtocol(r *http.Request) bool {
	marked, _ := r.Context().Value(queryProtocolKey{}).(bool)

	return marked
}
