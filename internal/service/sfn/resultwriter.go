package sfn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ResultWriter.Resource. Only s3:putObject is implemented.
const resultWriterResourceS3PutObject = "arn:aws:states:::s3:putObject"

// resultWriterFileName is the single combined result file kumo writes for
// every Map Run (see writeMapResultToS3 for why this deviates from AWS's
// real multi-file layout).
const resultWriterFileName = "result.json"

// resultWriterDef is the decoded ResultWriter field. JSON tags are
// lowerCamelCase; see itemBatcherDef in itembatcher.go for why.
//
// WriterConfig (Transformation/OutputType) is not implemented: kumo always
// writes the plain JSON array of item outputs, equivalent to AWS's
// Transformation "COMPACT" with OutputType "JSON". It is decoded only so
// validate.go can flag it as unimplemented.
type resultWriterDef struct {
	Resource     string          `json:"resource"`
	Parameters   map[string]any  `json:"parameters"`
	WriterConfig json.RawMessage `json:"writerConfig"`
}

// writeMapResultToS3 uploads outputs -- the Map state's would-be plain-array
// result -- as a single JSON file to kumo's S3 under Parameters.Prefix, and
// returns the Map state's output in AWS's documented
// ResultWriterDetails{Bucket, Key} shape (see
// https://docs.aws.amazon.com/step-functions/latest/dg/input-output-resultwriter.html#export-s3).
//
// This is a deliberate emulator simplification of AWS's real behavior: AWS
// partitions results into a manifest.json plus per-status
// SUCCEEDED_n.json/FAILED_n.json files, and the top-level result also
// carries a MapRunArn. kumo has no Map Run resource (DescribeMapRun/
// ListMapRuns and child-execution tracking are out of scope, see
// mapstate.go), so it writes one combined result file and omits MapRunArn.
func writeMapResultToS3(ctx context.Context, e *executionEngine, raw json.RawMessage, outputs []json.RawMessage, mapInput string) (string, error) {
	var def resultWriterDef
	if err := json.Unmarshal(raw, &def); err != nil {
		return "", fmt.Errorf("parse ResultWriter: %w", err)
	}

	if def.Resource != resultWriterResourceS3PutObject {
		return "", fmt.Errorf("unsupported ResultWriter resource %q", def.Resource)
	}

	params, err := resolveParameters(def.Parameters, mapInput)
	if err != nil {
		return "", fmt.Errorf("resolve ResultWriter Parameters: %w", err)
	}

	bucket, _ := params["Bucket"].(string)
	prefix, _ := params["Prefix"].(string)

	if bucket == "" {
		return "", fmt.Errorf("resultWriter s3:putObject requires Parameters.Bucket")
	}

	key := resultWriterKey(prefix)

	body, err := json.Marshal(outputs)
	if err != nil {
		return "", fmt.Errorf("marshal ResultWriter output: %w", err)
	}

	if err := e.putS3Object(ctx, bucket, key, body); err != nil {
		return "", fmt.Errorf("resultWriter: %w", err)
	}

	return marshalResultWriterOutput(bucket, key)
}

// resultWriterKey builds the combined result file's object key under Prefix.
func resultWriterKey(prefix string) string {
	if prefix == "" {
		return resultWriterFileName
	}

	return strings.TrimSuffix(prefix, "/") + "/" + resultWriterFileName
}

// marshalResultWriterOutput builds the Map state's output when ResultWriter
// exported to S3: {"ResultWriterDetails": {"Bucket": ..., "Key": ...}}. A
// plain map is used so the wire keys need not be exempted from tagliatelle.
func marshalResultWriterOutput(bucket, key string) (string, error) {
	result := map[string]any{
		"ResultWriterDetails": map[string]any{
			"Bucket": bucket,
			"Key":    key,
		},
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal ResultWriter result: %w", err)
	}

	return string(encoded), nil
}

// putS3Object writes an object to kumo's own S3 service using the same
// path-style addressing S3 itself registers its routes under
// (PUT /{bucket}/{key}).
func (e *executionEngine) putS3Object(ctx context.Context, bucket, key string, body []byte) error {
	url := fmt.Sprintf("%s/%s/%s", e.baseURL, bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request for s3://%s/%s: %w", bucket, key, err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("put s3://%s/%s: %w", bucket, key, err)
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response for put s3://%s/%s: %w", bucket, key, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("put s3://%s/%s: unexpected status %d: %s", bucket, key, resp.StatusCode, string(respBody))
	}

	return nil
}
