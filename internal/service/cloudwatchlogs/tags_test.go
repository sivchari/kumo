package cloudwatchlogs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogGroupTagsRoundTrip(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage(defaultBaseURL)
	service := New(storage, defaultBaseURL)
	ctx := context.Background()

	if err := storage.CreateLogGroup(ctx, &CreateLogGroupRequest{
		LogGroupName: "/ecs/kumo-local/kumo",
		Tags: map[string]string{
			"Project":   "kumo-local",
			"Component": "kumo",
		},
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"logGroupName":"/ecs/kumo-local/kumo"}`))
	req.Header.Set("X-Amz-Target", "Logs_20140328.ListTagsLogGroup")

	rec := httptest.NewRecorder()

	service.DispatchAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	for _, want := range []string{
		`"tags"`,
		`"Project":"kumo-local"`,
		`"Component":"kumo"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, got %s", want, body)
		}
	}
}
