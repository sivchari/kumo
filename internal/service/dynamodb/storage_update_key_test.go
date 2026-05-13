package dynamodb

import (
	"context"
	"errors"
	"testing"
)

func TestUpdateItemRejectsKeyAttributeUpdates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expr       string
		exprNames  map[string]string
		exprValues map[string]AttributeValue
	}{
		{
			name: "remove hash key",
			expr: "REMOVE pk",
		},
		{
			name:       "set range key through expression attribute name",
			expr:       "SET #sk = :new",
			exprNames:  map[string]string{"#sk": "sk"},
			exprValues: map[string]AttributeValue{":new": {S: ptr("changed")}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newKeyUpdateTestStorage(t)
			key := Item{"pk": {S: ptr("seed")}, "sk": {S: ptr("sort")}}

			_, err := store.UpdateItem(context.Background(), "key-update-test", key, tt.expr, tt.exprNames, tt.exprValues, ReturnValuesAllNew, ConditionInput{})
			if err == nil {
				t.Fatal("expected UpdateItem to reject key attribute update")
			}

			var tableErr *TableError
			if !errors.As(err, &tableErr) || tableErr.Code != "ValidationException" {
				t.Fatalf("expected ValidationException, got %v", err)
			}

			item, err := store.GetItem(context.Background(), "key-update-test", key)
			if err != nil {
				t.Fatal(err)
			}

			if item["pk"].S == nil || *item["pk"].S != "seed" {
				t.Fatalf("hash key was mutated: %#v", item["pk"])
			}

			if item["sk"].S == nil || *item["sk"].S != "sort" {
				t.Fatalf("range key was mutated: %#v", item["sk"])
			}
		})
	}
}

func newKeyUpdateTestStorage(t *testing.T) *MemoryStorage {
	t.Helper()

	store := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := store.CreateTable(ctx, &CreateTableRequest{
		TableName: "key-update-test",
		KeySchema: []KeySchemaElement{
			{AttributeName: "pk", KeyType: "HASH"},
			{AttributeName: "sk", KeyType: "RANGE"},
		},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "pk", AttributeType: "S"},
			{AttributeName: "sk", AttributeType: "S"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.PutItem(ctx, "key-update-test", Item{
		"pk":     {S: ptr("seed")},
		"sk":     {S: ptr("sort")},
		"status": {S: ptr("active")},
	}, false, ConditionInput{})
	if err != nil {
		t.Fatal(err)
	}

	return store
}
