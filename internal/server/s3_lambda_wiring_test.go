package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestS3ToLambdaInvoker_InvokeAsync_StatusHandling regression-tests a bug
// where InvokeAsync ignored the HTTP response status from the Lambda invoke
// endpoint: a 404 (function not found) or 500 returned nil, so the S3 event
// notification silently vanished instead of surfacing an error the caller
// could log. Any status >= 300 must now produce a non-nil error that
// includes the status code; success statuses (e.g. 202 Accepted, which is
// what the real Lambda invoke endpoint returns for async invocations) must
// still return nil.
func TestS3ToLambdaInvoker_InvokeAsync_StatusHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
		wantSubstr string
	}{
		{
			name:       "server error surfaces as an error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
			wantSubstr: "500",
		},
		{
			name:       "accepted returns no error",
			statusCode: http.StatusAccepted,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			// A dedicated Transport keeps the parallel subtests off the shared
			// http.DefaultTransport: httptest's Close closes its idle
			// connections, killing the sibling subtest's in-flight request.
			inv := &s3ToLambdaInvoker{
				baseURL:    srv.URL,
				httpClient: &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{}},
			}

			err := inv.InvokeAsync(context.Background(), "arn:aws:lambda:us-east-1:123456789012:function:my-func", []byte(`{}`))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("InvokeAsync() error = nil, want error containing %q", tt.wantSubstr)
				}

				if !strings.Contains(err.Error(), tt.wantSubstr) {
					t.Fatalf("InvokeAsync() error = %q, want it to contain %q", err.Error(), tt.wantSubstr)
				}

				return
			}

			if err != nil {
				t.Fatalf("InvokeAsync() error = %v, want nil", err)
			}
		})
	}
}
