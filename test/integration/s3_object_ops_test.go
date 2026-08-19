//go:build integration

package integration

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sivchari/golden"
)

// TestS3_ObjectACL verifies PutObjectAcl/GetObjectAcl roundtrips for both a
// canned ACL and an explicit grant list, plus GetObjectAcl on a missing key.
func TestS3_ObjectACL(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-object-acl"
	key := "acl-object.txt"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: strings.NewReader("x")}); err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// PUT with a canned ACL, GET back → a single FULL_CONTROL owner grant.
	if _, err := client.PutObjectAcl(ctx, &s3.PutObjectAclInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		ACL:    types.ObjectCannedACLPrivate,
	}); err != nil {
		t.Fatalf("failed to put object acl (canned): %v", err)
	}

	cannedResult, err := client.GetObjectAcl(ctx, &s3.GetObjectAclInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("failed to get object acl: %v", err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_canned", cannedResult)

	// PUT with an explicit grant list, GET back → the same grants.
	if _, err := client.PutObjectAcl(ctx, &s3.PutObjectAclInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		AccessControlPolicy: &types.AccessControlPolicy{
			Owner: &types.Owner{ID: aws.String("user-1"), DisplayName: aws.String("Alice")},
			Grants: []types.Grant{
				{
					Grantee:    &types.Grantee{Type: types.TypeCanonicalUser, ID: aws.String("user-1"), DisplayName: aws.String("Alice")},
					Permission: types.PermissionFullControl,
				},
				{
					Grantee:    &types.Grantee{Type: types.TypeGroup, URI: aws.String("http://acs.amazonaws.com/groups/global/AllUsers")},
					Permission: types.PermissionRead,
				},
			},
		},
	}); err != nil {
		t.Fatalf("failed to put object acl (explicit grants): %v", err)
	}

	explicitResult, err := client.GetObjectAcl(ctx, &s3.GetObjectAclInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("failed to get object acl: %v", err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_explicit", explicitResult)

	// GetObjectAcl on a missing key returns NoSuchKey, not a default ACL.
	_, err = client.GetObjectAcl(ctx, &s3.GetObjectAclInput{Bucket: aws.String(bucket), Key: aws.String("missing-key.txt")})
	if err == nil {
		t.Fatal("expected error getting acl of a missing key, got nil")
	}

	assertS3ErrorCode(t, err, "NoSuchKey")
}

// TestS3_ListObjectsV1_MarkerPagination verifies that the legacy (non
// list-type=2) ListObjects API paginates via Marker: page 1 is truncated
// once max-keys is hit, and page 2 (marker = last key of page 1) picks up
// where page 1 left off.
func TestS3_ListObjectsV1_MarkerPagination(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-list-objects-v1-marker"
	keys := []string{"a", "b", "c", "d", "e"}

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		for _, k := range keys {
			_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(k)})
		}
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	for _, k := range keys {
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(k), Body: strings.NewReader("x")}); err != nil {
			t.Fatalf("failed to put object %s: %v", k, err)
		}
	}

	page1, err := client.ListObjects(ctx, &s3.ListObjectsInput{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int32(2),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ETag", "LastModified", "ResultMetadata")).Assert(t.Name()+"_page1", page1)

	if len(page1.Contents) != 2 {
		t.Fatalf("page1 contents: got %d, want 2", len(page1.Contents))
	}

	if !aws.ToBool(page1.IsTruncated) {
		t.Fatal("page1 IsTruncated: got false, want true (5 total, 2 fetched)")
	}

	// AWS only populates NextMarker when a delimiter is set; without one,
	// clients page using the last key of the previous response as marker.
	lastKey := aws.ToString(page1.Contents[len(page1.Contents)-1].Key)

	page2, err := client.ListObjects(ctx, &s3.ListObjectsInput{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int32(2),
		Marker:  aws.String(lastKey),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ETag", "LastModified", "ResultMetadata")).Assert(t.Name()+"_page2", page2)

	if len(page2.Contents) != 2 {
		t.Fatalf("page2 contents: got %d, want 2 (entries after marker=%s)", len(page2.Contents), lastKey)
	}

	for _, c := range page2.Contents {
		if aws.ToString(c.Key) <= lastKey {
			t.Fatalf("page2 key %q should be > marker %q", aws.ToString(c.Key), lastKey)
		}
	}
}

// TestS3_UploadPartCopy copies an existing object into a new multipart
// upload (full source, then a byte-range source), completes the upload, and
// verifies the assembled object matches the copied bytes.
func TestS3_UploadPartCopy(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	srcBucket := "test-upload-part-copy-src"
	dstBucket := "test-upload-part-copy-dst"
	srcKey := "blob.txt"
	dstKey := "joined.txt"
	srcContent := "ABCDEFGHIJ"

	for _, b := range []string{srcBucket, dstBucket} {
		if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(b)}); err != nil {
			t.Fatalf("failed to create bucket %s: %v", b, err)
		}
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(srcBucket), Key: aws.String(srcKey)})
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(dstBucket), Key: aws.String(dstKey)})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(srcBucket)})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(dstBucket)})
	})

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(srcBucket), Key: aws.String(srcKey), Body: strings.NewReader(srcContent)}); err != nil {
		t.Fatalf("failed to put source object: %v", err)
	}

	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(dstBucket),
		Key:    aws.String(dstKey),
	})
	if err != nil {
		t.Fatalf("failed to create multipart upload: %v", err)
	}

	uploadID := createResult.UploadId
	copySource := srcBucket + "/" + srcKey

	// Part 1: full source (bytes 0-4, "ABCDE").
	part1, err := client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
		Bucket:          aws.String(dstBucket),
		Key:             aws.String(dstKey),
		UploadId:        uploadID,
		PartNumber:      aws.Int32(1),
		CopySource:      aws.String(copySource),
		CopySourceRange: aws.String("bytes=0-4"),
	})
	if err != nil {
		t.Fatalf("UploadPartCopy part 1: %v", err)
	}

	golden.New(t, golden.WithIgnoreFields("LastModified", "ETag", "ResultMetadata")).Assert(t.Name()+"_part1", part1)

	// Part 2: ranged copy (bytes 5-9, "FGHIJ").
	part2, err := client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
		Bucket:          aws.String(dstBucket),
		Key:             aws.String(dstKey),
		UploadId:        uploadID,
		PartNumber:      aws.Int32(2),
		CopySource:      aws.String(copySource),
		CopySourceRange: aws.String("bytes=5-9"),
	})
	if err != nil {
		t.Fatalf("UploadPartCopy part 2: %v", err)
	}

	// UploadPartCopy with a source range beyond the object's bounds fails.
	// Checked before completing the upload — completion consumes the
	// uploadID, and a subsequent UploadPartCopy against it would fail with
	// NoSuchUpload instead of exercising the range validation.
	_, err = client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
		Bucket:          aws.String(dstBucket),
		Key:             aws.String(dstKey),
		UploadId:        uploadID,
		PartNumber:      aws.Int32(3),
		CopySource:      aws.String(copySource),
		CopySourceRange: aws.String("bytes=20-30"),
	})
	if err == nil {
		t.Fatal("expected error for an out-of-bounds copy range, got nil")
	}

	assertS3ErrorCode(t, err, "InvalidArgument")

	// UploadPartCopy from a nonexistent source key fails with NoSuchKey.
	_, err = client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
		Bucket:     aws.String(dstBucket),
		Key:        aws.String(dstKey),
		UploadId:   uploadID,
		PartNumber: aws.Int32(3),
		CopySource: aws.String(srcBucket + "/missing.txt"),
	})
	if err == nil {
		t.Fatal("expected error for a missing copy source, got nil")
	}

	assertS3ErrorCode(t, err, "NoSuchKey")

	if _, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(dstBucket),
		Key:      aws.String(dstKey),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{PartNumber: aws.Int32(1), ETag: part1.CopyPartResult.ETag},
				{PartNumber: aws.Int32(2), ETag: part2.CopyPartResult.ETag},
			},
		},
	}); err != nil {
		t.Fatalf("failed to complete multipart upload: %v", err)
	}

	getResult, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(dstBucket), Key: aws.String(dstKey)})
	if err != nil {
		t.Fatalf("failed to get assembled object: %v", err)
	}
	defer getResult.Body.Close()

	body, err := io.ReadAll(getResult.Body)
	if err != nil {
		t.Fatalf("failed to read assembled body: %v", err)
	}

	if string(body) != srcContent {
		t.Fatalf("assembled body: got %q, want %q", string(body), srcContent)
	}
}

// TestS3_RestoreObject verifies AWS's RestoreObject status-code semantics:
// the first restore on a key returns 202 Accepted, a subsequent restore on
// the same key extends it and returns 200 OK, and RestoreObject on a
// nonexistent key returns 404 (NoSuchKey). The AWS SDK doesn't surface the
// raw HTTP status for a successful call, so the 202-vs-200 distinction is
// checked with a direct HTTP request.
func TestS3_RestoreObject(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-restore-object"
	key := "restore-me.txt"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: strings.NewReader("x")}); err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	const restoreBody = `<RestoreRequest><Days>3</Days><Tier>Standard</Tier></RestoreRequest>`

	firstResp := postRestoreRequest(t, bucket, key, restoreBody)
	if firstResp.StatusCode != http.StatusAccepted {
		t.Fatalf("first restore: got status %d, want %d", firstResp.StatusCode, http.StatusAccepted)
	}

	secondResp := postRestoreRequest(t, bucket, key, restoreBody)
	if secondResp.StatusCode != http.StatusOK {
		t.Fatalf("second restore: got status %d, want %d", secondResp.StatusCode, http.StatusOK)
	}

	_, err = client.RestoreObject(ctx, &s3.RestoreObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("missing-key.txt"),
		RestoreRequest: &types.RestoreRequest{
			Days: aws.Int32(1),
		},
	})
	if err == nil {
		t.Fatal("expected error restoring a missing key, got nil")
	}

	assertS3ErrorCode(t, err, "NoSuchKey")
}

// TestS3_PutObject_ConditionalWrites verifies If-Match / If-None-Match
// conditional PutObject semantics: If-None-Match:* only succeeds when the
// key is absent (412 PreconditionFailed otherwise), If-Match only succeeds
// when it matches the current ETag (412 on mismatch, 404 NoSuchKey when the
// key doesn't exist yet), and a non-"*" If-None-Match value is rejected
// with 501 NotImplemented (the only form AWS S3 supports on writes).
func TestS3_PutObject_ConditionalWrites(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-put-object-conditional"
	key := "conditional.txt"

	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	// If-None-Match:* succeeds when the key is absent.
	putResp, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader("v1"),
		IfNoneMatch: aws.String("*"),
	})
	if err != nil {
		t.Fatalf("If-None-Match:* on a missing key: %v", err)
	}

	firstETag := aws.ToString(putResp.ETag)
	if firstETag == "" {
		t.Fatal("expected an ETag on a successful conditional PutObject")
	}

	// If-None-Match:* now fails: the key exists.
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader("v2"),
		IfNoneMatch: aws.String("*"),
	})
	if err == nil {
		t.Fatal("expected If-None-Match:* to fail once the key exists")
	}

	assertS3ErrorCode(t, err, "PreconditionFailed")

	// If-Match with a stale ETag fails.
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(key),
		Body:    strings.NewReader("v3"),
		IfMatch: aws.String(`"stale-etag"`),
	})
	if err == nil {
		t.Fatal("expected If-Match with a stale ETag to fail")
	}

	assertS3ErrorCode(t, err, "PreconditionFailed")

	// If-Match with the current ETag succeeds and rotates the ETag.
	putResp2, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(key),
		Body:    strings.NewReader("v3"),
		IfMatch: aws.String(firstETag),
	})
	if err != nil {
		t.Fatalf("If-Match with the current ETag: %v", err)
	}

	if aws.ToString(putResp2.ETag) == firstETag {
		t.Fatal("expected a new ETag after a successful conditional PutObject")
	}

	// If-Match on a key that doesn't exist yet is NoSuchKey, not
	// PreconditionFailed.
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String("does-not-exist.txt"),
		Body:    strings.NewReader("x"),
		IfMatch: aws.String(`"whatever"`),
	})
	if err == nil {
		t.Fatal("expected If-Match on a missing key to fail")
	}

	assertS3ErrorCode(t, err, "NoSuchKey")

	// If-None-Match with a value other than "*" is not implemented.
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader("x"),
		IfNoneMatch: aws.String(`"some-etag"`),
	})
	if err == nil {
		t.Fatal("expected a non-* If-None-Match value to fail")
	}

	assertS3ErrorCode(t, err, "NotImplemented")
}

// postRestoreRequest issues a raw POST /{bucket}/{key}?restore request.
func postRestoreRequest(t *testing.T, bucket, key, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		testEndpoint()+"/"+bucket+"/"+key+"?restore", strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to build restore request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to POST restore request: %v", err)
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return resp
}
