package sfn

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestMapStateItemSelectorStaticAndContextAndInputPaths exercises
// ItemSelector's three reference forms in Inline mode: a static value, a
// "$$.Map.Item.Value"/"$$.Map.Item.Index" Context object reference, and a
// "$." reference against the Map state's own input.
func TestMapStateItemSelectorStaticAndContextAndInputPaths(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemsPath": "$.items",
				"ItemSelector": {
					"static": "fixed",
					"value.$": "$$.Map.Item.Value",
					"index.$": "$$.Map.Item.Index",
					"courier.$": "$.courier"
				},
				"ItemProcessor": ` + echoItemProcessorJSON + `,
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-item-selector", definition)

	input := `{"items":[{"a":1},{"a":2}],"courier":"UQS"}`
	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, input)

	var outputs []map[string]any
	if err := json.Unmarshal([]byte(exec.Output), &outputs); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if len(outputs) != 2 {
		t.Fatalf("outputs: got %d entries, want 2", len(outputs))
	}

	want := []map[string]any{
		{"static": "fixed", "value": map[string]any{"a": float64(1)}, "index": float64(0), "courier": "UQS"},
		{"static": "fixed", "value": map[string]any{"a": float64(2)}, "index": float64(1), "courier": "UQS"},
	}

	for i, w := range want {
		assertMapEqual(t, i, w, outputs[i])
	}
}

// assertMapEqual fails the test unless got deep-equals want, field by
// field, reporting the specific mismatched key when it doesn't.
func assertMapEqual(t *testing.T, index int, want, got map[string]any) {
	t.Helper()

	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			t.Fatalf("item %d: missing key %q in %+v", index, k, got)
		}

		wantJSON, _ := json.Marshal(wv)
		gotJSON, _ := json.Marshal(gv)

		if !bytes.Equal(wantJSON, gotJSON) {
			t.Fatalf("item %d: key %q: got %s, want %s", index, k, gotJSON, wantJSON)
		}
	}
}
