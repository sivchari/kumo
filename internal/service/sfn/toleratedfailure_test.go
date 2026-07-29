package sfn

import (
	"encoding/json"
	"fmt"
	"testing"
)

// conditionalFailProcessorJSON is a Distributed-mode ItemProcessor that
// fails an item whose "ok" field is not true, so tolerance tests can
// control exactly which items fail without any external HTTP dependency.
const conditionalFailProcessorJSON = `{
	"ProcessorConfig": {"Mode": "DISTRIBUTED", "ExecutionType": "STANDARD"},
	"StartAt": "Check",
	"States": {
		"Check": {
			"Type": "Choice",
			"Choices": [{"Variable": "$.ok", "BooleanEquals": true, "Next": "Ok"}],
			"Default": "Bad"
		},
		"Ok": {"Type": "Pass", "End": true},
		"Bad": {"Type": "Fail", "Error": "ItemFailed", "Cause": "item was not ok"}
	}
}`

// toleratedFailureMapDefinition builds a Map state using
// conditionalFailProcessorJSON with the given ToleratedFailure* field set
// verbatim in the state body (e.g. `"ToleratedFailurePercentage": 50`).
func toleratedFailureMapDefinition(toleranceField string) string {
	return fmt.Sprintf(`{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				%s,
				"ItemProcessor": %s,
				"End": true
			}
		}
	}`, toleranceField, conditionalFailProcessorJSON)
}

// oneOfFourFailsInput is one failing item ("ok": false) out of four (25%).
const oneOfFourFailsInput = `[{"ok":true},{"ok":true},{"ok":false},{"ok":true}]`

func TestMapStateToleratedFailurePercentageWithinToleranceSucceeds(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-tolerated-pct-ok", toleratedFailureMapDefinition(`"ToleratedFailurePercentage": 50`))

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, oneOfFourFailsInput)

	var outputs []json.RawMessage
	if err := json.Unmarshal([]byte(exec.Output), &outputs); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if len(outputs) != 4 {
		t.Fatalf("outputs: got %d entries, want 4", len(outputs))
	}

	if string(outputs[2]) != "null" {
		t.Fatalf("failed item output: got %s, want null", outputs[2])
	}

	if string(outputs[0]) == "null" {
		t.Fatalf("succeeded item output: got null, want the item's own output")
	}
}

func TestMapStateToleratedFailurePercentageExceedingToleranceFails(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-tolerated-pct-fail", toleratedFailureMapDefinition(`"ToleratedFailurePercentage": 10`))

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, oneOfFourFailsInput)

	if exec.Error != errorStatesExceedToleratedFailureThreshold {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesExceedToleratedFailureThreshold)
	}
}

func TestMapStateToleratedFailureCountWithinToleranceSucceeds(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-tolerated-count-ok", toleratedFailureMapDefinition(`"ToleratedFailureCount": 1`))

	startAndAwaitSuccess(t, store, sm.StateMachineArn, oneOfFourFailsInput)
}

func TestMapStateToleratedFailureCountExceedingToleranceFails(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-tolerated-count-fail", toleratedFailureMapDefinition(`"ToleratedFailureCount": 0`))

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, oneOfFourFailsInput)

	if exec.Error != errorStatesExceedToleratedFailureThreshold {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesExceedToleratedFailureThreshold)
	}
}
