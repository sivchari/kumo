// Package awsname holds resource-name validators shared between AWS service
// emulators (currently SNS topics, SQS queues — same character rules, different
// length caps).
package awsname

import "strings"

// IsValidMessagingResource returns true when name satisfies the character /
// suffix rules AWS applies to SNS topic and SQS queue names:
//
//   - 1..maxLen characters
//   - alphanumeric, underscore, hyphen
//   - a single trailing ".fifo" is permitted (FIFO queue / topic), and the
//     pre-suffix base must be non-empty; no other '.' is allowed
func IsValidMessagingResource(name string, maxLen int) bool {
	if name == "" || len(name) > maxLen {
		return false
	}

	base := name
	if strings.HasSuffix(name, ".fifo") {
		base = strings.TrimSuffix(name, ".fifo")
		if base == "" {
			return false
		}
	} else if strings.Contains(name, ".") {
		return false
	}

	for _, r := range base {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			continue
		}

		return false
	}

	return true
}
