package dynamodb

import (
	"encoding/json"
	"testing"
)

func FuzzConditionExpressionNoPanic(f *testing.F) {
	f.Add(`{"items":{"L":[null]}}`, `contains(items, :needle)`, `{":needle":{"S":"x"}}`)
	f.Add(`{"pk":{"S":"1"},"count":{"N":"3"}}`, `count >= :min`, `{":min":{"N":"1"}}`)
	f.Add(`{"meta":{"M":{"version":{"N":"1"}}}}`, `attribute_exists(meta.version)`, `{}`)
	f.Add(`{"tags":{"SS":["a","b"]}}`, `contains(tags, :tag)`, `{":tag":{"S":"a"}}`)
	f.Add(`{"name":{"S":"alice"}}`, `begins_with(name, :prefix)`, `{":prefix":{"S":"al"}}`)

	f.Fuzz(func(t *testing.T, itemJSON, expression, valuesJSON string) {
		if len(itemJSON) > 4096 || len(expression) > 512 || len(valuesJSON) > 4096 {
			t.Skip()
		}

		var item Item
		if err := json.Unmarshal([]byte(itemJSON), &item); err != nil {
			return
		}

		values := make(map[string]AttributeValue)
		if err := json.Unmarshal([]byte(valuesJSON), &values); err != nil {
			return
		}

		_, _ = evaluateCondition(item, ConditionInput{
			Expression: expression,
			ExprNames: map[string]string{
				"#items": "items",
				"#name":  "name",
			},
			ExprValues: values,
		})
	})
}

func FuzzAttributeValueJSONRoundTrip(f *testing.F) {
	f.Add(`{"S":"hello"}`)
	f.Add(`{"N":"123.45"}`)
	f.Add(`{"BOOL":true}`)
	f.Add(`{"NULL":true}`)
	f.Add(`{"L":[{"S":"x"},null,{"N":"1"}]}`)
	f.Add(`{"M":{"nested":{"SS":["a","b"]}}}`)

	f.Fuzz(func(t *testing.T, data string) {
		if len(data) > 4096 {
			t.Skip()
		}

		var first AttributeValue
		if err := json.Unmarshal([]byte(data), &first); err != nil {
			return
		}

		encoded, err := json.Marshal(first)
		if err != nil {
			t.Fatalf("marshal after unmarshal failed: %v", err)
		}

		var second AttributeValue
		if err := json.Unmarshal(encoded, &second); err != nil {
			t.Fatalf("unmarshal after marshal failed: %v encoded=%s", err, string(encoded))
		}
	})
}
