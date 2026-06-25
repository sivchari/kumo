package cognito

import (
	"time"

	"github.com/sivchari/kumo/internal/service"
)

// UserPoolStatus represents the status of a user pool.
type UserPoolStatus string

// User pool statuses.
const (
	UserPoolStatusEnabled  UserPoolStatus = "Enabled"
	UserPoolStatusDisabled UserPoolStatus = "Disabled"
)

// UserStatus represents the status of a user.
type UserStatus string

// User statuses.
const (
	UserStatusUnconfirmed         UserStatus = "UNCONFIRMED"
	UserStatusConfirmed           UserStatus = "CONFIRMED"
	UserStatusArchived            UserStatus = "ARCHIVED"
	UserStatusCompromised         UserStatus = "COMPROMISED"
	UserStatusUnknown             UserStatus = "UNKNOWN"
	UserStatusResetRequired       UserStatus = "RESET_REQUIRED"
	UserStatusForceChangePassword UserStatus = "FORCE_CHANGE_PASSWORD"
)

// AuthFlowType represents the authentication flow type.
type AuthFlowType string

// Authentication flow types.
const (
	AuthFlowUserPasswordAuth      AuthFlowType = "USER_PASSWORD_AUTH"
	AuthFlowUserSRPAuth           AuthFlowType = "USER_SRP_AUTH"
	AuthFlowRefreshTokenAuth      AuthFlowType = "REFRESH_TOKEN_AUTH"
	AuthFlowRefreshToken          AuthFlowType = "REFRESH_TOKEN"
	AuthFlowCustomAuth            AuthFlowType = "CUSTOM_AUTH"
	AuthFlowAdminUserPasswordAuth AuthFlowType = "ADMIN_USER_PASSWORD_AUTH"
)

// ChallengeNameType represents the challenge name type.
type ChallengeNameType string

// Challenge name types.
const (
	ChallengeNewPasswordRequired ChallengeNameType = "NEW_PASSWORD_REQUIRED"
	ChallengeSMSMFA              ChallengeNameType = "SMS_MFA"
	ChallengeSoftwareTokenMFA    ChallengeNameType = "SOFTWARE_TOKEN_MFA"
)

// UserPool represents a Cognito user pool.
type UserPool struct {
	ID                 string
	Name               string
	Status             UserPoolStatus
	CreationDate       time.Time
	LastModifiedDate   time.Time
	Policies           *UserPoolPolicies
	LambdaConfig       *LambdaConfig
	AutoVerifiedAttrs  []string
	UsernameAttributes []string
	MFAConfiguration   string
	EmailConfiguration *EmailConfiguration
	SigningKey         *signingKey // RSA key pair used to sign and publish JWTs (lazily generated)
}

// signingKeyJSON is the on-disk representation of a signingKey: the kid plus
// the PKCS#8 DER of the private key (encoding/json renders []byte as base64).
type signingKeyJSON struct {
	KeyID string `json:"kid"`
	DER   []byte `json:"der"`
}

// jwkSet is a JWK Set as published at /{userPoolId}/.well-known/jwks.json.
type jwkSet struct {
	Keys []jwk `json:"keys"`
}

// jwk is a single RSA public key in JWK form.
type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// UserPoolPolicies represents user pool policies.
type UserPoolPolicies struct {
	PasswordPolicy *PasswordPolicy
}

// PasswordPolicy represents password policy.
type PasswordPolicy struct {
	MinimumLength                 int32
	RequireUppercase              bool
	RequireLowercase              bool
	RequireNumbers                bool
	RequireSymbols                bool
	TemporaryPasswordValidityDays int32
}

// LambdaConfig represents Lambda trigger configuration.
type LambdaConfig struct {
	PreSignUp               string
	CustomMessage           string
	PostConfirmation        string
	PreAuthentication       string
	PostAuthentication      string
	DefineAuthChallenge     string
	CreateAuthChallenge     string
	VerifyAuthChallengeResp string
	PreTokenGeneration      string
	UserMigration           string
}

// EmailConfiguration represents email configuration.
type EmailConfiguration struct {
	SourceArn           string
	ReplyToEmailAddress string
	EmailSendingAccount string
}

// UserPoolClient represents a user pool client.
type UserPoolClient struct {
	ClientID                        string
	ClientName                      string
	UserPoolID                      string
	ClientSecret                    string
	CreationDate                    time.Time
	LastModifiedDate                time.Time
	RefreshTokenValidity            int32
	AccessTokenValidity             int32
	IDTokenValidity                 int32
	ExplicitAuthFlows               []string
	SupportedIdentityProviders      []string
	CallbackURLs                    []string
	LogoutURLs                      []string
	AllowedOAuthFlows               []string
	AllowedOAuthScopes              []string
	AllowedOAuthFlowsUserPoolClient bool
}

// User represents a user in a user pool.
type User struct {
	Username         string
	UserPoolID       string
	Sub              string // stable UUID used as the JWT "sub" claim
	Attributes       []UserAttribute
	UserCreateDate   time.Time
	UserLastModified time.Time
	Enabled          bool
	UserStatus       UserStatus
	Password         string
	MFAOptions       []MFAOption
}

// UserAttribute represents a user attribute.
type UserAttribute struct {
	Name  string
	Value string
}

// MFAOption represents an MFA option.
type MFAOption struct {
	DeliveryMedium string
	AttributeName  string
}

// CreateUserPoolRequest is the request for CreateUserPool.
type CreateUserPoolRequest struct {
	PoolName               string                   `json:"PoolName"`
	Policies               *UserPoolPoliciesInput   `json:"Policies,omitempty"`
	LambdaConfig           *LambdaConfigInput       `json:"LambdaConfig,omitempty"`
	AutoVerifiedAttributes []string                 `json:"AutoVerifiedAttributes,omitempty"`
	UsernameAttributes     []string                 `json:"UsernameAttributes,omitempty"`
	MfaConfiguration       string                   `json:"MfaConfiguration,omitempty"`
	EmailConfiguration     *EmailConfigurationInput `json:"EmailConfiguration,omitempty"`
	Region                 string                   `json:"-"`
}

// UserPoolPoliciesInput represents user pool policies in requests.
type UserPoolPoliciesInput struct {
	PasswordPolicy *PasswordPolicyInput `json:"PasswordPolicy,omitempty"`
}

// PasswordPolicyInput represents password policy in requests.
type PasswordPolicyInput struct {
	MinimumLength                 int32 `json:"MinimumLength,omitempty"`
	RequireUppercase              bool  `json:"RequireUppercase,omitempty"`
	RequireLowercase              bool  `json:"RequireLowercase,omitempty"`
	RequireNumbers                bool  `json:"RequireNumbers,omitempty"`
	RequireSymbols                bool  `json:"RequireSymbols,omitempty"`
	TemporaryPasswordValidityDays int32 `json:"TemporaryPasswordValidityDays,omitempty"`
}

// LambdaConfigInput represents Lambda config in requests.
type LambdaConfigInput struct {
	PreSignUp               string `json:"PreSignUp,omitempty"`
	CustomMessage           string `json:"CustomMessage,omitempty"`
	PostConfirmation        string `json:"PostConfirmation,omitempty"`
	PreAuthentication       string `json:"PreAuthentication,omitempty"`
	PostAuthentication      string `json:"PostAuthentication,omitempty"`
	DefineAuthChallenge     string `json:"DefineAuthChallenge,omitempty"`
	CreateAuthChallenge     string `json:"CreateAuthChallenge,omitempty"`
	VerifyAuthChallengeResp string `json:"VerifyAuthChallengeResponse,omitempty"`
	PreTokenGeneration      string `json:"PreTokenGeneration,omitempty"`
	UserMigration           string `json:"UserMigration,omitempty"`
}

// EmailConfigurationInput represents email configuration in requests.
type EmailConfigurationInput struct {
	SourceArn           string `json:"SourceArn,omitempty"`
	ReplyToEmailAddress string `json:"ReplyToEmailAddress,omitempty"`
	EmailSendingAccount string `json:"EmailSendingAccount,omitempty"`
}

// CreateUserPoolResponse is the response for CreateUserPool.
type CreateUserPoolResponse struct {
	UserPool *UserPoolOutput `json:"UserPool"`
}

// UserPoolOutput represents a user pool in API responses.
//
// Field set is sized to terraform-provider-aws's resourceUserPoolRead
// requirements: every nested struct it dereferences (Arn,
// AdminCreateUserConfig, AccountRecoverySetting, DeviceConfiguration,
// VerificationMessageTemplate, UserPoolAddOns, UsernameConfiguration,
// UserAttributeUpdateSettings) must be present (or explicitly nil) so the
// provider's flatten helpers don't nil-pointer-panic. SchemaAttributes,
// EstimatedNumberOfUsers, DeletionProtection, UserPoolTier, MfaConfiguration
// have sensible AWS-default values rather than zero values.
type UserPoolOutput struct {
	ID                          string                             `json:"Id"`
	Arn                         string                             `json:"Arn"`
	Name                        string                             `json:"Name"`
	Status                      string                             `json:"Status,omitempty"`
	CreationDate                float64                            `json:"CreationDate"`
	LastModifiedDate            float64                            `json:"LastModifiedDate"`
	Policies                    *UserPoolPoliciesOutput            `json:"Policies,omitempty"`
	LambdaConfig                *LambdaConfigOutput                `json:"LambdaConfig"`
	AutoVerifiedAttributes      []string                           `json:"AutoVerifiedAttributes"`
	UsernameAttributes          []string                           `json:"UsernameAttributes"`
	AliasAttributes             []string                           `json:"AliasAttributes"`
	MfaConfiguration            string                             `json:"MfaConfiguration"`
	EmailConfiguration          *EmailConfigurationOutput          `json:"EmailConfiguration,omitempty"`
	AdminCreateUserConfig       *AdminCreateUserConfigOutput       `json:"AdminCreateUserConfig"`
	AccountRecoverySetting      *AccountRecoverySettingOutput      `json:"AccountRecoverySetting"`
	DeviceConfiguration         *DeviceConfigurationOutput         `json:"DeviceConfiguration,omitempty"`
	VerificationMessageTemplate *VerificationMessageTemplateOutput `json:"VerificationMessageTemplate"`
	UserPoolAddOns              *UserPoolAddOnsOutput              `json:"UserPoolAddOns,omitempty"`
	UsernameConfiguration       *UsernameConfigurationOutput       `json:"UsernameConfiguration,omitempty"`
	UserAttributeUpdateSettings *UserAttributeUpdateSettingsOutput `json:"UserAttributeUpdateSettings,omitempty"`
	SchemaAttributes            []SchemaAttributeOutput            `json:"SchemaAttributes"`
	UserPoolTags                map[string]string                  `json:"UserPoolTags"`
	EstimatedNumberOfUsers      int32                              `json:"EstimatedNumberOfUsers"`
	DeletionProtection          string                             `json:"DeletionProtection"`
	UserPoolTier                string                             `json:"UserPoolTier"`
	SmsConfiguration            *SMSConfigurationOutput            `json:"SmsConfiguration,omitempty"`
	SmsAuthenticationMessage    string                             `json:"SmsAuthenticationMessage,omitempty"`
	EmailVerificationMessage    string                             `json:"EmailVerificationMessage,omitempty"`
	EmailVerificationSubject    string                             `json:"EmailVerificationSubject,omitempty"`
}

// AdminCreateUserConfigOutput mirrors AWS's AdminCreateUserConfigType.
type AdminCreateUserConfigOutput struct {
	AllowAdminCreateUserOnly  bool                   `json:"AllowAdminCreateUserOnly"`
	UnusedAccountValidityDays int32                  `json:"UnusedAccountValidityDays,omitempty"`
	InviteMessageTemplate     *MessageTemplateOutput `json:"InviteMessageTemplate,omitempty"`
}

// MessageTemplateOutput mirrors AWS's MessageTemplateType.
type MessageTemplateOutput struct {
	SMSMessage   string `json:"SMSMessage,omitempty"`
	EmailMessage string `json:"EmailMessage,omitempty"`
	EmailSubject string `json:"EmailSubject,omitempty"`
}

// AccountRecoverySettingOutput mirrors AWS's AccountRecoverySettingType.
type AccountRecoverySettingOutput struct {
	RecoveryMechanisms []RecoveryMechanismOutput `json:"RecoveryMechanisms"`
}

// RecoveryMechanismOutput is a single RecoveryOptionType entry.
type RecoveryMechanismOutput struct {
	Priority int32  `json:"Priority"`
	Name     string `json:"Name"`
}

// DeviceConfigurationOutput mirrors AWS's DeviceConfigurationType.
type DeviceConfigurationOutput struct {
	ChallengeRequiredOnNewDevice     bool `json:"ChallengeRequiredOnNewDevice"`
	DeviceOnlyRememberedOnUserPrompt bool `json:"DeviceOnlyRememberedOnUserPrompt"`
}

// VerificationMessageTemplateOutput mirrors AWS's VerificationMessageTemplateType.
type VerificationMessageTemplateOutput struct {
	SMSMessage         string `json:"SmsMessage,omitempty"`
	EmailMessage       string `json:"EmailMessage,omitempty"`
	EmailSubject       string `json:"EmailSubject,omitempty"`
	EmailMessageByLink string `json:"EmailMessageByLink,omitempty"`
	EmailSubjectByLink string `json:"EmailSubjectByLink,omitempty"`
	DefaultEmailOption string `json:"DefaultEmailOption,omitempty"`
}

// UserPoolAddOnsOutput mirrors AWS's UserPoolAddOnsType.
type UserPoolAddOnsOutput struct {
	AdvancedSecurityMode string `json:"AdvancedSecurityMode"`
}

// UsernameConfigurationOutput mirrors AWS's UsernameConfigurationType.
type UsernameConfigurationOutput struct {
	CaseSensitive bool `json:"CaseSensitive"`
}

// UserAttributeUpdateSettingsOutput mirrors AWS's UserAttributeUpdateSettingsType.
type UserAttributeUpdateSettingsOutput struct {
	AttributesRequireVerificationBeforeUpdate []string `json:"AttributesRequireVerificationBeforeUpdate"`
}

// SchemaAttributeOutput mirrors AWS's SchemaAttributeType (one entry per
// attribute in the user pool's schema).
type SchemaAttributeOutput struct {
	Name                       string                            `json:"Name"`
	AttributeDataType          string                            `json:"AttributeDataType"`
	DeveloperOnlyAttribute     bool                              `json:"DeveloperOnlyAttribute"`
	Mutable                    bool                              `json:"Mutable"`
	Required                   bool                              `json:"Required"`
	StringAttributeConstraints *StringAttributeConstraintsOutput `json:"StringAttributeConstraints,omitempty"`
	NumberAttributeConstraints *NumberAttributeConstraintsOutput `json:"NumberAttributeConstraints,omitempty"`
}

// StringAttributeConstraintsOutput mirrors AWS's StringAttributeConstraintsType.
type StringAttributeConstraintsOutput struct {
	MinLength string `json:"MinLength,omitempty"`
	MaxLength string `json:"MaxLength,omitempty"`
}

// NumberAttributeConstraintsOutput mirrors AWS's NumberAttributeConstraintsType.
type NumberAttributeConstraintsOutput struct {
	MinValue string `json:"MinValue,omitempty"`
	MaxValue string `json:"MaxValue,omitempty"`
}

// SMSConfigurationOutput mirrors AWS's SmsConfigurationType.
type SMSConfigurationOutput struct {
	SnsCallerArn string `json:"SnsCallerArn"`
	ExternalID   string `json:"ExternalId,omitempty"`
}

// UserPoolPoliciesOutput represents user pool policies in responses.
type UserPoolPoliciesOutput struct {
	PasswordPolicy *PasswordPolicyOutput `json:"PasswordPolicy,omitempty"`
}

// PasswordPolicyOutput represents password policy in responses.
type PasswordPolicyOutput struct {
	MinimumLength                 int32 `json:"MinimumLength"`
	RequireUppercase              bool  `json:"RequireUppercase"`
	RequireLowercase              bool  `json:"RequireLowercase"`
	RequireNumbers                bool  `json:"RequireNumbers"`
	RequireSymbols                bool  `json:"RequireSymbols"`
	TemporaryPasswordValidityDays int32 `json:"TemporaryPasswordValidityDays"`
}

// LambdaConfigOutput represents Lambda config in responses.
type LambdaConfigOutput struct {
	PreSignUp               string `json:"PreSignUp,omitempty"`
	CustomMessage           string `json:"CustomMessage,omitempty"`
	PostConfirmation        string `json:"PostConfirmation,omitempty"`
	PreAuthentication       string `json:"PreAuthentication,omitempty"`
	PostAuthentication      string `json:"PostAuthentication,omitempty"`
	DefineAuthChallenge     string `json:"DefineAuthChallenge,omitempty"`
	CreateAuthChallenge     string `json:"CreateAuthChallenge,omitempty"`
	VerifyAuthChallengeResp string `json:"VerifyAuthChallengeResponse,omitempty"`
	PreTokenGeneration      string `json:"PreTokenGeneration,omitempty"`
	UserMigration           string `json:"UserMigration,omitempty"`
}

// EmailConfigurationOutput represents email configuration in responses.
type EmailConfigurationOutput struct {
	SourceArn           string `json:"SourceArn,omitempty"`
	ReplyToEmailAddress string `json:"ReplyToEmailAddress,omitempty"`
	EmailSendingAccount string `json:"EmailSendingAccount,omitempty"`
}

// DescribeUserPoolRequest is the request for DescribeUserPool.
type DescribeUserPoolRequest struct {
	UserPoolID string `json:"UserPoolId"`
}

// DescribeUserPoolResponse is the response for DescribeUserPool.
type DescribeUserPoolResponse struct {
	UserPool *UserPoolOutput `json:"UserPool"`
}

// ListUserPoolsRequest is the request for ListUserPools.
type ListUserPoolsRequest struct {
	MaxResults int32  `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
}

// ListUserPoolsResponse is the response for ListUserPools.
type ListUserPoolsResponse struct {
	UserPools []UserPoolOutput `json:"UserPools"`
	NextToken string           `json:"NextToken,omitempty"`
}

// DeleteUserPoolRequest is the request for DeleteUserPool.
type DeleteUserPoolRequest struct {
	UserPoolID string `json:"UserPoolId"`
}

// DeleteUserPoolResponse is the response for DeleteUserPool.
type DeleteUserPoolResponse struct{}

// CreateUserPoolClientRequest is the request for CreateUserPoolClient.
type CreateUserPoolClientRequest struct {
	UserPoolID                      string   `json:"UserPoolId"`
	ClientName                      string   `json:"ClientName"`
	GenerateSecret                  bool     `json:"GenerateSecret,omitempty"`
	RefreshTokenValidity            int32    `json:"RefreshTokenValidity,omitempty"`
	AccessTokenValidity             int32    `json:"AccessTokenValidity,omitempty"`
	IDTokenValidity                 int32    `json:"IdTokenValidity,omitempty"`
	ExplicitAuthFlows               []string `json:"ExplicitAuthFlows,omitempty"`
	SupportedIdentityProviders      []string `json:"SupportedIdentityProviders,omitempty"`
	CallbackURLs                    []string `json:"CallbackURLs,omitempty"`
	LogoutURLs                      []string `json:"LogoutURLs,omitempty"`
	AllowedOAuthFlows               []string `json:"AllowedOAuthFlows,omitempty"`
	AllowedOAuthScopes              []string `json:"AllowedOAuthScopes,omitempty"`
	AllowedOAuthFlowsUserPoolClient bool     `json:"AllowedOAuthFlowsUserPoolClient,omitempty"`
}

// CreateUserPoolClientResponse is the response for CreateUserPoolClient.
type CreateUserPoolClientResponse struct {
	UserPoolClient *UserPoolClientOutput `json:"UserPoolClient"`
}

// UserPoolClientOutput represents a user pool client in API responses.
type UserPoolClientOutput struct {
	ClientID                        string   `json:"ClientId"`
	ClientName                      string   `json:"ClientName"`
	UserPoolID                      string   `json:"UserPoolId"`
	ClientSecret                    string   `json:"ClientSecret,omitempty"`
	CreationDate                    float64  `json:"CreationDate"`
	LastModifiedDate                float64  `json:"LastModifiedDate"`
	RefreshTokenValidity            int32    `json:"RefreshTokenValidity"`
	AccessTokenValidity             int32    `json:"AccessTokenValidity"`
	IDTokenValidity                 int32    `json:"IdTokenValidity"`
	ExplicitAuthFlows               []string `json:"ExplicitAuthFlows,omitempty"`
	SupportedIdentityProviders      []string `json:"SupportedIdentityProviders,omitempty"`
	CallbackURLs                    []string `json:"CallbackURLs,omitempty"`
	LogoutURLs                      []string `json:"LogoutURLs,omitempty"`
	AllowedOAuthFlows               []string `json:"AllowedOAuthFlows,omitempty"`
	AllowedOAuthScopes              []string `json:"AllowedOAuthScopes,omitempty"`
	AllowedOAuthFlowsUserPoolClient bool     `json:"AllowedOAuthFlowsUserPoolClient"`
}

// DescribeUserPoolClientRequest is the request for DescribeUserPoolClient.
type DescribeUserPoolClientRequest struct {
	UserPoolID string `json:"UserPoolId"`
	ClientID   string `json:"ClientId"`
}

// DescribeUserPoolClientResponse is the response for DescribeUserPoolClient.
type DescribeUserPoolClientResponse struct {
	UserPoolClient *UserPoolClientOutput `json:"UserPoolClient"`
}

// ListUserPoolClientsRequest is the request for ListUserPoolClients.
type ListUserPoolClientsRequest struct {
	UserPoolID string `json:"UserPoolId"`
	MaxResults int32  `json:"MaxResults,omitempty"`
	NextToken  string `json:"NextToken,omitempty"`
}

// ListUserPoolClientsResponse is the response for ListUserPoolClients.
type ListUserPoolClientsResponse struct {
	UserPoolClients []UserPoolClientOutput `json:"UserPoolClients"`
	NextToken       string                 `json:"NextToken,omitempty"`
}

// DeleteUserPoolClientRequest is the request for DeleteUserPoolClient.
type DeleteUserPoolClientRequest struct {
	UserPoolID string `json:"UserPoolId"`
	ClientID   string `json:"ClientId"`
}

// DeleteUserPoolClientResponse is the response for DeleteUserPoolClient.
type DeleteUserPoolClientResponse struct{}

// AdminCreateUserRequest is the request for AdminCreateUser.
type AdminCreateUserRequest struct {
	UserPoolID             string               `json:"UserPoolId"`
	Username               string               `json:"Username"`
	UserAttributes         []UserAttributeInput `json:"UserAttributes,omitempty"`
	TemporaryPassword      string               `json:"TemporaryPassword,omitempty"`
	ForceAliasCreation     bool                 `json:"ForceAliasCreation,omitempty"`
	MessageAction          string               `json:"MessageAction,omitempty"`
	DesiredDeliveryMediums []string             `json:"DesiredDeliveryMediums,omitempty"`
}

// UserAttributeInput represents a user attribute in requests.
type UserAttributeInput struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

// AdminCreateUserResponse is the response for AdminCreateUser.
type AdminCreateUserResponse struct {
	User *UserOutput `json:"User"`
}

// UserOutput represents a user in API responses.
type UserOutput struct {
	Username             string                `json:"Username"`
	Attributes           []UserAttributeOutput `json:"Attributes,omitempty"`
	UserCreateDate       float64               `json:"UserCreateDate"`
	UserLastModifiedDate float64               `json:"UserLastModifiedDate"`
	Enabled              bool                  `json:"Enabled"`
	UserStatus           string                `json:"UserStatus"`
	MFAOptions           []MFAOptionOutput     `json:"MFAOptions,omitempty"`
}

// UserAttributeOutput represents a user attribute in responses.
type UserAttributeOutput struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

// MFAOptionOutput represents an MFA option in responses.
type MFAOptionOutput struct {
	DeliveryMedium string `json:"DeliveryMedium"`
	AttributeName  string `json:"AttributeName"`
}

// AdminGetUserRequest is the request for AdminGetUser.
type AdminGetUserRequest struct {
	UserPoolID string `json:"UserPoolId"`
	Username   string `json:"Username"`
}

// AdminGetUserResponse is the response for AdminGetUser.
type AdminGetUserResponse struct {
	Username             string                `json:"Username"`
	UserAttributes       []UserAttributeOutput `json:"UserAttributes,omitempty"`
	UserCreateDate       float64               `json:"UserCreateDate"`
	UserLastModifiedDate float64               `json:"UserLastModifiedDate"`
	Enabled              bool                  `json:"Enabled"`
	UserStatus           string                `json:"UserStatus"`
	MFAOptions           []MFAOptionOutput     `json:"MFAOptions,omitempty"`
}

// AdminDeleteUserRequest is the request for AdminDeleteUser.
type AdminDeleteUserRequest struct {
	UserPoolID string `json:"UserPoolId"`
	Username   string `json:"Username"`
}

// AdminDeleteUserResponse is the response for AdminDeleteUser.
type AdminDeleteUserResponse struct{}

// ListUsersRequest is the request for ListUsers.
type ListUsersRequest struct {
	UserPoolID      string   `json:"UserPoolId"`
	AttributesToGet []string `json:"AttributesToGet,omitempty"`
	Limit           int32    `json:"Limit,omitempty"`
	PaginationToken string   `json:"PaginationToken,omitempty"`
	Filter          string   `json:"Filter,omitempty"`
}

// ListUsersResponse is the response for ListUsers.
type ListUsersResponse struct {
	Users           []UserOutput `json:"Users"`
	PaginationToken string       `json:"PaginationToken,omitempty"`
}

// SignUpRequest is the request for SignUp.
type SignUpRequest struct {
	ClientID       string               `json:"ClientId"`
	Username       string               `json:"Username"`
	Password       string               `json:"Password"`
	UserAttributes []UserAttributeInput `json:"UserAttributes,omitempty"`
	SecretHash     string               `json:"SecretHash,omitempty"`
}

// SignUpResponse is the response for SignUp.
type SignUpResponse struct {
	UserConfirmed bool   `json:"UserConfirmed"`
	UserSub       string `json:"UserSub"`
}

// ConfirmSignUpRequest is the request for ConfirmSignUp.
type ConfirmSignUpRequest struct {
	ClientID         string `json:"ClientId"`
	Username         string `json:"Username"`
	ConfirmationCode string `json:"ConfirmationCode"`
	SecretHash       string `json:"SecretHash,omitempty"`
}

// ConfirmSignUpResponse is the response for ConfirmSignUp.
type ConfirmSignUpResponse struct{}

// InitiateAuthRequest is the request for InitiateAuth.
type InitiateAuthRequest struct {
	AuthFlow       string            `json:"AuthFlow"`
	ClientID       string            `json:"ClientId"`
	AuthParameters map[string]string `json:"AuthParameters,omitempty"`
}

// InitiateAuthResponse is the response for InitiateAuth.
type InitiateAuthResponse struct {
	ChallengeName        string                `json:"ChallengeName,omitempty"`
	Session              string                `json:"Session,omitempty"`
	ChallengeParameters  map[string]string     `json:"ChallengeParameters,omitempty"`
	AuthenticationResult *AuthenticationResult `json:"AuthenticationResult,omitempty"`
}

// AdminInitiateAuthRequest is the request for AdminInitiateAuth.
type AdminInitiateAuthRequest struct {
	UserPoolID     string            `json:"UserPoolId"`
	ClientID       string            `json:"ClientId"`
	AuthFlow       string            `json:"AuthFlow"`
	AuthParameters map[string]string `json:"AuthParameters,omitempty"`
}

// AdminInitiateAuthResponse is the response for AdminInitiateAuth. It mirrors
// InitiateAuthResponse: the admin variant differs only in the auth flow, not
// the response shape.
type AdminInitiateAuthResponse struct {
	ChallengeName        string                `json:"ChallengeName,omitempty"`
	Session              string                `json:"Session,omitempty"`
	ChallengeParameters  map[string]string     `json:"ChallengeParameters,omitempty"`
	AuthenticationResult *AuthenticationResult `json:"AuthenticationResult,omitempty"`
}

// AuthenticationResult represents authentication result.
type AuthenticationResult struct {
	AccessToken  string `json:"AccessToken"`
	ExpiresIn    int32  `json:"ExpiresIn"`
	TokenType    string `json:"TokenType"`
	RefreshToken string `json:"RefreshToken,omitempty"`
	IDToken      string `json:"IdToken"`
}

// RespondToAuthChallengeRequest is the request for RespondToAuthChallenge.
type RespondToAuthChallengeRequest struct {
	ChallengeName      string            `json:"ChallengeName"`
	ClientID           string            `json:"ClientId"`
	ChallengeResponses map[string]string `json:"ChallengeResponses,omitempty"`
	Session            string            `json:"Session,omitempty"`
}

// RespondToAuthChallengeResponse is the response for RespondToAuthChallenge.
type RespondToAuthChallengeResponse struct {
	ChallengeName        string                `json:"ChallengeName,omitempty"`
	Session              string                `json:"Session,omitempty"`
	ChallengeParameters  map[string]string     `json:"ChallengeParameters,omitempty"`
	AuthenticationResult *AuthenticationResult `json:"AuthenticationResult,omitempty"`
}

// ErrorResponse represents a Cognito error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError represents a Cognito service error.
type ServiceError = service.CodedError

// MfaConfig represents the stored MFA configuration for a user pool.
type MfaConfig struct {
	MfaConfiguration              string                         `json:"MfaConfiguration"`
	SmsMfaConfiguration           *SmsMfaConfiguration           `json:"SmsMfaConfiguration,omitempty"`
	SoftwareTokenMfaConfiguration *SoftwareTokenMfaConfiguration `json:"SoftwareTokenMfaConfiguration,omitempty"`
}

// SmsMfaConfiguration represents SMS MFA configuration.
type SmsMfaConfiguration struct {
	SmsAuthenticationMessage string                        `json:"SmsAuthenticationMessage,omitempty"`
	SmsConfiguration         *SmsMfaConfigSmsConfiguration `json:"SmsConfiguration,omitempty"`
}

// SmsMfaConfigSmsConfiguration represents the SMS configuration within SMS MFA.
type SmsMfaConfigSmsConfiguration struct {
	SnsCallerArn string `json:"SnsCallerArn,omitempty"`
	ExternalID   string `json:"ExternalId,omitempty"`
}

// SoftwareTokenMfaConfiguration represents software token MFA configuration.
type SoftwareTokenMfaConfiguration struct {
	Enabled bool `json:"Enabled"`
}

// GetUserPoolMfaConfigRequest is the request for GetUserPoolMfaConfig.
type GetUserPoolMfaConfigRequest struct {
	UserPoolID string `json:"UserPoolId"`
}

// GetUserPoolMfaConfigResponse is the response for GetUserPoolMfaConfig.
type GetUserPoolMfaConfigResponse struct {
	MfaConfiguration              string                               `json:"MfaConfiguration"`
	SmsMfaConfiguration           *SmsMfaConfigurationOutput           `json:"SmsMfaConfiguration,omitempty"`
	SoftwareTokenMfaConfiguration *SoftwareTokenMfaConfigurationOutput `json:"SoftwareTokenMfaConfiguration,omitempty"`
}

// SmsMfaConfigurationOutput represents SMS MFA configuration in responses.
type SmsMfaConfigurationOutput struct {
	SmsAuthenticationMessage string                        `json:"SmsAuthenticationMessage,omitempty"`
	SmsConfiguration         *SmsMfaConfigSmsConfiguration `json:"SmsConfiguration,omitempty"`
}

// SoftwareTokenMfaConfigurationOutput represents software token MFA configuration in responses.
type SoftwareTokenMfaConfigurationOutput struct {
	Enabled bool `json:"Enabled"`
}

// SetUserPoolMfaConfigRequest is the request for SetUserPoolMfaConfig.
type SetUserPoolMfaConfigRequest struct {
	UserPoolID                    string                              `json:"UserPoolId"`
	MfaConfiguration              string                              `json:"MfaConfiguration"`
	SmsMfaConfiguration           *SmsMfaConfigurationInput           `json:"SmsMfaConfiguration,omitempty"`
	SoftwareTokenMfaConfiguration *SoftwareTokenMfaConfigurationInput `json:"SoftwareTokenMfaConfiguration,omitempty"`
}

// SmsMfaConfigurationInput represents SMS MFA configuration in requests.
type SmsMfaConfigurationInput struct {
	SmsAuthenticationMessage string                        `json:"SmsAuthenticationMessage,omitempty"`
	SmsConfiguration         *SmsMfaConfigSmsConfiguration `json:"SmsConfiguration,omitempty"`
}

// SoftwareTokenMfaConfigurationInput represents software token MFA configuration in requests.
type SoftwareTokenMfaConfigurationInput struct {
	Enabled bool `json:"Enabled"`
}

// SetUserPoolMfaConfigResponse is the response for SetUserPoolMfaConfig.
type SetUserPoolMfaConfigResponse struct {
	MfaConfiguration              string                               `json:"MfaConfiguration"`
	SmsMfaConfiguration           *SmsMfaConfigurationOutput           `json:"SmsMfaConfiguration,omitempty"`
	SoftwareTokenMfaConfiguration *SoftwareTokenMfaConfigurationOutput `json:"SoftwareTokenMfaConfiguration,omitempty"`
}
