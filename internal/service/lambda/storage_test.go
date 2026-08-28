package lambda

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnmarshalJSONBackfillsLastUpdateStatus(t *testing.T) {
	t.Parallel()

	// Snapshots written before LastUpdateStatus existed restore functions
	// with the field empty; the terraform AWS provider then fails every
	// UpdateFunctionConfiguration/UpdateFunctionCode waiting for Successful.
	snapshot := `{"functions":{"legacy-fn":{"FunctionName":"legacy-fn","State":"Active"}},"eventSourceMappings":{}}`

	s := NewMemoryStorage("http://localhost:4566")
	if err := json.Unmarshal([]byte(snapshot), s); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	fn, err := s.GetFunction(t.Context(), "legacy-fn")
	if err != nil {
		t.Fatalf("GetFunction: %v", err)
	}

	if fn.LastUpdateStatus != "Successful" {
		t.Errorf("restored LastUpdateStatus = %q, want %q", fn.LastUpdateStatus, "Successful")
	}
}

func TestGetFunctionTagsSafeAgainstConcurrentTagWrites(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage("http://localhost:4566")
	svc := New(storage, "http://localhost:4566")
	ctx := t.Context()

	fn, err := storage.CreateFunction(ctx, &CreateFunctionRequest{
		FunctionName: "race-fn",
		Role:         "arn:aws:iam::000000000000:role/test-role",
		Tags:         map[string]string{"seed": "value"},
	})
	if err != nil {
		t.Fatalf("CreateFunction: %v", err)
	}

	// GetFunction serializes the function's tags into the response; that
	// must not race with TagResource/UntagResource mutating the stored map
	// (caught by the race detector, or a concurrent-map panic).
	done := make(chan struct{})

	go func() {
		defer close(done)

		for i := range 200 {
			if err := storage.TagResource(ctx, fn.FunctionArn, map[string]string{"k": fmt.Sprint(i)}); err != nil {
				t.Errorf("TagResource: %v", err)

				return
			}
		}
	}()

	for range 200 {
		req := httptest.NewRequest(http.MethodGet, "/2015-03-31/functions/race-fn", http.NoBody)

		w := httptest.NewRecorder()
		svc.GetFunction(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GetFunction status: got %d, want %d", w.Code, http.StatusOK)
		}
	}

	<-done
}
