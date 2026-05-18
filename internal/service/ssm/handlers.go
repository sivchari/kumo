package ssm

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// PutParameter handles the PutParameter API.
func (s *Service) PutParameter(w http.ResponseWriter, r *http.Request) {
	var req PutParameterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSMError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeSSMError(w, ErrInvalidParameterValue, "Name is required", http.StatusBadRequest)

		return
	}

	if req.Value == "" {
		writeSSMError(w, ErrInvalidParameterValue, "Value is required", http.StatusBadRequest)

		return
	}

	param, err := s.storage.PutParameter(r.Context(), &req)
	if err != nil {
		handleSSMError(w, err)

		return
	}

	resp := &PutParameterResponse{
		Version: param.Version,
		Tier:    param.Tier,
	}

	writeJSONResponse(w, resp)
}

// GetParameter handles the GetParameter API.
func (s *Service) GetParameter(w http.ResponseWriter, r *http.Request) {
	var req GetParameterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSMError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeSSMError(w, ErrInvalidParameterValue, "Name is required", http.StatusBadRequest)

		return
	}

	param, err := s.storage.GetParameter(r.Context(), req.Name)
	if err != nil {
		handleSSMError(w, err)

		return
	}

	resp := &GetParameterResponse{
		Parameter: parameterToValue(param, req.WithDecryption),
	}

	writeJSONResponse(w, resp)
}

// GetParameters handles the GetParameters API.
func (s *Service) GetParameters(w http.ResponseWriter, r *http.Request) {
	var req GetParametersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSMError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if len(req.Names) == 0 {
		writeSSMError(w, ErrInvalidParameterValue, "Names is required", http.StatusBadRequest)

		return
	}

	params, invalidParams, err := s.storage.GetParameters(r.Context(), req.Names)
	if err != nil {
		writeSSMError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	resp := &GetParametersResponse{
		Parameters:        make([]*ParameterValue, 0, len(params)),
		InvalidParameters: invalidParams,
	}

	for _, p := range params {
		resp.Parameters = append(resp.Parameters, parameterToValue(p, req.WithDecryption))
	}

	writeJSONResponse(w, resp)
}

// GetParametersByPath handles the GetParametersByPath API.
func (s *Service) GetParametersByPath(w http.ResponseWriter, r *http.Request) {
	var req GetParametersByPathRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSMError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Path == "" {
		writeSSMError(w, ErrInvalidParameterValue, "Path is required", http.StatusBadRequest)

		return
	}

	params, nextToken, err := s.storage.GetParametersByPath(r.Context(), req.Path, req.Recursive, req.MaxResults, req.NextToken)
	if err != nil {
		writeSSMError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	resp := &GetParametersByPathResponse{
		Parameters: make([]*ParameterValue, 0, len(params)),
		NextToken:  nextToken,
	}

	for _, p := range params {
		resp.Parameters = append(resp.Parameters, parameterToValue(p, req.WithDecryption))
	}

	writeJSONResponse(w, resp)
}

// DeleteParameter handles the DeleteParameter API.
func (s *Service) DeleteParameter(w http.ResponseWriter, r *http.Request) {
	var req DeleteParameterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSMError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeSSMError(w, ErrInvalidParameterValue, "Name is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteParameter(r.Context(), req.Name)
	if err != nil {
		handleSSMError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// DeleteParameters handles the DeleteParameters API.
func (s *Service) DeleteParameters(w http.ResponseWriter, r *http.Request) {
	var req DeleteParametersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSMError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if len(req.Names) == 0 {
		writeSSMError(w, ErrInvalidParameterValue, "Names is required", http.StatusBadRequest)

		return
	}

	deleted, invalid, err := s.storage.DeleteParameters(r.Context(), req.Names)
	if err != nil {
		writeSSMError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	resp := &DeleteParametersResponse{
		DeletedParameters: deleted,
		InvalidParameters: invalid,
	}

	writeJSONResponse(w, resp)
}

// DescribeParameters handles the DescribeParameters API.
func (s *Service) DescribeParameters(w http.ResponseWriter, r *http.Request) {
	var req DescribeParametersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSMError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	params, nextToken, err := s.storage.DescribeParameters(r.Context(), req.ParameterFilters, req.MaxResults, req.NextToken)
	if err != nil {
		writeSSMError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)

		return
	}

	resp := &DescribeParametersResponse{
		Parameters: make([]*ParameterMetadata, 0, len(params)),
		NextToken:  nextToken,
	}

	for _, p := range params {
		resp.Parameters = append(resp.Parameters, parameterToMetadata(p))
	}

	writeJSONResponse(w, resp)
}

// parameterToValue converts a Parameter to ParameterValue.
// For SecureString parameters, the value is masked when withDecryption is false.
func parameterToValue(p *Parameter, withDecryption bool) *ParameterValue {
	value := p.Value
	if p.Type == ParameterTypeSecureString && !withDecryption {
		value = "kms:alias/aws/ssm:" + value
	}

	return &ParameterValue{
		Name:             p.Name,
		Type:             p.Type,
		Value:            value,
		Version:          p.Version,
		LastModifiedDate: toUnixTimestamp(p.LastModifiedDate),
		ARN:              p.ARN,
		DataType:         p.DataType,
	}
}

// parameterToMetadata converts a Parameter to ParameterMetadata.
func parameterToMetadata(p *Parameter) *ParameterMetadata {
	return &ParameterMetadata{
		Name:             p.Name,
		Type:             p.Type,
		Description:      p.Description,
		Version:          p.Version,
		LastModifiedDate: toUnixTimestamp(p.LastModifiedDate),
		Tier:             p.Tier,
		DataType:         p.DataType,
	}
}

// toUnixTimestamp converts a time.Time to Unix timestamp as float64.
func toUnixTimestamp(t time.Time) float64 {
	return float64(t.Unix()) + float64(t.Nanosecond())/1e9
}

// handleSSMError handles ParameterError and writes appropriate response.
func handleSSMError(w http.ResponseWriter, err error) {
	var ssmErr *ParameterError
	if errors.As(err, &ssmErr) {
		// SSM returns 400 for all parameter errors (not found, already exists, etc.)
		writeSSMError(w, ssmErr.Type, ssmErr.Message, http.StatusBadRequest)

		return
	}

	writeSSMError(w, ErrServiceException, "Internal server error", http.StatusInternalServerError)
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("X-Amzn-Requestid", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// ListTagsForResource handles the ListTagsForResource API.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	var req ListTagsForResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSMError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListTagsForResource(r.Context(), req.ResourceType, req.ResourceID)
	if err != nil {
		handleSSMError(w, err)

		return
	}

	writeJSONResponse(w, &ListTagsForResourceResponse{TagList: tags})
}

// AddTagsToResource handles the AddTagsToResource API.
func (s *Service) AddTagsToResource(w http.ResponseWriter, r *http.Request) {
	var req AddTagsToResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSMError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.AddTagsToResource(r.Context(), req.ResourceType, req.ResourceID, req.Tags); err != nil {
		handleSSMError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// RemoveTagsFromResource handles the RemoveTagsFromResource API.
func (s *Service) RemoveTagsFromResource(w http.ResponseWriter, r *http.Request) {
	var req RemoveTagsFromResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSSMError(w, ErrInvalidParameterValue, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.RemoveTagsFromResource(r.Context(), req.ResourceType, req.ResourceID, req.TagKeys); err != nil {
		handleSSMError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// writeSSMError writes an SSM error response.
func writeSSMError(w http.ResponseWriter, errType, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("X-Amzn-Requestid", uuid.New().String())
	w.Header().Set("X-Amzn-Errortype", errType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&ParameterError{
		Type:    errType,
		Message: message,
	})
}
