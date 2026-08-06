package sfn

import (
	"encoding/json"
	"fmt"
)

// itemBatcherDef is the decoded ItemBatcher field. JSON tags are
// lowerCamelCase but still decode AWS's PascalCase field names (see retry.go).
// The *Path fields take precedence over their static counterparts when both are set.
type itemBatcherDef struct {
	MaxItemsPerBatch          *int           `json:"maxItemsPerBatch"`
	MaxInputBytesPerBatch     *int           `json:"maxInputBytesPerBatch"`
	MaxItemsPerBatchPath      string         `json:"maxItemsPerBatchPath"`
	MaxInputBytesPerBatchPath string         `json:"maxInputBytesPerBatchPath"`
	BatchInput                map[string]any `json:"batchInput"`
}

// mapUnit is a single processor invocation: one item, or a batch envelope
// when ItemBatcher is set. itemCount is how many original items it
// represents, which tolerance thresholds count against.
type mapUnit struct {
	input     string
	itemCount int
}

// buildProcessorUnits groups items into one mapUnit per item, or, when
// ItemBatcher is set, into batch envelopes shaped as AWS documents:
// {"BatchInput": {...}, "Items": [...]}.
func buildProcessorUnits(raw json.RawMessage, items []json.RawMessage, mapInput string) ([]mapUnit, error) {
	if len(raw) == 0 {
		return itemUnits(items), nil
	}

	var def itemBatcherDef
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, fmt.Errorf("parse ItemBatcher: %w", err)
	}

	maxItemsPerBatch, err := resolveOptionalIntPath(def.MaxItemsPerBatch, def.MaxItemsPerBatchPath, mapInput)
	if err != nil {
		return nil, fmt.Errorf("resolve ItemBatcher MaxItemsPerBatchPath: %w", err)
	}

	maxInputBytesPerBatch, err := resolveOptionalIntPath(def.MaxInputBytesPerBatch, def.MaxInputBytesPerBatchPath, mapInput)
	if err != nil {
		return nil, fmt.Errorf("resolve ItemBatcher MaxInputBytesPerBatchPath: %w", err)
	}

	if maxItemsPerBatch == nil && maxInputBytesPerBatch == nil {
		return nil, fmt.Errorf("itemBatcher requires MaxItemsPerBatch(Path) or MaxInputBytesPerBatch(Path)")
	}

	batchInput, err := resolveBatchInput(def.BatchInput, mapInput)
	if err != nil {
		return nil, err
	}

	batches := groupIntoBatches(items, maxItemsPerBatch, maxInputBytesPerBatch)

	units := make([]mapUnit, len(batches))

	for i, batch := range batches {
		envelope, err := marshalBatchEnvelope(batchInput, batch)
		if err != nil {
			return nil, err
		}

		units[i] = mapUnit{input: envelope, itemCount: len(batch)}
	}

	return units, nil
}

// itemUnits wraps each item as its own one-item mapUnit, the default when
// ItemBatcher is not set.
func itemUnits(items []json.RawMessage) []mapUnit {
	units := make([]mapUnit, len(items))

	for i, item := range items {
		units[i] = mapUnit{input: string(item), itemCount: 1}
	}

	return units
}

// resolveBatchInput resolves ItemBatcher.BatchInput's static/".$" values
// against the Map state's own input, mirroring how ItemSelector resolves
// "$." paths (see itemselector.go).
func resolveBatchInput(batchInput map[string]any, mapInput string) (map[string]any, error) {
	if batchInput == nil {
		return map[string]any{}, nil
	}

	resolved, err := resolveParameters(batchInput, mapInput)
	if err != nil {
		return nil, fmt.Errorf("resolve ItemBatcher BatchInput: %w", err)
	}

	return resolved, nil
}

// groupIntoBatches splits items into batches bounded by maxItems and/or
// maxBytes. A batch always contains at least one item, even if that item
// alone exceeds maxBytes, so a single oversized item cannot stall batching.
func groupIntoBatches(items []json.RawMessage, maxItems, maxBytes *int) [][]json.RawMessage {
	var (
		batches      [][]json.RawMessage
		current      = newBatch(maxItems, len(items))
		currentBytes int
	)

	for _, item := range items {
		itemBytes := len(item)
		exceedsItems := maxItems != nil && len(current) >= *maxItems
		exceedsBytes := maxBytes != nil && len(current) > 0 && currentBytes+itemBytes > *maxBytes

		if exceedsItems || exceedsBytes {
			batches = append(batches, current)
			current = newBatch(maxItems, len(items))
			currentBytes = 0
		}

		current = append(current, item)
		currentBytes += itemBytes
	}

	if len(current) > 0 {
		batches = append(batches, current)
	}

	return batches
}

// newBatch allocates a batch slice, sized to maxItems when set so
// groupIntoBatches does not repeatedly grow it via append.
func newBatch(maxItems *int, totalItems int) []json.RawMessage {
	capacity := totalItems
	if maxItems != nil && *maxItems < capacity {
		capacity = *maxItems
	}

	return make([]json.RawMessage, 0, capacity)
}

// marshalBatchEnvelope builds one child processor invocation's input JSON.
// Uses a plain map so the wire keys "BatchInput"/"Items" (must match AWS's
// envelope exactly) don't require a struct-tag casing exception.
func marshalBatchEnvelope(batchInput map[string]any, items []json.RawMessage) (string, error) {
	envelope := map[string]any{
		"BatchInput": batchInput,
		"Items":      items,
	}

	encoded, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("marshal batch envelope: %w", err)
	}

	return string(encoded), nil
}
