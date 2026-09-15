package dynamodb

import (
	"context"
	"testing"
)

//nolint:gocognit,cyclop,funlen // Test function exercises multiple SET arithmetic scenarios sequentially.
func TestSetArithmetic(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName: "test-arithmetic",
		KeySchema: []KeySchemaElement{
			{AttributeName: "pk", KeyType: keyTypeHash},
		},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "pk", AttributeType: "S"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("SET path = path + :val", func(t *testing.T) {
		t.Parallel()

		key := Item{"pk": {S: ptr("arith-add")}}

		// Insert initial item with Counter=5.
		_, err := s.PutItem(ctx, "test-arithmetic", Item{
			"pk":            {S: ptr("arith-add")},
			testAttrCounter: {N: ptr("5")},
		}, false, ConditionInput{})
		if err != nil {
			t.Fatal(err)
		}

		// UpdateItem: SET Counter = Counter + :incr
		result, err := s.UpdateItem(ctx, "test-arithmetic", key,
			"SET Counter = Counter + :incr",
			nil,
			map[string]AttributeValue{testExprIncr: {N: ptr("3")}},
			ReturnValuesAllNew,
			ConditionInput{},
		)
		if err != nil {
			t.Fatal(err)
		}

		if result[testAttrCounter].N == nil || *result[testAttrCounter].N != "8" {
			t.Errorf("Counter = %v, want 8", result[testAttrCounter])
		}
	})

	t.Run("SET path = path - :val", func(t *testing.T) {
		t.Parallel()

		key := Item{"pk": {S: ptr("arith-sub")}}

		_, err := s.PutItem(ctx, "test-arithmetic", Item{
			"pk":            {S: ptr("arith-sub")},
			testAttrCounter: {N: ptr("10")},
		}, false, ConditionInput{})
		if err != nil {
			t.Fatal(err)
		}

		result, err := s.UpdateItem(ctx, "test-arithmetic", key,
			"SET Counter = Counter - :decr",
			nil,
			map[string]AttributeValue{":decr": {N: ptr("4")}},
			ReturnValuesAllNew,
			ConditionInput{},
		)
		if err != nil {
			t.Fatal(err)
		}

		if result[testAttrCounter].N == nil || *result[testAttrCounter].N != "6" {
			t.Errorf("Counter = %v, want 6", result[testAttrCounter])
		}
	})

	t.Run("SET path = if_not_exists(path, :default) + :incr (new item)", func(t *testing.T) {
		t.Parallel()

		key := Item{"pk": {S: ptr("arith-ifne-new")}}

		result, err := s.UpdateItem(ctx, "test-arithmetic", key,
			"SET #count = if_not_exists(#count, :zero) + :incr",
			map[string]string{testExprHashCount: testAttrCounter},
			map[string]AttributeValue{
				testExprZero: {N: ptr("0")},
				testExprIncr: {N: ptr("1")},
			},
			ReturnValuesAllNew,
			ConditionInput{},
		)
		if err != nil {
			t.Fatal(err)
		}

		if result[testAttrCounter].N == nil || *result[testAttrCounter].N != "1" {
			t.Errorf("Counter = %v, want 1", result[testAttrCounter])
		}

		// Second call should increment to 2.
		result, err = s.UpdateItem(ctx, "test-arithmetic", key,
			"SET #count = if_not_exists(#count, :zero) + :incr",
			map[string]string{testExprHashCount: testAttrCounter},
			map[string]AttributeValue{
				testExprZero: {N: ptr("0")},
				testExprIncr: {N: ptr("1")},
			},
			ReturnValuesAllNew,
			ConditionInput{},
		)
		if err != nil {
			t.Fatal(err)
		}

		if result[testAttrCounter].N == nil || *result[testAttrCounter].N != "2" {
			t.Errorf("Counter = %v, want 2", result[testAttrCounter])
		}
	})

	t.Run("SET with multiple assignments including if_not_exists + arithmetic", func(t *testing.T) {
		t.Parallel()

		key := Item{"pk": {S: ptr("arith-multi")}}

		result, err := s.UpdateItem(ctx, "test-arithmetic", key,
			"SET #count = if_not_exists(#count, :zero) + :incr, ExpiresAt = :exp",
			map[string]string{testExprHashCount: testAttrCounter},
			map[string]AttributeValue{
				testExprZero: {N: ptr("0")},
				testExprIncr: {N: ptr("1")},
				":exp":       {N: ptr("9999")},
			},
			ReturnValuesAllNew,
			ConditionInput{},
		)
		if err != nil {
			t.Fatal(err)
		}

		if result[testAttrCounter].N == nil || *result[testAttrCounter].N != "1" {
			t.Errorf("Counter = %v, want 1", result[testAttrCounter])
		}

		if result["ExpiresAt"].N == nil || *result["ExpiresAt"].N != "9999" {
			t.Errorf("ExpiresAt = %v, want 9999", result["ExpiresAt"])
		}
	})
}
