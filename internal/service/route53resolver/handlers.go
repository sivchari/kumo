package route53resolver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

const (
	errInternalServiceError = "InternalServiceError"
	errInvalidAction        = "InvalidAction"
)

// CreateResolverEndpoint handles the CreateResolverEndpoint action.
func (s *Service) CreateResolverEndpoint(w http.ResponseWriter, r *http.Request) {
	var req CreateResolverEndpointRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.CreatorRequestID == "" {
		writeResolverError(w, errInvalidParameter, "CreatorRequestId is required", http.StatusBadRequest)

		return
	}

	if req.Direction == "" {
		writeResolverError(w, errInvalidParameter, "Direction is required", http.StatusBadRequest)

		return
	}

	endpoint, err := s.storage.CreateResolverEndpoint(r.Context(), &req)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	writeJSONResponse(w, CreateResolverEndpointResponse{
		ResolverEndpoint: toResolverEndpointOutput(endpoint),
	})
}

// GetResolverEndpoint handles the GetResolverEndpoint action.
func (s *Service) GetResolverEndpoint(w http.ResponseWriter, r *http.Request) {
	var req GetResolverEndpointRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResolverEndpointID == "" {
		writeResolverError(w, errInvalidParameter, "ResolverEndpointId is required", http.StatusBadRequest)

		return
	}

	endpoint, err := s.storage.GetResolverEndpoint(r.Context(), req.ResolverEndpointID)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	writeJSONResponse(w, GetResolverEndpointResponse{
		ResolverEndpoint: toResolverEndpointOutput(endpoint),
	})
}

// DeleteResolverEndpoint handles the DeleteResolverEndpoint action.
func (s *Service) DeleteResolverEndpoint(w http.ResponseWriter, r *http.Request) {
	var req DeleteResolverEndpointRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResolverEndpointID == "" {
		writeResolverError(w, errInvalidParameter, "ResolverEndpointId is required", http.StatusBadRequest)

		return
	}

	endpoint, err := s.storage.DeleteResolverEndpoint(r.Context(), req.ResolverEndpointID)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	writeJSONResponse(w, DeleteResolverEndpointResponse{
		ResolverEndpoint: toResolverEndpointOutput(endpoint),
	})
}

// ListResolverEndpoints handles the ListResolverEndpoints action.
func (s *Service) ListResolverEndpoints(w http.ResponseWriter, r *http.Request) {
	var req ListResolverEndpointsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	endpoints, nextToken, err := s.storage.ListResolverEndpoints(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	outputs := make([]*ResolverEndpointOutput, 0, len(endpoints))
	for _, e := range endpoints {
		outputs = append(outputs, toResolverEndpointOutput(e))
	}

	writeJSONResponse(w, ListResolverEndpointsResponse{
		ResolverEndpoints: outputs,
		MaxResults:        req.MaxResults,
		NextToken:         nextToken,
	})
}

// CreateResolverRule handles the CreateResolverRule action.
func (s *Service) CreateResolverRule(w http.ResponseWriter, r *http.Request) {
	var req CreateResolverRuleRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.CreatorRequestID == "" {
		writeResolverError(w, errInvalidParameter, "CreatorRequestId is required", http.StatusBadRequest)

		return
	}

	if req.DomainName == "" {
		writeResolverError(w, errInvalidParameter, "DomainName is required", http.StatusBadRequest)

		return
	}

	if req.RuleType == "" {
		writeResolverError(w, errInvalidParameter, "RuleType is required", http.StatusBadRequest)

		return
	}

	rule, err := s.storage.CreateResolverRule(r.Context(), &req)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	writeJSONResponse(w, CreateResolverRuleResponse{
		ResolverRule: toResolverRuleOutput(rule),
	})
}

// GetResolverRule handles the GetResolverRule action.
func (s *Service) GetResolverRule(w http.ResponseWriter, r *http.Request) {
	var req GetResolverRuleRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResolverRuleID == "" {
		writeResolverError(w, errInvalidParameter, "ResolverRuleId is required", http.StatusBadRequest)

		return
	}

	rule, err := s.storage.GetResolverRule(r.Context(), req.ResolverRuleID)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	writeJSONResponse(w, GetResolverRuleResponse{
		ResolverRule: toResolverRuleOutput(rule),
	})
}

// DeleteResolverRule handles the DeleteResolverRule action.
func (s *Service) DeleteResolverRule(w http.ResponseWriter, r *http.Request) {
	var req DeleteResolverRuleRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResolverRuleID == "" {
		writeResolverError(w, errInvalidParameter, "ResolverRuleId is required", http.StatusBadRequest)

		return
	}

	rule, err := s.storage.DeleteResolverRule(r.Context(), req.ResolverRuleID)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	writeJSONResponse(w, DeleteResolverRuleResponse{
		ResolverRule: toResolverRuleOutput(rule),
	})
}

// ListResolverRules handles the ListResolverRules action.
func (s *Service) ListResolverRules(w http.ResponseWriter, r *http.Request) {
	var req ListResolverRulesRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	rules, nextToken, err := s.storage.ListResolverRules(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	outputs := make([]*ResolverRuleOutput, 0, len(rules))
	for _, rule := range rules {
		outputs = append(outputs, toResolverRuleOutput(rule))
	}

	writeJSONResponse(w, ListResolverRulesResponse{
		ResolverRules: outputs,
		MaxResults:    req.MaxResults,
		NextToken:     nextToken,
	})
}

// AssociateResolverRule handles the AssociateResolverRule action.
func (s *Service) AssociateResolverRule(w http.ResponseWriter, r *http.Request) {
	var req AssociateResolverRuleRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResolverRuleID == "" {
		writeResolverError(w, errInvalidParameter, "ResolverRuleId is required", http.StatusBadRequest)

		return
	}

	if req.VPCID == "" {
		writeResolverError(w, errInvalidParameter, "VPCId is required", http.StatusBadRequest)

		return
	}

	assoc, err := s.storage.AssociateResolverRule(r.Context(), &req)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	writeJSONResponse(w, AssociateResolverRuleResponse{
		ResolverRuleAssociation: toResolverRuleAssociationOutput(assoc),
	})
}

// DisassociateResolverRule handles the DisassociateResolverRule action.
func (s *Service) DisassociateResolverRule(w http.ResponseWriter, r *http.Request) {
	var req DisassociateResolverRuleRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResolverRuleID == "" {
		writeResolverError(w, errInvalidParameter, "ResolverRuleId is required", http.StatusBadRequest)

		return
	}

	if req.VPCID == "" {
		writeResolverError(w, errInvalidParameter, "VPCId is required", http.StatusBadRequest)

		return
	}

	assoc, err := s.storage.DisassociateResolverRule(r.Context(), req.ResolverRuleID, req.VPCID)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	writeJSONResponse(w, DisassociateResolverRuleResponse{
		ResolverRuleAssociation: toResolverRuleAssociationOutput(assoc),
	})
}

// ListResolverRuleAssociations handles the ListResolverRuleAssociations action.
func (s *Service) ListResolverRuleAssociations(w http.ResponseWriter, r *http.Request) {
	var req ListResolverRuleAssociationsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeResolverError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	associations, nextToken, err := s.storage.ListResolverRuleAssociations(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleResolverError(w, err)

		return
	}

	outputs := make([]*ResolverRuleAssociationOutput, 0, len(associations))
	for _, assoc := range associations {
		outputs = append(outputs, toResolverRuleAssociationOutput(assoc))
	}

	writeJSONResponse(w, ListResolverRuleAssociationsResponse{
		ResolverRuleAssociations: outputs,
		MaxResults:               req.MaxResults,
		NextToken:                nextToken,
	})
}

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "Route53Resolver.")

	switch action {
	case "CreateResolverEndpoint":
		s.CreateResolverEndpoint(w, r)
	case "GetResolverEndpoint":
		s.GetResolverEndpoint(w, r)
	case "DeleteResolverEndpoint":
		s.DeleteResolverEndpoint(w, r)
	case "ListResolverEndpoints":
		s.ListResolverEndpoints(w, r)
	case "CreateResolverRule":
		s.CreateResolverRule(w, r)
	case "GetResolverRule":
		s.GetResolverRule(w, r)
	case "DeleteResolverRule":
		s.DeleteResolverRule(w, r)
	case "ListResolverRules":
		s.ListResolverRules(w, r)
	case "AssociateResolverRule":
		s.AssociateResolverRule(w, r)
	case "DisassociateResolverRule":
		s.DisassociateResolverRule(w, r)
	case "ListResolverRuleAssociations":
		s.ListResolverRuleAssociations(w, r)
	default:
		writeResolverError(w, errInvalidAction, "The action "+action+" is not valid", http.StatusBadRequest)
	}
}

// toResolverEndpointOutput converts a ResolverEndpoint to ResolverEndpointOutput.
func toResolverEndpointOutput(e *ResolverEndpoint) *ResolverEndpointOutput {
	return &ResolverEndpointOutput{
		ID:                    e.ID,
		CreatorRequestID:      e.CreatorRequestID,
		ARN:                   e.ARN,
		Name:                  e.Name,
		SecurityGroupIDs:      e.SecurityGroupIDs,
		Direction:             e.Direction,
		IPAddressCount:        e.IPAddressCount,
		HostVPCID:             e.HostVPCID,
		Status:                e.Status,
		StatusMessage:         e.StatusMessage,
		CreationTime:          e.CreationTime,
		ModificationTime:      e.ModificationTime,
		OutpostArn:            e.OutpostArn,
		PreferredInstanceType: e.PreferredInstanceType,
		ResolverEndpointType:  e.ResolverEndpointType,
		Protocols:             e.Protocols,
	}
}

// toResolverRuleOutput converts a ResolverRule to ResolverRuleOutput.
func toResolverRuleOutput(r *ResolverRule) *ResolverRuleOutput {
	targetIPs := make([]TargetAddress, 0, len(r.TargetIPs))
	for _, t := range r.TargetIPs {
		targetIPs = append(targetIPs, TargetAddress{
			IP:       t.IP,
			Port:     t.Port,
			IPv6:     t.IPv6,
			Protocol: t.Protocol,
		})
	}

	return &ResolverRuleOutput{
		ID:                 r.ID,
		CreatorRequestID:   r.CreatorRequestID,
		ARN:                r.ARN,
		DomainName:         r.DomainName,
		Status:             r.Status,
		StatusMessage:      r.StatusMessage,
		RuleType:           r.RuleType,
		Name:               r.Name,
		TargetIPs:          targetIPs,
		ResolverEndpointID: r.ResolverEndpointID,
		OwnerID:            r.OwnerID,
		ShareStatus:        r.ShareStatus,
		CreationTime:       r.CreationTime,
		ModificationTime:   r.ModificationTime,
	}
}

// toResolverRuleAssociationOutput converts a ResolverRuleAssociation to ResolverRuleAssociationOutput.
func toResolverRuleAssociationOutput(a *ResolverRuleAssociation) *ResolverRuleAssociationOutput {
	return &ResolverRuleAssociationOutput{
		ID:             a.ID,
		ResolverRuleID: a.ResolverRuleID,
		Name:           a.Name,
		VPCID:          a.VPCID,
		Status:         a.Status,
		StatusMessage:  a.StatusMessage,
	}
}

// handleResolverError handles errors and writes appropriate response.
func handleResolverError(w http.ResponseWriter, err error) {
	var resolverErr *ResolverError
	if errors.As(err, &resolverErr) {
		status := http.StatusBadRequest
		if resolverErr.Code == errResourceNotFound {
			status = http.StatusNotFound
		}

		writeResolverError(w, resolverErr.Code, resolverErr.Message, status)

		return
	}

	writeResolverError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// writeResolverError writes a Route 53 Resolver error response in JSON format.
func writeResolverError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Type:    code,
		Message: message,
	})
}
