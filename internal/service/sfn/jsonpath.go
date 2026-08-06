package sfn

import (
	"fmt"
	"strings"
)

// resolveAnyJSONPath resolves "$" or "$.field[.field...]" against an
// already-decoded JSON value of any shape. Unlike other JSONPath resolvers,
// the root need not be an object, so Map's ItemsPath can select an array.
func resolveAnyJSONPath(path string, data any) (any, error) {
	if path == "$" {
		return data, nil
	}

	if !strings.HasPrefix(path, "$.") {
		return nil, fmt.Errorf("unsupported JSONPath %q: must start with $", path)
	}

	current := data

	for _, part := range strings.Split(strings.TrimPrefix(path, "$."), ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("jsonPath %q: cannot access field %q on non-object", path, part)
		}

		val, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("jsonPath %q: field %q not found", path, part)
		}

		current = val
	}

	return current, nil
}
