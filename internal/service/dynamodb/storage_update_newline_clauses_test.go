package dynamodb

import (
	"context"
	"testing"
)

// The aws-sdk-go-v2 expression builder joins clauses with a newline, so a
// combined update reached the parser as "ADD Values :v\nREMOVE Flag" and the
// second clause was not recognised at all.
func TestUpdateItemSplitsClausesOnNewlines(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName: "test-newline-clauses",
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

	key := Item{"pk": {S: new("newline-clauses")}}
	_, err = s.PutItem(ctx, "test-newline-clauses", Item{
		"pk":    {S: new("newline-clauses")},
		"Flag":  {BOOL: new(true)},
		"Items": {SS: []string{"a"}},
	}, false, ConditionInput{})
	if err != nil {
		t.Fatal(err)
	}

	// the exact shape the SDK builder produces: placeholder names, a newline
	// between the clauses and one at the end
	result, err := s.UpdateItem(
		ctx,
		"test-newline-clauses",
		key,
		"ADD #0 :0\nREMOVE #1\n",
		map[string]string{"#0": "Items", "#1": "Flag"},
		map[string]AttributeValue{":0": {SS: []string{"b"}}},
		ReturnValuesAllNew,
		ConditionInput{},
	)
	if err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	if _, ok := result["Flag"]; ok {
		t.Errorf("Flag was not removed: %v", result["Flag"])
	}

	got := result["Items"].SS
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("Items should hold a and b, got %v", got)
	}
}
