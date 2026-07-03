package dynamodb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScanPartitionsItemsAcrossSegments(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("http://localhost:4566")
	svc := New(store)

	if _, err := store.CreateTable(t.Context(), &CreateTableRequest{
		TableName: "parallel-scan-test",
		KeySchema: []KeySchemaElement{
			{AttributeName: "pk", KeyType: "HASH"},
		},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "pk", AttributeType: "S"},
		},
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	const itemCount = 50

	for i := range itemCount {
		item := Item{
			"pk":    {S: ptr(fmt.Sprintf("item-%02d", i))},
			"value": {S: ptr("payload")},
		}

		if _, err := store.PutItem(t.Context(), "parallel-scan-test", item, false, ConditionInput{}); err != nil {
			t.Fatalf("PutItem: %v", err)
		}
	}

	seen := make(map[string]struct{}, itemCount)

	for segment := range 4 {
		body := fmt.Sprintf(`{"TableName":"parallel-scan-test","Segment":%d,"TotalSegments":4}`, segment)

		var resp ScanResponse

		dispatchDynamoDBForParallelScanTest(t, svc, body, &resp)

		for _, item := range resp.Items {
			pk := item["pk"].S
			if pk == nil {
				t.Fatalf("item missing pk: %v", item)
			}

			if _, ok := seen[*pk]; ok {
				t.Fatalf("item %q returned by more than one segment", *pk)
			}

			seen[*pk] = struct{}{}
		}
	}

	if got, want := len(seen), itemCount; got != want {
		t.Fatalf("parallel scan returned %d unique items, want %d", got, want)
	}
}

func dispatchDynamoDBForParallelScanTest(t *testing.T, svc *Service, body string, out any) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810.Scan")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Scan status: got %d, body=%s", w.Code, w.Body.String())
	}

	if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
		t.Fatalf("Scan decode: %v; body=%s", err, w.Body.String())
	}
}
