//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/sivchari/golden"
)

func newKinesisClient(t *testing.T) *kinesis.Client {
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

	return kinesis.NewFromConfig(cfg, func(o *kinesis.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestKinesis_CreateAndDescribeStream(t *testing.T) {
	client := newKinesisClient(t)
	ctx := t.Context()

	streamName := "test-stream"

	// Create stream.
	_, err := client.CreateStream(ctx, &kinesis.CreateStreamInput{
		StreamName: aws.String(streamName),
		ShardCount: aws.Int32(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Describe stream.
	describeOutput, err := client.DescribeStream(ctx, &kinesis.DescribeStreamInput{
		StreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "StreamARN", "StreamCreationTimestamp")).Assert(t.Name()+"_describe", describeOutput)
}

func TestKinesis_ListStreams(t *testing.T) {
	client := newKinesisClient(t)
	ctx := t.Context()

	// Create a stream first.
	streamName := "test-list-stream"

	_, err := client.CreateStream(ctx, &kinesis.CreateStreamInput{
		StreamName: aws.String(streamName),
		ShardCount: aws.Int32(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List streams.
	listOutput, err := client.ListStreams(ctx, &kinesis.ListStreamsInput{
		Limit: aws.Int32(100),
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, name := range listOutput.StreamNames {
		if name == streamName {
			found = true

			break
		}
	}

	if !found {
		t.Error("created stream not found in list")
	}
}

func TestKinesis_ListShards(t *testing.T) {
	client := newKinesisClient(t)
	ctx := t.Context()

	streamName := "test-shards-stream"

	// Create a stream with multiple shards.
	_, err := client.CreateStream(ctx, &kinesis.CreateStreamInput{
		StreamName: aws.String(streamName),
		ShardCount: aws.Int32(2),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List shards.
	listOutput, err := client.ListShards(ctx, &kinesis.ListShardsInput{
		StreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_list", listOutput)
}

func TestKinesis_PutAndGetRecords(t *testing.T) {
	client := newKinesisClient(t)
	ctx := t.Context()

	streamName := "test-records-stream"

	// Create stream.
	_, err := client.CreateStream(ctx, &kinesis.CreateStreamInput{
		StreamName: aws.String(streamName),
		ShardCount: aws.Int32(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put record.
	putOutput, err := client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   aws.String(streamName),
		Data:         []byte("test data"),
		PartitionKey: aws.String("partition-1"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "SequenceNumber")).Assert(t.Name()+"_put", putOutput)

	// Get shard iterator.
	iteratorOutput, err := client.GetShardIterator(ctx, &kinesis.GetShardIteratorInput{
		StreamName:        aws.String(streamName),
		ShardId:           putOutput.ShardId,
		ShardIteratorType: types.ShardIteratorTypeTrimHorizon,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get records.
	getOutput, err := client.GetRecords(ctx, &kinesis.GetRecordsInput{
		ShardIterator: iteratorOutput.ShardIterator,
		Limit:         aws.Int32(100),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "NextShardIterator", "SequenceNumber", "ApproximateArrivalTimestamp")).Assert(t.Name()+"_get", getOutput)
}

func TestKinesis_PutRecords(t *testing.T) {
	client := newKinesisClient(t)
	ctx := t.Context()

	streamName := "test-put-records-stream"

	// Create stream.
	_, err := client.CreateStream(ctx, &kinesis.CreateStreamInput{
		StreamName: aws.String(streamName),
		ShardCount: aws.Int32(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put multiple records.
	putOutput, err := client.PutRecords(ctx, &kinesis.PutRecordsInput{
		StreamName: aws.String(streamName),
		Records: []types.PutRecordsRequestEntry{
			{
				Data:         []byte("record 1"),
				PartitionKey: aws.String("partition-1"),
			},
			{
				Data:         []byte("record 2"),
				PartitionKey: aws.String("partition-2"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "SequenceNumber")).Assert(t.Name()+"_put", putOutput)
}

func TestKinesis_DeleteStream(t *testing.T) {
	client := newKinesisClient(t)
	ctx := t.Context()

	streamName := "test-delete-stream"

	// Create stream.
	_, err := client.CreateStream(ctx, &kinesis.CreateStreamInput{
		StreamName: aws.String(streamName),
		ShardCount: aws.Int32(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete stream.
	_, err = client.DeleteStream(ctx, &kinesis.DeleteStreamInput{
		StreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.DescribeStream(ctx, &kinesis.DescribeStreamInput{
		StreamName: aws.String(streamName),
	})
	if err == nil {
		t.Fatal("expected error for deleted stream")
	}
}

func TestKinesis_StreamNotFound(t *testing.T) {
	client := newKinesisClient(t)
	ctx := t.Context()

	// Try to describe a non-existent stream.
	_, err := client.DescribeStream(ctx, &kinesis.DescribeStreamInput{
		StreamName: aws.String("nonexistent-stream"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent stream")
	}
}

func TestKinesis_GetShardIteratorTypes(t *testing.T) {
	client := newKinesisClient(t)
	ctx := t.Context()

	streamName := "test-iterator-types-stream"

	// Create stream.
	_, err := client.CreateStream(ctx, &kinesis.CreateStreamInput{
		StreamName: aws.String(streamName),
		ShardCount: aws.Int32(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put a record to get shard ID.
	putOutput, err := client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   aws.String(streamName),
		Data:         []byte("test"),
		PartitionKey: aws.String("key"),
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		iteratorType types.ShardIteratorType
	}{
		{"TRIM_HORIZON", types.ShardIteratorTypeTrimHorizon},
		{"LATEST", types.ShardIteratorTypeLatest},
		{"AT_SEQUENCE_NUMBER", types.ShardIteratorTypeAtSequenceNumber},
		{"AFTER_SEQUENCE_NUMBER", types.ShardIteratorTypeAfterSequenceNumber},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &kinesis.GetShardIteratorInput{
				StreamName:        aws.String(streamName),
				ShardId:           putOutput.ShardId,
				ShardIteratorType: tt.iteratorType,
			}

			if tt.iteratorType == types.ShardIteratorTypeAtSequenceNumber ||
				tt.iteratorType == types.ShardIteratorTypeAfterSequenceNumber {
				input.StartingSequenceNumber = putOutput.SequenceNumber
			}

			output, err := client.GetShardIterator(ctx, input)
			if err != nil {
				t.Fatal(err)
			}

			golden.New(t, golden.WithIgnoreFields("ResultMetadata", "ShardIterator")).Assert(t.Name()+"_iterator", output)
		})
	}
}

func TestKinesis_DescribeStreamSummary(t *testing.T) {
	client := newKinesisClient(t)
	ctx := t.Context()
	streamName := "test-stream-summary"

	_, err := client.CreateStream(ctx, &kinesis.CreateStreamInput{
		StreamName: aws.String(streamName),
		ShardCount: aws.Int32(1),
		StreamModeDetails: &types.StreamModeDetails{
			StreamMode: types.StreamModeProvisioned,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteStream(t.Context(), &kinesis.DeleteStreamInput{
			StreamName: aws.String(streamName),
		})
	})

	output, err := client.DescribeStreamSummary(ctx, &kinesis.DescribeStreamSummaryInput{
		StreamName: aws.String(streamName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields(
		"ResultMetadata", "StreamARN", "StreamCreationTimestamp",
	)).Assert(t.Name(), output)
}
