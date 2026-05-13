//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sivchari/golden"
)

func newS3Client(t *testing.T) *s3.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
		o.UsePathStyle = true
	})
}

func TestS3_CreateAndDeleteBucket(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-create-delete-bucket"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	// Head bucket to verify it exists
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to head bucket: %v", err)
	}

	// Delete bucket
	_, err = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to delete bucket: %v", err)
	}
}

func TestS3_ListBuckets(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()

	// Create a bucket first
	bucketName := "test-list-buckets"
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// List buckets
	result, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		t.Fatalf("failed to list buckets: %v", err)
	}

	found := false
	for _, b := range result.Buckets {
		if *b.Name == bucketName {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("bucket %s not found in list", bucketName)
	}
}

func TestS3_PutAndGetObject(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-put-get-object"
	key := "test-key.txt"
	content := "Hello, awsim!"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Put object
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(content)),
	})
	if err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// Get object
	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to get object: %v", err)
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(body) != content {
		t.Errorf("expected content %q, got %q", content, string(body))
	}
}

func TestS3_HeadObject(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-head-object"
	key := "test-key.txt"
	content := "Hello, awsim!"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Put object
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(content)),
	})
	if err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// Head object
	result, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ContentLength", "ETag", "LastModified", "ResultMetadata")).Assert(t.Name(), result)
}

func TestS3_DeleteObject(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-delete-object"
	key := "test-key.txt"
	content := "Hello, awsim!"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Put object
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(content)),
	})
	if err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// Delete object
	_, err = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to delete object: %v", err)
	}

	// Verify object is deleted
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Error("expected error when getting deleted object, got nil")
	}
}

func TestS3_ListObjects(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-list-objects"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		// Clean up objects
		for _, key := range []string{"file1.txt", "file2.txt", "dir/file3.txt"} {
			_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(key),
			})
		}
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Put multiple objects
	keys := []string{"file1.txt", "file2.txt", "dir/file3.txt"}
	for _, key := range keys {
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
			Body:   bytes.NewReader([]byte("content")),
		})
		if err != nil {
			t.Fatalf("failed to put object %s: %v", key, err)
		}
	}

	// List all objects
	result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ETag", "LastModified", "ResultMetadata")).Assert(t.Name()+"_all", result)

	// List with prefix
	result, err = client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String("dir/"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ETag", "LastModified", "ResultMetadata")).Assert(t.Name()+"_prefix", result)
}

func newS3PresignClient(t *testing.T) *s3.PresignClient {
	t.Helper()

	return s3.NewPresignClient(newS3Client(t))
}

func TestS3_PresignedURL_GetObject(t *testing.T) {
	client := newS3Client(t)
	presignClient := newS3PresignClient(t)
	ctx := t.Context()
	bucketName := "test-presigned-get-object"
	key := "test-presigned-key.txt"
	content := "Hello, presigned URL!"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Put object using regular client
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(content)),
	})
	if err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// Generate presigned URL for GetObject
	presignedReq, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 15 * time.Minute
	})
	if err != nil {
		t.Fatalf("failed to presign GetObject: %v", err)
	}

	// Use presigned URL to get the object
	resp, err := http.Get(presignedReq.URL)
	if err != nil {
		t.Fatalf("failed to GET presigned URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if string(body) != content {
		t.Errorf("expected content %q, got %q", content, string(body))
	}
}

func TestS3_PresignedURL_PutObject(t *testing.T) {
	client := newS3Client(t)
	presignClient := newS3PresignClient(t)
	ctx := t.Context()
	bucketName := "test-presigned-put-object"
	key := "test-presigned-put-key.txt"
	content := "Hello, presigned PUT!"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Generate presigned URL for PutObject
	presignedReq, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 15 * time.Minute
	})
	if err != nil {
		t.Fatalf("failed to presign PutObject: %v", err)
	}

	// Use presigned URL to put the object
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedReq.URL, bytes.NewReader([]byte(content)))
	if err != nil {
		t.Fatalf("failed to create PUT request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to PUT presigned URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify object was uploaded using regular client
	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to get object: %v", err)
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(body) != content {
		t.Errorf("expected content %q, got %q", content, string(body))
	}
}

func TestS3_PresignedURL_Expired(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-presigned-expired"
	key := "test-expired-key.txt"
	content := "Hello, expired!"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Put object using regular client
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(content)),
	})
	if err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// Generate presigned URL with 1 second expiration
	presignClient := newS3PresignClient(t)
	presignedReq, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 1 * time.Second
	})
	if err != nil {
		t.Fatalf("failed to presign GetObject: %v", err)
	}

	// Wait for URL to expire
	time.Sleep(2 * time.Second)

	// Use expired presigned URL
	resp, err := http.Get(presignedReq.URL)
	if err != nil {
		t.Fatalf("failed to GET expired presigned URL: %v", err)
	}
	defer resp.Body.Close()

	// Should return 403 Forbidden for expired URL
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 for expired URL, got %d", resp.StatusCode)
	}
}

// Multipart Upload Tests

func TestS3_MultipartUpload_BasicFlow(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-multipart-basic"
	key := "large-file.bin"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Create multipart upload
	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("UploadId", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	uploadID := createResult.UploadId

	// Upload parts
	part1Content := strings.Repeat("A", 5*1024*1024) // 5MB minimum part size
	part2Content := strings.Repeat("B", 5*1024*1024)

	part1Result, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucketName),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       strings.NewReader(part1Content),
	})
	if err != nil {
		t.Fatalf("failed to upload part 1: %v", err)
	}

	part2Result, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucketName),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(2),
		Body:       strings.NewReader(part2Content),
	})
	if err != nil {
		t.Fatalf("failed to upload part 2: %v", err)
	}

	// Complete multipart upload
	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{PartNumber: aws.Int32(1), ETag: part1Result.ETag},
				{PartNumber: aws.Int32(2), ETag: part2Result.ETag},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to complete multipart upload: %v", err)
	}

	// Verify the object was created
	getResult, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to get object: %v", err)
	}
	defer getResult.Body.Close()

	body, err := io.ReadAll(getResult.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	expectedContent := part1Content + part2Content
	if string(body) != expectedContent {
		t.Errorf("content mismatch: expected length %d, got %d", len(expectedContent), len(body))
	}
}

func TestS3_MultipartUpload_AbortUpload(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-multipart-abort"
	key := "aborted-file.bin"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Create multipart upload
	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("UploadId", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	uploadID := createResult.UploadId

	// Upload a part
	_, err = client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucketName),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       strings.NewReader("test data"),
	})
	if err != nil {
		t.Fatalf("failed to upload part: %v", err)
	}

	// Abort the upload
	_, err = client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatalf("failed to abort multipart upload: %v", err)
	}

	// Verify the upload is aborted by trying to list parts
	_, err = client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err == nil {
		t.Error("expected error when listing parts of aborted upload")
	}
}

// Versioning Tests

func TestS3_Versioning_PutAndGetBucketVersioning(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-versioning-config"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Get versioning status (should be empty initially)
	result, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_initial", result)

	// Enable versioning
	_, err = client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucketName),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify versioning is enabled
	result, err = client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_enabled", result)

	// Suspend versioning
	_, err = client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucketName),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusSuspended,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify versioning is suspended
	result, err = client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_suspended", result)
}

func TestS3_Versioning_PutObjectWithVersioning(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-versioning-put-object"
	key := "test-versioned-key.txt"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		// List and delete all versions
		versions, _ := client.ListObjectVersions(context.Background(), &s3.ListObjectVersionsInput{
			Bucket: aws.String(bucketName),
		})
		if versions != nil {
			for _, v := range versions.Versions {
				_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
					Bucket:    aws.String(bucketName),
					Key:       v.Key,
					VersionId: v.VersionId,
				})
			}
			for _, dm := range versions.DeleteMarkers {
				_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
					Bucket:    aws.String(bucketName),
					Key:       dm.Key,
					VersionId: dm.VersionId,
				})
			}
		}
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Enable versioning
	_, err = client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucketName),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		t.Fatalf("failed to enable versioning: %v", err)
	}

	// Put first version
	putResult1, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte("version 1")),
	})
	if err != nil {
		t.Fatalf("failed to put object v1: %v", err)
	}

	if putResult1.VersionId == nil || *putResult1.VersionId == "" {
		t.Error("expected version ID for first put, got empty")
	}

	// Put second version
	putResult2, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte("version 2")),
	})
	if err != nil {
		t.Fatalf("failed to put object v2: %v", err)
	}

	if putResult2.VersionId == nil || *putResult2.VersionId == "" {
		t.Error("expected version ID for second put, got empty")
	}

	if *putResult1.VersionId == *putResult2.VersionId {
		t.Error("expected different version IDs for different puts")
	}

	// Get latest version (should be v2)
	getResult, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to get object: %v", err)
	}
	defer getResult.Body.Close()

	body, _ := io.ReadAll(getResult.Body)
	if string(body) != "version 2" {
		t.Errorf("expected 'version 2', got %q", string(body))
	}

	// Get first version by version ID
	getResult1, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:    aws.String(bucketName),
		Key:       aws.String(key),
		VersionId: putResult1.VersionId,
	})
	if err != nil {
		t.Fatalf("failed to get object v1 by version ID: %v", err)
	}
	defer getResult1.Body.Close()

	body1, _ := io.ReadAll(getResult1.Body)
	if string(body1) != "version 1" {
		t.Errorf("expected 'version 1', got %q", string(body1))
	}
}

func TestS3_Versioning_DeleteObjectCreatesDeleteMarker(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-versioning-delete-marker"
	key := "test-delete-marker-key.txt"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		// List and delete all versions
		versions, _ := client.ListObjectVersions(context.Background(), &s3.ListObjectVersionsInput{
			Bucket: aws.String(bucketName),
		})
		if versions != nil {
			for _, v := range versions.Versions {
				_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
					Bucket:    aws.String(bucketName),
					Key:       v.Key,
					VersionId: v.VersionId,
				})
			}
			for _, dm := range versions.DeleteMarkers {
				_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
					Bucket:    aws.String(bucketName),
					Key:       dm.Key,
					VersionId: dm.VersionId,
				})
			}
		}
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Enable versioning
	_, err = client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucketName),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		t.Fatalf("failed to enable versioning: %v", err)
	}

	// Put object
	putResult, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte("content")),
	})
	if err != nil {
		t.Fatalf("failed to put object: %v", err)
	}

	// Delete object (should create delete marker)
	deleteResult, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to delete object: %v", err)
	}

	if deleteResult.DeleteMarker == nil || !*deleteResult.DeleteMarker {
		t.Error("expected delete marker to be true")
	}

	if deleteResult.VersionId == nil || *deleteResult.VersionId == "" {
		t.Error("expected version ID for delete marker")
	}

	// Try to get object (should fail with 404)
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Error("expected error when getting deleted object, got nil")
	}

	// Get object by original version ID (should succeed)
	getResult, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:    aws.String(bucketName),
		Key:       aws.String(key),
		VersionId: putResult.VersionId,
	})
	if err != nil {
		t.Fatalf("failed to get object by version ID: %v", err)
	}
	defer getResult.Body.Close()

	body, _ := io.ReadAll(getResult.Body)
	if string(body) != "content" {
		t.Errorf("expected 'content', got %q", string(body))
	}
}

func TestS3_MultipartUpload_ListMultipartUploads(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-list-multipart"
	key1 := "file1.bin"
	key2 := "file2.bin"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		// Abort any in-progress uploads
		uploads, _ := client.ListMultipartUploads(context.Background(), &s3.ListMultipartUploadsInput{
			Bucket: aws.String(bucketName),
		})
		if uploads != nil {
			for _, u := range uploads.Uploads {
				_, _ = client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
					Bucket:   aws.String(bucketName),
					Key:      u.Key,
					UploadId: u.UploadId,
				})
			}
		}
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Create two multipart uploads
	upload1, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key1),
	})
	if err != nil {
		t.Fatal(err)
	}

	upload2, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key2),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List multipart uploads
	listResult, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("UploadId", "Initiated", "ResultMetadata")).Assert(t.Name(), listResult)

	// Cleanup
	_, _ = client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key1),
		UploadId: upload1.UploadId,
	})
	_, _ = client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key2),
		UploadId: upload2.UploadId,
	})
}

func TestS3_MultipartUpload_ListParts(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-list-parts"
	key := "multipart-file.bin"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Create multipart upload
	createResult, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("UploadId", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	uploadID := createResult.UploadId

	// Upload two parts
	_, err = client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucketName),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(1),
		Body:       strings.NewReader("part 1 content"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucketName),
		Key:        aws.String(key),
		UploadId:   uploadID,
		PartNumber: aws.Int32(2),
		Body:       strings.NewReader("part 2 content"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List parts
	listResult, err := client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("UploadId", "ETag", "LastModified", "ResultMetadata")).Assert(t.Name()+"_list", listResult)

	// Cleanup
	_, _ = client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key),
		UploadId: uploadID,
	})
}

func TestS3_Versioning_ListObjectVersions(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-versioning-list-versions"
	key := "test-list-versions-key.txt"

	// Create bucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		// List and delete all versions
		versions, _ := client.ListObjectVersions(context.Background(), &s3.ListObjectVersionsInput{
			Bucket: aws.String(bucketName),
		})
		if versions != nil {
			for _, v := range versions.Versions {
				_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
					Bucket:    aws.String(bucketName),
					Key:       v.Key,
					VersionId: v.VersionId,
				})
			}
			for _, dm := range versions.DeleteMarkers {
				_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
					Bucket:    aws.String(bucketName),
					Key:       dm.Key,
					VersionId: dm.VersionId,
				})
			}
		}
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Enable versioning
	_, err = client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(bucketName),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		t.Fatalf("failed to enable versioning: %v", err)
	}

	// Put three versions
	for i := 1; i <= 3; i++ {
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
			Body:   bytes.NewReader([]byte("version " + string(rune('0'+i)))),
		})
		if err != nil {
			t.Fatalf("failed to put object v%d: %v", i, err)
		}
	}

	// Delete object (creates delete marker)
	_, err = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("failed to delete object: %v", err)
	}

	// List object versions
	listResult, err := client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatalf("failed to list object versions: %v", err)
	}

	// Should have 3 versions
	if len(listResult.Versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(listResult.Versions))
	}

	// Should have 1 delete marker
	if len(listResult.DeleteMarkers) != 1 {
		t.Errorf("expected 1 delete marker, got %d", len(listResult.DeleteMarkers))
	}

	// Check that only one version (the latest before delete) or delete marker is marked as latest
	latestCount := 0
	for _, v := range listResult.Versions {
		if v.IsLatest != nil && *v.IsLatest {
			latestCount++
		}
	}
	for _, dm := range listResult.DeleteMarkers {
		if dm.IsLatest != nil && *dm.IsLatest {
			latestCount++
		}
	}

	if latestCount != 1 {
		t.Errorf("expected exactly 1 latest version/marker, got %d", latestCount)
	}
}

func TestS3_DeleteObjects(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-delete-objects-bucket"

	// Create bucket.
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucketName),
		})
	})

	// Put multiple objects.
	keys := []string{"obj1.txt", "obj2.txt", "obj3.txt"}
	for _, key := range keys {
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
			Body:   strings.NewReader("content-" + key),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Delete multiple objects.
	deleteOutput, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &types.Delete{
			Objects: []types.ObjectIdentifier{
				{Key: aws.String("obj1.txt")},
				{Key: aws.String("obj2.txt")},
				{Key: aws.String("obj3.txt")},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("VersionId", "ResultMetadata")).Assert(t.Name(), deleteOutput)

	// Verify objects are deleted.
	listOutput, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(listOutput.Contents) != 0 {
		t.Errorf("expected 0 objects after delete, got %d", len(listOutput.Contents))
	}
}

func TestS3_CopyObject(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-copy-object-bucket"

	// Create bucket.
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put source object.
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("source.txt"),
		Body:   bytes.NewReader([]byte("copy me")),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Copy object within same bucket.
	copyOutput, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucketName),
		Key:        aws.String("dest.txt"),
		CopySource: aws.String(bucketName + "/source.txt"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ETag", "LastModified", "ResultMetadata",
		"ServerSideEncryption", "CopySourceVersionId",
	)).Assert(t.Name(), copyOutput)
}

// TestS3_CopyObject_URLEncodedSource verifies that the URL-encoded
// form of x-amz-copy-source is accepted, matching AWS S3 behavior.
// Some AWS Go SDK callers escape the entire "bucket/key" string with
// url.PathEscape to be safe with keys containing '/', '+', or spaces.
func TestS3_CopyObject_URLEncodedSource(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-copy-encoded-source-bucket"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("dir/source.txt"),
		Body:   bytes.NewReader([]byte("encoded copy")),
	})
	if err != nil {
		t.Fatal(err)
	}

	// url.PathEscape encodes '/' as %2F so the entire path becomes
	// percent-encoded — this is the form that previously failed.
	encoded := url.PathEscape(bucketName + "/dir/source.txt")

	_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucketName),
		Key:        aws.String("dir/dest.txt"),
		CopySource: aws.String(encoded),
	})
	if err != nil {
		t.Fatalf("CopyObject with URL-encoded source should succeed: %v", err)
	}

	got, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String("dir/dest.txt"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer got.Body.Close()

	body, err := io.ReadAll(got.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != "encoded copy" {
		t.Errorf("unexpected body: got=%q want=%q", string(body), "encoded copy")
	}
}

func TestS3_PutAndGetObjectTagging(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-tagging-bucket"
	key := "test-object"

	// Create bucket.
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
	})

	// Put object.
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader("original-body"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put tags.
	_, err = client.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{Key: aws.String("env"), Value: aws.String("test")},
				{Key: aws.String("team"), Value: aws.String("platform")},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get tags and verify sorted order via golden test.
	tagOutput, err := client.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), tagOutput.TagSet)

	// Verify body not overwritten.
	getOutput, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(getOutput.Body)
	if string(body) != "original-body" {
		t.Errorf("body = %q, want %q", string(body), "original-body")
	}
}

func TestS3_PutObject_KeyWithLeadingSlash(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-leading-slash-bucket"
	key := "/path/to/object.txt"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
	})

	// PutObject with a key that starts with "/" produces a double slash in
	// path-style URLs (e.g., /bucket//path/to/object.txt). Go's ServeMux
	// would return 301 for this without the path cleaning fix.
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader("hello"),
	})
	if err != nil {
		t.Fatalf("PutObject with leading slash key failed: %v", err)
	}

	getOutput, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject with leading slash key failed: %v", err)
	}

	gotBody, _ := io.ReadAll(getOutput.Body)
	if string(gotBody) != "hello" {
		t.Errorf("body = %q, want %q", string(gotBody), "hello")
	}
}

func TestS3_PublicAccessBlock(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-pab-bucket"

	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	if _, err := client.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{
		Bucket: aws.String(bucket),
		PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(true),
			IgnorePublicAcls:      aws.Bool(true),
			BlockPublicPolicy:     aws.Bool(true),
			RestrictPublicBuckets: aws.Bool(true),
		},
	}); err != nil {
		t.Fatalf("failed to put public access block: %v", err)
	}

	getResult, err := client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("failed to get public access block: %v", err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get", getResult)

	if _, err := client.DeletePublicAccessBlock(ctx, &s3.DeletePublicAccessBlockInput{
		Bucket: aws.String(bucket),
	}); err != nil {
		t.Fatalf("failed to delete public access block: %v", err)
	}

	if _, err := client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
		Bucket: aws.String(bucket),
	}); err == nil {
		t.Fatal("expected NoSuchPublicAccessBlockConfiguration after delete, got nil")
	}
}

func TestS3_BucketEncryption(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-sse-bucket"

	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	if _, err := client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
		Bucket: aws.String(bucket),
		ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{
				{
					ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
						SSEAlgorithm: types.ServerSideEncryptionAes256,
					},
					BucketKeyEnabled: aws.Bool(true),
				},
			},
		},
	}); err != nil {
		t.Fatalf("failed to put bucket encryption: %v", err)
	}

	getResult, err := client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("failed to get bucket encryption: %v", err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get", getResult)

	if _, err := client.DeleteBucketEncryption(ctx, &s3.DeleteBucketEncryptionInput{
		Bucket: aws.String(bucket),
	}); err != nil {
		t.Fatalf("failed to delete bucket encryption: %v", err)
	}

	if _, err := client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
		Bucket: aws.String(bucket),
	}); err == nil {
		t.Fatal("expected ServerSideEncryptionConfigurationNotFoundError after delete, got nil")
	}
}

func TestS3_BucketSubresourceStubs_DefaultResponses(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-subresource-defaults"

	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	if _, err := client.GetBucketAcl(ctx, &s3.GetBucketAclInput{Bucket: aws.String(bucket)}); err != nil {
		t.Errorf("GetBucketAcl: unexpected error: %v", err)
	}

	loc, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Errorf("GetBucketLocation: unexpected error: %v", err)
	} else if string(loc.LocationConstraint) != "us-east-1" {
		t.Errorf("LocationConstraint = %q, want %q", loc.LocationConstraint, "us-east-1")
	}

	if _, err := client.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucket)}); err != nil {
		t.Errorf("GetBucketLogging: unexpected error: %v", err)
	}

	if _, err := client.GetBucketAccelerateConfiguration(ctx, &s3.GetBucketAccelerateConfigurationInput{Bucket: aws.String(bucket)}); err != nil {
		t.Errorf("GetBucketAccelerateConfiguration: unexpected error: %v", err)
	}

	pay, err := client.GetBucketRequestPayment(ctx, &s3.GetBucketRequestPaymentInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Errorf("GetBucketRequestPayment: unexpected error: %v", err)
	} else if string(pay.Payer) != "BucketOwner" {
		t.Errorf("Payer = %q, want %q", pay.Payer, "BucketOwner")
	}
}

func TestS3_BucketSubresourceStubs_NoSuchErrors(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-subresource-nosuch"

	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	cases := []struct {
		name string
		call func() error
	}{
		{"GetBucketPolicy", func() error {
			_, err := client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})

			return err
		}},
		{"GetBucketCors", func() error {
			_, err := client.GetBucketCors(ctx, &s3.GetBucketCorsInput{Bucket: aws.String(bucket)})

			return err
		}},
		{"GetBucketLifecycleConfiguration", func() error {
			_, err := client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucket)})

			return err
		}},
		{"GetBucketWebsite", func() error {
			_, err := client.GetBucketWebsite(ctx, &s3.GetBucketWebsiteInput{Bucket: aws.String(bucket)})

			return err
		}},
		{"GetBucketReplication", func() error {
			_, err := client.GetBucketReplication(ctx, &s3.GetBucketReplicationInput{Bucket: aws.String(bucket)})

			return err
		}},
		{"GetBucketTagging", func() error {
			_, err := client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: aws.String(bucket)})

			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatalf("%s: expected NoSuch* error, got nil", tc.name)
			}
		})
	}
}

func TestS3_BucketCors(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucket := "test-bucket-cors"

	// Create bucket.
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})
	})

	// GetBucketCors before setting should return error.
	_, err = client.GetBucketCors(ctx, &s3.GetBucketCorsInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		t.Fatal("expected NoSuchCORSConfiguration error")
	}

	// PutBucketCors.
	_, err = client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(bucket),
		CORSConfiguration: &types.CORSConfiguration{
			CORSRules: []types.CORSRule{
				{
					AllowedHeaders: []string{"*"},
					AllowedMethods: []string{"GET", "PUT"},
					AllowedOrigins: []string{"https://example.com"},
					ExposeHeaders:  []string{"ETag"},
					MaxAgeSeconds:  aws.Int32(300),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// GetBucketCors after setting should return the CORS rules.
	getOutput, err := client.GetBucketCors(ctx, &s3.GetBucketCorsInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), getOutput)
}
