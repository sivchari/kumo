package apigateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// CreateRestAPI handles the CreateRestApi API.
func (s *Service) CreateRestAPI(w http.ResponseWriter, r *http.Request) {
	var req CreateRestAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BadRequestException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, "BadRequestException", "Name is required", http.StatusBadRequest)

		return
	}

	api, err := s.storage.CreateRestAPI(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toRestAPIResponse(api)
	writeResponse(w, resp, http.StatusCreated)
}

// GetRestAPI handles the GetRestApi API.
func (s *Service) GetRestAPI(w http.ResponseWriter, r *http.Request) {
	restAPIID := extractPathParam(r.URL.Path, "/apigateway/restapis/")

	api, err := s.storage.GetRestAPI(r.Context(), restAPIID)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toRestAPIResponse(api)
	writeResponse(w, resp, http.StatusOK)
}

// GetRestAPIs handles the GetRestApis API.
func (s *Service) GetRestAPIs(w http.ResponseWriter, r *http.Request) {
	apis, nextPosition, err := s.storage.GetRestAPIs(r.Context(), 25, "")
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]CreateRestAPIResponse, len(apis))

	for i, api := range apis {
		items[i] = *toRestAPIResponse(api)
	}

	resp := &GetRestAPIsResponse{
		Items:    items,
		Position: nextPosition,
	}

	writeResponse(w, resp, http.StatusOK)
}

// DeleteRestAPI handles the DeleteRestApi API.
func (s *Service) DeleteRestAPI(w http.ResponseWriter, r *http.Request) {
	restAPIID := extractPathParam(r.URL.Path, "/apigateway/restapis/")

	if err := s.storage.DeleteRestAPI(r.Context(), restAPIID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// CreateResource handles the CreateResource API.
func (s *Service) CreateResource(w http.ResponseWriter, r *http.Request) {
	restAPIID, parentID := extractResourceParams(r.URL.Path)

	var req CreateResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BadRequestException", "Invalid request body", http.StatusBadRequest)

		return
	}

	resource, err := s.storage.CreateResource(r.Context(), restAPIID, parentID, req.PathPart)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toResourceResponse(resource)
	writeResponse(w, resp, http.StatusCreated)
}

// GetResource handles the GetResource API.
func (s *Service) GetResource(w http.ResponseWriter, r *http.Request) {
	restAPIID, resourceID := extractRestAPIAndResourceID(r.URL.Path)

	resource, err := s.storage.GetResource(r.Context(), restAPIID, resourceID)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toResourceResponse(resource)
	writeResponse(w, resp, http.StatusOK)
}

// GetResources handles the GetResources API.
func (s *Service) GetResources(w http.ResponseWriter, r *http.Request) {
	parts := pathSegmentsAfterRestapis(r.URL.Path)
	if len(parts) < 1 {
		writeError(w, "BadRequestException", "restApiId is required", http.StatusBadRequest)

		return
	}

	resources, nextPosition, err := s.storage.GetResources(r.Context(), parts[0], 25, "")
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]ResourceResponse, len(resources))

	for i, res := range resources {
		items[i] = *toResourceResponse(res)
	}

	resp := &GetResourcesResponse{
		Items:    items,
		Position: nextPosition,
	}

	writeResponse(w, resp, http.StatusOK)
}

// DeleteResource handles the DeleteResource API.
func (s *Service) DeleteResource(w http.ResponseWriter, r *http.Request) {
	restAPIID, resourceID := extractRestAPIAndResourceID(r.URL.Path)

	if err := s.storage.DeleteResource(r.Context(), restAPIID, resourceID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// PutMethod handles the PutMethod API.
func (s *Service) PutMethod(w http.ResponseWriter, r *http.Request) {
	restAPIID, resourceID, httpMethod := extractMethodParams(r.URL.Path)

	var req PutMethodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BadRequestException", "Invalid request body", http.StatusBadRequest)

		return
	}

	method, err := s.storage.PutMethod(r.Context(), restAPIID, resourceID, httpMethod, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toMethodOutput(method)
	writeResponse(w, resp, http.StatusCreated)
}

// GetMethod handles the GetMethod API.
func (s *Service) GetMethod(w http.ResponseWriter, r *http.Request) {
	restAPIID, resourceID, httpMethod := extractMethodParams(r.URL.Path)

	method, err := s.storage.GetMethod(r.Context(), restAPIID, resourceID, httpMethod)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toMethodOutput(method)
	writeResponse(w, resp, http.StatusOK)
}

// DeleteMethod handles the DeleteMethod API.
func (s *Service) DeleteMethod(w http.ResponseWriter, r *http.Request) {
	restAPIID, resourceID, httpMethod := extractMethodParams(r.URL.Path)

	if err := s.storage.DeleteMethod(r.Context(), restAPIID, resourceID, httpMethod); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// PutIntegration handles the PutIntegration API.
func (s *Service) PutIntegration(w http.ResponseWriter, r *http.Request) {
	restAPIID, resourceID, httpMethod := extractIntegrationParams(r.URL.Path)

	var req PutIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BadRequestException", "Invalid request body", http.StatusBadRequest)

		return
	}

	integration, err := s.storage.PutIntegration(r.Context(), restAPIID, resourceID, httpMethod, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toIntegrationOutput(integration)
	writeResponse(w, resp, http.StatusCreated)
}

// GetIntegration handles the GetIntegration API.
func (s *Service) GetIntegration(w http.ResponseWriter, r *http.Request) {
	restAPIID, resourceID, httpMethod := extractIntegrationParams(r.URL.Path)

	integration, err := s.storage.GetIntegration(r.Context(), restAPIID, resourceID, httpMethod)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toIntegrationOutput(integration)
	writeResponse(w, resp, http.StatusOK)
}

// CreateDeployment handles the CreateDeployment API.
func (s *Service) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	restAPIID := extractDeploymentRestAPIID(r.URL.Path)

	var req CreateDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BadRequestException", "Invalid request body", http.StatusBadRequest)

		return
	}

	deployment, err := s.storage.CreateDeployment(r.Context(), restAPIID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toDeploymentResponse(deployment)
	writeResponse(w, resp, http.StatusCreated)
}

// GetDeployment handles the GetDeployment API.
func (s *Service) GetDeployment(w http.ResponseWriter, r *http.Request) {
	restAPIID, deploymentID := extractRestAPIAndDeploymentID(r.URL.Path)

	deployment, err := s.storage.GetDeployment(r.Context(), restAPIID, deploymentID)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toDeploymentResponse(deployment)
	writeResponse(w, resp, http.StatusOK)
}

// GetDeployments handles the GetDeployments API.
func (s *Service) GetDeployments(w http.ResponseWriter, r *http.Request) {
	restAPIID := extractDeploymentRestAPIID(r.URL.Path)

	deployments, nextPosition, err := s.storage.GetDeployments(r.Context(), restAPIID, 25, "")
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]DeploymentResponse, len(deployments))

	for i, d := range deployments {
		items[i] = *toDeploymentResponse(d)
	}

	resp := &GetDeploymentsResponse{
		Items:    items,
		Position: nextPosition,
	}

	writeResponse(w, resp, http.StatusOK)
}

// DeleteDeployment handles the DeleteDeployment API.
func (s *Service) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	restAPIID, deploymentID := extractRestAPIAndDeploymentID(r.URL.Path)

	if err := s.storage.DeleteDeployment(r.Context(), restAPIID, deploymentID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// CreateStage handles the CreateStage API.
func (s *Service) CreateStage(w http.ResponseWriter, r *http.Request) {
	restAPIID := extractStageRestAPIID(r.URL.Path)

	var req CreateStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BadRequestException", "Invalid request body", http.StatusBadRequest)

		return
	}

	stage, err := s.storage.CreateStage(r.Context(), restAPIID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toStageResponse(stage)
	writeResponse(w, resp, http.StatusCreated)
}

// GetStage handles the GetStage API.
func (s *Service) GetStage(w http.ResponseWriter, r *http.Request) {
	restAPIID, stageName := extractRestAPIAndStageName(r.URL.Path)

	stage, err := s.storage.GetStage(r.Context(), restAPIID, stageName)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toStageResponse(stage)
	writeResponse(w, resp, http.StatusOK)
}

// GetStages handles the GetStages API.
func (s *Service) GetStages(w http.ResponseWriter, r *http.Request) {
	restAPIID := extractStageRestAPIID(r.URL.Path)

	stages, err := s.storage.GetStages(r.Context(), restAPIID)
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]StageResponse, len(stages))

	for i, stage := range stages {
		items[i] = *toStageResponse(stage)
	}

	resp := &GetStagesResponse{
		Items: items,
	}

	writeResponse(w, resp, http.StatusOK)
}

// DeleteStage handles the DeleteStage API.
func (s *Service) DeleteStage(w http.ResponseWriter, r *http.Request) {
	restAPIID, stageName := extractRestAPIAndStageName(r.URL.Path)

	if err := s.storage.DeleteStage(r.Context(), restAPIID, stageName); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// CreateAuthorizer handles the CreateAuthorizer API.
func (s *Service) CreateAuthorizer(w http.ResponseWriter, r *http.Request) {
	restAPIID := extractAuthorizerRestAPIID(r.URL.Path)

	var req CreateAuthorizerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "BadRequestException", "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" || req.Type == "" {
		writeError(w, "BadRequestException", "Name and type are required", http.StatusBadRequest)

		return
	}

	authorizer, err := s.storage.CreateAuthorizer(r.Context(), restAPIID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toAuthorizerResponse(authorizer)
	writeResponse(w, resp, http.StatusCreated)
}

// GetAuthorizer handles the GetAuthorizer API.
func (s *Service) GetAuthorizer(w http.ResponseWriter, r *http.Request) {
	restAPIID, authorizerID := extractRestAPIAndAuthorizerID(r.URL.Path)

	authorizer, err := s.storage.GetAuthorizer(r.Context(), restAPIID, authorizerID)
	if err != nil {
		handleError(w, err)

		return
	}

	resp := toAuthorizerResponse(authorizer)
	writeResponse(w, resp, http.StatusOK)
}

// GetAuthorizers handles the GetAuthorizers API.
func (s *Service) GetAuthorizers(w http.ResponseWriter, r *http.Request) {
	restAPIID := extractAuthorizerRestAPIID(r.URL.Path)

	authorizers, err := s.storage.GetAuthorizers(r.Context(), restAPIID)
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]AuthorizerResponse, len(authorizers))

	for i, a := range authorizers {
		items[i] = *toAuthorizerResponse(a)
	}

	resp := &GetAuthorizersResponse{Items: items}
	writeResponse(w, resp, http.StatusOK)
}

// DeleteAuthorizer handles the DeleteAuthorizer API.
func (s *Service) DeleteAuthorizer(w http.ResponseWriter, r *http.Request) {
	restAPIID, authorizerID := extractRestAPIAndAuthorizerID(r.URL.Path)

	if err := s.storage.DeleteAuthorizer(r.Context(), restAPIID, authorizerID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// toRestAPIResponse converts a RestAPI to CreateRestAPIResponse.
func toRestAPIResponse(api *RestAPI) *CreateRestAPIResponse {
	return &CreateRestAPIResponse{
		ID:                     api.ID,
		Name:                   api.Name,
		Description:            api.Description,
		CreatedDate:            float64(api.CreatedDate.Unix()),
		Version:                api.Version,
		APIKeySource:           api.APIKeySource,
		EndpointConfiguration:  api.EndpointConfiguration,
		DisableExecuteAPIEndpt: api.DisableExecuteAPIEndpt,
		Tags:                   api.Tags,
		RootResourceID:         api.RootResourceID,
	}
}

// toResourceResponse converts a Resource to ResourceResponse.
func toResourceResponse(res *Resource) *ResourceResponse {
	methods := make(map[string]MethodOutput)

	for k, m := range res.ResourceMethods {
		methods[k] = *toMethodOutput(&m)
	}

	return &ResourceResponse{
		ID:              res.ID,
		ParentID:        res.ParentID,
		PathPart:        res.PathPart,
		Path:            res.Path,
		ResourceMethods: methods,
	}
}

// toMethodOutput converts a Method to MethodOutput.
func toMethodOutput(m *Method) *MethodOutput {
	output := &MethodOutput{
		HTTPMethod:        m.HTTPMethod,
		AuthorizationType: m.AuthorizationType,
		AuthorizerID:      m.AuthorizerID,
		APIKeyRequired:    m.APIKeyRequired,
		OperationName:     m.OperationName,
	}

	if m.MethodIntegration != nil {
		output.MethodIntegration = toIntegrationOutput(m.MethodIntegration)
	}

	return output
}

// toIntegrationOutput converts an Integration to IntegrationOutput.
func toIntegrationOutput(i *Integration) *IntegrationOutput {
	return &IntegrationOutput{
		Type:                i.Type,
		HTTPMethod:          i.HTTPMethod,
		URI:                 i.URI,
		ConnectionType:      i.ConnectionType,
		ConnectionID:        i.ConnectionID,
		PassthroughBehavior: i.PassthroughBehavior,
		ContentHandling:     i.ContentHandling,
		TimeoutInMillis:     i.TimeoutInMillis,
		CacheNamespace:      i.CacheNamespace,
		CacheKeyParameters:  i.CacheKeyParameters,
		RequestParameters:   i.RequestParameters,
		RequestTemplates:    i.RequestTemplates,
	}
}

// toDeploymentResponse converts a Deployment to DeploymentResponse.
func toDeploymentResponse(d *Deployment) *DeploymentResponse {
	return &DeploymentResponse{
		ID:          d.ID,
		Description: d.Description,
		CreatedDate: float64(d.CreatedDate.Unix()),
	}
}

// toStageResponse converts a Stage to StageResponse.
func toStageResponse(s *Stage) *StageResponse {
	return &StageResponse{
		StageName:           s.StageName,
		DeploymentID:        s.DeploymentID,
		Description:         s.Description,
		CacheClusterEnabled: s.CacheClusterEnabled,
		CacheClusterSize:    s.CacheClusterSize,
		CreatedDate:         float64(s.CreatedDate.Unix()),
		LastUpdatedDate:     float64(s.LastUpdatedDate.Unix()),
		Tags:                s.Tags,
	}
}

// toAuthorizerResponse converts an Authorizer to AuthorizerResponse.
func toAuthorizerResponse(a *Authorizer) *AuthorizerResponse {
	return &AuthorizerResponse{
		ID:                           a.ID,
		Name:                         a.Name,
		Type:                         a.Type,
		AuthorizerURI:                a.AuthorizerURI,
		IdentitySource:               a.IdentitySource,
		AuthorizerResultTTLInSeconds: a.AuthorizerResultTTLInSeconds,
	}
}

// extractPathParam returns the segment immediately after the given
// prefix. Tolerates both /apigateway/<...> (legacy SDK BaseEndpoint) and
// /<...> (terraform-provider-aws / aws-sdk-go-v2 against the unified
// endpoint) by stripping the optional /apigateway leading segment first.
func extractPathParam(path, prefix string) string {
	path = strings.TrimPrefix(path, "/apigateway")

	return strings.TrimPrefix(path, strings.TrimPrefix(prefix, "/apigateway"))
}

// pathSegmentsAfterRestapis returns the URL path segments after the
// "restapis" segment, regardless of whether the path is prefixed with
// /apigateway/. Used by the various extract*Params helpers below.
func pathSegmentsAfterRestapis(path string) []string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, p := range parts {
		if p == "restapis" && i+1 < len(parts) {
			return parts[i+1:]
		}
	}

	return nil
}

// extractResourceParams extracts restApiId and parentId from the path
// /restapis/{restApiId}/resources/{parentId}.
func extractResourceParams(path string) (restAPIID, parentID string) {
	parts := pathSegmentsAfterRestapis(path)
	if len(parts) >= 3 {
		return parts[0], parts[2]
	}

	return "", ""
}

// extractRestAPIAndResourceID extracts restApiId and resourceId from
// /restapis/{restApiId}/resources/{resourceId}.
func extractRestAPIAndResourceID(path string) (restAPIID, resourceID string) {
	parts := pathSegmentsAfterRestapis(path)
	if len(parts) >= 3 {
		return parts[0], parts[2]
	}

	return "", ""
}

// extractMethodParams extracts restApiId, resourceId, and httpMethod from
// /restapis/{restApiId}/resources/{resourceId}/methods/{httpMethod}.
func extractMethodParams(path string) (restAPIID, resourceID, httpMethod string) {
	parts := pathSegmentsAfterRestapis(path)
	if len(parts) >= 5 {
		return parts[0], parts[2], parts[4]
	}

	return "", "", ""
}

// extractIntegrationParams extracts restApiId, resourceId, and httpMethod
// from /restapis/{restApiId}/resources/{resourceId}/methods/{httpMethod}/integration.
func extractIntegrationParams(path string) (restAPIID, resourceID, httpMethod string) {
	parts := pathSegmentsAfterRestapis(path)
	if len(parts) >= 6 {
		return parts[0], parts[2], parts[4]
	}

	return "", "", ""
}

// extractDeploymentRestAPIID extracts restApiId from
// /restapis/{restApiId}/deployments.
func extractDeploymentRestAPIID(path string) string {
	parts := pathSegmentsAfterRestapis(path)
	if len(parts) >= 1 {
		return parts[0]
	}

	return ""
}

// extractRestAPIAndDeploymentID extracts restApiId and deploymentId from
// /restapis/{restApiId}/deployments/{deploymentId}.
func extractRestAPIAndDeploymentID(path string) (restAPIID, deploymentID string) {
	parts := pathSegmentsAfterRestapis(path)
	if len(parts) >= 3 {
		return parts[0], parts[2]
	}

	return "", ""
}

// extractStageRestAPIID extracts restApiId from
// /restapis/{restApiId}/stages.
func extractStageRestAPIID(path string) string {
	parts := pathSegmentsAfterRestapis(path)
	if len(parts) >= 1 {
		return parts[0]
	}

	return ""
}

// extractRestAPIAndStageName extracts restApiId and stageName from
// /restapis/{restApiId}/stages/{stageName}.
func extractRestAPIAndStageName(path string) (restAPIID, stageName string) {
	parts := pathSegmentsAfterRestapis(path)
	if len(parts) >= 3 {
		return parts[0], parts[2]
	}

	return "", ""
}

// extractAuthorizerRestAPIID extracts restApiId from
// /restapis/{restApiId}/authorizers.
func extractAuthorizerRestAPIID(path string) string {
	parts := pathSegmentsAfterRestapis(path)
	if len(parts) >= 1 {
		return parts[0]
	}

	return ""
}

// extractRestAPIAndAuthorizerID extracts restApiId and authorizerId from
// /restapis/{restApiId}/authorizers/{authorizerId}.
func extractRestAPIAndAuthorizerID(path string) (restAPIID, authorizerID string) {
	parts := pathSegmentsAfterRestapis(path)
	if len(parts) >= 3 {
		return parts[0], parts[2]
	}

	return "", ""
}

// writeResponse writes a JSON response.
func writeResponse(w http.ResponseWriter, resp any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&ErrorResponse{Type: code, Message: message})
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
	case "NotFoundException":
		return http.StatusNotFound
	case "BadRequestException":
		return http.StatusBadRequest
	case "ConflictException":
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
