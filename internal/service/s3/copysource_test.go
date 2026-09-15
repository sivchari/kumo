package s3

import "testing"

// wantBucketName/wantKeyName are the expected bucket/key names shared by
// TestParseCopySource's cases (they all reference the same "bucket/key.txt").
const (
	wantBucketName = "bucket"
	wantKeyName    = "key.txt"
)

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
		{"plain", "bucket/key.txt", wantBucketName, wantKeyName, ""},
		{"leading slash", "/bucket/key.txt", wantBucketName, wantKeyName, ""},
		{"encoded separator", "bucket%2Fkey.txt", wantBucketName, wantKeyName, ""},
		{"encoded subpath", "bucket/path%2Fto%2Fkey.txt", wantBucketName, "path/to/key.txt", ""},
		{"fully encoded with leading slash", "%2Fbucket%2Fkey.txt", wantBucketName, wantKeyName, ""},
		{"plus preserved", "bucket/file+name.txt", wantBucketName, "file+name.txt", ""},
		{"version id", "bucket/key.txt?versionId=v1", wantBucketName, wantKeyName, "v1"},
		{"encoded version id", "bucket%2Fkey.txt%3FversionId%3Dv1", wantBucketName, wantKeyName, "v1"},
		{"no separator", wantBucketName, "", "", ""},
		{testCaseEmpty, "", "", "", ""},
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
