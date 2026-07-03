package cloudfront

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// maxFunctionCodeBytes caps the decoded JS source size. Real CloudFront
// limits to 10 KiB; kumo allows 256 KiB so test fixtures with bundled
// dependencies still fit, but rejects anything that looks like a parser
// memory-exhaustion attempt. maxRequestBodyBytes does the same for the
// raw HTTP body before XML decode, defending against multi-GB streams.
const (
	maxFunctionCodeBytes = 256 * 1024
	maxRequestBodyBytes  = 1 * 1024 * 1024
)

// functionNamePattern matches the AWS-published format for function
// names: 1-64 alphanumeric or hyphen / underscore characters. Anything
// outside this set could escape into ARNs, Location headers, or storage
// map keys and should be rejected at the boundary.
var functionNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// CreateFunction handles POST /2020-05-31/function. Body is XML wrapping
// Name + FunctionConfig + base64(FunctionCode).
func (s *Service) CreateFunction(w http.ResponseWriter, r *http.Request) {
	body, err := readRequestBody(r)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, err.Error(), http.StatusBadRequest)

		return
	}

	var req xmlCreateFunctionRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := validateFunctionName(req.Name); err != nil {
		writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)

		return
	}

	code, err := decodeFunctionCode(req.FunctionCode)
	if err != nil {
		writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)

		return
	}

	fn, err := s.storage.CreateFunction(r.Context(), req.Name, req.FunctionConfig.Runtime, req.FunctionConfig.Comment, code)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	w.Header().Set("ETag", fn.ETag)
	w.Header().Set("Location", "/2020-05-31/function/"+fn.Name)
	writeXMLResponse(w, http.StatusCreated, functionSummaryResponse(fn, FunctionStageDevelopment))
}

// GetFunction handles GET /2020-05-31/function/{Name}. The function code
// is streamed back as the body (Content-Type: application/octet-stream)
// and the metadata rides on response headers.
func (s *Service) GetFunction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("Name")
	stage := r.URL.Query().Get("Stage")

	if err := validateFunctionName(name); err != nil {
		writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)

		return
	}

	fn, code, err := s.storage.GetFunction(r.Context(), name, stage)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	w.Header().Set("ETag", fn.ETag)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(code)
}

// DescribeFunction handles GET /2020-05-31/function/{Name}/describe. It
// returns just the metadata, not the code.
func (s *Service) DescribeFunction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("Name")
	stage := r.URL.Query().Get("Stage")

	if err := validateFunctionName(name); err != nil {
		writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)

		return
	}

	fn, err := s.storage.DescribeFunction(r.Context(), name, stage)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	w.Header().Set("ETag", fn.ETag)
	writeXMLResponse(w, http.StatusOK, functionSummaryResponse(fn, effectiveStage(stage)))
}

// ListFunctions handles GET /2020-05-31/function.
func (s *Service) ListFunctions(w http.ResponseWriter, r *http.Request) {
	stage := r.URL.Query().Get("Stage")

	fns, err := s.storage.ListFunctions(r.Context(), stage)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	items := make([]xmlFunctionSummary, 0, len(fns))
	for _, fn := range fns {
		items = append(items, functionSummaryXML(fn, effectiveStage(stage)))
	}

	writeXMLResponse(w, http.StatusOK, xmlListFunctionsResult{
		Xmlns:    cloudfrontXmlns,
		MaxItems: 100,
		Quantity: len(items),
		Items:    items,
	})
}

// UpdateFunction handles PUT /2020-05-31/function/{Name}. Requires
// If-Match header.
func (s *Service) UpdateFunction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("Name")

	if err := validateFunctionName(name); err != nil {
		writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)

		return
	}

	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		writeCloudFrontError(w, errPreconditionFailed, "The If-Match header is required", http.StatusPreconditionFailed)

		return
	}

	body, err := readRequestBody(r)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, err.Error(), http.StatusBadRequest)

		return
	}

	var req xmlUpdateFunctionRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	code, err := decodeFunctionCode(req.FunctionCode)
	if err != nil {
		writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)

		return
	}

	fn, err := s.storage.UpdateFunction(r.Context(), name, req.FunctionConfig.Runtime, req.FunctionConfig.Comment, code, ifMatch)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	w.Header().Set("ETag", fn.ETag)
	writeXMLResponse(w, http.StatusOK, functionSummaryResponse(fn, FunctionStageDevelopment))
}

// PublishFunction handles POST /2020-05-31/function/{Name}/publish.
// Requires If-Match.
func (s *Service) PublishFunction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("Name")

	if err := validateFunctionName(name); err != nil {
		writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)

		return
	}

	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		writeCloudFrontError(w, errPreconditionFailed, "The If-Match header is required", http.StatusPreconditionFailed)

		return
	}

	fn, err := s.storage.PublishFunction(r.Context(), name, ifMatch)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	w.Header().Set("ETag", fn.ETag)
	writeXMLResponse(w, http.StatusOK, functionSummaryResponse(fn, FunctionStageLive))
}

// DeleteFunction handles DELETE /2020-05-31/function/{Name}. Requires
// If-Match.
func (s *Service) DeleteFunction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("Name")

	if err := validateFunctionName(name); err != nil {
		writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)

		return
	}

	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		writeCloudFrontError(w, errPreconditionFailed, "The If-Match header is required", http.StatusPreconditionFailed)

		return
	}

	if err := s.storage.DeleteFunction(r.Context(), name, ifMatch); err != nil {
		handleFunctionError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestFunction handles POST /2020-05-31/function/{Name}/test. Runs the
// function code (per-stage) against a base64-encoded JSON event and
// returns the captured output.
func (s *Service) TestFunction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("Name")

	if err := validateFunctionName(name); err != nil {
		writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)

		return
	}

	body, err := readRequestBody(r)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, err.Error(), http.StatusBadRequest)

		return
	}

	var req xmlTestFunctionRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	stage := effectiveStage(req.Stage)

	fn, code, err := s.storage.GetFunction(r.Context(), name, stage)
	if err != nil {
		handleFunctionError(w, err)

		return
	}

	eventJSON, err := decodeEventObject(req.EventObject)
	if err != nil {
		writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)

		return
	}

	result := runFunction(code, eventJSON)

	writeXMLResponse(w, http.StatusOK, xmlTestResult{
		Xmlns:                 cloudfrontXmlns,
		FunctionSummary:       functionSummaryXML(fn, stage),
		ComputeUtilization:    "0",
		FunctionExecutionLogs: xmlFunctionLogList{Items: result.LogLines},
		FunctionErrorMessage:  result.Error,
		FunctionOutput:        result.Output,
	})
}

func functionSummaryResponse(fn *Function, stage string) xmlFunctionSummary {
	summary := functionSummaryXML(fn, stage)
	summary.Xmlns = cloudfrontXmlns

	return summary
}

// functionSummaryXML translates a Function into the wire shape AWS
// returns from every Function endpoint. Status follows the simple rule
// AWS uses on first edit: UNPUBLISHED until PublishFunction; UNASSOCIATED
// after publish (kumo doesn't model distribution associations yet).
func functionSummaryXML(fn *Function, stage string) xmlFunctionSummary {
	status := "UNPUBLISHED"
	if _, hasLive := fn.Code[FunctionStageLive]; hasLive {
		status = "UNASSOCIATED"
	}

	return xmlFunctionSummary{
		Name:   fn.Name,
		Status: status,
		FunctionConfig: xmlFunctionConfig{
			Comment: fn.Comment,
			Runtime: fn.Runtime,
		},
		FunctionMetadata: xmlFunctionMetadata{
			FunctionARN:      "arn:aws:cloudfront::123456789012:function/" + fn.Name,
			Stage:            stage,
			CreatedTime:      fn.CreatedTime.UTC().Format("2006-01-02T15:04:05.000Z"),
			LastModifiedTime: fn.LastModifiedTime.UTC().Format("2006-01-02T15:04:05.000Z"),
		},
	}
}

// decodeFunctionCode handles the base64 wrapping AWS uses for
// FunctionCode. Empty payloads are rejected. The decoded output is
// rejected when it exceeds maxFunctionCodeBytes — without that cap a
// caller can submit a multi-megabyte JS source and stress the quickjs
// (wazero) parser when TestFunction runs against it.
func decodeFunctionCode(encoded string) ([]byte, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, errors.New("FunctionCode is required")
	}

	out, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("FunctionCode is not valid base64: %w", err)
	}

	if len(out) > maxFunctionCodeBytes {
		return nil, fmt.Errorf("FunctionCode exceeds %d bytes", maxFunctionCodeBytes)
	}

	return out, nil
}

// validateFunctionName enforces the AWS-published shape (1-64
// alphanumeric / hyphen / underscore). Anything outside that set could
// land in headers, ARNs, or storage map keys with surprising effects.
func validateFunctionName(name string) error {
	if name == "" {
		return errors.New("Name is required")
	}

	if !functionNamePattern.MatchString(name) {
		return errors.New("Name must match [a-zA-Z0-9_-]{1,64}")
	}

	return nil
}

// readRequestBody reads at most maxRequestBodyBytes from r.Body. Without
// this cap, an attacker streaming a multi-GB request body would have
// the entire payload buffered in memory before XML decode rejects it.
func readRequestBody(r *http.Request) ([]byte, error) {
	limited := io.LimitReader(r.Body, maxRequestBodyBytes+1)

	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}

	if len(body) > maxRequestBodyBytes {
		return nil, fmt.Errorf("request body exceeds %d bytes", maxRequestBodyBytes)
	}

	return body, nil
}

// decodeEventObject normalises TestFunction's EventObject field. It is
// base64-encoded JSON; an empty value is treated as a null event.
func decodeEventObject(encoded string) (string, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return "", nil
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("EventObject is not valid base64: %w", err)
	}

	return string(raw), nil
}

// handleFunctionError translates a storage Error into the appropriate
// HTTP status / error code pair.
func handleFunctionError(w http.ResponseWriter, err error) {
	var sErr *Error
	if errors.As(err, &sErr) {
		switch sErr.Code {
		case "NoSuchFunctionExists":
			writeCloudFrontError(w, sErr.Code, sErr.Message, http.StatusNotFound)

			return
		case "FunctionAlreadyExists":
			writeCloudFrontError(w, sErr.Code, sErr.Message, http.StatusConflict)

			return
		case "InvalidIfMatchVersion":
			writeCloudFrontError(w, errInvalidIfMatchVersion, sErr.Message, http.StatusBadRequest)

			return
		case "PreconditionFailed":
			writeCloudFrontError(w, errPreconditionFailed, sErr.Message, http.StatusPreconditionFailed)

			return
		}

		writeCloudFrontError(w, sErr.Code, sErr.Message, http.StatusBadRequest)

		return
	}

	writeCloudFrontError(w, errInvalidArgument, err.Error(), http.StatusBadRequest)
}
