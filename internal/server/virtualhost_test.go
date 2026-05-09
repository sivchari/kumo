package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRouter_VirtualHostedStyleS3 covers AWS S3 virtual-hosted-style
// addressing: the bucket sits in the Host header rather than the
// URL path. Without rewrite the request `HEAD http://b.localhost/`
// hits the wildcard `/{bucket}` route with bucket="" and returns 200,
// which the AWS SDK reads as "bucket exists" → BucketAlreadyExists
// when terraform tries to create.
//
// Recognised host shapes:
//   - <bucket>.localhost(:port)       (kumo-style custom endpoint)
//   - <bucket>.s3.amazonaws.com
//   - <bucket>.s3-<region>.amazonaws.com
//   - <bucket>.s3.<region>.amazonaws.com
func TestRouter_VirtualHostedStyleS3(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	makeRouter := func() (*Router, *string) {
		r := NewRouter(logger)
		matched := new(string)

		r.Handle("PUT", "/{bucket}", func(w http.ResponseWriter, req *http.Request) {
			*matched = "create-bucket:" + req.PathValue("bucket")
			w.WriteHeader(http.StatusOK)
		})
		r.Handle("HEAD", "/{bucket}", func(w http.ResponseWriter, req *http.Request) {
			*matched = "head-bucket:" + req.PathValue("bucket")
			w.WriteHeader(http.StatusOK)
		})
		r.Handle("PUT", "/{bucket}/{key...}", func(w http.ResponseWriter, req *http.Request) {
			*matched = "put-object:" + req.PathValue("bucket") + "/" + req.PathValue("key")
			w.WriteHeader(http.StatusOK)
		})
		r.Handle("GET", "/", func(w http.ResponseWriter, _ *http.Request) {
			*matched = "list-buckets"
			w.WriteHeader(http.StatusOK)
		})

		return r, matched
	}

	cases := []struct {
		name   string
		method string
		host   string
		path   string
		want   string
	}{
		{
			name:   "path-style PUT bucket",
			method: "PUT",
			host:   "127.0.0.1:4566",
			path:   "/my-bucket",
			want:   "create-bucket:my-bucket",
		},
		{
			name:   "virtual-hosted PUT bucket via .localhost",
			method: "PUT",
			host:   "my-bucket.localhost:4566",
			path:   "/",
			want:   "create-bucket:my-bucket",
		},
		{
			name:   "virtual-hosted HEAD bucket",
			method: "HEAD",
			host:   "my-bucket.localhost:4566",
			path:   "/",
			want:   "head-bucket:my-bucket",
		},
		{
			name:   "virtual-hosted PUT object",
			method: "PUT",
			host:   "my-bucket.localhost:4566",
			path:   "/some/key.txt",
			want:   "put-object:my-bucket/some/key.txt",
		},
		{
			name:   "virtual-hosted via real AWS S3 host",
			method: "PUT",
			host:   "my-bucket.s3.us-east-1.amazonaws.com",
			path:   "/",
			want:   "create-bucket:my-bucket",
		},
		{
			name:   "virtual-hosted via legacy region-prefixed host",
			method: "PUT",
			host:   "my-bucket.s3-us-west-2.amazonaws.com",
			path:   "/",
			want:   "create-bucket:my-bucket",
		},
		{
			name:   "virtual-hosted via global host",
			method: "PUT",
			host:   "my-bucket.s3.amazonaws.com",
			path:   "/",
			want:   "create-bucket:my-bucket",
		},
		{
			name:   "list-buckets (no virtual host) stays at /",
			method: "GET",
			host:   "127.0.0.1:4566",
			path:   "/",
			want:   "list-buckets",
		},
		{
			name:   "list-buckets via plain s3.amazonaws.com (no bucket prefix)",
			method: "GET",
			host:   "s3.amazonaws.com",
			path:   "/",
			want:   "list-buckets",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, matched := makeRouter()

			req := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, nil)
			req.Host = tc.host

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if *matched != tc.want {
				t.Fatalf("host=%s path=%s: got handler %q, want %q (status=%d)",
					tc.host, tc.path, *matched, tc.want, rec.Code)
			}
		})
	}
}

// TestExtractBucketFromHost is a low-level test of the host-to-bucket
// helper. It documents which host shapes are treated as virtual-hosted.
func TestExtractBucketFromHost(t *testing.T) {
	t.Parallel()

	cases := []struct {
		host string
		want string
	}{
		{"my-bucket.localhost:4566", "my-bucket"},
		{"my-bucket.localhost", "my-bucket"},
		{"my-bucket.s3.amazonaws.com", "my-bucket"},
		{"my-bucket.s3.us-east-1.amazonaws.com", "my-bucket"},
		{"my-bucket.s3-us-west-2.amazonaws.com", "my-bucket"},
		{"my-bucket.s3.dualstack.us-east-1.amazonaws.com", "my-bucket"},

		// Negative cases — must NOT extract a bucket.
		{"127.0.0.1:4566", ""},
		{"localhost", ""},
		{"localhost:4566", ""},
		{"s3.amazonaws.com", ""},
		{"s3.us-east-1.amazonaws.com", ""},
		{"s3-us-west-2.amazonaws.com", ""},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			got := extractBucketFromHost(tc.host)
			if got != tc.want {
				t.Errorf("extractBucketFromHost(%q) = %q, want %q", tc.host, got, tc.want)
			}
		})
	}
}
