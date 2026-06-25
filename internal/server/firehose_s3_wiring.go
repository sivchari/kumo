package server

import (
	"bytes"
	"context"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/service/firehose"
	"github.com/sivchari/kumo/internal/service/s3"
)

// wireFirehoseToS3 connects the Firehose service to the S3 service so that
// records buffered by PutRecord/PutRecordBatch are actually delivered to the
// stream's S3 (or ExtendedS3) destination on the buffering interval.
//
// Without this wiring, Firehose only accumulates records in memory and they
// never appear in the destination bucket. Found while running the example
// audit pipeline (admin-ops -> ... -> Firehose -> S3) against kumo and seeing
// the audit bucket stay empty.
func wireFirehoseToS3(registry *service.Registry) {
	fhSvc, ok := registry.Get("firehose")
	if !ok {
		return
	}

	s3Svc, ok := registry.Get("s3")
	if !ok {
		return
	}

	fhTyped, ok := fhSvc.(*firehose.Service)
	if !ok {
		return
	}

	s3Typed, ok := s3Svc.(*s3.Service)
	if !ok {
		return
	}

	fhStorage, ok := fhTyped.Storage().(*firehose.MemoryStorage)
	if !ok {
		return
	}

	fhStorage.SetS3Putter(&firehoseToS3Putter{storage: s3Typed.Storage()})
}

// firehoseToS3Putter adapts the S3 storage layer to the firehose.S3Putter
// interface. The bucket argument is the bucket name (the firehose layer has
// already stripped the ARN prefix).
type firehoseToS3Putter struct {
	storage s3.Storage
}

// PutObject writes a delivered Firehose object into S3.
func (p *firehoseToS3Putter) PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	_, err := p.storage.PutObject(ctx, bucket, key, bytes.NewReader(data), map[string]string{"Content-Type": contentType})
	if err != nil {
		return err //nolint:wrapcheck // adapter is a thin pass-through
	}

	return nil
}
