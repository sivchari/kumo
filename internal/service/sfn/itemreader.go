package sfn

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ItemReader.ReaderConfig.InputType values kumo implements.
const (
	itemReaderInputTypeJSON  = "JSON"
	itemReaderInputTypeJSONL = "JSONL"
	itemReaderInputTypeCSV   = "CSV"
)

// ItemReader.ReaderConfig.CSVHeaderLocation values.
const (
	csvHeaderLocationFirstRow = "FIRST_ROW"
	csvHeaderLocationGiven    = "GIVEN"
)

// ItemReader.Resource values. Only s3:getObject is implemented;
// s3:listObjectsV2 is documented but out of scope (see readItemsFromS3).
const (
	itemReaderResourceS3GetObject     = "arn:aws:states:::s3:getObject"
	itemReaderResourceS3ListObjectsV2 = "arn:aws:states:::s3:listObjectsV2"
)

// itemReaderDef is the decoded ItemReader field. JSON tags are
// lowerCamelCase; see itemBatcherDef in itembatcher.go for why.
type itemReaderDef struct {
	Resource     string           `json:"resource"`
	Parameters   map[string]any   `json:"parameters"`
	ReaderConfig itemReaderConfig `json:"readerConfig"`
}

// itemReaderConfig is ItemReader.ReaderConfig. ItemsPointer and
// CSVDelimiter are not implemented. MaxItemsPath is decoded only so
// validate.go can flag it as unimplemented; the engine never reads it (see
// applyMaxItems, which only reads the static MaxItems).
type itemReaderConfig struct {
	InputType         string   `json:"inputType"`
	CSVHeaderLocation string   `json:"csvHeaderLocation"`
	CSVHeaders        []string `json:"csvHeaders"`
	MaxItems          *int     `json:"maxItems"`
	MaxItemsPath      string   `json:"maxItemsPath"`
}

// readItemsFromS3 resolves a Map state's ItemReader field into the item
// list it selects, fetching the underlying object from kumo's own S3 over
// plain HTTP (see (*executionEngine).getS3Object).
func readItemsFromS3(ctx context.Context, e *executionEngine, raw json.RawMessage, mapInput string) ([]json.RawMessage, error) {
	var def itemReaderDef
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, fmt.Errorf("parse ItemReader: %w", err)
	}

	if def.Resource != itemReaderResourceS3GetObject {
		if def.Resource == itemReaderResourceS3ListObjectsV2 {
			return nil, fmt.Errorf("itemReader resource %q is not implemented; only %q is supported", def.Resource, itemReaderResourceS3GetObject)
		}

		return nil, fmt.Errorf("unsupported ItemReader resource %q", def.Resource)
	}

	params, err := resolveParameters(def.Parameters, mapInput)
	if err != nil {
		return nil, fmt.Errorf("resolve ItemReader Parameters: %w", err)
	}

	bucket, _ := params["Bucket"].(string)
	key, _ := params["Key"].(string)

	if bucket == "" || key == "" {
		return nil, fmt.Errorf("itemReader s3:getObject requires Parameters.Bucket and Parameters.Key")
	}

	body, err := e.getS3Object(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("itemReader: %w", err)
	}

	items, err := parseItemReaderBody(body, &def.ReaderConfig)
	if err != nil {
		return nil, fmt.Errorf("itemReader: %w", err)
	}

	return applyMaxItems(items, def.ReaderConfig.MaxItems), nil
}

// getS3Object fetches an object from kumo's own S3 service using the same
// path-style addressing S3 itself registers its routes under
// (GET /{bucket}/{key}).
func (e *executionEngine) getS3Object(ctx context.Context, bucket, key string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s", e.baseURL, bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request for s3://%s/%s: %w", bucket, key, err)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get s3://%s/%s: %w", bucket, key, err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read s3://%s/%s: %w", bucket, key, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get s3://%s/%s: unexpected status %d: %s", bucket, key, resp.StatusCode, string(body))
	}

	return body, nil
}

// parseItemReaderBody parses an S3 object's body per ReaderConfig.InputType.
// An empty InputType defaults to JSON, matching the plain-JSON-array
// dataset AWS documents as the simplest ItemReader case.
func parseItemReaderBody(body []byte, cfg *itemReaderConfig) ([]json.RawMessage, error) {
	switch strings.ToUpper(cfg.InputType) {
	case itemReaderInputTypeJSON, "":
		return parseJSONArrayBody(body)
	case itemReaderInputTypeJSONL:
		return parseJSONLBody(body)
	case itemReaderInputTypeCSV:
		return parseCSVBody(body, cfg)
	default:
		return nil, fmt.Errorf("unsupported ReaderConfig.InputType %q", cfg.InputType)
	}
}

// parseJSONArrayBody parses ReaderConfig.InputType "JSON": a single JSON
// document containing an array.
func parseJSONArrayBody(body []byte) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("parse JSON array: %w", err)
	}

	return items, nil
}

// parseJSONLBody parses ReaderConfig.InputType "JSONL": one JSON document
// per line, blank lines skipped.
func parseJSONLBody(body []byte) ([]json.RawMessage, error) {
	var items []json.RawMessage

	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var v json.RawMessage
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			return nil, fmt.Errorf("parse JSONL line: %w", err)
		}

		items = append(items, v)
	}

	return items, nil
}

// parseCSVBody parses ReaderConfig.InputType "CSV", turning each data row
// into a JSON object keyed by the resolved header (see csvHeaders).
func parseCSVBody(body []byte, cfg *itemReaderConfig) ([]json.RawMessage, error) {
	reader := csv.NewReader(bytes.NewReader(body))
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}

	headers, rows, err := csvHeaders(records, cfg)
	if err != nil {
		return nil, err
	}

	items := make([]json.RawMessage, len(rows))

	for i, row := range rows {
		encoded, err := json.Marshal(csvRowToObject(headers, row))
		if err != nil {
			return nil, fmt.Errorf("marshal CSV row %d: %w", i, err)
		}

		items[i] = encoded
	}

	return items, nil
}

// csvHeaders resolves a CSV dataset's column headers and the remaining data
// rows, per ReaderConfig.CSVHeaderLocation. An empty CSVHeaderLocation
// defaults to FIRST_ROW, matching the ItemReader (Map) dev guide example.
func csvHeaders(records [][]string, cfg *itemReaderConfig) (headers []string, rows [][]string, err error) {
	switch cfg.CSVHeaderLocation {
	case csvHeaderLocationGiven:
		if len(cfg.CSVHeaders) == 0 {
			return nil, nil, fmt.Errorf("readerConfig.CSVHeaderLocation %q requires CSVHeaders", csvHeaderLocationGiven)
		}

		return cfg.CSVHeaders, records, nil
	case csvHeaderLocationFirstRow, "":
		if len(records) == 0 {
			return nil, nil, nil
		}

		return records[0], records[1:], nil
	default:
		return nil, nil, fmt.Errorf("unsupported ReaderConfig.CSVHeaderLocation %q", cfg.CSVHeaderLocation)
	}
}

// csvRowToObject zips a CSV data row with its headers into a JSON object,
// dropping any header without a corresponding column in a short row.
func csvRowToObject(headers, row []string) map[string]string {
	obj := make(map[string]string, len(headers))

	for i, header := range headers {
		if i < len(row) {
			obj[header] = row[i]
		}
	}

	return obj
}

// applyMaxItems trims items to ReaderConfig.MaxItems, per item order,
// leaving items unchanged when maxItems is unset or not smaller than
// len(items).
func applyMaxItems(items []json.RawMessage, maxItems *int) []json.RawMessage {
	if maxItems == nil || *maxItems < 0 || *maxItems >= len(items) {
		return items
	}

	return items[:*maxItems]
}
