package verifiedpermissions

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	targetPrefix    = "VerifiedPermissions."
	contentTypeJSON = "application/x-amz-json-1.0"
)

// Error codes returned to AVP clients.
const (
	errResourceNotFound = "ResourceNotFoundException"
	errValidation       = "ValidationException"
	errInvalidAction    = "InvalidAction"
	errInternal         = "InternalServerException"
)

// handlerFunc is the signature of an action handler.
type handlerFunc func(http.ResponseWriter, *http.Request)

// getActionHandlers maps action names to their handlers.
func (s *Service) getActionHandlers() map[string]handlerFunc {
	return map[string]handlerFunc{
		"CreatePolicyStore":    s.CreatePolicyStore,
		"GetPolicyStore":       s.GetPolicyStore,
		"DeletePolicyStore":    s.DeletePolicyStore,
		"ListPolicyStores":     s.ListPolicyStores,
		"PutSchema":            s.PutSchema,
		"GetSchema":            s.GetSchema,
		"CreatePolicy":         s.CreatePolicy,
		"GetPolicy":            s.GetPolicy,
		"ListPolicies":         s.ListPolicies,
		"UpdatePolicy":         s.UpdatePolicy,
		"DeletePolicy":         s.DeletePolicy,
		"CreateIdentitySource": s.CreateIdentitySource,
		"GetIdentitySource":    s.GetIdentitySource,
		"ListIdentitySources":  s.ListIdentitySources,
		"DeleteIdentitySource": s.DeleteIdentitySource,
		"IsAuthorized":         s.IsAuthorized,
		"ListTagsForResource":  s.ListTagsForResource,
		"TagResource":          s.TagResource,
		"UntagResource":        s.UntagResource,
	}
}

// DispatchAction routes a JSON 1.0 request to the handler named by the
// X-Amz-Target header suffix.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimPrefix(r.Header.Get("X-Amz-Target"), targetPrefix)

	handler, ok := s.getActionHandlers()[action]
	if !ok {
		writeVPError(w, errInvalidAction, "the action "+action+" is not valid", http.StatusBadRequest)

		return
	}

	handler(w, r)
}

// CreatePolicyStore handles the CreatePolicyStore action.
func (s *Service) CreatePolicyStore(w http.ResponseWriter, r *http.Request) {
	var req CreatePolicyStoreRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	mode := ""
	if req.ValidationSettings != nil {
		mode = req.ValidationSettings.Mode
	}

	store := s.storage.CreatePolicyStore(mode, req.Description)

	if len(req.Tags) > 0 {
		s.storage.TagResource(store.ARN, req.Tags)
	}

	writeJSONResponse(w, CreatePolicyStoreResponse{
		PolicyStoreID:   store.ID,
		ARN:             store.ARN,
		CreatedDate:     formatTime(store.CreatedDate),
		LastUpdatedDate: formatTime(store.LastUpdatedDate),
	})
}

// GetPolicyStore handles the GetPolicyStore action.
func (s *Service) GetPolicyStore(w http.ResponseWriter, r *http.Request) {
	var req GetPolicyStoreRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	store, err := s.storage.GetPolicyStore(req.PolicyStoreID)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSONResponse(w, GetPolicyStoreResponse{
		PolicyStoreID:      store.ID,
		ARN:                store.ARN,
		ValidationSettings: &ValidationSettings{Mode: store.ValidationMode},
		Description:        store.Description,
		CreatedDate:        formatTime(store.CreatedDate),
		LastUpdatedDate:    formatTime(store.LastUpdatedDate),
	})
}

// DeletePolicyStore handles the DeletePolicyStore action.
func (s *Service) DeletePolicyStore(w http.ResponseWriter, r *http.Request) {
	var req DeletePolicyStoreRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeletePolicyStore(req.PolicyStoreID); err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// ListPolicyStores handles the ListPolicyStores action.
func (s *Service) ListPolicyStores(w http.ResponseWriter, r *http.Request) {
	var req ListPolicyStoresRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	stores := s.storage.ListPolicyStores()
	items := make([]PolicyStoreItem, 0, len(stores))

	for _, store := range stores {
		items = append(items, PolicyStoreItem{
			PolicyStoreID:   store.ID,
			ARN:             store.ARN,
			CreatedDate:     formatTime(store.CreatedDate),
			LastUpdatedDate: formatTime(store.LastUpdatedDate),
		})
	}

	writeJSONResponse(w, ListPolicyStoresResponse{PolicyStores: items})
}

// PutSchema handles the PutSchema action.
func (s *Service) PutSchema(w http.ResponseWriter, r *http.Request) {
	var req PutSchemaRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	document := ""
	if req.Definition != nil {
		document = req.Definition.CedarJSON
	}

	schema, err := s.storage.PutSchema(req.PolicyStoreID, document)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSONResponse(w, PutSchemaResponse{
		PolicyStoreID:   schema.PolicyStoreID,
		Namespaces:      schema.Namespaces,
		CreatedDate:     formatTime(schema.CreatedDate),
		LastUpdatedDate: formatTime(schema.LastUpdatedDate),
	})
}

// GetSchema handles the GetSchema action.
func (s *Service) GetSchema(w http.ResponseWriter, r *http.Request) {
	var req GetSchemaRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	schema, err := s.storage.GetSchema(req.PolicyStoreID)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSONResponse(w, GetSchemaResponse{
		PolicyStoreID:   schema.PolicyStoreID,
		Schema:          schema.Document,
		Namespaces:      schema.Namespaces,
		CreatedDate:     formatTime(schema.CreatedDate),
		LastUpdatedDate: formatTime(schema.LastUpdatedDate),
	})
}

// CreatePolicy handles the CreatePolicy action.
func (s *Service) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	mutatePolicy(w, r,
		func(req *CreatePolicyRequest) *PolicyDefinition { return req.Definition },
		func(req *CreatePolicyRequest, static *StaticPolicyDefinition) (*Policy, error) {
			return s.storage.CreatePolicy(req.PolicyStoreID, static.Description, static.Statement)
		},
	)
}

// GetPolicy handles the GetPolicy action.
func (s *Service) GetPolicy(w http.ResponseWriter, r *http.Request) {
	var req GetPolicyRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	policy, err := s.storage.GetPolicy(req.PolicyStoreID, req.PolicyID)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSONResponse(w, GetPolicyResponse{
		PolicyStoreID: policy.PolicyStoreID,
		PolicyID:      policy.ID,
		PolicyType:    policy.PolicyType,
		Definition: &PolicyDefinition{Static: &StaticPolicyDefinition{
			Description: policy.Description,
			Statement:   policy.Statement,
		}},
		CreatedDate:     formatTime(policy.CreatedDate),
		LastUpdatedDate: formatTime(policy.LastUpdatedDate),
	})
}

// ListPolicies handles the ListPolicies action.
func (s *Service) ListPolicies(w http.ResponseWriter, r *http.Request) {
	listScoped(w, r,
		func(req *ListPoliciesRequest) string { return req.PolicyStoreID },
		s.storage.ListPolicies,
		toPolicyItem,
		func(items []PolicyItem) ListPoliciesResponse { return ListPoliciesResponse{Policies: items} },
	)
}

// toPolicyItem maps a stored policy to its ListPolicies item.
func toPolicyItem(policy *Policy) PolicyItem {
	return PolicyItem{
		PolicyStoreID:   policy.PolicyStoreID,
		PolicyID:        policy.ID,
		PolicyType:      policy.PolicyType,
		CreatedDate:     formatTime(policy.CreatedDate),
		LastUpdatedDate: formatTime(policy.LastUpdatedDate),
	}
}

// UpdatePolicy handles the UpdatePolicy action.
func (s *Service) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	mutatePolicy(w, r,
		func(req *UpdatePolicyRequest) *PolicyDefinition { return req.Definition },
		func(req *UpdatePolicyRequest, static *StaticPolicyDefinition) (*Policy, error) {
			return s.storage.UpdatePolicy(req.PolicyStoreID, req.PolicyID, static.Description, static.Statement)
		},
	)
}

// DeletePolicy handles the DeletePolicy action.
func (s *Service) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	var req DeletePolicyRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeletePolicy(req.PolicyStoreID, req.PolicyID); err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// CreateIdentitySource handles the CreateIdentitySource action.
func (s *Service) CreateIdentitySource(w http.ResponseWriter, r *http.Request) {
	var req CreateIdentitySourceRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	var (
		userPoolARN string
		clientIDs   []string
	)

	if req.Configuration != nil && req.Configuration.CognitoUserPoolConfiguration != nil {
		userPoolARN = req.Configuration.CognitoUserPoolConfiguration.UserPoolARN
		clientIDs = req.Configuration.CognitoUserPoolConfiguration.ClientIDs
	}

	src, err := s.storage.CreateIdentitySource(req.PolicyStoreID, req.PrincipalEntityType, userPoolARN, clientIDs)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSONResponse(w, CreateIdentitySourceResponse{
		IdentitySourceID: src.ID,
		PolicyStoreID:    src.PolicyStoreID,
		CreatedDate:      formatTime(src.CreatedDate),
		LastUpdatedDate:  formatTime(src.LastUpdatedDate),
	})
}

// GetIdentitySource handles the GetIdentitySource action.
func (s *Service) GetIdentitySource(w http.ResponseWriter, r *http.Request) {
	var req GetIdentitySourceRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	src, err := s.storage.GetIdentitySource(req.PolicyStoreID, req.IdentitySourceID)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	resp := GetIdentitySourceResponse{
		PolicyStoreID:       src.PolicyStoreID,
		IdentitySourceID:    src.ID,
		PrincipalEntityType: src.PrincipalEntityType,
		CreatedDate:         formatTime(src.CreatedDate),
		LastUpdatedDate:     formatTime(src.LastUpdatedDate),
	}

	if src.UserPoolARN != "" {
		resp.Configuration = &IdentitySourceConfiguration{
			CognitoUserPoolConfiguration: &CognitoUserPoolConfiguration{
				UserPoolARN: src.UserPoolARN,
				ClientIDs:   src.ClientIDs,
			},
		}
	}

	writeJSONResponse(w, resp)
}

// ListIdentitySources handles the ListIdentitySources action.
func (s *Service) ListIdentitySources(w http.ResponseWriter, r *http.Request) {
	listScoped(w, r,
		func(req *ListIdentitySourcesRequest) string { return req.PolicyStoreID },
		s.storage.ListIdentitySources,
		toIdentitySourceItem,
		func(items []IdentitySourceItem) ListIdentitySourcesResponse {
			return ListIdentitySourcesResponse{IdentitySources: items}
		},
	)
}

// toIdentitySourceItem maps a stored identity source to its list item.
func toIdentitySourceItem(src *IdentitySource) IdentitySourceItem {
	return IdentitySourceItem{
		PolicyStoreID:       src.PolicyStoreID,
		IdentitySourceID:    src.ID,
		PrincipalEntityType: src.PrincipalEntityType,
		CreatedDate:         formatTime(src.CreatedDate),
		LastUpdatedDate:     formatTime(src.LastUpdatedDate),
	}
}

// DeleteIdentitySource handles the DeleteIdentitySource action.
func (s *Service) DeleteIdentitySource(w http.ResponseWriter, r *http.Request) {
	var req DeleteIdentitySourceRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteIdentitySource(req.PolicyStoreID, req.IdentitySourceID); err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSONResponse(w, struct{}{})
}

// IsAuthorized handles the IsAuthorized action, evaluating the store's Cedar
// policies against the principal / action / resource / context.
func (s *Service) IsAuthorized(w http.ResponseWriter, r *http.Request) {
	var req IsAuthorizedRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.PolicyStoreID == "" {
		writeVPError(w, errValidation, "policyStoreId is required", http.StatusBadRequest)

		return
	}

	policies, err := s.storage.PoliciesFor(req.PolicyStoreID)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	ps, err := buildPolicySet(policies)
	if err != nil {
		writeVPError(w, errValidation, err.Error(), http.StatusBadRequest)

		return
	}

	creq, err := buildRequest(&req)
	if err != nil {
		writeVPError(w, errValidation, err.Error(), http.StatusBadRequest)

		return
	}

	decision, determining, errs := decide(ps, &creq)

	writeJSONResponse(w, IsAuthorizedResponse{
		Decision:            decision,
		DeterminingPolicies: determining,
		Errors:              errs,
	})
}

// ListTagsForResource handles the ListTagsForResource action. An unknown ARN
// returns an empty tag map rather than an error, so the Terraform provider's
// read-time tag fetch stays stable even when no tags were ever set.
func (s *Service) ListTagsForResource(w http.ResponseWriter, r *http.Request) {
	var req ListTagsForResourceRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceARN == "" {
		writeVPError(w, errValidation, "resourceArn is required", http.StatusBadRequest)

		return
	}

	writeJSONResponse(w, ListTagsForResourceResponse{Tags: s.storage.ListTags(req.ResourceARN)})
}

// TagResource handles the TagResource action.
func (s *Service) TagResource(w http.ResponseWriter, r *http.Request) {
	var req TagResourceRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceARN == "" {
		writeVPError(w, errValidation, "resourceArn is required", http.StatusBadRequest)

		return
	}

	s.storage.TagResource(req.ResourceARN, req.Tags)

	writeJSONResponse(w, struct{}{})
}

// UntagResource handles the UntagResource action.
func (s *Service) UntagResource(w http.ResponseWriter, r *http.Request) {
	var req UntagResourceRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ResourceARN == "" {
		writeVPError(w, errValidation, "resourceArn is required", http.StatusBadRequest)

		return
	}

	s.storage.UntagResource(req.ResourceARN, req.TagKeys)

	writeJSONResponse(w, struct{}{})
}

// notFound builds a ResourceNotFoundException service error.
func notFound(message string) *ServiceError {
	return &ServiceError{Code: errResourceNotFound, Message: message, Status: http.StatusNotFound}
}

// readJSONRequest decodes the request body into v, tolerating an empty body.
func readJSONRequest(r *http.Request, v any) error {
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

// writeJSONResponse writes a 200 JSON 1.0 response.
func writeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// writeVPError writes a JSON 1.0 error response.
func writeVPError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Type: code, Message: message})
}

// writeServiceError maps a typed ServiceError onto an AVP error response,
// falling back to an internal error for anything else.
func writeServiceError(w http.ResponseWriter, err error) {
	var se *ServiceError
	if errors.As(err, &se) {
		writeVPError(w, se.Code, se.Message, se.Status)

		return
	}

	writeVPError(w, errInternal, "internal server error", http.StatusInternalServerError)
}

// mutatePolicy handles the shared shape of the CreatePolicy/UpdatePolicy
// actions: decode the request, require and validate the static Cedar
// definition, run the storage mutation, and write the policy response.
func mutatePolicy[Req any](
	w http.ResponseWriter,
	r *http.Request,
	definition func(*Req) *PolicyDefinition,
	mutate func(*Req, *StaticPolicyDefinition) (*Policy, error),
) {
	var req Req
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	def := definition(&req)
	if def == nil || def.Static == nil {
		writeVPError(w, errValidation, "definition.static is required", http.StatusBadRequest)

		return
	}

	if err := validateStatement(def.Static.Statement); err != nil {
		writeVPError(w, errValidation, err.Error(), http.StatusBadRequest)

		return
	}

	policy, err := mutate(&req, def.Static)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSONResponse(w, toPolicyMutationResponse(policy))
}

// toPolicyMutationResponse maps a stored policy to the Create/Update response.
func toPolicyMutationResponse(policy *Policy) PolicyMutationResponse {
	return PolicyMutationResponse{
		PolicyStoreID:   policy.PolicyStoreID,
		PolicyID:        policy.ID,
		PolicyType:      policy.PolicyType,
		CreatedDate:     formatTime(policy.CreatedDate),
		LastUpdatedDate: formatTime(policy.LastUpdatedDate),
	}
}

// listScoped handles the shared shape of the store-scoped List* actions: decode
// the request, resolve its policy store id, fetch the items, map each one, and
// write the response. Decode and storage errors are reported uniformly.
func listScoped[Req, Src, Item, Resp any](
	w http.ResponseWriter,
	r *http.Request,
	storeID func(*Req) string,
	fetch func(string) ([]Src, error),
	mapItem func(Src) Item,
	wrap func([]Item) Resp,
) {
	var req Req
	if err := readJSONRequest(r, &req); err != nil {
		writeVPError(w, errValidation, "failed to parse request body", http.StatusBadRequest)

		return
	}

	srcs, err := fetch(storeID(&req))
	if err != nil {
		writeServiceError(w, err)

		return
	}

	items := make([]Item, 0, len(srcs))
	for _, src := range srcs {
		items = append(items, mapItem(src))
	}

	writeJSONResponse(w, wrap(items))
}

// formatTime renders a time in AVP's ISO8601 (RFC3339) date-time
// representation. Verified Permissions models its timestamps as date-time
// strings, not the epoch-seconds numbers other AWS JSON services use.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
