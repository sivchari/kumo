package sfn

import "fmt"

// mapTolerance is a Map state's resolved ToleratedFailurePercentage/
// ToleratedFailureCount (see
// https://docs.aws.amazon.com/step-functions/latest/dg/state-map-distributed.html#state-map-fields-failure-tolerance).
type mapTolerance struct {
	percentage *float64
	count      *int
}

// resolveMapTolerance reads a Map state's tolerance fields, resolving
// ToleratedFailurePercentagePath/ToleratedFailureCountPath against input
// (the Map state's effective input) when set, preferring the Path form over
// its static counterpart -- see resolveOptionalFloatPath/
// resolveOptionalIntPath in wait.go. Only requireDistributedModeForFields
// lets these fields take effect at all (they are Distributed-mode-only), so
// resolveMapTolerance itself does not need to check mode.
func resolveMapTolerance(state *stateDefinition, input string) (mapTolerance, error) {
	percentage, err := resolveOptionalFloatPath(state.ToleratedFailurePercentage, state.ToleratedFailurePercentagePath, input)
	if err != nil {
		return mapTolerance{}, fmt.Errorf("resolve ToleratedFailurePercentagePath: %w", err)
	}

	count, err := resolveOptionalIntPath(state.ToleratedFailureCount, state.ToleratedFailureCountPath, input)
	if err != nil {
		return mapTolerance{}, fmt.Errorf("resolve ToleratedFailureCountPath: %w", err)
	}

	return mapTolerance{percentage: percentage, count: count}, nil
}

// enabled reports whether either threshold was configured. When neither is
// set, AWS's documented default (a 0% tolerated failure percentage) means
// any single item failure fails the Map Run; runMapProcessorUnits achieves
// that identical result more cheaply via the non-tolerant fast-fail path in
// runMapUnits, without needing every unit to run to completion first.
func (t mapTolerance) enabled() bool {
	return t.percentage != nil || t.count != nil
}

// exceeds reports whether failedItems out of totalItems exceeds the
// configured threshold. Per AWS docs, a Map Run fails if it exceeds either
// configured threshold when both are set.
func (t mapTolerance) exceeds(failedItems, totalItems int) bool {
	if t.count != nil && failedItems > *t.count {
		return true
	}

	if t.percentage != nil && totalItems > 0 {
		failedPercentage := float64(failedItems) / float64(totalItems) * 100
		if failedPercentage > *t.percentage {
			return true
		}
	}

	return false
}
