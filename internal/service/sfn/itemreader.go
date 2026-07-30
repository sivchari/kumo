package sfn

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
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

// ItemReader.Resource values.
const (
	itemReaderResourceS3GetObject     = "arn:aws:states:::s3:getObject"
	itemReaderResourceS3ListObjectsV2 = "arn:aws:states:::s3:listObjectsV2"
)

// itemReaderTransformationLoadAndFlatten is ReaderConfig.Transformation's
// only non-default value, meaningful only for s3:listObjectsV2: it reads
// and parses the actual object contents (per InputType) referenced by each
// listing entry, instead of returning the S3 object metadata itself. kumo
// does not implement it -- see readItemsFromS3ListObjectsV2 -- since it is
// effectively a second, nested ItemReader pass per listed object.
const itemReaderTransformationLoadAndFlatten = "LOAD_AND_FLATTEN"

// itemReaderDef is the decoded ItemReader field. JSON tags are
// lowerCamelCase; see itemBatcherDef in itembatcher.go for why.
type itemReaderDef struct {
	Resource     string           `json:"resource"`
	Parameters   map[string]any   `json:"parameters"`
	ReaderConfig itemReaderConfig `json:"readerConfig"`
}

// itemReaderConfig is ItemReader.ReaderConfig. ItemsPointer and
// CSVDelimiter are not implemented. MaxItemsPath is resolved against the
// Map state's input in readItemsFromS3, preferring it over the static
// MaxItems when both are set.
type itemReaderConfig struct {
	InputType         string   `json:"inputType"`
	CSVHeaderLocation string   `json:"csvHeaderLocation"`
	CSVHeaders        []string `json:"csvHeaders"`
	MaxItems          *int     `json:"maxItems"`
	MaxItemsPath      string   `json:"maxItemsPath"`

	// Transformation only applies to s3:listObjectsV2 -- see
	// itemReaderTransformationLoadAndFlatten.
	Transformation string `json:"transformation"`
}

// readItemsFromS3 resolves a Map state's ItemReader field into the item
// list it selects, fetching the underlying object(s) from kumo's own S3
// over plain HTTP (see (*executionEngine).getS3Object/listS3ObjectsV2).
func readItemsFromS3(ctx context.Context, e *executionEngine, raw json.RawMessage, mapInput string) ([]json.RawMessage, error) {
	var def itemReaderDef
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, fmt.Errorf("parse ItemReader: %w", err)
	}

	params, err := resolveParameters(def.Parameters, mapInput)
	if err != nil {
		return nil, fmt.Errorf("resolve ItemReader Parameters: %w", err)
	}

	var items []json.RawMessage

	switch def.Resource {
	case itemReaderResourceS3GetObject:
		items, err = readItemsFromS3GetObject(ctx, e, params, &def.ReaderConfig)
	case itemReaderResourceS3ListObjectsV2:
		items, err = readItemsFromS3ListObjectsV2(ctx, e, params, &def.ReaderConfig)
	default:
		return nil, fmt.Errorf("unsupported ItemReader resource %q", def.Resource)
	}

	if err != nil {
		return nil, fmt.Errorf("itemReader: %w", err)
	}

	maxItems, err := resolveOptionalIntPath(def.ReaderConfig.MaxItems, def.ReaderConfig.MaxItemsPath, mapInput)
	if err != nil {
		return nil, fmt.Errorf("itemReader: resolve ReaderConfig.MaxItemsPath: %w", err)
	}

	return applyMaxItems(items, maxItems), nil
}

// readItemsFromS3GetObject implements the s3:getObject ItemReader Resource:
// fetch one object and parse its body per ReaderConfig.InputType.
func readItemsFromS3GetObject(ctx context.Context, e *executionEngine, params map[string]any, cfg *itemReaderConfig) ([]json.RawMessage, error) {
	bucket, _ := params["Bucket"].(string)
	key, _ := params["Key"].(string)

	if bucket == "" || key == "" {
		return nil, fmt.Errorf("s3:getObject requires Parameters.Bucket and Parameters.Key")
	}

	body, err := e.getS3Object(ctx, bucket, key)
	if err != nil {
		return nil, err
	}

	return parseItemReaderBody(body, cfg)
}

// readItemsFromS3ListObjectsV2 implements the s3:listObjectsV2 ItemReader
// Resource: items are the object summaries AWS's own ListObjectsV2 API
// action returns (Etag, Key, LastModified, Size, StorageClass -- see
// marshalS3ObjectSummaries), not the objects' own contents.
func readItemsFromS3ListObjectsV2(ctx context.Context, e *executionEngine, params map[string]any, cfg *itemReaderConfig) ([]json.RawMessage, error) {
	if strings.EqualFold(cfg.Transformation, itemReaderTransformationLoadAndFlatten) {
		return nil, fmt.Errorf(
			"readerConfig.Transformation %q is not implemented for %q; only the default object-metadata listing is supported",
			itemReaderTransformationLoadAndFlatten, itemReaderResourceS3ListObjectsV2,
		)
	}

	bucket, _ := params["Bucket"].(string)
	if bucket == "" {
		return nil, fmt.Errorf("s3:listObjectsV2 requires Parameters.Bucket")
	}

	prefix, _ := params["Prefix"].(string)

	objects, err := e.listS3ObjectsV2(ctx, bucket, prefix)
	if err != nil {
		return nil, err
	}

	return marshalS3ObjectSummaries(objects)
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

// s3ListObjectsV2TimeFormat matches kumo's own S3 service's LastModified
// XML format (see timeFormatISO in internal/service/s3/handlers.go).
const s3ListObjectsV2TimeFormat = "2006-01-02T15:04:05.000Z"

// s3ListBucketResult mirrors just the fields of kumo's own S3
// ListObjectsV2 XML response (see ListBucketResult in
// internal/service/s3/types.go) that the ItemReader s3:listObjectsV2
// integration needs. It is redefined here rather than imported since kumo
// services only ever talk to each other over HTTP (see
// getS3Object/putS3Object), never via direct package imports.
type s3ListBucketResult struct {
	XMLName               xml.Name          `xml:"ListBucketResult"`
	IsTruncated           bool              `xml:"IsTruncated"`
	Contents              []s3ObjectSummary `xml:"Contents"`
	NextContinuationToken string            `xml:"NextContinuationToken"`
}

// s3ObjectSummary is one ListObjectsV2 <Contents> entry.
type s3ObjectSummary struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

// listS3ObjectsV2 lists every object under prefix in bucket, following
// kumo's own S3 ListObjectsV2 continuation-token pagination until
// IsTruncated is false.
func (e *executionEngine) listS3ObjectsV2(ctx context.Context, bucket, prefix string) ([]s3ObjectSummary, error) {
	var (
		all               []s3ObjectSummary
		continuationToken string
	)

	for {
		page, err := e.getS3ListObjectsV2Page(ctx, bucket, prefix, continuationToken)
		if err != nil {
			return nil, err
		}

		all = append(all, page.Contents...)

		if !page.IsTruncated || page.NextContinuationToken == "" {
			return all, nil
		}

		continuationToken = page.NextContinuationToken
	}
}

// getS3ListObjectsV2Page fetches a single ListObjectsV2 page from kumo's
// own S3 service (GET /{bucket}?list-type=2&prefix=...&continuation-token=...).
func (e *executionEngine) getS3ListObjectsV2Page(ctx context.Context, bucket, prefix, continuationToken string) (*s3ListBucketResult, error) {
	q := url.Values{"list-type": {"2"}}
	if prefix != "" {
		q.Set("prefix", prefix)
	}

	if continuationToken != "" {
		q.Set("continuation-token", continuationToken)
	}

	reqURL := fmt.Sprintf("%s/%s?%s", e.baseURL, bucket, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request for s3://%s (list-type=2): %w", bucket, err)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list s3://%s: %w", bucket, err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read list s3://%s response: %w", bucket, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list s3://%s: unexpected status %d: %s", bucket, resp.StatusCode, string(body))
	}

	var result s3ListBucketResult
	if err := xml.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse list s3://%s response: %w", bucket, err)
	}

	return &result, nil
}

// marshalS3ObjectSummaries converts S3 object metadata into the item shape
// AWS documents for s3:listObjectsV2 (without LOAD_AND_FLATTEN): Etag, Key,
// LastModified (Unix seconds), Size, StorageClass -- see
// https://docs.aws.amazon.com/step-functions/latest/dg/input-output-itemreader.html.
func marshalS3ObjectSummaries(objects []s3ObjectSummary) ([]json.RawMessage, error) {
	items := make([]json.RawMessage, len(objects))

	for i, obj := range objects {
		lastModified, err := time.Parse(s3ListObjectsV2TimeFormat, obj.LastModified)
		if err != nil {
			return nil, fmt.Errorf("parse LastModified %q for key %q: %w", obj.LastModified, obj.Key, err)
		}

		encoded, err := json.Marshal(map[string]any{
			"Etag":         obj.ETag,
			"Key":          obj.Key,
			"LastModified": lastModified.Unix(),
			"Size":         obj.Size,
			"StorageClass": obj.StorageClass,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal object summary for key %q: %w", obj.Key, err)
		}

		items[i] = encoded
	}

	return items, nil
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
