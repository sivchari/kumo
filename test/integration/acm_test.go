//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

func newACMClient(t *testing.T) *acm.Client {
	t.Helper()

	return acm.NewFromConfig(awsConfig(t), func(o *acm.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

// requireACMError fails unless err is the ACM exception E sent with HTTP 400:
// the ACM API reference lists the operation-specific exceptions checked here,
// including ResourceNotFoundException, with HTTP 400.
func requireACMError[E error](t *testing.T, err error) {
	t.Helper()

	var target E
	if !errors.As(err, &target) {
		t.Fatalf("expected %T, got %v", target, err)
	}

	var respErr *awshttp.ResponseError
	if !errors.As(err, &respErr) {
		t.Fatalf("no HTTP response in %v", err)
	}

	if got := respErr.HTTPStatusCode(); got != http.StatusBadRequest {
		t.Errorf("HTTP status: got %d, want 400", got)
	}
}

// acmError is the error code and HTTP status of a failed ACM call, recorded
// in goldens.
type acmError struct {
	Code       string
	HTTPStatus int
}

// acmErrorOf returns the code and HTTP status of err, which must be an ACM
// API error.
func acmErrorOf(t *testing.T, err error) *acmError {
	t.Helper()

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an API error, got %v", err)
	}

	var respErr *awshttp.ResponseError
	if !errors.As(err, &respErr) {
		t.Fatalf("no HTTP response in %v", err)
	}

	return &acmError{Code: apiErr.ErrorCode(), HTTPStatus: respErr.HTTPStatusCode()}
}

// describePublicKey names the algorithm and size of a certificate's public
// key, such as "RSA 2048" or "ECDSA P-256".
func describePublicKey(t *testing.T, key any) string {
	t.Helper()

	switch k := key.(type) {
	case *rsa.PublicKey:
		return fmt.Sprintf("RSA %d", k.N.BitLen())
	case *ecdsa.PublicKey:
		return "ECDSA " + k.Curve.Params().Name
	default:
		t.Fatalf("unexpected public key type %T", key)

		return ""
	}
}

func TestACM_RequestAndDescribeCertificate(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Request certificate.
	requestOutput, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName: aws.String("example.com"),
		SubjectAlternativeNames: []string{
			"www.example.com",
			"api.example.com",
		},
		ValidationMethod: types.ValidationMethodDns,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CertificateArn", "ResultMetadata")).Assert(t.Name()+"_request", requestOutput)

	arn := *requestOutput.CertificateArn

	// Describe certificate.
	describeOutput, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CertificateArn", "CreatedAt", "IssuedAt", "NotBefore", "NotAfter", "Serial", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestACM_ListCertificates(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Request a certificate first.
	requestOutput, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName: aws.String("list-test.example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *requestOutput.CertificateArn

	// List certificates.
	listOutput, err := client.ListCertificates(ctx, &acm.ListCertificatesInput{
		MaxItems: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Find our certificate.
	found := false

	for _, cert := range listOutput.CertificateSummaryList {
		if cert.CertificateArn != nil && *cert.CertificateArn == arn {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("certificate %s not found in list", arn)
	}
}

func TestACM_ListCertificatesWithStatusFilter(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Request an email-validated certificate, which stays PENDING_VALIDATION.
	_, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName:       aws.String("filter-test.example.com"),
		ValidationMethod: types.ValidationMethodEmail,
	})
	if err != nil {
		t.Fatal(err)
	}

	// List with PENDING_VALIDATION filter.
	listOutput, err := client.ListCertificates(ctx, &acm.ListCertificatesInput{
		CertificateStatuses: []types.CertificateStatus{
			types.CertificateStatusPendingValidation,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// All returned certificates should be PENDING_VALIDATION.
	for _, cert := range listOutput.CertificateSummaryList {
		if cert.Status != types.CertificateStatusPendingValidation {
			t.Errorf("unexpected status: got %s, want PENDING_VALIDATION", cert.Status)
		}
	}
}

func TestACM_DeleteCertificate(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Request a certificate.
	requestOutput, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName: aws.String("delete-test.example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *requestOutput.CertificateArn

	// Delete the certificate.
	_, err = client.DeleteCertificate(ctx, &acm.DeleteCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's deleted by trying to describe it.
	_, err = client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err == nil {
		t.Fatal("expected error when describing deleted certificate")
	}
}

func TestACM_ImportCertificate(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Import a certificate (using dummy data - the emulator doesn't validate).
	importOutput, err := client.ImportCertificate(ctx, &acm.ImportCertificateInput{
		Certificate: []byte("-----BEGIN CERTIFICATE-----\nMIICert...\n-----END CERTIFICATE-----"),
		PrivateKey:  []byte("-----BEGIN PRIVATE KEY-----\nMIIPrivateKey...\n-----END PRIVATE KEY-----"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CertificateArn", "ResultMetadata")).Assert(t.Name()+"_import", importOutput)

	arn := *importOutput.CertificateArn

	// Describe the imported certificate.
	describeOutput, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CertificateArn", "CreatedAt", "IssuedAt", "ImportedAt", "Serial", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestACM_RequestCertificateWithOptions(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Request certificate with key algorithm option.
	requestOutput, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName:   aws.String("options-test.example.com"),
		KeyAlgorithm: types.KeyAlgorithmEcPrime256v1,
		Options: &types.CertificateOptions{
			CertificateTransparencyLoggingPreference: types.CertificateTransparencyLoggingPreferenceEnabled,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CertificateArn", "ResultMetadata")).Assert(t.Name()+"_request", requestOutput)

	arn := *requestOutput.CertificateArn

	// Describe to verify options.
	describeOutput, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CertificateArn", "CreatedAt", "IssuedAt", "NotBefore", "NotAfter", "Serial", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

// TestACM_RequestCertificateKeyAlgorithms requests a certificate with each
// KeyAlgorithm value, plus an empty and an undefined one, and records the
// algorithm DescribeCertificate reports and the key of the certificate
// GetCertificate returns, or the error.
func TestACM_RequestCertificateKeyAlgorithms(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	type outcome struct {
		KeyAlgorithm string
		Described    string    `json:",omitempty"`
		PublicKey    string    `json:",omitempty"`
		Error        *acmError `json:",omitempty"`
	}

	algorithms := []types.KeyAlgorithm{
		"", // the default
		types.KeyAlgorithmRsa2048,
		types.KeyAlgorithmEcPrime256v1,
		types.KeyAlgorithmEcSecp384r1,
		types.KeyAlgorithmRsa1024,
		types.KeyAlgorithmRsa3072,
		types.KeyAlgorithmRsa4096,
		types.KeyAlgorithmEcSecp521r1,
		"RSA_9999", // not a KeyAlgorithm value
	}

	outcomes := make([]outcome, 0, len(algorithms))

	for _, algorithm := range algorithms {
		requested, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
			DomainName:   aws.String("key-algorithm.example.com"),
			KeyAlgorithm: algorithm,
		})
		if err != nil {
			outcomes = append(outcomes, outcome{KeyAlgorithm: string(algorithm), Error: acmErrorOf(t, err)})

			continue
		}

		t.Cleanup(func() {
			_, _ = client.DeleteCertificate(context.Background(), &acm.DeleteCertificateInput{
				CertificateArn: requested.CertificateArn,
			})
		})

		described, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{CertificateArn: requested.CertificateArn})
		if err != nil {
			t.Fatal(err)
		}

		got, err := client.GetCertificate(ctx, &acm.GetCertificateInput{CertificateArn: requested.CertificateArn})
		if err != nil {
			t.Fatal(err)
		}

		block, _ := pem.Decode([]byte(aws.ToString(got.Certificate)))
		if block == nil {
			t.Fatalf("Certificate is not PEM: %q", aws.ToString(got.Certificate))
		}

		leaf, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("parse certificate: %v", err)
		}

		outcomes = append(outcomes, outcome{
			KeyAlgorithm: string(algorithm),
			Described:    string(described.Certificate.KeyAlgorithm),
			PublicKey:    describePublicKey(t, leaf.PublicKey),
		})
	}

	golden.New(t).Assert(t.Name(), outcomes)
}

func TestACM_DescribeNonExistentCertificate(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Try to describe a non-existent certificate.
	_, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String("arn:aws:acm:us-east-1:000000000000:certificate/non-existent"),
	})
	requireACMError[*types.ResourceNotFoundException](t, err)
}

func TestACM_DeleteNonExistentCertificate(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Try to delete a non-existent certificate.
	_, err := client.DeleteCertificate(ctx, &acm.DeleteCertificateInput{
		CertificateArn: aws.String("arn:aws:acm:us-east-1:000000000000:certificate/non-existent"),
	})
	requireACMError[*types.ResourceNotFoundException](t, err)
}

func TestACM_GetCertificate(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Import a certificate first (imported certificates are in ISSUED state).
	importOutput, err := client.ImportCertificate(ctx, &acm.ImportCertificateInput{
		Certificate:      []byte("-----BEGIN CERTIFICATE-----\nMIICert...\n-----END CERTIFICATE-----"),
		PrivateKey:       []byte("-----BEGIN PRIVATE KEY-----\nMIIPrivateKey...\n-----END PRIVATE KEY-----"),
		CertificateChain: []byte("-----BEGIN CERTIFICATE-----\nMIIChain...\n-----END CERTIFICATE-----"),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *importOutput.CertificateArn

	// Get the certificate.
	getOutput, err := client.GetCertificate(ctx, &acm.GetCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestACM_GetCertificatePendingValidation(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Request an email-validated certificate (will be in PENDING_VALIDATION state).
	requestOutput, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName:       aws.String("pending.example.com"),
		ValidationMethod: types.ValidationMethodEmail,
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *requestOutput.CertificateArn

	// Try to get the certificate (should fail because it's not issued).
	_, err = client.GetCertificate(ctx, &acm.GetCertificateInput{
		CertificateArn: aws.String(arn),
	})

	requireACMError[*types.RequestInProgressException](t, err)
}

func TestACM_GetIssuedCertificate(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	requestOutput, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName:              aws.String("get-issued.example.com"),
		SubjectAlternativeNames: []string{"*.get-issued.example.com"},
		ValidationMethod:        types.ValidationMethodDns,
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := requestOutput.CertificateArn

	t.Cleanup(func() {
		_, _ = client.DeleteCertificate(context.Background(), &acm.DeleteCertificateInput{
			CertificateArn: arn,
		})
	})

	waiter := acm.NewCertificateValidatedWaiter(client)
	if err := waiter.Wait(ctx, &acm.DescribeCertificateInput{CertificateArn: arn}, 30*time.Second); err != nil {
		t.Fatalf("wait for certificate validation: %v", err)
	}

	getOutput, err := client.GetCertificate(ctx, &acm.GetCertificateInput{CertificateArn: arn})
	if err != nil {
		t.Fatal(err)
	}

	block, _ := pem.Decode([]byte(aws.ToString(getOutput.Certificate)))
	if block == nil {
		t.Fatalf("Certificate is not PEM: %q", aws.ToString(getOutput.Certificate))
	}

	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	if want := []string{"get-issued.example.com", "*.get-issued.example.com"}; !slices.Equal(leaf.DNSNames, want) {
		t.Errorf("DNSNames: got %v, want %v", leaf.DNSNames, want)
	}

	chainBlock, _ := pem.Decode([]byte(aws.ToString(getOutput.CertificateChain)))
	if chainBlock == nil {
		t.Fatalf("CertificateChain is not PEM: %q", aws.ToString(getOutput.CertificateChain))
	}

	ca, err := x509.ParseCertificate(chainBlock.Bytes)
	if err != nil {
		t.Fatalf("parse chain: %v", err)
	}

	if err := leaf.CheckSignatureFrom(ca); err != nil {
		t.Errorf("certificate is not signed by its chain: %v", err)
	}

	describeOutput, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{CertificateArn: arn})
	if err != nil {
		t.Fatal(err)
	}

	if notAfter := aws.ToTime(describeOutput.Certificate.NotAfter); !notAfter.Equal(leaf.NotAfter) {
		t.Errorf("NotAfter: describe %v, certificate %v", notAfter, leaf.NotAfter)
	}
}

func TestACM_DNSValidatedCertificateIsIssued(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	requestOutput, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName:       aws.String("issued.example.com"),
		ValidationMethod: types.ValidationMethodDns,
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := requestOutput.CertificateArn

	t.Cleanup(func() {
		_, _ = client.DeleteCertificate(context.Background(), &acm.DeleteCertificateInput{
			CertificateArn: arn,
		})
	})

	// The SDK waiter polls DescribeCertificate until the certificate is ISSUED.
	waiter := acm.NewCertificateValidatedWaiter(client)
	if err := waiter.Wait(ctx, &acm.DescribeCertificateInput{CertificateArn: arn}, 30*time.Second); err != nil {
		t.Fatalf("wait for certificate validation: %v", err)
	}
}

func TestACM_CertificateTags(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	requestOutput, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName: aws.String("tags.example.com"),
		Tags: []types.Tag{
			{Key: aws.String("env"), Value: aws.String("dev")},
			{Key: aws.String("team"), Value: aws.String("web")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := requestOutput.CertificateArn

	t.Cleanup(func() {
		_, _ = client.DeleteCertificate(context.Background(), &acm.DeleteCertificateInput{
			CertificateArn: arn,
		})
	})

	listOutput, err := client.ListTagsForCertificate(ctx, &acm.ListTagsForCertificateInput{CertificateArn: arn})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_requested", listOutput)

	_, err = client.AddTagsToCertificate(ctx, &acm.AddTagsToCertificateInput{
		CertificateArn: arn,
		Tags: []types.Tag{
			{Key: aws.String("env"), Value: aws.String("prod")},
			{Key: aws.String("owner"), Value: aws.String("alice")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.RemoveTagsFromCertificate(ctx, &acm.RemoveTagsFromCertificateInput{
		CertificateArn: arn,
		Tags:           []types.Tag{{Key: aws.String("team")}},
	})
	if err != nil {
		t.Fatal(err)
	}

	listOutput, err = client.ListTagsForCertificate(ctx, &acm.ListTagsForCertificateInput{CertificateArn: arn})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_updated", listOutput)
}

// Placeholder PEM material for ImportCertificate; kumo does not parse it.
const (
	dummyCertificatePEM = "-----BEGIN CERTIFICATE-----\nMIICert...\n-----END CERTIFICATE-----"
	dummyPrivateKeyPEM  = "-----BEGIN PRIVATE KEY-----\nMIIPrivateKey...\n-----END PRIVATE KEY-----"
)

// acmTag returns an ACM tag with a key and a value.
func acmTag(key, value string) types.Tag {
	return types.Tag{Key: aws.String(key), Value: aws.String(value)}
}

// deleteACMCertificateOnCleanup deletes the certificate when the test ends.
func deleteACMCertificateOnCleanup(t *testing.T, client *acm.Client, arn *string) {
	t.Helper()

	t.Cleanup(func() {
		_, _ = client.DeleteCertificate(context.Background(), &acm.DeleteCertificateInput{CertificateArn: arn})
	})
}

// requestACMCertificate requests a certificate for domain with tags and
// deletes it when the test ends.
func requestACMCertificate(t *testing.T, client *acm.Client, domain string, tags ...types.Tag) *string {
	t.Helper()

	requested, err := client.RequestCertificate(t.Context(), &acm.RequestCertificateInput{
		DomainName: aws.String(domain),
		Tags:       tags,
	})
	if err != nil {
		t.Fatal(err)
	}

	deleteACMCertificateOnCleanup(t, client, requested.CertificateArn)

	return requested.CertificateArn
}

// listACMTags returns the tags of a certificate.
func listACMTags(t *testing.T, client *acm.Client, arn *string) *acm.ListTagsForCertificateOutput {
	t.Helper()

	listed, err := client.ListTagsForCertificate(t.Context(), &acm.ListTagsForCertificateInput{CertificateArn: arn})
	if err != nil {
		t.Fatal(err)
	}

	return listed
}

// TestACM_DuplicateTagKeys tags certificates with a key given twice in one
// call. Every tagging operation stores the key once, with the last value,
// the way AddTagsToCertificate overwrites a key that is already present.
func TestACM_DuplicateTagKeys(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()
	ignore := golden.WithIgnoreFields("ResultMetadata")

	duplicated := []types.Tag{acmTag("env", "dev"), acmTag("team", "web"), acmTag("env", "prod")}

	requested := requestACMCertificate(t, client, "duplicate-tags.example.com", duplicated...)

	golden.New(t, ignore).Assert(t.Name()+"_requested", listACMTags(t, client, requested))

	imported, err := client.ImportCertificate(ctx, &acm.ImportCertificateInput{
		Certificate: []byte(dummyCertificatePEM),
		PrivateKey:  []byte(dummyPrivateKeyPEM),
		Tags:        duplicated,
	})
	if err != nil {
		t.Fatal(err)
	}

	deleteACMCertificateOnCleanup(t, client, imported.CertificateArn)

	golden.New(t, ignore).Assert(t.Name()+"_imported", listACMTags(t, client, imported.CertificateArn))

	if _, err := client.AddTagsToCertificate(ctx, &acm.AddTagsToCertificateInput{
		CertificateArn: requested,
		Tags:           []types.Tag{acmTag("owner", "alice"), acmTag("owner", "bob")},
	}); err != nil {
		t.Fatal(err)
	}

	golden.New(t, ignore).Assert(t.Name()+"_added", listACMTags(t, client, requested))
}

// TestACM_TagQuotaCountsDistinctKeys requests a certificate with 50 tags
// that hold 49 distinct keys, then adds keys up to the quota of 50 tags per
// certificate, which counts distinct keys. It also adds 50 keys to another
// certificate in one call.
func TestACM_TagQuotaCountsDistinctKeys(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	keys := func(n int) []types.Tag {
		tags := make([]types.Tag, 0, n)
		for i := range n {
			tags = append(tags, acmTag(fmt.Sprintf("key-%02d", i), "v"))
		}

		return tags
	}

	requested := requestACMCertificate(t, client, "tag-quota.example.com", append(keys(49), acmTag("key-00", "again"))...)

	type step struct {
		Step  string
		Error *acmError `json:",omitempty"`
		Tags  int
	}

	var steps []step

	record := func(name string, arn *string, err error) {
		s := step{Step: name, Tags: len(listACMTags(t, client, arn).Tags)}
		if err != nil {
			s.Error = acmErrorOf(t, err)
		}

		steps = append(steps, s)
	}

	addTags := func(arn *string, tags ...types.Tag) error {
		_, err := client.AddTagsToCertificate(ctx, &acm.AddTagsToCertificateInput{CertificateArn: arn, Tags: tags})

		return err
	}

	record("request 50 tags with 49 keys", requested, nil)
	record("add a 50th key", requested, addTags(requested, acmTag("key-49", "v")))
	record("add a 51st key", requested, addTags(requested, acmTag("key-50", "v")))
	record("overwrite a key at the quota", requested, addTags(requested, acmTag("key-00", "overwritten")))

	untagged := requestACMCertificate(t, client, "tag-quota-add.example.com")
	record("add 50 keys in one call", untagged, addTags(untagged, keys(50)...))

	golden.New(t).Assert(t.Name(), steps)
}

// TestACM_InvalidTags sends every tagging operation tags that break the Tag
// type's length limits or use the reserved aws: prefix, and records the
// error of each call; a key and a value at the length limits are accepted.
func TestACM_InvalidTags(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	arn := requestACMCertificate(t, client, "invalid-tags.example.com")

	request := func(tags ...types.Tag) error {
		requested, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
			DomainName: aws.String("invalid-tags.example.com"),
			Tags:       tags,
		})
		if err == nil {
			deleteACMCertificateOnCleanup(t, client, requested.CertificateArn)
		}

		return err
	}

	importWith := func(tags ...types.Tag) error {
		imported, err := client.ImportCertificate(ctx, &acm.ImportCertificateInput{
			Certificate: []byte(dummyCertificatePEM),
			PrivateKey:  []byte(dummyPrivateKeyPEM),
			Tags:        tags,
		})
		if err == nil {
			deleteACMCertificateOnCleanup(t, client, imported.CertificateArn)
		}

		return err
	}

	add := func(tags ...types.Tag) error {
		_, err := client.AddTagsToCertificate(ctx, &acm.AddTagsToCertificateInput{CertificateArn: arn, Tags: tags})

		return err
	}

	remove := func(tags ...types.Tag) error {
		_, err := client.RemoveTagsFromCertificate(ctx, &acm.RemoveTagsFromCertificateInput{CertificateArn: arn, Tags: tags})

		return err
	}

	emptyKey := acmTag("", "v")
	reservedKey := acmTag("aws:reserved", "v")

	tooMany := make([]types.Tag, 0, 51)
	for i := range 51 {
		tooMany = append(tooMany, acmTag(fmt.Sprintf("key-%02d", i), "v"))
	}

	type outcome struct {
		Case  string
		Error *acmError `json:",omitempty"`
	}

	var outcomes []outcome

	record := func(name string, err error) {
		o := outcome{Case: name}
		if err != nil {
			o.Error = acmErrorOf(t, err)
		}

		outcomes = append(outcomes, o)
	}

	record("RequestCertificate empty key", request(emptyKey))
	record("ImportCertificate empty key", importWith(emptyKey))
	record("AddTagsToCertificate empty key", add(emptyKey))
	record("RemoveTagsFromCertificate empty key", remove(types.Tag{Key: aws.String("")}))
	record("AddTagsToCertificate 129 character key", add(acmTag(strings.Repeat("k", 129), "v")))
	record("AddTagsToCertificate 257 character value", add(acmTag("long-value", strings.Repeat("v", 257))))
	record("AddTagsToCertificate 128 character key and 256 character value", add(acmTag(strings.Repeat("k", 128), strings.Repeat("v", 256))))
	record("RequestCertificate reserved key", request(reservedKey))
	record("ImportCertificate reserved key", importWith(reservedKey))
	record("AddTagsToCertificate reserved key", add(reservedKey))
	record("RemoveTagsFromCertificate reserved key", remove(types.Tag{Key: aws.String("aws:reserved")}))
	record("RequestCertificate 51 tags", request(tooMany...))

	golden.New(t).Assert(t.Name(), outcomes)
}

// TestACM_RemoveTagsMatchesValue removes tags given with and without a
// value: one given with a value is removed only when the value matches, and
// one given without a value is removed whatever its value.
func TestACM_RemoveTagsMatchesValue(t *testing.T) {
	client := newACMClient(t)

	arn := requestACMCertificate(t, client, "remove-tags.example.com",
		acmTag("env", "prod"), acmTag("team", "web"), acmTag("owner", "alice"))

	if _, err := client.RemoveTagsFromCertificate(t.Context(), &acm.RemoveTagsFromCertificateInput{
		CertificateArn: arn,
		Tags:           []types.Tag{acmTag("env", "dev"), acmTag("team", "web"), {Key: aws.String("owner")}},
	}); err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), listACMTags(t, client, arn))
}

// TestACM_RemoveTagsRejectsInvalidValue removes two tags in one call, one of
// them given with a value longer than the Tag type allows. The call fails,
// and neither tag is removed.
func TestACM_RemoveTagsRejectsInvalidValue(t *testing.T) {
	client := newACMClient(t)

	arn := requestACMCertificate(t, client, "remove-invalid-value.example.com",
		acmTag("env", "prod"), acmTag("team", "web"))

	_, err := client.RemoveTagsFromCertificate(t.Context(), &acm.RemoveTagsFromCertificateInput{
		CertificateArn: arn,
		Tags:           []types.Tag{{Key: aws.String("env")}, acmTag("team", strings.Repeat("v", 257))},
	})

	type outcome struct {
		Error *acmError
		Tags  []types.Tag
	}

	got := outcome{Tags: listACMTags(t, client, arn).Tags}
	if err != nil {
		got.Error = acmErrorOf(t, err)
	}

	golden.New(t).Assert(t.Name(), got)
}

// bodyRecorder is an HTTP client for the SDK that keeps the status and body
// of the last response, so a test can check what kumo sent.
type bodyRecorder struct {
	status int
	body   string
}

func (r *bodyRecorder) Do(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if err != nil {
		return nil, err
	}

	r.status, r.body = resp.StatusCode, string(body)
	resp.Body = io.NopCloser(bytes.NewReader(body))

	return resp, nil
}

// TestACM_TagOperationResponseBodies records the responses of the tag
// operations: ListTagsForCertificate leaves Tags out for a certificate
// without tags, and AddTagsToCertificate and RemoveTagsFromCertificate
// answer with an empty body, as the API reference documents for them.
func TestACM_TagOperationResponseBodies(t *testing.T) {
	recorder := &bodyRecorder{}
	client := acm.NewFromConfig(awsConfig(t), func(o *acm.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
		o.HTTPClient = recorder
	})
	ctx := t.Context()

	arn := requestACMCertificate(t, client, "tag-responses.example.com")

	type response struct {
		Operation  string
		HTTPStatus int
		Body       string
	}

	var responses []response

	record := func(operation string) {
		responses = append(responses, response{Operation: operation, HTTPStatus: recorder.status, Body: recorder.body})
	}

	listACMTags(t, client, arn)
	record("ListTagsForCertificate without tags")

	if _, err := client.AddTagsToCertificate(ctx, &acm.AddTagsToCertificateInput{
		CertificateArn: arn,
		Tags:           []types.Tag{acmTag("env", "dev"), {Key: aws.String("team")}},
	}); err != nil {
		t.Fatal(err)
	}

	record("AddTagsToCertificate")

	if _, err := client.RemoveTagsFromCertificate(ctx, &acm.RemoveTagsFromCertificateInput{
		CertificateArn: arn,
		Tags:           []types.Tag{{Key: aws.String("team")}},
	}); err != nil {
		t.Fatal(err)
	}

	record("RemoveTagsFromCertificate")

	golden.New(t).Assert(t.Name()+"_responses", responses)
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_tags", listACMTags(t, client, arn))
}

// TestACM_ErrorResponses records the error of calls on a certificate that
// does not exist or is not issued yet, and of calls whose CertificateArn or
// Tags is empty.
func TestACM_ErrorResponses(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	pending, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName:       aws.String("pending-errors.example.com"),
		ValidationMethod: types.ValidationMethodEmail,
	})
	if err != nil {
		t.Fatal(err)
	}

	deleteACMCertificateOnCleanup(t, client, pending.CertificateArn)

	absent := aws.String("arn:aws:acm:us-east-1:000000000000:certificate/non-existent")
	tags := []types.Tag{{Key: aws.String("k")}}

	calls := []struct {
		name string
		call func() error
	}{
		{"DescribeCertificate unknown ARN", func() error {
			_, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{CertificateArn: absent})

			return err
		}},
		{"GetCertificate unknown ARN", func() error {
			_, err := client.GetCertificate(ctx, &acm.GetCertificateInput{CertificateArn: absent})

			return err
		}},
		{"DeleteCertificate unknown ARN", func() error {
			_, err := client.DeleteCertificate(ctx, &acm.DeleteCertificateInput{CertificateArn: absent})

			return err
		}},
		{"GetCertificate pending certificate", func() error {
			_, err := client.GetCertificate(ctx, &acm.GetCertificateInput{CertificateArn: pending.CertificateArn})

			return err
		}},
		{"ListTagsForCertificate unknown ARN", func() error {
			_, err := client.ListTagsForCertificate(ctx, &acm.ListTagsForCertificateInput{CertificateArn: absent})

			return err
		}},
		{"AddTagsToCertificate unknown ARN", func() error {
			_, err := client.AddTagsToCertificate(ctx, &acm.AddTagsToCertificateInput{CertificateArn: absent, Tags: tags})

			return err
		}},
		{"RemoveTagsFromCertificate unknown ARN", func() error {
			_, err := client.RemoveTagsFromCertificate(ctx, &acm.RemoveTagsFromCertificateInput{CertificateArn: absent, Tags: tags})

			return err
		}},
		{"ListTagsForCertificate empty ARN", func() error {
			_, err := client.ListTagsForCertificate(ctx, &acm.ListTagsForCertificateInput{CertificateArn: aws.String("")})

			return err
		}},
		{"AddTagsToCertificate empty tags", func() error {
			_, err := client.AddTagsToCertificate(ctx, &acm.AddTagsToCertificateInput{CertificateArn: absent, Tags: []types.Tag{}})

			return err
		}},
		{"RemoveTagsFromCertificate empty tags", func() error {
			_, err := client.RemoveTagsFromCertificate(ctx, &acm.RemoveTagsFromCertificateInput{CertificateArn: absent, Tags: []types.Tag{}})

			return err
		}},
	}

	type outcome struct {
		Case  string
		Error *acmError
	}

	outcomes := make([]outcome, 0, len(calls))

	for _, c := range calls {
		err := c.call()
		if err == nil {
			t.Fatalf("%s: expected an error", c.name)
		}

		outcomes = append(outcomes, outcome{Case: c.name, Error: acmErrorOf(t, err)})
	}

	golden.New(t).Assert(t.Name(), outcomes)
}

func TestACM_ListTagsForNonExistentCertificate(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	_, err := client.ListTagsForCertificate(ctx, &acm.ListTagsForCertificateInput{
		CertificateArn: aws.String("arn:aws:acm:us-east-1:000000000000:certificate/non-existent"),
	})

	requireACMError[*types.ResourceNotFoundException](t, err)
}
