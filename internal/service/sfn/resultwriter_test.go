package sfn

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestMapStateResultWriterExportsToS3(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)

	resultWriter := `{
		"Resource": "arn:aws:states:::s3:putObject",
		"Parameters": {"Bucket": "dst-bucket", "Prefix": "out"}
	}`

	definition := fmt.Sprintf(`{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ResultWriter": %s,
				"ItemProcessor": %s,
				"End": true
			}
		}
	}`, resultWriter, distributedItemProcessorJSON)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-resultwriter", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, threeItemArrayJSON)

	// JSON tags are lowerCamelCase (encoding/json matches the wire's
	// PascalCase case-insensitively; see itemBatcherDef in itembatcher.go
	// for why).
	var output struct {
		ResultWriterDetails struct {
			Bucket string `json:"bucket"`
			Key    string `json:"key"`
		} `json:"resultWriterDetails"`
	}

	if err := json.Unmarshal([]byte(exec.Output), &output); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if output.ResultWriterDetails.Bucket != "dst-bucket" {
		t.Fatalf("ResultWriterDetails.Bucket: got %q, want %q", output.ResultWriterDetails.Bucket, "dst-bucket")
	}

	if want := "out/result.json"; output.ResultWriterDetails.Key != want {
		t.Fatalf("ResultWriterDetails.Key: got %q, want %q", output.ResultWriterDetails.Key, want)
	}

	put := server.lastPut(t)
	if put.bucket != "dst-bucket" || put.key != "out/result.json" {
		t.Fatalf("PUT target: got bucket=%q key=%q, want bucket=%q key=%q", put.bucket, put.key, "dst-bucket", "out/result.json")
	}

	if want := threeItemArrayJSON; string(put.body) != want {
		t.Fatalf("PUT body: got %q, want %q", string(put.body), want)
	}
}

// resultWriterWriterConfigDefinition builds a Map state exporting to S3
// with resultWriterExtra (a WriterConfig JSON fragment, or "") added to
// ResultWriter, using a plain echo ItemProcessor: each item is itself
// already a one-element JSON array (see resultWriterWriterConfigInput), so
// each unit's own output is an array -- exactly the shape
// Transformation FLATTEN needs something to flatten.
func resultWriterWriterConfigDefinition(resultWriterExtra string) string {
	resultWriter := fmt.Sprintf(`{
		"Resource": "arn:aws:states:::s3:putObject",
		"Parameters": {"Bucket": "dst-bucket", "Prefix": "out"}
		%s
	}`, resultWriterExtra)

	return fmt.Sprintf(`{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ResultWriter": %s,
				"ItemProcessor": %s,
				"End": true
			}
		}
	}`, resultWriter, distributedItemProcessorJSON)
}

// resultWriterWriterConfigInput is resultWriterWriterConfigDefinition's Map
// input: three one-element arrays, one per item/unit.
const resultWriterWriterConfigInput = `[[1],[2],[3]]`

func TestMapStateResultWriterOutputIncludesMapRunArn(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)

	resultWriter := `{
		"Resource": "arn:aws:states:::s3:putObject",
		"Parameters": {"Bucket": "dst-bucket", "Prefix": "out"}
	}`

	definition := fmt.Sprintf(`{
		"StartAt": "Each",
		"States": {
			"Each": {
				"Type": "Map",
				"ResultWriter": %s,
				"ItemProcessor": %s,
				"End": true
			}
		}
	}`, resultWriter, distributedItemProcessorJSON)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-resultwriter-maprunarn", definition)

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `[1,2]`)

	var output struct {
		MapRunArn string `json:"mapRunArn"`
	}

	if err := json.Unmarshal([]byte(exec.Output), &output); err != nil {
		t.Fatalf("unmarshal execution output %q: %v", exec.Output, err)
	}

	if output.MapRunArn == "" {
		t.Fatalf("execution output %q: want a non-empty MapRunArn", exec.Output)
	}

	if !strings.Contains(output.MapRunArn, ":mapRun:") {
		t.Fatalf("MapRunArn %q: want it to contain the \":mapRun:\" ARN segment", output.MapRunArn)
	}
}

func TestMapStateResultWriterWriterConfigJSONLOutputType(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)

	definition := resultWriterWriterConfigDefinition(`, "WriterConfig": {"OutputType": "JSONL"}`)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-resultwriter-jsonl", definition)

	startAndAwaitSuccess(t, store, sm.StateMachineArn, resultWriterWriterConfigInput)

	put := server.lastPut(t)

	if want := "[1]\n[2]\n[3]\n"; string(put.body) != want {
		t.Fatalf("PUT body (JSONL): got %q, want %q", string(put.body), want)
	}
}

func TestMapStateResultWriterWriterConfigFlatten(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)

	definition := resultWriterWriterConfigDefinition(`, "WriterConfig": {"Transformation": "FLATTEN"}`)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-resultwriter-flatten", definition)

	startAndAwaitSuccess(t, store, sm.StateMachineArn, resultWriterWriterConfigInput)

	put := server.lastPut(t)

	if want := threeItemArrayJSON; string(put.body) != want {
		t.Fatalf("PUT body (FLATTEN): got %q, want %q", string(put.body), want)
	}
}

func TestMapStateResultWriterWriterConfigCompactIsDefaultAndUnflattened(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)

	definition := resultWriterWriterConfigDefinition("")

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-resultwriter-compact-default", definition)

	startAndAwaitSuccess(t, store, sm.StateMachineArn, resultWriterWriterConfigInput)

	put := server.lastPut(t)

	if want := `[[1],[2],[3]]`; string(put.body) != want {
		t.Fatalf("PUT body (default/COMPACT): got %q, want %q", string(put.body), want)
	}
}

func TestMapStateResultWriterWriterConfigTransformationNoneIsUnimplemented(t *testing.T) {
	t.Parallel()

	server := newFakeAWSServer(t)

	definition := resultWriterWriterConfigDefinition(`, "WriterConfig": {"Transformation": "NONE"}`)

	store := NewMemoryStorage(WithBaseURL(server.URL))
	sm := createExecutionTestStateMachine(t, store, "map-resultwriter-none", definition)

	exec := startAndAwaitFailure(t, store, sm.StateMachineArn, resultWriterWriterConfigInput)

	if exec.Error != errorStatesRuntime {
		t.Fatalf("execution error: got %q, want %q", exec.Error, errorStatesRuntime)
	}

	if !strings.Contains(exec.Cause, "NONE") {
		t.Fatalf("execution cause: got %q, want it to mention NONE", exec.Cause)
	}
}
