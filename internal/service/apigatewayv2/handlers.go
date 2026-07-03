package apigatewayv2

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ---- API handlers ----

// CreateAPI handles the CreateApi API.
func (s *Service) CreateAPI(w http.ResponseWriter, r *http.Request) {
	var req CreateAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeError(w, errBadRequest, "Name is required", http.StatusBadRequest)

		return
	}

	if req.ProtocolType == "" {
		writeError(w, errBadRequest, "ProtocolType is required", http.StatusBadRequest)

		return
	}

	api, err := s.storage.CreateAPI(r.Context(), &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, toAPIResponse(api), http.StatusCreated)
}

// GetAPI handles the GetApi API.
func (s *Service) GetAPI(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	api, err := s.storage.GetAPI(r.Context(), apiID)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, toAPIResponse(api), http.StatusOK)
}

// GetAPIs handles the GetApis API.
func (s *Service) GetAPIs(w http.ResponseWriter, r *http.Request) {
	apis, err := s.storage.GetAPIs(r.Context())
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]APIResponse, 0, len(apis))
	for _, api := range apis {
		items = append(items, *toAPIResponse(api))
	}

	writeResponse(w, &APIsResponse{Items: items}, http.StatusOK)
}

// UpdateAPI handles the UpdateApi API.
func (s *Service) UpdateAPI(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	var req UpdateAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	api, err := s.storage.UpdateAPI(r.Context(), apiID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, toAPIResponse(api), http.StatusOK)
}

// DeleteAPI handles the DeleteApi API.
func (s *Service) DeleteAPI(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	if err := s.storage.DeleteAPI(r.Context(), apiID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- Route handlers ----

// CreateRoute handles the CreateRoute API.
func (s *Service) CreateRoute(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	var req CreateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.RouteKey == "" {
		writeError(w, errBadRequest, "RouteKey is required", http.StatusBadRequest)

		return
	}

	route, err := s.storage.CreateRoute(r.Context(), apiID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, route, http.StatusCreated)
}

// GetRoute handles the GetRoute API.
func (s *Service) GetRoute(w http.ResponseWriter, r *http.Request) {
	apiID, routeID := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	route, err := s.storage.GetRoute(r.Context(), apiID, routeID)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, route, http.StatusOK)
}

// GetRoutes handles the GetRoutes API.
func (s *Service) GetRoutes(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	routes, err := s.storage.GetRoutes(r.Context(), apiID)
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]Route, 0, len(routes))
	for _, route := range routes {
		items = append(items, *route)
	}

	writeResponse(w, &RoutesResponse{Items: items}, http.StatusOK)
}

// UpdateRoute handles the UpdateRoute API.
func (s *Service) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	apiID, routeID := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	var req CreateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	route, err := s.storage.UpdateRoute(r.Context(), apiID, routeID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, route, http.StatusOK)
}

// DeleteRoute handles the DeleteRoute API.
func (s *Service) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	apiID, routeID := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	if err := s.storage.DeleteRoute(r.Context(), apiID, routeID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- Integration handlers ----

// CreateIntegration handles the CreateIntegration API.
func (s *Service) CreateIntegration(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	var req CreateIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.IntegrationType == "" {
		writeError(w, errBadRequest, "IntegrationType is required", http.StatusBadRequest)

		return
	}

	integration, err := s.storage.CreateIntegration(r.Context(), apiID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, integration, http.StatusCreated)
}

// GetIntegration handles the GetIntegration API.
func (s *Service) GetIntegration(w http.ResponseWriter, r *http.Request) {
	apiID, integrationID := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	integration, err := s.storage.GetIntegration(r.Context(), apiID, integrationID)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, integration, http.StatusOK)
}

// GetIntegrations handles the GetIntegrations API.
func (s *Service) GetIntegrations(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	integrations, err := s.storage.GetIntegrations(r.Context(), apiID)
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]Integration, 0, len(integrations))
	for _, integration := range integrations {
		items = append(items, *integration)
	}

	writeResponse(w, &IntegrationsResponse{Items: items}, http.StatusOK)
}

// UpdateIntegration handles the UpdateIntegration API.
func (s *Service) UpdateIntegration(w http.ResponseWriter, r *http.Request) {
	apiID, integrationID := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	var req CreateIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	integration, err := s.storage.UpdateIntegration(r.Context(), apiID, integrationID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, integration, http.StatusOK)
}

// DeleteIntegration handles the DeleteIntegration API.
func (s *Service) DeleteIntegration(w http.ResponseWriter, r *http.Request) {
	apiID, integrationID := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	if err := s.storage.DeleteIntegration(r.Context(), apiID, integrationID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- Stage handlers ----

// CreateStage handles the CreateStage API.
func (s *Service) CreateStage(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	var req CreateStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.StageName == "" {
		writeError(w, errBadRequest, "StageName is required", http.StatusBadRequest)

		return
	}

	stage, err := s.storage.CreateStage(r.Context(), apiID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, toStageResponse(stage), http.StatusCreated)
}

// GetStage handles the GetStage API.
func (s *Service) GetStage(w http.ResponseWriter, r *http.Request) {
	apiID, stageName := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	stage, err := s.storage.GetStage(r.Context(), apiID, stageName)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, toStageResponse(stage), http.StatusOK)
}

// GetStages handles the GetStages API.
func (s *Service) GetStages(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	stages, err := s.storage.GetStages(r.Context(), apiID)
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]StageResponse, 0, len(stages))
	for _, stage := range stages {
		items = append(items, *toStageResponse(stage))
	}

	writeResponse(w, &StagesResponse{Items: items}, http.StatusOK)
}

// UpdateStage handles the UpdateStage API.
func (s *Service) UpdateStage(w http.ResponseWriter, r *http.Request) {
	apiID, stageName := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	var req CreateStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	stage, err := s.storage.UpdateStage(r.Context(), apiID, stageName, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, toStageResponse(stage), http.StatusOK)
}

// DeleteStage handles the DeleteStage API.
func (s *Service) DeleteStage(w http.ResponseWriter, r *http.Request) {
	apiID, stageName := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	if err := s.storage.DeleteStage(r.Context(), apiID, stageName); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- Deployment handlers ----

// CreateDeployment handles the CreateDeployment API.
func (s *Service) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	var req CreateDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	deployment, err := s.storage.CreateDeployment(r.Context(), apiID, &req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, toDeploymentResponse(deployment), http.StatusCreated)
}

// GetDeployment handles the GetDeployment API.
func (s *Service) GetDeployment(w http.ResponseWriter, r *http.Request) {
	apiID, deploymentID := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	deployment, err := s.storage.GetDeployment(r.Context(), apiID, deploymentID)
	if err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, toDeploymentResponse(deployment), http.StatusOK)
}

// GetDeployments handles the GetDeployments API.
func (s *Service) GetDeployments(w http.ResponseWriter, r *http.Request) {
	apiID := segmentAt(r.URL.Path, 0)

	deployments, err := s.storage.GetDeployments(r.Context(), apiID)
	if err != nil {
		handleError(w, err)

		return
	}

	items := make([]DeploymentResponse, 0, len(deployments))
	for _, deployment := range deployments {
		items = append(items, *toDeploymentResponse(deployment))
	}

	writeResponse(w, &DeploymentsResponse{Items: items}, http.StatusOK)
}

// DeleteDeployment handles the DeleteDeployment API.
func (s *Service) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	apiID, deploymentID := segmentAt(r.URL.Path, 0), segmentAt(r.URL.Path, 2)

	if err := s.storage.DeleteDeployment(r.Context(), apiID, deploymentID); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- Tag handlers ----

// GetTags handles the GetTags API.
func (s *Service) GetTags(w http.ResponseWriter, r *http.Request) {
	arn := extractTagARN(r.URL.Path)

	tags, err := s.storage.GetTags(r.Context(), arn)
	if err != nil {
		handleError(w, err)

		return
	}

	if tags == nil {
		tags = map[string]string{}
	}

	writeResponse(w, &TagsResponse{Tags: tags}, http.StatusOK)
}

// TagResource handles the TagResource API.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	arn := extractTagARN(r.URL.Path)

	var req TagResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, errBadRequest, "Invalid request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagResource(r.Context(), arn, req.Tags); err != nil {
		handleError(w, err)

		return
	}

	writeResponse(w, struct{}{}, http.StatusCreated)
}

// UntagResource handles the UntagResource API.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	arn := extractTagARN(r.URL.Path)
	keys := r.URL.Query()["tagKeys"]

	if err := s.storage.UntagResource(r.Context(), arn, keys); err != nil {
		handleError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- Response conversions ----

// toAPIResponse converts an API to an APIResponse with an ISO8601 createdDate.
func toAPIResponse(api *API) *APIResponse {
	return &APIResponse{
		APIID:                     api.APIID,
		Name:                      api.Name,
		ProtocolType:              api.ProtocolType,
		Description:               api.Description,
		Version:                   api.Version,
		APIKeySelectionExpression: api.APIKeySelectionExpression,
		RouteSelectionExpression:  api.RouteSelectionExpression,
		DisableExecuteAPIEndpoint: api.DisableExecuteAPIEndpoint,
		DisableSchemaValidation:   api.DisableSchemaValidation,
		IPAddressType:             api.IPAddressType,
		CorsConfiguration:         api.CorsConfiguration,
		APIEndpoint:               api.APIEndpoint,
		APIGatewayManaged:         api.APIGatewayManaged,
		CreatedDate:               formatTime(api.CreatedDate),
		Tags:                      api.Tags,
	}
}

// toStageResponse converts a Stage to a StageResponse with ISO8601 dates.
func toStageResponse(stage *Stage) *StageResponse {
	return &StageResponse{
		StageName:                   stage.StageName,
		Description:                 stage.Description,
		DeploymentID:                stage.DeploymentID,
		ClientCertificateID:         stage.ClientCertificateID,
		DefaultRouteSettings:        stage.DefaultRouteSettings,
		RouteSettings:               stage.RouteSettings,
		StageVariables:              stage.StageVariables,
		AccessLogSettings:           stage.AccessLogSettings,
		AutoDeploy:                  stage.AutoDeploy,
		LastDeploymentStatusMessage: stage.LastDeploymentStatusMessage,
		APIGatewayManaged:           stage.APIGatewayManaged,
		CreatedDate:                 formatTime(stage.CreatedDate),
		LastUpdatedDate:             formatTime(stage.LastUpdatedDate),
		Tags:                        stage.Tags,
	}
}

// toDeploymentResponse converts a Deployment to a DeploymentResponse.
func toDeploymentResponse(d *Deployment) *DeploymentResponse {
	return &DeploymentResponse{
		DeploymentID:     d.DeploymentID,
		Description:      d.Description,
		DeploymentStatus: d.DeploymentStatus,
		AutoDeployed:     d.AutoDeployed,
		CreatedDate:      formatTime(d.CreatedDate),
	}
}

// formatTime renders a timestamp as an ISO8601 (RFC3339) string. The
// apigatewayv2 SDK deserializes createdDate/lastUpdatedDate with
// smithytime.ParseDateTime, so it must be a string rather than epoch seconds.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
}

// ---- Path helpers ----

// segmentAt returns the i-th URL path segment after the "apis" segment.
// For /v2/apis/{apiId}/routes/{routeId} segment 0 is apiId and segment 2 is
// routeId. Returns "" when the segment is absent.
func segmentAt(path string, i int) string {
	parts := pathSegmentsAfterApis(path)
	if i < 0 || i >= len(parts) {
		return ""
	}

	return parts[i]
}

// pathSegmentsAfterApis returns the URL path segments after the "apis"
// segment, tolerating both the /apigatewayv2 prefix and the bare /v2 path.
func pathSegmentsAfterApis(path string) []string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, p := range parts {
		if p == "apis" && i+1 < len(parts) {
			return parts[i+1:]
		}
	}

	return nil
}

// extractTagARN returns the resource ARN that follows the "/tags/" segment.
func extractTagARN(path string) string {
	const marker = "/tags/"

	idx := strings.Index(path, marker)
	if idx < 0 {
		return ""
	}

	return path[idx+len(marker):]
}

// parseResourceARN extracts the apiId and optional stageName from an
// API Gateway v2 ARN of the form
// arn:aws:apigateway:{region}::/apis/{apiId}[/stages/{stageName}].
func parseResourceARN(arn string) (apiID, stageName string) {
	const apisMarker = "/apis/"

	idx := strings.Index(arn, apisMarker)
	if idx < 0 {
		return "", ""
	}

	parts := strings.Split(arn[idx+len(apisMarker):], "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}

	apiID = parts[0]

	if len(parts) >= 3 && parts[1] == "stages" {
		stageName = parts[2]
	}

	return apiID, stageName
}

// ---- Response writers ----

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
	w.Header().Set("x-amzn-ErrorType", code)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&ErrorResponse{Message: message})
}

// handleError maps a service error to an HTTP error response.
func handleError(w http.ResponseWriter, err error) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		writeError(w, svcErr.Code, svcErr.Message, getErrorStatus(svcErr.Code))

		return
	}

	writeError(w, "InternalServiceError", err.Error(), http.StatusInternalServerError)
}

// getErrorStatus returns the HTTP status code for a given error code.
func getErrorStatus(code string) int {
	switch code {
	case errNotFound:
		return http.StatusNotFound
	case "ConflictException":
		return http.StatusConflict
	case errBadRequest:
		return http.StatusBadRequest
	default:
		return http.StatusBadRequest
	}
}
