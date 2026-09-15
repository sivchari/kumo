package dynamodb

import (
	"context"
	"testing"
)

//nolint:funlen // Test function exercises multiple Query scenarios.
func TestQueryKeyConditionExpression(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName: "test-query-keycond",
		KeySchema: []KeySchemaElement{
			{AttributeName: "PK", KeyType: keyTypeHash},
			{AttributeName: "SK", KeyType: keyTypeRange},
		},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "PK", AttributeType: "S"},
			{AttributeName: "SK", AttributeType: "S"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert items with different SK values.
	items := []Item{
		{"PK": {S: ptr("tenant1")}, "SK": {S: ptr("100")}},
		{"PK": {S: ptr("tenant1")}, "SK": {S: ptr("200")}},
		{"PK": {S: ptr("tenant1")}, "SK": {S: ptr("300")}},
		{"PK": {S: ptr("tenant1")}, "SK": {S: ptr("400")}},
		{"PK": {S: ptr("tenant2")}, "SK": {S: ptr("100")}},
	}
	for _, item := range items {
		if _, err := s.PutItem(ctx, "test-query-keycond", item, false, ConditionInput{}); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("SK >= :val filters range key", func(t *testing.T) {
		t.Parallel()

		results, _, _, err := s.Query(ctx, "test-query-keycond", "",
			"PK = :pk AND SK >= :sk",
			"",
			nil,
			map[string]AttributeValue{
				testExprPK: {S: ptr("tenant1")},
				":sk":      {S: ptr("200")},
			},
			0, nil, true)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 3 {
			t.Errorf("got %d results, want 3 (SK >= 200 should match 200, 300, 400)", len(results))
		}
	})

	t.Run("SK > :val filters range key", func(t *testing.T) {
		t.Parallel()

		results, _, _, err := s.Query(ctx, "test-query-keycond", "",
			"PK = :pk AND SK > :sk",
			"",
			nil,
			map[string]AttributeValue{
				testExprPK: {S: ptr("tenant1")},
				":sk":      {S: ptr("200")},
			},
			0, nil, true)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 2 {
			t.Errorf("got %d results, want 2 (SK > 200 should match 300, 400)", len(results))
		}
	})

	t.Run("SK BETWEEN :lo AND :hi filters range key", func(t *testing.T) {
		t.Parallel()

		results, _, _, err := s.Query(ctx, "test-query-keycond", "",
			"PK = :pk AND SK BETWEEN :lo AND :hi",
			"",
			nil,
			map[string]AttributeValue{
				testExprPK: {S: ptr("tenant1")},
				":lo":      {S: ptr("200")},
				":hi":      {S: ptr("300")},
			},
			0, nil, true)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 2 {
			t.Errorf("got %d results, want 2 (SK BETWEEN 200 AND 300 should match 200, 300)", len(results))
		}
	})

	t.Run("partition key only returns all items for that key", func(t *testing.T) {
		t.Parallel()

		results, _, _, err := s.Query(ctx, "test-query-keycond", "",
			"PK = :pk",
			"",
			nil,
			map[string]AttributeValue{
				testExprPK: {S: ptr("tenant1")},
			},
			0, nil, true)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 4 {
			t.Errorf("got %d results, want 4 (all tenant1 items)", len(results))
		}
	})
}

//nolint:funlen // Test function with setup and multiple subtests.
func TestDeleteItemReturnValues(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName: "test-delete-return",
		KeySchema: []KeySchemaElement{
			{AttributeName: "PK", KeyType: keyTypeHash},
			{AttributeName: "SK", KeyType: keyTypeRange},
		},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "PK", AttributeType: "S"},
			{AttributeName: "SK", AttributeType: "S"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert an item.
	_, err = s.PutItem(ctx, "test-delete-return", Item{
		"PK":   {S: ptr("pk1")},
		"SK":   {S: ptr("sk1")},
		"Data": {S: ptr("value")},
	}, false, ConditionInput{})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("DeleteItem with ALL_OLD returns old item", func(t *testing.T) {
		t.Parallel()

		oldItem, err := s.DeleteItem(ctx, "test-delete-return",
			Item{"PK": {S: ptr("pk1")}, "SK": {S: ptr("sk1")}},
			true, // returnOld = true (ALL_OLD)
			ConditionInput{},
		)
		if err != nil {
			t.Fatal(err)
		}

		if oldItem == nil {
			t.Fatal("oldItem should not be nil when item existed")
		}

		if oldItem["Data"].S == nil || *oldItem["Data"].S != "value" {
			t.Errorf("oldItem[Data] = %v, want value", oldItem["Data"])
		}
	})

	t.Run("DeleteItem non-existent returns nil", func(t *testing.T) {
		t.Parallel()

		oldItem, err := s.DeleteItem(ctx, "test-delete-return",
			Item{"PK": {S: ptr("pk1")}, "SK": {S: ptr("sk-nonexist")}},
			true,
			ConditionInput{},
		)
		if err != nil {
			t.Fatal(err)
		}

		if oldItem != nil {
			t.Errorf("oldItem should be nil for non-existent item, got %v", oldItem)
		}
	})
}
