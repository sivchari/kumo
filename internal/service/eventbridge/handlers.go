package eventbridge

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
		"CreateEventBus":         s.CreateEventBus,
		"DeleteEventBus":         s.DeleteEventBus,
		"DescribeEventBus":       s.DescribeEventBus,
		"ListEventBuses":         s.ListEventBuses,
		"PutRule":                s.PutRule,
		"DeleteRule":             s.DeleteRule,
		"DescribeRule":           s.DescribeRule,
		"ListRules":              s.ListRules,
		"PutTargets":             s.PutTargets,
		"RemoveTargets":          s.RemoveTargets,
		"ListTargetsByRule":      s.ListTargetsByRule,
		"PutEvents":              s.PutEvents,
		"CreateConnection":       s.CreateConnection,
		"DescribeConnection":     s.DescribeConnection,
		"DeleteConnection":       s.DeleteConnection,
		"CreateApiDestination":   s.CreateAPIDestination,
		"DescribeApiDestination": s.DescribeAPIDestination,
		"DeleteApiDestination":   s.DeleteAPIDestination,
		"ListTagsForResource":    s.ListTagsForResource,
		"TagResource":            s.TagResource,
		"UntagResource":          s.UntagResource,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AWSEvents.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "InvalidAction", "The action "+action+" is not valid for this endpoint.", http.StatusBadRequest)
}

// CreateEventBus handles the CreateEventBus API.
func (s *Service) CreateEventBus(w http.ResponseWriter, r *http.Request) {
	var req CreateEventBusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	eventBus, err := s.storage.CreateEventBus(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateEventBusResponse{
		EventBusArn: eventBus.Arn,
	}

	writeResponse(w, resp)
}

// DeleteEventBus handles the DeleteEventBus API.
func (s *Service) DeleteEventBus(w http.ResponseWriter, r *http.Request) {
	var req DeleteEventBusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteEventBus(r.Context(), req.Name); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteEventBusResponse{})
}

// DescribeEventBus handles the DescribeEventBus API.
func (s *Service) DescribeEventBus(w http.ResponseWriter, r *http.Request) {
	var req DescribeEventBusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	eventBus, err := s.storage.DescribeEventBus(r.Context(), req.Name)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DescribeEventBusResponse{
		Name:        eventBus.Name,
		Arn:         eventBus.Arn,
		Description: eventBus.Description,
	}

	writeResponse(w, resp)
}

// ListEventBuses handles the ListEventBuses API.
func (s *Service) ListEventBuses(w http.ResponseWriter, r *http.Request) {
	var req ListEventBusesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	eventBuses, nextToken, err := s.storage.ListEventBuses(r.Context(), req.NamePrefix, req.Limit, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	outputs := make([]EventBusOutput, len(eventBuses))

	for i, eb := range eventBuses {
		outputs[i] = EventBusOutput{
			Name:        eb.Name,
			Arn:         eb.Arn,
			Description: eb.Description,
			ManagedBy:   eb.ManagedBy,
		}
	}

	resp := &ListEventBusesResponse{
		EventBuses: outputs,
		NextToken:  nextToken,
	}

	writeResponse(w, resp)
}

// PutRule handles the PutRule API.
func (s *Service) PutRule(w http.ResponseWriter, r *http.Request) {
	var req PutRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	rule, err := s.storage.PutRule(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &PutRuleResponse{
		RuleArn: rule.Arn,
	}

	writeResponse(w, resp)
}

// DeleteRule handles the DeleteRule API.
func (s *Service) DeleteRule(w http.ResponseWriter, r *http.Request) {
	var req DeleteRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteRule(r.Context(), req.EventBusName, req.Name, req.Force); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteRuleResponse{})
}

// DescribeRule handles the DescribeRule API.
func (s *Service) DescribeRule(w http.ResponseWriter, r *http.Request) {
	var req DescribeRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	rule, err := s.storage.DescribeRule(r.Context(), req.EventBusName, req.Name)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DescribeRuleResponse{
		Name:               rule.Name,
		Arn:                rule.Arn,
		EventBusName:       rule.EventBusName,
		EventPattern:       rule.EventPattern,
		ScheduleExpression: rule.ScheduleExpression,
		State:              string(rule.State),
		Description:        rule.Description,
		RoleArn:            rule.RoleArn,
	}

	writeResponse(w, resp)
}

// ListRules handles the ListRules API.
func (s *Service) ListRules(w http.ResponseWriter, r *http.Request) {
	var req ListRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	rules, nextToken, err := s.storage.ListRules(r.Context(), req.EventBusName, req.NamePrefix, req.Limit, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	outputs := make([]RuleOutput, len(rules))

	for i, rule := range rules {
		outputs[i] = RuleOutput{
			Name:               rule.Name,
			Arn:                rule.Arn,
			EventBusName:       rule.EventBusName,
			EventPattern:       rule.EventPattern,
			ScheduleExpression: rule.ScheduleExpression,
			State:              string(rule.State),
			Description:        rule.Description,
			RoleArn:            rule.RoleArn,
		}
	}

	resp := &ListRulesResponse{
		Rules:     outputs,
		NextToken: nextToken,
	}

	writeResponse(w, resp)
}

// PutTargets handles the PutTargets API.
func (s *Service) PutTargets(w http.ResponseWriter, r *http.Request) {
	var req PutTargetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	failedEntries, err := s.storage.PutTargets(r.Context(), req.EventBusName, req.Rule, req.Targets)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &PutTargetsResponse{
		FailedEntryCount: int32(len(failedEntries)), //nolint:gosec // slice length bounded by API limits
		FailedEntries:    failedEntries,
	}

	writeResponse(w, resp)
}

// RemoveTargets handles the RemoveTargets API.
func (s *Service) RemoveTargets(w http.ResponseWriter, r *http.Request) {
	var req RemoveTargetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	failedEntries, err := s.storage.RemoveTargets(r.Context(), req.EventBusName, req.Rule, req.IDs, req.Force)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &RemoveTargetsResponse{
		FailedEntryCount: int32(len(failedEntries)), //nolint:gosec // slice length bounded by API limits
		FailedEntries:    failedEntries,
	}

	writeResponse(w, resp)
}

// ListTargetsByRule handles the ListTargetsByRule API.
func (s *Service) ListTargetsByRule(w http.ResponseWriter, r *http.Request) {
	var req ListTargetsByRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	targets, nextToken, err := s.storage.ListTargetsByRule(r.Context(), req.EventBusName, req.Rule, req.Limit, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	outputs := make([]TargetOutput, len(targets))

	for i, target := range targets {
		outputs[i] = TargetOutput{
			ID:             target.ID,
			Arn:            target.Arn,
			RoleArn:        target.RoleArn,
			Input:          target.Input,
			InputPath:      target.InputPath,
			HTTPParameters: target.HTTPParameters,
		}

		if target.InputTransformer != nil {
			outputs[i].InputTransformer = &InputTransformerOutput{
				InputPathsMap: target.InputTransformer.InputPathsMap,
				InputTemplate: target.InputTransformer.InputTemplate,
			}
		}
	}

	resp := &ListTargetsByRuleResponse{
		Targets:   outputs,
		NextToken: nextToken,
	}

	writeResponse(w, resp)
}

// PutEvents handles the PutEvents API.
func (s *Service) PutEvents(w http.ResponseWriter, r *http.Request) {
	var req PutEventsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	entries, err := s.storage.PutEvents(r.Context(), req.Entries)
	if err != nil {
		handleError(w, err)

		return
	}

	var failedCount int32

	for _, entry := range entries {
		if entry.ErrorCode != "" {
			failedCount++
		}
	}

	resp := &PutEventsResponse{
		FailedEntryCount: failedCount,
		Entries:          entries,
	}

	writeResponse(w, resp)
}

// CreateConnection handles the CreateConnection API.
func (s *Service) CreateConnection(w http.ResponseWriter, r *http.Request) {
	var req CreateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	conn, err := s.storage.CreateConnection(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &CreateConnectionResponse{
		ConnectionArn:    conn.Arn,
		ConnectionState:  conn.ConnectionState,
		CreationTime:     float64(conn.CreationTime.Unix()),
		LastModifiedTime: float64(conn.LastModifiedTime.Unix()),
	})
}

// DescribeConnection handles the DescribeConnection API.
func (s *Service) DescribeConnection(w http.ResponseWriter, r *http.Request) {
	var req DescribeConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	conn, err := s.storage.DescribeConnection(r.Context(), req.Name)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DescribeConnectionResponse{
		Name:               conn.Name,
		ConnectionArn:      conn.Arn,
		ConnectionState:    conn.ConnectionState,
		AuthorizationType:  conn.AuthorizationType,
		CreationTime:       float64(conn.CreationTime.Unix()),
		LastModifiedTime:   float64(conn.LastModifiedTime.Unix()),
		LastAuthorizedTime: float64(conn.LastAuthorizedTime.Unix()),
	})
}

// DeleteConnection handles the DeleteConnection API.
func (s *Service) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	var req DeleteConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	conn, err := s.storage.DeleteConnection(r.Context(), req.Name)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteConnectionResponse{
		ConnectionArn:   conn.Arn,
		ConnectionState: "DELETING",
	})
}

// CreateAPIDestination handles the CreateApiDestination API.
func (s *Service) CreateAPIDestination(w http.ResponseWriter, r *http.Request) {
	var req CreateAPIDestinationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	dest, err := s.storage.CreateAPIDestination(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &CreateAPIDestinationResponse{
		APIDestinationArn:   dest.Arn,
		APIDestinationState: dest.APIDestinationState,
		CreationTime:        float64(dest.CreationTime.Unix()),
		LastModifiedTime:    float64(dest.LastModifiedTime.Unix()),
	})
}

// DescribeAPIDestination handles the DescribeApiDestination API.
func (s *Service) DescribeAPIDestination(w http.ResponseWriter, r *http.Request) {
	var req DescribeAPIDestinationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	dest, err := s.storage.DescribeAPIDestination(r.Context(), req.Name)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DescribeAPIDestinationResponse{
		Name:                         dest.Name,
		APIDestinationArn:            dest.Arn,
		ConnectionArn:                dest.ConnectionArn,
		InvocationEndpoint:           dest.InvocationEndpoint,
		HTTPMethod:                   dest.HTTPMethod,
		InvocationRateLimitPerSecond: dest.InvocationRateLimitPerSecond,
		APIDestinationState:          dest.APIDestinationState,
		CreationTime:                 float64(dest.CreationTime.Unix()),
		LastModifiedTime:             float64(dest.LastModifiedTime.Unix()),
	})
}

// DeleteAPIDestination handles the DeleteApiDestination API.
func (s *Service) DeleteAPIDestination(w http.ResponseWriter, r *http.Request) {
	var req DeleteAPIDestinationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteAPIDestination(r.Context(), req.Name); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteAPIDestinationResponse{})
}

// ListTagsForResource handles the ListTagsForResource API.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	var req ListTagsForResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListTagsForResource(r.Context(), req.ResourceARN)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &ListTagsForResourceResponse{Tags: tags})
}

// TagResource handles the TagResource API.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	var req TagResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(r.Context(), req.ResourceARN, req.Tags); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &TagResourceResponse{})
}

// UntagResource handles the UntagResource API.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	var req UntagResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.UntagResource(r.Context(), req.ResourceARN, req.TagKeys); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &UntagResourceResponse{})
}

// GetDeliveredEvents returns events that were matched against rules and delivered to targets.
// This is a kumo-specific endpoint for test verification.
func (s *Service) GetDeliveredEvents(w http.ResponseWriter, r *http.Request) {
	events := s.storage.GetDeliveredEvents(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(events)
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
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
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
	case "ResourceNotFoundException":
		return http.StatusNotFound
	case "ResourceAlreadyExistsException":
		return http.StatusConflict
	case "ValidationException":
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}
