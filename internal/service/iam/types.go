package iam

import "time"

// User represents an IAM user.
type User struct {
	UserName         string           `xml:"UserName"`
	UserID           string           `xml:"UserId"`
	Arn              string           `xml:"Arn"`
	Path             string           `xml:"Path"`
	CreateDate       time.Time        `xml:"CreateDate"`
	PasswordLastUsed *time.Time       `xml:"PasswordLastUsed,omitempty"`
	Tags             []Tag            `xml:"Tags>member,omitempty"`
	AttachedPolicies []AttachedPolicy `xml:"-"`
}

// Role represents an IAM role.
type Role struct {
	RoleName                 string            `xml:"RoleName"`
	RoleID                   string            `xml:"RoleId"`
	Arn                      string            `xml:"Arn"`
	Path                     string            `xml:"Path"`
	CreateDate               time.Time         `xml:"CreateDate"`
	AssumeRolePolicyDocument string            `xml:"AssumeRolePolicyDocument"`
	Description              string            `xml:"Description,omitempty"`
	MaxSessionDuration       int               `xml:"MaxSessionDuration,omitempty"`
	Tags                     []Tag             `xml:"Tags>member,omitempty"`
	AttachedPolicies         []AttachedPolicy  `xml:"-"`
	InlinePolicies           map[string]string `xml:"-"`
}

// OIDCProvider represents an IAM OpenID Connect provider.
type OIDCProvider struct {
	Arn            string    `xml:"Arn"`
	URL            string    `xml:"Url"`
	ClientIDList   []string  `xml:"ClientIDList>member,omitempty"`
	ThumbprintList []string  `xml:"ThumbprintList>member,omitempty"`
	CreateDate     time.Time `xml:"CreateDate"`
}

// InstanceProfile represents an IAM instance profile.
type InstanceProfile struct {
	InstanceProfileName string    `xml:"InstanceProfileName"`
	InstanceProfileID   string    `xml:"InstanceProfileId"`
	Arn                 string    `xml:"Arn"`
	Path                string    `xml:"Path"`
	CreateDate          time.Time `xml:"CreateDate"`
	Roles               []Role    `xml:"Roles>member,omitempty"`
	Tags                []Tag     `xml:"Tags>member,omitempty"`
}

// CreateInstanceProfileResponse is the response for CreateInstanceProfile.
type CreateInstanceProfileResponse struct {
	CreateInstanceProfileResult CreateInstanceProfileResult `xml:"CreateInstanceProfileResult"`
	ResponseMetadata            ResponseMetadata            `xml:"ResponseMetadata"`
}

// CreateInstanceProfileResult contains the new InstanceProfile.
type CreateInstanceProfileResult struct {
	InstanceProfile InstanceProfile `xml:"InstanceProfile"`
}

// GetInstanceProfileResponse is the response for GetInstanceProfile.
type GetInstanceProfileResponse struct {
	GetInstanceProfileResult GetInstanceProfileResult `xml:"GetInstanceProfileResult"`
	ResponseMetadata         ResponseMetadata         `xml:"ResponseMetadata"`
}

// GetInstanceProfileResult contains the looked-up InstanceProfile.
type GetInstanceProfileResult struct {
	InstanceProfile InstanceProfile `xml:"InstanceProfile"`
}

// DeleteInstanceProfileResponse is the response for DeleteInstanceProfile.
type DeleteInstanceProfileResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// ListInstanceProfilesResponse is the response for ListInstanceProfiles.
type ListInstanceProfilesResponse struct {
	ListInstanceProfilesResult ListInstanceProfilesResult `xml:"ListInstanceProfilesResult"`
	ResponseMetadata           ResponseMetadata           `xml:"ResponseMetadata"`
}

// ListInstanceProfilesResult contains the list of InstanceProfiles.
type ListInstanceProfilesResult struct {
	InstanceProfiles []InstanceProfile `xml:"InstanceProfiles>member"`
	IsTruncated      bool              `xml:"IsTruncated"`
	Marker           string            `xml:"Marker,omitempty"`
}

// ListInstanceProfilesForRoleResultV2 is the real response for
// ListInstanceProfilesForRole replacing the empty stub.
type ListInstanceProfilesForRoleResultV2 struct {
	InstanceProfiles []InstanceProfile `xml:"InstanceProfiles>member"`
	IsTruncated      bool              `xml:"IsTruncated"`
	Marker           string            `xml:"Marker,omitempty"`
}

// ListInstanceProfilesForRoleResponseV2 wraps the real response.
type ListInstanceProfilesForRoleResponseV2 struct {
	ListInstanceProfilesForRoleResult ListInstanceProfilesForRoleResultV2 `xml:"ListInstanceProfilesForRoleResult"`
	ResponseMetadata                  ResponseMetadata                    `xml:"ResponseMetadata"`
}

// AddRoleToInstanceProfileResponse is the response for AddRoleToInstanceProfile.
type AddRoleToInstanceProfileResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// RemoveRoleFromInstanceProfileResponse is the response for
// RemoveRoleFromInstanceProfile.
type RemoveRoleFromInstanceProfileResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// Policy represents an IAM policy.
type Policy struct {
	PolicyName       string    `xml:"PolicyName"`
	PolicyID         string    `xml:"PolicyId"`
	Arn              string    `xml:"Arn"`
	Path             string    `xml:"Path"`
	DefaultVersionID string    `xml:"DefaultVersionId"`
	AttachmentCount  int       `xml:"AttachmentCount"`
	IsAttachable     bool      `xml:"IsAttachable"`
	CreateDate       time.Time `xml:"CreateDate"`
	UpdateDate       time.Time `xml:"UpdateDate"`
	Description      string    `xml:"Description,omitempty"`
	Tags             []Tag     `xml:"Tags>member,omitempty"`
	PolicyDocument   string    `xml:"-"`
}

// AttachedPolicy represents a policy attached to a user or role.
type AttachedPolicy struct {
	PolicyName string `xml:"PolicyName"`
	PolicyArn  string `xml:"PolicyArn"`
}

// AccessKey represents an IAM access key.
type AccessKey struct {
	AccessKeyID     string    `xml:"AccessKeyId"`
	SecretAccessKey string    `xml:"SecretAccessKey,omitempty"`
	Status          string    `xml:"Status"`
	UserName        string    `xml:"UserName"`
	CreateDate      time.Time `xml:"CreateDate"`
}

// AccessKeyMetadata represents metadata for an access key (without secret).
type AccessKeyMetadata struct {
	AccessKeyID string    `xml:"AccessKeyId"`
	Status      string    `xml:"Status"`
	UserName    string    `xml:"UserName"`
	CreateDate  time.Time `xml:"CreateDate"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// Request types.

// CreateUserRequest represents a CreateUser request.
type CreateUserRequest struct {
	UserName string `xml:"UserName"`
	Path     string `xml:"Path"`
	Tags     []Tag  `xml:"Tags>member"`
}

// DeleteUserRequest represents a DeleteUser request.
type DeleteUserRequest struct {
	UserName string `xml:"UserName"`
}

// GetUserRequest represents a GetUser request.
type GetUserRequest struct {
	UserName string `xml:"UserName"`
}

// ListUsersRequest represents a ListUsers request.
type ListUsersRequest struct {
	PathPrefix string `xml:"PathPrefix"`
	Marker     string `xml:"Marker"`
	MaxItems   int    `xml:"MaxItems"`
}

// CreateRoleRequest represents a CreateRole request.
type CreateRoleRequest struct {
	RoleName                 string `xml:"RoleName"`
	AssumeRolePolicyDocument string `xml:"AssumeRolePolicyDocument"`
	Path                     string `xml:"Path"`
	Description              string `xml:"Description"`
	MaxSessionDuration       int    `xml:"MaxSessionDuration"`
	Tags                     []Tag  `xml:"Tags>member"`
}

// DeleteRoleRequest represents a DeleteRole request.
type DeleteRoleRequest struct {
	RoleName string `xml:"RoleName"`
}

// GetRoleRequest represents a GetRole request.
type GetRoleRequest struct {
	RoleName string `xml:"RoleName"`
}

// ListRolesRequest represents a ListRoles request.
type ListRolesRequest struct {
	PathPrefix string `xml:"PathPrefix"`
	Marker     string `xml:"Marker"`
	MaxItems   int    `xml:"MaxItems"`
}

// CreatePolicyRequest represents a CreatePolicy request.
type CreatePolicyRequest struct {
	PolicyName     string `xml:"PolicyName"`
	PolicyDocument string `xml:"PolicyDocument"`
	Path           string `xml:"Path"`
	Description    string `xml:"Description"`
	Tags           []Tag  `xml:"Tags>member"`
}

// DeletePolicyRequest represents a DeletePolicy request.
type DeletePolicyRequest struct {
	PolicyArn string `xml:"PolicyArn"`
}

// GetPolicyRequest represents a GetPolicy request.
type GetPolicyRequest struct {
	PolicyArn string `xml:"PolicyArn"`
}

// ListPoliciesRequest represents a ListPolicies request.
type ListPoliciesRequest struct {
	Scope             string `xml:"Scope"`
	OnlyAttached      bool   `xml:"OnlyAttached"`
	PathPrefix        string `xml:"PathPrefix"`
	PolicyUsageFilter string `xml:"PolicyUsageFilter"`
	Marker            string `xml:"Marker"`
	MaxItems          int    `xml:"MaxItems"`
}

// AttachUserPolicyRequest represents an AttachUserPolicy request.
type AttachUserPolicyRequest struct {
	UserName  string `xml:"UserName"`
	PolicyArn string `xml:"PolicyArn"`
}

// DetachUserPolicyRequest represents a DetachUserPolicy request.
type DetachUserPolicyRequest struct {
	UserName  string `xml:"UserName"`
	PolicyArn string `xml:"PolicyArn"`
}

// AttachRolePolicyRequest represents an AttachRolePolicy request.
type AttachRolePolicyRequest struct {
	RoleName  string `xml:"RoleName"`
	PolicyArn string `xml:"PolicyArn"`
}

// DetachRolePolicyRequest represents a DetachRolePolicy request.
type DetachRolePolicyRequest struct {
	RoleName  string `xml:"RoleName"`
	PolicyArn string `xml:"PolicyArn"`
}

// CreateAccessKeyRequest represents a CreateAccessKey request.
type CreateAccessKeyRequest struct {
	UserName string `xml:"UserName"`
}

// DeleteAccessKeyRequest represents a DeleteAccessKey request.
type DeleteAccessKeyRequest struct {
	UserName    string `xml:"UserName"`
	AccessKeyID string `xml:"AccessKeyId"`
}

// ListAccessKeysRequest represents a ListAccessKeys request.
type ListAccessKeysRequest struct {
	UserName string `xml:"UserName"`
	Marker   string `xml:"Marker"`
	MaxItems int    `xml:"MaxItems"`
}

// Response types.

// CreateUserResponse represents a CreateUser response.
type CreateUserResponse struct {
	CreateUserResult CreateUserResult `xml:"CreateUserResult"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// CreateUserResult contains the result of CreateUser.
type CreateUserResult struct {
	User User `xml:"User"`
}

// DeleteUserResponse represents a DeleteUser response.
type DeleteUserResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// GetUserResponse represents a GetUser response.
type GetUserResponse struct {
	GetUserResult    GetUserResult    `xml:"GetUserResult"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// GetUserResult contains the result of GetUser.
type GetUserResult struct {
	User User `xml:"User"`
}

// ListUsersResponse represents a ListUsers response.
type ListUsersResponse struct {
	ListUsersResult  ListUsersResult  `xml:"ListUsersResult"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// ListUsersResult contains the result of ListUsers.
type ListUsersResult struct {
	Users       []User `xml:"Users>member"`
	IsTruncated bool   `xml:"IsTruncated"`
	Marker      string `xml:"Marker,omitempty"`
}

// CreateRoleResponse represents a CreateRole response.
type CreateRoleResponse struct {
	CreateRoleResult CreateRoleResult `xml:"CreateRoleResult"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// CreateRoleResult contains the result of CreateRole.
type CreateRoleResult struct {
	Role Role `xml:"Role"`
}

// DeleteRoleResponse represents a DeleteRole response.
type DeleteRoleResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// GetRoleResponse represents a GetRole response.
type GetRoleResponse struct {
	GetRoleResult    GetRoleResult    `xml:"GetRoleResult"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// GetRoleResult contains the result of GetRole.
type GetRoleResult struct {
	Role Role `xml:"Role"`
}

// ListRolesResponse represents a ListRoles response.
type ListRolesResponse struct {
	ListRolesResult  ListRolesResult  `xml:"ListRolesResult"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// ListRolesResult contains the result of ListRoles.
type ListRolesResult struct {
	Roles       []Role `xml:"Roles>member"`
	IsTruncated bool   `xml:"IsTruncated"`
	Marker      string `xml:"Marker,omitempty"`
}

// CreatePolicyResponse represents a CreatePolicy response.
type CreatePolicyResponse struct {
	CreatePolicyResult CreatePolicyResult `xml:"CreatePolicyResult"`
	ResponseMetadata   ResponseMetadata   `xml:"ResponseMetadata"`
}

// CreatePolicyResult contains the result of CreatePolicy.
type CreatePolicyResult struct {
	Policy Policy `xml:"Policy"`
}

// DeletePolicyResponse represents a DeletePolicy response.
type DeletePolicyResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// GetPolicyResponse represents a GetPolicy response.
type GetPolicyResponse struct {
	GetPolicyResult  GetPolicyResult  `xml:"GetPolicyResult"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// GetPolicyResult contains the result of GetPolicy.
type GetPolicyResult struct {
	Policy Policy `xml:"Policy"`
}

// ListPoliciesResponse represents a ListPolicies response.
type ListPoliciesResponse struct {
	ListPoliciesResult ListPoliciesResult `xml:"ListPoliciesResult"`
	ResponseMetadata   ResponseMetadata   `xml:"ResponseMetadata"`
}

// ListPoliciesResult contains the result of ListPolicies.
type ListPoliciesResult struct {
	Policies    []Policy `xml:"Policies>member"`
	IsTruncated bool     `xml:"IsTruncated"`
	Marker      string   `xml:"Marker,omitempty"`
}

// AttachUserPolicyResponse represents an AttachUserPolicy response.
type AttachUserPolicyResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// DetachUserPolicyResponse represents a DetachUserPolicy response.
type DetachUserPolicyResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// AttachRolePolicyResponse represents an AttachRolePolicy response.
type AttachRolePolicyResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// DetachRolePolicyResponse represents a DetachRolePolicy response.
type DetachRolePolicyResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// CreateAccessKeyResponse represents a CreateAccessKey response.
type CreateAccessKeyResponse struct {
	CreateAccessKeyResult CreateAccessKeyResult `xml:"CreateAccessKeyResult"`
	ResponseMetadata      ResponseMetadata      `xml:"ResponseMetadata"`
}

// CreateAccessKeyResult contains the result of CreateAccessKey.
type CreateAccessKeyResult struct {
	AccessKey AccessKey `xml:"AccessKey"`
}

// DeleteAccessKeyResponse represents a DeleteAccessKey response.
type DeleteAccessKeyResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// ListAccessKeysResponse represents a ListAccessKeys response.
type ListAccessKeysResponse struct {
	ListAccessKeysResult ListAccessKeysResult `xml:"ListAccessKeysResult"`
	ResponseMetadata     ResponseMetadata     `xml:"ResponseMetadata"`
}

// ListAccessKeysResult contains the result of ListAccessKeys.
type ListAccessKeysResult struct {
	AccessKeyMetadata []AccessKeyMetadata `xml:"AccessKeyMetadata>member"`
	IsTruncated       bool                `xml:"IsTruncated"`
	Marker            string              `xml:"Marker,omitempty"`
}

// PutRolePolicyRequest represents a PutRolePolicy request.
type PutRolePolicyRequest struct {
	RoleName       string `xml:"RoleName"`
	PolicyName     string `xml:"PolicyName"`
	PolicyDocument string `xml:"PolicyDocument"`
}

// PutRolePolicyResponse represents a PutRolePolicy response.
type PutRolePolicyResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// GetRolePolicyResponse represents a GetRolePolicy response.
type GetRolePolicyResponse struct {
	GetRolePolicyResult GetRolePolicyResult `xml:"GetRolePolicyResult"`
	ResponseMetadata    ResponseMetadata    `xml:"ResponseMetadata"`
}

// GetRolePolicyResult contains the result of GetRolePolicy.
type GetRolePolicyResult struct {
	RoleName       string `xml:"RoleName"`
	PolicyName     string `xml:"PolicyName"`
	PolicyDocument string `xml:"PolicyDocument"`
}

// DeleteRolePolicyResponse represents a DeleteRolePolicy response.
type DeleteRolePolicyResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// ListRolePoliciesResponse represents a ListRolePolicies response.
type ListRolePoliciesResponse struct {
	ListRolePoliciesResult ListRolePoliciesResult `xml:"ListRolePoliciesResult"`
	ResponseMetadata       ResponseMetadata       `xml:"ResponseMetadata"`
}

// ListRolePoliciesResult contains the result of ListRolePolicies.
type ListRolePoliciesResult struct {
	PolicyNames []string `xml:"PolicyNames>member"`
	IsTruncated bool     `xml:"IsTruncated"`
	Marker      string   `xml:"Marker,omitempty"`
}

// ListAttachedRolePoliciesResponse represents a ListAttachedRolePolicies response.
type ListAttachedRolePoliciesResponse struct {
	ListAttachedRolePoliciesResult ListAttachedRolePoliciesResult `xml:"ListAttachedRolePoliciesResult"`
	ResponseMetadata               ResponseMetadata               `xml:"ResponseMetadata"`
}

// ListAttachedRolePoliciesResult contains the result of ListAttachedRolePolicies.
type ListAttachedRolePoliciesResult struct {
	AttachedPolicies []AttachedPolicy `xml:"AttachedPolicies>member"`
	IsTruncated      bool             `xml:"IsTruncated"`
	Marker           string           `xml:"Marker,omitempty"`
}

// CreateOpenIDConnectProviderResponse is the response for CreateOpenIDConnectProvider.
type CreateOpenIDConnectProviderResponse struct {
	CreateOpenIDConnectProviderResult CreateOpenIDConnectProviderResult `xml:"CreateOpenIDConnectProviderResult"`
	ResponseMetadata                  ResponseMetadata                  `xml:"ResponseMetadata"`
}

// CreateOpenIDConnectProviderResult contains the OIDC provider ARN.
type CreateOpenIDConnectProviderResult struct {
	OpenIDConnectProviderArn string `xml:"OpenIDConnectProviderArn"`
}

// GetOpenIDConnectProviderResponse is the response for GetOpenIDConnectProvider.
type GetOpenIDConnectProviderResponse struct {
	GetOpenIDConnectProviderResult GetOpenIDConnectProviderResult `xml:"GetOpenIDConnectProviderResult"`
	ResponseMetadata               ResponseMetadata               `xml:"ResponseMetadata"`
}

// GetOpenIDConnectProviderResult contains the OIDC provider details.
type GetOpenIDConnectProviderResult struct {
	URL            string    `xml:"Url"`
	ClientIDList   []string  `xml:"ClientIDList>member,omitempty"`
	ThumbprintList []string  `xml:"ThumbprintList>member,omitempty"`
	CreateDate     time.Time `xml:"CreateDate"`
}

// DeleteOpenIDConnectProviderResponse is the response for DeleteOpenIDConnectProvider.
type DeleteOpenIDConnectProviderResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// ListOpenIDConnectProvidersResponse is the response for ListOpenIDConnectProviders.
type ListOpenIDConnectProvidersResponse struct {
	ListOpenIDConnectProvidersResult ListOpenIDConnectProvidersResult `xml:"ListOpenIDConnectProvidersResult"`
	ResponseMetadata                 ResponseMetadata                 `xml:"ResponseMetadata"`
}

// ListOpenIDConnectProvidersResult contains the list of OIDC provider ARNs.
type ListOpenIDConnectProvidersResult struct {
	OpenIDConnectProviderList []OIDCProviderEntry `xml:"OpenIDConnectProviderList>member"`
}

// OIDCProviderEntry is a single entry in the OIDC provider list.
type OIDCProviderEntry struct {
	Arn string `xml:"Arn"`
}

// UpdateOpenIDConnectProviderThumbprintResponse is the response for
// UpdateOpenIDConnectProviderThumbprint.
type UpdateOpenIDConnectProviderThumbprintResponse struct {
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// ResponseMetadata contains the request ID.
type ResponseMetadata struct {
	RequestID string `xml:"RequestId"`
}

// ErrorResponse represents an IAM error response.
type ErrorResponse struct {
	Error            ErrorDetail      `xml:"Error"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// ErrorDetail contains the error details.
type ErrorDetail struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// Error represents an IAM error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}
