package dynamodb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// DynamoDB always includes every requested table in Responses (as an empty
// list when no keys matched) and always includes UnprocessedKeys, even when
// empty. Verified against amazon/dynamodb-local:
//
//	{"Responses":{"table":[]},"UnprocessedKeys":{}}
//
// These tests assert on the raw JSON body because SDK-side unmarshalling
// masks omitted fields.
//
//nolint:funlen // Exercises the response shape across all hit/miss combinations.
func TestBatchGetItemResponseShape(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("http://localhost:4566")
	svc := New(store)

	for _, tableName := range []string{"batch-get-hit", "batch-get-miss"} {
		if _, err := store.CreateTable(t.Context(), &CreateTableRequest{
			TableName: tableName,
			KeySchema: []KeySchemaElement{
				{AttributeName: "pk", KeyType: "HASH"},
			},
			AttributeDefinitions: []AttributeDefinition{
				{AttributeName: "pk", AttributeType: "S"},
			},
		}); err != nil {
			t.Fatalf("CreateTable: %v", err)
		}
	}

	if _, err := store.PutItem(t.Context(), "batch-get-hit", Item{
		"pk":   {S: ptr("existing")},
		"name": {S: ptr("value")},
	}, false, ConditionInput{}); err != nil {
		t.Fatalf("PutItem: %v", err)
	}

	t.Run("all keys miss", func(t *testing.T) {
		t.Parallel()

		req := `{
			"RequestItems":{
				"batch-get-miss":{"Keys":[{"pk":{"S":"nope-1"}},{"pk":{"S":"nope-2"}}]}
			}
		}`

		responses, unprocessed := dispatchBatchGetItemRaw(t, svc, req)

		items, ok := responses["batch-get-miss"]
		if !ok {
			t.Fatalf("Responses must include the requested table even with zero matches, got keys: %v", responseKeys(responses))
		}

		if len(items) != 0 {
			t.Fatalf("Responses[batch-get-miss] length: got %d, want 0", len(items))
		}

		if unprocessed == nil {
			t.Fatal("UnprocessedKeys must be present (as {}) even when empty")
		}
	})

	t.Run("hit and miss mixed", func(t *testing.T) {
		t.Parallel()

		req := `{
			"RequestItems":{
				"batch-get-hit":{"Keys":[{"pk":{"S":"existing"}},{"pk":{"S":"nope"}}]}
			}
		}`

		responses, _ := dispatchBatchGetItemRaw(t, svc, req)

		if got, want := len(responses["batch-get-hit"]), 1; got != want {
			t.Fatalf("Responses[batch-get-hit] length: got %d, want %d", got, want)
		}
	})

	t.Run("multiple tables with one empty", func(t *testing.T) {
		t.Parallel()

		req := `{
			"RequestItems":{
				"batch-get-hit":{"Keys":[{"pk":{"S":"existing"}}]},
				"batch-get-miss":{"Keys":[{"pk":{"S":"nope"}}]}
			}
		}`

		responses, _ := dispatchBatchGetItemRaw(t, svc, req)

		if got, want := len(responses["batch-get-hit"]), 1; got != want {
			t.Fatalf("Responses[batch-get-hit] length: got %d, want %d", got, want)
		}

		items, ok := responses["batch-get-miss"]
		if !ok {
			t.Fatalf("Responses must include all requested tables, got keys: %v", responseKeys(responses))
		}

		if len(items) != 0 {
			t.Fatalf("Responses[batch-get-miss] length: got %d, want 0", len(items))
		}
	})
}

func dispatchBatchGetItemRaw(t *testing.T, svc *Service, body string) (map[string][]json.RawMessage, map[string]json.RawMessage) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810.BatchGetItem")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("BatchGetItem status: got %d, body=%s", w.Code, w.Body.String())
	}

	var raw struct {
		Responses       map[string][]json.RawMessage `json:"Responses"`       //nolint:tagliatelle // DynamoDB wire format uses PascalCase.
		UnprocessedKeys map[string]json.RawMessage   `json:"UnprocessedKeys"` //nolint:tagliatelle // DynamoDB wire format uses PascalCase.
	}

	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v; body=%s", err, w.Body.String())
	}

	if raw.Responses == nil {
		t.Fatalf("Responses field is missing from response body: %s", w.Body.String())
	}

	return raw.Responses, raw.UnprocessedKeys
}

func responseKeys(responses map[string][]json.RawMessage) []string {
	keys := make([]string, 0, len(responses))
	for k := range responses {
		keys = append(keys, k)
	}

	return keys
}
