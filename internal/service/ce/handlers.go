// Package ce provides AWS Cost Explorer service emulation.
package ce

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
		"GetCostAndUsage":                s.GetCostAndUsage,
		"GetDimensionValues":             s.GetDimensionValues,
		"GetTags":                        s.GetTags,
		"GetCostForecast":                s.GetCostForecast,
		"CreateCostCategoryDefinition":   s.CreateCostCategoryDefinition,
		"DescribeCostCategoryDefinition": s.DescribeCostCategoryDefinition,
		"DeleteCostCategoryDefinition":   s.DeleteCostCategoryDefinition,
		"ListCostCategoryDefinitions":    s.ListCostCategoryDefinitions,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AWSInsightsIndexService.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeCEError(w, "InvalidAction", "The action "+action+" is not valid for this endpoint.", http.StatusBadRequest)
}

// GetCostAndUsage handles the GetCostAndUsage API.
func (s *Service) GetCostAndUsage(w http.ResponseWriter, r *http.Request) {
	var req GetCostAndUsageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCEError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.GetCostAndUsage(r.Context(), &req)
	if err != nil {
		handleCEError(w, err)

		return
	}

	writeCEResponse(w, resp)
}

// GetDimensionValues handles the GetDimensionValues API.
func (s *Service) GetDimensionValues(w http.ResponseWriter, r *http.Request) {
	var req GetDimensionValuesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCEError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.GetDimensionValues(r.Context(), &req)
	if err != nil {
		handleCEError(w, err)

		return
	}

	writeCEResponse(w, resp)
}

// GetTags handles the GetTags API.
func (s *Service) GetTags(w http.ResponseWriter, r *http.Request) {
	var req GetTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCEError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.GetTags(r.Context(), &req)
	if err != nil {
		handleCEError(w, err)

		return
	}

	writeCEResponse(w, resp)
}

// GetCostForecast handles the GetCostForecast API.
func (s *Service) GetCostForecast(w http.ResponseWriter, r *http.Request) {
	var req GetCostForecastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCEError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	resp, err := s.storage.GetCostForecast(r.Context(), &req)
	if err != nil {
		handleCEError(w, err)

		return
	}

	writeCEResponse(w, resp)
}

// CreateCostCategoryDefinition handles the CreateCostCategoryDefinition API.
func (s *Service) CreateCostCategoryDefinition(w http.ResponseWriter, r *http.Request) {
	var req CreateCostCategoryDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCEError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	cc, err := s.storage.CreateCostCategoryDefinition(r.Context(), &req)
	if err != nil {
		handleCEError(w, err)

		return
	}

	resp := &CreateCostCategoryDefinitionResponse{
		CostCategoryArn: cc.CostCategoryArn,
		EffectiveStart:  cc.EffectiveStart,
	}

	writeCEResponse(w, resp)
}

// DescribeCostCategoryDefinition handles the DescribeCostCategoryDefinition API.
func (s *Service) DescribeCostCategoryDefinition(w http.ResponseWriter, r *http.Request) {
	var req DescribeCostCategoryDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCEError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.CostCategoryArn == "" {
		writeCEError(w, errValidation, "CostCategoryArn is required", http.StatusBadRequest)

		return
	}

	cc, err := s.storage.DescribeCostCategoryDefinition(r.Context(), req.CostCategoryArn)
	if err != nil {
		handleCEError(w, err)

		return
	}

	resp := &DescribeCostCategoryDefinitionResponse{
		CostCategory: cc,
	}

	writeCEResponse(w, resp)
}

// DeleteCostCategoryDefinition handles the DeleteCostCategoryDefinition API.
func (s *Service) DeleteCostCategoryDefinition(w http.ResponseWriter, r *http.Request) {
	var req DeleteCostCategoryDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCEError(w, errValidation, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.CostCategoryArn == "" {
		writeCEError(w, errValidation, "CostCategoryArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteCostCategoryDefinition(r.Context(), req.CostCategoryArn); err != nil {
		handleCEError(w, err)

		return
	}

	resp := &DeleteCostCategoryDefinitionResponse{
		CostCategoryArn: req.CostCategoryArn,
	}

	writeCEResponse(w, resp)
}

// ListCostCategoryDefinitions handles the ListCostCategoryDefinitions API.
func (s *Service) ListCostCategoryDefinitions(w http.ResponseWriter, r *http.Request) {
	var req ListCostCategoryDefinitionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is acceptable for list operations
		req = ListCostCategoryDefinitionsRequest{}
	}

	resp, err := s.storage.ListCostCategoryDefinitions(r.Context(), &req)
	if err != nil {
		handleCEError(w, err)

		return
	}

	writeCEResponse(w, resp)
}

// writeCEResponse writes a JSON response.
func writeCEResponse(w http.ResponseWriter, resp any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeCEError writes an error response.
func writeCEError(w http.ResponseWriter, code, message string, status int) {
	service.WriteJSONError(w, service.ContentTypeAmzJSON11, code, message, status)
}

// handleCEError handles Cost Explorer errors.
func handleCEError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		status := getErrorStatus(svcErr.Code)
		writeCEError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeCEError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errNotFound:
		return http.StatusNotFound
	case errDataUnavailable, errBillExpiration:
		return http.StatusBadRequest
	case errLimitExceeded, errServiceQuota, errBackfillInProgress:
		return http.StatusTooManyRequests
	case errInvalidNextToken:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}
