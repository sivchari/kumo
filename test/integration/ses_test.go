//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/sivchari/golden"
)

func newSESClient(t *testing.T) *ses.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return ses.NewFromConfig(cfg, func(o *ses.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestSES_VerifyEmailIdentity(t *testing.T) {
	client := newSESClient(t)
	ctx := t.Context()

	emailAddress := "verify-test@example.com"

	output, err := client.VerifyEmailIdentity(ctx, &ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(emailAddress),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), output)

	t.Cleanup(func() {
		_, _ = client.DeleteIdentity(context.Background(), &ses.DeleteIdentityInput{
			Identity: aws.String(emailAddress),
		})
	})
}

func TestSES_ListIdentities(t *testing.T) {
	client := newSESClient(t)
	ctx := t.Context()

	emailAddress := "list-test@example.com"

	_, err := client.VerifyEmailIdentity(ctx, &ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(emailAddress),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteIdentity(context.Background(), &ses.DeleteIdentityInput{
			Identity: aws.String(emailAddress),
		})
	})

	output, err := client.ListIdentities(ctx, &ses.ListIdentitiesInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, identity := range output.Identities {
		if identity == emailAddress {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("verified identity %s not found in list", emailAddress)
	}
}

func TestSES_DeleteIdentity(t *testing.T) {
	client := newSESClient(t)
	ctx := t.Context()

	emailAddress := "delete-test@example.com"

	_, err := client.VerifyEmailIdentity(ctx, &ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(emailAddress),
	})
	if err != nil {
		t.Fatal(err)
	}

	output, err := client.DeleteIdentity(ctx, &ses.DeleteIdentityInput{
		Identity: aws.String(emailAddress),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), output)

	// Verify identity is no longer listed.
	listOutput, err := client.ListIdentities(ctx, &ses.ListIdentitiesInput{})
	if err != nil {
		t.Fatal(err)
	}

	for _, identity := range listOutput.Identities {
		if identity == emailAddress {
			t.Errorf("deleted identity %s still found in list", emailAddress)
		}
	}
}

func TestSES_GetIdentityVerificationAttributes(t *testing.T) {
	client := newSESClient(t)
	ctx := t.Context()

	emailAddress := "verify-attrs-test@example.com"

	_, err := client.VerifyEmailIdentity(ctx, &ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(emailAddress),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteIdentity(context.Background(), &ses.DeleteIdentityInput{
			Identity: aws.String(emailAddress),
		})
	})

	output, err := client.GetIdentityVerificationAttributes(ctx, &ses.GetIdentityVerificationAttributesInput{
		Identities: []string{emailAddress},
	})
	if err != nil {
		t.Fatal(err)
	}

	attrs, exists := output.VerificationAttributes[emailAddress]
	if !exists {
		t.Fatalf("verification attributes for %s not found", emailAddress)
	}

	if attrs.VerificationStatus != types.VerificationStatusSuccess {
		t.Errorf("expected verification status Success, got %s", attrs.VerificationStatus)
	}
}

func TestSES_SendEmail(t *testing.T) {
	client := newSESClient(t)
	ctx := t.Context()

	source := "send-test@example.com"

	_, err := client.VerifyEmailIdentity(ctx, &ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(source),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteIdentity(context.Background(), &ses.DeleteIdentityInput{
			Identity: aws.String(source),
		})
	})

	output, err := client.SendEmail(ctx, &ses.SendEmailInput{
		Source: aws.String(source),
		Destination: &types.Destination{
			ToAddresses: []string{"recipient@example.com"},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data: aws.String("Test Subject"),
			},
			Body: &types.Body{
				Text: &types.Content{
					Data: aws.String("Test body content"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("MessageId", "ResultMetadata")).Assert(t.Name(), output)
}

func TestSES_SendRawEmail(t *testing.T) {
	client := newSESClient(t)
	ctx := t.Context()

	source := "raw-send-test@example.com"

	_, err := client.VerifyEmailIdentity(ctx, &ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(source),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteIdentity(context.Background(), &ses.DeleteIdentityInput{
			Identity: aws.String(source),
		})
	})

	rawMessage := "From: " + source + "\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Raw Test Subject\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Raw test body content"

	output, err := client.SendRawEmail(ctx, &ses.SendRawEmailInput{
		Source:       aws.String(source),
		Destinations: []string{"recipient@example.com"},
		RawMessage: &types.RawMessage{
			Data: []byte(rawMessage),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("MessageId", "ResultMetadata")).Assert(t.Name(), output)
}

func TestSES_Mailbox(t *testing.T) {
	client := newSESClient(t)
	ctx := t.Context()

	source := "mailbox-test@example.com"

	_, err := client.VerifyEmailIdentity(ctx, &ses.VerifyEmailIdentityInput{
		EmailAddress: aws.String(source),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteIdentity(context.Background(), &ses.DeleteIdentityInput{
			Identity: aws.String(source),
		})
	})

	// Send an email.
	_, err = client.SendEmail(ctx, &ses.SendEmailInput{
		Source: aws.String(source),
		Destination: &types.Destination{
			ToAddresses: []string{"recipient@example.com"},
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data: aws.String("Mailbox Test Subject"),
			},
			Body: &types.Body{
				Text: &types.Content{
					Data: aws.String("Mailbox test body"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check mailbox via kumo-specific endpoint.
	resp, err := http.Get("http://localhost:4566/_aws/ses?email=" + source)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var emails []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		t.Fatal(err)
	}

	if len(emails) == 0 {
		t.Fatal("no emails found in mailbox")
	}

	found := false

	for _, email := range emails {
		if email["subject"] == "Mailbox Test Subject" {
			found = true

			break
		}
	}

	if !found {
		t.Error("expected email not found in mailbox")
	}
}
