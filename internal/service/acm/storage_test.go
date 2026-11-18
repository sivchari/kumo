package acm

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// duplicateTagSnapshot is a snapshot saved before kumo checked tags: the
// certificate holds the key env twice.
const duplicateTagSnapshot = `{"certificates":{
"arn:aws:acm:us-east-1:000000000000:certificate/dup":{
  "CertificateArn":"arn:aws:acm:us-east-1:000000000000:certificate/dup",
  "DomainName":"dup.example.com","Status":"PENDING_VALIDATION","Type":"AMAZON_ISSUED",
  "Tags":[{"Key":"env","Value":"dev"},{"Key":"team","Value":"web"},{"Key":"env","Value":"prod"}]}
}}`

// TestTags_DuplicateKeysFromSnapshot loads a certificate holding a tag key
// twice: the key is listed once, with its last value, and a later tagging
// call updates that one entry.
func TestTags_DuplicateKeysFromSnapshot(t *testing.T) {
	t.Parallel()

	const (
		arn    = "arn:aws:acm:us-east-1:000000000000:certificate/dup"
		envKey = "env"
	)

	store := NewMemoryStorage()
	if err := json.Unmarshal([]byte(duplicateTagSnapshot), store); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	ctx := t.Context()
	team := Tag{Key: "team", Value: "web"}

	loaded, err := store.ListTagsForCertificate(ctx, arn)
	if err != nil {
		t.Fatalf("ListTagsForCertificate: %v", err)
	}

	if want := []Tag{{envKey, "prod"}, team}; !slices.Equal(loaded, want) {
		t.Errorf("loaded tags %v, want %v", loaded, want)
	}

	// A certificate stored with a key twice, as before the load normalized
	// it, is updated in one entry too.
	store.Certificates[arn].Tags = []Tag{{envKey, "dev"}, team, {envKey, "prod"}}

	if err := store.AddTagsToCertificate(ctx, arn, []Tag{{envKey, "stage"}, {"owner", "alice"}}); err != nil {
		t.Fatalf("AddTagsToCertificate: %v", err)
	}

	added, err := store.ListTagsForCertificate(ctx, arn)
	if err != nil {
		t.Fatalf("ListTagsForCertificate: %v", err)
	}

	if want := []Tag{{envKey, "stage"}, team, {"owner", "alice"}}; !slices.Equal(added, want) {
		t.Errorf("tags after AddTagsToCertificate %v, want %v", added, want)
	}
}

// TestNewMemoryStorage_RegionFromEnv checks that certificate ARNs carry the
// region from AWS_DEFAULT_REGION, and us-east-1 when it is empty.
func TestNewMemoryStorage_RegionFromEnv(t *testing.T) {
	cases := []struct {
		name       string
		env        string
		wantRegion string
	}{
		{"AWS_DEFAULT_REGION", "eu-west-1", "eu-west-1"},
		{"empty falls back to us-east-1", "", "us-east-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AWS_DEFAULT_REGION", tc.env)

			store := NewMemoryStorage()
			ctx := t.Context()

			requested, err := store.RequestCertificate(ctx, &RequestCertificateInput{DomainName: "region.example.com"})
			if err != nil {
				t.Fatalf("RequestCertificate: %v", err)
			}

			imported, err := store.ImportCertificate(ctx, &ImportCertificateInput{
				Certificate: []byte("cert"),
				PrivateKey:  []byte("key"),
			})
			if err != nil {
				t.Fatalf("ImportCertificate: %v", err)
			}

			wantPrefix := "arn:aws:acm:" + tc.wantRegion + ":" + accountID + ":certificate/"

			for _, arn := range []string{requested.CertificateArn, imported.CertificateArn} {
				if !strings.HasPrefix(arn, wantPrefix) {
					t.Errorf("ARN %s, want prefix %s", arn, wantPrefix)
				}
			}
		})
	}
}
