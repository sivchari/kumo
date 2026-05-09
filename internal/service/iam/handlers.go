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

// UpdateRole handles the UpdateRole action. terraform aws_iam_role
// and alchemy's Role both call this on every apply after GetRole;
// without it those resources fail on second-and-later runs.
//
// AWS treats Description and MaxSessionDuration as optional — present
// fields update, missing fields preserve current state. We honour
// that with nil-pointer semantics on the storage layer.
func (s *Service) UpdateRole(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	var description *string

	if r.PostFormValue("Description") != "" || r.URL.Query().Get("Description") != "" {
		v := getFormValue(r, "Description")
		description = &v
	}

	var maxSession *int

	if raw := getFormValue(r, "MaxSessionDuration"); raw != "" {
		v := parseIntValue(r, "MaxSessionDuration")
		maxSession = &v
	}

	if err := s.storage.UpdateRole(r.Context(), roleName, description, maxSession); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, UpdateRoleResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// TagRole handles the TagRole action. terraform / pulumi /
// alchemy all use this to add `Name` / `alchemy_stage` /
// resource tracking tags after role creation.
func (s *Service) TagRole(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	tags := parseTags(r)
	if len(tags) == 0 {
		writeIAMError(w, errInvalidParameter, "Tags is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.TagRole(r.Context(), roleName, tags); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, TagRoleResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// UpdateAssumeRolePolicy handles the UpdateAssumeRolePolicy action.
// PolicyDocument is required and replaces the role's trust policy
// verbatim — same semantics PutRolePolicy has for inline policies.
func (s *Service) UpdateAssumeRolePolicy(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	policyDocument := getFormValue(r, "PolicyDocument")
	if policyDocument == "" {
		writeIAMError(w, errInvalidParameter, "PolicyDocument is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.UpdateAssumeRolePolicy(r.Context(), roleName, policyDocument); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, UpdateAssumeRolePolicyResponse{
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

// ListInstanceProfilesForRole returns the profiles attached to a role.
func (s *Service) ListInstanceProfilesForRole(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	profiles, err := s.storage.ListInstanceProfilesForRoleReal(r.Context(), roleName)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, ListInstanceProfilesForRoleResponseV2{
		ListInstanceProfilesForRoleResult: ListInstanceProfilesForRoleResultV2{InstanceProfiles: profiles},
		ResponseMetadata:                  ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreateInstanceProfile handles the CreateInstanceProfile action.
func (s *Service) CreateInstanceProfile(w http.ResponseWriter, r *http.Request) {
	name := getFormValue(r, "InstanceProfileName")
	if name == "" {
		writeIAMError(w, errInvalidParameter, "InstanceProfileName is required", http.StatusBadRequest)

		return
	}

	profile, err := s.storage.CreateInstanceProfile(r.Context(), name, getFormValue(r, "Path"))
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, CreateInstanceProfileResponse{
		CreateInstanceProfileResult: CreateInstanceProfileResult{InstanceProfile: *profile},
		ResponseMetadata:            ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteInstanceProfile handles the DeleteInstanceProfile action.
func (s *Service) DeleteInstanceProfile(w http.ResponseWriter, r *http.Request) {
	name := getFormValue(r, "InstanceProfileName")
	if name == "" {
		writeIAMError(w, errInvalidParameter, "InstanceProfileName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteInstanceProfile(r.Context(), name); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, DeleteInstanceProfileResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// GetInstanceProfile handles the GetInstanceProfile action.
func (s *Service) GetInstanceProfile(w http.ResponseWriter, r *http.Request) {
	name := getFormValue(r, "InstanceProfileName")
	if name == "" {
		writeIAMError(w, errInvalidParameter, "InstanceProfileName is required", http.StatusBadRequest)

		return
	}

	profile, err := s.storage.GetInstanceProfile(r.Context(), name)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, GetInstanceProfileResponse{
		GetInstanceProfileResult: GetInstanceProfileResult{InstanceProfile: *profile},
		ResponseMetadata:         ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ListInstanceProfiles handles the ListInstanceProfiles action.
func (s *Service) ListInstanceProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.storage.ListInstanceProfiles(r.Context(), getFormValue(r, "PathPrefix"), parseMaxItems(r))
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, ListInstanceProfilesResponse{
		ListInstanceProfilesResult: ListInstanceProfilesResult{InstanceProfiles: profiles},
		ResponseMetadata:           ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// AddRoleToInstanceProfile handles the AddRoleToInstanceProfile action.
func (s *Service) AddRoleToInstanceProfile(w http.ResponseWriter, r *http.Request) {
	profileName := getFormValue(r, "InstanceProfileName")
	roleName := getFormValue(r, "RoleName")

	if profileName == "" || roleName == "" {
		writeIAMError(w, errInvalidParameter, "InstanceProfileName and RoleName are required", http.StatusBadRequest)

		return
	}

	if err := s.storage.AddRoleToInstanceProfile(r.Context(), profileName, roleName); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, AddRoleToInstanceProfileResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// RemoveRoleFromInstanceProfile handles the RemoveRoleFromInstanceProfile action.
func (s *Service) RemoveRoleFromInstanceProfile(w http.ResponseWriter, r *http.Request) {
	profileName := getFormValue(r, "InstanceProfileName")
	roleName := getFormValue(r, "RoleName")

	if profileName == "" || roleName == "" {
		writeIAMError(w, errInvalidParameter, "InstanceProfileName and RoleName are required", http.StatusBadRequest)

		return
	}

	if err := s.storage.RemoveRoleFromInstanceProfile(r.Context(), profileName, roleName); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, RemoveRoleFromInstanceProfileResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// PutRolePolicy handles the PutRolePolicy action.
func (s *Service) PutRolePolicy(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	policyName := getFormValue(r, "PolicyName")
	policyDoc := getFormValue(r, "PolicyDocument")

	switch {
	case roleName == "":
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	case policyName == "":
		writeIAMError(w, errInvalidParameter, "PolicyName is required", http.StatusBadRequest)

		return
	case policyDoc == "":
		writeIAMError(w, errInvalidParameter, "PolicyDocument is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutRolePolicy(r.Context(), roleName, policyName, policyDoc); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, PutRolePolicyResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// GetRolePolicy handles the GetRolePolicy action.
func (s *Service) GetRolePolicy(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	policyName := getFormValue(r, "PolicyName")

	if roleName == "" || policyName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName and PolicyName are required", http.StatusBadRequest)

		return
	}

	doc, err := s.storage.GetRolePolicy(r.Context(), roleName, policyName)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, GetRolePolicyResponse{
		GetRolePolicyResult: GetRolePolicyResult{
			RoleName:       roleName,
			PolicyName:     policyName,
			PolicyDocument: doc,
		},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteRolePolicy handles the DeleteRolePolicy action.
func (s *Service) DeleteRolePolicy(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	policyName := getFormValue(r, "PolicyName")

	if roleName == "" || policyName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName and PolicyName are required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteRolePolicy(r.Context(), roleName, policyName); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, DeleteRolePolicyResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ListRolePolicies handles the ListRolePolicies action.
func (s *Service) ListRolePolicies(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	names, err := s.storage.ListRolePolicies(r.Context(), roleName)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, ListRolePoliciesResponse{
		ListRolePoliciesResult: ListRolePoliciesResult{PolicyNames: names, IsTruncated: false},
		ResponseMetadata:       ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ListAttachedRolePolicies handles the ListAttachedRolePolicies action.
func (s *Service) ListAttachedRolePolicies(w http.ResponseWriter, r *http.Request) {
	roleName := getFormValue(r, "RoleName")
	if roleName == "" {
		writeIAMError(w, errInvalidParameter, "RoleName is required", http.StatusBadRequest)

		return
	}

	attached, err := s.storage.ListAttachedRolePolicies(r.Context(), roleName)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, ListAttachedRolePoliciesResponse{
		ListAttachedRolePoliciesResult: ListAttachedRolePoliciesResult{
			AttachedPolicies: attached,
			IsTruncated:      false,
		},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreateOpenIDConnectProvider handles the CreateOpenIDConnectProvider action.
func (s *Service) CreateOpenIDConnectProvider(w http.ResponseWriter, r *http.Request) {
	url := getFormValue(r, "Url")
	if url == "" {
		writeIAMError(w, errInvalidParameter, "Url is required", http.StatusBadRequest)

		return
	}

	clientIDs := parseStringList(r, "ClientIDList")
	thumbprints := parseStringList(r, "ThumbprintList")

	provider, err := s.storage.CreateOIDCProvider(r.Context(), url, clientIDs, thumbprints)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, CreateOpenIDConnectProviderResponse{
		CreateOpenIDConnectProviderResult: CreateOpenIDConnectProviderResult{
			OpenIDConnectProviderArn: provider.Arn,
		},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// GetOpenIDConnectProvider handles the GetOpenIDConnectProvider action.
func (s *Service) GetOpenIDConnectProvider(w http.ResponseWriter, r *http.Request) {
	arn := getFormValue(r, "OpenIDConnectProviderArn")
	if arn == "" {
		writeIAMError(w, errInvalidParameter, "OpenIDConnectProviderArn is required", http.StatusBadRequest)

		return
	}

	provider, err := s.storage.GetOIDCProvider(r.Context(), arn)
	if err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, GetOpenIDConnectProviderResponse{
		GetOpenIDConnectProviderResult: GetOpenIDConnectProviderResult{
			URL:            provider.URL,
			ClientIDList:   provider.ClientIDList,
			ThumbprintList: provider.ThumbprintList,
			CreateDate:     provider.CreateDate,
		},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteOpenIDConnectProvider handles the DeleteOpenIDConnectProvider action.
func (s *Service) DeleteOpenIDConnectProvider(w http.ResponseWriter, r *http.Request) {
	arn := getFormValue(r, "OpenIDConnectProviderArn")
	if arn == "" {
		writeIAMError(w, errInvalidParameter, "OpenIDConnectProviderArn is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.DeleteOIDCProvider(r.Context(), arn); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, DeleteOpenIDConnectProviderResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ListOpenIDConnectProviders handles the ListOpenIDConnectProviders action.
func (s *Service) ListOpenIDConnectProviders(w http.ResponseWriter, r *http.Request) {
	arns, err := s.storage.ListOIDCProviders(r.Context())
	if err != nil {
		handleIAMError(w, err)

		return
	}

	entries := make([]OIDCProviderEntry, 0, len(arns))
	for _, arn := range arns {
		entries = append(entries, OIDCProviderEntry{Arn: arn})
	}

	writeIAMXMLResponse(w, ListOpenIDConnectProvidersResponse{
		ListOpenIDConnectProvidersResult: ListOpenIDConnectProvidersResult{
			OpenIDConnectProviderList: entries,
		},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// UpdateOpenIDConnectProviderThumbprint handles the
// UpdateOpenIDConnectProviderThumbprint action.
func (s *Service) UpdateOpenIDConnectProviderThumbprint(w http.ResponseWriter, r *http.Request) {
	arn := getFormValue(r, "OpenIDConnectProviderArn")
	if arn == "" {
		writeIAMError(w, errInvalidParameter, "OpenIDConnectProviderArn is required", http.StatusBadRequest)

		return
	}

	thumbprints := parseStringList(r, "ThumbprintList")
	if len(thumbprints) == 0 {
		writeIAMError(w, errInvalidParameter, "ThumbprintList is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.UpdateOIDCProviderThumbprint(r.Context(), arn, thumbprints); err != nil {
		handleIAMError(w, err)

		return
	}

	writeIAMXMLResponse(w, UpdateOpenIDConnectProviderThumbprintResponse{
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// parseStringList reads "<name>.member.N" entries (the IAM convention) into a
// list, stopping at the first missing index.
func parseStringList(r *http.Request, name string) []string {
	var out []string

	for i := 1; ; i++ {
		v := getFormValue(r, fmt.Sprintf("%s.member.%d", name, i))
		if v == "" {
			break
		}

		out = append(out, v)
	}

	return out
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
