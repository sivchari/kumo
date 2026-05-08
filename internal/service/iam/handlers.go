package iam

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Error codes.
const (
	errInvalidParameter = "InvalidParameterValue"
	errInternalError    = "InternalError"
	errInvalidAction    = "InvalidAction"
)

// CreateUser handles the CreateUser action.
func (s *Service) CreateUser(w http.ResponseWriter, r *http.Request) {
	userName := getFormValue(r, "UserName")
	if userName == "" {
		writeIAMError(w, errInvalidParameter, "UserName is required", http.StatusBadRequest)

		return
	}

	req := &CreateUserRequest{
		UserName: userName,
		Path:     getFormValue(r, "Path"),
		Tags:     parseTags(r),
	}

	user, err := s.storage.CreateUser(r.Context(), req)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, CreateUserResponse{
		CreateUserResult: CreateUserResult{User: *user},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteUser handles the DeleteUser action.
func (s *Service) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userName := getFormValue(r, "UserName")
	if userName == "" {
		writeIAMError(w, errInvalidParameter, "UserName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteUser(r.Context(), userName); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, DeleteUserResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// GetUser handles the GetUser action.
func (s *Service) GetUser(w http.ResponseWriter, r *http.Request) {
	userName := getFormValue(r, "UserName")
	if userName == "" {
		writeIAMError(w, errInvalidParameter, "UserName is required", http.StatusBadRequest)

		return
	}

	user, err := s.storage.GetUser(r.Context(), userName)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, GetUserResponse{
		GetUserResult:    GetUserResult{User: *user},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ListUsers handles the ListUsers action.
func (s *Service) ListUsers(w http.ResponseWriter, r *http.Request) {
	pathPrefix := getFormValue(r, "PathPrefix")
	maxItems := parseMaxItems(r)

	users, err := s.storage.ListUsers(r.Context(), pathPrefix, maxItems)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, ListUsersResponse{
		ListUsersResult:  ListUsersResult{Users: users, IsTruncated: false},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreateRole handles the CreateRole action.
func (s *Service) CreateRole(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	assumeRolePolicy := getFormValue(r, "AssumeRolePolicyDocument")
	if assumeRolePolicy == "" {
		writeIAMError(w, errInvalidParameter, "AssumeRolePolicyDocument is required", http.StatusBadRequest)

		return
	}

	req := &CreateRoleRequest{
		RoleName:                 roleName,
		AssumeRolePolicyDocument: assumeRolePolicy,
		Path:                     getFormValue(r, "Path"),
		Description:              getFormValue(r, "Description"),
		MaxSessionDuration:       parseIntValue(r, "MaxSessionDuration"),
		Tags:                     parseTags(r),
	}

	role, err := s.storage.CreateRole(r.Context(), req)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, CreateRoleResponse{
		CreateRoleResult: CreateRoleResult{Role: *role},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteRole handles the DeleteRole action.
func (s *Service) DeleteRole(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteRole(r.Context(), roleName); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, DeleteRoleResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// GetRole handles the GetRole action.
func (s *Service) GetRole(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	role, err := s.storage.GetRole(r.Context(), roleName)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, GetRoleResponse{
		GetRoleResult:    GetRoleResult{Role: *role},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ListRoles handles the ListRoles action.
func (s *Service) ListRoles(w http.ResponseWriter, r *http.Request) {
	pathPrefix := getFormValue(r, "PathPrefix")
	maxItems := parseMaxItems(r)

	roles, err := s.storage.ListRoles(r.Context(), pathPrefix, maxItems)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, ListRolesResponse{
		ListRolesResult:  ListRolesResult{Roles: roles, IsTruncated: false},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreatePolicy handles the CreatePolicy action.
func (s *Service) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	policyName := getFormValue(r, "PolicyName")
	if policyName == "" {
		writeIAMError(w, errInvalidParameter, "PolicyName is required", http.StatusBadRequest)

		return
	}

	policyDocument := getFormValue(r, "PolicyDocument")
	if policyDocument == "" {
		writeIAMError(w, errInvalidParameter, "PolicyDocument is required", http.StatusBadRequest)

		return
	}

	req := &CreatePolicyRequest{
		PolicyName:     policyName,
		PolicyDocument: policyDocument,
		Path:           getFormValue(r, "Path"),
		Description:    getFormValue(r, "Description"),
		Tags:           parseTags(r),
	}

	policy, err := s.storage.CreatePolicy(r.Context(), req)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, CreatePolicyResponse{
		CreatePolicyResult: CreatePolicyResult{Policy: *policy},
		ResponseMetadata:   ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeletePolicy handles the DeletePolicy action.
func (s *Service) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	policyArn := getFormValue(r, "PolicyArn")
	if policyArn == "" {
		writeIAMError(w, errInvalidParameter, "PolicyArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeletePolicy(r.Context(), policyArn); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, DeletePolicyResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// GetPolicy handles the GetPolicy action.
func (s *Service) GetPolicy(w http.ResponseWriter, r *http.Request) {
	policyArn := getFormValue(r, "PolicyArn")
	if policyArn == "" {
		writeIAMError(w, errInvalidParameter, "PolicyArn is required", http.StatusBadRequest)

		return
	}

	policy, err := s.storage.GetPolicy(r.Context(), policyArn)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, GetPolicyResponse{
		GetPolicyResult:  GetPolicyResult{Policy: *policy},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ListPolicies handles the ListPolicies action.
func (s *Service) ListPolicies(w http.ResponseWriter, r *http.Request) {
	pathPrefix := getFormValue(r, "PathPrefix")
	maxItems := parseMaxItems(r)
	onlyAttached := getFormValue(r, "OnlyAttached") == "true"

	policies, err := s.storage.ListPolicies(r.Context(), pathPrefix, maxItems, onlyAttached)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, ListPoliciesResponse{
		ListPoliciesResult: ListPoliciesResult{Policies: policies, IsTruncated: false},
		ResponseMetadata:   ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// AttachUserPolicy handles the AttachUserPolicy action.
func (s *Service) AttachUserPolicy(w http.ResponseWriter, r *http.Request) {
	userName := getFormValue(r, "UserName")
	if userName == "" {
		writeIAMError(w, errInvalidParameter, "UserName is required", http.StatusBadRequest)

		return
	}

	policyArn := getFormValue(r, "PolicyArn")
	if policyArn == "" {
		writeIAMError(w, errInvalidParameter, "PolicyArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.AttachUserPolicy(r.Context(), userName, policyArn); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, AttachUserPolicyResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DetachUserPolicy handles the DetachUserPolicy action.
func (s *Service) DetachUserPolicy(w http.ResponseWriter, r *http.Request) {
	userName := getFormValue(r, "UserName")
	if userName == "" {
		writeIAMError(w, errInvalidParameter, "UserName is required", http.StatusBadRequest)

		return
	}

	policyArn := getFormValue(r, "PolicyArn")
	if policyArn == "" {
		writeIAMError(w, errInvalidParameter, "PolicyArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DetachUserPolicy(r.Context(), userName, policyArn); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, DetachUserPolicyResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// AttachRolePolicy handles the AttachRolePolicy action.
func (s *Service) AttachRolePolicy(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	policyArn := getFormValue(r, "PolicyArn")
	if policyArn == "" {
		writeIAMError(w, errInvalidParameter, "PolicyArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.AttachRolePolicy(r.Context(), roleName, policyArn); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, AttachRolePolicyResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DetachRolePolicy handles the DetachRolePolicy action.
func (s *Service) DetachRolePolicy(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	policyArn := getFormValue(r, "PolicyArn")
	if policyArn == "" {
		writeIAMError(w, errInvalidParameter, "PolicyArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DetachRolePolicy(r.Context(), roleName, policyArn); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, DetachRolePolicyResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreateAccessKey handles the CreateAccessKey action.
func (s *Service) CreateAccessKey(w http.ResponseWriter, r *http.Request) {
	userName := getFormValue(r, "UserName")
	if userName == "" {
		writeIAMError(w, errInvalidParameter, "UserName is required", http.StatusBadRequest)

		return
	}

	accessKey, err := s.storage.CreateAccessKey(r.Context(), userName)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, CreateAccessKeyResponse{
		CreateAccessKeyResult: CreateAccessKeyResult{AccessKey: *accessKey},
		ResponseMetadata:      ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteAccessKey handles the DeleteAccessKey action.
func (s *Service) DeleteAccessKey(w http.ResponseWriter, r *http.Request) {
	userName := getFormValue(r, "UserName")
	if userName == "" {
		writeIAMError(w, errInvalidParameter, "UserName is required", http.StatusBadRequest)

		return
	}

	accessKeyID := getFormValue(r, "AccessKeyId")
	if accessKeyID == "" {
		writeIAMError(w, errInvalidParameter, "AccessKeyId is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteAccessKey(r.Context(), userName, accessKeyID); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, DeleteAccessKeyResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ListAccessKeys handles the ListAccessKeys action.
func (s *Service) ListAccessKeys(w http.ResponseWriter, r *http.Request) {
	userName := getFormValue(r, "UserName")
	if userName == "" {
		writeIAMError(w, errInvalidParameter, "UserName is required", http.StatusBadRequest)

		return
	}

	maxItems := parseMaxItems(r)

	keys, err := s.storage.ListAccessKeys(r.Context(), userName, maxItems)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, ListAccessKeysResponse{
		ListAccessKeysResult: ListAccessKeysResult{AccessKeyMetadata: keys, IsTruncated: false},
		ResponseMetadata:     ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ListInstanceProfilesForRole returns an empty list. Instance profiles are
// not modeled; AWS clients call this during role tear-down to detach profiles
// before deletion, so an empty list is the correct "no profiles" response.
func (s *Service) ListInstanceProfilesForRole(w http.ResponseWriter, _ *http.Request) {
	writeIAMXMLResponse(w, struct {
		XMLName                           xml.Name `xml:"ListInstanceProfilesForRoleResponse"`
		ListInstanceProfilesForRoleResult struct {
			InstanceProfiles struct{} `xml:"InstanceProfiles"`
			IsTruncated      bool     `xml:"IsTruncated"`
		} `xml:"ListInstanceProfilesForRoleResult"`
		ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
	}{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)

	handler, ok := s.actionHandlers[action]
	if !ok {
		writeIAMError(w, errInvalidAction, fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)

		return
	}

	handler(w, r)
}

// getFormValue extracts a form value from the request.
// It tries JSON body first, then form values.
func getFormValue(r *http.Request, key string) string {
	// Try to read from JSON body if content type is JSON.
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "application/x-amz-json") {
		return getJSONValue(r, key)
	}

	// Parse form if not already parsed.
	if r.Form == nil {
		_ = r.ParseForm()
	}

	return r.FormValue(key)
}

// getJSONValue extracts a value from JSON request body.
func getJSONValue(r *http.Request, key string) string {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return ""
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}

	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case string:
			return v
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(v)
		}
	}

	return ""
}

// parseMaxItems parses the MaxItems parameter.
func parseMaxItems(r *http.Request) int {
	maxItemsStr := getFormValue(r, "MaxItems")
	if maxItemsStr == "" {
		return 0
	}

	maxItems, err := strconv.Atoi(maxItemsStr)
	if err != nil {
		return 0
	}

	return maxItems
}

// parseIntValue parses an integer parameter.
func parseIntValue(r *http.Request, key string) int {
	valStr := getFormValue(r, key)
	if valStr == "" {
		return 0
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0
	}

	return val
}

// parseTags parses tags from the request.
func parseTags(r *http.Request) []Tag {
	var tags []Tag

	for i := 1; ; i++ {
		keyParam := fmt.Sprintf("Tags.member.%d.Key", i)
		valueParam := fmt.Sprintf("Tags.member.%d.Value", i)

		key := getFormValue(r, keyParam)
		if key == "" {
			break
		}

		value := getFormValue(r, valueParam)

		tags = append(tags, Tag{Key: key, Value: value})
	}

	return tags
}

// extractAction extracts the action name from the request.
func extractAction(r *http.Request) string {
	// Try X-Amz-Target header first.
	target := r.Header.Get("X-Amz-Target")
	if target != "" {
		if idx := strings.LastIndex(target, "."); idx >= 0 {
			return target[idx+1:]
		}
	}

	// Try form value.
	if r.Form == nil {
		_ = r.ParseForm()
	}

	return r.FormValue("Action")
}

// writeIAMXMLResponse writes an XML response.
func writeIAMXMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

// writeIAMError writes an IAM error response.
func writeIAMError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Type:    "Sender",
			Code:    code,
			Message: message,
		},
		ResponseMetadata: ResponseMetadata{RequestID: requestID},
	})
}

// handleIAMError handles IAM errors and writes the appropriate response.
func handleIAMError(w http.ResponseWriter, err error) {
	var iamErr *Error
	if errors.As(err, &iamErr) {
		status := http.StatusBadRequest

		switch iamErr.Code {
		case errNoSuchEntity:
			status = http.StatusNotFound
		case errDeleteConflict:
			status = http.StatusConflict
		case errLimitExceeded:
			status = http.StatusBadRequest
		}

		writeIAMError(w, iamErr.Code, iamErr.Message, status)

		return
	}

	writeIAMError(w, errInternalError, "Internal server error", http.StatusInternalServerError)
}
