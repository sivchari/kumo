package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const knownFunctionURLID = "known"

const knownID = "abc123"

func TestExtractFunctionURLHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		host   string
		wantID string
		wantOK bool
	}{
		{"localhost with port", "abc123.lambda-url.localhost:4566", knownID, true},
		{"localhost without port", "abc123.lambda-url.localhost", knownID, true},
		{"on.aws with region", "abc123.lambda-url.us-east-1.on.aws", knownID, true},
		{"upper-case id is lowered", "ABC123.lambda-url.LOCALHOST", knownID, true},
		{"extra label after region", "abc123.lambda-url.us-east-1.extra.on.aws", "", false},
		{"on.aws without region", "abc123.lambda-url.on.aws", "", false},
		{"two id labels", "a.b.lambda-url.localhost", "", false},
		{"empty id", ".lambda-url.localhost", "", false},
		{"other domain", "abc123.lambda-url.example.com", "", false},
		{"execute-api host", "api1.execute-api.localhost", "", false},
		{"plain host", "127.0.0.1:4566", "", false},
		{"empty", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			id, ok := extractFunctionURLHost(tc.host)
			if id != tc.wantID || ok != tc.wantOK {
				t.Fatalf("extractFunctionURLHost(%q) = %q, %v; want %q, %v", tc.host, id, ok, tc.wantID, tc.wantOK)
			}
		})
	}
}

// newFunctionURLRouter wires a router with a function URL handler that owns
// one id, an execute-api handler and an S3-style wildcard route, recording
// which of them served a request.
func newFunctionURLRouter(calls *[]string) *Router {
	r := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)))

	r.AddFunctionURLHandler(func(w http.ResponseWriter, req *http.Request, urlID string) bool {
		if urlID != knownFunctionURLID {
			return false
		}

		*calls = append(*calls, "function-url "+urlID+" "+req.URL.Path)

		w.WriteHeader(http.StatusAccepted)

		return true
	})

	r.AddExecuteAPIHandler(func(w http.ResponseWriter, _ *http.Request, apiID, invokePath string) bool {
		*calls = append(*calls, "execute-api "+apiID+" "+invokePath)

		w.WriteHeader(http.StatusOK)

		return true
	})

	r.Handle(http.MethodGet, "/{bucket}/{key...}", func(w http.ResponseWriter, req *http.Request) {
		*calls = append(*calls, "s3 "+req.URL.Path)

		w.WriteHeader(http.StatusOK)
	})

	return r
}

func TestRouter_FunctionURLDispatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		host       string
		path       string
		wantStatus int
		wantCall   string
	}{
		{"function url health reaches the function", "known.lambda-url.localhost:4566", "/health", http.StatusAccepted, "function-url known /health"},
		{"function url path is passed untouched", "known.lambda-url.us-east-1.on.aws", "/items/1", http.StatusAccepted, "function-url known /items/1"},
		{"plain health stays healthy", "127.0.0.1:4566", "/health", http.StatusOK, ""},
		{"execute-api still dispatches", "api1.execute-api.localhost", "/dev/items", http.StatusOK, "execute-api api1 /dev/items"},
		{"s3 virtual host still rewrites", "my-bucket.localhost:4566", "/key.txt", http.StatusOK, "s3 /my-bucket/key.txt"},
		{"unknown url id is refused", "other.lambda-url.localhost", "/", http.StatusForbidden, ""},
		{"malformed lambda-url host falls through", "known.lambda-url.example.com", "/b/k", http.StatusOK, "s3 /b/k"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var calls []string

			r := newFunctionURLRouter(&calls)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+tc.host+tc.path, http.NoBody)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}

			got := strings.Join(calls, ";")
			if got != tc.wantCall {
				t.Fatalf("handled by %q, want %q", got, tc.wantCall)
			}
		})
	}
}

func TestRouter_UnknownFunctionURLIsAccessDenied(t *testing.T) {
	t.Parallel()

	var calls []string

	r := newFunctionURLRouter(&calls)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://other.lambda-url.localhost/", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden || rec.Header().Get("x-amzn-ErrorType") != "AccessDeniedException" {
		t.Fatalf("got %d %q", rec.Code, rec.Header().Get("x-amzn-ErrorType"))
	}

	if strings.TrimSpace(rec.Body.String()) != `{"Message":null}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}
