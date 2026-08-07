package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// outputFormatText is the --output value that prints a --query result as
// raw text instead of JSON.
const outputFormatText = "text"

// writeOutput is the single place every generated command (cli/gen_*.go)
// funnels its API response through, honoring the --query and --output
// persistent flags registered in NewRootCmd. With no --query it behaves like
// the previous json.NewEncoder(os.Stdout).Encode(out); with --query set, it
// runs a limited JMESPath-style lookup (see queryJSON) against out's JSON
// representation and, when --output=text, prints the matched value as plain
// text instead of JSON.
func writeOutput(out any) error {
	if queryFlag == "" {
		return encodeJSON(out)
	}

	data, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("failed to decode output: %w", err)
	}

	result, err := queryJSON(decoded, queryFlag)
	if err != nil {
		return err
	}

	if outputFormat != outputFormatText {
		return encodeJSON(result)
	}

	if _, err := fmt.Fprintln(os.Stdout, formatText(result)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

// queryJSON walks query (a dot-separated path such as
// "Table.AttributeDefinitions[0].AttributeName") over v, a value decoded
// from JSON via json.Unmarshal into an any (so objects are map[string]any
// and arrays are []any). This is a deliberately limited JMESPath subset:
// only field access and numeric array indexing are supported.
func queryJSON(v any, query string) (any, error) {
	cur := v

	for _, segment := range strings.Split(query, ".") {
		name, indexes, err := parseQuerySegment(segment)
		if err != nil {
			return nil, fmt.Errorf("failed to query %q: %w", query, err)
		}

		if name != "" {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("failed to query %q: field %q is not an object", query, name)
			}

			val, ok := m[name]
			if !ok {
				return nil, fmt.Errorf("failed to query %q: field %q not found", query, name)
			}

			cur = val
		}

		for _, idx := range indexes {
			arr, ok := cur.([]any)
			if !ok {
				return nil, fmt.Errorf("failed to query %q: index %d applied to a non-array value", query, idx)
			}

			if idx < 0 || idx >= len(arr) {
				return nil, fmt.Errorf("failed to query %q: index %d out of range (len %d)", query, idx, len(arr))
			}

			cur = arr[idx]
		}
	}

	return cur, nil
}

// parseQuerySegment splits one dot-separated query segment (e.g.
// "Foo[0][1]") into its field name ("Foo") and its array indexes, in order.
// name is empty when the segment is index-only (e.g. "[0]").
func parseQuerySegment(segment string) (string, []int, error) {
	name := segment

	var indexes []int

	for {
		start := strings.IndexByte(name, '[')
		if start == -1 {
			break
		}

		end := strings.IndexByte(name[start:], ']')
		if end == -1 {
			return "", nil, fmt.Errorf("unterminated %q in %q", "[", segment)
		}

		end += start

		idx, err := strconv.Atoi(name[start+1 : end])
		if err != nil {
			return "", nil, fmt.Errorf("invalid array index in %q: %w", segment, err)
		}

		indexes = append(indexes, idx)
		name = name[:start] + name[end+1:]
	}

	return name, indexes, nil
}

// formatText renders v (a value produced by queryJSON) as plain text for
// --output=text: strings and scalars print unquoted, everything else falls
// back to compact JSON.
func formatText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}

		return string(b)
	}
}
