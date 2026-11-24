package dynamodb

import (
	"context"
	"testing"
)

func TestSetListAppend(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName: "test-list-append",
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

	_, err = s.PutItem(ctx, "test-list-append", Item{
		"pk":         {S: new("a")},
		testAttrTags: {L: []*AttributeValue{{S: new("x")}}},
	}, false, ConditionInput{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := s.UpdateItem(ctx, "test-list-append", key,
		"SET tags = list_append(tags, :v), #l = list_append(if_not_exists(#l, :e), :v)",
		map[string]string{"#l": "log"},
		map[string]AttributeValue{
			":v": {L: []*AttributeValue{{S: new("y")}}},
			":e": {L: []*AttributeValue{}},
		},
		ReturnValuesAllNew,
		ConditionInput{},
	)
	if err != nil {
		t.Fatal(err)
	}

	if tags := result[testAttrTags].L; len(tags) != 2 || *tags[0].S != "x" || *tags[1].S != "y" {
		t.Errorf("tags = %v, want [x y]", result[testAttrTags])
	}

	if log := result["log"].L; len(log) != 1 || *log[0].S != "y" {
		t.Errorf("log = %v, want [y]", result["log"])
	}
}
