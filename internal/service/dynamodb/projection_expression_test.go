package dynamodb

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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
			{AttributeName: "pk", KeyType: keyTypeHash},
			{AttributeName: "sk", KeyType: keyTypeRange},
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
			"pk":           {S: new("tenant-a")},
			"sk":           {S: new("001")},
			testAttrName:   {S: new("first")},
			testAttrStatus: {S: new("active")},
			"secret":       {S: new("hidden")},
		},
		{
			"pk":           {S: new("tenant-a")},
			"sk":           {S: new("002")},
			testAttrName:   {S: new("second")},
			testAttrStatus: {S: new("active")},
			"secret":       {S: new("hidden")},
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

		assertProjectedItem(t, resp.Item, testAttrName, testAttrStatus)
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
			assertProjectedItem(t, item, testAttrName, testAttrStatus)
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
			assertProjectedItem(t, item, testAttrName, testAttrStatus)
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

		assertProjectedItem(t, items[0], testAttrName, testAttrStatus)
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

		assertProjectedItem(t, resp.Responses[0].Item, testAttrName, testAttrStatus)
	})
}

func dispatchDynamoDBForProjectionTest(t *testing.T, svc *Service, action, body string, out any) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
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

func TestProjectionExpressionNestedPaths(t *testing.T) {
	t.Parallel()

	const settings = "settings"

	item := Item{
		"id": {S: new("1")},
		settings: {M: map[string]*AttributeValue{
			"theme": {S: new("dark")},
			"lang":  {S: new("en")},
		}},
		"tags": {L: []*AttributeValue{{S: new("a")}, {S: new("b")}, {S: new("c")}}},
	}

	proj, err := parseProjectionExpression("#s.theme, tags[2], tags[0], settings.missing, tags[9]",
		map[string]string{"#s": settings})
	if err != nil {
		t.Fatalf("parseProjectionExpression: %v", err)
	}

	got := proj.project(item)

	want := Item{
		settings: {M: map[string]*AttributeValue{"theme": {S: new("dark")}}},
		"tags":   {L: []*AttributeValue{{S: new("a")}, {S: new("c")}}},
	}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		t.Fatalf("projected item = %s, want %s", gotJSON, wantJSON)
	}
}

var invalidProjectionTests = []struct {
	name  string
	expr  string
	names map[string]string
	want  string
}{
	{
		name: "parent and child",
		expr: "m, m.a",
		want: "Invalid ProjectionExpression: Two document paths overlap with each other; " +
			"must remove or rewrite one of these paths; path one: [m], path two: [m, a]",
	},
	{
		name: "duplicate",
		expr: "a, a",
		want: "Invalid ProjectionExpression: Two document paths overlap with each other; " +
			"must remove or rewrite one of these paths; path one: [a], path two: [a]",
	},
	{
		name: "list element and its member",
		expr: "l[0], l[0].x",
		want: "Invalid ProjectionExpression: Two document paths overlap with each other; " +
			"must remove or rewrite one of these paths; path one: [l, [0]], path two: [l, [0], x]",
	},
	{
		name:  "overlap through placeholder",
		expr:  "#m.a, m",
		names: map[string]string{"#m": "m"},
		want: "Invalid ProjectionExpression: Two document paths overlap with each other; " +
			"must remove or rewrite one of these paths; path one: [m, a], path two: [m]",
	},
	{
		name: "undefined placeholder",
		expr: "a, #u",
		want: "Invalid ProjectionExpression: An expression attribute name used in the document path " +
			"is not defined; attribute name: #u",
	},
	{
		name:  "undefined placeholder in nested path",
		expr:  "#m.#u",
		names: map[string]string{"#m": "m"},
		want: "Invalid ProjectionExpression: An expression attribute name used in the document path " +
			"is not defined; attribute name: #u",
	},
}

func TestParseProjectionExpression_Invalid(t *testing.T) {
	t.Parallel()

	for _, tt := range invalidProjectionTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseProjectionExpression(tt.expr, tt.names)

			var tErr *TableError
			if !errors.As(err, &tErr) {
				t.Fatalf("error = %v, want *TableError", err)
			}

			if tErr.Code != errCodeValidation || tErr.Message != tt.want {
				t.Fatalf("error = %s: %s, want %s: %s", tErr.Code, tErr.Message, errCodeValidation, tt.want)
			}
		})
	}
}

func TestParseProjectionExpression_Valid(t *testing.T) {
	t.Parallel()

	for _, expr := range []string{
		"",
		"m.a, m.b",
		"l[0], l[1]",
		"l[0].x, l[0].y, l[1]",
		"a, ab",
		"m.a, n.a",
	} {
		if _, err := parseProjectionExpression(expr, nil); err != nil {
			t.Errorf("parseProjectionExpression(%q) = %v, want nil", expr, err)
		}
	}
}
