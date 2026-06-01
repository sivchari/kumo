package execapi

import "strings"

// SplitPath splits a URL path into non-empty segments. "/" yields nil.
func SplitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}

	return strings.Split(trimmed, "/")
}

// MatchPath matches a path template against request segments. Template
// segments may be literal, a single-segment wildcard {name}, or a greedy
// wildcard {name+} matching the remainder. It returns captured parameters and
// a specificity score (number of literal segments matched), or ok=false.
func MatchPath(template string, reqSegs []string) (map[string]string, int, bool) {
	tmplSegs := SplitPath(template)
	params := map[string]string{}
	score := 0

	for i := range tmplSegs {
		seg := tmplSegs[i]

		// Greedy wildcard {name+} matches the entire remainder.
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "+}") {
			name := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "+}")
			params[name] = strings.Join(reqSegs[i:], "/")

			return params, score, true
		}

		if i >= len(reqSegs) {
			return nil, -1, false
		}

		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
			params[name] = reqSegs[i]

			continue
		}

		if seg != reqSegs[i] {
			return nil, -1, false
		}

		score++
	}

	if len(tmplSegs) != len(reqSegs) {
		return nil, -1, false
	}

	return params, score, true
}
