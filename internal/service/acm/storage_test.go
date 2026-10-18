package acm

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Literals reused across the ACM tests.
const (
	absentCertificateARN = "arn:aws:acm:us-east-1:000000000000:certificate/absent"
	tagKeyEnv            = "env"
	tagKeyTeam           = "team"
	tagKeyOwner          = "owner"
	tagKeyReserved       = "aws:reserved"
	tagValDev            = "dev"
	tagValProd           = "prod"
	tagValWeb            = "web"
)

func requireCode(t *testing.T, err error, want string) {
	t.Helper()

	var acmErr *Error
	if !errors.As(err, &acmErr) {
		t.Fatalf("error: got %v, want code %s", err, want)
	}

	if acmErr.Code != want {
		t.Fatalf("error code: got %s, want %s", acmErr.Code, want)
	}
}

func TestRequestCertificate_ValidationStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		method           string
		wantStatus       string
		wantDomainStatus string
		wantIssued       bool
		wantRecord       bool
	}{
		{"dns", "DNS", statusIssued, validationStatusSuccess, true, true},
		{"default is dns", "", statusIssued, validationStatusSuccess, true, true},
		{"email stays pending", "EMAIL", statusPendingValidation, statusPendingValidation, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := NewMemoryStorage()

			cert, err := store.RequestCertificate(t.Context(), &RequestCertificateInput{
				DomainName:              "example.com",
				SubjectAlternativeNames: []string{"www.example.com"},
				ValidationMethod:        tc.method,
			})
			if err != nil {
				t.Fatalf("RequestCertificate: %v", err)
			}

			if cert.Status != tc.wantStatus {
				t.Errorf("Status: got %s, want %s", cert.Status, tc.wantStatus)
			}

			if (cert.IssuedAt != nil) != tc.wantIssued {
				t.Errorf("IssuedAt: got %v, want set=%v", cert.IssuedAt, tc.wantIssued)
			}

			if len(cert.DomainValidationOptions) != 2 {
				t.Fatalf("DomainValidationOptions: got %d entries, want 2", len(cert.DomainValidationOptions))
			}

			for _, dv := range cert.DomainValidationOptions {
				if dv.ValidationStatus != tc.wantDomainStatus {
					t.Errorf("%s ValidationStatus: got %s, want %s", dv.DomainName, dv.ValidationStatus, tc.wantDomainStatus)
				}

				if (dv.ResourceRecord != nil) != tc.wantRecord {
					t.Errorf("%s ResourceRecord: got %v, want set=%v", dv.DomainName, dv.ResourceRecord, tc.wantRecord)
				}
			}
		})
	}
}

func TestRequestCertificate_ValidationRecords(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := t.Context()

	request := func(domain string, sans ...string) map[string]ResourceRecord {
		t.Helper()

		cert, err := store.RequestCertificate(ctx, &RequestCertificateInput{
			DomainName:              domain,
			SubjectAlternativeNames: sans,
			ValidationMethod:        validationMethodDNS,
		})
		if err != nil {
			t.Fatalf("RequestCertificate: %v", err)
		}

		records := make(map[string]ResourceRecord, len(cert.DomainValidationOptions))
		for _, dv := range cert.DomainValidationOptions {
			records[dv.DomainName] = *dv.ResourceRecord
		}

		return records
	}

	first := request("example.com", "*.example.com", "www.example.com")

	base := first["example.com"]
	if !strings.HasPrefix(base.Name, "_") || !strings.HasSuffix(base.Name, ".example.com.") {
		t.Errorf("record name %q, want _<token>.example.com.", base.Name)
	}

	if !strings.HasPrefix(base.Value, "_") || !strings.HasSuffix(base.Value, ".acm-validations.aws.") {
		t.Errorf("record value %q, want _<token>.acm-validations.aws.", base.Value)
	}

	if got := first["*.example.com"]; got != base {
		t.Errorf("wildcard record %+v, want the base domain's %+v", got, base)
	}

	www := first["www.example.com"]
	if www.Name == base.Name || www.Value == base.Value {
		t.Errorf("www.example.com record %+v shares a string with example.com's %+v", www, base)
	}

	if !strings.HasSuffix(www.Name, ".www.example.com.") {
		t.Errorf("record name %q, want _<token>.www.example.com.", www.Name)
	}

	// A later certificate for the same domains reuses the same records.
	second := request("*.example.com", "www.example.com")
	if second["*.example.com"] != base || second["www.example.com"] != www {
		t.Errorf("records changed across requests: got %+v, want %+v", second, first)
	}
}

func TestCertificateTags_Lifecycle(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := t.Context()

	cert, err := store.RequestCertificate(ctx, &RequestCertificateInput{
		DomainName: "tags.example.com",
		Tags:       []Tag{{Key: tagKeyEnv, Value: tagValDev}},
	})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	assertTags := func(t *testing.T, want []Tag) {
		t.Helper()

		got, err := store.ListTagsForCertificate(ctx, cert.CertificateArn)
		if err != nil {
			t.Fatalf("ListTagsForCertificate: %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("tags: got %v, want %v", got, want)
		}
	}

	assertTags(t, []Tag{{Key: tagKeyEnv, Value: tagValDev}})

	if err := store.AddTagsToCertificate(ctx, cert.CertificateArn, []Tag{
		{Key: tagKeyEnv, Value: tagValProd},
		{Key: tagKeyTeam, Value: tagValWeb},
		{Key: tagKeyOwner, Value: "alice"},
	}); err != nil {
		t.Fatalf("AddTagsToCertificate: %v", err)
	}

	assertTags(t, []Tag{{Key: tagKeyEnv, Value: tagValProd}, {Key: tagKeyTeam, Value: tagValWeb}, {Key: tagKeyOwner, Value: "alice"}})

	stale := "dev"
	current := tagValWeb

	if err := store.RemoveTagsFromCertificate(ctx, cert.CertificateArn, []TagToRemove{
		{Key: tagKeyEnv, Value: &stale},
		{Key: tagKeyTeam, Value: &current},
		{Key: tagKeyOwner},
	}); err != nil {
		t.Fatalf("RemoveTagsFromCertificate: %v", err)
	}

	// env is kept because the given value did not match.
	assertTags(t, []Tag{{Key: tagKeyEnv, Value: tagValProd}})
}

func TestCertificateTags_Errors(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := t.Context()

	cert, err := store.RequestCertificate(ctx, &RequestCertificateInput{DomainName: "errors.example.com"})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	_, err = store.ListTagsForCertificate(ctx, absentCertificateARN)
	requireCode(t, err, errNotFound)

	requireCode(t, store.AddTagsToCertificate(ctx, absentCertificateARN, []Tag{{Key: "k"}}), errNotFound)
	requireCode(t, store.RemoveTagsFromCertificate(ctx, absentCertificateARN, []TagToRemove{{Key: "k"}}), errNotFound)

	requireCode(t, store.AddTagsToCertificate(ctx, cert.CertificateArn, []Tag{{Key: tagKeyReserved}}), errInvalidTag)
	requireCode(t, store.RemoveTagsFromCertificate(ctx, cert.CertificateArn, []TagToRemove{{Key: tagKeyReserved}}), errInvalidTag)

	_, err = store.RequestCertificate(ctx, &RequestCertificateInput{
		DomainName: "reserved.example.com",
		Tags:       []Tag{{Key: tagKeyReserved}},
	})
	requireCode(t, err, errInvalidTag)

	tags := make([]Tag, maxTagsPerCertificate)
	for i := range tags {
		tags[i] = Tag{Key: fmt.Sprintf("key-%d", i)}
	}

	if err := store.AddTagsToCertificate(ctx, cert.CertificateArn, tags); err != nil {
		t.Fatalf("AddTagsToCertificate with %d tags: %v", len(tags), err)
	}

	requireCode(t, store.AddTagsToCertificate(ctx, cert.CertificateArn, []Tag{{Key: "one-too-many"}}), errTooManyTags)

	// Overwriting an existing key does not count as a new tag.
	if err := store.AddTagsToCertificate(ctx, cert.CertificateArn, []Tag{{Key: "key-0", Value: "v"}}); err != nil {
		t.Fatalf("AddTagsToCertificate overwrite at the limit: %v", err)
	}
}

func TestImportCertificate_Tags(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := t.Context()

	cert, err := store.ImportCertificate(ctx, &ImportCertificateInput{
		Certificate: []byte("cert"),
		PrivateKey:  []byte("key"),
		Tags:        []Tag{{Key: tagKeyEnv, Value: tagValDev}},
	})
	if err != nil {
		t.Fatalf("ImportCertificate: %v", err)
	}

	got, err := store.ListTagsForCertificate(ctx, cert.CertificateArn)
	if err != nil {
		t.Fatalf("ListTagsForCertificate: %v", err)
	}

	if want := []Tag{{Key: tagKeyEnv, Value: tagValDev}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tags: got %v, want %v", got, want)
	}
}
