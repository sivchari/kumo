package dynamodb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListTagsOfResource_EmptyArray(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage("http://localhost:4566"))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ResourceArn":"arn:aws:dynamodb:us-east-1:000000000000:table/foo"}`))
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810.ListTagsOfResource")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	tags, ok := resp["Tags"]
	if !ok {
		t.Fatalf("response missing Tags field; body=%s (terraform-provider-aws requires the field even when empty)", w.Body.String())
	}

	tagsSlice, ok := tags.([]any)
	if !ok {
		t.Fatalf("Tags is %T, want []any", tags)
	}

	if len(tagsSlice) != 0 {
		t.Fatalf("Tags = %v, want empty array", tagsSlice)
	}
}

func TestTagResource_NoOp(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage("http://localhost:4566"))

	for _, action := range []string{"TagResource", "UntagResource"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/",
				strings.NewReader(`{"ResourceArn":"arn:aws:dynamodb:us-east-1:000000000000:table/foo","Tags":[{"Key":"k","Value":"v"}]}`))
			req.Header.Set("X-Amz-Target", "DynamoDB_20120810."+action)

			w := httptest.NewRecorder()
			svc.DispatchAction(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status: got %d, body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestDescribeContinuousBackups_DisabledForExistingTable(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage("http://localhost:4566")
	svc := New(store)

	if _, err := store.CreateTable(t.Context(), &CreateTableRequest{
		TableName: "items",
		KeySchema: []KeySchemaElement{{AttributeName: "id", KeyType: "HASH"}},
		AttributeDefinitions: []AttributeDefinition{
			{AttributeName: "id", AttributeType: "S"},
		},
		BillingMode: "PAY_PER_REQUEST",
	}); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"TableName":"items"}`))
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810.DescribeContinuousBackups")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", w.Code, w.Body.String())
	}

	var resp describeContinuousBackupsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got, want := resp.ContinuousBackupsDescription.ContinuousBackupsStatus, continuousBackupsDisabled; got != want {
		t.Errorf("ContinuousBackupsStatus: got %q, want %q", got, want)
	}

	if got, want := resp.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus, continuousBackupsDisabled; got != want {
		t.Errorf("PointInTimeRecoveryStatus: got %q, want %q", got, want)
	}
}

func TestDescribeContinuousBackups_TableNotFound(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage("http://localhost:4566"))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"TableName":"missing"}`))
	req.Header.Set("X-Amz-Target", "DynamoDB_20120810.DescribeContinuousBackups")

	w := httptest.NewRecorder()
	svc.DispatchAction(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400, body=%s", w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "TableNotFoundException") {
		t.Fatalf("expected TableNotFoundException, got %s", w.Body.String())
	}
}
