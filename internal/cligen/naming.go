package cligen

import (
	"strings"
	"unicode"
)

// initialisms are multi-letter acronyms that must be kept together as a
// single kebab-case word (e.g. "QueueURL" -> "queue-url", not "queue-u-r-l").
// Sorted longest-first so matching is greedy and unambiguous.
var initialisms = []string{
	"JSON", "HTML", "HTTP", "CIDR",
	"URL", "ARN", "AWS", "KMS", "SSE", "ACL", "MFA", "API", "TTL", "VPC",
	"ID",
}

// toKebabCase converts a Go exported identifier (struct field name, method
// name) into a kebab-case CLI flag/command name, e.g. "QueueURL" -> "queue-url",
// "GetQueueUrl" -> "get-queue-url".
func toKebabCase(name string) string {
	return strings.ToLower(strings.Join(splitWords(name), "-"))
}

// toWords splits a Go exported identifier into human-readable, space
// separated words with only the first word capitalized, e.g. "QueueName" ->
// "Queue name". Used for generated flag/command descriptions.
func toWords(name string) string {
	words := splitWords(name)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}

	if len(words) > 0 && words[0] != "" {
		words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
	}

	return strings.Join(words, " ")
}

// splitWords splits a Go exported identifier into its constituent words,
// treating known initialisms (see initialisms) as single words and grouping
// consecutive digits together.
func splitWords(name string) []string {
	runes := []rune(name)
	n := len(runes)

	var words []string

	for i := 0; i < n; {
		switch {
		case unicode.IsDigit(runes[i]):
			j := i + 1
			for j < n && unicode.IsDigit(runes[j]) {
				j++
			}

			words = append(words, string(runes[i:j]))
			i = j
		case matchInitialism(runes, i) != "":
			w := matchInitialism(runes, i)
			words = append(words, w)
			i += len(w)
		case unicode.IsUpper(runes[i]):
			j := i + 1
			for j < n && unicode.IsLower(runes[j]) {
				j++
			}

			words = append(words, string(runes[i:j]))
			i = j
		case unicode.IsLower(runes[i]):
			j := i + 1
			for j < n && unicode.IsLower(runes[j]) {
				j++
			}

			words = append(words, string(runes[i:j]))
			i = j
		default:
			i++
		}
	}

	return words
}

// matchInitialism returns the longest initialism matching runes at pos, or ""
// if none matches. A match is only accepted when it is not immediately
// followed by a lowercase letter (which would mean the uppercase run is
// actually the start of a normal capitalized word, e.g. "Url" is not "URL"+"l").
func matchInitialism(runes []rune, pos int) string {
	remaining := string(runes[pos:])

	for _, init := range initialisms {
		if !strings.HasPrefix(remaining, init) {
			continue
		}

		end := pos + len(init)
		if end == len(runes) || !unicode.IsLower(runes[end]) {
			return init
		}
	}

	return ""
}
