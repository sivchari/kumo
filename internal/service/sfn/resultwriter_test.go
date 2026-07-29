package sfn

import (
	"encoding/json"
	"fmt"
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

	exec := startAndAwaitSuccess(t, store, sm.StateMachineArn, `[1,2,3]`)

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

	if want := `[1,2,3]`; string(put.body) != want {
		t.Fatalf("PUT body: got %q, want %q", string(put.body), want)
	}
}
