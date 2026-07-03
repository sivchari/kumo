package sesv2

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEpochSeconds_MarshalsAsJSONNumber(t *testing.T) {
	// Pin to a known instant so the assertion is exact.
	ts := time.Unix(1700000000, 500_000_000) // 1.7e9 + 0.5 sec

	got, err := json.Marshal(epochSeconds(ts))
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	out := string(got)
	if strings.Contains(out, `"`) {
		t.Fatalf("expected JSON number, got string %q", out)
	}

	if _, err := strconv.ParseFloat(out, 64); err != nil {
		t.Fatalf("expected parseable float, got %q (err=%v)", out, err)
	}

	// Must round-trip to the same instant (within float precision).
	const want = 1700000000.5

	got64, _ := strconv.ParseFloat(out, 64)
	if diff := got64 - want; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("expected ~%v, got %v", want, got64)
	}
}

func TestSendEmail_RawEmailWithoutDestination(t *testing.T) {
	storage := &MemoryStorage{}
	ctx := context.Background()

	rawMessage := "From: sender@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Cc: cc@example.com\r\n" +
		"Subject: Test Subject\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Test body"

	req := &SendEmailRequest{
		FromEmailAddress: "sender@example.com",
		// Destination is intentionally nil
		Content: &EmailContent{
			Raw: &RawEmail{
				Data: []byte(rawMessage),
			},
		},
	}

	messageID, err := storage.SendEmail(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if messageID == "" {
		t.Fatal("expected non-empty message ID")
	}

	// Verify that the sent email was stored
	sentEmails, err := storage.GetSentEmails(ctx)
	if err != nil {
		t.Fatalf("failed to get sent emails: %v", err)
	}

	if len(sentEmails) == 0 {
		t.Fatal("expected sent email to be stored")
	}

	email := sentEmails[0]
	if email.MessageID != messageID {
		t.Errorf("expected message ID %s, got %s", messageID, email.MessageID)
	}

	// Verify destination was extracted from MIME headers
	if email.Destination == nil {
		t.Fatal("expected destination to be extracted from MIME headers")
	}

	if len(email.Destination.ToAddresses) != 1 || email.Destination.ToAddresses[0] != "recipient@example.com" {
		t.Errorf("expected To: recipient@example.com, got %v", email.Destination.ToAddresses)
	}

	if len(email.Destination.CcAddresses) != 1 || email.Destination.CcAddresses[0] != "cc@example.com" {
		t.Errorf("expected Cc: cc@example.com, got %v", email.Destination.CcAddresses)
	}

	if email.Subject != "Test Subject" {
		t.Errorf("expected subject 'Test Subject', got '%s'", email.Subject)
	}
}

func TestSendEmail_SimpleEmailWithoutDestination_ShouldFail(t *testing.T) {
	storage := &MemoryStorage{}
	ctx := context.Background()

	req := &SendEmailRequest{
		FromEmailAddress: "sender@example.com",
		// Destination is nil
		Content: &EmailContent{
			Simple: &SimpleEmail{
				Subject: &Content{
					Data: "Test",
				},
				Body: &Body{
					Text: &Content{
						Data: "Test body",
					},
				},
			},
		},
	}

	_, err := storage.SendEmail(ctx, req)
	if err == nil {
		t.Fatal("expected error for simple email without destination")
	}

	var identityErr *IdentityError
	if !errors.As(err, &identityErr) {
		t.Fatalf("expected IdentityError, got %T", err)
	}

	if identityErr.Message != "Destination is required" {
		t.Errorf("expected 'Destination is required', got '%s'", identityErr.Message)
	}
}

func TestEmailTemplate_CreateAndGet(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	if _, err := storage.CreateEmailTemplate(ctx, &CreateEmailTemplateRequest{
		TemplateName: "welcome",
		TemplateContent: &EmailTemplateContent{
			Subject: "Hello",
			Text:    "Body text",
			HTML:    "<p>Body HTML</p>",
		},
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err := storage.GetEmailTemplate(ctx, "welcome")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if got.TemplateContent == nil || got.TemplateContent.Subject != "Hello" {
		t.Fatalf("get returned unexpected content: %+v", got.TemplateContent)
	}
}

func TestEmailTemplate_Update(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	if _, err := storage.CreateEmailTemplate(ctx, &CreateEmailTemplateRequest{
		TemplateName: "welcome",
		TemplateContent: &EmailTemplateContent{
			Subject: "Hello",
			Text:    "Body text",
			HTML:    "<p>Body HTML</p>",
		},
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if _, err := storage.UpdateEmailTemplate(ctx, "welcome", &UpdateEmailTemplateRequest{
		TemplateContent: &EmailTemplateContent{
			Subject: "Hello v2",
			Text:    "Body text v2",
		},
	}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	updated, err := storage.GetEmailTemplate(ctx, "welcome")
	if err != nil {
		t.Fatalf("get after update failed: %v", err)
	}

	if updated.TemplateContent.Subject != "Hello v2" {
		t.Errorf("expected updated subject, got %q", updated.TemplateContent.Subject)
	}

	if updated.TemplateContent.HTML != "" {
		t.Errorf("expected HTML cleared on update, got %q", updated.TemplateContent.HTML)
	}
}

func TestEmailTemplate_List(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	if _, err := storage.CreateEmailTemplate(ctx, &CreateEmailTemplateRequest{
		TemplateName:    "welcome",
		TemplateContent: &EmailTemplateContent{Subject: "Hello"},
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	list, _, err := storage.ListEmailTemplates(ctx, "", 10)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if len(list) != 1 || list[0].Name != "welcome" {
		t.Fatalf("unexpected list result: %+v", list)
	}
}

func TestEmailTemplate_Delete(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	if _, err := storage.CreateEmailTemplate(ctx, &CreateEmailTemplateRequest{
		TemplateName:    "welcome",
		TemplateContent: &EmailTemplateContent{Subject: "Hello"},
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if err := storage.DeleteEmailTemplate(ctx, "welcome"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if _, err := storage.GetEmailTemplate(ctx, "welcome"); err == nil {
		t.Fatal("expected error for deleted template")
	}
}

func TestEmailTemplate_DuplicateCreate(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	req := &CreateEmailTemplateRequest{
		TemplateName:    "dup",
		TemplateContent: &EmailTemplateContent{Subject: "s"},
	}

	if _, err := storage.CreateEmailTemplate(ctx, req); err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err := storage.CreateEmailTemplate(ctx, req)
	if err == nil {
		t.Fatal("expected error on duplicate create")
	}

	var iErr *IdentityError
	if !errors.As(err, &iErr) || iErr.Code != errAlreadyExists {
		t.Fatalf("expected errAlreadyExists, got %v", err)
	}
}

func TestEmailTemplate_GetNotFound(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	_, err := storage.GetEmailTemplate(ctx, "missing")
	if err == nil {
		t.Fatal("expected error for missing template")
	}

	var iErr *IdentityError
	if !errors.As(err, &iErr) || iErr.Code != errNotFound {
		t.Fatalf("expected errNotFound, got %v", err)
	}
}

// newBulkSendFixture seeds a template named "promo" and returns a request that
// targets two distinct recipients with per-entry replacement data.
func newBulkSendFixture(t *testing.T, storage *MemoryStorage) *SendBulkEmailRequest {
	t.Helper()

	if _, err := storage.CreateEmailTemplate(context.Background(), &CreateEmailTemplateRequest{
		TemplateName: "promo",
		TemplateContent: &EmailTemplateContent{
			Subject: "Promo",
			Text:    "Hello {{name}}",
		},
	}); err != nil {
		t.Fatalf("template create failed: %v", err)
	}

	return &SendBulkEmailRequest{
		FromEmailAddress: "sender@example.com",
		DefaultContent: &BulkEmailContent{
			Template: &Template{TemplateName: "promo", TemplateData: "{}"},
		},
		BulkEmailEntries: []BulkEmailEntry{
			{
				Destination: &Destination{ToAddresses: []string{"a@example.com"}},
				ReplacementEmailContent: &ReplacementEmailContent{
					ReplacementTemplate: &ReplacementTemplate{ReplacementTemplateData: `{"name":"A"}`},
				},
			},
			{
				Destination: &Destination{ToAddresses: []string{"b@example.com"}},
				ReplacementEmailContent: &ReplacementEmailContent{
					ReplacementTemplate: &ReplacementTemplate{ReplacementTemplateData: `{"name":"B"}`},
				},
			},
		},
	}
}

func TestSendBulkEmail_AssignsMessageIDPerEntry(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	req := newBulkSendFixture(t, storage)

	resp, err := storage.SendBulkEmail(ctx, req)
	if err != nil {
		t.Fatalf("send bulk failed: %v", err)
	}

	if len(resp.BulkEmailEntryResults) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.BulkEmailEntryResults))
	}

	ids := map[string]struct{}{}

	for i, r := range resp.BulkEmailEntryResults {
		if r.Status != "SUCCESS" {
			t.Errorf("entry %d: status=%q want SUCCESS (err=%q)", i, r.Status, r.Error)
		}

		if r.MessageID == "" {
			t.Errorf("entry %d: empty MessageId", i)
		}

		ids[r.MessageID] = struct{}{}
	}

	if len(ids) != 2 {
		t.Errorf("expected 2 distinct MessageIds, got %d", len(ids))
	}
}

func TestSendBulkEmail_RecordsSentEmails(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()
	req := newBulkSendFixture(t, storage)

	if _, err := storage.SendBulkEmail(ctx, req); err != nil {
		t.Fatalf("send bulk failed: %v", err)
	}

	sent, err := storage.GetSentEmails(ctx)
	if err != nil {
		t.Fatalf("get sent emails failed: %v", err)
	}

	if len(sent) != 2 {
		t.Fatalf("expected 2 stored emails, got %d", len(sent))
	}

	if sent[0].TemplateName != "promo" {
		t.Errorf("expected TemplateName=promo, got %q", sent[0].TemplateName)
	}

	if sent[0].TemplateData != `{"name":"A"}` {
		t.Errorf("expected per-entry replacement data, got %q", sent[0].TemplateData)
	}
}

func TestSendBulkEmail_UnknownTemplateFails(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	_, err := storage.SendBulkEmail(ctx, &SendBulkEmailRequest{
		FromEmailAddress: "sender@example.com",
		DefaultContent: &BulkEmailContent{
			Template: &Template{TemplateName: "nope"},
		},
		BulkEmailEntries: []BulkEmailEntry{
			{Destination: &Destination{ToAddresses: []string{"a@example.com"}}},
		},
	})
	if err == nil {
		t.Fatal("expected error for unknown template")
	}

	var iErr *IdentityError
	if !errors.As(err, &iErr) || iErr.Code != errNotFound {
		t.Fatalf("expected errNotFound, got %v", err)
	}
}

func TestSendBulkEmail_EntryWithoutDestinationFailsIndividually(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	if _, err := storage.CreateEmailTemplate(ctx, &CreateEmailTemplateRequest{
		TemplateName:    "tpl",
		TemplateContent: &EmailTemplateContent{Subject: "S"},
	}); err != nil {
		t.Fatalf("template create failed: %v", err)
	}

	resp, err := storage.SendBulkEmail(ctx, &SendBulkEmailRequest{
		FromEmailAddress: "sender@example.com",
		DefaultContent: &BulkEmailContent{
			Template: &Template{TemplateName: "tpl"},
		},
		BulkEmailEntries: []BulkEmailEntry{
			{Destination: &Destination{ToAddresses: []string{"a@example.com"}}},
			{ /* destination missing */ },
		},
	})
	if err != nil {
		t.Fatalf("send bulk failed: %v", err)
	}

	if got := resp.BulkEmailEntryResults[0].Status; got != "SUCCESS" {
		t.Errorf("entry 0 status=%q want SUCCESS", got)
	}

	if got := resp.BulkEmailEntryResults[1].Status; got != "FAILED" {
		t.Errorf("entry 1 status=%q want FAILED", got)
	}
}
