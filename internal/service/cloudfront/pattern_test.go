package cloudfront

import "testing"

const docQuestion = "*.doc?"

// TestMatchPathPattern covers the wildcard rules from the CloudFront
// developer guide: `*` is zero or more characters (across `/`), `?` exactly
// one, matching is case-sensitive and a leading `/` is implied.
func TestMatchPathPattern(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, pattern, path string
		want                bool
	}{
		{"star across slash", jpgPattern, "images/sample.jpg", true},
		{"star across nested slash", jpgPattern, "images/product1/sample.jpg", true},
		{"extension mismatch", jpgPattern, sampleGIF, false},
		{"leading slash on pattern", "/images/*.jpg", "images/sample.jpg", true},
		{"trailing star", "images/*", sampleGIF, true},
		{"leading star", "*.gif", sampleGIF, true},
		{"star spans path segments", "a*.jpg", "abra/cadabra/magic.jpg", true},
		{"question marks match exactly two", "a??.jpg", "ant.jpg", true},
		{"question marks reject more", "a??.jpg", "apple.jpg", false},
		{"question mark rejects fewer", docQuestion, "report.doc", false},
		{"question mark exactly one", docQuestion, "report.docx", true},
		{"question mark rejects two", docQuestion, "report.docxx", false},
		{"question mark matches one multibyte character", "a?b", "aéb", true},
		{"question mark rejects two characters", "a?b", "aééb", false},
		{"case sensitive extension", "*.jpg", "LOGO.JPG", false},
		{"case sensitive prefix", "API/*", "api/items", false},
		{"star needs the slash", apiPattern, "api", false},
		{"star matches empty", apiPattern, "api/", true},
		{"star matches deep path", apiPattern, "api/v1/items", true},
		{"lone star matches empty path", "*", "", true},
		{"lone star matches anything", "*", "anything/at/all", true},
		{"star matches decoded newline", "a*b", "a\nb", true},
		{"star matches only newline", "*", "\n", true},
		{"star matches trailing newline", "api/*", "api/line1\nline2", true},
		{"exact match", exactFile, exactFile, true},
		{"exact rejects suffix", exactFile, "exact.txt.bak", false},
		{"empty pattern matches only empty path", "", "", true},
		{"empty pattern rejects path", "", "x", false},
		{"consecutive stars", "a**b", "a/x/b", true},
		{"star backtracks to a later match", "*ab", "aab", true},
		{"star retries after a partial match", "a*b*c", "axbxbxc", true},
		{"star retries and fails", "a*b*c", "axbxbxd", false},
		{"trailing stars after full match", "ab**", "ab", true},
		{"literal after star must exist", "ab*c", "ab", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := matchPathPattern(tc.pattern, tc.path); got != tc.want {
				t.Errorf("matchPathPattern(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
			}
		})
	}
}

// TestResolveBehavior — ordered behaviours win in list order, the default
// behaviour is the fallback, and a distribution without one yields nil.
func TestResolveBehavior(t *testing.T) {
	t.Parallel()

	dist := &Distribution{DistributionConfig: &DistributionConfig{
		DefaultCacheBehavior: &DefaultCacheBehavior{TargetOriginID: originDefault},
		CacheBehaviors: &CacheBehaviors{Quantity: 2, Items: []CacheBehavior{
			{PathPattern: jpgPattern, DefaultCacheBehavior: DefaultCacheBehavior{TargetOriginID: originJPG}},
			{PathPattern: "images/*", DefaultCacheBehavior: DefaultCacheBehavior{TargetOriginID: "images"}},
		}},
	}}

	for path, want := range map[string]string{
		"images/a.jpg":     originJPG,
		"images/a.gif":     "images",
		"other/a.jpg":      originDefault,
		"":                 originDefault,
		"images/sub/b.jpg": originJPG,
	} {
		if got := resolveBehavior(dist, path); got == nil || got.TargetOriginID != want {
			t.Errorf("resolveBehavior(%q) = %+v, want target %q", path, got, want)
		}
	}

	if resolveBehavior(&Distribution{DistributionConfig: &DistributionConfig{}}, "x") != nil {
		t.Error("a distribution without a default behaviour must resolve to nil")
	}

	if resolveBehavior(&Distribution{}, "x") != nil {
		t.Error("a distribution without a config must resolve to nil")
	}
}

func TestValidatePathPatterns(t *testing.T) {
	t.Parallel()

	behaviors := func(patterns ...string) *DistributionConfig {
		items := make([]CacheBehavior, 0, len(patterns))
		for _, pattern := range patterns {
			items = append(items, CacheBehavior{PathPattern: pattern})
		}

		return &DistributionConfig{CacheBehaviors: &CacheBehaviors{Quantity: len(items), Items: items}}
	}

	for name, cfg := range map[string]*DistributionConfig{
		"duplicate":     behaviors(apiPattern, jpgPattern, apiPattern),
		"empty":         behaviors(apiPattern, ""),
		"only empty":    behaviors(""),
		"adjacent twin": behaviors(exactFile, exactFile),
	} {
		if err := validatePathPatterns(cfg); err == nil || errorCode(t, err) != errInvalidArgument {
			t.Errorf("%s: err = %v, want InvalidArgument", name, err)
		}
	}

	for name, cfg := range map[string]*DistributionConfig{
		"no behaviors":         {},
		"distinct":             behaviors(apiPattern, jpgPattern),
		"differs only by case": behaviors("API/*", apiPattern),
	} {
		if err := validatePathPatterns(cfg); err != nil {
			t.Errorf("%s: unexpected error %v", name, err)
		}
	}
}
