package sfn

import (
	"encoding/json"
	"fmt"
)

// applyItemSelector transforms every item per the Map state's ItemSelector
// field. Values may be static, "$."-prefixed paths against mapInput, or
// "$$."-prefixed Context refs. An unset ItemSelector leaves items unchanged.
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
// Map.Item.Source (AWS sets it for S3 ListObjectsV2 datasets) is not modeled.
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
