package xray

import (
	"errors"
	"net/http"

	"github.com/sivchari/kumo/internal/service"
)

// PutTraceSegments handles the PutTraceSegments operation.
func (s *Service) PutTraceSegments(w http.ResponseWriter, r *http.Request) {
	var req PutTraceSegmentsInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if len(req.TraceSegmentDocuments) == 0 {
		writeError(w, errInvalidRequest, "TraceSegmentDocuments is required", http.StatusBadRequest)

		return
	}

	unprocessed, err := s.storage.PutTraceSegments(r.Context(), req.TraceSegmentDocuments)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, PutTraceSegmentsOutput{
		UnprocessedTraceSegments: unprocessed,
	})
}

// GetTraceSummaries handles the GetTraceSummaries operation.
func (s *Service) GetTraceSummaries(w http.ResponseWriter, r *http.Request) {
	var req GetTraceSummariesInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.StartTime == nil || req.EndTime == nil {
		writeError(w, errInvalidRequest, "StartTime and EndTime are required", http.StatusBadRequest)

		return
	}

	summaries, err := s.storage.GetTraceSummaries(r.Context(), req.StartTime.ToTime(), req.EndTime.ToTime())
	if err != nil {
		handleStorageError(w, err)

		return
	}

	responses := make([]TraceSummaryResponse, 0, len(summaries))

	for i := range summaries {
		responses = append(responses, TraceSummaryResponse{
			ID:                summaries[i].ID,
			Duration:          summaries[i].Duration,
			ResponseTime:      summaries[i].ResponseTime,
			HasFault:          summaries[i].HasFault,
			HasError:          summaries[i].HasError,
			HasThrottle:       summaries[i].HasThrottle,
			IsPartial:         summaries[i].IsPartial,
			HTTP:              summaries[i].HTTP,
			Annotations:       summaries[i].Annotations,
			Users:             summaries[i].Users,
			ServiceIDs:        summaries[i].ServiceIDs,
			EntryPoint:        summaries[i].EntryPoint,
			FaultRootCauses:   summaries[i].FaultRootCauses,
			ErrorRootCauses:   summaries[i].ErrorRootCauses,
			AvailabilityZones: summaries[i].AvailabilityZones,
			InstanceIDs:       summaries[i].InstanceIDs,
			ResourceARNs:      summaries[i].ResourceARNs,
			MatchedEventTime:  summaries[i].MatchedEventTime,
			Revision:          summaries[i].Revision,
		})
	}

	writeJSONResponse(w, GetTraceSummariesOutput{
		TraceSummaries:       responses,
		TracesProcessedCount: int64(len(responses)),
	})
}

// BatchGetTraces handles the BatchGetTraces operation.
func (s *Service) BatchGetTraces(w http.ResponseWriter, r *http.Request) {
	var req BatchGetTracesInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if len(req.TraceIDs) == 0 {
		writeError(w, errInvalidRequest, "TraceIds is required", http.StatusBadRequest)

		return
	}

	traces, unprocessed, err := s.storage.BatchGetTraces(r.Context(), req.TraceIDs)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	responses := make([]TraceResponse, 0, len(traces))

	for _, trace := range traces {
		segments := make([]SegmentResponse, 0, len(trace.Segments))

		for _, seg := range trace.Segments {
			segments = append(segments, SegmentResponse{
				ID:       seg.ID,
				Document: seg.Document,
			})
		}

		responses = append(responses, TraceResponse{
			ID:            trace.ID,
			Duration:      trace.Duration,
			LimitExceeded: trace.LimitExceeded,
			Segments:      segments,
		})
	}

	writeJSONResponse(w, BatchGetTracesOutput{
		Traces:              responses,
		UnprocessedTraceIDs: unprocessed,
	})
}

// GetServiceGraph handles the GetServiceGraph operation.
func (s *Service) GetServiceGraph(w http.ResponseWriter, r *http.Request) {
	var req GetServiceGraphInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.StartTime == nil || req.EndTime == nil {
		writeError(w, errInvalidRequest, "StartTime and EndTime are required", http.StatusBadRequest)

		return
	}

	services, err := s.storage.GetServiceGraph(r.Context(), req.StartTime.ToTime(), req.EndTime.ToTime(), req.GroupName)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, GetServiceGraphOutput{
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Services:  services,
	})
}

// CreateGroup handles the CreateGroup operation.
func (s *Service) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateGroupInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.GroupName == "" {
		writeError(w, errInvalidRequest, "GroupName is required", http.StatusBadRequest)

		return
	}

	group, err := s.storage.CreateGroup(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, CreateGroupOutput{
		Group: &GroupResponse{
			GroupName:             group.GroupName,
			GroupARN:              group.GroupARN,
			FilterExpression:      group.FilterExpression,
			InsightsConfiguration: group.InsightsConfiguration,
		},
	})
}

// DeleteGroup handles the DeleteGroup operation.
func (s *Service) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	var req DeleteGroupInput
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeError(w, errInvalidRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.GroupName == "" && req.GroupARN == "" {
		writeError(w, errInvalidRequest, "GroupName or GroupARN is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteGroup(r.Context(), req.GroupName, req.GroupARN); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// Helper functions.

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeJSON, v)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeJSON, code, message, status)
}

// handleStorageError handles storage errors and writes appropriate response.
func handleStorageError(w http.ResponseWriter, err error) {
	var xrayErr *Error
	if errors.As(err, &xrayErr) {
		status := http.StatusBadRequest
		if xrayErr.Code == errNotFound {
			status = http.StatusNotFound
		}

		writeError(w, xrayErr.Code, xrayErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", "Internal server error", http.StatusInternalServerError)
}
