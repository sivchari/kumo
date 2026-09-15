package location

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

// CreateMap handles the CreateMap API.
func (s *Service) CreateMap(w http.ResponseWriter, r *http.Request) {
	var req CreateMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	if req.MapName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgMapNameRequired,
		})

		return
	}

	result, err := s.storage.CreateMap(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DescribeMap handles the DescribeMap API.
func (s *Service) DescribeMap(w http.ResponseWriter, r *http.Request) {
	mapName := r.PathValue("MapName")
	if mapName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgMapNameRequired,
		})

		return
	}

	result, err := s.storage.DescribeMap(r.Context(), mapName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// UpdateMap handles the UpdateMap API.
func (s *Service) UpdateMap(w http.ResponseWriter, r *http.Request) {
	mapName := r.PathValue("MapName")
	if mapName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgMapNameRequired,
		})

		return
	}

	var req UpdateMapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.UpdateMap(r.Context(), mapName, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DeleteMap handles the DeleteMap API.
func (s *Service) DeleteMap(w http.ResponseWriter, r *http.Request) {
	mapName := r.PathValue("MapName")
	if mapName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgMapNameRequired,
		})

		return
	}

	if err := s.storage.DeleteMap(r.Context(), mapName); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, struct{}{})
}

// ListMaps handles the ListMaps API.
func (s *Service) ListMaps(w http.ResponseWriter, r *http.Request) {
	var req ListMapsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.ListMaps(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// CreatePlaceIndex handles the CreatePlaceIndex API.
func (s *Service) CreatePlaceIndex(w http.ResponseWriter, r *http.Request) {
	var req CreatePlaceIndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	if req.IndexName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIndexNameRequired,
		})

		return
	}

	result, err := s.storage.CreatePlaceIndex(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DescribePlaceIndex handles the DescribePlaceIndex API.
func (s *Service) DescribePlaceIndex(w http.ResponseWriter, r *http.Request) {
	indexName := r.PathValue("IndexName")
	if indexName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIndexNameRequired,
		})

		return
	}

	result, err := s.storage.DescribePlaceIndex(r.Context(), indexName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// UpdatePlaceIndex handles the UpdatePlaceIndex API.
func (s *Service) UpdatePlaceIndex(w http.ResponseWriter, r *http.Request) {
	indexName := r.PathValue("IndexName")
	if indexName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIndexNameRequired,
		})

		return
	}

	var req UpdatePlaceIndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.UpdatePlaceIndex(r.Context(), indexName, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DeletePlaceIndex handles the DeletePlaceIndex API.
func (s *Service) DeletePlaceIndex(w http.ResponseWriter, r *http.Request) {
	indexName := r.PathValue("IndexName")
	if indexName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIndexNameRequired,
		})

		return
	}

	if err := s.storage.DeletePlaceIndex(r.Context(), indexName); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, struct{}{})
}

// ListPlaceIndexes handles the ListPlaceIndexes API.
func (s *Service) ListPlaceIndexes(w http.ResponseWriter, r *http.Request) {
	var req ListPlaceIndexesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.ListPlaceIndexes(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// CreateRouteCalculator handles the CreateRouteCalculator API.
func (s *Service) CreateRouteCalculator(w http.ResponseWriter, r *http.Request) {
	var req CreateRouteCalculatorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	if req.CalculatorName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgCalculatorNameRequired,
		})

		return
	}

	result, err := s.storage.CreateRouteCalculator(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DescribeRouteCalculator handles the DescribeRouteCalculator API.
func (s *Service) DescribeRouteCalculator(w http.ResponseWriter, r *http.Request) {
	calcName := r.PathValue("CalculatorName")
	if calcName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgCalculatorNameRequired,
		})

		return
	}

	result, err := s.storage.DescribeRouteCalculator(r.Context(), calcName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// UpdateRouteCalculator handles the UpdateRouteCalculator API.
func (s *Service) UpdateRouteCalculator(w http.ResponseWriter, r *http.Request) {
	calcName := r.PathValue("CalculatorName")
	if calcName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgCalculatorNameRequired,
		})

		return
	}

	var req UpdateRouteCalculatorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.UpdateRouteCalculator(r.Context(), calcName, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DeleteRouteCalculator handles the DeleteRouteCalculator API.
func (s *Service) DeleteRouteCalculator(w http.ResponseWriter, r *http.Request) {
	calcName := r.PathValue("CalculatorName")
	if calcName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgCalculatorNameRequired,
		})

		return
	}

	if err := s.storage.DeleteRouteCalculator(r.Context(), calcName); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, struct{}{})
}

// ListRouteCalculators handles the ListRouteCalculators API.
func (s *Service) ListRouteCalculators(w http.ResponseWriter, r *http.Request) {
	var req ListRouteCalculatorsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.ListRouteCalculators(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// CreateGeofenceCollection handles the CreateGeofenceCollection API.
func (s *Service) CreateGeofenceCollection(w http.ResponseWriter, r *http.Request) {
	var req CreateGeofenceCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	if req.CollectionName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgCollectionNameRequired,
		})

		return
	}

	result, err := s.storage.CreateGeofenceCollection(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DescribeGeofenceCollection handles the DescribeGeofenceCollection API.
func (s *Service) DescribeGeofenceCollection(w http.ResponseWriter, r *http.Request) {
	collName := r.PathValue("CollectionName")
	if collName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgCollectionNameRequired,
		})

		return
	}

	result, err := s.storage.DescribeGeofenceCollection(r.Context(), collName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// UpdateGeofenceCollection handles the UpdateGeofenceCollection API.
func (s *Service) UpdateGeofenceCollection(w http.ResponseWriter, r *http.Request) {
	collName := r.PathValue("CollectionName")
	if collName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgCollectionNameRequired,
		})

		return
	}

	var req UpdateGeofenceCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.UpdateGeofenceCollection(r.Context(), collName, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DeleteGeofenceCollection handles the DeleteGeofenceCollection API.
func (s *Service) DeleteGeofenceCollection(w http.ResponseWriter, r *http.Request) {
	collName := r.PathValue("CollectionName")
	if collName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgCollectionNameRequired,
		})

		return
	}

	if err := s.storage.DeleteGeofenceCollection(r.Context(), collName); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, struct{}{})
}

// ListGeofenceCollections handles the ListGeofenceCollections API.
func (s *Service) ListGeofenceCollections(w http.ResponseWriter, r *http.Request) {
	var req ListGeofenceCollectionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.ListGeofenceCollections(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// CreateTracker handles the CreateTracker API.
func (s *Service) CreateTracker(w http.ResponseWriter, r *http.Request) {
	var req CreateTrackerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	if req.TrackerName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgTrackerNameRequired,
		})

		return
	}

	result, err := s.storage.CreateTracker(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DescribeTracker handles the DescribeTracker API.
func (s *Service) DescribeTracker(w http.ResponseWriter, r *http.Request) {
	trackerName := r.PathValue("TrackerName")
	if trackerName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgTrackerNameRequired,
		})

		return
	}

	result, err := s.storage.DescribeTracker(r.Context(), trackerName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// UpdateTracker handles the UpdateTracker API.
func (s *Service) UpdateTracker(w http.ResponseWriter, r *http.Request) {
	trackerName := r.PathValue("TrackerName")
	if trackerName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgTrackerNameRequired,
		})

		return
	}

	var req UpdateTrackerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.UpdateTracker(r.Context(), trackerName, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DeleteTracker handles the DeleteTracker API.
func (s *Service) DeleteTracker(w http.ResponseWriter, r *http.Request) {
	trackerName := r.PathValue("TrackerName")
	if trackerName == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgTrackerNameRequired,
		})

		return
	}

	if err := s.storage.DeleteTracker(r.Context(), trackerName); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, struct{}{})
}

// ListTrackers handles the ListTrackers API.
func (s *Service) ListTrackers(w http.ResponseWriter, r *http.Request) {
	var req ListTrackersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.ListTrackers(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// Helper functions.

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, resp any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	_ = json.NewEncoder(w).Encode(resp)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, err *Error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&ErrorResponse{
		Type:    err.Code,
		Message: err.Message,
	})
}

// handleError converts storage errors to HTTP error responses.
func handleError(w http.ResponseWriter, err error) {
	var locErr *Error
	if errors.As(err, &locErr) {
		status := getErrorStatus(locErr.Code)
		writeError(w, status, locErr)

		return
	}

	writeError(w, http.StatusInternalServerError, &Error{
		Code:    errInternalServerException,
		Message: err.Error(),
	})
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errResourceNotFoundException:
		return http.StatusNotFound
	case errValidationException:
		return http.StatusBadRequest
	case errConflictException:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
