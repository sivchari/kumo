package s3

import (
	"regexp"
	"testing"
)

var hexPattern = regexp.MustCompile(`^[0-9a-f]*$`)

func TestRandomHex_LengthAndUniqueness(t *testing.T) {
	t.Parallel()

	for _, n := range []int{0, 1, 8, 16} {
		got := randomHex(n)
		if len(got) != 2*n {
			t.Errorf("randomHex(%d) length = %d, want %d", n, len(got), 2*n)
		}

		if !hexPattern.MatchString(got) {
			t.Errorf("randomHex(%d) = %q, want lowercase hex", n, got)
		}
	}

	if a, b := randomHex(16), randomHex(16); a == b {
		t.Errorf("consecutive randomHex(16) calls returned the same value %q", a)
	}
}

func TestGenerateUploadID_Format(t *testing.T) {
	t.Parallel()

	id := generateUploadID()
	if len(id) != 32 || !hexPattern.MatchString(id) {
		t.Errorf("generateUploadID() = %q, want 32 hex characters", id)
	}

	if other := generateUploadID(); id == other {
		t.Errorf("consecutive generateUploadID() calls returned the same value %q", id)
	}
}
