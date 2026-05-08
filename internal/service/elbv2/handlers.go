package elbv2

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Error codes for ELB.
const (
	errInvalidParameter = "InvalidParameterValue"
	errInternalError    = "InternalError"
	errInvalidAction    = "InvalidAction"
)

// CreateLoadBalancer handles the CreateLoadBalancer action.
func (s *Service) CreateLoadBalancer(w http.ResponseWriter, r *http.Request) {
	var req CreateLoadBalancerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeELBError(w, errInvalidParameter, "Name is required", http.StatusBadRequest)

		return
	}

	lb, err := s.storage.CreateLoadBalancer(r.Context(), &req)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLCreateLoadBalancerResponse{
		Xmlns: elbXMLNS,
		Result: XMLCreateLoadBalancerResult{
			LoadBalancers: XMLLoadBalancers{
				Members: []XMLLoadBalancer{convertToXMLLoadBalancer(lb)},
			},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteLoadBalancer handles the DeleteLoadBalancer action.
func (s *Service) DeleteLoadBalancer(w http.ResponseWriter, r *http.Request) {
	var req DeleteLoadBalancerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LoadBalancerArn == "" {
		writeELBError(w, errInvalidParameter, "LoadBalancerArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteLoadBalancer(r.Context(), req.LoadBalancerArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeleteLoadBalancerResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeleteLoadBalancerResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeLoadBalancers handles the DescribeLoadBalancers action.
func (s *Service) DescribeLoadBalancers(w http.ResponseWriter, r *http.Request) {
	var req DescribeLoadBalancersRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	lbs, err := s.storage.DescribeLoadBalancers(r.Context(), req.LoadBalancerArns, req.Names)
	if err != nil {
		handleELBError(w, err)

		return
	}

	xmlLbs := make([]XMLLoadBalancer, 0, len(lbs))
	for _, lb := range lbs {
		xmlLbs = append(xmlLbs, convertToXMLLoadBalancer(lb))
	}

	writeELBXMLResponse(w, XMLDescribeLoadBalancersResponse{
		Xmlns: elbXMLNS,
		Result: XMLDescribeLoadBalancersResult{
			LoadBalancers: XMLLoadBalancers{Members: xmlLbs},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreateTargetGroup handles the CreateTargetGroup action.
func (s *Service) CreateTargetGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateTargetGroupRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeELBError(w, errInvalidParameter, "Name is required", http.StatusBadRequest)

		return
	}

	tg, err := s.storage.CreateTargetGroup(r.Context(), &req)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLCreateTargetGroupResponse{
		Xmlns: elbXMLNS,
		Result: XMLCreateTargetGroupResult{
			TargetGroups: XMLTargetGroups{
				Members: []XMLTargetGroup{convertToXMLTargetGroup(tg)},
			},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteTargetGroup handles the DeleteTargetGroup action.
func (s *Service) DeleteTargetGroup(w http.ResponseWriter, r *http.Request) {
	var req DeleteTargetGroupRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TargetGroupArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteTargetGroup(r.Context(), req.TargetGroupArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeleteTargetGroupResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeleteTargetGroupResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeTargetGroups handles the DescribeTargetGroups action.
func (s *Service) DescribeTargetGroups(w http.ResponseWriter, r *http.Request) {
	var req DescribeTargetGroupsRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	tgs, err := s.storage.DescribeTargetGroups(r.Context(), req.TargetGroupArns, req.Names, req.LoadBalancerArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	xmlTgs := make([]XMLTargetGroup, 0, len(tgs))
	for _, tg := range tgs {
		xmlTgs = append(xmlTgs, convertToXMLTargetGroup(tg))
	}

	writeELBXMLResponse(w, XMLDescribeTargetGroupsResponse{
		Xmlns: elbXMLNS,
		Result: XMLDescribeTargetGroupsResult{
			TargetGroups: XMLTargetGroups{Members: xmlTgs},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// RegisterTargets handles the RegisterTargets action. The Query form pattern
// Targets.member.N.{Id,Port,AvailabilityZone} is read directly from r.Form
// because the generic form-to-JSON converter does not understand the nested
// member.N.<field> shape and would emit flat dotted keys instead.
func (s *Service) RegisterTargets(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	tgArn := r.Form.Get("TargetGroupArn")
	if tgArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.RegisterTargets(r.Context(), tgArn, parseELBTargetsFromForm(r.Form)); err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLRegisterTargetsResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLRegisterTargetsResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeregisterTargets handles the DeregisterTargets action.
func (s *Service) DeregisterTargets(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse form data", http.StatusBadRequest)

		return
	}

	tgArn := r.Form.Get("TargetGroupArn")
	if tgArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeregisterTargets(r.Context(), tgArn, parseELBTargetsFromForm(r.Form)); err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeregisterTargetsResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeregisterTargetsResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// parseELBTargetsFromForm reads Targets.member.N.{Id,Port,AvailabilityZone}.
func parseELBTargetsFromForm(form map[string][]string) []Target {
	byIdx := make(map[int]*Target)

	for key, values := range form {
		applyELBTargetFormEntry(byIdx, key, values)
	}

	indexes := make([]int, 0, len(byIdx))
	for n := range byIdx {
		indexes = append(indexes, n)
	}

	sort.Ints(indexes)

	out := make([]Target, 0, len(indexes))
	for _, n := range indexes {
		out = append(out, *byIdx[n])
	}

	return out
}

func applyELBTargetFormEntry(byIdx map[int]*Target, key string, values []string) {
	suffix, ok := strings.CutPrefix(key, "Targets.member.")
	if !ok || len(values) == 0 {
		return
	}

	dot := strings.Index(suffix, ".")
	if dot < 0 {
		return
	}

	n, err := strconv.Atoi(suffix[:dot])
	if err != nil {
		return
	}

	entry, exists := byIdx[n]
	if !exists {
		entry = &Target{}
		byIdx[n] = entry
	}

	switch suffix[dot+1:] {
	case "Id":
		entry.ID = values[0]
	case "Port":
		if v, err := strconv.Atoi(values[0]); err == nil {
			entry.Port = v
		}
	case "AvailabilityZone":
		entry.AvailabilityZone = values[0]
	}
}

// CreateListener handles the CreateListener action.
func (s *Service) CreateListener(w http.ResponseWriter, r *http.Request) {
	var req CreateListenerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LoadBalancerArn == "" {
		writeELBError(w, errInvalidParameter, "LoadBalancerArn is required", http.StatusBadRequest)

		return
	}

	listener, err := s.storage.CreateListener(r.Context(), &req)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLCreateListenerResponse{
		Xmlns: elbXMLNS,
		Result: XMLCreateListenerResult{
			Listeners: XMLListeners{
				Members: []XMLListener{convertToXMLListener(listener)},
			},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteListener handles the DeleteListener action.
func (s *Service) DeleteListener(w http.ResponseWriter, r *http.Request) {
	var req DeleteListenerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ListenerArn == "" {
		writeELBError(w, errInvalidParameter, "ListenerArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteListener(r.Context(), req.ListenerArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeleteListenerResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeleteListenerResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)
	handler := s.getActionHandler(action)

	if handler == nil {
		writeELBError(w, errInvalidAction, fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)

		return
	}

	handler(w, r)
}

// getActionHandler returns the handler function for the given action.
func (s *Service) getActionHandler(action string) func(http.ResponseWriter, *http.Request) {
	handlers := map[string]func(http.ResponseWriter, *http.Request){
		"CreateLoadBalancer":    s.CreateLoadBalancer,
		"DeleteLoadBalancer":    s.DeleteLoadBalancer,
		"DescribeLoadBalancers": s.DescribeLoadBalancers,
		"CreateTargetGroup":     s.CreateTargetGroup,
		"DeleteTargetGroup":     s.DeleteTargetGroup,
		"DescribeTargetGroups":  s.DescribeTargetGroups,
		"RegisterTargets":       s.RegisterTargets,
		"DeregisterTargets":     s.DeregisterTargets,
		"CreateListener":        s.CreateListener,
		"DeleteListener":        s.DeleteListener,
	}

	return handlers[action]
}

// Helper functions.

// convertToXMLLoadBalancer converts a LoadBalancer to XMLLoadBalancer.
func convertToXMLLoadBalancer(lb *LoadBalancer) XMLLoadBalancer {
	azs := make([]XMLAvailabilityZone, 0, len(lb.AvailabilityZones))
	for _, az := range lb.AvailabilityZones {
		azs = append(azs, XMLAvailabilityZone{
			ZoneName: az.ZoneName,
			SubnetID: az.SubnetID,
		})
	}

	return XMLLoadBalancer{
		LoadBalancerArn:       lb.LoadBalancerArn,
		DNSName:               lb.DNSName,
		CanonicalHostedZoneID: lb.CanonicalHostedZoneID,
		CreatedTime:           lb.CreatedTime.Format("2006-01-02T15:04:05.000Z"),
		LoadBalancerName:      lb.LoadBalancerName,
		Scheme:                lb.Scheme,
		VpcID:                 lb.VpcID,
		State:                 XMLLoadBalancerState{Code: lb.State.Code, Reason: lb.State.Reason},
		Type:                  lb.Type,
		AvailabilityZones:     XMLAvailabilityZones{Members: azs},
		SecurityGroups:        XMLSecurityGroups{Members: lb.SecurityGroups},
		IPAddressType:         lb.IPAddressType,
	}
}

// convertToXMLTargetGroup converts a TargetGroup to XMLTargetGroup.
func convertToXMLTargetGroup(tg *TargetGroup) XMLTargetGroup {
	return XMLTargetGroup{
		TargetGroupArn:             tg.TargetGroupArn,
		TargetGroupName:            tg.TargetGroupName,
		Protocol:                   tg.Protocol,
		Port:                       tg.Port,
		VpcID:                      tg.VpcID,
		HealthCheckEnabled:         tg.HealthCheckEnabled,
		HealthCheckIntervalSeconds: tg.HealthCheckIntervalSeconds,
		HealthCheckPath:            tg.HealthCheckPath,
		HealthCheckPort:            tg.HealthCheckPort,
		HealthCheckProtocol:        tg.HealthCheckProtocol,
		HealthCheckTimeoutSeconds:  tg.HealthCheckTimeoutSeconds,
		HealthyThresholdCount:      tg.HealthyThresholdCount,
		UnhealthyThresholdCount:    tg.UnhealthyThresholdCount,
		TargetType:                 tg.TargetType,
		LoadBalancerArns:           XMLLoadBalancerArns{Members: tg.LoadBalancerArns},
	}
}

// convertToXMLListener converts a Listener to XMLListener.
func convertToXMLListener(l *Listener) XMLListener {
	actions := make([]XMLAction, 0, len(l.DefaultActions))
	for _, a := range l.DefaultActions {
		actions = append(actions, XMLAction(a))
	}

	return XMLListener{
		ListenerArn:     l.ListenerArn,
		LoadBalancerArn: l.LoadBalancerArn,
		Port:            l.Port,
		Protocol:        l.Protocol,
		DefaultActions:  XMLActions{Members: actions},
	}
}

// readELBJSONRequest reads and decodes JSON request body.
func readELBJSONRequest(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// extractAction extracts the action name from the request.
func extractAction(r *http.Request) string {
	// Try X-Amz-Target header (format: "ElasticLoadBalancing.ActionName").
	target := r.Header.Get("X-Amz-Target")
	if target != "" {
		if idx := strings.LastIndex(target, "."); idx >= 0 {
			return target[idx+1:]
		}
	}

	// Fallback to URL query parameter.
	return r.URL.Query().Get("Action")
}

// writeELBXMLResponse writes an XML response with HTTP 200 OK.
func writeELBXMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

// writeELBError writes an ELB error response in XML format.
func writeELBError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(XMLErrorResponse{
		Error: XMLError{
			Type:    "Sender",
			Code:    code,
			Message: message,
		},
		RequestID: requestID,
	})
}

// handleELBError handles ELB errors and writes the appropriate response.
func handleELBError(w http.ResponseWriter, err error) {
	var elbErr *Error
	if errors.As(err, &elbErr) {
		writeELBError(w, elbErr.Code, elbErr.Message, http.StatusBadRequest)

		return
	}

	writeELBError(w, errInternalError, "Internal server error", http.StatusInternalServerError)
}
