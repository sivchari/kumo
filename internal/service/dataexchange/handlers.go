// Package dataexchange provides an AWS Data Exchange service emulator.
package dataexchange

import (
	"encoding/json"
	"net/http"
	"strings"
)

// CreateDataSet handles POST /v1/data-sets.
func (s *Service) CreateDataSet(w http.ResponseWriter, r *http.Request) {
	var input CreateDataSetInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, "ValidationException", "invalid request body", http.StatusBadRequest)

		return
	}

	if input.Name == "" {
		writeError(w, "ValidationException", "name is required", http.StatusBadRequest)

		return
	}

	if input.AssetType == "" {
		input.AssetType = "S3_SNAPSHOT"
	}

	ds := s.storage.CreateDataSet(&input)
	writeJSON(w, http.StatusCreated, ds)
}

// GetDataSet handles GET /v1/data-sets/{dataSetId}.
func (s *Service) GetDataSet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("dataSetId")
	if id == "" {
		writeError(w, "ValidationException", "data set ID is required", http.StatusBadRequest)

		return
	}

	ds, err := s.storage.GetDataSet(id)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, http.StatusOK, ds)
}

// ListDataSets handles GET /v1/data-sets.
func (s *Service) ListDataSets(w http.ResponseWriter, _ *http.Request) {
	dataSets := s.storage.ListDataSets()
	writeJSON(w, http.StatusOK, &DataSetsResponse{
		DataSets: dataSets,
	})
}

// UpdateDataSet handles PUT /v1/data-sets/{dataSetId}.
func (s *Service) UpdateDataSet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("dataSetId")
	if id == "" {
		writeError(w, "ValidationException", "data set ID is required", http.StatusBadRequest)

		return
	}

	var input UpdateDataSetInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, "ValidationException", "invalid request body", http.StatusBadRequest)

		return
	}

	ds, err := s.storage.UpdateDataSet(id, &input)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, http.StatusOK, ds)
}

// DeleteDataSet handles DELETE /v1/data-sets/{dataSetId}.
func (s *Service) DeleteDataSet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("dataSetId")
	if id == "" {
		writeError(w, "ValidationException", "data set ID is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteDataSet(id); err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateRevision handles POST /v1/data-sets/{dataSetId}/revisions.
func (s *Service) CreateRevision(w http.ResponseWriter, r *http.Request) {
	dataSetID := r.PathValue("dataSetId")
	if dataSetID == "" {
		writeError(w, "ValidationException", "data set ID is required", http.StatusBadRequest)

		return
	}

	var input CreateRevisionInput
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, "ValidationException", "invalid request body", http.StatusBadRequest)

			return
		}
	}

	rev, err := s.storage.CreateRevision(dataSetID, &input)
	if err != nil {
		if strings.Contains(err.Error(), "ResourceNotFoundException") {
			writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

			return
		}

		writeError(w, "InternalServerException", err.Error(), http.StatusInternalServerError)

		return
	}

	writeJSON(w, http.StatusCreated, rev)
}

// GetRevision handles GET /v1/data-sets/{dataSetId}/revisions/{revisionId}.
func (s *Service) GetRevision(w http.ResponseWriter, r *http.Request) {
	dataSetID := r.PathValue("dataSetId")
	revisionID := r.PathValue("revisionId")

	if dataSetID == "" || revisionID == "" {
		writeError(w, "ValidationException", "data set ID and revision ID are required", http.StatusBadRequest)

		return
	}

	rev, err := s.storage.GetRevision(dataSetID, revisionID)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, http.StatusOK, rev)
}

// ListRevisions handles GET /v1/data-sets/{dataSetId}/revisions.
func (s *Service) ListRevisions(w http.ResponseWriter, r *http.Request) {
	dataSetID := r.PathValue("dataSetId")
	if dataSetID == "" {
		writeError(w, "ValidationException", "data set ID is required", http.StatusBadRequest)

		return
	}

	revisions, err := s.storage.ListRevisions(dataSetID)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, http.StatusOK, &RevisionsResponse{
		Revisions: revisions,
	})
}

// UpdateRevision handles PUT /v1/data-sets/{dataSetId}/revisions/{revisionId}.
func (s *Service) UpdateRevision(w http.ResponseWriter, r *http.Request) {
	dataSetID := r.PathValue("dataSetId")
	revisionID := r.PathValue("revisionId")

	if dataSetID == "" || revisionID == "" {
		writeError(w, "ValidationException", "data set ID and revision ID are required", http.StatusBadRequest)

		return
	}

	var input UpdateRevisionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, "ValidationException", "invalid request body", http.StatusBadRequest)

		return
	}

	rev, err := s.storage.UpdateRevision(dataSetID, revisionID, &input)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, http.StatusOK, rev)
}

// DeleteRevision handles DELETE /v1/data-sets/{dataSetId}/revisions/{revisionId}.
func (s *Service) DeleteRevision(w http.ResponseWriter, r *http.Request) {
	dataSetID := r.PathValue("dataSetId")
	revisionID := r.PathValue("revisionId")

	if dataSetID == "" || revisionID == "" {
		writeError(w, "ValidationException", "data set ID and revision ID are required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteRevision(dataSetID, revisionID); err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateJob handles POST /v1/jobs.
func (s *Service) CreateJob(w http.ResponseWriter, r *http.Request) {
	var input CreateJobInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, "ValidationException", "invalid request body", http.StatusBadRequest)

		return
	}

	if input.Type == "" {
		writeError(w, "ValidationException", "type is required", http.StatusBadRequest)

		return
	}

	job := s.storage.CreateJob(&input)
	writeJSON(w, http.StatusCreated, job)
}

// GetJob handles GET /v1/jobs/{jobId}.
func (s *Service) GetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("jobId")
	if id == "" {
		writeError(w, "ValidationException", "job ID is required", http.StatusBadRequest)

		return
	}

	job, err := s.storage.GetJob(id)
	if err != nil {
		writeError(w, "ResourceNotFoundException", err.Error(), http.StatusNotFound)

		return
	}

	writeJSON(w, http.StatusOK, job)
}

// ListJobs handles GET /v1/jobs.
func (s *Service) ListJobs(w http.ResponseWriter, _ *http.Request) {
	jobs := s.storage.ListJobs()
	writeJSON(w, http.StatusOK, &JobsResponse{
		Jobs: jobs,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, errType, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-ErrorType", errType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&ErrorResponse{
		Message: message,
		Type:    errType,
	})
}
