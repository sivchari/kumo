package route53

import (
	"strings"
	"testing"
)

func TestNewIDShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		gen    func() string
		prefix string
		path   string
	}{
		{name: "hosted zone", gen: newHostedZoneID, prefix: "Z", path: "/hostedzone/"},
		{name: "change", gen: newChangeID, prefix: "C", path: "/change/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			const draws = 10000

			seen := make(map[string]struct{}, draws)

			for range draws {
				id := tt.gen()

				if len(id) != 1+idRandomLength {
					t.Fatalf("id %q has length %d, want %d", id, len(id), 1+idRandomLength)
				}

				if got := len(tt.path + id); got > 32 {
					t.Fatalf("%s%s has length %d, want at most 32", tt.path, id, got)
				}

				if !strings.HasPrefix(id, tt.prefix) {
					t.Fatalf("id %q does not start with %q", id, tt.prefix)
				}

				if i := strings.IndexFunc(id, func(r rune) bool { return !strings.ContainsRune(idAlphabet, r) }); i >= 0 {
					t.Fatalf("id %q has %q outside [A-Z0-9]", id, id[i])
				}

				if _, dup := seen[id]; dup {
					t.Fatalf("id %q generated twice in %d draws", id, draws)
				}

				seen[id] = struct{}{}
			}
		})
	}
}

func TestNewIDUsesWholeAlphabet(t *testing.T) {
	t.Parallel()

	counts := make(map[byte]int, len(idAlphabet))

	for range 2000 {
		id := newHostedZoneID()
		for i := 1; i < len(id); i++ {
			counts[id[i]]++
		}
	}

	for i := range len(idAlphabet) {
		if counts[idAlphabet[i]] == 0 {
			t.Errorf("character %q never generated", idAlphabet[i])
		}
	}
}
