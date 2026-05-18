//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/sivchari/golden"
)

func newDynamoDBClient(t *testing.T) *dynamodb.Client {
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

	return dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestDynamoDB_CreateAndDeleteTable(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-create-delete"

	// Create table.
	createOutput, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TableArn", "TableId", "CreationDateTime", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Delete table.
	_, err = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		t.Fatalf("failed to delete table: %v", err)
	}
}

func TestDynamoDB_ListTables(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-list"

	// Create table.
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// List tables - dynamic list, skip golden test.
	_, err = client.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDynamoDB_DescribeTable(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-describe"

	// Create table.
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Describe table.
	descOutput, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TableArn", "TableId", "CreationDateTime", "TableSizeBytes", "ItemCount", "ResultMetadata")).Assert(t.Name(), descOutput)
}

func TestDynamoDB_PutAndGetItem(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-put-get"

	// Create table.
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put item.
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":   &types.AttributeValueMemberS{Value: "test-id"},
			"name": &types.AttributeValueMemberS{Value: "Test Item"},
			"age":  &types.AttributeValueMemberN{Value: "25"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get item.
	getOutput, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "test-id"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestDynamoDB_DeleteItem(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-delete-item"

	// Create table.
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put item.
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":   &types.AttributeValueMemberS{Value: "delete-me"},
			"name": &types.AttributeValueMemberS{Value: "To Delete"},
		},
	})
	if err != nil {
		t.Fatalf("failed to put item: %v", err)
	}

	// Delete item.
	_, err = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "delete-me"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify item is deleted.
	getOutput, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "delete-me"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get_after_delete", getOutput)
}

func TestDynamoDB_UpdateItem(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-update-item"

	// Create table.
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put initial item.
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":   &types.AttributeValueMemberS{Value: "update-me"},
			"name": &types.AttributeValueMemberS{Value: "Original"},
		},
	})
	if err != nil {
		t.Fatalf("failed to put item: %v", err)
	}

	// Update item.
	updateOutput, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "update-me"},
		},
		UpdateExpression: aws.String("SET #n = :name"),
		ExpressionAttributeNames: map[string]string{
			"#n": "name",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name": &types.AttributeValueMemberS{Value: "Updated"},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_update", updateOutput)

	// Verify item is updated.
	getOutput, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "update-me"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get_after_update", getOutput)
}

func TestDynamoDB_Query(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-query"

	// Create table with sort key.
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String("sk"),
				KeyType:       types.KeyTypeRange,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("sk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put multiple items.
	items := []struct {
		pk   string
		sk   string
		data string
	}{
		{"user-1", "item-1", "data1"},
		{"user-1", "item-2", "data2"},
		{"user-1", "item-3", "data3"},
		{"user-2", "item-1", "data4"},
	}

	for _, item := range items {
		_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item: map[string]types.AttributeValue{
				"pk":   &types.AttributeValueMemberS{Value: item.pk},
				"sk":   &types.AttributeValueMemberS{Value: item.sk},
				"data": &types.AttributeValueMemberS{Value: item.data},
			},
		})
		if err != nil {
			t.Fatalf("failed to put item: %v", err)
		}
	}

	// Query items for user-1.
	queryOutput, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "user-1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), queryOutput)
}

func TestDynamoDB_Scan(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-scan"

	// Create table.
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put multiple items.
	for i := 0; i < 5; i++ {
		_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item: map[string]types.AttributeValue{
				"pk":   &types.AttributeValueMemberS{Value: "item-" + string(rune('a'+i))},
				"data": &types.AttributeValueMemberS{Value: "data"},
			},
		})
		if err != nil {
			t.Fatalf("failed to put item: %v", err)
		}
	}

	// Scan all items.
	scanOutput, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), scanOutput)
}

func TestDynamoDB_CompositeKey(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-composite-key"

	// Create table with composite key.
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("pk"),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String("sk"),
				KeyType:       types.KeyTypeRange,
			},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("pk"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("sk"),
				AttributeType: types.ScalarAttributeTypeN,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put item with composite key.
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":   &types.AttributeValueMemberS{Value: "user-1"},
			"sk":   &types.AttributeValueMemberN{Value: "100"},
			"name": &types.AttributeValueMemberS{Value: "Test User"},
		},
	})
	if err != nil {
		t.Fatalf("failed to put item: %v", err)
	}

	// Get item with composite key.
	getOutput, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "user-1"},
			"sk": &types.AttributeValueMemberN{Value: "100"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestDynamoDB_UpdateTimeToLive(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-ttl"

	// Create table.
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Enable TTL.
	updateOutput, err := client.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(tableName),
		TimeToLiveSpecification: &types.TimeToLiveSpecification{
			AttributeName: aws.String("ttl"),
			Enabled:       aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_update", updateOutput)

	// Describe TTL.
	describeOutput, err := client.DescribeTimeToLive(ctx, &dynamodb.DescribeTimeToLiveInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestDynamoDB_PutItem_ConditionExpression_AttributeNotExists(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-condition-put"

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// First put should succeed (item does not exist).
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":   &types.AttributeValueMemberS{Value: "id-1"},
			"data": &types.AttributeValueMemberS{Value: "first"},
		},
		ConditionExpression: aws.String("attribute_not_exists(pk)"),
	})
	if err != nil {
		t.Fatalf("first PutItem should succeed: %v", err)
	}

	// Second put with same key should fail (item exists).
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":   &types.AttributeValueMemberS{Value: "id-1"},
			"data": &types.AttributeValueMemberS{Value: "second"},
		},
		ConditionExpression: aws.String("attribute_not_exists(pk)"),
	})
	if err == nil {
		t.Fatal("second PutItem should fail with ConditionalCheckFailedException")
	}

	var ccfe *types.ConditionalCheckFailedException
	if !errors.As(err, &ccfe) {
		t.Fatalf("expected ConditionalCheckFailedException, got: %T: %v", err, err)
	}

	// Verify original item is preserved.
	getOutput, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "id-1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestDynamoDB_PutItem_ConditionExpression_Equality(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-condition-equality"

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put initial item.
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":     &types.AttributeValueMemberS{Value: "id-1"},
			"status": &types.AttributeValueMemberS{Value: "active"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update with correct condition should succeed.
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":     &types.AttributeValueMemberS{Value: "id-1"},
			"status": &types.AttributeValueMemberS{Value: "inactive"},
		},
		ConditionExpression: aws.String("#s = :expected"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expected": &types.AttributeValueMemberS{Value: "active"},
		},
	})
	if err != nil {
		t.Fatalf("conditional put with matching status should succeed: %v", err)
	}

	// Update with wrong condition should fail.
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":     &types.AttributeValueMemberS{Value: "id-1"},
			"status": &types.AttributeValueMemberS{Value: "deleted"},
		},
		ConditionExpression: aws.String("#s = :expected"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expected": &types.AttributeValueMemberS{Value: "active"},
		},
	})
	if err == nil {
		t.Fatal("conditional put with wrong status should fail")
	}

	var ccfe *types.ConditionalCheckFailedException
	if !errors.As(err, &ccfe) {
		t.Fatalf("expected ConditionalCheckFailedException, got: %T: %v", err, err)
	}
}

func TestDynamoDB_DeleteItem_ConditionExpression(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-condition-delete"

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put item.
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":     &types.AttributeValueMemberS{Value: "id-1"},
			"status": &types.AttributeValueMemberS{Value: "active"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete with wrong condition should fail.
	_, err = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "id-1"},
		},
		ConditionExpression: aws.String("#s = :expected"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expected": &types.AttributeValueMemberS{Value: "inactive"},
		},
	})
	if err == nil {
		t.Fatal("delete with wrong condition should fail")
	}

	var ccfe *types.ConditionalCheckFailedException
	if !errors.As(err, &ccfe) {
		t.Fatalf("expected ConditionalCheckFailedException, got: %T: %v", err, err)
	}

	// Delete with correct condition should succeed.
	_, err = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "id-1"},
		},
		ConditionExpression: aws.String("#s = :expected"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expected": &types.AttributeValueMemberS{Value: "active"},
		},
	})
	if err != nil {
		t.Fatalf("delete with correct condition should succeed: %v", err)
	}
}

func TestDynamoDB_UpdateItem_ConditionExpression(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-condition-update"

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put initial item.
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"pk":      &types.AttributeValueMemberS{Value: "id-1"},
			"version": &types.AttributeValueMemberN{Value: "1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update with correct version (optimistic locking).
	_, err = client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "id-1"},
		},
		UpdateExpression:    aws.String("SET version = :newver"),
		ConditionExpression: aws.String("version = :curver"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":curver": &types.AttributeValueMemberN{Value: "1"},
			":newver": &types.AttributeValueMemberN{Value: "2"},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		t.Fatalf("update with correct version should succeed: %v", err)
	}

	// Update with stale version should fail.
	_, err = client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: "id-1"},
		},
		UpdateExpression:    aws.String("SET version = :newver"),
		ConditionExpression: aws.String("version = :curver"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":curver": &types.AttributeValueMemberN{Value: "1"},
			":newver": &types.AttributeValueMemberN{Value: "3"},
		},
	})
	if err == nil {
		t.Fatal("update with stale version should fail")
	}

	var ccfe2 *types.ConditionalCheckFailedException
	if !errors.As(err, &ccfe2) {
		t.Fatalf("expected ConditionalCheckFailedException, got: %T: %v", err, err)
	}
}

func TestDynamoDB_TransactWriteItems(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-transact-write"

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Successful transaction: put two items.
	_, err = client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{Put: &types.Put{
				TableName:           aws.String(tableName),
				Item:                map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "tx-1"}, "data": &types.AttributeValueMemberS{Value: "first"}},
				ConditionExpression: aws.String("attribute_not_exists(pk)"),
			}},
			{Put: &types.Put{
				TableName:           aws.String(tableName),
				Item:                map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "tx-2"}, "data": &types.AttributeValueMemberS{Value: "second"}},
				ConditionExpression: aws.String("attribute_not_exists(pk)"),
			}},
		},
	})
	if err != nil {
		t.Fatalf("transaction should succeed: %v", err)
	}

	// Verify both items exist.
	get1, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "tx-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get_tx1", get1)

	get2, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "tx-2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get_tx2", get2)

	// Failed transaction: one condition fails, nothing should be written.
	_, err = client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{Put: &types.Put{
				TableName:           aws.String(tableName),
				Item:                map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "tx-3"}, "data": &types.AttributeValueMemberS{Value: "third"}},
				ConditionExpression: aws.String("attribute_not_exists(pk)"),
			}},
			{Put: &types.Put{
				TableName:           aws.String(tableName),
				Item:                map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "tx-1"}, "data": &types.AttributeValueMemberS{Value: "overwrite"}},
				ConditionExpression: aws.String("attribute_not_exists(pk)"),
			}},
		},
	})
	if err == nil {
		t.Fatal("transaction should fail because tx-1 already exists")
	}

	var txErr *types.TransactionCanceledException
	if !errors.As(err, &txErr) {
		t.Fatalf("expected TransactionCanceledException, got: %T: %v", err, err)
	}

	// Verify tx-3 was NOT created (all-or-nothing).
	get3, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "tx-3"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get_tx3_not_exists", get3)
}

func TestDynamoDB_TransactGetItems(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-transact-get"

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put items.
	for _, id := range []string{"g1", "g2"} {
		_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item: map[string]types.AttributeValue{
				"pk":   &types.AttributeValueMemberS{Value: id},
				"data": &types.AttributeValueMemberS{Value: "data-" + id},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// TransactGetItems.
	result, err := client.TransactGetItems(ctx, &dynamodb.TransactGetItemsInput{
		TransactItems: []types.TransactGetItem{
			{Get: &types.Get{
				TableName: aws.String(tableName),
				Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "g1"}},
			}},
			{Get: &types.Get{
				TableName: aws.String(tableName),
				Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "g2"}},
			}},
			{Get: &types.Get{
				TableName: aws.String(tableName),
				Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "missing"}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), result)
}

func TestDynamoDB_GlobalSecondaryIndex(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-gsi"

	// Create table with GSI.
	createOutput, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("gsi_pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("gsi-index"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("gsi_pk"), KeyType: types.KeyTypeHash},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TableArn", "TableId", "CreationDateTime", "ResultMetadata", "IndexArn")).Assert(t.Name()+"_create", createOutput)

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put items with GSI key.
	for _, item := range []struct{ pk, sk, gsiPK, data string }{
		{"user-1", "order-1", "region-east", "data1"},
		{"user-1", "order-2", "region-west", "data2"},
		{"user-2", "order-3", "region-east", "data3"},
	} {
		_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item: map[string]types.AttributeValue{
				"pk":     &types.AttributeValueMemberS{Value: item.pk},
				"sk":     &types.AttributeValueMemberS{Value: item.sk},
				"gsi_pk": &types.AttributeValueMemberS{Value: item.gsiPK},
				"data":   &types.AttributeValueMemberS{Value: item.data},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Query via GSI.
	queryOutput, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("gsi-index"),
		KeyConditionExpression: aws.String("gsi_pk = :val"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":val": &types.AttributeValueMemberS{Value: "region-east"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_query_gsi", queryOutput)

	// Query via table primary key still works.
	queryTableOutput, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("pk = :val"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":val": &types.AttributeValueMemberS{Value: "user-1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_query_table", queryTableOutput)
}

func TestDynamoDB_BatchWriteItem(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-batch-write"

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Batch write 3 items.
	_, err = client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			tableName: {
				{PutRequest: &types.PutRequest{Item: map[string]types.AttributeValue{
					"pk": &types.AttributeValueMemberS{Value: "bw-1"}, "data": &types.AttributeValueMemberS{Value: "one"},
				}}},
				{PutRequest: &types.PutRequest{Item: map[string]types.AttributeValue{
					"pk": &types.AttributeValueMemberS{Value: "bw-2"}, "data": &types.AttributeValueMemberS{Value: "two"},
				}}},
				{PutRequest: &types.PutRequest{Item: map[string]types.AttributeValue{
					"pk": &types.AttributeValueMemberS{Value: "bw-3"}, "data": &types.AttributeValueMemberS{Value: "three"},
				}}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify items via scan.
	scanOutput, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_scan", scanOutput)

	// Batch delete one item.
	_, err = client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			tableName: {
				{DeleteRequest: &types.DeleteRequest{Key: map[string]types.AttributeValue{
					"pk": &types.AttributeValueMemberS{Value: "bw-2"},
				}}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify bw-2 deleted.
	getOutput, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "bw-2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_deleted", getOutput)
}

func TestDynamoDB_BatchGetItem(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-batch-get"

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// Put items.
	for _, id := range []string{"bg-1", "bg-2", "bg-3"} {
		_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item: map[string]types.AttributeValue{
				"pk":   &types.AttributeValueMemberS{Value: id},
				"data": &types.AttributeValueMemberS{Value: "data-" + id},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Batch get 2 existing + 1 missing.
	result, err := client.BatchGetItem(ctx, &dynamodb.BatchGetItemInput{
		RequestItems: map[string]types.KeysAndAttributes{
			tableName: {
				Keys: []map[string]types.AttributeValue{
					{"pk": &types.AttributeValueMemberS{Value: "bg-1"}},
					{"pk": &types.AttributeValueMemberS{Value: "bg-3"}},
					{"pk": &types.AttributeValueMemberS{Value: "bg-missing"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), result)
}

func TestDynamoDB_UpdateItem_AttributeUpdates(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-attribute-updates"

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("OwnerId"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("Key"), KeyType: types.KeyTypeRange},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("OwnerId"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("Key"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"OwnerId": &types.AttributeValueMemberS{Value: "owner-1"},
			"Key":     &types.AttributeValueMemberS{Value: "key-1"},
			"Name":    &types.AttributeValueMemberS{Value: "original"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	updateOutput, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"OwnerId": &types.AttributeValueMemberS{Value: "owner-1"},
			"Key":     &types.AttributeValueMemberS{Value: "key-1"},
		},
		AttributeUpdates: map[string]types.AttributeValueUpdate{
			"Name": {
				Action: types.AttributeActionPut,
				Value:  &types.AttributeValueMemberS{Value: "updated"},
			},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), updateOutput)

	getOutput, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"OwnerId": &types.AttributeValueMemberS{Value: "owner-1"},
			"Key":     &types.AttributeValueMemberS{Value: "key-1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	name, ok := getOutput.Item["Name"].(*types.AttributeValueMemberS)
	if !ok || name.Value != "updated" {
		t.Errorf("expected Name=updated, got %v", getOutput.Item["Name"])
	}
}

func TestDynamoDB_UpdateTable(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-update-table"

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(tableName),
		BillingMode: types.BillingModePayPerRequest,
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// UpdateTable should succeed and return the table description.
	updateOutput, err := client.UpdateTable(ctx, &dynamodb.UpdateTableInput{
		TableName:   aws.String(tableName),
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields(
		"TableArn", "TableId", "CreationDateTime", "TableSizeBytes", "ItemCount", "ResultMetadata",
	)).Assert(t.Name(), updateOutput)
}

func TestDynamoDB_StreamSpecification(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-stream-spec"

	createOutput, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(tableName),
		BillingMode: types.BillingModePayPerRequest,
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		StreamSpecification: &types.StreamSpecification{
			StreamEnabled:  aws.Bool(true),
			StreamViewType: types.StreamViewTypeNewAndOldImages,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	golden.New(t, golden.WithIgnoreFields(
		"TableArn", "TableId", "CreationDateTime", "LatestStreamArn",
		"TableSizeBytes", "ItemCount", "ResultMetadata",
	)).Assert(t.Name()+"_create", createOutput)

	// DescribeTable should also include stream info.
	descOutput, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		t.Fatal(err)
	}

	if descOutput.Table.LatestStreamArn == nil || *descOutput.Table.LatestStreamArn == "" {
		t.Error("expected LatestStreamArn to be set")
	}

	if descOutput.Table.StreamSpecification == nil || !*descOutput.Table.StreamSpecification.StreamEnabled {
		t.Error("expected StreamSpecification.StreamEnabled to be true")
	}
}

func TestDynamoDB_TagOperations(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-tag-ops"

	// Create table.
	createOutput, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	tableArn := *createOutput.TableDescription.TableArn

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// ListTagsOfResource -- initially empty.
	listOutput1, err := client.ListTagsOfResource(ctx, &dynamodb.ListTagsOfResourceInput{
		ResourceArn: aws.String(tableArn),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_list_empty", listOutput1)

	// TagResource -- add two tags.
	_, err = client.TagResource(ctx, &dynamodb.TagResourceInput{
		ResourceArn: aws.String(tableArn),
		Tags: []types.Tag{
			{Key: aws.String("env"), Value: aws.String("dev")},
			{Key: aws.String("team"), Value: aws.String("platform")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ListTagsOfResource -- should have two tags.
	listOutput2, err := client.ListTagsOfResource(ctx, &dynamodb.ListTagsOfResourceInput{
		ResourceArn: aws.String(tableArn),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_list_after_tag", listOutput2)

	// UntagResource -- remove "env" tag.
	_, err = client.UntagResource(ctx, &dynamodb.UntagResourceInput{
		ResourceArn: aws.String(tableArn),
		TagKeys:     []string{"env"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ListTagsOfResource -- should have one tag.
	listOutput3, err := client.ListTagsOfResource(ctx, &dynamodb.ListTagsOfResourceInput{
		ResourceArn: aws.String(tableArn),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_list_after_untag", listOutput3)
}

func TestDynamoDB_DescribeContinuousBackups(t *testing.T) {
	client := newDynamoDBClient(t)
	ctx := t.Context()
	tableName := "test-table-backups"

	// Create table.
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	// DescribeContinuousBackups.
	backupsOutput, err := client.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), backupsOutput)
}
