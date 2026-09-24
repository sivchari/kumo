package lambda

import (
	"encoding/json"
	"errors"
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
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/2015-03-31/functions/race-fn", http.NoBody)

		w := httptest.NewRecorder()
		svc.GetFunction(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("GetFunction status: got %d, want %d", w.Code, http.StatusOK)
		}
	}

	<-done
}

const (
	testFunctionName = "my-fn"
	testRoleARN      = "arn:aws:iam::000000000000:role/test-role"
)

// newTestStorageWithFunctions returns a storage in ap-northeast-1 holding
// the named functions.
func newTestStorageWithFunctions(t *testing.T, names ...string) *MemoryStorage {
	t.Helper()

	s := NewMemoryStorage("http://localhost:4566")
	s.region = "ap-northeast-1"

	for _, name := range names {
		if _, err := s.CreateFunction(t.Context(), &CreateFunctionRequest{
			FunctionName: name,
			Role:         testRoleARN,
		}); err != nil {
			t.Fatalf("CreateFunction(%q): %v", name, err)
		}
	}

	return s
}

func TestGetFunctionResolvesFunctionNameFormats(t *testing.T) {
	t.Parallel()

	// A function whose name is itself an ARN stays reachable by that exact
	// name.
	arnNamed := "arn:aws:lambda:ap-northeast-1:000000000000:function:other-fn"
	s := newTestStorageWithFunctions(t, testFunctionName, arnNamed)

	tests := []struct {
		name         string
		functionName string
		want         string // "" means ResourceNotFoundException
	}{
		{name: "name", functionName: testFunctionName, want: testFunctionName},
		{name: "full ARN", functionName: "arn:aws:lambda:ap-northeast-1:000000000000:function:my-fn", want: testFunctionName},
		{name: "partial ARN", functionName: "000000000000:function:my-fn", want: testFunctionName},
		{name: "name with $LATEST", functionName: "my-fn:$LATEST", want: testFunctionName},
		{name: "full ARN with $LATEST", functionName: "arn:aws:lambda:ap-northeast-1:000000000000:function:my-fn:$LATEST", want: testFunctionName},
		{name: "function named like an ARN", functionName: arnNamed, want: arnNamed},
		{name: "version qualifier", functionName: "my-fn:1"},
		{name: "alias qualifier", functionName: "arn:aws:lambda:ap-northeast-1:000000000000:function:my-fn:live"},
		{name: "another region", functionName: "arn:aws:lambda:us-east-1:000000000000:function:my-fn"},
		{name: "another account", functionName: "111111111111:function:my-fn"},
		{name: "missing function", functionName: "arn:aws:lambda:ap-northeast-1:000000000000:function:missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fn, err := s.GetFunction(t.Context(), tt.functionName)

			if tt.want == "" {
				var lambdaErr *FunctionError
				if !errors.As(err, &lambdaErr) || lambdaErr.Type != ErrResourceNotFound {
					t.Errorf("GetFunction(%q) = %v, %v; want ResourceNotFoundException", tt.functionName, fn, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("GetFunction(%q): %v", tt.functionName, err)
			}

			if fn.FunctionName != tt.want {
				t.Errorf("GetFunction(%q) resolved to %q, want %q", tt.functionName, fn.FunctionName, tt.want)
			}
		})
	}
}

func TestDeleteFunctionByARN(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")

	fn, err := s.CreateFunction(t.Context(), &CreateFunctionRequest{
		FunctionName: testFunctionName,
		Role:         testRoleARN,
	})
	if err != nil {
		t.Fatalf("CreateFunction: %v", err)
	}

	if err := s.DeleteFunction(t.Context(), fn.FunctionArn); err != nil {
		t.Fatalf("DeleteFunction(%q): %v", fn.FunctionArn, err)
	}

	if _, err := s.GetFunction(t.Context(), testFunctionName); err == nil {
		t.Error("function still exists after DeleteFunction by ARN")
	}
}
