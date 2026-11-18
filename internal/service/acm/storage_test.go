package acm

import (
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
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

// fakeClock is a settable clock for MemoryStorage.now.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.t = c.t.Add(d)
}

// newClockedStorage returns a storage whose clock starts at a fixed time
// and that keeps DNS-validated certificates pending for delay.
func newClockedStorage(delay time.Duration) (*MemoryStorage, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)}
	store := NewMemoryStorage(WithDNSValidationDelay(delay))
	store.now = clock.now

	return store, clock
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
	if !regexp.MustCompile(`^_[0-9a-f]{32}\.example\.com\.$`).MatchString(base.Name) {
		t.Errorf("record name %q, want _<token>.example.com.", base.Name)
	}

	if !regexp.MustCompile(`^_[0-9a-f]{32}\.[a-z]{10}\.acm-validations\.aws\.$`).MatchString(base.Value) {
		t.Errorf("record value %q, want _<token>.<label>.acm-validations.aws.", base.Value)
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

// assertPendingDNS checks that the certificate under arn is
// PENDING_VALIDATION with a validation record per domain, and that
// GetCertificate refuses it.
func assertPendingDNS(t *testing.T, store *MemoryStorage, arn, when string) {
	t.Helper()

	got, err := store.DescribeCertificate(t.Context(), arn)
	if err != nil {
		t.Fatalf("DescribeCertificate: %v", err)
	}

	if got.Status != statusPendingValidation || got.IssuedAt != nil {
		t.Errorf("%s: Status %s IssuedAt %v, want PENDING_VALIDATION and no IssuedAt", when, got.Status, got.IssuedAt)
	}

	for _, dv := range got.DomainValidationOptions {
		if dv.ValidationStatus != statusPendingValidation || dv.ResourceRecord == nil {
			t.Errorf("%s: %s ValidationStatus %s record %v, want PENDING_VALIDATION with a record",
				when, dv.DomainName, dv.ValidationStatus, dv.ResourceRecord)
		}
	}

	_, err = store.GetCertificate(t.Context(), arn)
	requireCode(t, err, errRequestPending)
}

// assertIssuedAt checks that cert is ISSUED as of issuedAt, with every
// domain's validation succeeded.
func assertIssuedAt(t *testing.T, cert *Certificate, issuedAt time.Time) {
	t.Helper()

	if cert.Status != statusIssued || cert.IssuedAt == nil || !cert.IssuedAt.Equal(issuedAt) {
		t.Errorf("Status %s IssuedAt %v, want ISSUED at %v", cert.Status, cert.IssuedAt, issuedAt)
	}

	for _, dv := range cert.DomainValidationOptions {
		if dv.ValidationStatus != validationStatusSuccess {
			t.Errorf("%s ValidationStatus %s, want SUCCESS", dv.DomainName, dv.ValidationStatus)
		}
	}
}

// listCount returns how many certificates ListCertificates returns with
// the given status.
func listCount(t *testing.T, store *MemoryStorage, status string) int {
	t.Helper()

	certs, _, err := store.ListCertificates(t.Context(), []string{status}, 0, "")
	if err != nil {
		t.Fatalf("ListCertificates: %v", err)
	}

	return len(certs)
}

// TestDNSValidationDelay checks that with a delay a DNS-validated
// certificate stays PENDING_VALIDATION, with its validation records, until
// the delay passes, and is then reported as ISSUED by every read.
func TestDNSValidationDelay(t *testing.T) {
	t.Parallel()

	const delay = 10 * time.Minute

	store, clock := newClockedStorage(delay)
	ctx := t.Context()

	requested, err := store.RequestCertificate(ctx, &RequestCertificateInput{
		DomainName:              "delayed.example.com",
		SubjectAlternativeNames: []string{"*.delayed.example.com"},
	})
	if err != nil {
		t.Fatalf("RequestCertificate: %v", err)
	}

	arn := requested.CertificateArn

	assertPendingDNS(t, store, arn, "on request")

	clock.advance(delay - time.Second)
	assertPendingDNS(t, store, arn, "just before the delay passes")

	if n := listCount(t, store, statusPendingValidation); n != 1 {
		t.Errorf("ListCertificates PENDING_VALIDATION: got %d certificates, want 1", n)
	}

	clock.advance(time.Second)

	// ListCertificates is the first read after the delay passes.
	if n := listCount(t, store, statusIssued); n != 1 {
		t.Errorf("ListCertificates ISSUED: got %d certificates, want 1", n)
	}

	got, err := store.DescribeCertificate(ctx, arn)
	if err != nil {
		t.Fatalf("DescribeCertificate: %v", err)
	}

	assertIssuedAt(t, got, requested.CreatedAt.Add(delay))

	if requested.Status != statusPendingValidation || requested.DomainValidationOptions[0].ValidationStatus != statusPendingValidation {
		t.Error("issuing the certificate changed the copy RequestCertificate returned")
	}

	if _, err := store.GetCertificate(ctx, arn); err != nil {
		t.Errorf("GetCertificate after the delay: %v", err)
	}
}

// preIssuingSnapshot is a snapshot saved by a kumo that left every
// requested certificate PENDING_VALIDATION: one DNS-validated and one
// email-validated certificate.
const preIssuingSnapshot = `{"certificates":{
"arn:aws:acm:us-east-1:000000000000:certificate/dns":{
  "CertificateArn":"arn:aws:acm:us-east-1:000000000000:certificate/dns",
  "DomainName":"old-dns.example.com","Status":"PENDING_VALIDATION","Type":"AMAZON_ISSUED",
  "KeyAlgorithm":"RSA_2048","Serial":"00112233445566778899aabbccddeeff","CreatedAt":"2026-10-01T00:00:00Z",
  "DomainValidationOptions":[{"DomainName":"old-dns.example.com","ValidationDomain":"old-dns.example.com",
    "ValidationStatus":"PENDING_VALIDATION","ValidationMethod":"DNS",
    "ResourceRecord":{"Name":"_acme-challenge.old-dns.example.com","Type":"CNAME","Value":"_0a1b2c3d.acm-validations.aws."}}]},
"arn:aws:acm:us-east-1:000000000000:certificate/email":{
  "CertificateArn":"arn:aws:acm:us-east-1:000000000000:certificate/email",
  "DomainName":"old-email.example.com","Status":"PENDING_VALIDATION","Type":"AMAZON_ISSUED",
  "KeyAlgorithm":"RSA_2048","CreatedAt":"2026-10-01T00:00:00Z",
  "DomainValidationOptions":[{"DomainName":"old-email.example.com","ValidationDomain":"old-email.example.com",
    "ValidationStatus":"PENDING_VALIDATION","ValidationMethod":"EMAIL"}]}
}}`

// TestUnmarshalJSON_IssuesPendingDNSCertificates loads a snapshot saved
// before DNS-validated certificates were issued: its DNS-validated
// certificate is issued on load, as if it had been requested with the
// current delay, and its email-validated one stays pending.
func TestUnmarshalJSON_IssuesPendingDNSCertificates(t *testing.T) {
	t.Parallel()

	const (
		dnsARN   = "arn:aws:acm:us-east-1:000000000000:certificate/dns"
		emailARN = "arn:aws:acm:us-east-1:000000000000:certificate/email"
	)

	createdAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		delay      time.Duration
		wantIssued bool
	}{
		{"no delay", 0, true},
		{"delay passed since the request", time.Hour, true},
		{"delay not passed yet", 90 * 24 * time.Hour, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, _ := newClockedStorage(tc.delay)
			if err := json.Unmarshal([]byte(preIssuingSnapshot), store); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			// Inspect the stored certificates directly, so only the load
			// itself can have issued them.
			dns := store.Certificates[dnsARN]

			if got := dns.Status == statusIssued; got != tc.wantIssued {
				t.Fatalf("DNS certificate Status %s, want issued=%v", dns.Status, tc.wantIssued)
			}

			if tc.wantIssued {
				if want := createdAt.Add(tc.delay); dns.IssuedAt == nil || !dns.IssuedAt.Equal(want) {
					t.Errorf("IssuedAt %v, want %v", dns.IssuedAt, want)
				}

				if dns.DomainValidationOptions[0].ValidationStatus != validationStatusSuccess {
					t.Errorf("ValidationStatus %s, want SUCCESS", dns.DomainValidationOptions[0].ValidationStatus)
				}

				got, err := store.GetCertificate(t.Context(), dnsARN)
				if err != nil {
					t.Fatalf("GetCertificate: %v", err)
				}

				if leaf := parsePEMCertificate(t, got.CertificateBody); !slices.Equal(leaf.DNSNames, []string{"old-dns.example.com"}) {
					t.Errorf("DNSNames %v, want [old-dns.example.com]", leaf.DNSNames)
				}
			}

			if email := store.Certificates[emailARN]; email.Status != statusPendingValidation {
				t.Errorf("email certificate Status %s, want PENDING_VALIDATION", email.Status)
			}
		})
	}
}

func TestDNSValidationDelayFromEnv(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value  string
		want   time.Duration
		wantOK bool
	}{
		{"", 0, false},
		{"30s", 30 * time.Second, true},
		{"0", 0, true},
		{"1h30m", 90 * time.Minute, true},
		{"soon", 0, false},
		{"-5s", 0, false},
	}

	for _, tc := range cases {
		got, ok := dnsValidationDelay(tc.value)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("dnsValidationDelay(%q) = %v, %v; want %v, %v", tc.value, got, ok, tc.want, tc.wantOK)
		}
	}
}
