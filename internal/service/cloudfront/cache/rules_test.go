package cache

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var (
	now    = time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	defCfg = DistributionConfig{
		MinTTL:     0,
		DefaultTTL: 24 * time.Hour,
		MaxTTL:     365 * 24 * time.Hour,
	}
)

// TestEffectiveTTL_PrecedenceTable exercises the CloudFront precedence
// chain s-maxage > max-age > Expires > DefaultTTL. Each row pins one
// header combination and the expected resulting TTL.
func TestEffectiveTTL_PrecedenceTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		headers http.Header
		want    time.Duration
	}{
		{
			name:    "no headers → DefaultTTL",
			headers: http.Header{},
			want:    24 * time.Hour,
		},
		{
			name:    "max-age wins over DefaultTTL",
			headers: http.Header{"Cache-Control": {"max-age=60"}},
			want:    60 * time.Second,
		},
		{
			name:    "s-maxage wins over max-age",
			headers: http.Header{"Cache-Control": {"max-age=60, s-maxage=120"}},
			want:    120 * time.Second,
		},
		{
			name:    "Expires used when no Cache-Control",
			headers: http.Header{"Expires": {now.Add(5 * time.Minute).UTC().Format(http.TimeFormat)}},
			want:    5 * time.Minute,
		},
		{
			name: "Expires ignored if Cache-Control present",
			headers: http.Header{
				"Cache-Control": {"max-age=42"},
				"Expires":       {now.Add(time.Hour).UTC().Format(http.TimeFormat)},
			},
			want: 42 * time.Second,
		},
		{
			name:    "no-store → 0",
			headers: http.Header{"Cache-Control": {"no-store, max-age=60"}},
			want:    0,
		},
		{
			name:    "private → 0 (CloudFront treats as do-not-store)",
			headers: http.Header{"Cache-Control": {"private, max-age=60"}},
			want:    0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EffectiveTTL(tc.headers, defCfg, now)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEffectiveTTL_Clamp covers the [MinTTL, MaxTTL] guard around the
// raw value. CloudFront clamps regardless of where the TTL came from.
func TestEffectiveTTL_Clamp(t *testing.T) {
	t.Parallel()

	cfg := DistributionConfig{MinTTL: 10 * time.Second, DefaultTTL: time.Hour, MaxTTL: time.Minute}

	got := EffectiveTTL(http.Header{"Cache-Control": {"max-age=1"}}, cfg, now)
	if got != 10*time.Second {
		t.Fatalf("clamp to MinTTL: got %v, want 10s", got)
	}

	got = EffectiveTTL(http.Header{"Cache-Control": {"max-age=99999"}}, cfg, now)
	if got != time.Minute {
		t.Fatalf("clamp to MaxTTL: got %v, want 1m", got)
	}
}

// TestIsCacheable enumerates the directives and status codes that
// flip cacheability either way.
func TestIsCacheable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		headers http.Header
		status  int
		want    bool
	}{
		{"200 with no directives", http.Header{}, 200, true},
		{"200 + no-store", http.Header{"Cache-Control": {"no-store"}}, 200, false},
		{"200 + private", http.Header{"Cache-Control": {"private"}}, 200, false},
		{"301 redirect cacheable", http.Header{}, 301, true},
		{"500 not cacheable by default", http.Header{}, 500, false},
		{"404 cacheable (negative caching)", http.Header{}, 404, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := IsCacheable(tc.headers, tc.status)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestMustRevalidate covers no-cache and must-revalidate.
func TestMustRevalidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		header string
		want   bool
	}{
		{"", false},
		{"max-age=60", false},
		{"no-cache", true},
		{"must-revalidate", true},
		{"max-age=60, no-cache", true},
	}

	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			h := http.Header{}
			if tc.header != "" {
				h.Set("Cache-Control", tc.header)
			}

			if got := MustRevalidate(h); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestVaryHeaders checks normalisation: case-insensitive, deduped,
// stable order. Multi-line Vary headers are concatenated.
func TestVaryHeaders(t *testing.T) {
	t.Parallel()

	h := http.Header{}
	h.Add("Vary", "Accept-Language")
	h.Add("Vary", "accept-encoding, ACCEPT-LANGUAGE")

	got := VaryHeaders(h)
	want := []string{"accept-encoding", "accept-language"}

	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d (%v)", len(got), len(want), got)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// TestVaryDisablesCache verifies the `Vary: *` short-circuit.
func TestVaryDisablesCache(t *testing.T) {
	t.Parallel()

	cases := []struct {
		header string
		want   bool
	}{
		{"", false},
		{"Accept-Language", false},
		{"*", true},
		{"Accept-Language, *", true},
	}

	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			h := http.Header{}
			if tc.header != "" {
				h.Set("Vary", tc.header)
			}

			if got := VaryDisablesCache(h); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestKey_VariantSeparation confirms that two requests differing
// only in a Vary'd header produce distinct keys.
func TestKey_VariantSeparation(t *testing.T) {
	t.Parallel()

	r1 := httptest.NewRequest(http.MethodGet, "/img.png?w=100", http.NoBody)
	r1.Header.Set("Accept-Language", "en")

	r2 := httptest.NewRequest(http.MethodGet, "/img.png?w=100", http.NoBody)
	r2.Header.Set("Accept-Language", "ja")

	vary := []string{"accept-language"}

	k1 := Key(r1, vary)
	k2 := Key(r2, vary)

	if k1 == k2 {
		t.Fatalf("Vary'd headers should split keys, both = %q", k1)
	}

	r2.Header.Set("Accept-Language", "en")

	if Key(r1, vary) != Key(r2, vary) {
		t.Fatalf("same Vary value should collide; got %q vs %q", k1, Key(r2, vary))
	}
}

// TestKey_QueryStringStable ensures the query-string part of the
// key is order-independent (foo=1&bar=2 and bar=2&foo=1 collide).
func TestKey_QueryStringStable(t *testing.T) {
	t.Parallel()

	a := httptest.NewRequest(http.MethodGet, "/p?foo=1&bar=2", http.NoBody)
	b := httptest.NewRequest(http.MethodGet, "/p?bar=2&foo=1", http.NoBody)

	if Key(a, nil) != Key(b, nil) {
		t.Fatalf("query order should not matter:\n  a=%q\n  b=%q", Key(a, nil), Key(b, nil))
	}
}
