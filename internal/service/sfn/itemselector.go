package sfn

import (
	"encoding/json"
	"fmt"
)

// applyItemSelector transforms every item per the Map state's ItemSelector
// field, which works identically in Inline and Distributed mode (see
// https://docs.aws.amazon.com/step-functions/latest/dg/input-output-itemselector.html).
// ItemSelector's key-value pairs may be static values, "$."-prefixed
// JSONPath references against the Map state's own input (mapInput), or
// "$$."-prefixed Context object references: $$.Map.Item.Value (the
// current item) and $$.Map.Item.Index (its 0-based index). An unset
// ItemSelector leaves items unchanged.
func applyItemSelector(raw json.RawMessage, items []json.RawMessage, mapInput string) ([]json.RawMessage, error) {
	if len(raw) == 0 {
		return items, nil
	}

	var selector map[string]any
	if err := json.Unmarshal(raw, &selector); err != nil {
		return nil, fmt.Errorf("parse ItemSelector: %w", err)
	}

	selected := make([]json.RawMessage, len(items))

	for i, item := range items {
		itemContext, err := mapItemContext(i, item)
		if err != nil {
			return nil, fmt.Errorf("itemSelector: build Context object for item %d: %w", i, err)
		}

		resolved, err := resolveParametersWithContext(selector, mapInput, itemContext)
		if err != nil {
			return nil, fmt.Errorf("itemSelector: item %d: %w", i, err)
		}

		encoded, err := json.Marshal(resolved)
		if err != nil {
			return nil, fmt.Errorf("itemSelector: marshal item %d: %w", i, err)
		}

		selected[i] = encoded
	}

	return selected, nil
}

// mapItemContext builds the "$$." Context object exposed to ItemSelector
// for one item: {"Map": {"Item": {"Index": index, "Value": <item>}}}.
// Map.Item.Source (which real AWS sets for S3 ListObjectsV2 datasets) is
// not modeled; kumo's ItemReader always yields STATE_DATA-equivalent items.
func mapItemContext(index int, item json.RawMessage) (map[string]any, error) {
	var value any
	if err := json.Unmarshal(item, &value); err != nil {
		return nil, fmt.Errorf("parse item: %w", err)
	}

	return map[string]any{
		"Map": map[string]any{
			"Item": map[string]any{
				"Index": index,
				"Value": value,
			},
		},
	}, nil
}
