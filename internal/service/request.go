package service

import (
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
