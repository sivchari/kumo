//go:build go1.27

package kumo_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/sivchari/kumo"
)

func TestNewTestServerHealth(t *testing.T) {
	t.Parallel()

	srv := kumo.NewTestServer(t)

	// The in-memory client routes every request to the server, whatever the
	// destination address, so the host here is arbitrary.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/health", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get /health: %v", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if got, want := string(body), `{"status":"healthy"}`; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestNewTestServerSynctest(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		srv := kumo.NewTestServer(t)

		client := s3.NewFromConfig(aws.Config{
			Region:      "us-east-1",
			Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""),
			HTTPClient:  srv.Client(),
		}, func(o *s3.Options) {
			o.BaseEndpoint = aws.String("http://example.com")
			o.UsePathStyle = true
		})

		ctx := context.Background()

		if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String("kumo-synctest"),
		}); err != nil {
			t.Fatalf("create bucket: %v", err)
		}

		out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
		if err != nil {
			t.Fatalf("list buckets: %v", err)
		}

		if len(out.Buckets) != 1 || aws.ToString(out.Buckets[0].Name) != "kumo-synctest" {
			t.Errorf("ListBuckets = %+v, want a single bucket named kumo-synctest", out.Buckets)
		}
	})
}
