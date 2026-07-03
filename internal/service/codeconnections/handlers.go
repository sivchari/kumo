// Package codeconnections provides AWS CodeConnections service emulation.
package codeconnections

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service"
)

// Error codes for CodeConnections handlers.
const (
	errInvalidAction           = "InvalidAction"
	errInternalServerException = "InternalServerException"
	errInvalidInputException   = "InvalidInputException"
)

// handlerFunc is the type for action handler functions.
type handlerFunc func(http.ResponseWriter, *http.Request)

// getActionHandlers returns the action handlers map.
func (s *Service) getActionHandlers() map[string]handlerFunc {
	return map[string]handlerFunc{
		// Connection operations
		"CreateConnection": s.CreateConnection,
		"GetConnection":    s.GetConnection,
		"DeleteConnection": s.DeleteConnection,
		"ListConnections":  s.ListConnections,
		// Host operations
		"CreateHost": s.CreateHost,
		"GetHost":    s.GetHost,
		"DeleteHost": s.DeleteHost,
		"ListHosts":  s.ListHosts,
		"UpdateHost": s.UpdateHost,
		// Repository link operations
		"CreateRepositoryLink": s.CreateRepositoryLink,
		"GetRepositoryLink":    s.GetRepositoryLink,
		"DeleteRepositoryLink": s.DeleteRepositoryLink,
		"ListRepositoryLinks":  s.ListRepositoryLinks,
		"UpdateRepositoryLink": s.UpdateRepositoryLink,
		// Tag operations
		"ListTagsForResource": s.ListTagsForResource,
		"TagResource":         s.TagResource,
		"UntagResource":       s.UntagResource,
	}
}

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "CodeConnections_20231201.")

	handlers := s.getActionHandlers()

	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeCodeConnectionsError(w, errInvalidAction, "The action "+action+" is not valid", http.StatusBadRequest)
}

// CreateConnection handles the CreateConnection action.
func (s *Service) CreateConnection(w http.ResponseWriter, r *http.Request) {
	var req CreateConnectionRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ConnectionName == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "ConnectionName is required.", http.StatusBadRequest)

		return
	}

	conn, err := s.storage.CreateConnection(r.Context(), req.ConnectionName, req.ProviderType, req.HostArn, req.Tags)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, CreateConnectionResponse{
		ConnectionArn: conn.ConnectionArn,
		Tags:          req.Tags,
	})
}

// GetConnection handles the GetConnection action.
func (s *Service) GetConnection(w http.ResponseWriter, r *http.Request) {
	var req GetConnectionRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ConnectionArn == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "ConnectionArn is required.", http.StatusBadRequest)

		return
	}

	conn, err := s.storage.GetConnection(r.Context(), req.ConnectionArn)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, GetConnectionResponse{
		Connection: convertConnectionToOutput(conn),
	})
}

// DeleteConnection handles the DeleteConnection action.
func (s *Service) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	var req DeleteConnectionRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ConnectionArn == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "ConnectionArn is required.", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteConnection(r.Context(), req.ConnectionArn); err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, DeleteConnectionResponse{})
}

// ListConnections handles the ListConnections action.
func (s *Service) ListConnections(w http.ResponseWriter, r *http.Request) {
	var req ListConnectionsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	connections, nextToken, err := s.storage.ListConnections(r.Context(), req.ProviderTypeFilter, req.HostArnFilter, req.NextToken, req.MaxResults)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	outputs := make([]ConnectionOutput, 0, len(connections))
	for _, conn := range connections {
		outputs = append(outputs, *convertConnectionToOutput(conn))
	}

	writeJSONResponse(w, ListConnectionsResponse{
		Connections: outputs,
		NextToken:   nextToken,
	})
}

// CreateHost handles the CreateHost action.
func (s *Service) CreateHost(w http.ResponseWriter, r *http.Request) {
	var req CreateHostRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "Name is required.", http.StatusBadRequest)

		return
	}

	if req.ProviderType == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "ProviderType is required.", http.StatusBadRequest)

		return
	}

	if req.ProviderEndpoint == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "ProviderEndpoint is required.", http.StatusBadRequest)

		return
	}

	vpcConfig := convertVpcConfigInputToInternal(req.VpcConfiguration)

	host, err := s.storage.CreateHost(r.Context(), req.Name, req.ProviderType, req.ProviderEndpoint, vpcConfig, req.Tags)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, CreateHostResponse{
		HostArn: host.HostArn,
		Tags:    req.Tags,
	})
}

// GetHost handles the GetHost action.
func (s *Service) GetHost(w http.ResponseWriter, r *http.Request) {
	var req GetHostRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.HostArn == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "HostArn is required.", http.StatusBadRequest)

		return
	}

	host, err := s.storage.GetHost(r.Context(), req.HostArn)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, GetHostResponse{
		Name:             host.Name,
		Status:           host.Status,
		ProviderType:     string(host.ProviderType),
		ProviderEndpoint: host.ProviderEndpoint,
		VpcConfiguration: convertVpcConfigToOutput(host.VpcConfiguration),
	})
}

// DeleteHost handles the DeleteHost action.
func (s *Service) DeleteHost(w http.ResponseWriter, r *http.Request) {
	var req DeleteHostRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.HostArn == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "HostArn is required.", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteHost(r.Context(), req.HostArn); err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, DeleteHostResponse{})
}

// ListHosts handles the ListHosts action.
func (s *Service) ListHosts(w http.ResponseWriter, r *http.Request) {
	var req ListHostsRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	hosts, nextToken, err := s.storage.ListHosts(r.Context(), req.NextToken, req.MaxResults)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	outputs := make([]HostOutput, 0, len(hosts))
	for _, host := range hosts {
		outputs = append(outputs, convertHostToOutput(host))
	}

	writeJSONResponse(w, ListHostsResponse{
		Hosts:     outputs,
		NextToken: nextToken,
	})
}

// UpdateHost handles the UpdateHost action.
func (s *Service) UpdateHost(w http.ResponseWriter, r *http.Request) {
	var req UpdateHostRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.HostArn == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "HostArn is required.", http.StatusBadRequest)

		return
	}

	vpcConfig := convertVpcConfigInputToInternal(req.VpcConfiguration)

	if err := s.storage.UpdateHost(r.Context(), req.HostArn, req.ProviderEndpoint, vpcConfig); err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, UpdateHostResponse{})
}

// CreateRepositoryLink handles the CreateRepositoryLink action.
func (s *Service) CreateRepositoryLink(w http.ResponseWriter, r *http.Request) {
	var req CreateRepositoryLinkRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ConnectionArn == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "ConnectionArn is required.", http.StatusBadRequest)

		return
	}

	if req.OwnerID == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "OwnerId is required.", http.StatusBadRequest)

		return
	}

	if req.RepositoryName == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "RepositoryName is required.", http.StatusBadRequest)

		return
	}

	repoLink, err := s.storage.CreateRepositoryLink(r.Context(), req.ConnectionArn, req.OwnerID, req.RepositoryName, req.EncryptionKeyArn, req.Tags)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, CreateRepositoryLinkResponse{
		RepositoryLinkInfo: convertRepositoryLinkToOutput(repoLink),
	})
}

// GetRepositoryLink handles the GetRepositoryLink action.
func (s *Service) GetRepositoryLink(w http.ResponseWriter, r *http.Request) {
	var req GetRepositoryLinkRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.RepositoryLinkID == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "RepositoryLinkId is required.", http.StatusBadRequest)

		return
	}

	repoLink, err := s.storage.GetRepositoryLink(r.Context(), req.RepositoryLinkID)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, GetRepositoryLinkResponse{
		RepositoryLinkInfo: convertRepositoryLinkToOutput(repoLink),
	})
}

// DeleteRepositoryLink handles the DeleteRepositoryLink action.
func (s *Service) DeleteRepositoryLink(w http.ResponseWriter, r *http.Request) {
	var req DeleteRepositoryLinkRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.RepositoryLinkID == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "RepositoryLinkId is required.", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteRepositoryLink(r.Context(), req.RepositoryLinkID); err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, DeleteRepositoryLinkResponse{})
}

// ListRepositoryLinks handles the ListRepositoryLinks action.
func (s *Service) ListRepositoryLinks(w http.ResponseWriter, r *http.Request) {
	var req ListRepositoryLinksRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	links, nextToken, err := s.storage.ListRepositoryLinks(r.Context(), req.NextToken, req.MaxResults)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	outputs := make([]RepositoryLinkOutput, 0, len(links))
	for _, link := range links {
		outputs = append(outputs, *convertRepositoryLinkToOutput(link))
	}

	writeJSONResponse(w, ListRepositoryLinksResponse{
		RepositoryLinks: outputs,
		NextToken:       nextToken,
	})
}

// UpdateRepositoryLink handles the UpdateRepositoryLink action.
func (s *Service) UpdateRepositoryLink(w http.ResponseWriter, r *http.Request) {
	var req UpdateRepositoryLinkRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.RepositoryLinkID == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "RepositoryLinkId is required.", http.StatusBadRequest)

		return
	}

	repoLink, err := s.storage.UpdateRepositoryLink(r.Context(), req.RepositoryLinkID, req.ConnectionArn, req.EncryptionKeyArn)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, UpdateRepositoryLinkResponse{
		RepositoryLinkInfo: convertRepositoryLinkToOutput(repoLink),
	})
}

// ListTagsForResource handles the ListTagsForResource action.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	var req ListTagsForResourceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceArn == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "ResourceArn is required.", http.StatusBadRequest)

		return
	}

	tags, err := s.storage.ListTagsForResource(r.Context(), req.ResourceArn)
	if err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, ListTagsForResourceResponse{
		Tags: tags,
	})
}

// TagResource handles the TagResource action.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	var req TagResourceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceArn == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "ResourceArn is required.", http.StatusBadRequest)

		return
	}

	if len(req.Tags) == 0 {
		writeCodeConnectionsError(w, errInvalidInputException, "Tags are required.", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(r.Context(), req.ResourceArn, req.Tags); err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, TagResourceResponse{})
}

// UntagResource handles the UntagResource action.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	var req UntagResourceRequest
	if err := service.ReadJSONRequest(r, &req); err != nil {
		writeCodeConnectionsError(w, errInvalidInputException, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceArn == "" {
		writeCodeConnectionsError(w, errInvalidInputException, "ResourceArn is required.", http.StatusBadRequest)

		return
	}

	if len(req.TagKeys) == 0 {
		writeCodeConnectionsError(w, errInvalidInputException, "TagKeys are required.", http.StatusBadRequest)

		return
	}

	if err := s.storage.UntagResource(r.Context(), req.ResourceArn, req.TagKeys); err != nil {
		handleCodeConnectionsError(w, err)

		return
	}

	writeJSONResponse(w, UntagResourceResponse{})
}

// convertConnectionToOutput converts internal Connection to API output.
func convertConnectionToOutput(conn *Connection) *ConnectionOutput {
	return &ConnectionOutput{
		ConnectionArn:    conn.ConnectionArn,
		ConnectionName:   conn.ConnectionName,
		ConnectionStatus: string(conn.ConnectionStatus),
		OwnerAccountID:   conn.OwnerAccountID,
		ProviderType:     string(conn.ProviderType),
		HostArn:          conn.HostArn,
	}
}

// convertHostToOutput converts internal Host to API output.
func convertHostToOutput(host *Host) HostOutput {
	return HostOutput{
		HostArn:          host.HostArn,
		Name:             host.Name,
		Status:           host.Status,
		ProviderType:     string(host.ProviderType),
		ProviderEndpoint: host.ProviderEndpoint,
		VpcConfiguration: convertVpcConfigToOutput(host.VpcConfiguration),
		StatusMessage:    host.StatusMessage,
	}
}

// convertVpcConfigToOutput converts internal VpcConfiguration to API output.
func convertVpcConfigToOutput(cfg *VpcConfiguration) *VpcConfigOutput {
	if cfg == nil {
		return nil
	}

	return &VpcConfigOutput{
		VpcID:            cfg.VpcID,
		SubnetIDs:        cfg.SubnetIDs,
		SecurityGroupIDs: cfg.SecurityGroupIDs,
		TLSCertificate:   cfg.TLSCertificate,
	}
}

// convertVpcConfigInputToInternal converts VpcConfigInput to internal VpcConfiguration.
func convertVpcConfigInputToInternal(input *VpcConfigInput) *VpcConfiguration {
	if input == nil {
		return nil
	}

	return &VpcConfiguration{
		VpcID:            input.VpcID,
		SubnetIDs:        input.SubnetIDs,
		SecurityGroupIDs: input.SecurityGroupIDs,
		TLSCertificate:   input.TLSCertificate,
	}
}

// convertRepositoryLinkToOutput converts internal RepositoryLink to API output.
func convertRepositoryLinkToOutput(link *RepositoryLink) *RepositoryLinkOutput {
	return &RepositoryLinkOutput{
		RepositoryLinkArn: link.RepositoryLinkArn,
		RepositoryLinkID:  link.RepositoryLinkID,
		ConnectionArn:     link.ConnectionArn,
		OwnerID:           link.OwnerID,
		ProviderType:      string(link.ProviderType),
		RepositoryName:    link.RepositoryName,
		EncryptionKeyArn:  link.EncryptionKeyArn,
	}
}

// writeJSONResponse writes a JSON response with HTTP 200 OK.
func writeJSONResponse(w http.ResponseWriter, v any) {
	service.WriteJSONResponse(w, service.ContentTypeAmzJSON11, v)
}

// writeCodeConnectionsError writes a CodeConnections error response in JSON format.
func writeCodeConnectionsError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Type:    code,
		Message: message,
	})
}

// handleCodeConnectionsError handles CodeConnections errors and writes the appropriate response.
func handleCodeConnectionsError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		status := http.StatusBadRequest

		switch svcErr.Code {
		case errResourceNotFoundException:
			status = http.StatusNotFound
		case errInternalServerException:
			status = http.StatusInternalServerError
		}

		writeCodeConnectionsError(w, svcErr.Code, svcErr.Message, status)

		return
	}

	writeCodeConnectionsError(w, errInternalServerException, "Internal server error", http.StatusInternalServerError)
}
