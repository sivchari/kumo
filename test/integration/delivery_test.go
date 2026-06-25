//go:build integration

package integration

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	fhtypes "github.com/aws/aws-sdk-go-v2/service/firehose/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// uniqueSuffix yields a per-run token so delivery tests don't collide on
// resource names left behind by an earlier run against a long-lived server.
func uniqueSuffix() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// TestFirehose_DeliversToS3 verifies the Firehose -> S3 data-plane: a record
// put to a stream with an ExtendedS3 destination is delivered as an object
// under the configured prefix, containing the record bytes.
func TestFirehose_DeliversToS3(t *testing.T) {
	ctx := t.Context()
	s3c := newS3Client(t)
	fhc := createFirehoseClient(t)

	suffix := uniqueSuffix()
	bucket := "firehose-delivery-it-" + suffix
	stream := "firehose-delivery-it-stream-" + suffix

	const prefix = "audit/"

	createDeliveryBucket(t, s3c, bucket)

	_, err := fhc.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(stream),
		DeliveryStreamType: fhtypes.DeliveryStreamTypeDirectPut,
		ExtendedS3DestinationConfiguration: &fhtypes.ExtendedS3DestinationConfiguration{
			BucketARN:      aws.String("arn:aws:s3:::" + bucket),
			RoleARN:        aws.String("arn:aws:iam::000000000000:role/firehose"),
			Prefix:         aws.String(prefix),
			BufferingHints: &fhtypes.BufferingHints{IntervalInSeconds: aws.Int32(1), SizeInMBs: aws.Int32(1)},
		},
	})
	if err != nil {
		t.Fatalf("CreateDeliveryStream: %v", err)
	}

	t.Cleanup(func() {
		_, _ = fhc.DeleteDeliveryStream(context.Background(), &firehose.DeleteDeliveryStreamInput{
			DeliveryStreamName: aws.String(stream),
		})
	})

	const payload = `{"sub":"demo","level":"read-write"}` + "\n"

	_, err = fhc.PutRecord(ctx, &firehose.PutRecordInput{
		DeliveryStreamName: aws.String(stream),
		Record:             &fhtypes.Record{Data: []byte(payload)},
	})
	if err != nil {
		t.Fatalf("PutRecord: %v", err)
	}

	keys := waitForS3Keys(t, s3c, bucket, prefix, 20*time.Second)
	if len(keys) == 0 {
		t.Fatal("no Firehose object delivered to S3 under the audit/ prefix")
	}

	body := getS3Object(t, s3c, bucket, keys[0])
	if !strings.Contains(string(body), `"sub":"demo"`) {
		t.Errorf("delivered object body: got %q, want it to contain the record", string(body))
	}
}

// TestCloudTrail_DeliversToS3 verifies the CloudTrail -> S3 data-plane: while a
// trail is logging, management API calls are delivered as gzipped log files
// under the trail's cloudtrail/AWSLogs/... prefix.
func TestCloudTrail_DeliversToS3(t *testing.T) {
	ctx := t.Context()
	s3c := newS3Client(t)
	ctc := newCloudTrailClient(t)

	suffix := uniqueSuffix()
	bucket := "cloudtrail-delivery-it-" + suffix
	trail := "cloudtrail-delivery-it-trail-" + suffix

	const prefix = "cloudtrail/AWSLogs/000000000000/CloudTrail/"

	createDeliveryBucket(t, s3c, bucket)

	_, err := ctc.CreateTrail(ctx, &cloudtrail.CreateTrailInput{
		Name:         aws.String(trail),
		S3BucketName: aws.String(bucket),
		S3KeyPrefix:  aws.String("cloudtrail"),
	})
	if err != nil {
		t.Fatalf("CreateTrail: %v", err)
	}

	t.Cleanup(func() {
		_, _ = ctc.StopLogging(context.Background(), &cloudtrail.StopLoggingInput{Name: aws.String(trail)})
		_, _ = ctc.DeleteTrail(context.Background(), &cloudtrail.DeleteTrailInput{Name: aws.String(trail)})
	})

	if _, err := ctc.StartLogging(ctx, &cloudtrail.StartLoggingInput{Name: aws.String(trail)}); err != nil {
		t.Fatalf("StartLogging: %v", err)
	}

	// Generate a few management calls now that logging is enabled so the sink
	// has events to deliver.
	for range 3 {
		if _, err := ctc.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{}); err != nil {
			t.Fatalf("DescribeTrails: %v", err)
		}
	}

	keys := waitForS3Keys(t, s3c, bucket, prefix, 20*time.Second)
	if len(keys) == 0 {
		t.Fatal("no CloudTrail object delivered to S3 under the cloudtrail/AWSLogs/ prefix")
	}

	if !hasGzLogObject(keys) {
		t.Errorf("delivered keys %v: want at least one .json.gz log object", keys)
	}
}

// createDeliveryBucket creates a bucket for a delivery test and registers
// best-effort cleanup.
func createDeliveryBucket(t *testing.T, client *s3.Client, bucket string) {
	t.Helper()

	if _, err := client.CreateBucket(t.Context(), &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("CreateBucket %q: %v", bucket, err)
	}

	t.Cleanup(func() { deleteBucketBestEffort(client, bucket) })
}

// waitForS3Keys polls ListObjectsV2 until objects appear under prefix or the
// timeout elapses, returning the matching keys.
func waitForS3Keys(t *testing.T, client *s3.Client, bucket, prefix string, timeout time.Duration) []string {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		out, err := client.ListObjectsV2(t.Context(), &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(prefix),
		})
		if err == nil && len(out.Contents) > 0 {
			keys := make([]string, 0, len(out.Contents))
			for _, o := range out.Contents {
				keys = append(keys, aws.ToString(o.Key))
			}

			return keys
		}

		if time.Now().After(deadline) {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// getS3Object fetches an object body.
func getS3Object(t *testing.T, client *s3.Client, bucket, key string) []byte {
	t.Helper()

	out, err := client.GetObject(t.Context(), &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("GetObject %q: %v", key, err)
	}

	defer func() { _ = out.Body.Close() }()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		t.Fatalf("read object %q: %v", key, err)
	}

	if strings.HasSuffix(key, ".gz") {
		return gunzip(t, data)
	}

	return data
}

func gunzip(t *testing.T, data []byte) []byte {
	t.Helper()

	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}

	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}

	return out
}

func hasGzLogObject(keys []string) bool {
	for _, k := range keys {
		if strings.HasSuffix(k, ".json.gz") {
			return true
		}
	}

	return false
}

// deleteBucketBestEffort empties and deletes a bucket, ignoring errors.
func deleteBucketBestEffort(client *s3.Client, bucket string) {
	ctx := context.Background()

	out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
	if err == nil {
		for _, o := range out.Contents {
			_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: o.Key})
		}
	}

	_, _ = client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
}
