package acm

import (
	"errors"
	"strings"
	"testing"
)

// validationEmail is the EMAIL validation method, which keeps a requested
// certificate pending.
const validationEmail = "EMAIL"

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

			requested, err := store.RequestCertificate(ctx, &RequestCertificateInput{
				DomainName:       "region.example.com",
				ValidationMethod: validationEmail,
			})
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
		{"email stays pending", validationEmail, statusPendingValidation, statusPendingValidation, false, false},
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
