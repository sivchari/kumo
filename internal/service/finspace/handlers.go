package finspace

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// CreateKxEnvironment handles the CreateKxEnvironment action.
func (s *Service) CreateKxEnvironment(w http.ResponseWriter, r *http.Request) {
	var req CreateKxEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.CreateKxEnvironment(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// GetKxEnvironment handles the GetKxEnvironment action.
func (s *Service) GetKxEnvironment(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")

	resp, err := s.storage.GetKxEnvironment(r.Context(), environmentID)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// DeleteKxEnvironment handles the DeleteKxEnvironment action.
func (s *Service) DeleteKxEnvironment(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")

	if err := s.storage.DeleteKxEnvironment(r.Context(), environmentID); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &DeleteKxEnvironmentResponse{})
}

// ListKxEnvironments handles the ListKxEnvironments action.
func (s *Service) ListKxEnvironments(w http.ResponseWriter, r *http.Request) {
	maxResults := 0

	if maxResultsStr := r.URL.Query().Get("maxResults"); maxResultsStr != "" {
		var err error

		maxResults, err = strconv.Atoi(maxResultsStr)
		if err != nil {
			writeError(w, errValidation, "Invalid maxResults", http.StatusBadRequest)

			return
		}
	}

	nextToken := r.URL.Query().Get("nextToken")

	resp, err := s.storage.ListKxEnvironments(r.Context(), maxResults, nextToken)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// UpdateKxEnvironment handles the UpdateKxEnvironment action.
func (s *Service) UpdateKxEnvironment(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")

	var req UpdateKxEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.EnvironmentID = environmentID

	resp, err := s.storage.UpdateKxEnvironment(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// CreateKxDatabase handles the CreateKxDatabase action.
func (s *Service) CreateKxDatabase(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")

	var req CreateKxDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.EnvironmentID = environmentID

	resp, err := s.storage.CreateKxDatabase(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// GetKxDatabase handles the GetKxDatabase action.
func (s *Service) GetKxDatabase(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")
	databaseName := r.PathValue("databaseName")

	resp, err := s.storage.GetKxDatabase(r.Context(), environmentID, databaseName)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// DeleteKxDatabase handles the DeleteKxDatabase action.
func (s *Service) DeleteKxDatabase(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")
	databaseName := r.PathValue("databaseName")

	if err := s.storage.DeleteKxDatabase(r.Context(), environmentID, databaseName); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &DeleteKxDatabaseResponse{})
}

// ListKxDatabases handles the ListKxDatabases action.
func (s *Service) ListKxDatabases(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")
	maxResults := 0

	if maxResultsStr := r.URL.Query().Get("maxResults"); maxResultsStr != "" {
		var err error

		maxResults, err = strconv.Atoi(maxResultsStr)
		if err != nil {
			writeError(w, errValidation, "Invalid maxResults", http.StatusBadRequest)

			return
		}
	}

	nextToken := r.URL.Query().Get("nextToken")

	resp, err := s.storage.ListKxDatabases(r.Context(), environmentID, maxResults, nextToken)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// UpdateKxDatabase handles the UpdateKxDatabase action.
func (s *Service) UpdateKxDatabase(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")
	databaseName := r.PathValue("databaseName")

	var req UpdateKxDatabaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.EnvironmentID = environmentID
	req.DatabaseName = databaseName

	resp, err := s.storage.UpdateKxDatabase(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// CreateKxUser handles the CreateKxUser action.
func (s *Service) CreateKxUser(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")

	var req CreateKxUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.EnvironmentID = environmentID

	resp, err := s.storage.CreateKxUser(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// GetKxUser handles the GetKxUser action.
func (s *Service) GetKxUser(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")
	userName := r.PathValue("userName")

	resp, err := s.storage.GetKxUser(r.Context(), environmentID, userName)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// DeleteKxUser handles the DeleteKxUser action.
func (s *Service) DeleteKxUser(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")
	userName := r.PathValue("userName")

	if err := s.storage.DeleteKxUser(r.Context(), environmentID, userName); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &DeleteKxUserResponse{})
}

// ListKxUsers handles the ListKxUsers action.
func (s *Service) ListKxUsers(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")
	maxResults := 0

	if maxResultsStr := r.URL.Query().Get("maxResults"); maxResultsStr != "" {
		var err error

		maxResults, err = strconv.Atoi(maxResultsStr)
		if err != nil {
			writeError(w, errValidation, "Invalid maxResults", http.StatusBadRequest)

			return
		}
	}

	nextToken := r.URL.Query().Get("nextToken")

	resp, err := s.storage.ListKxUsers(r.Context(), environmentID, maxResults, nextToken)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// UpdateKxUser handles the UpdateKxUser action.
func (s *Service) UpdateKxUser(w http.ResponseWriter, r *http.Request) {
	environmentID := r.PathValue("environmentId")
	userName := r.PathValue("userName")

	var req UpdateKxUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.EnvironmentID = environmentID
	req.UserName = userName

	resp, err := s.storage.UpdateKxUser(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, resp)
}

// TagResource handles the TagResource action.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	resourceARN := r.URL.Query().Get("resourceArn")

	var req TagResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(r.Context(), resourceARN, req.Tags); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &TagResourceResponse{})
}

// UntagResource handles the UntagResource action.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	resourceARN := r.URL.Query().Get("resourceArn")
	tagKeys := r.URL.Query()["tagKeys"]

	if err := s.storage.UntagResource(r.Context(), resourceARN, tagKeys); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &UntagResourceResponse{})
}

// ListTagsForResource handles the ListTagsForResource action.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	resourceARN := r.URL.Query().Get("resourceArn")

	tags, err := s.storage.ListTagsForResource(r.Context(), resourceARN)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &ListTagsForResourceResponse{Tags: tags})
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, code, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errResp := &Error{
		Code:    code,
		Message: message,
	}

	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleStorageError(w http.ResponseWriter, err error) {
	var fsErr *Error

	if errors.As(err, &fsErr) {
		statusCode := http.StatusBadRequest

		switch fsErr.Code {
		case errResourceNotFound:
			statusCode = http.StatusNotFound
		case errConflict:
			statusCode = http.StatusConflict
		}

		writeError(w, fsErr.Code, fsErr.Message, statusCode)

		return
	}

	writeError(w, "InternalException", err.Error(), http.StatusInternalServerError)
}
