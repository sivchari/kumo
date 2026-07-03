package service_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/sivchari/kumo/internal/service"
)

func TestWriteJSONResponse(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()

	service.WriteJSONResponse(rec, service.ContentTypeAmzJSON11, map[string]string{"name": "alice"})

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != 200 {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}

	if got := res.Header.Get("Content-Type"); got != service.ContentTypeAmzJSON11 {
		t.Errorf("Content-Type = %q, want %q", got, service.ContentTypeAmzJSON11)
	}

	if res.Header.Get("X-Amzn-Requestid") == "" {
		t.Error("X-Amzn-Requestid header was not set")
	}

	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["name"] != "alice" {
		t.Errorf("body name = %q, want alice", body["name"])
	}
}
