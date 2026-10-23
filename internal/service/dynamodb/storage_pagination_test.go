package dynamodb

import (
	"context"
	"fmt"
	"testing"
)

// assertPageCounts checks a Scan/Query page's ScannedCount and Count
// (len(items)) against the expected values. A Count mismatch is fatal since
// callers commonly index into items right after.
func assertPageCounts(t *testing.T, label string, items []Item, scanned, wantScanned, wantCount int) {
	t.Helper()

	if scanned != wantScanned {
		t.Errorf("%s ScannedCount = %d, want %d", label, scanned, wantScanned)
	}

	if len(items) != wantCount {
		t.Fatalf("%s Count = %d, want %d", label, len(items), wantCount)
	}
}

// assertLastEvaluatedKey checks whether LastEvaluatedKey is present or absent
// as expected.
func assertLastEvaluatedKey(t *testing.T, label string, lastKey Item, wantPresent bool) {
	t.Helper()

	if wantPresent && lastKey == nil {
		t.Fatalf("%s LastEvaluatedKey is nil, want present", label)
	}

	if !wantPresent && lastKey != nil {
		t.Errorf("%s LastEvaluatedKey = %v, want nil", label, lastKey)
	}
}

// assertItemSortKey checks an item's "SK" attribute against the expected value.
func assertItemSortKey(t *testing.T, label string, item Item, want string) {
	t.Helper()

	if got := item["SK"].S; got == nil || *got != want {
		t.Errorf("%s SK = %v, want %q", label, item["SK"], want)
	}
}

// TestScanLimitAppliedBeforeFilter is the exact repro from GitHub issue #860:
// a table with 2 items, neither matching the FilterExpression, and Limit: 1.
// AWS applies Limit to the number of items EVALUATED before the filter runs,
// so the first Scan must evaluate exactly 1 item, return zero matches, and
// return a LastEvaluatedKey so the caller can continue. Continuing with that
// key must evaluate the second (and last) item and return no LastEvaluatedKey,
// since the item space is now exhausted.
func TestScanLimitAppliedBeforeFilter(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName:            "scan-limit-before-filter",
		KeySchema:            []KeySchemaElement{{AttributeName: "pk", KeyType: keyTypeHash}},
		AttributeDefinitions: []AttributeDefinition{{AttributeName: "pk", AttributeType: "S"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, pk := range []string{"a", "b"} {
		if _, err := s.PutItem(ctx, "scan-limit-before-filter", Item{
			"pk":       {S: new(pk)},
			"category": {S: new(testAttrOther)},
		}, false, ConditionInput{}); err != nil {
			t.Fatal(err)
		}
	}

	filterExpr := "category = :want"
	filterValues := map[string]AttributeValue{":want": {S: new("target")}}

	items, lastKey, scanned, err := s.Scan(ctx, "scan-limit-before-filter", filterExpr, nil, filterValues, 1, nil, nil, nil)
	if err != nil {
		t.Fatalf("first Scan: %v", err)
	}

	assertPageCounts(t, "first Scan", items, scanned, 1, 0)
	assertLastEvaluatedKey(t, "first Scan", lastKey, true)

	if firstPK := lastKey["pk"].S; firstPK == nil || *firstPK != "a" {
		t.Errorf("first Scan LastEvaluatedKey[pk] = %v, want \"a\" (scan order is by serialized key)", lastKey)
	}

	items, lastKey, scanned, err = s.Scan(ctx, "scan-limit-before-filter", filterExpr, nil, filterValues, 1, lastKey, nil, nil)
	if err != nil {
		t.Fatalf("second Scan: %v", err)
	}

	assertPageCounts(t, "second Scan", items, scanned, 1, 0)
	assertLastEvaluatedKey(t, "second Scan", lastKey, false)
}

// TestQueryLimitAppliedBeforeFilter verifies that, like Scan, Query applies
// Limit to the number of items evaluated within the key-condition-matching
// set BEFORE the FilterExpression is applied. A filter that excludes the
// first two of four sort-key-ordered items must still count them toward
// Limit, so Limit=3 evaluates items sk=1..3, matches only sk=3, and leaves
// sk=4 for a continuation page.
func TestQueryLimitAppliedBeforeFilter(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := context.Background()

	_, err := s.CreateTable(ctx, &CreateTableRequest{
		TableName: "query-limit-before-filter",
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

	categories := map[string]string{"1": testAttrOther, "2": testAttrOther, "3": "target", "4": "target"}
	for _, sk := range []string{"1", "2", "3", "4"} {
		if _, err := s.PutItem(ctx, "query-limit-before-filter", Item{
			"PK":       {S: new("tenant1")},
			"SK":       {S: new(sk)},
			"category": {S: new(categories[sk])},
		}, false, ConditionInput{}); err != nil {
			t.Fatal(err)
		}
	}

	keyCondExpr := "PK = :pk"
	filterExpr := "category = :want"
	exprValues := map[string]AttributeValue{
		testExprPK: {S: new("tenant1")},
		":want":    {S: new("target")},
	}

	items, lastKey, scanned, err := s.Query(ctx, "query-limit-before-filter", "", keyCondExpr, filterExpr,
		nil, exprValues, 3, nil, true)
	if err != nil {
		t.Fatalf("first Query: %v", err)
	}

	assertPageCounts(t, "first Query", items, scanned, 3, 1)
	assertLastEvaluatedKey(t, "first Query", lastKey, true)
	assertItemSortKey(t, "first Query matched item", items[0], "3")
	assertItemSortKey(t, "first Query LastEvaluatedKey", lastKey, "3")

	items, lastKey, scanned, err = s.Query(ctx, "query-limit-before-filter", "", keyCondExpr, filterExpr,
		nil, exprValues, 3, lastKey, true)
	if err != nil {
		t.Fatalf("second Query: %v", err)
	}

	assertPageCounts(t, "second Query", items, scanned, 1, 1)
	assertLastEvaluatedKey(t, "second Query", lastKey, false)
}

const (
	testIndexTable    = "index-pagination"
	testIndexName     = "account_index"
	testIndexHashKey  = "account"
	testIndexRangeKey = "created"
	testIndexAccount  = "acc"
	testIndexItems    = 25
	testIndexLimit    = 10
)

// TestQueryIndexPagination follows LastEvaluatedKey through a GSI query whose
// items share the index partition key (and, with a range key, have duplicate
// range values). Every item must be returned exactly once, in index order,
// and LastEvaluatedKey must carry the index key attributes.
func TestQueryIndexPagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		rangeKey    string
		scanForward bool
	}{
		{name: "hash only", scanForward: true},
		{name: "hash only descending", scanForward: false},
		{name: "hash and range", rangeKey: testIndexRangeKey, scanForward: true},
		{name: "hash and range descending", rangeKey: testIndexRangeKey, scanForward: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			indexKeySchema := []KeySchemaElement{{AttributeName: testIndexHashKey, KeyType: keyTypeHash}}
			if tt.rangeKey != "" {
				indexKeySchema = append(indexKeySchema, KeySchemaElement{AttributeName: tt.rangeKey, KeyType: keyTypeRange})
			}

			s := setupIndexPaginationTable(t, indexKeySchema, testIndexItems)
			items := queryIndexAllPages(t, s, indexKeySchema, testIndexLimit, tt.scanForward)

			seen := make(map[string]struct{}, len(items))

			for _, item := range items {
				pk := *item["pk"].S
				if _, dup := seen[pk]; dup {
					t.Errorf("item %s returned more than once", pk)
				}

				seen[pk] = struct{}{}
			}

			if len(seen) != testIndexItems {
				t.Errorf("distinct items = %d, want %d", len(seen), testIndexItems)
			}

			if tt.rangeKey != "" {
				assertIndexRangeOrder(t, items, tt.scanForward)
			}
		})
	}
}

// setupIndexPaginationTable creates a table with a GSI using indexKeySchema
// and puts n items that all share the same index partition key. Range values
// repeat every three items so pagination must break ties on the table key.
func setupIndexPaginationTable(t *testing.T, indexKeySchema []KeySchemaElement, n int) *MemoryStorage {
	t.Helper()

	s := NewMemoryStorage("http://localhost:4566")

	_, err := s.CreateTable(t.Context(), &CreateTableRequest{
		TableName: testIndexTable,
		KeySchema: []KeySchemaElement{{AttributeName: "pk", KeyType: keyTypeHash}},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "pk", AttributeType: "S"},
			{AttributeName: testIndexHashKey, AttributeType: "S"},
			{AttributeName: testIndexRangeKey, AttributeType: "S"},
		},
		GlobalSecondaryIndexes: []GlobalSecondaryIndex{{
			IndexName:  testIndexName,
			KeySchema:  indexKeySchema,
			Projection: Projection{ProjectionType: testProjectionAll},
		}},
	})

	if err != nil {
		t.Fatal(err)
	}

	for i := range n {
		if _, err := s.PutItem(t.Context(), testIndexTable, Item{
			"pk":              {S: new(fmt.Sprintf("item_%02d", i))},
			testIndexHashKey:  {S: new(testIndexAccount)},
			testIndexRangeKey: {S: new(fmt.Sprintf("2024-01-%02d", i%3))},
		}, false, ConditionInput{}); err != nil {
			t.Fatal(err)
		}
	}

	return s
}

// queryIndexAllPages follows LastEvaluatedKey until it is absent and returns
// all items, checking that every LastEvaluatedKey carries the index key.
func queryIndexAllPages(t *testing.T, s *MemoryStorage, indexKeySchema []KeySchemaElement, limit int, scanForward bool) []Item {
	t.Helper()

	keyCondExpr := testIndexHashKey + " = :a"
	exprValues := map[string]AttributeValue{":a": {S: new(testIndexAccount)}}
	maxPages := testIndexItems/limit + 2

	var (
		all      []Item
		startKey Item
	)

	for page := 1; page <= maxPages; page++ {
		items, lastKey, _, err := s.Query(t.Context(), testIndexTable, testIndexName, keyCondExpr, "",
			nil, exprValues, limit, startKey, scanForward)

		if err != nil {
			t.Fatalf("page %d Query: %v", page, err)
		}

		all = append(all, items...)

		if lastKey == nil {
			return all
		}

		for _, ks := range indexKeySchema {
			if _, ok := lastKey[ks.AttributeName]; !ok {
				t.Fatalf("page %d LastEvaluatedKey = %v, missing index key %q", page, lastKey, ks.AttributeName)
			}
		}

		startKey = lastKey
	}

	t.Fatalf("pagination did not terminate after %d pages, got %d items", maxPages, len(all))

	return nil
}

// assertIndexRangeOrder checks that items are sorted by the index range key
// in the requested direction. Equal neighbours are allowed.
func assertIndexRangeOrder(t *testing.T, items []Item, scanForward bool) {
	t.Helper()

	for i := 1; i < len(items); i++ {
		prev, cur := *items[i-1][testIndexRangeKey].S, *items[i][testIndexRangeKey].S
		if prev != cur && (prev < cur) != scanForward {
			t.Errorf("items[%d] %s = %q out of order after %q", i, testIndexRangeKey, cur, prev)
		}
	}
}
