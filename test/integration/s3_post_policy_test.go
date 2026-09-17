//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	postPolicyContent    = "hello, POST policy"
	msgPostAccessDenied  = "AccessDenied"
	msgPostConditionFail = "Invalid according to Policy: Policy Condition failed: "
)

// s3ErrorBody is the S3 XML error document, including the size elements that
// POST Object adds for content-length-range violations.
type s3ErrorBody struct {
	Code           string `xml:"Code"`
	Message        string `xml:"Message"`
	ProposedSize   string `xml:"ProposedSize"`
	MinSizeAllowed string `xml:"MinSizeAllowed"`
	MaxSizeAllowed string `xml:"MaxSizeAllowed"`
}

// readS3Error asserts the response status, code and message of an S3 error
// and returns the decoded document.
func readS3Error(t *testing.T, resp *http.Response, wantStatus int, wantCode, wantMessage string) s3ErrorBody {
	t.Helper()

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read error body: %v", err)
	}

	if resp.StatusCode != wantStatus {
		t.Fatalf("expected status %d, got %d: %s", wantStatus, resp.StatusCode, body)
	}

	var parsed s3ErrorBody
	if err := xml.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decode error body %q: %v", body, err)
	}

	if parsed.Code != wantCode || parsed.Message != wantMessage {
		t.Fatalf("expected %s %q, got %s %q", wantCode, wantMessage, parsed.Code, parsed.Message)
	}

	return parsed
}

func expectPostAccepted(t *testing.T, resp *http.Response) {
	t.Helper()

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 204, got %d: %s", resp.StatusCode, body)
	}
}

// newPostPolicyBucket creates bucket and removes keys and the bucket on cleanup.
func newPostPolicyBucket(t *testing.T, client *s3.Client, bucket string, keys ...string) {
	t.Helper()

	if _, err := client.CreateBucket(t.Context(), &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		for _, key := range keys {
			_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		}

		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})
}

// presignPost signs a POST for bucket/key with the given extra policy conditions.
func presignPost(t *testing.T, bucket, key string, conditions ...any) *s3.PresignedPostRequest {
	t.Helper()

	presigned, err := newS3PresignClient(t).PresignPostObject(t.Context(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignPostOptions) {
		opts.Expires = 15 * time.Minute
		opts.Conditions = conditions
	})
	if err != nil {
		t.Fatalf("failed to presign POST: %v", err)
	}

	return presigned
}

func assertObjectAbsent(t *testing.T, client *s3.Client, bucket, key string) {
	t.Helper()

	_, err := client.HeadObject(t.Context(), &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})

	var notFound *types.NotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("expected %s/%s to be absent, got err=%v", bucket, key, err)
	}
}

func TestS3_PresignedPost_PolicyConditionFailed(t *testing.T) {
	client := newS3Client(t)
	bucket := "test-post-policy-condition"
	key := "outside/upload.txt"
	newPostPolicyBucket(t, client, bucket, key)

	presigned := presignPost(t, bucket, key, []any{"starts-with", "$key", "allowed/"})

	resp := postPresignedForm(t, presigned, []byte(postPolicyContent), nil)
	readS3Error(t, resp, http.StatusForbidden, msgPostAccessDenied, msgPostConditionFail+`["starts-with", "$key", "allowed/"]`)
	assertObjectAbsent(t, client, bucket, key)
}

func TestS3_PresignedPost_ExtraInputField(t *testing.T) {
	client := newS3Client(t)
	bucket := "test-post-policy-extra-field"
	key := "upload.txt"
	newPostPolicyBucket(t, client, bucket, key)

	presigned := presignPost(t, bucket, key)

	resp := postPresignedForm(t, presigned, []byte(postPolicyContent), map[string]string{"x-custom": "1"})
	readS3Error(t, resp, http.StatusForbidden, msgPostAccessDenied, "Invalid according to Policy: Extra input fields: x-custom")
	assertObjectAbsent(t, client, bucket, key)
}

func TestS3_PresignedPost_IgnoredFieldIsAccepted(t *testing.T) {
	client := newS3Client(t)
	bucket := "test-post-policy-ignored-field"
	key := "upload.txt"
	newPostPolicyBucket(t, client, bucket, key)

	presigned := presignPost(t, bucket, key)

	expectPostAccepted(t, postPresignedForm(t, presigned, []byte(postPolicyContent), map[string]string{"x-ignore-tracker": "1"}))
}

func TestS3_PresignedPost_ContentLengthRange(t *testing.T) {
	client := newS3Client(t)
	bucket := "test-post-policy-length-range"
	key := "upload.txt"
	newPostPolicyBucket(t, client, bucket, key)

	presigned := presignPost(t, bucket, key, []any{"content-length-range", 10, 20})

	small := readS3Error(t, postPresignedForm(t, presigned, bytes.Repeat([]byte("a"), 5), nil),
		http.StatusBadRequest, "EntityTooSmall", "Your proposed upload is smaller than the minimum allowed size")
	if small.ProposedSize != "5" || small.MinSizeAllowed != "10" {
		t.Fatalf("EntityTooSmall elements: got ProposedSize=%q MinSizeAllowed=%q", small.ProposedSize, small.MinSizeAllowed)
	}

	large := readS3Error(t, postPresignedForm(t, presigned, bytes.Repeat([]byte("a"), 30), nil),
		http.StatusBadRequest, "EntityTooLarge", "Your proposed upload exceeds the maximum allowed size")
	if large.ProposedSize != "30" || large.MaxSizeAllowed != "20" {
		t.Fatalf("EntityTooLarge elements: got ProposedSize=%q MaxSizeAllowed=%q", large.ProposedSize, large.MaxSizeAllowed)
	}

	for _, size := range []int{10, 20} {
		expectPostAccepted(t, postPresignedForm(t, presigned, bytes.Repeat([]byte("a"), size), nil))
	}
}

func TestS3_PresignedPost_ContentLengthRangeZeroValues(t *testing.T) {
	client := newS3Client(t)
	bucket := "test-post-policy-length-zero"
	key := "upload.txt"
	newPostPolicyBucket(t, client, bucket, key)

	empty := readS3Error(t, postPresignedForm(t, presignPost(t, bucket, key, []any{"content-length-range", 1, 10}), nil, nil),
		http.StatusBadRequest, "EntityTooSmall", "Your proposed upload is smaller than the minimum allowed size")
	if empty.ProposedSize != "0" || empty.MinSizeAllowed != "1" {
		t.Fatalf("empty upload elements: got ProposedSize=%q MinSizeAllowed=%q", empty.ProposedSize, empty.MinSizeAllowed)
	}

	zeroMax := readS3Error(t, postPresignedForm(t, presignPost(t, bucket, key, []any{"content-length-range", 0, 0}), []byte("a"), nil),
		http.StatusBadRequest, "EntityTooLarge", "Your proposed upload exceeds the maximum allowed size")
	if zeroMax.ProposedSize != "1" || zeroMax.MaxSizeAllowed != "0" {
		t.Fatalf("zero maximum elements: got ProposedSize=%q MaxSizeAllowed=%q", zeroMax.ProposedSize, zeroMax.MaxSizeAllowed)
	}
}

func TestS3_PresignedPost_ContentTypeList(t *testing.T) {
	client := newS3Client(t)
	bucket := "test-post-policy-content-type-list"
	key := "upload.txt"
	newPostPolicyBucket(t, client, bucket, key)

	presigned := presignPost(t, bucket, key, []any{"starts-with", "$Content-Type", "image/"})

	expectPostAccepted(t, postPresignedForm(t, presigned, []byte(postPolicyContent), map[string]string{"Content-Type": "image/png,image/gif"}))

	resp := postPresignedForm(t, presigned, []byte(postPolicyContent), map[string]string{"Content-Type": "image/png,text/plain"})
	readS3Error(t, resp, http.StatusForbidden, msgPostAccessDenied, msgPostConditionFail+`["starts-with", "$Content-Type", "image/"]`)
}

func TestS3_PresignedPost_FieldNamesMatchCaseInsensitively(t *testing.T) {
	client := newS3Client(t)
	bucket := "test-post-policy-field-case"
	key := "upload.txt"
	newPostPolicyBucket(t, client, bucket, key)

	presigned := presignPost(t, bucket, key, []any{"starts-with", "$Content-Type", ""})

	expectPostAccepted(t, postPresignedForm(t, presigned, []byte(postPolicyContent), map[string]string{"content-type": "text/csv"}))
}

func TestS3_PresignedPost_KeyIsValidatedAfterFilenameExpansion(t *testing.T) {
	client := newS3Client(t)
	bucket := "test-post-policy-filename"
	stored := "allowed/upload.txt"
	newPostPolicyBucket(t, client, bucket, stored)

	presigned := presignPost(t, bucket, "allowed/${filename}", []any{"starts-with", "$key", "allowed/"})

	expectPostAccepted(t, postPresignedForm(t, presigned, []byte(postPolicyContent), nil))

	if _, err := client.HeadObject(t.Context(), &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(stored)}); err != nil {
		t.Fatalf("expected %s to be stored: %v", stored, err)
	}
}

func TestS3_PresignedPost_MalformedPolicy(t *testing.T) {
	client := newS3Client(t)
	bucket := "test-post-policy-malformed"
	key := "upload.txt"
	newPostPolicyBucket(t, client, bucket, key)

	presigned := presignPost(t, bucket, key)
	presigned.Values["policy"] = "%%%not-base64%%%"

	resp := postPresignedForm(t, presigned, []byte(postPolicyContent), nil)
	readS3Error(t, resp, http.StatusBadRequest, "InvalidPolicyDocument", "The content of the form does not meet the conditions specified in the policy document.")
	assertObjectAbsent(t, client, bucket, key)
}
