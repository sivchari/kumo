package dynamodb

import (
	"context"
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
		KeySchema:            []KeySchemaElement{{AttributeName: "pk", KeyType: "HASH"}},
		AttributeDefinitions: []AttributeDefinition{{AttributeName: "pk", AttributeType: "S"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, pk := range []string{"a", "b"} {
		if _, err := s.PutItem(ctx, "scan-limit-before-filter", Item{
			"pk":       {S: ptr(pk)},
			"category": {S: ptr("other")},
		}, false, ConditionInput{}); err != nil {
			t.Fatal(err)
		}
	}

	filterExpr := "category = :want"
	filterValues := map[string]AttributeValue{":want": {S: ptr("target")}}

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
			{AttributeName: "PK", KeyType: "HASH"},
			{AttributeName: "SK", KeyType: "RANGE"},
		},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "PK", AttributeType: "S"},
			{AttributeName: "SK", AttributeType: "S"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	categories := map[string]string{"1": "other", "2": "other", "3": "target", "4": "target"}
	for _, sk := range []string{"1", "2", "3", "4"} {
		if _, err := s.PutItem(ctx, "query-limit-before-filter", Item{
			"PK":       {S: ptr("tenant1")},
			"SK":       {S: ptr(sk)},
			"category": {S: ptr(categories[sk])},
		}, false, ConditionInput{}); err != nil {
			t.Fatal(err)
		}
	}

	keyCondExpr := "PK = :pk"
	filterExpr := "category = :want"
	exprValues := map[string]AttributeValue{
		":pk":   {S: ptr("tenant1")},
		":want": {S: ptr("target")},
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
