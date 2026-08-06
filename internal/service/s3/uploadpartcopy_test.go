package s3

import "testing"

// TestParseCopySourceRange exercises parseCopySourceRange's edge cases
// directly: this is pure string-parsing logic (whitespace tolerance,
// rejecting suffix/open/inverted ranges and the wrong unit) that isn't
// meaningfully distinguishable through the HTTP-level golden tests, which
// only observe "valid range" vs. "InvalidArgument" and can't exercise every
// malformed-header shape.
func TestParseCopySourceRange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		header string
		start  int64
		end    int64
		want   bool // valid
		isNil  bool // result is nil (absent header)
	}{
		{"", 0, 0, true, true},
		{"bytes=0-99", 0, 99, true, false},
		{"bytes=100-199", 100, 199, true, false},
		{"bytes= 0 - 99 ", 0, 99, true, false}, // tolerate spaces
		{"bytes=-99", 0, 0, false, false},      // suffix not allowed
		{"bytes=100-", 0, 0, false, false},     // open not allowed
		{"bytes=200-100", 0, 0, false, false},  // inverted
		{"items=0-99", 0, 0, false, false},     // wrong unit
	}

	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			got, err := parseCopySourceRange(tc.header)
			if tc.want && err != nil {
				t.Fatalf("want valid, got err=%v", err)
			}

			if !tc.want && err == nil {
				t.Fatalf("want error, got nil")
			}

			if tc.isNil && got != nil {
				t.Fatalf("want nil result, got %+v", got)
			}

			if !tc.isNil && got != nil && (got.Start != tc.start || got.End != tc.end) {
				t.Fatalf("got (%d, %d), want (%d, %d)", got.Start, got.End, tc.start, tc.end)
			}
		})
	}
}
