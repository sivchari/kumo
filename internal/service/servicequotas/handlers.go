package servicequotas

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
		// Service operations
		"ListServices": s.ListServices,
		// Quota operations
		"GetServiceQuota":             s.GetServiceQuota,
		"ListServiceQuotas":           s.ListServiceQuotas,
		"GetAWSDefaultServiceQuota":   s.GetAWSDefaultServiceQuota,
		"ListAWSDefaultServiceQuotas": s.ListAWSDefaultServiceQuotas,
		// Quota change request operations
		"RequestServiceQuotaIncrease":            s.RequestServiceQuotaIncrease,
		"GetRequestedServiceQuotaChange":         s.GetRequestedServiceQuotaChange,
		"ListRequestedServiceQuotaChangeHistory": s.ListRequestedServiceQuotaChangeHistory,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "ServiceQuotasV20190624.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "UnknownOperationException", "The operation "+action+" is not valid.", http.StatusBadRequest)
}

// ListServices handles the ListServices API.
func (s *Service) ListServices(w http.ResponseWriter, r *http.Request) {
	var req ListServicesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "IllegalArgumentException", "Invalid request body", http.StatusBadRequest)

		return
	}

	maxResults := int32(100)
	if req.MaxResults != nil && *req.MaxResults > 0 {
		maxResults = *req.MaxResults
	}

	services, nextToken, err := s.storage.ListServices(r.Context(), maxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	serviceOutputs := make([]ServiceInfoOutput, 0, len(services))
	for _, svc := range services {
		serviceOutputs = append(serviceOutputs, ServiceInfoOutput{
			ServiceCode: svc.ServiceCode,
			ServiceName: svc.ServiceName,
		})
	}

	resp := &ListServicesResponse{
		Services:  serviceOutputs,
		NextToken: nextToken,
	}

	writeResponse(w, resp)
}

// GetServiceQuota handles the GetServiceQuota API.
func (s *Service) GetServiceQuota(w http.ResponseWriter, r *http.Request) {
	var req GetServiceQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "IllegalArgumentException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ServiceCode == "" {
		writeError(w, "IllegalArgumentException", "ServiceCode is required", http.StatusBadRequest)

		return
	}

	if req.QuotaCode == "" {
		writeError(w, "IllegalArgumentException", "QuotaCode is required", http.StatusBadRequest)

		return
	}

	quota, err := s.storage.GetServiceQuota(r.Context(), req.ServiceCode, req.QuotaCode)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &GetServiceQuotaResponse{
		Quota: convertToServiceQuotaOutput(quota),
	}

	writeResponse(w, resp)
}

// ListServiceQuotas handles the ListServiceQuotas API.
func (s *Service) ListServiceQuotas(w http.ResponseWriter, r *http.Request) {
	var req ListServiceQuotasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "IllegalArgumentException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ServiceCode == "" {
		writeError(w, "IllegalArgumentException", "ServiceCode is required", http.StatusBadRequest)

		return
	}

	maxResults := int32(100)
	if req.MaxResults != nil && *req.MaxResults > 0 {
		maxResults = *req.MaxResults
	}

	quotas, nextToken, err := s.storage.ListServiceQuotas(r.Context(), req.ServiceCode, maxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	quotaOutputs := make([]ServiceQuotaOutput, 0, len(quotas))
	for _, quota := range quotas {
		quotaOutputs = append(quotaOutputs, *convertToServiceQuotaOutput(quota))
	}

	resp := &ListServiceQuotasResponse{
		Quotas:    quotaOutputs,
		NextToken: nextToken,
	}

	writeResponse(w, resp)
}

// GetAWSDefaultServiceQuota handles the GetAWSDefaultServiceQuota API.
func (s *Service) GetAWSDefaultServiceQuota(w http.ResponseWriter, r *http.Request) {
	var req GetAWSDefaultServiceQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "IllegalArgumentException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ServiceCode == "" {
		writeError(w, "IllegalArgumentException", "ServiceCode is required", http.StatusBadRequest)

		return
	}

	if req.QuotaCode == "" {
		writeError(w, "IllegalArgumentException", "QuotaCode is required", http.StatusBadRequest)

		return
	}

	quota, err := s.storage.GetAWSDefaultServiceQuota(r.Context(), req.ServiceCode, req.QuotaCode)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &GetAWSDefaultServiceQuotaResponse{
		Quota: convertToServiceQuotaOutput(quota),
	}

	writeResponse(w, resp)
}

// ListAWSDefaultServiceQuotas handles the ListAWSDefaultServiceQuotas API.
func (s *Service) ListAWSDefaultServiceQuotas(w http.ResponseWriter, r *http.Request) {
	var req ListAWSDefaultServiceQuotasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "IllegalArgumentException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ServiceCode == "" {
		writeError(w, "IllegalArgumentException", "ServiceCode is required", http.StatusBadRequest)

		return
	}

	maxResults := int32(100)
	if req.MaxResults != nil && *req.MaxResults > 0 {
		maxResults = *req.MaxResults
	}

	quotas, nextToken, err := s.storage.ListAWSDefaultServiceQuotas(r.Context(), req.ServiceCode, maxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	quotaOutputs := make([]ServiceQuotaOutput, 0, len(quotas))
	for _, quota := range quotas {
		quotaOutputs = append(quotaOutputs, *convertToServiceQuotaOutput(quota))
	}

	resp := &ListAWSDefaultServiceQuotasResponse{
		Quotas:    quotaOutputs,
		NextToken: nextToken,
	}

	writeResponse(w, resp)
}

// RequestServiceQuotaIncrease handles the RequestServiceQuotaIncrease API.
func (s *Service) RequestServiceQuotaIncrease(w http.ResponseWriter, r *http.Request) {
	var req RequestServiceQuotaIncreaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "IllegalArgumentException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ServiceCode == "" {
		writeError(w, "IllegalArgumentException", "ServiceCode is required", http.StatusBadRequest)

		return
	}

	if req.QuotaCode == "" {
		writeError(w, "IllegalArgumentException", "QuotaCode is required", http.StatusBadRequest)

		return
	}

	if req.DesiredValue <= 0 {
		writeError(w, "IllegalArgumentException", "DesiredValue must be greater than 0", http.StatusBadRequest)

		return
	}

	request, err := s.storage.RequestServiceQuotaIncrease(r.Context(), req.ServiceCode, req.QuotaCode, req.DesiredValue)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &RequestServiceQuotaIncreaseResponse{
		RequestedQuota: convertToRequestedQuotaOutput(request),
	}

	writeResponse(w, resp)
}

// GetRequestedServiceQuotaChange handles the GetRequestedServiceQuotaChange API.
func (s *Service) GetRequestedServiceQuotaChange(w http.ResponseWriter, r *http.Request) {
	var req GetRequestedServiceQuotaChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "IllegalArgumentException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.RequestID == "" {
		writeError(w, "IllegalArgumentException", "RequestId is required", http.StatusBadRequest)

		return
	}

	request, err := s.storage.GetRequestedServiceQuotaChange(r.Context(), req.RequestID)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &GetRequestedServiceQuotaChangeResponse{
		RequestedQuota: convertToRequestedQuotaOutput(request),
	}

	writeResponse(w, resp)
}

// ListRequestedServiceQuotaChangeHistory handles the ListRequestedServiceQuotaChangeHistory API.
func (s *Service) ListRequestedServiceQuotaChangeHistory(w http.ResponseWriter, r *http.Request) {
	var req ListRequestedServiceQuotaChangeHistoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "IllegalArgumentException", "Invalid request body", http.StatusBadRequest)

		return
	}

	maxResults := int32(100)
	if req.MaxResults != nil && *req.MaxResults > 0 {
		maxResults = *req.MaxResults
	}

	requests, nextToken, err := s.storage.ListRequestedServiceQuotaChangeHistory(
		r.Context(),
		req.ServiceCode,
		req.QuotaCode,
		req.Status,
		maxResults,
		req.NextToken,
	)
	if err != nil {
		handleError(w, err)

		return
	}

	requestOutputs := make([]RequestedServiceQuotaChangeOutput, 0, len(requests))
	for _, request := range requests {
		requestOutputs = append(requestOutputs, *convertToRequestedQuotaOutput(request))
	}

	resp := &ListRequestedServiceQuotaChangeHistoryResponse{
		RequestedQuotas: requestOutputs,
		NextToken:       nextToken,
	}

	writeResponse(w, resp)
}

// Helper functions.

// convertToServiceQuotaOutput converts a ServiceQuota to ServiceQuotaOutput.
func convertToServiceQuotaOutput(quota *ServiceQuota) *ServiceQuotaOutput {
	return &ServiceQuotaOutput{
		QuotaARN:            quota.QuotaARN,
		QuotaCode:           quota.QuotaCode,
		QuotaName:           quota.QuotaName,
		ServiceCode:         quota.ServiceCode,
		ServiceName:         quota.ServiceName,
		Value:               quota.Value,
		Unit:                quota.Unit,
		Adjustable:          quota.Adjustable,
		GlobalQuota:         quota.GlobalQuota,
		Description:         quota.Description,
		QuotaAppliedAtLevel: quota.QuotaAppliedAtLevel,
	}
}

// convertToRequestedQuotaOutput converts a QuotaChangeRequest to RequestedServiceQuotaChangeOutput.
func convertToRequestedQuotaOutput(request *QuotaChangeRequest) *RequestedServiceQuotaChangeOutput {
	return &RequestedServiceQuotaChangeOutput{
		ID:                    request.ID,
		ServiceCode:           request.ServiceCode,
		ServiceName:           request.ServiceName,
		QuotaCode:             request.QuotaCode,
		QuotaName:             request.QuotaName,
		DesiredValue:          request.DesiredValue,
		Status:                request.Status,
		Created:               float64(request.Created.Unix()),
		LastUpdated:           float64(request.LastUpdated.Unix()),
		CaseID:                request.CaseID,
		Requester:             request.Requester,
		QuotaARN:              request.QuotaARN,
		Unit:                  request.Unit,
		GlobalQuota:           request.GlobalQuota,
		QuotaRequestedAtLevel: request.QuotaRequestedAtLevel,
	}
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
	var sqErr *Error
	if errors.As(err, &sqErr) {
		status := getErrorStatus(sqErr.Code)
		writeError(w, sqErr.Code, sqErr.Message, status)

		return
	}

	writeError(w, "ServiceException", err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errNoSuchResourceException:
		return http.StatusNotFound
	case errIllegalArgumentException:
		return http.StatusBadRequest
	case errTooManyRequestsException:
		return http.StatusTooManyRequests
	default:
		return http.StatusBadRequest
	}
}
