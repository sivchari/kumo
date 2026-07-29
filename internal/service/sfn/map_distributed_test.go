package sfn

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMapStateItemReaderWithoutDistributedModeFailsAtRuntime verifies
// requireDistributedModeForFields: ItemReader is a Distributed-mode-only
// field (see mapstate.go), so setting it on an Inline-mode ItemProcessor
// (the default when ProcessorConfig is absent) must fail the execution
// rather than silently ignore ItemReader.
func TestMapStateItemReaderWithoutDistributedModeFailsAtRuntime(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemReader": {
					"Resource": "arn:aws:states:::s3:getObject",
					"Parameters": {"Bucket": "b", "Key": "k"},
					"ReaderConfig": {"InputType": "JSON"}
				},
				"ItemProcessor": ` + echoItemProcessorJSON + `,
				"End": true
			}
		}
	}`

	store := NewMemoryStorage()
	sm := createExecutionTestStateMachine(t, store, "map-itemreader-inline-mode", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{}`)

	if exec.Error != errorStatesRuntime {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesRuntime)
	}

	if !strings.Contains(exec.Cause, "ItemReader") || !strings.Contains(exec.Cause, "DISTRIBUTED") {
		t.Fatalf("execution cause: got %q, want it to mention ItemReader and DISTRIBUTED", exec.Cause)
	}
}

// TestValidateMapDistributedOnlyFieldWarnsWithoutDistributedMode covers the
// validate.go half of the same rule: a definition using ItemReader without
// Distributed mode still validates OK (kumo never rejects a definition the
// engine can parse), but with a WARNING diagnostic.
func TestValidateMapDistributedOnlyFieldWarnsWithoutDistributedMode(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemReader": {"Resource": "arn:aws:states:::s3:getObject", "Parameters": {"Bucket": "b", "Key": "k"}},
				"ItemProcessor": ` + echoItemProcessorJSON + `,
				"End": true
			}
		}
	}`

	diagnostics := validateStateMachineDefinition(definition)

	tt := &diagnosticTest{
		wantSeverity: diagnosticSeverityWarning,
		wantCode:     codeUnsupportedRuntimeConstruct,
		wantLocation: "/States/Each/ItemReader",
	}
	assertHasDiagnostic(t, diagnostics, tt)
}

// TestValidateMapDistributedModeRequiresExecutionType covers AWS's
// documented requirement that ProcessorConfig.Mode "DISTRIBUTED" must also
// set ExecutionType.
func TestValidateMapDistributedModeRequiresExecutionType(t *testing.T) {
	t.Parallel()

	definition := `{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemProcessor": {
					"ProcessorConfig": {"Mode": "DISTRIBUTED"},
					"StartAt": "Echo",
					"States": {"Echo": {"Type": "Pass", "End": true}}
				},
				"End": true
			}
		}
	}`

	diagnostics := validateStateMachineDefinition(definition)

	tt := &diagnosticTest{
		wantSeverity: diagnosticSeverityError,
		wantCode:     codeSchemaValidationFailed,
		wantLocation: "/States/Each/ItemProcessor/ProcessorConfig/ExecutionType",
	}
	assertHasDiagnostic(t, diagnostics, tt)

	if got := diagnosticsResult(diagnostics); got != validationResultFail {
		t.Fatalf("result: got %q, want %q", got, validationResultFail)
	}
}

// TestMapStateDistributedEndToEnd combines ItemReader (S3 JSON array),
// ItemSelector (Context object), ItemBatcher, and a Task processor calling
// Lambda, exercising the full distributed-Map pipeline together.
func TestMapStateDistributedEndToEnd(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t).withEchoLambda()
	server.putObject("nums-bucket", "nums.json", `[1,2,3,4]`)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-distributed-end-to-end", endToEndDefinition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{}`)

	var results []endToEndBatchResult
	if err := json.Unmarshal([]byte(exec.Output), &results); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	assertEndToEndBatches(t, results)
}

// endToEndDefinition combines ItemReader (S3 JSON array), ItemSelector
// (Context object), ItemBatcher, and a Task processor calling Lambda.
const endToEndDefinition = `{
	"StartAt": "Each",
	"States": {
		"Each": {
			"Type": "Map",
			"ItemReader": {
				"Resource": "arn:aws:states:::s3:getObject",
				"Parameters": {"Bucket": "nums-bucket", "Key": "nums.json"},
				"ReaderConfig": {"InputType": "JSON"}
			},
			"ItemSelector": {"value.$": "$$.Map.Item.Value"},
			"ItemBatcher": {"MaxItemsPerBatch": 2},
			"ItemProcessor": {
				"ProcessorConfig": {"Mode": "DISTRIBUTED", "ExecutionType": "STANDARD"},
				"StartAt": "Invoke",
				"States": {
					"Invoke": {
						"Type": "Task",
						"Resource": "arn:aws:states:::lambda:invoke",
						"Parameters": {"FunctionName": "echo-fn", "Payload.$": "$"},
						"End": true
					}
				}
			},
			"End": true
		}
	}
}`

// endToEndBatchResult decodes one batch's Lambda-invoke Task output. JSON
// tags are lowerCamelCase (encoding/json matches the wire's PascalCase
// case-insensitively; see itemBatcherDef in itembatcher.go for why).
type endToEndBatchResult struct {
	StatusCode int `json:"statusCode"`
	Payload    struct {
		BatchInput map[string]any `json:"batchInput"`
		Items      []any          `json:"items"`
	} `json:"payload"`
}

// assertEndToEndBatches checks that ItemReader's four items, after
// ItemSelector and ItemBatcher(MaxItemsPerBatch: 2), reached the Lambda
// processor as two batches of the expected transformed items.
func assertEndToEndBatches(t *testing.T, results []endToEndBatchResult) {
	t.Helper()

	if len(results) != 2 {
		t.Fatalf("batches: got %d, want 2", len(results))
	}

	wantBatches := [][]any{
		{map[string]any{"value": float64(1)}, map[string]any{"value": float64(2)}},
		{map[string]any{"value": float64(3)}, map[string]any{"value": float64(4)}},
	}

	for i, want := range wantBatches {
		if results[i].StatusCode != 200 {
			t.Fatalf("batch %d StatusCode: got %d, want 200", i, results[i].StatusCode)
		}

		if !jsonDeepEqual(results[i].Payload.Items, want) {
			t.Fatalf("batch %d items: got %+v, want %+v", i, results[i].Payload.Items, want)
		}
	}
}
