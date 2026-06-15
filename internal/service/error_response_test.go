package service_test

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/sivchari/kumo/internal/service"
)

func TestWriteJSONError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	service.WriteJSONError(rec, service.ContentTypeAmzJSON10, "ValidationException", "bad input", 400)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != 400 {
		t.Errorf("status = %d, want 400", res.StatusCode)
	}

	if got := res.Header.Get("Content-Type"); got != service.ContentTypeAmzJSON10 {
		t.Errorf("Content-Type = %q, want %q", got, service.ContentTypeAmzJSON10)
	}

	if res.Header.Get("X-Amzn-Requestid") == "" {
		t.Error("X-Amzn-Requestid header was not set")
	}

	body, _ := io.ReadAll(res.Body)
	want := `{"__type":"ValidationException","message":"bad input"}` + "\n"

	if string(body) != want {
		t.Errorf("body = %q, want %q", string(body), want)
	}
}
