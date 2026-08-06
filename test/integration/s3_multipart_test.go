//go:build integration

package integration

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestS3_CompleteMultipartUpload_RejectsEmptyPartList verifies that
// completing a multipart upload with no parts fails with MalformedXML, and
// that the upload remains open (and completable with real parts)
// afterwards.
func TestS3_CompleteMultipartUpload_RejectsEmptyPartList(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-mpu-empty-parts"
	key := "object"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to create multipart upload: %v", err)
	}

	uploadID := createResult.UploadId

	partResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       strings.NewReader("hello"),
	})
	if err != nil {
		t.Fatalf("failed to upload part: %v", err)
	}

	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{},
	})
	if err == nil {
		t.Fatal("expected empty part list to be rejected, got nil error")
	}

	assertS3ErrorCode(t, err, "MalformedXML")

	// The rejected completion must not have consumed the upload — a real
	// completion with the uploaded part still succeeds.
	if _, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{PartNumber: aws.Int32(1), ETag: partResult.ETag},
			},
		},
	}); err != nil {
		t.Fatalf("multipart upload should remain available after rejected empty completion: %v", err)
	}

	getResult, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("failed to get completed object: %v", err)
	}
	defer getResult.Body.Close()

	body, err := io.ReadAll(getResult.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(body) != "hello" {
		t.Fatalf("body = %q, want %q", string(body), "hello")
	}
}

// TestS3_CompleteMultipartUpload_CreatesVersionWhenBucketVersioningEnabled
// verifies that completing a multipart upload on a versioning-enabled
// bucket creates a new object version, and that both the versioned and
// current copies are retrievable.
func TestS3_CompleteMultipartUpload_CreatesVersionWhenBucketVersioningEnabled(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-mpu-versioning-enabled"
	key := "object"
	const body = "hello"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		cleanupBucketVersions(t, client, bucket)
	})

	if _, err := client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucket),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	}); err != nil {
		t.Fatalf("failed to enable versioning: %v", err)
	}

	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to create multipart upload: %v", err)
	}

	partResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   createResult.UploadId,
		PartNumber: aws.Int32(1),
		Body:       strings.NewReader(body),
	})
	if err != nil {
		t.Fatalf("failed to upload part: %v", err)
	}

	completeResult, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: createResult.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{PartNumber: aws.Int32(1), ETag: partResult.ETag},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to complete multipart upload: %v", err)
	}

	if aws.ToString(completeResult.VersionId) == "" {
		t.Fatal("expected CompleteMultipartUpload to report a version ID, got empty")
	}

	versionedResult, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:    aws.String(bucket),
		Key:       aws.String(key),
		VersionId: completeResult.VersionId,
	})
	if err != nil {
		t.Fatalf("failed to get object by version ID: %v", err)
	}
	defer versionedResult.Body.Close()

	versionedBody, _ := io.ReadAll(versionedResult.Body)
	if string(versionedBody) != body {
		t.Fatalf("versioned body: got %q, want %q", versionedBody, body)
	}

	currentResult, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("failed to get current object: %v", err)
	}
	defer currentResult.Body.Close()

	if aws.ToString(currentResult.VersionId) != aws.ToString(completeResult.VersionId) {
		t.Fatalf("current version ID: got %q, want %q", aws.ToString(currentResult.VersionId), aws.ToString(completeResult.VersionId))
	}
}

// TestS3_CompleteMultipartUpload_UsesNullVersionWhenVersioningSuspended
// verifies that completing multipart uploads on a versioning-suspended
// bucket reuses the "null" version ID, overwriting the prior null version
// rather than accumulating one per completion.
func TestS3_CompleteMultipartUpload_UsesNullVersionWhenVersioningSuspended(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-mpu-versioning-suspended"
	key := "object"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		cleanupBucketVersions(t, client, bucket)
	})

	if _, err := client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucket),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusSuspended,
		},
	}); err != nil {
		t.Fatalf("failed to suspend versioning: %v", err)
	}

	first := completeSinglePartMultipartUpload(t, client, bucket, key, "first")
	if aws.ToString(first.VersionId) != "null" {
		t.Fatalf("first version ID: got %q, want %q", aws.ToString(first.VersionId), "null")
	}

	second := completeSinglePartMultipartUpload(t, client, bucket, key, "second")
	if aws.ToString(second.VersionId) != "null" {
		t.Fatalf("second version ID: got %q, want %q", aws.ToString(second.VersionId), "null")
	}

	versions, err := client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("failed to list object versions: %v", err)
	}

	if len(versions.Versions) != 1 {
		t.Fatalf("versions: got %d, want 1 null version", len(versions.Versions))
	}

	nullResult, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), VersionId: aws.String("null")})
	if err != nil {
		t.Fatalf("failed to get null version: %v", err)
	}
	defer nullResult.Body.Close()

	nullBody, _ := io.ReadAll(nullResult.Body)
	if string(nullBody) != "second" {
		t.Fatalf("surviving null version body: got %q, want %q", nullBody, "second")
	}
}

// completeSinglePartMultipartUpload drives a full single-part multipart
// upload and returns the CompleteMultipartUpload output.
func completeSinglePartMultipartUpload(t *testing.T, client *s3.Client, bucket, key, body string) *s3.CompleteMultipartUploadOutput {
	t.Helper()

	ctx := context.Background()

	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to create multipart upload: %v", err)
	}

	partResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   createResult.UploadId,
		PartNumber: aws.Int32(1),
		Body:       strings.NewReader(body),
	})
	if err != nil {
		t.Fatalf("failed to upload part: %v", err)
	}

	completeResult, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: createResult.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{PartNumber: aws.Int32(1), ETag: partResult.ETag},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to complete multipart upload: %v", err)
	}

	return completeResult
}

// cleanupBucketVersions deletes every version and delete marker in a
// bucket, then the bucket itself.
func cleanupBucketVersions(t *testing.T, client *s3.Client, bucket string) {
	t.Helper()

	ctx := context.Background()

	versions, _ := client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{Bucket: aws.String(bucket)})
	if versions != nil {
		for _, v := range versions.Versions {
			_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: v.Key, VersionId: v.VersionId})
		}

		for _, dm := range versions.DeleteMarkers {
			_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: dm.Key, VersionId: dm.VersionId})
		}
	}

	_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
}

// TestS3_CreateMultipartUpload_CarriesContentTypeMetadataAndSSE drives the
// full CreateMultipartUpload -> UploadPart -> CompleteMultipartUpload cycle
// with Content-Type, metadata, and SSE-KMS set on the initiate request, and
// verifies the completed object carries all three groups.
func TestS3_CreateMultipartUpload_CarriesContentTypeMetadataAndSSE(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-mpu-metadata"
	key := "object"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:               aws.String(bucket),
		Key:                  aws.String(key),
		ContentType:          aws.String("image/jpeg"),
		Metadata:             map[string]string{"origin": "cam1"},
		ServerSideEncryption: types.ServerSideEncryptionAwsKms,
		SSEKMSKeyId:          aws.String("mpu-key"),
	})
	if err != nil {
		t.Fatalf("failed to create multipart upload: %v", err)
	}

	partResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   createResult.UploadId,
		PartNumber: aws.Int32(1),
		Body:       strings.NewReader("hello multipart"),
	})
	if err != nil {
		t.Fatalf("failed to upload part: %v", err)
	}

	if _, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: createResult.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{PartNumber: aws.Int32(1), ETag: partResult.ETag},
			},
		},
	}); err != nil {
		t.Fatalf("failed to complete multipart upload: %v", err)
	}

	headResult, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("failed to head object: %v", err)
	}

	if aws.ToString(headResult.ContentType) != "image/jpeg" {
		t.Errorf("ContentType: got %q, want image/jpeg", aws.ToString(headResult.ContentType))
	}

	if got := headResult.Metadata["origin"]; got != "cam1" {
		t.Errorf("Metadata[origin]: got %q, want cam1", got)
	}

	if headResult.ServerSideEncryption != types.ServerSideEncryptionAwsKms {
		t.Errorf("ServerSideEncryption: got %q, want %s", headResult.ServerSideEncryption, types.ServerSideEncryptionAwsKms)
	}

	if aws.ToString(headResult.SSEKMSKeyId) != "mpu-key" {
		t.Errorf("SSEKMSKeyId: got %q, want mpu-key", aws.ToString(headResult.SSEKMSKeyId))
	}
}

// TestS3_CreateMultipartUpload_FallsBackToDestinationBucketDefaultEncryption
// verifies that, absent SSE headers on the initiate request, a multipart
// upload picks up the destination bucket's default encryption — matching
// AWS's CreateMultipartUpload semantics, and PutObject's (see
// TestS3_PutObjectFallsBackToBucketDefaultEncryption).
func TestS3_CreateMultipartUpload_FallsBackToDestinationBucketDefaultEncryption(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-mpu-default-encryption"
	key := "object"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	if _, err := client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
		Bucket: aws.String(bucket),
		ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{
				{
					ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
						SSEAlgorithm:   types.ServerSideEncryptionAwsKms,
						KMSMasterKeyID: aws.String("bucket-default-key"),
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("failed to put bucket encryption: %v", err)
	}

	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to create multipart upload: %v", err)
	}

	partResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   createResult.UploadId,
		PartNumber: aws.Int32(1),
		Body:       strings.NewReader("hello multipart"),
	})
	if err != nil {
		t.Fatalf("failed to upload part: %v", err)
	}

	if _, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: createResult.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{PartNumber: aws.Int32(1), ETag: partResult.ETag},
			},
		},
	}); err != nil {
		t.Fatalf("failed to complete multipart upload: %v", err)
	}

	headResult, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("failed to head object: %v", err)
	}

	if headResult.ServerSideEncryption != types.ServerSideEncryptionAwsKms {
		t.Errorf("ServerSideEncryption: got %q, want %s", headResult.ServerSideEncryption, types.ServerSideEncryptionAwsKms)
	}

	if aws.ToString(headResult.SSEKMSKeyId) != "bucket-default-key" {
		t.Errorf("SSEKMSKeyId: got %q, want bucket-default-key", aws.ToString(headResult.SSEKMSKeyId))
	}
}

// TestS3_CreateMultipartUpload_NoHeadersFallsBackToOctetStream is a
// regression guard: when the initiate request carries no Content-Type, the
// completed object still defaults to application/octet-stream with no
// metadata or SSE.
func TestS3_CreateMultipartUpload_NoHeadersFallsBackToOctetStream(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-mpu-no-headers"
	key := "object"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to create multipart upload: %v", err)
	}

	partResult, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   createResult.UploadId,
		PartNumber: aws.Int32(1),
		Body:       strings.NewReader("plain body"),
	})
	if err != nil {
		t.Fatalf("failed to upload part: %v", err)
	}

	if _, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: createResult.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{PartNumber: aws.Int32(1), ETag: partResult.ETag},
			},
		},
	}); err != nil {
		t.Fatalf("failed to complete multipart upload: %v", err)
	}

	headResult, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("failed to head object: %v", err)
	}

	if aws.ToString(headResult.ContentType) != "application/octet-stream" {
		t.Errorf("ContentType: got %q, want application/octet-stream", aws.ToString(headResult.ContentType))
	}

	if len(headResult.Metadata) != 0 {
		t.Errorf("Metadata: got %v, want empty", headResult.Metadata)
	}

	if headResult.ServerSideEncryption != "" {
		t.Errorf("ServerSideEncryption: got %q, want empty", headResult.ServerSideEncryption)
	}
}
