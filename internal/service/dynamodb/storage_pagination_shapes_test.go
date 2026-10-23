package dynamodb

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

const (
	testShapesTable = "pagination-shapes"
	testShapesIndex = "shape_index"
	testShapesGroup = "grp"
	testShapesOwner = "owner"
	testShapesLimit = 2

	testProjectionAll = "ALL"
)

// queryPage is one Query call's parameters, reused across pages.
type queryPage struct {
	index       string
	keyCondExpr string
	filterExpr  string
	exprValues  map[string]AttributeValue
	scanForward bool
}

func keyElement(name, keyType string) KeySchemaElement {
	return KeySchemaElement{AttributeName: name, KeyType: keyType}
}

// createShapesTable creates a table with the given key schema and at most
// one index, global or local, then puts items.
func createShapesTable(t *testing.T, tableKey, gsi, lsi []KeySchemaElement, items []Item) *MemoryStorage {
	t.Helper()

	req := &CreateTableRequest{TableName: testShapesTable, KeySchema: tableKey}

	for _, name := range keyNames(tableKey, gsi, lsi) {
		req.AttributeDefinitions = append(req.AttributeDefinitions, AttributeDefinition{AttributeName: name, AttributeType: "S"})
	}

	if gsi != nil {
		req.GlobalSecondaryIndexes = []GlobalSecondaryIndex{{IndexName: testShapesIndex, KeySchema: gsi, Projection: Projection{ProjectionType: testProjectionAll}}}
	}

	if lsi != nil {
		req.LocalSecondaryIndexes = []LocalSecondaryIndex{{IndexName: testShapesIndex, KeySchema: lsi, Projection: Projection{ProjectionType: testProjectionAll}}}
	}

	s := NewMemoryStorage("http://localhost:4566")
	if _, err := s.CreateTable(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	for _, item := range items {
		if _, err := s.PutItem(t.Context(), testShapesTable, item, false, ConditionInput{}); err != nil {
			t.Fatal(err)
		}
	}

	return s
}

// queryAllPages follows LastEvaluatedKey with Limit testShapesLimit until it
// is absent, failing if it never is.
func queryAllPages(t *testing.T, s *MemoryStorage, q queryPage) []Item {
	t.Helper()

	var (
		all      []Item
		startKey Item
	)

	for range 50 {
		items, lastKey, _, err := s.Query(t.Context(), testShapesTable, q.index, q.keyCondExpr, q.filterExpr,
			nil, q.exprValues, testShapesLimit, startKey, q.scanForward)
		if err != nil {
			t.Fatalf("Query: %v", err)
		}

		all = append(all, items...)

		if lastKey == nil {
			return all
		}

		startKey = lastKey
	}

	t.Fatalf("pagination did not terminate, got %d items", len(all))

	return nil
}

// attrValues returns each item's string or number value of name, in order.
func attrValues(items []Item, name string) []string {
	values := make([]string, 0, len(items))

	for _, item := range items {
		v := item[name]

		switch {
		case v.S != nil:
			values = append(values, *v.S)
		case v.N != nil:
			values = append(values, *v.N)
		}
	}

	return values
}

func sItem(attrs ...string) Item {
	item := make(Item, len(attrs)/2)
	for i := 0; i+1 < len(attrs); i += 2 {
		item[attrs[i]] = AttributeValue{S: new(attrs[i+1])}
	}

	return item
}

//nolint:funlen // Table-driven test covering the index and key shapes.
func TestQueryPaginationShapes(t *testing.T) {
	t.Parallel()

	byGroup := map[string]AttributeValue{":g": {S: new(testShapesGroup)}}
	byPK := map[string]AttributeValue{":p": {S: new("p")}}

	tests := []struct {
		name     string
		tableKey []KeySchemaElement
		gsi      []KeySchemaElement
		lsi      []KeySchemaElement
		items    []Item
		query    queryPage
		attr     string
		want     []string
	}{
		{
			name:     "composite table key whose values contain the separator",
			tableKey: []KeySchemaElement{keyElement("pk", keyTypeHash), keyElement("sk", keyTypeRange)},
			gsi:      []KeySchemaElement{keyElement("g", keyTypeHash)},
			items: []Item{
				sItem("pk", "a|b", "sk", "c", "g", testShapesGroup, "id", "1"),
				sItem("pk", "a", "sk", "b|c", "g", testShapesGroup, "id", "2"),
				sItem("pk", "a", "sk", "b", "g", testShapesGroup, "id", "3"),
				sItem("pk", "b", "sk", "a", "g", testShapesGroup, "id", "4"),
				sItem("pk", "a|b", "sk", "a", "g", testShapesGroup, "id", "5"),
			},
			query: queryPage{index: testShapesIndex, keyCondExpr: "g = :g", exprValues: byGroup, scanForward: true},
			attr:  "id",
			want:  []string{"3", "2", "5", "1", "4"},
		},
		{
			name:     "index range key is the table range key",
			tableKey: []KeySchemaElement{keyElement("pk", keyTypeHash), keyElement("sk", keyTypeRange)},
			gsi:      []KeySchemaElement{keyElement("g", keyTypeHash), keyElement("sk", keyTypeRange)},
			items: []Item{
				sItem("pk", "x", "sk", "2", "g", testShapesGroup),
				sItem("pk", "y", "sk", "1", "g", testShapesGroup),
				sItem("pk", "x", "sk", "1", "g", testShapesGroup),
				sItem("pk", "z", "sk", "2", "g", testShapesGroup),
				sItem("pk", "y", "sk", "3", "g", testShapesGroup),
			},
			query: queryPage{index: testShapesIndex, keyCondExpr: "g = :g", exprValues: byGroup, scanForward: false},
			attr:  "pk",
			want:  []string{"y", "z", "x", "y", "x"},
		},
		{
			name:     "local secondary index with duplicate range values",
			tableKey: []KeySchemaElement{keyElement("pk", keyTypeHash), keyElement("sk", keyTypeRange)},
			lsi:      []KeySchemaElement{keyElement("pk", keyTypeHash), keyElement(testShapesOwner, keyTypeRange)},
			items: []Item{
				sItem("pk", "p", "sk", "1", testShapesOwner, "bob"),
				sItem("pk", "p", "sk", "2", testShapesOwner, "amy"),
				sItem("pk", "p", "sk", "3", testShapesOwner, "bob"),
				sItem("pk", "p", "sk", "4", testShapesOwner, "amy"),
				sItem("pk", "p", "sk", "5", testShapesOwner, "amy"),
			},
			query: queryPage{index: testShapesIndex, keyCondExpr: "pk = :p", exprValues: byPK, scanForward: true},
			attr:  "sk",
			want:  []string{"2", "4", "5", "1", "3"},
		},
		{
			name:     "filter applied to each page",
			tableKey: []KeySchemaElement{keyElement("pk", keyTypeHash), keyElement("sk", keyTypeRange)},
			items: []Item{
				sItem("pk", "p", "sk", "1", testShapesOwner, "amy"),
				sItem("pk", "p", "sk", "2", testShapesOwner, "bob"),
				sItem("pk", "p", "sk", "3", testShapesOwner, "bob"),
				sItem("pk", "p", "sk", "4", testShapesOwner, "amy"),
				sItem("pk", "p", "sk", "5", testShapesOwner, "amy"),
			},
			query: queryPage{
				keyCondExpr: "pk = :p",
				filterExpr:  testShapesOwner + " = :o",
				exprValues:  map[string]AttributeValue{":p": {S: new("p")}, ":o": {S: new("amy")}},
				scanForward: true,
			},
			attr: "sk",
			want: []string{"1", "4", "5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := createShapesTable(t, tt.tableKey, tt.gsi, tt.lsi, tt.items)

			got := attrValues(queryAllPages(t, s, tt.query), tt.attr)
			if !slices.Equal(got, tt.want) {
				t.Errorf("%s across pages = %v, want %v", tt.attr, got, tt.want)
			}
		})
	}
}

func TestQueryResumesAfterDeletedStartKey(t *testing.T) {
	t.Parallel()

	items := make([]Item, 0, 5)
	for i := range 5 {
		items = append(items, sItem("pk", "p", "sk", fmt.Sprint(i)))
	}

	s := createShapesTable(t, []KeySchemaElement{keyElement("pk", keyTypeHash), keyElement("sk", keyTypeRange)}, nil, nil, items)
	exprValues := map[string]AttributeValue{":p": {S: new("p")}}

	first, lastKey, _, err := s.Query(t.Context(), testShapesTable, "", "pk = :p", "", nil, exprValues, testShapesLimit, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.DeleteItem(t.Context(), testShapesTable, lastKey, false, ConditionInput{}); err != nil {
		t.Fatal(err)
	}

	rest := queryAllPagesFrom(t, s, exprValues, lastKey)

	got := attrValues(slices.Concat(first, rest), "sk")
	if want := []string{"0", "1", "2", "3", "4"}; !slices.Equal(got, want) {
		t.Errorf("sk across pages = %v, want %v", got, want)
	}
}

// queryAllPagesFrom pages through partition "p" starting at startKey.
func queryAllPagesFrom(t *testing.T, s *MemoryStorage, exprValues map[string]AttributeValue, startKey Item) []Item {
	t.Helper()

	var all []Item

	for range 50 {
		items, lastKey, _, err := s.Query(t.Context(), testShapesTable, "", "pk = :p", "", nil, exprValues, testShapesLimit, startKey, true)
		if err != nil {
			t.Fatal(err)
		}

		all = append(all, items...)

		if lastKey == nil {
			return all
		}

		startKey = lastKey
	}

	t.Fatalf("pagination did not terminate, got %d items", len(all))

	return nil
}

func TestQueryRejectsStartKeyWithoutIndexKey(t *testing.T) {
	t.Parallel()

	s := createShapesTable(t,
		[]KeySchemaElement{keyElement("pk", keyTypeHash)},
		[]KeySchemaElement{keyElement("g", keyTypeHash)}, nil,
		[]Item{sItem("pk", "a", "g", testShapesGroup)})

	items, lastKey, _, err := s.Query(t.Context(), testShapesTable, testShapesIndex, "g = :g", "", nil,
		map[string]AttributeValue{":g": {S: new(testShapesGroup)}}, testShapesLimit, sItem("pk", "a"), true)

	var tableErr *TableError
	if !errors.As(err, &tableErr) || tableErr.Code != errCodeValidation {
		t.Errorf("Query error = %v, want a ValidationException", err)
	}

	if items != nil || lastKey != nil {
		t.Errorf("Query = %v, %v, want no page", items, lastKey)
	}
}

func TestScanPaginatesNumericKeysInNumericOrder(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	if _, err := s.CreateTable(t.Context(), &CreateTableRequest{
		TableName:            testShapesTable,
		KeySchema:            []KeySchemaElement{keyElement("id", keyTypeHash)},
		AttributeDefinitions: []AttributeDefinition{{AttributeName: "id", AttributeType: "N"}},
	}); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"100", "9", "10", "1.5"} {
		if _, err := s.PutItem(t.Context(), testShapesTable, Item{"id": {N: new(id)}}, false, ConditionInput{}); err != nil {
			t.Fatal(err)
		}
	}

	var (
		all      []Item
		startKey Item
	)

	for range 10 {
		items, lastKey, _, err := s.Scan(t.Context(), testShapesTable, "", nil, nil, 1, startKey, nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		all = append(all, items...)

		if lastKey == nil {
			break
		}

		startKey = lastKey
	}

	if got, want := attrValues(all, "id"), []string{"1.5", "9", "10", "100"}; !slices.Equal(got, want) {
		t.Errorf("ids across pages = %v, want %v", got, want)
	}
}
