package dynamodb

import (
	"context"
	"errors"
	"testing"
)

func TestPutItemRejectsMissingHashKey(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	createKeyValidationTable(t, s, "test-missing-hash-key", []KeySchemaElement{
		{AttributeName: "pk", KeyType: "HASH"},
	})

	ctx := context.Background()

	_, err := s.PutItem(ctx, "test-missing-hash-key", Item{
		"name": {S: ptr("missing key")},
	}, false, ConditionInput{})
	expectValidationException(t, err)
}

func TestPutItemRejectsWrongHashKeyType(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	createKeyValidationTable(t, s, "test-wrong-hash-key-type", []KeySchemaElement{
		{AttributeName: "pk", KeyType: "HASH"},
	})

	ctx := context.Background()

	_, err := s.PutItem(ctx, "test-wrong-hash-key-type", Item{
		"pk": {BOOL: ptr(true)},
	}, false, ConditionInput{})
	expectValidationException(t, err)
}

func TestGetItemRejectsIncompleteCompositeKey(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	createKeyValidationTable(t, s, "test-incomplete-composite-key", []KeySchemaElement{
		{AttributeName: "pk", KeyType: "HASH"},
		{AttributeName: "sk", KeyType: "RANGE"},
	})

	ctx := context.Background()

	_, err := s.GetItem(ctx, "test-incomplete-composite-key", Item{
		"pk": {S: ptr("p1")},
	})
	expectValidationException(t, err)
}

func TestTransactWriteItemsRejectsEmptyAction(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	createKeyValidationTable(t, s, "test-transact-empty-action", []KeySchemaElement{
		{AttributeName: "pk", KeyType: "HASH"},
	})

	ctx := context.Background()

	_, err := s.TransactWriteItems(ctx, []TransactWriteItem{{}})
	expectValidationException(t, err)
}

func TestBatchWriteItemRejectsEmptyWriteRequest(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	createKeyValidationTable(t, s, "test-batch-empty-write", []KeySchemaElement{
		{AttributeName: "pk", KeyType: "HASH"},
	})

	ctx := context.Background()

	_, err := s.BatchWriteItem(ctx, map[string][]WriteRequest{
		"test-batch-empty-write": {{}},
	})
	expectValidationException(t, err)
}

func TestBatchGetItemRejectsIncompleteKey(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	createKeyValidationTable(t, s, "test-batch-incomplete-key", []KeySchemaElement{
		{AttributeName: "pk", KeyType: "HASH"},
		{AttributeName: "sk", KeyType: "RANGE"},
	})

	ctx := context.Background()

	_, err := s.BatchGetItem(ctx, map[string]KeysAndAttributes{
		"test-batch-incomplete-key": {Keys: []Item{{"pk": {S: ptr("p1")}}}},
	})
	expectValidationException(t, err)
}

func createKeyValidationTable(t *testing.T, s *MemoryStorage, tableName string, keySchema []KeySchemaElement) {
	t.Helper()

	ctx := context.Background()
	defs := make([]AttributeDefinition, 0, len(keySchema))

	for _, key := range keySchema {
		defs = append(defs, AttributeDefinition{AttributeName: key.AttributeName, AttributeType: "S"})
	}

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName:            tableName,
		KeySchema:            keySchema,
		AttributeDefinitions: defs,
	})
	if err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
}

func expectValidationException(t *testing.T, err error) {
	t.Helper()

	var tableErr *TableError
	if !errors.As(err, &tableErr) {
		t.Fatalf("got err %v, want ValidationException", err)
	}

	if tableErr.Code != "ValidationException" {
		t.Fatalf("got TableError code %s, want ValidationException", tableErr.Code)
	}
}
