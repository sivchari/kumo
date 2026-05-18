package s3

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestCompleteMultipartUploadRejectsInvalidPartOrder(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()
	parts := createMultipartOrderTestParts(t, store)

	_, err := store.CompleteMultipartUpload(ctx, "mpu-order-test", "object", parts.uploadID, []PartRequest{
		{PartNumber: 2, ETag: parts.part2.ETag},
		{PartNumber: 1, ETag: parts.part1.ETag},
	})
	expectMultipartCode(t, err, "InvalidPartOrder")
}

func TestCompleteMultipartUploadRejectsDuplicatePartNumbers(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()
	parts := createMultipartOrderTestParts(t, store)

	_, err := store.CompleteMultipartUpload(ctx, "mpu-order-test", "object", parts.uploadID, []PartRequest{
		{PartNumber: 1, ETag: parts.part1.ETag},
		{PartNumber: 1, ETag: parts.part1.ETag},
	})
	expectMultipartCode(t, err, "InvalidPartOrder")
}

type multipartOrderTestParts struct {
	uploadID string
	part1    *Part
	part2    *Part
}

func createMultipartOrderTestParts(t *testing.T, store *MemoryStorage) multipartOrderTestParts {
	t.Helper()

	ctx := context.Background()
	if err := store.CreateBucket(ctx, "mpu-order-test"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	upload, err := store.CreateMultipartUpload(ctx, "mpu-order-test", "object")
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}

	part1, err := store.UploadPart(ctx, "mpu-order-test", "object", upload.UploadID, 1, bytes.NewReader([]byte("hello ")))
	if err != nil {
		t.Fatalf("UploadPart 1: %v", err)
	}

	part2, err := store.UploadPart(ctx, "mpu-order-test", "object", upload.UploadID, 2, bytes.NewReader([]byte("world")))
	if err != nil {
		t.Fatalf("UploadPart 2: %v", err)
	}

	return multipartOrderTestParts{uploadID: upload.UploadID, part1: part1, part2: part2}
}

func expectMultipartCode(t *testing.T, err error, code string) {
	t.Helper()

	var multipartErr *MultipartError
	if !errors.As(err, &multipartErr) {
		t.Fatalf("got err %v, want MultipartError code %s", err, code)
	}

	if multipartErr.Code != code {
		t.Fatalf("got MultipartError code %s, want %s", multipartErr.Code, code)
	}
}
