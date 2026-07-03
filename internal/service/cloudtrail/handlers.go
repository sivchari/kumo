package cloudtrail

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// handlerFunc is a type alias for handler functions.
type handlerFunc func(http.ResponseWriter, *http.Request)

// getActionHandlers returns a map of action names to handler functions.
func (s *Service) getActionHandlers() map[string]handlerFunc {
	return map[string]handlerFunc{
		"CreateTrail":    s.CreateTrail,
		"DeleteTrail":    s.DeleteTrail,
		"GetTrail":       s.GetTrail,
		"DescribeTrails": s.DescribeTrails,
		"StartLogging":   s.StartLogging,
		"StopLogging":    s.StopLogging,
		"LookupEvents":   s.LookupEvents,
		"GetTrailStatus": s.GetTrailStatus,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "CloudTrail_20131101.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "UnknownOperationException", "The operation "+action+" is not valid.", http.StatusBadRequest)
}

// CreateTrail handles the CreateTrail API.
func (s *Service) CreateTrail(w http.ResponseWriter, r *http.Request) {
	var req CreateTrailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	trail, err := s.storage.CreateTrail(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateTrailResponse{
		Name:                       trail.Name,
		TrailARN:                   trail.TrailARN,
		S3BucketName:               trail.S3BucketName,
		S3KeyPrefix:                trail.S3KeyPrefix,
		IncludeGlobalServiceEvents: boolPtr(trail.IncludeGlobalServiceEvents),
		IsMultiRegionTrail:         boolPtr(trail.IsMultiRegionTrail),
		LogFileValidationEnabled:   boolPtr(trail.LogFileValidationEnabled),
		CloudWatchLogsLogGroupArn:  trail.CloudWatchLogsLogGroupArn,
		CloudWatchLogsRoleArn:      trail.CloudWatchLogsRoleArn,
		KMSKeyID:                   trail.KMSKeyID,
		IsOrganizationTrail:        boolPtr(trail.IsOrganizationTrail),
	}

	writeResponse(w, resp)
}

// DeleteTrail handles the DeleteTrail API.
func (s *Service) DeleteTrail(w http.ResponseWriter, r *http.Request) {
	var req DeleteTrailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, "ValidationException", "Trail name is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteTrail(r.Context(), req.Name); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteTrailResponse{})
}

// GetTrail handles the GetTrail API.
func (s *Service) GetTrail(w http.ResponseWriter, r *http.Request) {
	var req GetTrailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, "ValidationException", "Trail name is required", http.StatusBadRequest)

		return
	}

	trail, err := s.storage.GetTrail(r.Context(), req.Name)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &GetTrailResponse{
		Trail: convertToTrailOutput(trail),
	}

	writeResponse(w, resp)
}

// DescribeTrails handles the DescribeTrails API.
func (s *Service) DescribeTrails(w http.ResponseWriter, r *http.Request) {
	var req DescribeTrailsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	trails, err := s.storage.DescribeTrails(r.Context(), req.TrailNameList)
	if err != nil {
		handleError(w, err)

		return
	}

	trailList := make([]TrailOutput, 0, len(trails))
	for _, trail := range trails {
		trailList = append(trailList, *convertToTrailOutput(trail))
	}

	resp := &DescribeTrailsResponse{
		TrailList: trailList,
	}

	writeResponse(w, resp)
}

// StartLogging handles the StartLogging API.
func (s *Service) StartLogging(w http.ResponseWriter, r *http.Request) {
	var req StartLoggingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, "ValidationException", "Trail name is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.StartLogging(r.Context(), req.Name); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &StartLoggingResponse{})
}

// StopLogging handles the StopLogging API.
func (s *Service) StopLogging(w http.ResponseWriter, r *http.Request) {
	var req StopLoggingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, "ValidationException", "Trail name is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.StopLogging(r.Context(), req.Name); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &StopLoggingResponse{})
}

// LookupEvents handles the LookupEvents API.
func (s *Service) LookupEvents(w http.ResponseWriter, r *http.Request) {
	var req LookupEventsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	events, nextToken, err := s.storage.LookupEvents(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	eventOutputs := make([]EventOutput, 0, len(events))
	for _, event := range events {
		eventOutputs = append(eventOutputs, EventOutput{
			EventID:         event.EventID,
			EventName:       event.EventName,
			EventSource:     event.EventSource,
			EventTime:       float64(event.EventTime.Unix()),
			Username:        event.Username,
			CloudTrailEvent: event.CloudTrailEvent,
		})
	}

	resp := &LookupEventsResponse{
		Events:    eventOutputs,
		NextToken: nextToken,
	}

	writeResponse(w, resp)
}

// GetTrailStatus handles the GetTrailStatus API.
func (s *Service) GetTrailStatus(w http.ResponseWriter, r *http.Request) {
	var req GetTrailStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, "ValidationException", "Trail name is required", http.StatusBadRequest)

		return
	}

	trail, err := s.storage.GetTrailStatus(r.Context(), req.Name)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &GetTrailStatusResponse{
		IsLogging: boolPtr(trail.IsLogging),
	}

	writeResponse(w, resp)
}

// Helper functions.

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool {
	return &b
}

// convertToTrailOutput converts a Trail to TrailOutput.
func convertToTrailOutput(trail *Trail) *TrailOutput {
	return &TrailOutput{
		Name:                       trail.Name,
		TrailARN:                   trail.TrailARN,
		S3BucketName:               trail.S3BucketName,
		S3KeyPrefix:                trail.S3KeyPrefix,
		IncludeGlobalServiceEvents: trail.IncludeGlobalServiceEvents,
		IsMultiRegionTrail:         trail.IsMultiRegionTrail,
		HomeRegion:                 trail.HomeRegion,
		LogFileValidationEnabled:   trail.LogFileValidationEnabled,
		CloudWatchLogsLogGroupArn:  trail.CloudWatchLogsLogGroupArn,
		CloudWatchLogsRoleArn:      trail.CloudWatchLogsRoleArn,
		KMSKeyID:                   trail.KMSKeyID,
		HasCustomEventSelectors:    trail.HasCustomEventSelectors,
		HasInsightSelectors:        trail.HasInsightSelectors,
		IsOrganizationTrail:        trail.IsOrganizationTrail,
	}
}

// writeResponse writes a JSON response.
func writeResponse(w http.ResponseWriter, resp any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeAmzJSON11, code, message, status)
}

// handleError handles service errors.
func handleError(w http.ResponseWriter, err error) {
	var ctErr *Error
	if errors.As(err, &ctErr) {
		status := getErrorStatus(ctErr.Code)
		writeError(w, ctErr.Code, ctErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errTrailNotFound:
		return http.StatusNotFound
	case errTrailAlreadyExists:
		return http.StatusConflict
	case errValidationError:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}
