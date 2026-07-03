// Package globalaccelerator provides AWS Global Accelerator service emulation.
package globalaccelerator

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
		"CreateAccelerator":     s.CreateAccelerator,
		"DescribeAccelerator":   s.DescribeAccelerator,
		"ListAccelerators":      s.ListAccelerators,
		"UpdateAccelerator":     s.UpdateAccelerator,
		"DeleteAccelerator":     s.DeleteAccelerator,
		"CreateListener":        s.CreateListener,
		"DescribeListener":      s.DescribeListener,
		"ListListeners":         s.ListListeners,
		"UpdateListener":        s.UpdateListener,
		"DeleteListener":        s.DeleteListener,
		"CreateEndpointGroup":   s.CreateEndpointGroup,
		"DescribeEndpointGroup": s.DescribeEndpointGroup,
		"ListEndpointGroups":    s.ListEndpointGroups,
		"UpdateEndpointGroup":   s.UpdateEndpointGroup,
		"DeleteEndpointGroup":   s.DeleteEndpointGroup,
	}
}

// DispatchAction dispatches the request to the appropriate handler.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "GlobalAccelerator_V20180706.")

	handlers := s.getActionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeError(w, "InvalidAction", "The action "+action+" is not valid for this endpoint.", http.StatusBadRequest)
}

// CreateAccelerator handles the CreateAccelerator API.
func (s *Service) CreateAccelerator(w http.ResponseWriter, r *http.Request) {
	var req CreateAcceleratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	accelerator, err := s.storage.CreateAccelerator(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateAcceleratorResponse{
		Accelerator: acceleratorToOutput(accelerator),
	}

	writeResponse(w, resp)
}

// DescribeAccelerator handles the DescribeAccelerator API.
func (s *Service) DescribeAccelerator(w http.ResponseWriter, r *http.Request) {
	var req DescribeAcceleratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	accelerator, err := s.storage.GetAccelerator(r.Context(), req.AcceleratorArn)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DescribeAcceleratorResponse{
		Accelerator: acceleratorToOutput(accelerator),
	}

	writeResponse(w, resp)
}

// ListAccelerators handles the ListAccelerators API.
func (s *Service) ListAccelerators(w http.ResponseWriter, r *http.Request) {
	var req ListAcceleratorsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	accelerators, nextToken, err := s.storage.ListAccelerators(r.Context(), req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	outputs := make([]AcceleratorOutput, len(accelerators))
	for i, acc := range accelerators {
		outputs[i] = *acceleratorToOutput(acc)
	}

	resp := &ListAcceleratorsResponse{
		Accelerators: outputs,
		NextToken:    nextToken,
	}

	writeResponse(w, resp)
}

// UpdateAccelerator handles the UpdateAccelerator API.
func (s *Service) UpdateAccelerator(w http.ResponseWriter, r *http.Request) {
	var req UpdateAcceleratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	accelerator, err := s.storage.UpdateAccelerator(r.Context(), req.AcceleratorArn, req.Name, req.IPAddressType, req.Enabled)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &UpdateAcceleratorResponse{
		Accelerator: acceleratorToOutput(accelerator),
	}

	writeResponse(w, resp)
}

// DeleteAccelerator handles the DeleteAccelerator API.
func (s *Service) DeleteAccelerator(w http.ResponseWriter, r *http.Request) {
	var req DeleteAcceleratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteAccelerator(r.Context(), req.AcceleratorArn); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteAcceleratorResponse{})
}

// CreateListener handles the CreateListener API.
func (s *Service) CreateListener(w http.ResponseWriter, r *http.Request) {
	var req CreateListenerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	listener, err := s.storage.CreateListener(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateListenerResponse{
		Listener: listenerToOutput(listener),
	}

	writeResponse(w, resp)
}

// DescribeListener handles the DescribeListener API.
func (s *Service) DescribeListener(w http.ResponseWriter, r *http.Request) {
	var req DescribeListenerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	listener, err := s.storage.GetListener(r.Context(), req.ListenerArn)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DescribeListenerResponse{
		Listener: listenerToOutput(listener),
	}

	writeResponse(w, resp)
}

// ListListeners handles the ListListeners API.
func (s *Service) ListListeners(w http.ResponseWriter, r *http.Request) {
	var req ListListenersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	listeners, nextToken, err := s.storage.ListListeners(r.Context(), req.AcceleratorArn, req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	outputs := make([]ListenerOutput, len(listeners))
	for i, l := range listeners {
		outputs[i] = *listenerToOutput(l)
	}

	resp := &ListListenersResponse{
		Listeners: outputs,
		NextToken: nextToken,
	}

	writeResponse(w, resp)
}

// UpdateListener handles the UpdateListener API.
func (s *Service) UpdateListener(w http.ResponseWriter, r *http.Request) {
	var req UpdateListenerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	listener, err := s.storage.UpdateListener(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &UpdateListenerResponse{
		Listener: listenerToOutput(listener),
	}

	writeResponse(w, resp)
}

// DeleteListener handles the DeleteListener API.
func (s *Service) DeleteListener(w http.ResponseWriter, r *http.Request) {
	var req DeleteListenerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteListener(r.Context(), req.ListenerArn); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteListenerResponse{})
}

// CreateEndpointGroup handles the CreateEndpointGroup API.
func (s *Service) CreateEndpointGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateEndpointGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	endpointGroup, err := s.storage.CreateEndpointGroup(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &CreateEndpointGroupResponse{
		EndpointGroup: endpointGroupToOutput(endpointGroup),
	}

	writeResponse(w, resp)
}

// DescribeEndpointGroup handles the DescribeEndpointGroup API.
func (s *Service) DescribeEndpointGroup(w http.ResponseWriter, r *http.Request) {
	var req DescribeEndpointGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	endpointGroup, err := s.storage.GetEndpointGroup(r.Context(), req.EndpointGroupArn)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &DescribeEndpointGroupResponse{
		EndpointGroup: endpointGroupToOutput(endpointGroup),
	}

	writeResponse(w, resp)
}

// ListEndpointGroups handles the ListEndpointGroups API.
func (s *Service) ListEndpointGroups(w http.ResponseWriter, r *http.Request) {
	var req ListEndpointGroupsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	endpointGroups, nextToken, err := s.storage.ListEndpointGroups(r.Context(), req.ListenerArn, req.MaxResults, req.NextToken)
	if err != nil {
		handleError(w, err)

		return
	}

	outputs := make([]EndpointGroupOutput, len(endpointGroups))
	for i, eg := range endpointGroups {
		outputs[i] = *endpointGroupToOutput(eg)
	}

	resp := &ListEndpointGroupsResponse{
		EndpointGroups: outputs,
		NextToken:      nextToken,
	}

	writeResponse(w, resp)
}

// UpdateEndpointGroup handles the UpdateEndpointGroup API.
func (s *Service) UpdateEndpointGroup(w http.ResponseWriter, r *http.Request) {
	var req UpdateEndpointGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	endpointGroup, err := s.storage.UpdateEndpointGroup(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := &UpdateEndpointGroupResponse{
		EndpointGroup: endpointGroupToOutput(endpointGroup),
	}

	writeResponse(w, resp)
}

// DeleteEndpointGroup handles the DeleteEndpointGroup API.
func (s *Service) DeleteEndpointGroup(w http.ResponseWriter, r *http.Request) {
	var req DeleteEndpointGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "ValidationException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteEndpointGroup(r.Context(), req.EndpointGroupArn); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, &DeleteEndpointGroupResponse{})
}

// acceleratorToOutput converts an Accelerator to AcceleratorOutput.
func acceleratorToOutput(acc *Accelerator) *AcceleratorOutput {
	ipSets := make([]IPSetOutput, len(acc.IPSets))

	for i, ip := range acc.IPSets {
		ipSets[i] = IPSetOutput(ip)
	}

	events := make([]EventOutput, len(acc.Events))
	for i, e := range acc.Events {
		events[i] = EventOutput{
			Message:   e.Message,
			Timestamp: float64(e.Timestamp.Unix()),
		}
	}

	return &AcceleratorOutput{
		AcceleratorArn: acc.AcceleratorArn,
		Name:           acc.Name,
		IPAddressType:  string(acc.IPAddressType),
		Enabled:        acc.Enabled,
		IPSets:         ipSets,
		DNSName:        acc.DNSName,
		Status:         string(acc.Status),
		CreatedTime:    float64(acc.CreatedTime.Unix()),
		LastModified:   float64(acc.LastModified.Unix()),
		DualStackDNS:   acc.DualStackDNS,
		Events:         events,
	}
}

// listenerToOutput converts a Listener to ListenerOutput.
func listenerToOutput(l *Listener) *ListenerOutput {
	portRanges := make([]PortRangeOutput, len(l.PortRanges))
	for i, pr := range l.PortRanges {
		portRanges[i] = PortRangeOutput(pr)
	}

	return &ListenerOutput{
		ListenerArn:    l.ListenerArn,
		PortRanges:     portRanges,
		Protocol:       string(l.Protocol),
		ClientAffinity: string(l.ClientAffinity),
	}
}

// endpointGroupToOutput converts an EndpointGroup to EndpointGroupOutput.
func endpointGroupToOutput(eg *EndpointGroup) *EndpointGroupOutput {
	endpoints := make([]EndpointDescriptionOutput, len(eg.EndpointDescriptions))
	for i, ep := range eg.EndpointDescriptions {
		endpoints[i] = EndpointDescriptionOutput{
			EndpointID:                  ep.EndpointID,
			Weight:                      ep.Weight,
			HealthState:                 string(ep.HealthState),
			HealthReason:                ep.HealthReason,
			ClientIPPreservationEnabled: ep.ClientIPPreservationEnabled,
		}
	}

	portOverrides := make([]PortOverrideOutput, len(eg.PortOverrides))
	for i, po := range eg.PortOverrides {
		portOverrides[i] = PortOverrideOutput(po)
	}

	return &EndpointGroupOutput{
		EndpointGroupArn:           eg.EndpointGroupArn,
		EndpointGroupRegion:        eg.EndpointGroupRegion,
		EndpointDescriptions:       endpoints,
		TrafficDialPercentage:      eg.TrafficDialPercentage,
		HealthCheckPort:            eg.HealthCheckPort,
		HealthCheckProtocol:        string(eg.HealthCheckProtocol),
		HealthCheckPath:            eg.HealthCheckPath,
		HealthCheckIntervalSeconds: eg.HealthCheckIntervalSeconds,
		ThresholdCount:             eg.ThresholdCount,
		PortOverrides:              portOverrides,
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
	case errNotFound, errListenerNotFound, errEndpointNotFound:
		return http.StatusNotFound
	case errAcceleratorEnabled:
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
