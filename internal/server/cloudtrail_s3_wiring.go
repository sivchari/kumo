package server

import (
	"bytes"
	"context"

	"github.com/sivchari/kumo/internal/service"
	"github.com/sivchari/kumo/internal/service/cloudtrail"
	"github.com/sivchari/kumo/internal/service/s3"
)

// wireCloudTrailToS3 connects the CloudTrail service to the S3 service so that,
// while a trail is logging, captured management API calls are delivered as
// gzipped CloudTrail log files into the trail's S3 bucket/prefix.
//
// Without this wiring, CloudTrail only flips an IsLogging flag and never writes
// anything to S3, so the trail's bucket stays empty.
func wireCloudTrailToS3(registry *service.Registry) {
	ctSvc, ok := registry.Get("cloudtrail")
	if !ok {
		return
	}

	s3Svc, ok := registry.Get("s3")
	if !ok {
		return
	}

	ctTyped, ok := ctSvc.(*cloudtrail.Service)
	if !ok {
		return
	}

	s3Typed, ok := s3Svc.(*s3.Service)
	if !ok {
		return
	}

	ctStorage, ok := ctTyped.Storage().(*cloudtrail.MemoryStorage)
	if !ok {
		return
	}

	ctStorage.SetS3Putter(&cloudtrailToS3Putter{storage: s3Typed.Storage()})
}

// cloudtrailToS3Putter adapts the S3 storage layer to the cloudtrail.S3Putter
// interface.
type cloudtrailToS3Putter struct {
	storage s3.Storage
}

// PutObject writes a delivered CloudTrail log (or marker) object into S3.
func (p *cloudtrailToS3Putter) PutObject(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	_, err := p.storage.PutObject(ctx, bucket, key, bytes.NewReader(data), map[string]string{"Content-Type": contentType})
	if err != nil {
		return err //nolint:wrapcheck // adapter is a thin pass-through
	}

	return nil
}
