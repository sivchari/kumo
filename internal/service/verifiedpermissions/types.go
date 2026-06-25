package verifiedpermissions

import "time"

// PolicyStore is a single Cedar policy store held in memory.
type PolicyStore struct {
	ID              string    `json:"id"`
	ARN             string    `json:"arn"`
	ValidationMode  string    `json:"validationMode"`
	Description     string    `json:"description,omitempty"`
	CreatedDate     time.Time `json:"createdDate"`
	LastUpdatedDate time.Time `json:"lastUpdatedDate"`
}

// Schema is the Cedar schema document registered for a policy store.
type Schema struct {
	PolicyStoreID   string    `json:"policyStoreId"`
	Document        string    `json:"document"`
	Namespaces      []string  `json:"namespaces"`
	CreatedDate     time.Time `json:"createdDate"`
	LastUpdatedDate time.Time `json:"lastUpdatedDate"`
}

// Policy is a static Cedar policy. The statement is stored as Cedar text and
// parsed on demand during evaluation.
type Policy struct {
	ID              string    `json:"id"`
	PolicyStoreID   string    `json:"policyStoreId"`
	PolicyType      string    `json:"policyType"`
	Description     string    `json:"description,omitempty"`
	Statement       string    `json:"statement"`
	CreatedDate     time.Time `json:"createdDate"`
	LastUpdatedDate time.Time `json:"lastUpdatedDate"`
}

// IdentitySource is a Cognito user pool identity source registered for parity.
type IdentitySource struct {
	ID                  string    `json:"id"`
	PolicyStoreID       string    `json:"policyStoreId"`
	PrincipalEntityType string    `json:"principalEntityType"`
	UserPoolARN         string    `json:"userPoolArn"`
	ClientIDs           []string  `json:"clientIds"`
	CreatedDate         time.Time `json:"createdDate"`
	LastUpdatedDate     time.Time `json:"lastUpdatedDate"`
}

// errorResponse is the AWS JSON protocol error envelope.
type errorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError is a typed error carrying the AVP error code and HTTP status.
type ServiceError struct {
	Code    string
	Message string
	Status  int
}

// Error implements the error interface.
func (e *ServiceError) Error() string {
	return e.Code + ": " + e.Message
}

// EntityIdentifier identifies a Cedar entity by type and id.
type EntityIdentifier struct {
	EntityType string `json:"entityType"`
	EntityID   string `json:"entityId"`
}

// ActionIdentifier identifies a Cedar action by type and id.
type ActionIdentifier struct {
	ActionType string `json:"actionType"`
	ActionID   string `json:"actionId"`
}

// AttributeValue is the AVP context value union. Exactly one field is set,
// mirroring the Smithy union the AWS SDK serializes.
type AttributeValue struct {
	String           *string                   `json:"string,omitempty"`
	Long             *int64                    `json:"long,omitempty"`
	Boolean          *bool                     `json:"boolean,omitempty"`
	Record           map[string]AttributeValue `json:"record,omitempty"`
	Set              []AttributeValue          `json:"set,omitempty"`
	EntityIdentifier *EntityIdentifier         `json:"entityIdentifier,omitempty"`
}

// ContextDefinition carries the request context map for IsAuthorized.
type ContextDefinition struct {
	ContextMap map[string]AttributeValue `json:"contextMap,omitempty"`
}

// ValidationSettings holds the schema validation mode of a policy store.
type ValidationSettings struct {
	Mode string `json:"mode"`
}

// CreatePolicyStoreRequest is the input for CreatePolicyStore.
type CreatePolicyStoreRequest struct {
	ValidationSettings *ValidationSettings `json:"validationSettings,omitempty"`
	Description        string              `json:"description,omitempty"`
	ClientToken        string              `json:"clientToken,omitempty"`
	Tags               map[string]string   `json:"tags,omitempty"`
}

// CreatePolicyStoreResponse is the output for CreatePolicyStore.
type CreatePolicyStoreResponse struct {
	PolicyStoreID   string `json:"policyStoreId"`
	ARN             string `json:"arn"`
	CreatedDate     string `json:"createdDate"`
	LastUpdatedDate string `json:"lastUpdatedDate"`
}

// GetPolicyStoreRequest is the input for GetPolicyStore.
type GetPolicyStoreRequest struct {
	PolicyStoreID string `json:"policyStoreId"`
}

// GetPolicyStoreResponse is the output for GetPolicyStore.
type GetPolicyStoreResponse struct {
	PolicyStoreID      string              `json:"policyStoreId"`
	ARN                string              `json:"arn"`
	ValidationSettings *ValidationSettings `json:"validationSettings,omitempty"`
	Description        string              `json:"description,omitempty"`
	CreatedDate        string              `json:"createdDate"`
	LastUpdatedDate    string              `json:"lastUpdatedDate"`
}

// DeletePolicyStoreRequest is the input for DeletePolicyStore.
type DeletePolicyStoreRequest struct {
	PolicyStoreID string `json:"policyStoreId"`
}

// ListPolicyStoresRequest is the input for ListPolicyStores.
type ListPolicyStoresRequest struct {
	MaxResults int32  `json:"maxResults,omitempty"`
	NextToken  string `json:"nextToken,omitempty"`
}

// PolicyStoreItem is a single entry in the ListPolicyStores result.
type PolicyStoreItem struct {
	PolicyStoreID   string `json:"policyStoreId"`
	ARN             string `json:"arn"`
	CreatedDate     string `json:"createdDate"`
	LastUpdatedDate string `json:"lastUpdatedDate"`
}

// ListPolicyStoresResponse is the output for ListPolicyStores.
type ListPolicyStoresResponse struct {
	PolicyStores []PolicyStoreItem `json:"policyStores"`
	NextToken    *string           `json:"nextToken,omitempty"`
}

// SchemaDefinition wraps the Cedar schema JSON document.
type SchemaDefinition struct {
	CedarJSON string `json:"cedarJson,omitempty"`
}

// PutSchemaRequest is the input for PutSchema.
type PutSchemaRequest struct {
	PolicyStoreID string            `json:"policyStoreId"`
	Definition    *SchemaDefinition `json:"definition,omitempty"`
}

// PutSchemaResponse is the output for PutSchema.
type PutSchemaResponse struct {
	PolicyStoreID   string   `json:"policyStoreId"`
	Namespaces      []string `json:"namespaces"`
	CreatedDate     string   `json:"createdDate"`
	LastUpdatedDate string   `json:"lastUpdatedDate"`
}

// GetSchemaRequest is the input for GetSchema.
type GetSchemaRequest struct {
	PolicyStoreID string `json:"policyStoreId"`
}

// GetSchemaResponse is the output for GetSchema.
type GetSchemaResponse struct {
	PolicyStoreID   string   `json:"policyStoreId"`
	Schema          string   `json:"schema"`
	Namespaces      []string `json:"namespaces"`
	CreatedDate     string   `json:"createdDate"`
	LastUpdatedDate string   `json:"lastUpdatedDate"`
}

// StaticPolicyDefinition is a static Cedar policy body.
type StaticPolicyDefinition struct {
	Description string `json:"description,omitempty"`
	Statement   string `json:"statement"`
}

// PolicyDefinition wraps a static policy definition.
type PolicyDefinition struct {
	Static *StaticPolicyDefinition `json:"static,omitempty"`
}

// CreatePolicyRequest is the input for CreatePolicy.
type CreatePolicyRequest struct {
	PolicyStoreID string            `json:"policyStoreId"`
	Definition    *PolicyDefinition `json:"definition,omitempty"`
	ClientToken   string            `json:"clientToken,omitempty"`
}

// PolicyMutationResponse is the output for CreatePolicy and UpdatePolicy, which
// return the same policy-metadata shape.
type PolicyMutationResponse struct {
	PolicyStoreID   string `json:"policyStoreId"`
	PolicyID        string `json:"policyId"`
	PolicyType      string `json:"policyType"`
	CreatedDate     string `json:"createdDate"`
	LastUpdatedDate string `json:"lastUpdatedDate"`
}

// GetPolicyRequest is the input for GetPolicy.
type GetPolicyRequest struct {
	PolicyStoreID string `json:"policyStoreId"`
	PolicyID      string `json:"policyId"`
}

// GetPolicyResponse is the output for GetPolicy.
type GetPolicyResponse struct {
	PolicyStoreID   string            `json:"policyStoreId"`
	PolicyID        string            `json:"policyId"`
	PolicyType      string            `json:"policyType"`
	Definition      *PolicyDefinition `json:"definition,omitempty"`
	CreatedDate     string            `json:"createdDate"`
	LastUpdatedDate string            `json:"lastUpdatedDate"`
}

// ListPoliciesRequest is the input for ListPolicies.
type ListPoliciesRequest struct {
	PolicyStoreID string `json:"policyStoreId"`
	MaxResults    int32  `json:"maxResults,omitempty"`
	NextToken     string `json:"nextToken,omitempty"`
}

// PolicyItem is a single entry in the ListPolicies result.
type PolicyItem struct {
	PolicyStoreID   string `json:"policyStoreId"`
	PolicyID        string `json:"policyId"`
	PolicyType      string `json:"policyType"`
	CreatedDate     string `json:"createdDate"`
	LastUpdatedDate string `json:"lastUpdatedDate"`
}

// ListPoliciesResponse is the output for ListPolicies.
type ListPoliciesResponse struct {
	Policies  []PolicyItem `json:"policies"`
	NextToken *string      `json:"nextToken,omitempty"`
}

// UpdatePolicyRequest is the input for UpdatePolicy.
type UpdatePolicyRequest struct {
	PolicyStoreID string            `json:"policyStoreId"`
	PolicyID      string            `json:"policyId"`
	Definition    *PolicyDefinition `json:"definition,omitempty"`
}

// DeletePolicyRequest is the input for DeletePolicy.
type DeletePolicyRequest struct {
	PolicyStoreID string `json:"policyStoreId"`
	PolicyID      string `json:"policyId"`
}

// CognitoUserPoolConfiguration configures a Cognito identity source.
type CognitoUserPoolConfiguration struct {
	UserPoolARN string   `json:"userPoolArn"`
	ClientIDs   []string `json:"clientIds,omitempty"`
}

// IdentitySourceConfiguration wraps an identity source provider configuration.
type IdentitySourceConfiguration struct {
	CognitoUserPoolConfiguration *CognitoUserPoolConfiguration `json:"cognitoUserPoolConfiguration,omitempty"`
}

// CreateIdentitySourceRequest is the input for CreateIdentitySource.
type CreateIdentitySourceRequest struct {
	PolicyStoreID       string                       `json:"policyStoreId"`
	PrincipalEntityType string                       `json:"principalEntityType,omitempty"`
	Configuration       *IdentitySourceConfiguration `json:"configuration,omitempty"`
	ClientToken         string                       `json:"clientToken,omitempty"`
}

// CreateIdentitySourceResponse is the output for CreateIdentitySource.
type CreateIdentitySourceResponse struct {
	IdentitySourceID string `json:"identitySourceId"`
	PolicyStoreID    string `json:"policyStoreId"`
	CreatedDate      string `json:"createdDate"`
	LastUpdatedDate  string `json:"lastUpdatedDate"`
}

// GetIdentitySourceRequest is the input for GetIdentitySource.
type GetIdentitySourceRequest struct {
	PolicyStoreID    string `json:"policyStoreId"`
	IdentitySourceID string `json:"identitySourceId"`
}

// GetIdentitySourceResponse is the output for GetIdentitySource.
type GetIdentitySourceResponse struct {
	PolicyStoreID       string                       `json:"policyStoreId"`
	IdentitySourceID    string                       `json:"identitySourceId"`
	PrincipalEntityType string                       `json:"principalEntityType,omitempty"`
	CreatedDate         string                       `json:"createdDate"`
	LastUpdatedDate     string                       `json:"lastUpdatedDate"`
	Configuration       *IdentitySourceConfiguration `json:"configuration,omitempty"`
}

// ListIdentitySourcesRequest is the input for ListIdentitySources.
type ListIdentitySourcesRequest struct {
	PolicyStoreID string `json:"policyStoreId"`
	MaxResults    int32  `json:"maxResults,omitempty"`
	NextToken     string `json:"nextToken,omitempty"`
}

// IdentitySourceItem is a single entry in the ListIdentitySources result.
type IdentitySourceItem struct {
	PolicyStoreID       string `json:"policyStoreId"`
	IdentitySourceID    string `json:"identitySourceId"`
	PrincipalEntityType string `json:"principalEntityType,omitempty"`
	CreatedDate         string `json:"createdDate"`
	LastUpdatedDate     string `json:"lastUpdatedDate"`
}

// ListIdentitySourcesResponse is the output for ListIdentitySources.
type ListIdentitySourcesResponse struct {
	IdentitySources []IdentitySourceItem `json:"identitySources"`
	NextToken       *string              `json:"nextToken,omitempty"`
}

// DeleteIdentitySourceRequest is the input for DeleteIdentitySource.
type DeleteIdentitySourceRequest struct {
	PolicyStoreID    string `json:"policyStoreId"`
	IdentitySourceID string `json:"identitySourceId"`
}

// IsAuthorizedRequest is the input for IsAuthorized.
type IsAuthorizedRequest struct {
	PolicyStoreID string             `json:"policyStoreId"`
	Principal     *EntityIdentifier  `json:"principal,omitempty"`
	Action        *ActionIdentifier  `json:"action,omitempty"`
	Resource      *EntityIdentifier  `json:"resource,omitempty"`
	Context       *ContextDefinition `json:"context,omitempty"`
}

// DeterminingPolicyItem names a policy that determined the decision.
type DeterminingPolicyItem struct {
	PolicyID string `json:"policyId"`
}

// EvaluationErrorItem describes a Cedar evaluation error.
type EvaluationErrorItem struct {
	ErrorDescription string `json:"errorDescription"`
}

// IsAuthorizedResponse is the output for IsAuthorized.
type IsAuthorizedResponse struct {
	Decision            string                  `json:"decision"`
	DeterminingPolicies []DeterminingPolicyItem `json:"determiningPolicies"`
	Errors              []EvaluationErrorItem   `json:"errors"`
}

// ListTagsForResourceRequest is the input for ListTagsForResource.
type ListTagsForResourceRequest struct {
	ResourceARN string `json:"resourceArn"`
}

// ListTagsForResourceResponse is the output for ListTagsForResource. Verified
// Permissions models tags as a map, not a key/value list.
type ListTagsForResourceResponse struct {
	Tags map[string]string `json:"tags"`
}

// TagResourceRequest is the input for TagResource.
type TagResourceRequest struct {
	ResourceARN string            `json:"resourceArn"`
	Tags        map[string]string `json:"tags"`
}

// UntagResourceRequest is the input for UntagResource.
type UntagResourceRequest struct {
	ResourceARN string   `json:"resourceArn"`
	TagKeys     []string `json:"tagKeys"`
}
