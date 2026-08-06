package resiliencehub

import (
	"encoding/json"
	"errors"
	"net/http"
)

// CreateApp handles the CreateApp API.
func (s *Service) CreateApp(w http.ResponseWriter, r *http.Request) {
	var req CreateAppRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.Name, "Name") {
		return
	}

	app, err := s.storage.CreateApp(&req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &CreateAppResponse{App: app})
}

// DescribeApp handles the DescribeApp API.
func (s *Service) DescribeApp(w http.ResponseWriter, r *http.Request) {
	var req DescribeAppRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.AppARN, "appArn") {
		return
	}

	app, err := s.storage.DescribeApp(req.AppARN)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DescribeAppResponse{App: app})
}

// UpdateApp handles the UpdateApp API.
func (s *Service) UpdateApp(w http.ResponseWriter, r *http.Request) {
	var req UpdateAppRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.AppARN, "appArn") {
		return
	}

	app, err := s.storage.UpdateApp(&req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &UpdateAppResponse{App: app})
}

// DeleteApp handles the DeleteApp API.
func (s *Service) DeleteApp(w http.ResponseWriter, r *http.Request) {
	var req DeleteAppRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.AppARN, "appArn") {
		return
	}

	if err := s.storage.DeleteApp(req.AppARN); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DeleteAppResponse{AppARN: req.AppARN})
}

// ListApps handles the ListApps API.
func (s *Service) ListApps(w http.ResponseWriter, r *http.Request) {
	var req ListAppsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// For list operations, empty body is acceptable
		req = ListAppsRequest{}
	}

	apps, nextToken, err := s.storage.ListApps(&req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListAppsResponse{
		AppSummaries: apps,
		NextToken:    nextToken,
	})
}

// CreateResiliencyPolicy handles the CreateResiliencyPolicy API.
func (s *Service) CreateResiliencyPolicy(w http.ResponseWriter, r *http.Request) {
	var req CreateResiliencyPolicyRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.PolicyName, "policyName") {
		return
	}

	if !requireResilienceHubParameter(w, req.Tier, "tier") {
		return
	}

	policy, err := s.storage.CreateResiliencyPolicy(&req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &CreateResiliencyPolicyResponse{Policy: policy})
}

// DescribeResiliencyPolicy handles the DescribeResiliencyPolicy API.
func (s *Service) DescribeResiliencyPolicy(w http.ResponseWriter, r *http.Request) {
	var req DescribeResiliencyPolicyRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.PolicyARN, "policyArn") {
		return
	}

	policy, err := s.storage.DescribeResiliencyPolicy(req.PolicyARN)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DescribeResiliencyPolicyResponse{Policy: policy})
}

// UpdateResiliencyPolicy handles the UpdateResiliencyPolicy API.
func (s *Service) UpdateResiliencyPolicy(w http.ResponseWriter, r *http.Request) {
	var req UpdateResiliencyPolicyRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.PolicyARN, "policyArn") {
		return
	}

	policy, err := s.storage.UpdateResiliencyPolicy(&req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &UpdateResiliencyPolicyResponse{Policy: policy})
}

// DeleteResiliencyPolicy handles the DeleteResiliencyPolicy API.
func (s *Service) DeleteResiliencyPolicy(w http.ResponseWriter, r *http.Request) {
	var req DeleteResiliencyPolicyRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.PolicyARN, "policyArn") {
		return
	}

	if err := s.storage.DeleteResiliencyPolicy(req.PolicyARN); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DeleteResiliencyPolicyResponse{PolicyARN: req.PolicyARN})
}

// ListResiliencyPolicies handles the ListResiliencyPolicies API.
func (s *Service) ListResiliencyPolicies(w http.ResponseWriter, r *http.Request) {
	var req ListResiliencyPoliciesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// For list operations, empty body is acceptable
		req = ListResiliencyPoliciesRequest{}
	}

	policies, nextToken, err := s.storage.ListResiliencyPolicies(&req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListResiliencyPoliciesResponse{
		ResiliencyPolicies: policies,
		NextToken:          nextToken,
	})
}

// StartAppAssessment handles the StartAppAssessment API.
func (s *Service) StartAppAssessment(w http.ResponseWriter, r *http.Request) {
	var req StartAppAssessmentRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.AppARN, "appArn") {
		return
	}

	if !requireResilienceHubParameter(w, req.AppVersion, "appVersion") {
		return
	}

	if !requireResilienceHubParameter(w, req.AssessmentName, "assessmentName") {
		return
	}

	assessment, err := s.storage.StartAppAssessment(&req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &StartAppAssessmentResponse{Assessment: assessment})
}

// DescribeAppAssessment handles the DescribeAppAssessment API.
func (s *Service) DescribeAppAssessment(w http.ResponseWriter, r *http.Request) {
	var req DescribeAppAssessmentRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.AssessmentARN, "assessmentArn") {
		return
	}

	assessment, err := s.storage.DescribeAppAssessment(req.AssessmentARN)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DescribeAppAssessmentResponse{Assessment: assessment})
}

// DeleteAppAssessment handles the DeleteAppAssessment API.
func (s *Service) DeleteAppAssessment(w http.ResponseWriter, r *http.Request) {
	var req DeleteAppAssessmentRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.AssessmentARN, "assessmentArn") {
		return
	}

	if err := s.storage.DeleteAppAssessment(req.AssessmentARN); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &DeleteAppAssessmentResponse{
		AssessmentARN:    req.AssessmentARN,
		AssessmentStatus: "Success",
	})
}

// ListAppAssessments handles the ListAppAssessments API.
func (s *Service) ListAppAssessments(w http.ResponseWriter, r *http.Request) {
	var req ListAppAssessmentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// For list operations, empty body is acceptable
		req = ListAppAssessmentsRequest{}
	}

	assessments, nextToken, err := s.storage.ListAppAssessments(&req)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListAppAssessmentsResponse{
		AssessmentSummaries: assessments,
		NextToken:           nextToken,
	})
}

// TagResource handles the TagResource API.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	var req TagResourceRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.ResourceARN, "resourceArn") {
		return
	}

	if err := s.storage.TagResource(req.ResourceARN, req.Tags); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &TagResourceResponse{})
}

// UntagResource handles the UntagResource API.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	var req UntagResourceRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.ResourceARN, "resourceArn") {
		return
	}

	if err := s.storage.UntagResource(req.ResourceARN, req.TagKeys); err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &UntagResourceResponse{})
}

// ListTagsForResource handles the ListTagsForResource API.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	var req ListTagsForResourceRequest
	if !decodeResilienceHubRequest(w, r, &req) {
		return
	}

	if !requireResilienceHubParameter(w, req.ResourceARN, "resourceArn") {
		return
	}

	tags, err := s.storage.ListTagsForResource(req.ResourceARN)
	if err != nil {
		handleError(w, err)

		return
	}

	writeJSON(w, &ListTagsForResourceResponse{Tags: tags})
}

// decodeResilienceHubRequest decodes the JSON request body into req, writing
// the standard ResilienceHub decode-failure error and returning false on
// failure.
func decodeResilienceHubRequest(w http.ResponseWriter, r *http.Request, req any) bool {
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    "ValidationException",
			Message: "Invalid request body",
		})

		return false
	}

	return true
}

// requireResilienceHubParameter writes the standard ResilienceHub
// ValidationException error and returns false if value is empty.
func requireResilienceHubParameter(w http.ResponseWriter, value, name string) bool {
	if value == "" {
		writeError(w, http.StatusBadRequest, &Error{
			Code:    "ValidationException",
			Message: name + " is required",
		})

		return false
	}

	return true
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, statusCode int, e *Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(e)
}

// handleError handles storage errors.
func handleError(w http.ResponseWriter, err error) {
	var e *Error
	if errors.As(err, &e) {
		var statusCode int

		switch e.Code {
		case errResourceNotFound:
			statusCode = http.StatusNotFound
		case errConflict:
			statusCode = http.StatusConflict
		default:
			statusCode = http.StatusBadRequest
		}

		writeError(w, statusCode, e)

		return
	}

	writeError(w, http.StatusInternalServerError, &Error{
		Code:    "InternalServerException",
		Message: err.Error(),
	})
}
