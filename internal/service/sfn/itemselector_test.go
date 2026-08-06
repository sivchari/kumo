package sfn

import (
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
		if !jsonDeepEqual(outputs[i], w) {
			t.Fatalf("item %d: got %+v, want %+v", i, outputs[i], w)
		}
	}
}
