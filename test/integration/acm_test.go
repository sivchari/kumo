//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/sivchari/golden"
)

func newACMClient(t *testing.T) *acm.Client {
	t.Helper()

	return acm.NewFromConfig(awsConfig(t), func(o *acm.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
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

	golden.New(t, golden.WithIgnoreFields("CertificateArn", "CreatedAt", "IssuedAt", "Serial", "Value", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
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

	// Request a certificate.
	_, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName: aws.String("filter-test.example.com"),
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

	golden.New(t, golden.WithIgnoreFields("CertificateArn", "CreatedAt", "IssuedAt", "Serial", "Value", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestACM_DescribeNonExistentCertificate(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Try to describe a non-existent certificate.
	_, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String("arn:aws:acm:us-east-1:000000000000:certificate/non-existent"),
	})
	if err == nil {
		t.Fatal("expected error when describing non-existent certificate")
	}
}

func TestACM_DeleteNonExistentCertificate(t *testing.T) {
	client := newACMClient(t)
	ctx := t.Context()

	// Try to delete a non-existent certificate.
	_, err := client.DeleteCertificate(ctx, &acm.DeleteCertificateInput{
		CertificateArn: aws.String("arn:aws:acm:us-east-1:000000000000:certificate/non-existent"),
	})
	if err == nil {
		t.Fatal("expected error when deleting non-existent certificate")
	}
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

	// Request a certificate (will be in PENDING_VALIDATION state).
	requestOutput, err := client.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName: aws.String("pending.example.com"),
	})
	if err != nil {
		t.Fatal(err)
	}

	arn := *requestOutput.CertificateArn

	// Try to get the certificate (should fail because it's not issued).
	_, err = client.GetCertificate(ctx, &acm.GetCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err == nil {
		t.Fatal("expected error when getting pending certificate")
	}
}
