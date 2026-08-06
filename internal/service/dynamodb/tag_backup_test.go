package dynamodb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDescribeContinuousBackups_TableNotFound(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage("http://localhost:4566"))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"TableName":"missing"}`))
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810.DescribeContinuousBackups")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400, body=%s", w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "TableNotFoundException") {
		t.Fatalf("expected TableNotFoundException, got %s", w.Body.String())
	}
}
