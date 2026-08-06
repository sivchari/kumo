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

// ResultWriter.WriterConfig.OutputType values.
const (
	resultWriterOutputTypeJSON  = "JSON"
	resultWriterOutputTypeJSONL = "JSONL"
)

// ResultWriter.WriterConfig.Transformation values.
const (
	resultWriterTransformationCompact = "COMPACT"
	resultWriterTransformationFlatten = "FLATTEN"
	resultWriterTransformationNone    = "NONE"
)

// resultWriterDef is the decoded ResultWriter field. JSON tags are
// lowerCamelCase; see itemBatcherDef in itembatcher.go for why.
type resultWriterDef struct {
	Resource     string          `json:"resource"`
	Parameters   map[string]any  `json:"parameters"`
	WriterConfig json.RawMessage `json:"writerConfig"`
}

// resultWriterConfig is the decoded ResultWriter.WriterConfig field (see
// applyResultWriterTransformation/marshalResultWriterBody for how each
// value is applied).
type resultWriterConfig struct {
	OutputType     string `json:"outputType"`
	Transformation string `json:"transformation"`
}

// writeMapResultToS3 uploads outputs as a single combined file (a
// deliberate emulator simplification of AWS's manifest.json + per-status
// SUCCEEDED_n.json/FAILED_n.json layout), returning the documented
// {MapRunArn, ResultWriterDetails: {Bucket, Key}} shape. mapRunArn is
// omitted when "" (Map Run could not be recorded).
func writeMapResultToS3(ctx context.Context, e *executionEngine, raw json.RawMessage, outputs []json.RawMessage, mapInput, mapRunArn string) (string, error) {
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

	var cfg resultWriterConfig
	if len(def.WriterConfig) > 0 {
		if err := json.Unmarshal(def.WriterConfig, &cfg); err != nil {
			return "", fmt.Errorf("parse ResultWriter WriterConfig: %w", err)
		}
	}

	transformed, err := applyResultWriterTransformation(outputs, cfg.Transformation)
	if err != nil {
		return "", fmt.Errorf("resultWriter: %w", err)
	}

	body, err := marshalResultWriterBody(transformed, cfg.OutputType)
	if err != nil {
		return "", fmt.Errorf("resultWriter: %w", err)
	}

	key := resultWriterKey(prefix)

	if err := e.putS3Object(ctx, bucket, key, body); err != nil {
		return "", fmt.Errorf("resultWriter: %w", err)
	}

	return marshalResultWriterOutput(bucket, key, mapRunArn)
}

// applyResultWriterTransformation reshapes outputs per
// WriterConfig.Transformation: "" or COMPACT (the default) passes outputs
// through unchanged; FLATTEN splices any array-valued entry's elements into
// the result. NONE is rejected -- it requires per-item child-execution
// metadata (ExecutionArn, Status, ...) kumo doesn't track since Map units
// run in-process rather than as real child executions.
func applyResultWriterTransformation(outputs []json.RawMessage, transformation string) ([]json.RawMessage, error) {
	switch strings.ToUpper(transformation) {
	case "", resultWriterTransformationCompact:
		return outputs, nil
	case resultWriterTransformationFlatten:
		return flattenResultWriterOutputs(outputs), nil
	case resultWriterTransformationNone:
		return nil, fmt.Errorf(
			"writerConfig.Transformation %q is not implemented: it requires per-item child-execution metadata kumo does not track for Map units; use COMPACT or FLATTEN instead",
			resultWriterTransformationNone,
		)
	default:
		return nil, fmt.Errorf("unsupported WriterConfig.Transformation %q", transformation)
	}
}

// flattenResultWriterOutputs splices array-valued entries' elements into the
// result; other entries pass through unchanged. A JSON null entry is
// dropped this way too, matching a failed unit contributing nothing.
func flattenResultWriterOutputs(outputs []json.RawMessage) []json.RawMessage {
	flattened := make([]json.RawMessage, 0, len(outputs))

	for _, out := range outputs {
		var elems []json.RawMessage
		if json.Unmarshal(out, &elems) == nil {
			flattened = append(flattened, elems...)

			continue
		}

		flattened = append(flattened, out)
	}

	return flattened
}

// marshalResultWriterBody serializes outputs per WriterConfig.OutputType:
// "" or JSON (the default) as a single JSON array document; JSONL as
// newline-delimited JSON documents, one per entry.
func marshalResultWriterBody(outputs []json.RawMessage, outputType string) ([]byte, error) {
	switch strings.ToUpper(outputType) {
	case "", resultWriterOutputTypeJSON:
		body, err := json.Marshal(outputs)
		if err != nil {
			return nil, fmt.Errorf("marshal ResultWriter output: %w", err)
		}

		return body, nil
	case resultWriterOutputTypeJSONL:
		var buf bytes.Buffer

		for _, out := range outputs {
			buf.Write(out)
			buf.WriteByte('\n')
		}

		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("unsupported WriterConfig.OutputType %q", outputType)
	}
}

// resultWriterKey builds the combined result file's object key under Prefix.
func resultWriterKey(prefix string) string {
	if prefix == "" {
		return resultWriterFileName
	}

	return strings.TrimSuffix(prefix, "/") + "/" + resultWriterFileName
}

// marshalResultWriterOutput builds the Map state's
// {MapRunArn, ResultWriterDetails: {Bucket, Key}} output. Uses a plain map
// so the wire keys need not be exempted from tagliatelle.
func marshalResultWriterOutput(bucket, key, mapRunArn string) (string, error) {
	result := map[string]any{
		"ResultWriterDetails": map[string]any{
			"Bucket": bucket,
			"Key":    key,
		},
	}

	if mapRunArn != "" {
		result["MapRunArn"] = mapRunArn
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
