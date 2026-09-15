package macie2

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

// EnableMacie handles the EnableMacie API.
func (s *Service) EnableMacie(w http.ResponseWriter, r *http.Request) {
	var req EnableMacieRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.EnableMacie(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// GetMacieSession handles the GetMacieSession API.
func (s *Service) GetMacieSession(w http.ResponseWriter, r *http.Request) {
	result, err := s.storage.GetMacieSession(r.Context())
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// UpdateMacieSession handles the UpdateMacieSession API.
func (s *Service) UpdateMacieSession(w http.ResponseWriter, r *http.Request) {
	var req UpdateMacieSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.UpdateMacieSession(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DisableMacie handles the DisableMacie API.
func (s *Service) DisableMacie(w http.ResponseWriter, r *http.Request) {
	result, err := s.storage.DisableMacie(r.Context())
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// CreateAllowList handles the CreateAllowList API.
func (s *Service) CreateAllowList(w http.ResponseWriter, r *http.Request) {
	var req CreateAllowListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgNameRequired,
		})

		return
	}

	result, err := s.storage.CreateAllowList(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// GetAllowList handles the GetAllowList API.
func (s *Service) GetAllowList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIDRequired,
		})

		return
	}

	result, err := s.storage.GetAllowList(r.Context(), id)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// UpdateAllowList handles the UpdateAllowList API.
func (s *Service) UpdateAllowList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIDRequired,
		})

		return
	}

	var req UpdateAllowListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.UpdateAllowList(r.Context(), id, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DeleteAllowList handles the DeleteAllowList API.
func (s *Service) DeleteAllowList(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIDRequired,
		})

		return
	}

	if err := s.storage.DeleteAllowList(r.Context(), id); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, struct{}{})
}

// ListAllowLists handles the ListAllowLists API.
func (s *Service) ListAllowLists(w http.ResponseWriter, r *http.Request) {
	result, err := s.storage.ListAllowLists(r.Context())
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// CreateClassificationJob handles the CreateClassificationJob API.
func (s *Service) CreateClassificationJob(w http.ResponseWriter, r *http.Request) {
	var req CreateClassificationJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgNameRequired,
		})

		return
	}

	if req.JobType == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: "JobType is required",
		})

		return
	}

	result, err := s.storage.CreateClassificationJob(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DescribeClassificationJob handles the DescribeClassificationJob API.
func (s *Service) DescribeClassificationJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: "JobId is required",
		})

		return
	}

	result, err := s.storage.DescribeClassificationJob(r.Context(), jobID)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// ListClassificationJobs handles the ListClassificationJobs API.
func (s *Service) ListClassificationJobs(w http.ResponseWriter, r *http.Request) {
	var req ListClassificationJobsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.ListClassificationJobs(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// UpdateClassificationJob handles the UpdateClassificationJob API.
func (s *Service) UpdateClassificationJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: "JobId is required",
		})

		return
	}

	var req UpdateClassificationJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.UpdateClassificationJob(r.Context(), jobID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// CreateCustomDataIdentifier handles the CreateCustomDataIdentifier API.
func (s *Service) CreateCustomDataIdentifier(w http.ResponseWriter, r *http.Request) {
	var req CreateCustomDataIdentifierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgNameRequired,
		})

		return
	}

	if req.Regex == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: "Regex is required",
		})

		return
	}

	result, err := s.storage.CreateCustomDataIdentifier(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// GetCustomDataIdentifier handles the GetCustomDataIdentifier API.
func (s *Service) GetCustomDataIdentifier(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIDRequired,
		})

		return
	}

	result, err := s.storage.GetCustomDataIdentifier(r.Context(), id)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DeleteCustomDataIdentifier handles the DeleteCustomDataIdentifier API.
func (s *Service) DeleteCustomDataIdentifier(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIDRequired,
		})

		return
	}

	if err := s.storage.DeleteCustomDataIdentifier(r.Context(), id); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, struct{}{})
}

// ListCustomDataIdentifiers handles the ListCustomDataIdentifiers API.
func (s *Service) ListCustomDataIdentifiers(w http.ResponseWriter, r *http.Request) {
	var req ListCustomDataIdentifiersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.ListCustomDataIdentifiers(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// CreateFindingsFilter handles the CreateFindingsFilter API.
func (s *Service) CreateFindingsFilter(w http.ResponseWriter, r *http.Request) {
	var req CreateFindingsFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgNameRequired,
		})

		return
	}

	if req.Action == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: "Action is required",
		})

		return
	}

	result, err := s.storage.CreateFindingsFilter(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// GetFindingsFilter handles the GetFindingsFilter API.
func (s *Service) GetFindingsFilter(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIDRequired,
		})

		return
	}

	result, err := s.storage.GetFindingsFilter(r.Context(), id)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// UpdateFindingsFilter handles the UpdateFindingsFilter API.
func (s *Service) UpdateFindingsFilter(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIDRequired,
		})

		return
	}

	var req UpdateFindingsFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.UpdateFindingsFilter(r.Context(), id, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// DeleteFindingsFilter handles the DeleteFindingsFilter API.
func (s *Service) DeleteFindingsFilter(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgIDRequired,
		})

		return
	}

	if err := s.storage.DeleteFindingsFilter(r.Context(), id); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, struct{}{})
}

// ListFindingsFilters handles the ListFindingsFilters API.
func (s *Service) ListFindingsFilters(w http.ResponseWriter, r *http.Request) {
	result, err := s.storage.ListFindingsFilters(r.Context())
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// GetFindings handles the GetFindings API.
func (s *Service) GetFindings(w http.ResponseWriter, r *http.Request) {
	var req GetFindingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.GetFindings(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, result)
}

// ListFindings handles the ListFindings API.
func (s *Service) ListFindings(w http.ResponseWriter, r *http.Request) {
	var req ListFindingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    errValidationException,
			Message: msgInvalidRequestBody,
		})

		return
	}

	result, err := s.storage.ListFindings(r.Context(), &req)
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
	var macieErr *Error
	if errors.As(err, &macieErr) {
		status := getErrorStatus(macieErr.Code)
		writeError(w, status, macieErr)

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
	case errServiceQuotaExceededException:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
