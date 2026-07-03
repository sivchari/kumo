// Package emrserverless provides the EMR Serverless service implementation.
package emrserverless

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/sivchari/kumo/internal/service"
)

// CreateApplication handles the CreateApplication API operation.
func (s *Service) CreateApplication(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateApplicationInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errValidationException, "Invalid request body", http.StatusBadRequest)

		return
	}

	app, err := s.storage.CreateApplication(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &CreateApplicationOutput{
		ApplicationID: app.ApplicationID,
		Arn:           app.Arn,
		Name:          app.Name,
	}

	writeJSON(w, output)
}

// GetApplication handles the GetApplication API operation.
func (s *Service) GetApplication(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	applicationID := extractApplicationID(r.URL.Path)
	if applicationID == "" {
		writeError(w, errValidationException, "Application ID is required", http.StatusBadRequest)

		return
	}

	app, err := s.storage.GetApplication(ctx, applicationID)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &GetApplicationOutput{
		Application: app,
	}

	writeJSON(w, output)
}

// ListApplications handles the ListApplications API operation.
func (s *Service) ListApplications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	req := &ListApplicationsInput{
		NextToken: query.Get("nextToken"),
		States:    query["states"],
	}

	if maxResultsStr := query.Get("maxResults"); maxResultsStr != "" {
		var maxResults int32

		if err := parseIntParam(maxResultsStr, &maxResults); err != nil {
			writeError(w, errValidationException, "Invalid maxResults parameter", http.StatusBadRequest)

			return
		}

		req.MaxResults = maxResults
	}

	output, err := s.storage.ListApplications(ctx, req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, output)
}

// UpdateApplication handles the UpdateApplication API operation.
func (s *Service) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	applicationID := extractApplicationID(r.URL.Path)
	if applicationID == "" {
		writeError(w, errValidationException, "Application ID is required", http.StatusBadRequest)

		return
	}

	var req UpdateApplicationInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errValidationException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.ApplicationID = applicationID

	app, err := s.storage.UpdateApplication(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &UpdateApplicationOutput{
		Application: app,
	}

	writeJSON(w, output)
}

// DeleteApplication handles the DeleteApplication API operation.
func (s *Service) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	applicationID := extractApplicationID(r.URL.Path)
	if applicationID == "" {
		writeError(w, errValidationException, "Application ID is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteApplication(ctx, applicationID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// StartApplication handles the StartApplication API operation.
func (s *Service) StartApplication(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	applicationID := extractApplicationIDFromAction(r.URL.Path, "start")
	if applicationID == "" {
		writeError(w, errValidationException, "Application ID is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.StartApplication(ctx, applicationID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// StopApplication handles the StopApplication API operation.
func (s *Service) StopApplication(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	applicationID := extractApplicationIDFromAction(r.URL.Path, "stop")
	if applicationID == "" {
		writeError(w, errValidationException, "Application ID is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.StopApplication(ctx, applicationID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// StartJobRun handles the StartJobRun API operation.
func (s *Service) StartJobRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	applicationID := extractApplicationIDFromJobRuns(r.URL.Path)
	if applicationID == "" {
		writeError(w, errValidationException, "Application ID is required", http.StatusBadRequest)

		return
	}

	var req StartJobRunInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, errValidationException, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.ApplicationID = applicationID

	jobRun, err := s.storage.StartJobRun(ctx, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &StartJobRunOutput{
		ApplicationID: jobRun.ApplicationID,
		JobRunID:      jobRun.JobRunID,
		Arn:           jobRun.Arn,
	}

	writeJSON(w, output)
}

// GetJobRun handles the GetJobRun API operation.
func (s *Service) GetJobRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	applicationID, jobRunID := extractApplicationAndJobRunID(r.URL.Path)
	if applicationID == "" || jobRunID == "" {
		writeError(w, errValidationException, "Application ID and JobRun ID are required", http.StatusBadRequest)

		return
	}

	jobRun, err := s.storage.GetJobRun(ctx, applicationID, jobRunID)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &GetJobRunOutput{
		JobRun: jobRun,
	}

	writeJSON(w, output)
}

// ListJobRuns handles the ListJobRuns API operation.
func (s *Service) ListJobRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	applicationID := extractApplicationIDFromJobRuns(r.URL.Path)
	if applicationID == "" {
		writeError(w, errValidationException, "Application ID is required", http.StatusBadRequest)

		return
	}

	query := r.URL.Query()
	req := &ListJobRunsInput{
		ApplicationID:   applicationID,
		NextToken:       query.Get("nextToken"),
		States:          query["states"],
		Mode:            query.Get("mode"),
		CreatedAtBefore: query.Get("createdAtBefore"),
		CreatedAtAfter:  query.Get("createdAtAfter"),
	}

	if maxResultsStr := query.Get("maxResults"); maxResultsStr != "" {
		var maxResults int32

		if err := parseIntParam(maxResultsStr, &maxResults); err != nil {
			writeError(w, errValidationException, "Invalid maxResults parameter", http.StatusBadRequest)

			return
		}

		req.MaxResults = maxResults
	}

	output, err := s.storage.ListJobRuns(ctx, req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, output)
}

// CancelJobRun handles the CancelJobRun API operation.
func (s *Service) CancelJobRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	applicationID, jobRunID := extractApplicationAndJobRunID(r.URL.Path)
	if applicationID == "" || jobRunID == "" {
		writeError(w, errValidationException, "Application ID and JobRun ID are required", http.StatusBadRequest)

		return
	}

	jobRun, err := s.storage.CancelJobRun(ctx, applicationID, jobRunID)
	if err != nil {
		handleError(w, err)

		return
	}

	output := &CancelJobRunOutput{
		ApplicationID: jobRun.ApplicationID,
		JobRunID:      jobRun.JobRunID,
	}

	writeJSON(w, output)
}

// Helper functions.

// extractApplicationID extracts the application ID from a URL path like /applications/{applicationId}.
func extractApplicationID(path string) string {
	const prefix = "/applications/"

	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	remainder := strings.TrimPrefix(path, prefix)

	// Remove any trailing path segments.
	if idx := strings.Index(remainder, "/"); idx != -1 {
		remainder = remainder[:idx]
	}

	return remainder
}

// extractApplicationIDFromAction extracts the application ID from a URL path like /applications/{applicationId}/{action}.
func extractApplicationIDFromAction(path, action string) string {
	const prefix = "/applications/"

	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	remainder := strings.TrimPrefix(path, prefix)
	suffix := "/" + action

	if !strings.HasSuffix(remainder, suffix) {
		return ""
	}

	return strings.TrimSuffix(remainder, suffix)
}

// extractApplicationIDFromJobRuns extracts the application ID from a URL path like /applications/{applicationId}/jobruns.
func extractApplicationIDFromJobRuns(path string) string {
	const prefix = "/applications/"

	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	remainder := strings.TrimPrefix(path, prefix)

	// Find the /jobruns part.
	idx := strings.Index(remainder, "/jobruns")
	if idx == -1 {
		return ""
	}

	return remainder[:idx]
}

// extractApplicationAndJobRunID extracts both IDs from a URL path like /applications/{applicationId}/jobruns/{jobRunId}.
func extractApplicationAndJobRunID(path string) (string, string) {
	const prefix = "/applications/"

	if !strings.HasPrefix(path, prefix) {
		return "", ""
	}

	remainder := strings.TrimPrefix(path, prefix)

	// Split by /jobruns/.
	parts := strings.Split(remainder, "/jobruns/")
	if len(parts) != 2 {
		return "", ""
	}

	applicationID := parts[0]
	jobRunID := parts[1]

	// Remove any trailing path segments from jobRunID.
	if idx := strings.Index(jobRunID, "/"); idx != -1 {
		jobRunID = jobRunID[:idx]
	}

	return applicationID, jobRunID
}

// parseIntParam parses an integer parameter from a string.
func parseIntParam(s string, result *int32) error {
	var val int

	for _, c := range s {
		if c < '0' || c > '9' {
			return errors.New("invalid integer")
		}

		val = val*10 + int(c-'0')
	}

	*result = int32(val) //nolint:gosec // G115: val is bounded by input string length

	return nil
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
func writeError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeJSON, code, message, status)
}

// handleError handles storage errors and writes an appropriate HTTP response.
func handleError(w http.ResponseWriter, err error) {
	var emrErr *Error
	if errors.As(err, &emrErr) {
		writeError(w, emrErr.Code, emrErr.Message, emrErr.HTTPStatusCode())

		return
	}

	writeError(w, errInternalServerError, "Internal server error", http.StatusInternalServerError)
}
