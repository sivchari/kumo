package rekognition

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
		// Collection management
		"CreateCollection":   s.CreateCollection,
		"DeleteCollection":   s.DeleteCollection,
		"ListCollections":    s.ListCollections,
		"DescribeCollection": s.DescribeCollection,

		// Face operations
		"IndexFaces":  s.IndexFaces,
		"ListFaces":   s.ListFaces,
		"SearchFaces": s.SearchFaces,
		"DeleteFaces": s.DeleteFaces,

		// Detection operations
		"DetectFaces":            s.DetectFaces,
		"DetectLabels":           s.DetectLabels,
		"DetectText":             s.DetectText,
		"RecognizeCelebrities":   s.RecognizeCelebrities,
		"DetectModerationLabels": s.DetectModerationLabels,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "RekognitionService.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "InvalidAction", "The action "+action+" is not valid for this endpoint.", http.StatusBadRequest)
}

// CreateCollection handles the CreateCollection API.
func (s *Service) CreateCollection(w http.ResponseWriter, r *http.Request) {
	var req CreateCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.CreateCollection(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// DeleteCollection handles the DeleteCollection API.
func (s *Service) DeleteCollection(w http.ResponseWriter, r *http.Request) {
	var req DeleteCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.CollectionID == "" {
		writeError(w, errInvalidParameter, "CollectionId is required", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.DeleteCollection(r.Context(), req.CollectionID)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// ListCollections handles the ListCollections API.
func (s *Service) ListCollections(w http.ResponseWriter, r *http.Request) {
	var req ListCollectionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is acceptable for list operations
		req = ListCollectionsRequest{}
	}

	resp, err := s.storage.ListCollections(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// DescribeCollection handles the DescribeCollection API.
func (s *Service) DescribeCollection(w http.ResponseWriter, r *http.Request) {
	var req DescribeCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.CollectionID == "" {
		writeError(w, errInvalidParameter, "CollectionId is required", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.DescribeCollection(r.Context(), req.CollectionID)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// IndexFaces handles the IndexFaces API.
func (s *Service) IndexFaces(w http.ResponseWriter, r *http.Request) {
	var req IndexFacesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.IndexFaces(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// ListFaces handles the ListFaces API.
func (s *Service) ListFaces(w http.ResponseWriter, r *http.Request) {
	var req ListFacesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.ListFaces(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// SearchFaces handles the SearchFaces API.
func (s *Service) SearchFaces(w http.ResponseWriter, r *http.Request) {
	var req SearchFacesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.SearchFaces(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// DeleteFaces handles the DeleteFaces API.
func (s *Service) DeleteFaces(w http.ResponseWriter, r *http.Request) {
	var req DeleteFacesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.DeleteFaces(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// DetectFaces handles the DetectFaces API.
func (s *Service) DetectFaces(w http.ResponseWriter, r *http.Request) {
	var req DetectFacesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.DetectFaces(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// DetectLabels handles the DetectLabels API.
func (s *Service) DetectLabels(w http.ResponseWriter, r *http.Request) {
	var req DetectLabelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.DetectLabels(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// DetectText handles the DetectText API.
func (s *Service) DetectText(w http.ResponseWriter, r *http.Request) {
	var req DetectTextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.DetectText(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// RecognizeCelebrities handles the RecognizeCelebrities API.
func (s *Service) RecognizeCelebrities(w http.ResponseWriter, r *http.Request) {
	var req RecognizeCelebritiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.RecognizeCelebrities(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
}

// DetectModerationLabels handles the DetectModerationLabels API.
func (s *Service) DetectModerationLabels(w http.ResponseWriter, r *http.Request) {
	var req DetectModerationLabelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errInvalidParameter, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.DetectModerationLabels(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, resp)
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

// handleError handles Rekognition errors.
func handleError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		status := getErrorStatus(svcErr.Code)
		writeError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeError(w, errInternalServer, err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errResourceNotFound:
		return http.StatusBadRequest
	case errResourceExists:
		return http.StatusBadRequest
	case errInvalidParameter:
		return http.StatusBadRequest
	case errAccessDenied:
		return http.StatusForbidden
	case errImageTooLarge:
		return http.StatusBadRequest
	case errInvalidImageFormat:
		return http.StatusBadRequest
	case errProvisionedThroughput, errThrottling:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}
