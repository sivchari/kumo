package elbv2

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDescribeListenerAttributesReturnsEmptyAttributes(t *testing.T) {
	t.Parallel()

	service := New(NewMemoryStorage())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("Action=DescribeListenerAttributes&ListenerArn=arn-listener"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()

	service.DescribeListenerAttributes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, want := range []string{
		"<DescribeListenerAttributesResponse",
		"<DescribeListenerAttributesResult>",
		"<Attributes>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got %s", want, body)
		}
	}
}
