//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
	"github.com/sivchari/golden"
)

func TestFirehose_CreateDeliveryStream(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createFirehoseClient(t)

	streamName := "test-delivery-stream"

	// Create delivery stream.
	result, err := client.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DeliveryStreamARN", "ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFirehose_DescribeDeliveryStream(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createFirehoseClient(t)

	streamName := "describe-test-stream"

	// Create delivery stream.
	_, err := client.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Describe delivery stream.
	result, err := client.DescribeDeliveryStream(ctx, &firehose.DescribeDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields(
		"DeliveryStreamARN",
		"CreateTimestamp",
		"LastUpdateTimestamp",
		"VersionId",
		"Destinations",
		"ResultMetadata",
	)).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFirehose_ListDeliveryStreams(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createFirehoseClient(t)

	streamName1 := "list-test-stream-1"
	streamName2 := "list-test-stream-2"

	// Create delivery streams.
	_, err := client.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName1),
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName2),
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
	})
	if err != nil {
		t.Fatal(err)
	}

	// List delivery streams.
	result, err := client.ListDeliveryStreams(ctx, &firehose.ListDeliveryStreamsInput{
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DeliveryStreamNames", "ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName1),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName2),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFirehose_PutRecord(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createFirehoseClient(t)

	streamName := "put-record-test-stream"

	// Create delivery stream.
	_, err := client.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put record.
	result, err := client.PutRecord(ctx, &firehose.PutRecordInput{
		DeliveryStreamName: aws.String(streamName),
		Record: &types.Record{
			Data: []byte("test data"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("RecordId", "ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFirehose_PutRecordBatch(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createFirehoseClient(t)

	streamName := "put-record-batch-test-stream"

	// Create delivery stream.
	_, err := client.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put record batch.
	result, err := client.PutRecordBatch(ctx, &firehose.PutRecordBatchInput{
		DeliveryStreamName: aws.String(streamName),
		Records: []types.Record{
			{Data: []byte("test data 1")},
			{Data: []byte("test data 2")},
			{Data: []byte("test data 3")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("RequestResponses", "ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFirehose_UpdateDestination(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createFirehoseClient(t)

	streamName := "update-dest-test-stream"

	// Create delivery stream.
	_, err := client.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get stream description to get destination ID.
	descResult, err := client.DescribeDeliveryStream(ctx, &firehose.DescribeDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(descResult.DeliveryStreamDescription.Destinations) != 1 {
		t.Fatalf("expected 1 destination, got %d", len(descResult.DeliveryStreamDescription.Destinations))
	}

	destID := descResult.DeliveryStreamDescription.Destinations[0].DestinationId
	versionID := descResult.DeliveryStreamDescription.VersionId

	// Update destination.
	result, err := client.UpdateDestination(ctx, &firehose.UpdateDestinationInput{
		DeliveryStreamName:             aws.String(streamName),
		CurrentDeliveryStreamVersionId: versionID,
		DestinationId:                  destID,
		S3DestinationUpdate: &types.S3DestinationUpdate{
			BucketARN: aws.String("arn:aws:s3:::test-bucket"),
			RoleARN:   aws.String("arn:aws:iam::000000000000:role/test-role"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFirehose_ListTagsForDeliveryStream(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createFirehoseClient(t)

	streamName := "list-tags-test-stream"

	// Create delivery stream with tags (Terraform supplies them on create).
	_, err := client.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
		Tags: []types.Tag{
			{Key: aws.String("env"), Value: aws.String("local")},
			{Key: aws.String("app"), Value: aws.String("idp")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// List tags (this is what Terraform calls right after create).
	result, err := client.ListTagsForDeliveryStream(ctx, &firehose.ListTagsForDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFirehose_TagDeliveryStream(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createFirehoseClient(t)

	streamName := "tag-test-stream"

	// Create delivery stream.
	_, err := client.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Tag the delivery stream.
	_, err = client.TagDeliveryStream(ctx, &firehose.TagDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
		Tags: []types.Tag{
			{Key: aws.String("env"), Value: aws.String("local")},
			{Key: aws.String("team"), Value: aws.String("sec")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify the tags are readable back.
	result, err := client.ListTagsForDeliveryStream(ctx, &firehose.ListTagsForDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFirehose_UntagDeliveryStream(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	client := createFirehoseClient(t)

	streamName := "untag-test-stream"

	// Create delivery stream with tags.
	_, err := client.CreateDeliveryStream(ctx, &firehose.CreateDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
		DeliveryStreamType: types.DeliveryStreamTypeDirectPut,
		Tags: []types.Tag{
			{Key: aws.String("env"), Value: aws.String("local")},
			{Key: aws.String("team"), Value: aws.String("sec")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Remove one tag.
	_, err = client.UntagDeliveryStream(ctx, &firehose.UntagDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
		TagKeys:            []string{"team"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify only the remaining tag is returned.
	result, err := client.ListTagsForDeliveryStream(ctx, &firehose.ListTagsForDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDeliveryStream(ctx, &firehose.DeleteDeliveryStreamInput{
		DeliveryStreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func createFirehoseClient(t *testing.T) *firehose.Client {
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

	return firehose.NewFromConfig(cfg, func(o *firehose.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}
