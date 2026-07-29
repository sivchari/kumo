package sfn

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// distributedItemProcessorJSON is a minimal echo ItemProcessor running in
// Distributed mode, required for every ItemReader/ItemBatcher/ResultWriter/
// ToleratedFailure* test (see requireDistributedModeForFields).
const distributedItemProcessorJSON = `{
	"ProcessorConfig": {"Mode": "DISTRIBUTED", "ExecutionType": "STANDARD"},
	"StartAt": "Echo",
	"States": {"Echo": {"Type": "Pass", "End": true}}
}`

// itemReaderMapDefinition builds a single-state Map state machine definition
// reading its items via ItemReader, with an echo ItemProcessor.
func itemReaderMapDefinition(readerConfigJSON string) string {
	return fmt.Sprintf(`{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ItemReader": {
					"Resource": "arn:aws:states:::s3:getObject",
					"Parameters": {"Bucket": "src-bucket", "Key": "items.dat"},
					"ReaderConfig": %s
				},
				"ItemProcessor": %s,
				"End": true
			}
		}
	}`, readerConfigJSON, distributedItemProcessorJSON)
}

func TestMapStateItemReaderJSONArray(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)
	server.putObject("src-bucket", "items.dat", `[{"n":1},{"n":2},{"n":3}]`)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-itemreader-json", itemReaderMapDefinition(`{"InputType": "JSON"}`))

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{}`)

	if want := `[{"n":1},{"n":2},{"n":3}]`; exec.Output != want {
		t.Fatalf("execution output: got %q, want %q", exec.Output, want)
	}
}

func TestMapStateItemReaderJSONL(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)
	server.putObject("src-bucket", "items.dat", "{\"n\":1}\n{\"n\":2}\n")

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-itemreader-jsonl", itemReaderMapDefinition(`{"InputType": "JSONL"}`))

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{}`)

	if want := `[{"n":1},{"n":2}]`; exec.Output != want {
		t.Fatalf("execution output: got %q, want %q", exec.Output, want)
	}
}

func TestMapStateItemReaderCSVFirstRowHeader(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)
	server.putObject("src-bucket", "items.dat", "id,name\n1,alice\n2,bob\n")

	readerConfig := `{"InputType": "CSV", "CSVHeaderLocation": "FIRST_ROW"}`
	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-itemreader-csv", itemReaderMapDefinition(readerConfig))

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{}`)

	var outputs []map[string]string
	if err := json.Unmarshal([]byte(exec.Output), &outputs); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	want := []map[string]string{
		{"id": "1", "name": "alice"},
		{"id": "2", "name": "bob"},
	}

	if len(outputs) != len(want) || outputs[0]["id"] != want[0]["id"] || outputs[0]["name"] != want[0]["name"] ||
		outputs[1]["id"] != want[1]["id"] || outputs[1]["name"] != want[1]["name"] {
		t.Fatalf("csv rows: got %+v, want %+v", outputs, want)
	}
}

func TestMapStateItemReaderMaxItems(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)
	server.putObject("src-bucket", "items.dat", `[1,2,3,4,5]`)

	readerConfig := `{"InputType": "JSON", "MaxItems": 2}`
	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-itemreader-maxitems", itemReaderMapDefinition(readerConfig))

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `{}`)

	if want := `[1,2]`; exec.Output != want {
		t.Fatalf("execution output: got %q, want %q", exec.Output, want)
	}
}

func TestMapStateItemReaderMissingObjectFails(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)
	// No object seeded at src-bucket/items.dat.

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-itemreader-missing", itemReaderMapDefinition(`{"InputType": "JSON"}`))

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, `{}`)

	if exec.Error != errorStatesRuntime {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesRuntime)
	}

	if !strings.Contains(exec.Cause, "404") {
		t.Fatalf("execution cause: got %q, want it to mention the 404 status", exec.Cause)
	}
}
