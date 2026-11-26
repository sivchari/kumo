package dynamodb

import "testing"

func TestTransactWriteItemsCancellationReasons(t *testing.T) {
	const table = "cancellation_reasons"

	t.Parallel()

	storage := NewMemoryStorage("http://localhost:4566")

	_, err := storage.CreateTable(t.Context(), &CreateTableRequest{
		TableName:            table,
		KeySchema:            []KeySchemaElement{{AttributeName: "pk", KeyType: "HASH"}},
		AttributeDefinitions: []AttributeDefinition{{AttributeName: "pk", AttributeType: "S"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	passing, failing := "passing", "failing"

	reasons, err := storage.TransactWriteItems(t.Context(), []TransactWriteItem{
		{Put: &TransactPut{TableName: table, Item: Item{"pk": {S: &passing}}}},
		{Put: &TransactPut{TableName: table, Item: Item{"pk": {S: &failing}}, ConditionExpression: "attribute_exists(pk)"}},
	})
	if err == nil {
		t.Fatal("expected the transaction to be canceled")
	}

	if len(reasons) != 2 {
		t.Fatalf("got %d reasons, want 2", len(reasons))
	}

	if reasons[0].Code != "None" || reasons[0].Message != "" {
		t.Errorf("passing item: got %q %q, want code None and no message", reasons[0].Code, reasons[0].Message)
	}

	if reasons[1].Code != "ConditionalCheckFailed" || reasons[1].Message != "The conditional request failed" {
		t.Errorf("failing item: got %q %q", reasons[1].Code, reasons[1].Message)
	}
}
