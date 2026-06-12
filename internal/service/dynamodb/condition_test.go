package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

//nolint:cyclop,funlen // Test function exercises multiple storage operations sequentially.
func TestConditionThroughStorage(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName: "test",
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

	// PutItem with attribute_not_exists should succeed on new item.
	_, err = s.PutItem(ctx, "test", Item{
		"pk":      {S: ptr("1")},
		"version": {N: ptr("1")},
		"status":  {S: ptr("active")},
	}, false, ConditionInput{Expression: "attribute_not_exists(pk)"})
	if err != nil {
		t.Fatalf("first put should succeed: %v", err)
	}

	// PutItem with attribute_not_exists should fail on existing item.
	_, err = s.PutItem(ctx, "test", Item{
		"pk":      {S: ptr("1")},
		"version": {N: ptr("99")},
	}, false, ConditionInput{Expression: "attribute_not_exists(pk)"})
	if err == nil {
		t.Fatal("second put should fail")
	}

	var tErr *TableError
	if !errors.As(err, &tErr) || tErr.Code != ErrCodeConditionalCheckFailed {
		t.Fatalf("expected ConditionalCheckFailedException, got: %v", err)
	}

	// Verify original item preserved.
	item, err := s.GetItem(ctx, "test", Item{"pk": {S: ptr("1")}})
	if err != nil {
		t.Fatal(err)
	}

	if item["version"].N == nil || *item["version"].N != "1" {
		t.Fatalf("version should be 1, got: %v", item["version"])
	}

	// UpdateItem with version check (optimistic locking).
	_, err = s.UpdateItem(ctx, "test", Item{"pk": {S: ptr("1")}},
		"SET version = :new", nil,
		map[string]AttributeValue{
			":cur": {N: ptr("1")},
			":new": {N: ptr("2")},
		},
		ReturnValuesAllNew,
		ConditionInput{
			Expression: "version = :cur",
			ExprValues: map[string]AttributeValue{":cur": {N: ptr("1")}},
		},
	)
	if err != nil {
		t.Fatalf("update with correct version should succeed: %v", err)
	}

	// UpdateItem with stale version should fail.
	_, err = s.UpdateItem(ctx, "test", Item{"pk": {S: ptr("1")}},
		"SET version = :new", nil,
		map[string]AttributeValue{
			":cur": {N: ptr("1")},
			":new": {N: ptr("3")},
		},
		"",
		ConditionInput{
			Expression: "version = :cur",
			ExprValues: map[string]AttributeValue{":cur": {N: ptr("1")}},
		},
	)
	if err == nil {
		t.Fatal("update with stale version should fail")
	}

	// Comparison operators: version >= 2 should pass.
	_, err = s.UpdateItem(ctx, "test", Item{"pk": {S: ptr("1")}},
		"SET version = :new", nil,
		map[string]AttributeValue{
			":new": {N: ptr("3")},
		},
		ReturnValuesAllNew,
		ConditionInput{
			Expression: "version >= :min",
			ExprValues: map[string]AttributeValue{":min": {N: ptr("2")}},
		},
	)
	if err != nil {
		t.Fatalf("update with version >= 2 should succeed: %v", err)
	}

	// Comparison: version < 2 should fail (version is now 3).
	_, err = s.UpdateItem(ctx, "test", Item{"pk": {S: ptr("1")}},
		"SET version = :new", nil,
		map[string]AttributeValue{":new": {N: ptr("4")}},
		"",
		ConditionInput{
			Expression: "version < :max",
			ExprValues: map[string]AttributeValue{":max": {N: ptr("2")}},
		},
	)
	if err == nil {
		t.Fatal("update with version < 2 should fail (version=3)")
	}

	// String comparison: status = "active" AND version > 1.
	_, err = s.UpdateItem(ctx, "test", Item{"pk": {S: ptr("1")}},
		"SET #s = :new_status", map[string]string{"#s": "status"},
		map[string]AttributeValue{":new_status": {S: ptr("done")}},
		ReturnValuesAllNew,
		ConditionInput{
			Expression: "#s = :expected AND version > :min_ver",
			ExprNames:  map[string]string{"#s": "status"},
			ExprValues: map[string]AttributeValue{
				":expected": {S: ptr("active")},
				":min_ver":  {N: ptr("1")},
			},
		},
	)
	if err != nil {
		t.Fatalf("compound condition should succeed: %v", err)
	}

	// DeleteItem with wrong condition should fail.
	_, err = s.DeleteItem(ctx, "test", Item{"pk": {S: ptr("1")}}, false, ConditionInput{
		Expression: "status = :expected",
		ExprValues: map[string]AttributeValue{":expected": {S: ptr("active")}},
	})
	if err == nil {
		t.Fatal("delete with wrong status should fail (status=done)")
	}

	// DeleteItem with correct condition should succeed.
	_, err = s.DeleteItem(ctx, "test", Item{"pk": {S: ptr("1")}}, false, ConditionInput{
		Expression: "status = :expected",
		ExprValues: map[string]AttributeValue{":expected": {S: ptr("done")}},
	})
	if err != nil {
		t.Fatalf("delete with correct status should succeed: %v", err)
	}

	// Verify deleted.
	item, err = s.GetItem(ctx, "test", Item{"pk": {S: ptr("1")}})
	if err != nil {
		t.Fatal(err)
	}

	if item != nil {
		t.Fatalf("item should be deleted, got: %v", item)
	}
}

//nolint:cyclop,funlen // Test function exercises multiple transaction operations sequentially.
func TestTransactWriteItems(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName:            "accounts",
		KeySchema:            []KeySchemaElement{{AttributeName: "id", KeyType: "HASH"}},
		AttributeDefinitions: []AttributeDefinition{{AttributeName: "id", AttributeType: "S"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Successful transaction: put two items atomically.
	reasons, err := s.TransactWriteItems(ctx, []TransactWriteItem{
		{Put: &TransactPut{
			TableName:           "accounts",
			Item:                Item{"id": {S: ptr("acc-1")}, "balance": {N: ptr("100")}},
			ConditionExpression: "attribute_not_exists(id)",
		}},
		{Put: &TransactPut{
			TableName:           "accounts",
			Item:                Item{"id": {S: ptr("acc-2")}, "balance": {N: ptr("200")}},
			ConditionExpression: "attribute_not_exists(id)",
		}},
	})
	if err != nil {
		t.Fatalf("transaction should succeed: %v", err)
	}

	if reasons != nil {
		t.Fatalf("expected no cancellation reasons, got: %v", reasons)
	}

	// Verify both items exist.
	item1, _ := s.GetItem(ctx, "accounts", Item{"id": {S: ptr("acc-1")}})
	item2, _ := s.GetItem(ctx, "accounts", Item{"id": {S: ptr("acc-2")}})

	if item1 == nil || item2 == nil {
		t.Fatal("both items should exist after transaction")
	}

	// Failed transaction: second put has condition that fails.
	reasons, err = s.TransactWriteItems(ctx, []TransactWriteItem{
		{Put: &TransactPut{
			TableName:           "accounts",
			Item:                Item{"id": {S: ptr("acc-3")}, "balance": {N: ptr("300")}},
			ConditionExpression: "attribute_not_exists(id)",
		}},
		{Put: &TransactPut{
			TableName:           "accounts",
			Item:                Item{"id": {S: ptr("acc-1")}, "balance": {N: ptr("999")}},
			ConditionExpression: "attribute_not_exists(id)", // Fails: acc-1 already exists.
		}},
	})

	var tErr *TableError
	if !errors.As(err, &tErr) || tErr.Code != "TransactionCanceledException" {
		t.Fatalf("expected TransactionCanceledException, got: %v", err)
	}

	// Verify reasons: first should be empty, second should be ConditionalCheckFailed.
	if reasons[0].Code != "" {
		t.Fatalf("first reason should be empty, got: %s", reasons[0].Code)
	}

	if reasons[1].Code != "ConditionalCheckFailed" {
		t.Fatalf("second reason should be ConditionalCheckFailed, got: %s", reasons[1].Code)
	}

	// Verify acc-3 was NOT created (all-or-nothing).
	item3, _ := s.GetItem(ctx, "accounts", Item{"id": {S: ptr("acc-3")}})
	if item3 != nil {
		t.Fatal("acc-3 should not exist after failed transaction")
	}

	// Transaction with Update and ConditionCheck.
	_, err = s.TransactWriteItems(ctx, []TransactWriteItem{
		{Update: &TransactUpdate{
			TableName:        "accounts",
			Key:              Item{"id": {S: ptr("acc-1")}},
			UpdateExpression: "SET balance = :new",
			ExpressionAttributeValues: map[string]AttributeValue{
				":new": {N: ptr("150")},
			},
		}},
		{ConditionCheck: &TransactConditionCheck{
			TableName:           "accounts",
			Key:                 Item{"id": {S: ptr("acc-2")}},
			ConditionExpression: "attribute_exists(id)",
		}},
	})
	if err != nil {
		t.Fatalf("update+conditioncheck transaction should succeed: %v", err)
	}

	// Verify acc-1 balance updated.
	item1, _ = s.GetItem(ctx, "accounts", Item{"id": {S: ptr("acc-1")}})
	if item1["balance"].N == nil || *item1["balance"].N != "150" {
		t.Fatalf("acc-1 balance should be 150, got: %v", item1["balance"])
	}

	// Transaction with Delete.
	_, err = s.TransactWriteItems(ctx, []TransactWriteItem{
		{Delete: &TransactDelete{
			TableName: "accounts",
			Key:       Item{"id": {S: ptr("acc-2")}},
		}},
	})
	if err != nil {
		t.Fatalf("delete transaction should succeed: %v", err)
	}

	item2, _ = s.GetItem(ctx, "accounts", Item{"id": {S: ptr("acc-2")}})
	if item2 != nil {
		t.Fatal("acc-2 should be deleted")
	}
}

func TestTransactGetItems(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName:            "users",
		KeySchema:            []KeySchemaElement{{AttributeName: "pk", KeyType: "HASH"}},
		AttributeDefinitions: []AttributeDefinition{{AttributeName: "pk", AttributeType: "S"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert items.
	for _, id := range []string{"u1", "u2", "u3"} {
		_, err = s.PutItem(ctx, "users", Item{"pk": {S: ptr(id)}, "name": {S: ptr("User-" + id)}}, false, ConditionInput{})
		if err != nil {
			t.Fatal(err)
		}
	}

	// TransactGetItems: get u1 and u3 (skip u2).
	items, err := s.TransactGetItems(ctx, []TransactGetItem{
		{Get: &TransactGet{TableName: "users", Key: Item{"pk": {S: ptr("u1")}}}},
		{Get: &TransactGet{TableName: "users", Key: Item{"pk": {S: ptr("u3")}}}},
		{Get: &TransactGet{TableName: "users", Key: Item{"pk": {S: ptr("missing")}}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 results, got %d", len(items))
	}

	if items[0] == nil || *items[0]["name"].S != "User-u1" {
		t.Fatalf("first item should be u1, got: %v", items[0])
	}

	if items[1] == nil || *items[1]["name"].S != "User-u3" {
		t.Fatalf("second item should be u3, got: %v", items[1])
	}

	if items[2] != nil {
		t.Fatalf("third item should be nil (not found), got: %v", items[2])
	}
}

//nolint:funlen // Table-driven test with comprehensive condition expression coverage.
func TestEvaluateCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		item    Item
		cond    ConditionInput
		want    bool
		wantErr bool
	}{
		{
			name: "empty expression returns true",
			item: Item{"pk": {S: ptr("1")}},
			cond: ConditionInput{},
			want: true,
		},
		{
			name: "attribute_exists succeeds when attribute present",
			item: Item{"pk": {S: ptr("1")}, "name": {S: ptr("Alice")}},
			cond: ConditionInput{
				Expression: "attribute_exists(name)",
			},
			want: true,
		},
		{
			name: "attribute_exists fails when attribute missing",
			item: Item{"pk": {S: ptr("1")}},
			cond: ConditionInput{
				Expression: "attribute_exists(name)",
			},
			want: false,
		},
		{
			name: "attribute_not_exists succeeds when attribute missing",
			item: Item{"pk": {S: ptr("1")}},
			cond: ConditionInput{
				Expression: "attribute_not_exists(name)",
			},
			want: true,
		},
		{
			name: "attribute_not_exists on nil item (new item)",
			item: nil,
			cond: ConditionInput{
				Expression: "attribute_not_exists(pk)",
			},
			want: true,
		},
		{
			name: "attribute_not_exists fails when attribute present",
			item: Item{"pk": {S: ptr("1")}, "name": {S: ptr("Alice")}},
			cond: ConditionInput{
				Expression: "attribute_not_exists(name)",
			},
			want: false,
		},
		{
			name: "string equality",
			item: Item{"pk": {S: ptr("1")}, "status": {S: ptr("active")}},
			cond: ConditionInput{
				Expression: "status = :val",
				ExprValues: map[string]AttributeValue{
					":val": {S: ptr("active")},
				},
			},
			want: true,
		},
		{
			name: "string inequality",
			item: Item{"pk": {S: ptr("1")}, "status": {S: ptr("active")}},
			cond: ConditionInput{
				Expression: "status <> :val",
				ExprValues: map[string]AttributeValue{
					":val": {S: ptr("inactive")},
				},
			},
			want: true,
		},
		{
			name: "number comparison less than",
			item: Item{"pk": {S: ptr("1")}, "age": {N: ptr("25")}},
			cond: ConditionInput{
				Expression: "age < :val",
				ExprValues: map[string]AttributeValue{
					":val": {N: ptr("30")},
				},
			},
			want: true,
		},
		{
			name: "number comparison greater equal",
			item: Item{"pk": {S: ptr("1")}, "age": {N: ptr("25")}},
			cond: ConditionInput{
				Expression: "age >= :val",
				ExprValues: map[string]AttributeValue{
					":val": {N: ptr("25")},
				},
			},
			want: true,
		},
		{
			name: "AND both true",
			item: Item{"pk": {S: ptr("1")}, "status": {S: ptr("active")}, "age": {N: ptr("25")}},
			cond: ConditionInput{
				Expression: "status = :status AND age >= :age",
				ExprValues: map[string]AttributeValue{
					":status": {S: ptr("active")},
					":age":    {N: ptr("20")},
				},
			},
			want: true,
		},
		{
			name: "AND left false",
			item: Item{"pk": {S: ptr("1")}, "status": {S: ptr("inactive")}, "age": {N: ptr("25")}},
			cond: ConditionInput{
				Expression: "status = :status AND age >= :age",
				ExprValues: map[string]AttributeValue{
					":status": {S: ptr("active")},
					":age":    {N: ptr("20")},
				},
			},
			want: false,
		},
		{
			name: "OR one true",
			item: Item{"pk": {S: ptr("1")}, "status": {S: ptr("inactive")}},
			cond: ConditionInput{
				Expression: "status = :s1 OR status = :s2",
				ExprValues: map[string]AttributeValue{
					":s1": {S: ptr("active")},
					":s2": {S: ptr("inactive")},
				},
			},
			want: true,
		},
		{
			name: "NOT expression",
			item: Item{"pk": {S: ptr("1")}, "status": {S: ptr("active")}},
			cond: ConditionInput{
				Expression: "NOT status = :val",
				ExprValues: map[string]AttributeValue{
					":val": {S: ptr("inactive")},
				},
			},
			want: true,
		},
		{
			name: "expression attribute names",
			item: Item{"pk": {S: ptr("1")}, "status": {S: ptr("active")}},
			cond: ConditionInput{
				Expression: "#s = :val",
				ExprNames:  map[string]string{"#s": "status"},
				ExprValues: map[string]AttributeValue{
					":val": {S: ptr("active")},
				},
			},
			want: true,
		},
		{
			name: "begins_with true",
			item: Item{"pk": {S: ptr("1")}, "email": {S: ptr("alice@example.com")}},
			cond: ConditionInput{
				Expression: "begins_with(email, :prefix)",
				ExprValues: map[string]AttributeValue{
					":prefix": {S: ptr("alice@")},
				},
			},
			want: true,
		},
		{
			name: "begins_with false",
			item: Item{"pk": {S: ptr("1")}, "email": {S: ptr("bob@example.com")}},
			cond: ConditionInput{
				Expression: "begins_with(email, :prefix)",
				ExprValues: map[string]AttributeValue{
					":prefix": {S: ptr("alice@")},
				},
			},
			want: false,
		},
		{
			name: "contains string",
			item: Item{"pk": {S: ptr("1")}, "name": {S: ptr("Alice Smith")}},
			cond: ConditionInput{
				Expression: "contains(name, :sub)",
				ExprValues: map[string]AttributeValue{
					":sub": {S: ptr("Smith")},
				},
			},
			want: true,
		},
		{
			name: "contains string set",
			item: Item{"pk": {S: ptr("1")}, "tags": {SS: []string{"golang", "rust", "python"}}},
			cond: ConditionInput{
				Expression: "contains(tags, :tag)",
				ExprValues: map[string]AttributeValue{
					":tag": {S: ptr("rust")},
				},
			},
			want: true,
		},
		{
			name: "contains string set missing",
			item: Item{"pk": {S: ptr("1")}, "tags": {SS: []string{"golang", "rust"}}},
			cond: ConditionInput{
				Expression: "contains(tags, :tag)",
				ExprValues: map[string]AttributeValue{
					":tag": {S: ptr("java")},
				},
			},
			want: false,
		},
		{
			name: "size comparison",
			item: Item{"pk": {S: ptr("1")}, "items": {L: []*AttributeValue{{S: ptr("a")}, {S: ptr("b")}, {S: ptr("c")}}}},
			cond: ConditionInput{
				Expression: "size(items) > :min",
				ExprValues: map[string]AttributeValue{
					":min": {N: ptr("2")},
				},
			},
			want: true,
		},
		{
			name: "size comparison fails",
			item: Item{"pk": {S: ptr("1")}, "items": {L: []*AttributeValue{{S: ptr("a")}}}},
			cond: ConditionInput{
				Expression: "size(items) > :min",
				ExprValues: map[string]AttributeValue{
					":min": {N: ptr("2")},
				},
			},
			want: false,
		},
		{
			name: "parenthesized expression",
			item: Item{"pk": {S: ptr("1")}, "a": {N: ptr("1")}, "b": {N: ptr("2")}},
			cond: ConditionInput{
				Expression: "(a = :v1) AND (b = :v2)",
				ExprValues: map[string]AttributeValue{
					":v1": {N: ptr("1")},
					":v2": {N: ptr("2")},
				},
			},
			want: true,
		},
		{
			name: "idempotency pattern: attribute_not_exists on pk",
			item: Item{"pk": {S: ptr("existing-id")}, "data": {S: ptr("value")}},
			cond: ConditionInput{
				Expression: "attribute_not_exists(pk)",
			},
			want: false,
		},
		{
			name: "nested path attribute_exists",
			item: Item{"pk": {S: ptr("1")}, "meta": {M: map[string]*AttributeValue{"version": {N: ptr("1")}}}},
			cond: ConditionInput{
				Expression: "attribute_exists(meta.version)",
			},
			want: true,
		},
		{
			name: "attribute_type matches string",
			item: Item{"pk": {S: ptr("1")}, "name": {S: ptr("Alice")}},
			cond: ConditionInput{
				Expression: "attribute_type(name, :t)",
				ExprValues: map[string]AttributeValue{":t": {S: ptr("S")}},
			},
			want: true,
		},
		{
			name: "attribute_type mismatch returns false",
			item: Item{"pk": {S: ptr("1")}, "name": {S: ptr("Alice")}},
			cond: ConditionInput{
				Expression: "attribute_type(name, :t)",
				ExprValues: map[string]AttributeValue{":t": {S: ptr("N")}},
			},
			want: false,
		},
		{
			name: "attribute_type on list",
			item: Item{"pk": {S: ptr("1")}, "tags": {L: []*AttributeValue{{S: ptr("a")}}}},
			cond: ConditionInput{
				Expression: "attribute_type(tags, :t)",
				ExprValues: map[string]AttributeValue{":t": {S: ptr("L")}},
			},
			want: true,
		},
		{
			name: "attribute_type on missing attribute returns false (no error)",
			item: Item{"pk": {S: ptr("1")}},
			cond: ConditionInput{
				Expression: "attribute_type(name, :t)",
				ExprValues: map[string]AttributeValue{":t": {S: ptr("S")}},
			},
			want: false,
		},
		{
			name: "attribute_type with non-string type operand errors",
			item: Item{"pk": {S: ptr("1")}, "name": {S: ptr("Alice")}},
			cond: ConditionInput{
				Expression: "attribute_type(name, :t)",
				ExprValues: map[string]AttributeValue{":t": {N: ptr("1")}},
			},
			wantErr: true,
		},
		{
			name: "size on missing attribute returns false (no error)",
			item: Item{"pk": {S: ptr("1")}},
			cond: ConditionInput{
				Expression: "size(items) > :min",
				ExprValues: map[string]AttributeValue{":min": {N: ptr("2")}},
			},
			want: false,
		},
		{
			name: "size compared to attribute operand matches",
			item: Item{"pk": {S: ptr("1")}, "items": {L: []*AttributeValue{{S: ptr("a")}, {S: ptr("b")}}}, "minLen": {N: ptr("1")}},
			cond: ConditionInput{
				Expression: "size(items) > minLen",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := evaluateCondition(tt.item, tt.cond)
			if (err != nil) != tt.wantErr {
				t.Errorf("evaluateCondition() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.want {
				t.Errorf("evaluateCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

//nolint:funlen // Table-driven test covering the IN operator cases.
func TestEvaluateConditionIn(t *testing.T) {
	t.Parallel()

	item := Item{
		"pk":     {S: ptr("1")},
		"status": {S: ptr("running")},
		"count":  {N: ptr("5")},
		"other":  {S: ptr("running")},
	}

	tests := []struct {
		name    string
		expr    string
		values  map[string]AttributeValue
		want    bool
		wantErr bool
	}{
		{
			name:   "IN matches first operand",
			expr:   "status IN (:a, :b)",
			values: map[string]AttributeValue{":a": {S: ptr("running")}, ":b": {S: ptr("pending")}},
			want:   true,
		},
		{
			name:   "IN matches later operand",
			expr:   "status IN (:a, :b, :c)",
			values: map[string]AttributeValue{":a": {S: ptr("pending")}, ":b": {S: ptr("done")}, ":c": {S: ptr("running")}},
			want:   true,
		},
		{
			name:   "IN with no matching operand",
			expr:   "status IN (:a, :b)",
			values: map[string]AttributeValue{":a": {S: ptr("pending")}, ":b": {S: ptr("done")}},
			want:   false,
		},
		{
			name:   "IN with attribute path operand",
			expr:   "status IN (other)",
			values: nil,
			want:   true,
		},
		{
			name:   "IN with numeric operands",
			expr:   "count IN (:a, :b)",
			values: map[string]AttributeValue{":a": {N: ptr("3")}, ":b": {N: ptr("5")}},
			want:   true,
		},
		{
			name:   "lowercase in keyword",
			expr:   "status in (:a)",
			values: map[string]AttributeValue{":a": {S: ptr("running")}},
			want:   true,
		},
		{
			name:   "NOT IN composition",
			expr:   "NOT status IN (:a, :b)",
			values: map[string]AttributeValue{":a": {S: ptr("pending")}, ":b": {S: ptr("done")}},
			want:   true,
		},
		{
			name:   "IN combined with AND",
			expr:   "pk = :pk AND status IN (:a, :b)",
			values: map[string]AttributeValue{":pk": {S: ptr("1")}, ":a": {S: ptr("running")}, ":b": {S: ptr("done")}},
			want:   true,
		},
		{
			name:   "parenthesized IN in OR chain",
			expr:   "(status IN (:a)) OR count = :n",
			values: map[string]AttributeValue{":a": {S: ptr("done")}, ":n": {N: ptr("5")}},
			want:   true,
		},
		{
			name:    "IN without parenthesized list",
			expr:    "status IN :a",
			values:  map[string]AttributeValue{":a": {S: ptr("running")}},
			wantErr: true,
		},
		{
			name:    "IN with empty operand list",
			expr:    "status IN ()",
			values:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := evaluateCondition(item, ConditionInput{
				Expression: tt.expr,
				ExprValues: tt.values,
			})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got result %v", tt.expr, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.expr, err)
			}

			if got != tt.want {
				t.Fatalf("evaluateCondition(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestEvaluateConditionInOperandLimit(t *testing.T) {
	t.Parallel()

	item := Item{"status": {S: ptr("running")}}

	// Build an IN list with one operand over the DynamoDB limit.
	operands := make([]string, 0, maxInOperands+1)
	values := make(map[string]AttributeValue, maxInOperands+1)

	for i := range maxInOperands + 1 {
		ph := fmt.Sprintf(":v%d", i)
		operands = append(operands, ph)
		values[ph] = AttributeValue{S: ptr(fmt.Sprintf("state%d", i))}
	}

	_, err := evaluateCondition(item, ConditionInput{
		Expression: "status IN (" + strings.Join(operands, ", ") + ")",
		ExprValues: values,
	})
	if err == nil {
		t.Fatalf("expected error for IN list with %d operands", maxInOperands+1)
	}
}

// TestFilterExpressionFailsClosed verifies that Scan and Query reject an
// unparseable FilterExpression with ValidationException instead of silently
// matching every item (or none).
//
//nolint:funlen // Exercises the Scan and Query fail-closed paths sequentially.
func TestFilterExpressionFailsClosed(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName: "filter-test",
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

	for i, status := range []string{"pending", "running", "done"} {
		_, err := s.PutItem(ctx, "filter-test", Item{
			"pk":     {S: ptr(fmt.Sprintf("%d", i))},
			"status": {S: ptr(status)},
		}, false, ConditionInput{})
		if err != nil {
			t.Fatal(err)
		}
	}

	// IN now filters correctly on Scan.
	items, _, scanned, err := s.Scan(ctx, "filter-test", "#s IN (:a, :b)",
		map[string]string{"#s": "status"},
		map[string]AttributeValue{":a": {S: ptr("pending")}, ":b": {S: ptr("running")}},
		0, nil, nil, nil)
	if err != nil {
		t.Fatalf("scan with IN filter: %v", err)
	}

	if len(items) != 2 || scanned != 3 {
		t.Fatalf("expected 2 of 3 items to match IN filter, got %d of %d", len(items), scanned)
	}

	const errCodeValidation = "ValidationException"

	// An unparseable filter is a ValidationException, not match-all.
	items, _, scanned, err = s.Scan(ctx, "filter-test", "complete garbage !!!",
		nil, nil, 0, nil, nil, nil)

	var tErr *TableError
	if !errors.As(err, &tErr) || tErr.Code != errCodeValidation {
		t.Fatalf("expected ValidationException for invalid scan filter, got: %v", err)
	}

	if len(items) != 0 || scanned != 0 {
		t.Fatalf("invalid filter must not return results, got %d items (scanned %d)", len(items), scanned)
	}

	// Query takes the same path.
	queryItems, _, queryScanned, err := s.Query(ctx, "filter-test", "", "pk = :pk", "complete garbage !!!",
		nil, map[string]AttributeValue{":pk": {S: ptr("0")}}, 0, nil, true)

	if !errors.As(err, &tErr) || tErr.Code != errCodeValidation {
		t.Fatalf("expected ValidationException for invalid query filter, got: %v", err)
	}

	if len(queryItems) != 0 || queryScanned != 0 {
		t.Fatalf("invalid filter must not return results, got %d items (scanned %d)", len(queryItems), queryScanned)
	}
}

// TestExpressionValidationEdgeCases covers the follow-up gaps to PR #808: an
// invalid filter must be rejected even on an empty table (not only when an item
// is scanned), and an unparseable KeyConditionExpression must be a
// ValidationException rather than a silent empty result.
func TestExpressionValidationEdgeCases(t *testing.T) {
	t.Parallel()

	const errCodeValidation = "ValidationException"

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName:            "edge",
		KeySchema:            []KeySchemaElement{{AttributeName: "pk", KeyType: "HASH"}},
		AttributeDefinitions: []AttributeDefinition{{AttributeName: "pk", AttributeType: "S"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var tErr *TableError

	// Empty table: an invalid filter must still be a ValidationException, not an
	// empty 200 result (the table has no items, so the per-item loop never runs).
	if _, _, _, err := s.Scan(ctx, "edge", "complete garbage !!!", nil, nil, 0, nil, nil, nil); !errors.As(err, &tErr) || tErr.Code != errCodeValidation {
		t.Fatalf("empty-table scan with invalid filter: expected ValidationException, got: %v", err)
	}

	if _, _, _, err := s.Query(ctx, "edge", "", "pk = :pk", "complete garbage !!!", nil,
		map[string]AttributeValue{":pk": {S: ptr("missing")}}, 0, nil, true); !errors.As(err, &tErr) || tErr.Code != errCodeValidation {
		t.Fatalf("empty-table query with invalid filter: expected ValidationException, got: %v", err)
	}

	// Unparseable KeyConditionExpression must be a ValidationException, not a
	// silent empty result.
	_, err = s.PutItem(ctx, "edge", Item{"pk": {S: ptr("1")}}, false, ConditionInput{})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, _, err := s.Query(ctx, "edge", "", "complete garbage !!!", "", nil,
		map[string]AttributeValue{":pk": {S: ptr("1")}}, 0, nil, true); !errors.As(err, &tErr) || tErr.Code != errCodeValidation {
		t.Fatalf("query with invalid KeyConditionExpression: expected ValidationException, got: %v", err)
	}
}
