package dynamodb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// DynamoDB always includes UnprocessedItems, as {} when every request was
// processed. boto3's Table.batch_writer reads it without a default.
func TestBatchWriteItemResponseIncludesUnprocessedItems(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("http://localhost:4566")
	svc := New(store)

	if _, err := store.CreateTable(t.Context(), &CreateTableRequest{
		TableName: "batch-write",
		KeySchema: []KeySchemaElement{
			{AttributeName: "pk", KeyType: keyTypeHash},
		},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "pk", AttributeType: "S"},
		},
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	body := `{"RequestItems":{"batch-write":[{"PutRequest":{"Item":{"pk":{"S":"a"}}}}]}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810.BatchWriteItem")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("BatchWriteItem status: got %d, body=%s", w.Code, w.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v; body=%s", err, w.Body.String())
	}

	if got := string(raw["UnprocessedItems"]); got != "{}" {
		t.Fatalf("UnprocessedItems: got %q, want {} (body=%s)", got, w.Body.String())
	}
}
