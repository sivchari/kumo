package configservice

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
		"PutConfigurationRecorder":         s.PutConfigurationRecorder,
		"DeleteConfigurationRecorder":      s.DeleteConfigurationRecorder,
		"DescribeConfigurationRecorders":   s.DescribeConfigurationRecorders,
		"StartConfigurationRecorder":       s.StartConfigurationRecorder,
		"StopConfigurationRecorder":        s.StopConfigurationRecorder,
		"PutConfigRule":                    s.PutConfigRule,
		"DeleteConfigRule":                 s.DeleteConfigRule,
		"DescribeConfigRules":              s.DescribeConfigRules,
		"GetComplianceDetailsByConfigRule": s.GetComplianceDetailsByConfigRule,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "StarlingDoveService.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "UnknownOperationException", "The operation "+action+" is not valid.", http.StatusBadRequest)
}

// PutConfigurationRecorder handles the PutConfigurationRecorder API.
func (s *Service) PutConfigurationRecorder(w http.ResponseWriter, r *http.Request) {
	var req PutConfigurationRecorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutConfigurationRecorder(r.Context(), &req); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &PutConfigurationRecorderResponse{})
}

// DeleteConfigurationRecorder handles the DeleteConfigurationRecorder API.
func (s *Service) DeleteConfigurationRecorder(w http.ResponseWriter, r *http.Request) {
	var req DeleteConfigurationRecorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ConfigurationRecorderName == "" {
		writeError(w, "ValidationException", "Configuration recorder name is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteConfigurationRecorder(r.Context(), req.ConfigurationRecorderName); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteConfigurationRecorderResponse{})
}

// DescribeConfigurationRecorders handles the DescribeConfigurationRecorders API.
func (s *Service) DescribeConfigurationRecorders(w http.ResponseWriter, r *http.Request) {
	var req DescribeConfigurationRecordersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	recorders, err := s.storage.DescribeConfigurationRecorders(r.Context(), req.ConfigurationRecorderNames)
	if err != nil {
		handleError(w, err)

		return
	}

	recorderOutputs := make([]ConfigurationRecorderOutput, 0, len(recorders))
	for _, recorder := range recorders {
		recorderOutputs = append(recorderOutputs, *convertToConfigurationRecorderOutput(recorder))
	}

	resp := &DescribeConfigurationRecordersResponse{
		ConfigurationRecorders: recorderOutputs,
	}

	writeResponse(w, resp)
}

// StartConfigurationRecorder handles the StartConfigurationRecorder API.
func (s *Service) StartConfigurationRecorder(w http.ResponseWriter, r *http.Request) {
	var req StartConfigurationRecorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ConfigurationRecorderName == "" {
		writeError(w, "ValidationException", "Configuration recorder name is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.StartConfigurationRecorder(r.Context(), req.ConfigurationRecorderName); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &StartConfigurationRecorderResponse{})
}

// StopConfigurationRecorder handles the StopConfigurationRecorder API.
func (s *Service) StopConfigurationRecorder(w http.ResponseWriter, r *http.Request) {
	var req StopConfigurationRecorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ConfigurationRecorderName == "" {
		writeError(w, "ValidationException", "Configuration recorder name is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.StopConfigurationRecorder(r.Context(), req.ConfigurationRecorderName); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &StopConfigurationRecorderResponse{})
}

// PutConfigRule handles the PutConfigRule API.
func (s *Service) PutConfigRule(w http.ResponseWriter, r *http.Request) {
	var req PutConfigRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if _, err := s.storage.PutConfigRule(r.Context(), &req); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &PutConfigRuleResponse{})
}

// DeleteConfigRule handles the DeleteConfigRule API.
func (s *Service) DeleteConfigRule(w http.ResponseWriter, r *http.Request) {
	var req DeleteConfigRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ConfigRuleName == "" {
		writeError(w, "ValidationException", "Config rule name is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteConfigRule(r.Context(), req.ConfigRuleName); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteConfigRuleResponse{})
}

// DescribeConfigRules handles the DescribeConfigRules API.
func (s *Service) DescribeConfigRules(w http.ResponseWriter, r *http.Request) {
	var req DescribeConfigRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	rules, err := s.storage.DescribeConfigRules(r.Context(), req.ConfigRuleNames)
	if err != nil {
		handleError(w, err)

		return
	}

	ruleOutputs := make([]ConfigRuleOutput, 0, len(rules))
	for _, rule := range rules {
		ruleOutputs = append(ruleOutputs, *convertToConfigRuleOutput(rule))
	}

	resp := &DescribeConfigRulesResponse{
		ConfigRules: ruleOutputs,
	}

	writeResponse(w, resp)
}

// GetComplianceDetailsByConfigRule handles the GetComplianceDetailsByConfigRule API.
func (s *Service) GetComplianceDetailsByConfigRule(w http.ResponseWriter, r *http.Request) {
	var req GetComplianceDetailsByConfigRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.ConfigRuleName == "" {
		writeError(w, "ValidationException", "Config rule name is required", http.StatusBadRequest)

		return
	}

	results, nextToken, err := s.storage.GetComplianceDetailsByConfigRule(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resultOutputs := make([]EvaluationResultOutput, 0, len(results))
	for _, result := range results {
		resultOutputs = append(resultOutputs, EvaluationResultOutput{
			ComplianceType:     result.ComplianceType,
			ResultRecordedTime: float64(result.ResultRecordedTime.Unix()),
		})
	}

	resp := &GetComplianceDetailsByConfigRuleResponse{
		EvaluationResults: resultOutputs,
		NextToken:         nextToken,
	}

	writeResponse(w, resp)
}

// Helper functions.

// convertToConfigurationRecorderOutput converts a ConfigurationRecorder to ConfigurationRecorderOutput.
func convertToConfigurationRecorderOutput(recorder *ConfigurationRecorder) *ConfigurationRecorderOutput {
	output := &ConfigurationRecorderOutput{
		Name:    recorder.Name,
		RoleARN: recorder.RoleARN,
	}

	if recorder.RecordingGroup != nil {
		output.RecordingGroup = &RecordingGroupOutput{
			AllSupported:               recorder.RecordingGroup.AllSupported,
			IncludeGlobalResourceTypes: recorder.RecordingGroup.IncludeGlobalResourceTypes,
			ResourceTypes:              recorder.RecordingGroup.ResourceTypes,
		}
	}

	return output
}

// convertToConfigRuleOutput converts a ConfigRule to ConfigRuleOutput.
func convertToConfigRuleOutput(rule *ConfigRule) *ConfigRuleOutput {
	output := &ConfigRuleOutput{
		ConfigRuleName:  rule.ConfigRuleName,
		ConfigRuleARN:   rule.ConfigRuleARN,
		ConfigRuleID:    rule.ConfigRuleID,
		Description:     rule.Description,
		ConfigRuleState: rule.ConfigRuleState,
	}

	if rule.Source != nil {
		output.Source = &SourceOutput{
			Owner:            rule.Source.Owner,
			SourceIdentifier: rule.Source.SourceIdentifier,
		}
	}

	if rule.Scope != nil {
		output.Scope = &ScopeOutput{
			ComplianceResourceTypes: rule.Scope.ComplianceResourceTypes,
		}
	}

	return output
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
	var cfgErr *Error
	if errors.As(err, &cfgErr) {
		status := getErrorStatus(cfgErr.Code)
		writeError(w, cfgErr.Code, cfgErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errNoSuchConfigurationRecorder, errNoSuchConfigRule:
		return http.StatusNotFound
	case errMaxNumberOfConfigurationRecordersExceeded:
		return http.StatusConflict
	case errInvalidParameterValue:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}
