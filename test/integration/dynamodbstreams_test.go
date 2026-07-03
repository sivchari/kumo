//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	"github.com/sivchari/golden"
)

func newDynamoDBStreamsClient(t *testing.T) *dynamodbstreams.Client {
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

	return dynamodbstreams.NewFromConfig(cfg, func(o *dynamodbstreams.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

// createStreamTable creates a DynamoDB table with streams enabled and returns
// the stream ARN. It registers a cleanup function to delete the table.
func createStreamTable(t *testing.T, tableName string) string {
	t.Helper()

	ddbClient := newDynamoDBClient(t)
	ctx := t.Context()

	createOutput, err := ddbClient.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []dynamodbtypes.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: dynamodbtypes.KeyTypeHash},
		},
		AttributeDefinitions: []dynamodbtypes.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: dynamodbtypes.ScalarAttributeTypeS},
		},
		BillingMode: dynamodbtypes.BillingModePayPerRequest,
		StreamSpecification: &dynamodbtypes.StreamSpecification{
			StreamEnabled:  aws.Bool(true),
			StreamViewType: dynamodbtypes.StreamViewTypeNewAndOldImages,
		},
	})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() {
		_, _ = ddbClient.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	if createOutput.TableDescription.LatestStreamArn == nil {
		t.Fatal("expected LatestStreamArn to be set")
	}

	return *createOutput.TableDescription.LatestStreamArn
}

func TestDynamoDBStreams_DescribeStream(t *testing.T) {
	streamsClient := newDynamoDBStreamsClient(t)
	ctx := t.Context()
	streamArn := createStreamTable(t, "test-streams-describe")

	describeOutput, err := streamsClient.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: aws.String(streamArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ResultMetadata", "StreamArn", "StreamLabel", "CreationRequestDateTime",
		"StartingSequenceNumber",
	)).Assert(t.Name(), describeOutput)
}

func TestDynamoDBStreams_GetShardIteratorAndGetRecords(t *testing.T) {
	streamsClient := newDynamoDBStreamsClient(t)
	ddbClient := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-streams-records"
	streamArn := createStreamTable(t, tableName)

	// Put an item to generate a stream record.
	_, err := ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]dynamodbtypes.AttributeValue{
			"pk":   &dynamodbtypes.AttributeValueMemberS{Value: "stream-item-1"},
			"data": &dynamodbtypes.AttributeValueMemberS{Value: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("failed to put item: %v", err)
	}

	// Brief pause to allow stream record to propagate.
	time.Sleep(100 * time.Millisecond)

	// DescribeStream to get shard info.
	describeOutput, err := streamsClient.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: aws.String(streamArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(describeOutput.StreamDescription.Shards) == 0 {
		t.Fatal("expected at least one shard")
	}

	shardID := describeOutput.StreamDescription.Shards[0].ShardId

	// GetShardIterator with TRIM_HORIZON.
	iteratorOutput, err := streamsClient.GetShardIterator(ctx, &dynamodbstreams.GetShardIteratorInput{
		StreamArn:         aws.String(streamArn),
		ShardId:           shardID,
		ShardIteratorType: "TRIM_HORIZON",
	})
	if err != nil {
		t.Fatal(err)
	}

	if iteratorOutput.ShardIterator == nil {
		t.Fatal("expected shard iterator to be non-nil")
	}

	// GetRecords.
	recordsOutput, err := streamsClient.GetRecords(ctx, &dynamodbstreams.GetRecordsInput{
		ShardIterator: iteratorOutput.ShardIterator,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recordsOutput.Records) == 0 {
		t.Fatal("expected at least one record")
	}

	golden.New(t, golden.WithIgnoreFields(
		"ResultMetadata", "NextShardIterator",
		"EventID", "SequenceNumber", "ApproximateCreationDateTime",
	)).Assert(t.Name()+"_records", recordsOutput)
}

func TestDynamoDBStreams_MultipleOperations(t *testing.T) {
	streamsClient := newDynamoDBStreamsClient(t)
	ddbClient := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-streams-multi-ops"
	streamArn := createStreamTable(t, tableName)

	// INSERT: Put an item.
	_, err := ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]dynamodbtypes.AttributeValue{
			"pk":   &dynamodbtypes.AttributeValueMemberS{Value: "multi-1"},
			"data": &dynamodbtypes.AttributeValueMemberS{Value: "original"},
		},
	})
	if err != nil {
		t.Fatalf("failed to put item: %v", err)
	}

	// MODIFY: Update the item.
	_, err = ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"pk": &dynamodbtypes.AttributeValueMemberS{Value: "multi-1"},
		},
		UpdateExpression: aws.String("SET #d = :val"),
		ExpressionAttributeNames: map[string]string{
			"#d": "data",
		},
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":val": &dynamodbtypes.AttributeValueMemberS{Value: "updated"},
		},
	})
	if err != nil {
		t.Fatalf("failed to update item: %v", err)
	}

	// REMOVE: Delete the item.
	_, err = ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"pk": &dynamodbtypes.AttributeValueMemberS{Value: "multi-1"},
		},
	})
	if err != nil {
		t.Fatalf("failed to delete item: %v", err)
	}

	// Brief pause to allow stream records to propagate.
	time.Sleep(100 * time.Millisecond)

	// DescribeStream to get shard info.
	describeOutput, err := streamsClient.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: aws.String(streamArn),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(describeOutput.StreamDescription.Shards) == 0 {
		t.Fatal("expected at least one shard")
	}

	shardID := describeOutput.StreamDescription.Shards[0].ShardId

	// GetShardIterator.
	iteratorOutput, err := streamsClient.GetShardIterator(ctx, &dynamodbstreams.GetShardIteratorInput{
		StreamArn:         aws.String(streamArn),
		ShardId:           shardID,
		ShardIteratorType: "TRIM_HORIZON",
	})
	if err != nil {
		t.Fatal(err)
	}

	// GetRecords - should contain INSERT, MODIFY, REMOVE events.
	recordsOutput, err := streamsClient.GetRecords(ctx, &dynamodbstreams.GetRecordsInput{
		ShardIterator: iteratorOutput.ShardIterator,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recordsOutput.Records) < 3 {
		t.Fatalf("expected at least 3 records (INSERT, MODIFY, REMOVE), got %d", len(recordsOutput.Records))
	}

	golden.New(t, golden.WithIgnoreFields(
		"ResultMetadata", "NextShardIterator",
		"EventID", "SequenceNumber", "ApproximateCreationDateTime",
	)).Assert(t.Name()+"_records", recordsOutput)
}
