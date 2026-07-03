//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/sivchari/golden"
)

func newSESv2Client(t *testing.T) *sesv2.Client {
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

	return sesv2.NewFromConfig(cfg, func(o *sesv2.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566/ses")
	})
}

func TestSESv2_CreateAndGetEmailIdentity(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	emailIdentity := "test@example.com"

	// Create email identity.
	createOutput, err := client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(emailIdentity),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Tokens", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Get email identity.
	getOutput, err := client.GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{
		EmailIdentity: aws.String(emailIdentity),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Tokens", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestSESv2_CreateDomainIdentity(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	domainIdentity := "example.com"

	// Create domain identity.
	createOutput, err := client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(domainIdentity),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Tokens", "ResultMetadata")).Assert(t.Name(), createOutput)
}

func TestSESv2_ListEmailIdentities(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	// Create an email identity.
	emailIdentity := "list-test@example.com"
	_, err := client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(emailIdentity),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List email identities.
	listOutput, err := client.ListEmailIdentities(ctx, &sesv2.ListEmailIdentitiesInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, identity := range listOutput.EmailIdentities {
		if identity.IdentityName != nil && *identity.IdentityName == emailIdentity {
			found = true

			break
		}
	}

	if !found {
		t.Error("created email identity not found in list")
	}
}

func TestSESv2_DeleteEmailIdentity(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	emailIdentity := "delete-test@example.com"

	// Create email identity.
	_, err := client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(emailIdentity),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete email identity.
	_, err = client.DeleteEmailIdentity(ctx, &sesv2.DeleteEmailIdentityInput{
		EmailIdentity: aws.String(emailIdentity),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{
		EmailIdentity: aws.String(emailIdentity),
	})
	if err == nil {
		t.Error("expected error for deleted email identity")
	}
}

func TestSESv2_CreateAndGetConfigurationSet(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	configSetName := "test-config-set"

	// Create configuration set.
	_, err := client.CreateConfigurationSet(ctx, &sesv2.CreateConfigurationSetInput{
		ConfigurationSetName: aws.String(configSetName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get configuration set.
	getOutput, err := client.GetConfigurationSet(ctx, &sesv2.GetConfigurationSetInput{
		ConfigurationSetName: aws.String(configSetName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestSESv2_ListConfigurationSets(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	configSetName := "test-list-config-set"

	// Create configuration set.
	_, err := client.CreateConfigurationSet(ctx, &sesv2.CreateConfigurationSetInput{
		ConfigurationSetName: aws.String(configSetName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List configuration sets.
	listOutput, err := client.ListConfigurationSets(ctx, &sesv2.ListConfigurationSetsInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, name := range listOutput.ConfigurationSets {
		if name == configSetName {
			found = true

			break
		}
	}

	if !found {
		t.Error("created configuration set not found in list")
	}
}

func TestSESv2_DeleteConfigurationSet(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	configSetName := "test-delete-config-set"

	// Create configuration set.
	_, err := client.CreateConfigurationSet(ctx, &sesv2.CreateConfigurationSetInput{
		ConfigurationSetName: aws.String(configSetName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete configuration set.
	_, err = client.DeleteConfigurationSet(ctx, &sesv2.DeleteConfigurationSetInput{
		ConfigurationSetName: aws.String(configSetName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.GetConfigurationSet(ctx, &sesv2.GetConfigurationSetInput{
		ConfigurationSetName: aws.String(configSetName),
	})
	if err == nil {
		t.Error("expected error for deleted configuration set")
	}
}

func TestSESv2_SendEmail(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	// Create email identity first.
	emailIdentity := "sender@example.com"
	_, err := client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(emailIdentity),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Send email.
	sendOutput, err := client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(emailIdentity),
		Destination: &types.Destination{
			ToAddresses: []string{"recipient@example.com"},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data: aws.String("Test Subject"),
				},
				Body: &types.Body{
					Text: &types.Content{
						Data: aws.String("Test body content"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("MessageId", "ResultMetadata")).Assert(t.Name(), sendOutput)
}

func TestSESv2_SendRawEmail(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	// Create email identity first.
	emailIdentity := "raw-sender@example.com"
	_, _ = client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(emailIdentity),
	})

	// Build raw MIME message.
	rawMessage := "From: raw-sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Raw Test Subject\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		"Raw test body content"

	// Send raw email.
	_, err := client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(emailIdentity),
		Destination: &types.Destination{
			ToAddresses: []string{"recipient@example.com"},
		},
		Content: &types.EmailContent{
			Raw: &types.RawMessage{
				Data: []byte(rawMessage),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify sent email via kumo-specific endpoint.
	resp, err := http.Get("http://localhost:4566/kumo/ses/v2/sent-emails")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result struct {
		SentEmails []json.RawMessage `json:"SentEmails"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	// Find the raw email sent in this test (other tests may have sent emails too).
	raw := findSentEmail(t, result.SentEmails, emailIdentity, "Raw Test Subject")
	golden.New(t, golden.WithIgnoreFields("MessageId", "SentAt")).Assert(t.Name(), raw)
}

func TestSESv2_SendRawEmailWithoutDestination(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	// Create email identity first.
	emailIdentity := "raw-no-dest@example.com"
	_, _ = client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(emailIdentity),
	})

	// Build raw MIME message with recipients in headers only.
	rawMessage := "From: raw-no-dest@example.com\r\n" +
		"To: to-recipient@example.com\r\n" +
		"Cc: cc-recipient@example.com\r\n" +
		"Subject: Raw No Destination Test\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		"Raw email without explicit Destination"

	// Send raw email without Destination.
	_, err := client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(emailIdentity),
		Content: &types.EmailContent{
			Raw: &types.RawMessage{
				Data: []byte(rawMessage),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify sent email via kumo-specific endpoint.
	resp, err := http.Get("http://localhost:4566/kumo/ses/v2/sent-emails")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result struct {
		SentEmails []json.RawMessage `json:"SentEmails"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	raw := findSentEmail(t, result.SentEmails, emailIdentity, "Raw No Destination Test")
	golden.New(t, golden.WithIgnoreFields("MessageId", "SentAt")).Assert(t.Name(), raw)
}

func TestSESv2_EmailIdentityNotFound(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	// Try to get non-existent email identity.
	_, err := client.GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{
		EmailIdentity: aws.String("nonexistent@example.com"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent email identity")
	}
}

func TestSESv2_GetSentEmails(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	// Create email identity.
	fromEmail := "test-sent@example.com"
	_, err := client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(fromEmail),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Send email.
	_, err = client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(fromEmail),
		Destination: &types.Destination{
			ToAddresses: []string{"recipient@example.com"},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data: aws.String("Test Subject"),
				},
				Body: &types.Body{
					Text: &types.Content{
						Data: aws.String("Test body"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get sent emails via kumo-specific endpoint.
	resp, err := http.Get("http://localhost:4566/kumo/ses/v2/sent-emails")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, resp.StatusCode, body)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	sentEmails, ok := result["SentEmails"]
	if !ok {
		t.Fatal("SentEmails field not found in response")
	}

	emails, ok := sentEmails.([]interface{})
	if !ok {
		t.Fatal("SentEmails is not an array")
	}

	if len(emails) == 0 {
		t.Fatal("no sent emails found")
	}

	// Find our email (other tests may have sent emails too).
	var found bool

	for _, e := range emails {
		email, ok := e.(map[string]interface{})
		if !ok {
			continue
		}

		if email["FromEmailAddress"] == fromEmail && email["Subject"] == "Test Subject" {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("sent email from %s with subject 'Test Subject' not found", fromEmail)
	}
}

func TestSESv2_EmailTemplate_CRUD(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	const name = "tmpl-crud"

	t.Cleanup(func() {
		_, _ = client.DeleteEmailTemplate(context.Background(), &sesv2.DeleteEmailTemplateInput{
			TemplateName: aws.String(name),
		})
	})

	if _, err := client.CreateEmailTemplate(ctx, &sesv2.CreateEmailTemplateInput{
		TemplateName: aws.String(name),
		TemplateContent: &types.EmailTemplateContent{
			Subject: aws.String("Hi"),
			Text:    aws.String("Body"),
			Html:    aws.String("<p>Body</p>"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	getOutput, err := client.GetEmailTemplate(ctx, &sesv2.GetEmailTemplateInput{
		TemplateName: aws.String(name),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get", getOutput)

	if _, err := client.UpdateEmailTemplate(ctx, &sesv2.UpdateEmailTemplateInput{
		TemplateName: aws.String(name),
		TemplateContent: &types.EmailTemplateContent{
			Subject: aws.String("Hi v2"),
			Text:    aws.String("Body v2"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	getOutput, err = client.GetEmailTemplate(ctx, &sesv2.GetEmailTemplateInput{
		TemplateName: aws.String(name),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get_after_update", getOutput)

	if _, err := client.DeleteEmailTemplate(ctx, &sesv2.DeleteEmailTemplateInput{
		TemplateName: aws.String(name),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.GetEmailTemplate(ctx, &sesv2.GetEmailTemplateInput{
		TemplateName: aws.String(name),
	}); err == nil {
		t.Fatal("expected error for deleted template")
	}
}

func TestSESv2_ListEmailTemplates(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	const name = "tmpl-list"

	t.Cleanup(func() {
		_, _ = client.DeleteEmailTemplate(context.Background(), &sesv2.DeleteEmailTemplateInput{
			TemplateName: aws.String(name),
		})
	})

	if _, err := client.CreateEmailTemplate(ctx, &sesv2.CreateEmailTemplateInput{
		TemplateName: aws.String(name),
		TemplateContent: &types.EmailTemplateContent{
			Subject: aws.String("S"),
			Text:    aws.String("B"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	out, err := client.ListEmailTemplates(ctx, &sesv2.ListEmailTemplatesInput{})
	if err != nil {
		t.Fatal(err)
	}

	// Extract the metadata entry for our template — other tests in the same
	// run may have left additional templates, so we cannot golden-assert the
	// whole response. CreatedTimestamp is dynamic and ignored.
	var match types.EmailTemplateMetadata

	found := false

	for _, m := range out.TemplatesMetadata {
		if m.TemplateName != nil && *m.TemplateName == name {
			match = m
			found = true

			break
		}
	}

	if !found {
		t.Fatalf("expected template %q in list", name)
	}

	golden.New(t, golden.WithIgnoreFields("CreatedTimestamp")).Assert(t.Name(), match)
}

func TestSESv2_SendBulkEmail(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	const (
		fromEmail = "bulk-sender@example.com"
		tmpl      = "tmpl-bulk"
	)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = client.DeleteEmailIdentity(cleanupCtx, &sesv2.DeleteEmailIdentityInput{
			EmailIdentity: aws.String(fromEmail),
		})
		_, _ = client.DeleteEmailTemplate(cleanupCtx, &sesv2.DeleteEmailTemplateInput{
			TemplateName: aws.String(tmpl),
		})
	})

	if _, err := client.CreateEmailIdentity(ctx, &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(fromEmail),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.CreateEmailTemplate(ctx, &sesv2.CreateEmailTemplateInput{
		TemplateName: aws.String(tmpl),
		TemplateContent: &types.EmailTemplateContent{
			Subject: aws.String("Bulk subject"),
			Text:    aws.String("Hello {{name}}"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := client.SendBulkEmail(ctx, &sesv2.SendBulkEmailInput{
		FromEmailAddress: aws.String(fromEmail),
		DefaultContent: &types.BulkEmailContent{
			Template: &types.Template{
				TemplateName: aws.String(tmpl),
				TemplateData: aws.String("{}"),
			},
		},
		BulkEmailEntries: []types.BulkEmailEntry{
			{
				Destination: &types.Destination{ToAddresses: []string{"a@example.com"}},
				ReplacementEmailContent: &types.ReplacementEmailContent{
					ReplacementTemplate: &types.ReplacementTemplate{
						ReplacementTemplateData: aws.String(`{"name":"A"}`),
					},
				},
			},
			{
				Destination: &types.Destination{ToAddresses: []string{"b@example.com"}},
				ReplacementEmailContent: &types.ReplacementEmailContent{
					ReplacementTemplate: &types.ReplacementTemplate{
						ReplacementTemplateData: aws.String(`{"name":"B"}`),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("MessageId", "ResultMetadata")).Assert(t.Name(), resp)
}

func TestSESv2_SendBulkEmail_UnknownTemplate(t *testing.T) {
	client := newSESv2Client(t)
	ctx := t.Context()

	_, err := client.SendBulkEmail(ctx, &sesv2.SendBulkEmailInput{
		FromEmailAddress: aws.String("missing-tmpl@example.com"),
		DefaultContent: &types.BulkEmailContent{
			Template: &types.Template{
				TemplateName: aws.String("does-not-exist"),
				TemplateData: aws.String("{}"),
			},
		},
		BulkEmailEntries: []types.BulkEmailEntry{
			{
				Destination: &types.Destination{ToAddresses: []string{"a@example.com"}},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func findSentEmail(t *testing.T, emails []json.RawMessage, from, subject string) json.RawMessage {
	t.Helper()

	for _, raw := range emails {
		var email map[string]interface{}
		if err := json.Unmarshal(raw, &email); err != nil {
			continue
		}

		if email["FromEmailAddress"] == from && email["Subject"] == subject {
			return raw
		}
	}

	t.Fatalf("sent email from %s with subject %q not found", from, subject)

	return nil
}
