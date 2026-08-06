package securitylake

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// CreateDataLake handles the CreateDataLake action.
func (s *Service) CreateDataLake(w http.ResponseWriter, r *http.Request) {
	var req CreateDataLakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	dataLakes, err := s.storage.CreateDataLake(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &CreateDataLakeResponse{DataLakes: dataLakes})
}

// DeleteDataLake handles the DeleteDataLake action.
func (s *Service) DeleteDataLake(w http.ResponseWriter, r *http.Request) {
	var req DeleteDataLakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteDataLake(r.Context(), req.Regions); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &DeleteDataLakeResponse{})
}

// ListDataLakes handles the ListDataLakes action.
func (s *Service) ListDataLakes(w http.ResponseWriter, r *http.Request) {
	regions := r.URL.Query()["regions"]

	dataLakes, err := s.storage.ListDataLakes(r.Context(), regions)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &ListDataLakesResponse{DataLakes: dataLakes})
}

// UpdateDataLake handles the UpdateDataLake action.
func (s *Service) UpdateDataLake(w http.ResponseWriter, r *http.Request) {
	var req UpdateDataLakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	dataLakes, err := s.storage.UpdateDataLake(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &UpdateDataLakeResponse{DataLakes: dataLakes})
}

// CreateSubscriber handles the CreateSubscriber action.
func (s *Service) CreateSubscriber(w http.ResponseWriter, r *http.Request) {
	var req CreateSubscriberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	subscriber, err := s.storage.CreateSubscriber(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &CreateSubscriberResponse{Subscriber: subscriber})
}

// GetSubscriber handles the GetSubscriber action.
func (s *Service) GetSubscriber(w http.ResponseWriter, r *http.Request) {
	subscriberID := r.PathValue("subscriberId")

	subscriber, err := s.storage.GetSubscriber(r.Context(), subscriberID)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &GetSubscriberResponse{Subscriber: subscriber})
}

// DeleteSubscriber handles the DeleteSubscriber action.
func (s *Service) DeleteSubscriber(w http.ResponseWriter, r *http.Request) {
	subscriberID := r.PathValue("subscriberId")

	if err := s.storage.DeleteSubscriber(r.Context(), subscriberID); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &DeleteSubscriberResponse{})
}

// UpdateSubscriber handles the UpdateSubscriber action.
func (s *Service) UpdateSubscriber(w http.ResponseWriter, r *http.Request) {
	subscriberID := r.PathValue("subscriberId")

	var req UpdateSubscriberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	req.SubscriberID = subscriberID

	subscriber, err := s.storage.UpdateSubscriber(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &UpdateSubscriberResponse{Subscriber: subscriber})
}

// ListSubscribers handles the ListSubscribers action.
func (s *Service) ListSubscribers(w http.ResponseWriter, r *http.Request) {
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

	subscribers, resultNextToken, err := s.storage.ListSubscribers(r.Context(), maxResults, nextToken)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &ListSubscribersResponse{
		Subscribers: subscribers,
		NextToken:   resultNextToken,
	})
}

// CreateAwsLogSource handles the CreateAwsLogSource action.
func (s *Service) CreateAwsLogSource(w http.ResponseWriter, r *http.Request) {
	var req CreateAwsLogSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	failed, err := s.storage.CreateAwsLogSource(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &CreateAwsLogSourceResponse{Failed: failed})
}

// DeleteAwsLogSource handles the DeleteAwsLogSource action.
func (s *Service) DeleteAwsLogSource(w http.ResponseWriter, r *http.Request) {
	var req DeleteAwsLogSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	failed, err := s.storage.DeleteAwsLogSource(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &DeleteAwsLogSourceResponse{Failed: failed})
}

// ListLogSources handles the ListLogSources action.
func (s *Service) ListLogSources(w http.ResponseWriter, r *http.Request) {
	var req ListLogSourcesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	sources, nextToken, err := s.storage.ListLogSources(r.Context(), &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &ListLogSourcesResponse{
		Sources:   sources,
		NextToken: nextToken,
	})
}

// TagResource handles the TagResource action.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	resourceARN := r.PathValue("resourceArn")

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
	resourceARN := r.PathValue("resourceArn")
	tagKeys := r.URL.Query()["tagKeys"]

	if err := s.storage.UntagResource(r.Context(), resourceARN, tagKeys); err != nil {
		handleStorageError(w, err)

		return
	}

	writeJSON(w, &UntagResourceResponse{})
}

// ListTagsForResource handles the ListTagsForResource action.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	resourceARN := r.PathValue("resourceArn")

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
	var slErr *Error

	if errors.As(err, &slErr) {
		statusCode := http.StatusBadRequest

		switch slErr.Code {
		case errResourceNotFound:
			statusCode = http.StatusNotFound
		case errConflict:
			statusCode = http.StatusConflict
		}

		writeError(w, slErr.Code, slErr.Message, statusCode)

		return
	}

	writeError(w, "InternalException", err.Error(), http.StatusInternalServerError)
}
