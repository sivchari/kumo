package dynamodb

import (
	"context"
	"testing"
)

func TestUpdateItemInvalidUTF8ExpressionDoesNotPanic(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := store.CreateTable(ctx, &CreateTableRequest{
		TableName: "invalid-utf8-update-test",
		KeySchema: []KeySchemaElement{
			{AttributeName: "pk", KeyType: "HASH"},
		},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "pk", AttributeType: "S"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("UpdateItem panicked for invalid UTF-8 update expression: %v", r)
		}
	}()

	_, _ = store.UpdateItem(ctx, "invalid-utf8-update-test", Item{"pk": {S: ptr("seed")}}, "\x98 REMOVE", nil, nil, ReturnValuesAllNew, ConditionInput{})
}
