package s3

import "testing"

// TestParseCopySource covers the shapes AWS clients send in the
// x-amz-copy-source header: plain, leading-slash, and URL-encoded.
// AWS S3 accepts all of these so kumo must too.
func TestParseCopySource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		source      string
		wantBucket  string
		wantKey     string
		wantVersion string
	}{
		{"plain", "bucket/key.txt", "bucket", "key.txt", ""},
		{"leading slash", "/bucket/key.txt", "bucket", "key.txt", ""},
		{"encoded separator", "bucket%2Fkey.txt", "bucket", "key.txt", ""},
		{"encoded subpath", "bucket/path%2Fto%2Fkey.txt", "bucket", "path/to/key.txt", ""},
		{"fully encoded with leading slash", "%2Fbucket%2Fkey.txt", "bucket", "key.txt", ""},
		{"plus preserved", "bucket/file+name.txt", "bucket", "file+name.txt", ""},
		{"version id", "bucket/key.txt?versionId=v1", "bucket", "key.txt", "v1"},
		{"encoded version id", "bucket%2Fkey.txt%3FversionId%3Dv1", "bucket", "key.txt", "v1"},
		{"no separator", "bucket", "", "", ""},
		{"empty", "", "", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotBucket, gotKey, gotVersion := parseCopySource(tc.source)
			if gotBucket != tc.wantBucket || gotKey != tc.wantKey || gotVersion != tc.wantVersion {
				t.Errorf("parseCopySource(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tc.source, gotBucket, gotKey, gotVersion, tc.wantBucket, tc.wantKey, tc.wantVersion)
			}
		})
	}
}
