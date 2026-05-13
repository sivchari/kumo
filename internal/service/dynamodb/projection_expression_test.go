package dynamodb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//nolint:funlen // Exercises all DynamoDB read APIs that accept ProjectionExpression.
func TestReadAPIsApplyProjectionExpression(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("http://localhost:4566")
	svc := New(store)

	if _, err := store.CreateTable(t.Context(), &CreateTableRequest{
		TableName: "projection-test",
		KeySchema: []KeySchemaElement{
			{AttributeName: "pk", KeyType: "HASH"},
			{AttributeName: "sk", KeyType: "RANGE"},
		},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "pk", AttributeType: "S"},
			{AttributeName: "sk", AttributeType: "S"},
		},
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	for _, item := range []Item{
		{
			"pk":     {S: ptr("tenant-a")},
			"sk":     {S: ptr("001")},
			"name":   {S: ptr("first")},
			"status": {S: ptr("active")},
			"secret": {S: ptr("hidden")},
		},
		{
			"pk":     {S: ptr("tenant-a")},
			"sk":     {S: ptr("002")},
			"name":   {S: ptr("second")},
			"status": {S: ptr("active")},
			"secret": {S: ptr("hidden")},
		},
	} {
		if _, err := store.PutItem(t.Context(), "projection-test", item, false, ConditionInput{}); err != nil {
			t.Fatalf("PutItem: %v", err)
		}
	}

	t.Run("GetItem", func(t *testing.T) {
		t.Parallel()

		req := `{
			"TableName":"projection-test",
			"Key":{"pk":{"S":"tenant-a"},"sk":{"S":"001"}},
			"ProjectionExpression":"#n, status",
			"ExpressionAttributeNames":{"#n":"name"}
		}`

		var resp GetItemResponse

		dispatchDynamoDBForProjectionTest(t, svc, "GetItem", req, &resp)

		assertProjectedItem(t, resp.Item, "name", "status")
	})

	t.Run("Query", func(t *testing.T) {
		t.Parallel()

		req := `{
			"TableName":"projection-test",
			"KeyConditionExpression":"pk = :pk",
			"ProjectionExpression":"#n, status",
			"ExpressionAttributeNames":{"#n":"name"},
			"ExpressionAttributeValues":{":pk":{"S":"tenant-a"}}
		}`

		var resp QueryResponse

		dispatchDynamoDBForProjectionTest(t, svc, "Query", req, &resp)

		if got, want := len(resp.Items), 2; got != want {
			t.Fatalf("Items length: got %d, want %d", got, want)
		}

		for _, item := range resp.Items {
			assertProjectedItem(t, item, "name", "status")
		}
	})

	t.Run("Scan", func(t *testing.T) {
		t.Parallel()

		req := `{
			"TableName":"projection-test",
			"ProjectionExpression":"#n, status",
			"ExpressionAttributeNames":{"#n":"name"}
		}`

		var resp ScanResponse

		dispatchDynamoDBForProjectionTest(t, svc, "Scan", req, &resp)

		if got, want := len(resp.Items), 2; got != want {
			t.Fatalf("Items length: got %d, want %d", got, want)
		}

		for _, item := range resp.Items {
			assertProjectedItem(t, item, "name", "status")
		}
	})

	t.Run("BatchGetItem", func(t *testing.T) {
		t.Parallel()

		req := `{
			"RequestItems":{
				"projection-test":{
					"Keys":[{"pk":{"S":"tenant-a"},"sk":{"S":"001"}}],
					"ProjectionExpression":"#n, status",
					"ExpressionAttributeNames":{"#n":"name"}
				}
			}
		}`

		var resp BatchGetItemResponse

		dispatchDynamoDBForProjectionTest(t, svc, "BatchGetItem", req, &resp)

		items := resp.Responses["projection-test"]
		if got, want := len(items), 1; got != want {
			t.Fatalf("Items length: got %d, want %d", got, want)
		}

		assertProjectedItem(t, items[0], "name", "status")
	})

	t.Run("TransactGetItems", func(t *testing.T) {
		t.Parallel()

		req := `{
			"TransactItems":[{
				"Get":{
					"TableName":"projection-test",
					"Key":{"pk":{"S":"tenant-a"},"sk":{"S":"001"}},
					"ProjectionExpression":"#n, status",
					"ExpressionAttributeNames":{"#n":"name"}
				}
			}]
		}`

		var resp TransactGetItemsResponse

		dispatchDynamoDBForProjectionTest(t, svc, "TransactGetItems", req, &resp)

		if got, want := len(resp.Responses), 1; got != want {
			t.Fatalf("Responses length: got %d, want %d", got, want)
		}

		assertProjectedItem(t, resp.Responses[0].Item, "name", "status")
	})
}

func dispatchDynamoDBForProjectionTest(t *testing.T, svc *Service, action, body string, out any) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810."+action)

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("%s status: got %d, body=%s", action, w.Code, w.Body.String())
	}

	if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
		t.Fatalf("%s decode: %v; body=%s", action, err, w.Body.String())
	}
}

func assertProjectedItem(t *testing.T, item Item, names ...string) {
	t.Helper()

	if len(item) != len(names) {
		t.Fatalf("projected item = %v, want only %v", item, names)
	}

	for _, name := range names {
		if _, ok := item[name]; !ok {
			t.Fatalf("projected item = %v, missing %q", item, name)
		}
	}
}
