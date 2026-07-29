package sfn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

// itemBatcherMapDefinition builds a single-state Map state machine
// definition batching its items via ItemBatcher, with an echo
// ItemProcessor so the Map's output reflects exactly what envelope each
// batch invocation received.
func itemBatcherMapDefinition(itemBatcherJSON string) string {
	return fmt.Sprintf(`{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemBatcher": %s,
				"ItemProcessor": %s,
				"End": true
			}
		}
	}`, itemBatcherJSON, distributedItemProcessorJSON)
}

// batchEnvelope decodes the AWS-documented ItemBatcher processor input
// shape: {"BatchInput": {...}, "Items": [...]}. JSON tags are lowerCamelCase
// (encoding/json matches the wire's PascalCase case-insensitively; see
// itemBatcherDef in itembatcher.go for why).
type batchEnvelope struct {
	BatchInput map[string]any `json:"batchInput"`
	Items      []any          `json:"items"`
}

func TestMapStateItemBatcherGroupsItemsAndBuildsEnvelope(t *testing.T) {
	t.Parallel()

	itemBatcher := `{"MaxItemsPerBatch": 2, "BatchInput": {"tag": "batch"}}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-itembatcher-group", itemBatcherMapDefinition(itemBatcher))

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `[1,2,3,4,5]`)

	var envelopes []batchEnvelope
	if err := json.Unmarshal([]byte(exec.Output), &envelopes); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if len(envelopes) != 3 {
		t.Fatalf("batches: got %d, want 3", len(envelopes))
	}

	wantItems := [][]any{
		{float64(1), float64(2)},
		{float64(3), float64(4)},
		{float64(5)},
	}

	for i, want := range wantItems {
		if !jsonDeepEqual(envelopes[i].Items, want) {
			t.Fatalf("batch %d items: got %+v, want %+v", i, envelopes[i].Items, want)
		}

		if envelopes[i].BatchInput["tag"] != "batch" {
			t.Fatalf("batch %d BatchInput: got %+v, want tag=batch", i, envelopes[i].BatchInput)
		}
	}
}

func TestMapStateItemBatcherResolvesBatchInputFromMapInput(t *testing.T) {
	t.Parallel()

	itemBatcher := `{"MaxItemsPerBatch": 10, "BatchInput": {"label.$": "$.tag"}}`
	definition := fmt.Sprintf(`{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemsPath": "$.items",
				"ItemBatcher": %s,
				"ItemProcessor": %s,
				"End": true
			}
		}
	}`, itemBatcher, distributedItemProcessorJSON)

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-itembatcher-input-path", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"items":[1,2,3],"tag":"hello"}`)

	var envelopes []batchEnvelope
	if err := json.Unmarshal([]byte(exec.Output), &envelopes); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if len(envelopes) != 1 {
		t.Fatalf("batches: got %d, want 1", len(envelopes))
	}

	if got := envelopes[0].BatchInput["label"]; got != "hello" {
		t.Fatalf("BatchInput.label: got %v, want %q", got, "hello")
	}
}

// jsonDeepEqual compares two already-decoded JSON values by re-encoding
// them, avoiding a reflect.DeepEqual dependency on map/slice identity.
func jsonDeepEqual(a, b any) bool {
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}

	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}

	return bytes.Equal(aJSON, bJSON)
}
