package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Host/result literals shared across virtualHostCases and the host-parsing
// test below.
const (
	pathStyleHost      = "127.0.0.1:4566"
	myBucketHost       = "my-bucket.localhost:4566"
	myBucketName       = "my-bucket"
	resultCreateBucket = "create-bucket:my-bucket"
	resultListBuckets  = "list-buckets"
)

// virtualHostCase is one row in the virtual-hosted dispatch table.
type virtualHostCase struct {
	name   string
	method string
	host   string
	path   string
	want   string
}

// virtualHostCases enumerates host / path combinations the router has
// to dispatch correctly. Defined once and reused so the test function
// itself stays inside funlen budget.
var virtualHostCases = []virtualHostCase{
	{
		name: "path-style PUT bucket", method: http.MethodPut,
		host: pathStyleHost, path: "/my-bucket",
		want: resultCreateBucket,
	},
	{
		name: "virtual-hosted PUT bucket via .localhost", method: http.MethodPut,
		host: myBucketHost, path: "/",
		want: resultCreateBucket,
	},
	{
		name: "virtual-hosted HEAD bucket", method: "HEAD",
		host: myBucketHost, path: "/",
		want: "head-bucket:my-bucket",
	},
	{
		name: "virtual-hosted PUT object", method: http.MethodPut,
		host: myBucketHost, path: "/some/key.txt",
		want: "put-object:my-bucket/some/key.txt",
	},
	{
		name: "virtual-hosted via real AWS S3 host", method: http.MethodPut,
		host: "my-bucket.s3.us-east-1.amazonaws.com", path: "/",
		want: resultCreateBucket,
	},
	{
		name: "virtual-hosted via legacy region-prefixed host", method: http.MethodPut,
		host: "my-bucket.s3-us-west-2.amazonaws.com", path: "/",
		want: resultCreateBucket,
	},
	{
		name: "virtual-hosted via global host", method: http.MethodPut,
		host: "my-bucket.s3.amazonaws.com", path: "/",
		want: resultCreateBucket,
	},
	{
		name: "list-buckets (no virtual host) stays at /", method: http.MethodGet,
		host: pathStyleHost, path: "/",
		want: resultListBuckets,
	},
	{
		name: "list-buckets via plain s3.amazonaws.com (no bucket prefix)", method: http.MethodGet,
		host: "s3.amazonaws.com", path: "/",
		want: resultListBuckets,
	},
}

// newS3StyleRouter wires up the four S3-style routes the dispatch
// tests exercise. The returned *string captures which handler matched.
func newS3StyleRouter() (*Router, *string) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewRouter(logger)
	matched := new(string)

	r.Handle(http.MethodPut, "/{bucket}", func(w http.ResponseWriter, req *http.Request) {
		*matched = "create-bucket:" + req.PathValue("bucket")

		w.WriteHeader(http.StatusOK)
	})
	r.Handle("HEAD", "/{bucket}", func(w http.ResponseWriter, req *http.Request) {
		*matched = "head-bucket:" + req.PathValue("bucket")

		w.WriteHeader(http.StatusOK)
	})
	r.Handle(http.MethodPut, "/{bucket}/{key...}", func(w http.ResponseWriter, req *http.Request) {
		*matched = "put-object:" + req.PathValue("bucket") + "/" + req.PathValue("key")

		w.WriteHeader(http.StatusOK)
	})
	r.Handle(http.MethodGet, "/", func(w http.ResponseWriter, _ *http.Request) {
		*matched = resultListBuckets

		w.WriteHeader(http.StatusOK)
	})

	return r, matched
}

// TestRouter_VirtualHostedStyleS3 covers AWS S3 virtual-hosted-style
// addressing: the bucket sits in the Host header rather than the
// URL path. Without rewrite the request `HEAD http://b.localhost/`
// hits the wildcard `/{bucket}` route with bucket="" and returns 200,
// which the AWS SDK reads as "bucket exists" → BucketAlreadyExists
// when terraform tries to create.
func TestRouter_VirtualHostedStyleS3(t *testing.T) {
	t.Parallel()

	for _, tc := range virtualHostCases {
		t.Run(tc.name, func(t *testing.T) {
			r, matched := newS3StyleRouter()

			req := httptest.NewRequestWithContext(t.Context(), tc.method, "http://"+tc.host+tc.path, http.NoBody)
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
		{myBucketHost, myBucketName},
		{"my-bucket.localhost", myBucketName},
		{"my-bucket.s3.amazonaws.com", myBucketName},
		{"my-bucket.s3.us-east-1.amazonaws.com", myBucketName},
		{"my-bucket.s3-us-west-2.amazonaws.com", myBucketName},
		{"my-bucket.s3.dualstack.us-east-1.amazonaws.com", myBucketName},

		// Negative cases — must NOT extract a bucket.
		{pathStyleHost, ""},
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
