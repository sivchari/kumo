package dynamodb

import (
	"context"
	"testing"
)

// DynamoDB numbers carry up to 38 digits, so SET arithmetic and ADD must not
// round through float64.
func TestUpdateNumbersAreExact(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName: "test-exact-number",
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

	key := Item{"pk": {S: new("a")}}

	_, err = s.PutItem(ctx, "test-exact-number", Item{
		"pk":      {S: new("a")},
		"balance": {N: new("10.10")},
		"id":      {N: new("9007199254740993")},
	}, false, ConditionInput{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := s.UpdateItem(ctx, "test-exact-number", key,
		"SET balance = balance + :b ADD id :one",
		nil,
		map[string]AttributeValue{":b": {N: new("0.20")}, ":one": {N: new("1")}},
		ReturnValuesAllNew,
		ConditionInput{},
	)
	if err != nil {
		t.Fatal(err)
	}

	if got := *result["balance"].N; got != "10.3" {
		t.Errorf("balance = %s, want 10.3", got)
	}

	if got := *result["id"].N; got != "9007199254740994" {
		t.Errorf("id = %s, want 9007199254740994", got)
	}
}
