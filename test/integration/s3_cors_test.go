//go:build integration

package integration

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestS3_CORS(t *testing.T) {
	s3Client := newS3Client(t)
	ctx := t.Context()

	bucketName := "cors-test-bucket"

	// 1. Create bucket.
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Set CORS configuration.
	corsXML := `<CORSConfiguration>
		<CORSRule>
			<AllowedOrigin>http://localhost:3200</AllowedOrigin>
			<AllowedMethod>GET</AllowedMethod>
			<AllowedMethod>PUT</AllowedMethod>
			<AllowedMethod>POST</AllowedMethod>
			<AllowedMethod>HEAD</AllowedMethod>
			<AllowedHeader>*</AllowedHeader>
		</CORSRule>
	</CORSConfiguration>`

	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		testEndpoint()+"/"+bucketName+"?cors",
		bytes.NewReader([]byte(corsXML)))
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PutBucketCors returned %d", resp.StatusCode)
	}

	// 3. Upload an object.
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("test.txt"),
		Body:   bytes.NewReader([]byte("hello")),
	})
	if err != nil {
		t.Fatal(err)
	}

	// 4. GET with matching Origin - should have CORS headers.
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		testEndpoint()+"/"+bucketName+"/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}

	getReq.Header.Set("Origin", "http://localhost:3200")

	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = io.Copy(io.Discard, getResp.Body)
	getResp.Body.Close()

	acao := getResp.Header.Get("Access-Control-Allow-Origin")
	if acao != "http://localhost:3200" {
		t.Errorf("expected Access-Control-Allow-Origin=http://localhost:3200, got %q", acao)
	}

	// 5. GET with non-matching Origin - should NOT have CORS headers.
	getReq2, err := http.NewRequestWithContext(ctx, http.MethodGet,
		testEndpoint()+"/"+bucketName+"/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}

	getReq2.Header.Set("Origin", "http://evil.example.com")

	getResp2, err := http.DefaultClient.Do(getReq2)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = io.Copy(io.Discard, getResp2.Body)
	getResp2.Body.Close()

	acao2 := getResp2.Header.Get("Access-Control-Allow-Origin")
	if acao2 != "" {
		t.Errorf("expected no CORS header for non-matching origin, got %q", acao2)
	}

	// 6. OPTIONS preflight with matching Origin.
	optReq, err := http.NewRequestWithContext(ctx, http.MethodOptions,
		testEndpoint()+"/"+bucketName+"/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}

	optReq.Header.Set("Origin", "http://localhost:3200")
	optReq.Header.Set("Access-Control-Request-Method", "PUT")

	optResp, err := http.DefaultClient.Do(optReq)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = io.Copy(io.Discard, optResp.Body)
	optResp.Body.Close()

	if optResp.StatusCode != http.StatusOK {
		t.Fatalf("OPTIONS returned %d", optResp.StatusCode)
	}

	optACO := optResp.Header.Get("Access-Control-Allow-Origin")
	if optACO != "http://localhost:3200" {
		t.Errorf("OPTIONS: expected Access-Control-Allow-Origin=http://localhost:3200, got %q", optACO)
	}

	// 7. GET without Origin - should NOT have CORS headers.
	getReq3, err := http.NewRequestWithContext(ctx, http.MethodGet,
		testEndpoint()+"/"+bucketName+"/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}

	getResp3, err := http.DefaultClient.Do(getReq3)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = io.Copy(io.Discard, getResp3.Body)
	getResp3.Body.Close()

	acao3 := getResp3.Header.Get("Access-Control-Allow-Origin")
	if acao3 != "" {
		t.Errorf("expected no CORS header without Origin, got %q", acao3)
	}
}
