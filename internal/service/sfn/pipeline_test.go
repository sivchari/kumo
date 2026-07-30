package sfn

import (
	"fmt"
	"testing"
)

// TestTaskStatePipelineAppliesFullDataFlow exercises a Task state's full
// standard field pipeline end to end: InputPath narrows the raw input
// before it becomes the Lambda payload, ResultSelector reshapes the Lambda
// response, ResultPath merges it back into the (InputPath-filtered)
// effective input, and OutputPath narrows the merged output.
func TestTaskStatePipelineAppliesFullDataFlow(t *testing.T) {
	t.Parallel()

	server := newEchoLambdaServer(t)

	definition := fmt.Sprintf(`{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:%s",
				"InputPath": "$.payload",
				"ResultSelector": {"echoed.$": "$.greeting"},
				"ResultPath": "$.result",
				"OutputPath": "$.result",
				"End": true
			}
		}
	}`, "echo-fn")

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "task-pipeline", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"payload":{"greeting":"hi"},"noise":"ignore-me"}`)

	if exec.Output != `{"echoed":"hi"}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"echoed":"hi"}`)
	}
}

// TestPassStatePipelineAppliesResultAndResultPath mirrors the ASL spec's
// own worked example: a Pass state's static Result is merged into its
// input at ResultPath, rather than replacing it outright.
func TestPassStatePipelineAppliesResultAndResultPath(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "ProvideTestData",
		"States": {
			"ProvideTestData": {
				"Type": "Pass",
				"Result": {"x-datum": 0.381018, "y-datum": 622.2269926397355},
				"ResultPath": "$.coords",
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "pass-resultpath", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"georefOf":"Home"}`)

	assertJSONEqual(t, exec.Output, `{"georefOf":"Home","coords":{"x-datum":0.381018,"y-datum":622.2269926397355}}`)
}

// TestPassStatePipelineDefaultsToPassThrough checks that a Pass state with
// neither Result nor ResultPath simply copies its input to its output.
func TestPassStatePipelineDefaultsToPassThrough(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "NoOp",
		"States": {"NoOp": {"Type": "Pass", "End": true}}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "pass-passthrough", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"a":1}`)

	if exec.Output != `{"a":1}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{"a":1}`)
	}
}

// TestParallelStatePipelineAppliesResultSelectorAndResultPath checks that a
// Parallel state's branch-output array passes through ResultSelector
// (which, per the spec, resolves "$." references against the result, here
// "$" selecting the whole array) before ResultPath merges it into the
// state's effective input.
func TestParallelStatePipelineAppliesResultSelectorAndResultPath(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Fork",
		"States": {
			"Fork": {
				"Type": "Parallel",
				"Branches": [
					{"StartAt": "A", "States": {"A": {"Type": "Pass", "Result": {"branch": "a"}, "End": true}}},
					{"StartAt": "B", "States": {"B": {"Type": "Pass", "Result": {"branch": "b"}, "End": true}}}
				],
				"ResultSelector": {"all.$": "$"},
				"ResultPath": "$.parallelResult",
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "parallel-pipeline", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"seed":1}`)

	assertJSONEqual(t, exec.Output, `{"seed":1,"parallelResult":{"all":[{"branch":"a"},{"branch":"b"}]}}`)
}

// TestMapStatePipelineAppliesResultSelectorAndResultPath checks the same
// ResultSelector/ResultPath combination for a Map state, whose ItemsPath
// resolves against the InputPath-filtered effective input.
func TestMapStatePipelineAppliesResultSelectorAndResultPath(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemsPath": "$.items",
				"ItemProcessor": {"StartAt": "Tag", "States": {"Tag": {"Type": "Pass", "Result": {"doubled": true}, "End": true}}},
				"ResultSelector": {"outputs.$": "$"},
				"ResultPath": "$.mapResult",
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-pipeline", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"items":[1,2,3]}`)

	assertJSONEqual(t, exec.Output, `{
		"items": [1, 2, 3],
		"mapResult": {"outputs": [{"doubled": true}, {"doubled": true}, {"doubled": true}]}
	}`)
}

// TestWaitStatePipelineAppliesInputPathAndOutputPath checks that a Wait
// state resolves its duration against the InputPath-filtered effective
// input and narrows its (pass-through) output with OutputPath.
func TestWaitStatePipelineAppliesInputPathAndOutputPath(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "W",
		"States": {
			"W": {
				"Type": "Wait",
				"InputPath": "$.payload",
				"Seconds": 0,
				"OutputPath": "$.value",
				"Next": "Done"
			},
			"Done": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "wait-pipeline", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"payload":{"value":1},"noise":true}`)

	if exec.Output != `1` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `1`)
	}
}

// TestChoiceStatePipelineAppliesInputPathAndOutputPath checks that a
// Choice state's rules are evaluated against the InputPath-filtered
// effective input, and that OutputPath narrows the (pass-through) output
// used as the matched Next state's input.
func TestChoiceStatePipelineAppliesInputPathAndOutputPath(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Decide",
		"States": {
			"Decide": {
				"Type": "Choice",
				"InputPath": "$.payload",
				"OutputPath": "$.mode",
				"Choices": [{"Variable": "$.mode", "StringEquals": "fast", "Next": "Done"}],
				"Default": "Done"
			},
			"Done": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "choice-pipeline", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"payload":{"mode":"fast"},"noise":true}`)

	if exec.Output != `"fast"` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `"fast"`)
	}
}

// TestSucceedStateNullInputPathYieldsEmptyObject checks the ASL spec's
// explicit-null semantics: a null InputPath discards the raw input
// entirely, yielding an empty JSON object as the effective input/output.
func TestSucceedStateNullInputPathYieldsEmptyObject(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Done",
		"States": {
			"Done": {"Type": "Succeed", "InputPath": null}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "succeed-null-inputpath", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{"result":"ok"}`)

	if exec.Output != `{}` {
		t.Fatalf("execution output: got %q, want %q", exec.Output, `{}`)
	}
}

// TestTaskResultPathFailureReportsUncatchableStatesRuntime checks
// requirement: a data-flow field failure that occurs after a Task's own
// work already succeeded (here, ResultPath cannot apply because the
// InputPath-filtered effective input is a JSON string rather than an
// object) reports as States.Runtime and is never caught, even by an
// explicit States.ALL Catcher -- matching AWS's documented States.Runtime
// behavior for InputPath/OutputPath/ResultPath failures.
func TestTaskResultPathFailureReportsUncatchableStatesRuntime(t *testing.T) {
	t.Parallel()

	server := newEchoLambdaServer(t)

	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:echo-fn",
				"InputPath": "$.payload",
				"ResultPath": "$.x",
				"Catch": [{"ErrorEquals": ["States.ALL"], "Next": "Fallback"}],
				"End": true
			},
			"Fallback": {"Type": "Pass", "End": true}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "task-resultpath-runtime-error", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{"payload":"just-a-string"}`)

	if exec.Error != errorStatesRuntime {
		t.Fatalf("execution error: got %q, want %q (States.ALL must not catch a ResultPath failure)", exec.Error, errorStatesRuntime)
	}
}

// TestTaskOutputPathFailureReportsStatesRuntime checks that an OutputPath
// which matches nothing also reports States.Runtime, per AWS's
// documentation that States.Runtime is often caused by errors such as
// attempting to apply InputPath or OutputPath on a null JSON payload.
func TestTaskOutputPathFailureReportsStatesRuntime(t *testing.T) {
	t.Parallel()

	server := newEchoLambdaServer(t)

	definition := `{
		"StartAt": "Invoke",
		"States": {
			"Invoke": {
				"Type": "Task",
				"Resource": "arn:aws:lambda:us-east-1:000000000000:function:echo-fn",
				"OutputPath": "$.missing",
				"End": true
			}
		}
	}`

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "task-outputpath-runtime-error", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{"present":true}`)

	if exec.Error != errorStatesRuntime {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesRuntime)
	}
}
