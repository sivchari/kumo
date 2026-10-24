package route53

import "testing"

// TestNormalizeDNSName pins the canonical hosted zone name: a trailing dot,
// ASCII A-Z folded to a-z, and every other byte kept as sent.
func TestNormalizeDNSName(t *testing.T) {
	t.Parallel()

	const canonical = "example.com."

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"mixed case without trailing dot", "Example.COM", canonical},
		{"mixed case with trailing dot", "Example.COM.", canonical},
		{"already canonical", canonical, canonical},
		{"empty", "", ""},
		{"bytes next to A-Z are kept", "@AZ[.Example.COM", "@az[.example.com."},
		{"non-ASCII letters are kept", "EX\u00c4MPLE.com", "ex\u00c4mple.com."},
		{"invalid UTF-8 is kept", "Ex\xe4mple.COM", "ex\xe4mple.com."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeDNSName(tt.in); got != tt.want {
				t.Errorf("normalizeDNSName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
