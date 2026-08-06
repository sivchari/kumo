package sfn

import (
	"bytes"
	"encoding/json"
)

// jsonDeepEqual reports whether a and b represent the same JSON value. Each
// argument may be an already-decoded Go value (e.g. map[string]any,
// []any, from unmarshaling a test fixture) or a raw JSON document (a
// string), so callers are not sensitive to map key ordering or to
// formatting differences between two JSON documents.
func jsonDeepEqual(a, b any) bool {
	aJSON, ok := normalizedJSON(a)
	if !ok {
		return false
	}

	bJSON, ok := normalizedJSON(b)
	if !ok {
		return false
	}

	return bytes.Equal(aJSON, bJSON)
}

// normalizedJSON returns the canonical JSON encoding of v, and whether
// encoding succeeded. A string v is first decoded as a JSON document (so
// two differently formatted documents still compare equal) before being
// re-encoded; any other v is encoded as-is.
func normalizedJSON(v any) ([]byte, bool) {
	if s, ok := v.(string); ok {
		var decoded any
		if err := json.Unmarshal([]byte(s), &decoded); err != nil {
			return nil, false
		}

		v = decoded
	}

	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}

	return b, true
}
