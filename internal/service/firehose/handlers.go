package firehose

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
		"CreateDeliveryStream":   s.CreateDeliveryStream,
		"DeleteDeliveryStream":   s.DeleteDeliveryStream,
		"DescribeDeliveryStream": s.DescribeDeliveryStream,
		"ListDeliveryStreams":    s.ListDeliveryStreams,
		"PutRecord":              s.PutRecord,
		"PutRecordBatch":         s.PutRecordBatch,
		"UpdateDestination":      s.UpdateDestination,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "Firehose_20150804.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "InvalidAction", "The action "+action+" is not valid for this endpoint.", http.StatusBadRequest)
}

// CreateDeliveryStream handles the CreateDeliveryStream API.
func (s *Service) CreateDeliveryStream(w http.ResponseWriter, r *http.Request) {
	var req CreateDeliveryStreamInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DeliveryStreamName == "" {
		writeError(w, "ValidationException", "DeliveryStreamName is required", http.StatusBadRequest)

		return
	}

	stream, err := s.storage.CreateDeliveryStream(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &CreateDeliveryStreamOutput{
		DeliveryStreamARN: stream.DeliveryStreamARN,
	})
}

// DeleteDeliveryStream handles the DeleteDeliveryStream API.
func (s *Service) DeleteDeliveryStream(w http.ResponseWriter, r *http.Request) {
	var req DeleteDeliveryStreamInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DeliveryStreamName == "" {
		writeError(w, "ValidationException", "DeliveryStreamName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteDeliveryStream(r.Context(), req.DeliveryStreamName, req.AllowForceDelete); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteDeliveryStreamOutput{})
}

// DescribeDeliveryStream handles the DescribeDeliveryStream API.
func (s *Service) DescribeDeliveryStream(w http.ResponseWriter, r *http.Request) {
	var req DescribeDeliveryStreamInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DeliveryStreamName == "" {
		writeError(w, "ValidationException", "DeliveryStreamName is required", http.StatusBadRequest)

		return
	}

	stream, err := s.storage.DescribeDeliveryStream(r.Context(), req.DeliveryStreamName, req.Limit, req.ExclusiveStartDestinationID)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DescribeDeliveryStreamOutput{
		DeliveryStreamDescription: DeliveryStreamDescription{
			DeliveryStreamName:   stream.DeliveryStreamName,
			DeliveryStreamARN:    stream.DeliveryStreamARN,
			DeliveryStreamStatus: string(stream.DeliveryStreamStatus),
			DeliveryStreamType:   string(stream.DeliveryStreamType),
			CreateTimestamp:      float64(stream.CreateTimestamp.Unix()),
			LastUpdateTimestamp:  float64(stream.LastUpdateTimestamp.Unix()),
			VersionID:            stream.VersionID,
			Destinations:         stream.Destinations,
			HasMoreDestinations:  stream.HasMoreDestinations,
			Source:               stream.Source,
		},
	}

	writeResponse(w, resp)
}

// ListDeliveryStreams handles the ListDeliveryStreams API.
func (s *Service) ListDeliveryStreams(w http.ResponseWriter, r *http.Request) {
	var req ListDeliveryStreamsInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	names, hasMore, err := s.storage.ListDeliveryStreams(r.Context(), req.DeliveryStreamType, req.ExclusiveStartDeliveryStreamName, req.Limit)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &ListDeliveryStreamsOutput{
		DeliveryStreamNames:    names,
		HasMoreDeliveryStreams: hasMore,
	})
}

// PutRecord handles the PutRecord API.
func (s *Service) PutRecord(w http.ResponseWriter, r *http.Request) {
	var req PutRecordInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DeliveryStreamName == "" {
		writeError(w, "ValidationException", "DeliveryStreamName is required", http.StatusBadRequest)

		return
	}

	if len(req.Record.Data) == 0 {
		writeError(w, "ValidationException", "Record.Data is required", http.StatusBadRequest)

		return
	}

	recordID, err := s.storage.PutRecord(r.Context(), req.DeliveryStreamName, req.Record)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &PutRecordOutput{
		RecordID: recordID,
	})
}

// PutRecordBatch handles the PutRecordBatch API.
func (s *Service) PutRecordBatch(w http.ResponseWriter, r *http.Request) {
	var req PutRecordBatchInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DeliveryStreamName == "" {
		writeError(w, "ValidationException", "DeliveryStreamName is required", http.StatusBadRequest)

		return
	}

	if len(req.Records) == 0 {
		writeError(w, "ValidationException", "Records is required", http.StatusBadRequest)

		return
	}

	responses, failedCount, err := s.storage.PutRecordBatch(r.Context(), req.DeliveryStreamName, req.Records)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &PutRecordBatchOutput{
		FailedPutCount:   failedCount,
		RequestResponses: responses,
	})
}

// UpdateDestination handles the UpdateDestination API.
func (s *Service) UpdateDestination(w http.ResponseWriter, r *http.Request) {
	var req UpdateDestinationInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.DeliveryStreamName == "" {
		writeError(w, "ValidationException", "DeliveryStreamName is required", http.StatusBadRequest)

		return
	}

	if req.CurrentDeliveryStreamVersionID == "" {
		writeError(w, "ValidationException", "CurrentDeliveryStreamVersionId is required", http.StatusBadRequest)

		return
	}

	if req.DestinationID == "" {
		writeError(w, "ValidationException", "DestinationId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.UpdateDestination(r.Context(), &req); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &UpdateDestinationOutput{})
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
	var svcErr *Error
	if errors.As(err, &svcErr) {
		status := getErrorStatus(svcErr.Code)
		writeError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errResourceNotFound:
		return http.StatusNotFound
	case errResourceInUse:
		return http.StatusConflict
	case errInvalidArgument:
		return http.StatusBadRequest
	case errLimitExceeded:
		return http.StatusBadRequest
	case errServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}
