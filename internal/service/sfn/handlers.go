package sfn

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// handlerFunc is a type alias for handler functions.
type handlerFunc func(http.ResponseWriter, *http.Request)

// getActionHandlers returns a map of action names to handler functions.
func (s *Service) getActionHandlers() map[string]handlerFunc {
	return map[string]handlerFunc{
		"CreateStateMachine":   s.CreateStateMachine,
		"DeleteStateMachine":   s.DeleteStateMachine,
		"DescribeStateMachine": s.DescribeStateMachine,
		"ListStateMachines":    s.ListStateMachines,
		"StartExecution":       s.StartExecution,
		"StopExecution":        s.StopExecution,
		"DescribeExecution":    s.DescribeExecution,
		"ListExecutions":       s.ListExecutions,
		"GetExecutionHistory":  s.GetExecutionHistory,
		"SendTaskSuccess":      s.SendTaskSuccess,
		"SendTaskFailure":      s.SendTaskFailure,
		"SendTaskHeartbeat":    s.SendTaskHeartbeat,
		// Tag + validation stubs — see definition_stub.go.
		// Required by terraform-provider-aws plan/apply of aws_sfn_state_machine.
		"ValidateStateMachineDefinition": s.ValidateStateMachineDefinition,
		"ListStateMachineVersions":       s.ListStateMachineVersions,
		"ListStateMachineAliases":        s.ListStateMachineAliases,
		"ListTagsForResource":            s.ListTagsForResource,
		"TagResource":                    s.TagResource,
		"UntagResource":                  s.UntagResource,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AWSStepFunctions.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "InvalidAction", "The action "+action+" is not valid for this endpoint.", http.StatusBadRequest)
}

// CreateStateMachine handles the CreateStateMachine API.
func (s *Service) CreateStateMachine(w http.ResponseWriter, r *http.Request) {
	var req CreateStateMachineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	sm, err := s.storage.CreateStateMachine(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateStateMachineResponse{
		StateMachineArn: sm.StateMachineArn,
		CreationDate:    float64(sm.CreationDate.Unix()),
	}

	writeResponse(w, resp)
}

// DeleteStateMachine handles the DeleteStateMachine API.
func (s *Service) DeleteStateMachine(w http.ResponseWriter, r *http.Request) {
	var req DeleteStateMachineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteStateMachine(r.Context(), req.StateMachineArn); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteStateMachineResponse{})
}

// DescribeStateMachine handles the DescribeStateMachine API.
func (s *Service) DescribeStateMachine(w http.ResponseWriter, r *http.Request) {
	var req DescribeStateMachineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	sm, err := s.storage.DescribeStateMachine(r.Context(), req.StateMachineArn)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DescribeStateMachineResponse{
		StateMachineArn:      sm.StateMachineArn,
		Name:                 sm.Name,
		Status:               string(sm.Status),
		Definition:           sm.Definition,
		RoleArn:              sm.RoleArn,
		Type:                 string(sm.Type),
		CreationDate:         float64(sm.CreationDate.Unix()),
		LoggingConfiguration: sm.LoggingConfiguration,
		TracingConfiguration: sm.TracingConfiguration,
		Label:                sm.Label,
		RevisionID:           sm.RevisionID,
		Description:          sm.Description,
	}

	writeResponse(w, resp)
}

// ListStateMachines handles the ListStateMachines API.
func (s *Service) ListStateMachines(w http.ResponseWriter, r *http.Request) {
	var req ListStateMachinesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	stateMachines, nextToken, err := s.storage.ListStateMachines(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]StateMachineListItem, len(stateMachines))
	for i, sm := range stateMachines {
		items[i] = StateMachineListItem{
			StateMachineArn: sm.StateMachineArn,
			Name:            sm.Name,
			Type:            string(sm.Type),
			CreationDate:    float64(sm.CreationDate.Unix()),
		}
	}

	resp := &ListStateMachinesResponse{
		StateMachines: items,
		NextToken:     nextToken,
	}

	writeResponse(w, resp)
}

// StartExecution handles the StartExecution API.
func (s *Service) StartExecution(w http.ResponseWriter, r *http.Request) {
	var req StartExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	exec, err := s.storage.StartExecution(r.Context(), req.StateMachineArn, req.Name, req.Input, req.TraceHeader)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &StartExecutionResponse{
		ExecutionArn: exec.ExecutionArn,
		StartDate:    float64(exec.StartDate.Unix()),
	}

	writeResponse(w, resp)
}

// StopExecution handles the StopExecution API.
func (s *Service) StopExecution(w http.ResponseWriter, r *http.Request) {
	var req StopExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	exec, err := s.storage.StopExecution(r.Context(), req.ExecutionArn, req.Error, req.Cause)
	if err != nil {
		handleError(w, err)

		return
	}

	var stopDate float64
	if exec.StopDate != nil {
		stopDate = float64(exec.StopDate.Unix())
	}

	resp := &StopExecutionResponse{
		StopDate: stopDate,
	}

	writeResponse(w, resp)
}

// DescribeExecution handles the DescribeExecution API.
func (s *Service) DescribeExecution(w http.ResponseWriter, r *http.Request) {
	var req DescribeExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	exec, err := s.storage.DescribeExecution(r.Context(), req.ExecutionArn)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DescribeExecutionResponse{
		ExecutionArn:    exec.ExecutionArn,
		StateMachineArn: exec.StateMachineArn,
		Name:            exec.Name,
		Status:          string(exec.Status),
		StartDate:       float64(exec.StartDate.Unix()),
		Input:           exec.Input,
		InputDetails:    exec.InputDetails,
		Output:          exec.Output,
		OutputDetails:   exec.OutputDetails,
		Error:           exec.Error,
		Cause:           exec.Cause,
		TraceHeader:     exec.TraceHeader,
		RedriveCount:    exec.RedriveCount,
		RedriveStatus:   exec.RedriveStatus,
	}

	if exec.StopDate != nil {
		resp.StopDate = float64(exec.StopDate.Unix())
	}

	if exec.RedriveDate != nil {
		resp.RedriveDate = float64(exec.RedriveDate.Unix())
	}

	writeResponse(w, resp)
}

// ListExecutions handles the ListExecutions API.
func (s *Service) ListExecutions(w http.ResponseWriter, r *http.Request) {
	var req ListExecutionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	executions, nextToken, err := s.storage.ListExecutions(r.Context(), req.StateMachineArn, req.StatusFilter, req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]ExecutionListItem, len(executions))

	for i, exec := range executions {
		item := ExecutionListItem{
			ExecutionArn:    exec.ExecutionArn,
			StateMachineArn: exec.StateMachineArn,
			Name:            exec.Name,
			Status:          string(exec.Status),
			StartDate:       float64(exec.StartDate.Unix()),
			RedriveCount:    exec.RedriveCount,
		}

		if exec.StopDate != nil {
			item.StopDate = float64(exec.StopDate.Unix())
		}

		if exec.RedriveDate != nil {
			item.RedriveDate = float64(exec.RedriveDate.Unix())
		}

		items[i] = item
	}

	resp := &ListExecutionsResponse{
		Executions: items,
		NextToken:  nextToken,
	}

	writeResponse(w, resp)
}

// GetExecutionHistory handles the GetExecutionHistory API.
func (s *Service) GetExecutionHistory(w http.ResponseWriter, r *http.Request) {
	var req GetExecutionHistoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	events, nextToken, err := s.storage.GetExecutionHistory(r.Context(), req.ExecutionArn, req.MaxResults, req.NextToken, req.ReverseOrder)
	if err != nil {
		handleError(w, err)

		return
	}

	eventOutputs := make([]HistoryEventOutput, len(events))
	for i, event := range events {
		eventOutputs[i] = HistoryEventOutput{
			Timestamp:                      float64(event.Timestamp.Unix()),
			Type:                           string(event.Type),
			ID:                             event.ID,
			PreviousEventID:                event.PreviousEventID,
			ExecutionStartedEventDetails:   event.ExecutionStartedEventDetails,
			ExecutionSucceededEventDetails: event.ExecutionSucceededEventDetails,
			ExecutionFailedEventDetails:    event.ExecutionFailedEventDetails,
			ExecutionAbortedEventDetails:   event.ExecutionAbortedEventDetails,
			ExecutionTimedOutEventDetails:  event.ExecutionTimedOutEventDetails,
		}
	}

	resp := &GetExecutionHistoryResponse{
		Events:    eventOutputs,
		NextToken: nextToken,
	}

	writeResponse(w, resp)
}

// writeResponse writes a JSON response.
func writeResponse(w http.ResponseWriter, resp any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&ErrorResponse{
		Type:    code,
		Message: message,
	})
}

// handleError handles service errors.
func handleError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		status := getErrorStatus(svcErr.Code)
		writeError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errStateMachineDoesNotExist, errExecutionDoesNotExist:
		return http.StatusNotFound
	case errStateMachineAlreadyExists, errExecutionAlreadyExists:
		return http.StatusConflict
	case errInvalidArn, errInvalidDefinition:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}

// SendTaskSuccess handles the SendTaskSuccess API.
// Since kumo does not manage task tokens, it always returns InvalidToken.
func (s *Service) SendTaskSuccess(w http.ResponseWriter, _ *http.Request) {
	writeError(w, "InvalidToken", "Invalid token", http.StatusBadRequest)
}

// SendTaskFailure handles the SendTaskFailure API.
// Since kumo does not manage task tokens, it always returns InvalidToken.
func (s *Service) SendTaskFailure(w http.ResponseWriter, _ *http.Request) {
	writeError(w, "InvalidToken", "Invalid token", http.StatusBadRequest)
}

// SendTaskHeartbeat handles the SendTaskHeartbeat API.
// Since kumo does not manage task tokens, it always returns InvalidToken.
func (s *Service) SendTaskHeartbeat(w http.ResponseWriter, _ *http.Request) {
	writeError(w, "InvalidToken", "Invalid token", http.StatusBadRequest)
}
