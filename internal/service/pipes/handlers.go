package pipes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
)

// CreatePipe handles the CreatePipe API operation.
func (s *Service) CreatePipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get pipe name from URL path.
	name := extractPipeName(r.URL.Path)
	if name == "" {
		writeError(w, errValidationException, "Pipe name is required", http.StatusBadRequest)

		return
	}

	var req CreatePipeInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errValidationException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.Name = name

	pipe, err := s.storage.CreatePipe(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &CreatePipeOutput{
		Arn:              pipe.Arn,
		Name:             pipe.Name,
		DesiredState:     pipe.DesiredState,
		CurrentState:     pipe.CurrentState,
		CreationTime:     pipe.CreationTime,
		LastModifiedTime: pipe.LastModifiedTime,
	}

	writeJSON(w, output)
}

// DescribePipe handles the DescribePipe API operation.
func (s *Service) DescribePipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get pipe name from URL path.
	name := extractPipeName(r.URL.Path)
	if name == "" {
		writeError(w, errValidationException, "Pipe name is required", http.StatusBadRequest)

		return
	}

	pipe, err := s.storage.DescribePipe(ctx, name)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &DescribePipeOutput{
		Arn:                  pipe.Arn,
		Name:                 pipe.Name,
		Source:               pipe.Source,
		Target:               pipe.Target,
		RoleArn:              pipe.RoleArn,
		Description:          pipe.Description,
		DesiredState:         pipe.DesiredState,
		CurrentState:         pipe.CurrentState,
		StateReason:          pipe.StateReason,
		Enrichment:           pipe.Enrichment,
		EnrichmentParameters: pipe.EnrichmentParameters,
		SourceParameters:     pipe.SourceParameters,
		TargetParameters:     pipe.TargetParameters,
		LogConfiguration:     pipe.LogConfiguration,
		KmsKeyIdentifier:     pipe.KmsKeyIdentifier,
		Tags:                 pipe.Tags,
		CreationTime:         pipe.CreationTime,
		LastModifiedTime:     pipe.LastModifiedTime,
	}

	writeJSON(w, output)
}

// UpdatePipe handles the UpdatePipe API operation.
func (s *Service) UpdatePipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get pipe name from URL path.
	name := extractPipeName(r.URL.Path)
	if name == "" {
		writeError(w, errValidationException, "Pipe name is required", http.StatusBadRequest)

		return
	}

	var req UpdatePipeInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errValidationException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.Name = name

	pipe, err := s.storage.UpdatePipe(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &UpdatePipeOutput{
		Arn:              pipe.Arn,
		Name:             pipe.Name,
		DesiredState:     pipe.DesiredState,
		CurrentState:     pipe.CurrentState,
		CreationTime:     pipe.CreationTime,
		LastModifiedTime: pipe.LastModifiedTime,
	}

	writeJSON(w, output)
}

// DeletePipe handles the DeletePipe API operation.
func (s *Service) DeletePipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get pipe name from URL path.
	name := extractPipeName(r.URL.Path)
	if name == "" {
		writeError(w, errValidationException, "Pipe name is required", http.StatusBadRequest)

		return
	}

	pipe, err := s.storage.DeletePipe(ctx, name)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &DeletePipeOutput{
		Arn:              pipe.Arn,
		Name:             pipe.Name,
		DesiredState:     pipe.DesiredState,
		CurrentState:     pipe.CurrentState,
		CreationTime:     pipe.CreationTime,
		LastModifiedTime: pipe.LastModifiedTime,
	}

	writeJSON(w, output)
}

// ListPipes handles the ListPipes API operation.
func (s *Service) ListPipes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters.
	query := r.URL.Query()
	req := &ListPipesInput{
		CurrentState: query.Get("CurrentState"),
		DesiredState: query.Get("DesiredState"),
		NamePrefix:   query.Get("NamePrefix"),
		NextToken:    query.Get("NextToken"),
		SourcePrefix: query.Get("SourcePrefix"),
		TargetPrefix: query.Get("TargetPrefix"),
	}

	// Parse limit if provided.
	if limitStr := query.Get("Limit"); limitStr != "" {
		var limit int32

		if _, err := parseIntParam(limitStr, &limit); err != nil {
			writeError(w, errValidationException, "Invalid Limit parameter", http.StatusBadRequest)

			return
		}

		req.Limit = limit
	}

	output, err := s.storage.ListPipes(ctx, req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, output)
}

// StartPipe handles the StartPipe API operation.
func (s *Service) StartPipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get pipe name from URL path: /v1/pipes/{Name}/start
	name := extractPipeNameFromAction(r.URL.Path, "start")
	if name == "" {
		writeError(w, errValidationException, "Pipe name is required", http.StatusBadRequest)

		return
	}

	pipe, err := s.storage.StartPipe(ctx, name)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &StartPipeOutput{
		Arn:              pipe.Arn,
		Name:             pipe.Name,
		DesiredState:     pipe.DesiredState,
		CurrentState:     pipe.CurrentState,
		CreationTime:     pipe.CreationTime,
		LastModifiedTime: pipe.LastModifiedTime,
	}

	writeJSON(w, output)
}

// StopPipe handles the StopPipe API operation.
func (s *Service) StopPipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get pipe name from URL path: /v1/pipes/{Name}/stop
	name := extractPipeNameFromAction(r.URL.Path, "stop")
	if name == "" {
		writeError(w, errValidationException, "Pipe name is required", http.StatusBadRequest)

		return
	}

	pipe, err := s.storage.StopPipe(ctx, name)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &StopPipeOutput{
		Arn:              pipe.Arn,
		Name:             pipe.Name,
		DesiredState:     pipe.DesiredState,
		CurrentState:     pipe.CurrentState,
		CreationTime:     pipe.CreationTime,
		LastModifiedTime: pipe.LastModifiedTime,
	}

	writeJSON(w, output)
}

// TagResource handles the TagResource API operation.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get resource ARN from URL path.
	arn := extractResourceArn(r.URL.Path)
	if arn == "" {
		writeError(w, errValidationException, "Resource ARN is required", http.StatusBadRequest)

		return
	}

	// URL decode the ARN.
	decodedArn, err := url.PathUnescape(arn)
	if err != nil {
		writeError(w, errValidationException, "Invalid resource ARN", http.StatusBadRequest)

		return
	}

	var req TagResourceInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errValidationException, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(ctx, decodedArn, req.Tags); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// UntagResource handles the UntagResource API operation.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get resource ARN from URL path.
	arn := extractResourceArn(r.URL.Path)
	if arn == "" {
		writeError(w, errValidationException, "Resource ARN is required", http.StatusBadRequest)

		return
	}

	// URL decode the ARN.
	decodedArn, err := url.PathUnescape(arn)
	if err != nil {
		writeError(w, errValidationException, "Invalid resource ARN", http.StatusBadRequest)

		return
	}

	// Get tag keys from query parameters.
	tagKeys := r.URL.Query()["tagKeys"]

	if err := s.storage.UntagResource(ctx, decodedArn, tagKeys); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// ListTagsForResource handles the ListTagsForResource API operation.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get resource ARN from URL path.
	arn := extractResourceArn(r.URL.Path)
	if arn == "" {
		writeError(w, errValidationException, "Resource ARN is required", http.StatusBadRequest)

		return
	}

	// URL decode the ARN.
	decodedArn, err := url.PathUnescape(arn)
	if err != nil {
		writeError(w, errValidationException, "Invalid resource ARN", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListTagsForResource(ctx, decodedArn)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &ListTagsForResourceOutput{
		Tags: tags,
	}

	writeJSON(w, output)
}

// Helper functions.

// extractPipeName extracts the pipe name from a URL path like /v1/pipes/{Name}.
func extractPipeName(path string) string {
	// Remove prefix and extract name.
	const prefix = "/v1/pipes/"

	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	name := strings.TrimPrefix(path, prefix)
	// Remove any trailing slashes or additional path segments.
	if idx := strings.Index(name, "/"); idx != -1 {
		name = name[:idx]
	}

	return name
}

// extractPipeNameFromAction extracts the pipe name from a URL path like /v1/pipes/{Name}/{action}.
func extractPipeNameFromAction(path, action string) string {
	// Remove prefix and extract name.
	const prefix = "/v1/pipes/"

	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	remainder := strings.TrimPrefix(path, prefix)
	// Expected format: {Name}/{action}
	suffix := "/" + action

	if !strings.HasSuffix(remainder, suffix) {
		return ""
	}

	return strings.TrimSuffix(remainder, suffix)
}

// extractResourceArn extracts the resource ARN from a URL path like /tags/{arn}.
func extractResourceArn(path string) string {
	// Remove prefix and extract ARN.
	const prefix = "/tags/"

	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	return strings.TrimPrefix(path, prefix)
}

// parseIntParam parses an integer parameter from a string.
func parseIntParam(s string, result *int32) (int32, error) {
    var val int

    for _, c := range s {
        if c < '0' || c > '9' {
            return 0, errors.New("invalid integer")
        }
        val = val*10 + int(c-'0')
    }

    // Check for overflow before casting
    if val > math.MaxInt32 || val < math.MinInt32 { 
        return 0, fmt.Errorf("value %d out of int32 range", val)
    }

    *result = int32(val)
    return *result, nil
}


// writeJSON writes a JSON response with 200 OK status.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, _, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := &ErrorResponse{
		Message: message,
	}

	json.NewEncoder(w).Encode(resp) //nolint:errcheck,gosec // best effort error handling
}

// handleError handles storage errors and writes an appropriate HTTP response.
func handleError(w http.ResponseWriter, err error) {
	var pipeErr *Error
	if errors.As(err, &pipeErr) {
		writeError(w, pipeErr.Code, pipeErr.Message, pipeErr.HTTPStatusCode())

		return
	}

	writeError(w, errInternalException, "Internal server error", http.StatusInternalServerError)
}
