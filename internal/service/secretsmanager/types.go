// Package secretsmanager provides Secrets Manager service emulation for kumo.
package secretsmanager

import (
	"time"
)

// Secret represents a secret in Secrets Manager.
type Secret struct {
	ARN                            string
	Name                           string
	Description                    string
	KmsKeyID                       string
	VersionID                      string
	SecretString                   string
	SecretBinary                   []byte
	CreatedDate                    time.Time
	LastChangedDate                time.Time
	LastAccessedDate               *time.Time
	DeletedDate                    *time.Time
	Tags                           []Tag
	VersionIDs                     map[string]*SecretVersion
	RotationEnabled                bool
	RotationLambdaARN              string
	RotationRules                  *RotationRules
	PrimaryRegion                  string
	ReplicationStatus              []ReplicationStatus
	OwningService                  string
	NextRotationDate               *time.Time
	LastRotationDate               *time.Time
	RecoveryWindowInDays           int64
	ScheduledDeletionDate          *time.Time
	Type                           string
	ExternalSecretRotationMetadata []ExternalSecretRotationMetadataItem
	ExternalSecretRotationRoleArn  string
}

// SecretVersion represents a version of a secret.
type SecretVersion struct {
	VersionID     string
	SecretString  string
	SecretBinary  []byte
	VersionStages []string
	CreatedDate   time.Time
	KmsKeyID      string
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// RotationRules represents rotation rules.
type RotationRules struct {
	AutomaticallyAfterDays int64  `json:"AutomaticallyAfterDays,omitempty"`
	Duration               string `json:"Duration,omitempty"`
	ScheduleExpression     string `json:"ScheduleExpression,omitempty"`
}

// ReplicationStatus represents replication status.
type ReplicationStatus struct {
	Region           string `json:"Region"`
	KmsKeyID         string `json:"KmsKeyId,omitempty"`
	Status           string `json:"Status"`
	StatusMessage    string `json:"StatusMessage,omitempty"`
	LastAccessedDate *int64 `json:"LastAccessedDate,omitempty"`
}

// CreateSecretRequest is the request for CreateSecret.
type CreateSecretRequest struct {
	Name                        string          `json:"Name"`
	ClientRequestToken          string          `json:"ClientRequestToken,omitempty"`
	Description                 string          `json:"Description,omitempty"`
	KmsKeyID                    string          `json:"KmsKeyId,omitempty"`
	SecretBinary                []byte          `json:"SecretBinary,omitempty"`
	SecretString                string          `json:"SecretString,omitempty"`
	Tags                        []Tag           `json:"Tags,omitempty"`
	AddReplicaRegions           []ReplicaRegion `json:"AddReplicaRegions,omitempty"`
	ForceOverwriteReplicaSecret bool            `json:"ForceOverwriteReplicaSecret,omitempty"`
}

// ReplicaRegion represents a replica region.
type ReplicaRegion struct {
	Region   string `json:"Region"`
	KmsKeyID string `json:"KmsKeyId,omitempty"`
}

// CreateSecretResponse is the response for CreateSecret.
type CreateSecretResponse struct {
	ARN               string              `json:"ARN"`
	Name              string              `json:"Name"`
	VersionID         string              `json:"VersionId,omitempty"`
	ReplicationStatus []ReplicationStatus `json:"ReplicationStatus,omitempty"`
}

// GetSecretValueRequest is the request for GetSecretValue.
type GetSecretValueRequest struct {
	SecretID     string `json:"SecretId"`
	VersionID    string `json:"VersionId,omitempty"`
	VersionStage string `json:"VersionStage,omitempty"`
}

// GetSecretValueResponse is the response for GetSecretValue.
type GetSecretValueResponse struct {
	ARN           string   `json:"ARN"`
	Name          string   `json:"Name"`
	VersionID     string   `json:"VersionId"`
	SecretBinary  []byte   `json:"SecretBinary,omitempty"`
	SecretString  string   `json:"SecretString,omitempty"`
	VersionStages []string `json:"VersionStages"`
	CreatedDate   float64  `json:"CreatedDate"`
}

// PutSecretValueRequest is the request for PutSecretValue.
type PutSecretValueRequest struct {
	SecretID           string   `json:"SecretId"`
	ClientRequestToken string   `json:"ClientRequestToken,omitempty"`
	SecretBinary       []byte   `json:"SecretBinary,omitempty"`
	SecretString       string   `json:"SecretString,omitempty"`
	VersionStages      []string `json:"VersionStages,omitempty"`
}

// PutSecretValueResponse is the response for PutSecretValue.
type PutSecretValueResponse struct {
	ARN           string   `json:"ARN"`
	Name          string   `json:"Name"`
	VersionID     string   `json:"VersionId"`
	VersionStages []string `json:"VersionStages"`
}

// DeleteSecretRequest is the request for DeleteSecret.
type DeleteSecretRequest struct {
	SecretID                   string `json:"SecretId"`
	RecoveryWindowInDays       int64  `json:"RecoveryWindowInDays,omitempty"`
	ForceDeleteWithoutRecovery bool   `json:"ForceDeleteWithoutRecovery,omitempty"`
}

// DeleteSecretResponse is the response for DeleteSecret.
type DeleteSecretResponse struct {
	ARN          string  `json:"ARN"`
	Name         string  `json:"Name"`
	DeletionDate float64 `json:"DeletionDate"`
}

// ListSecretsRequest is the request for ListSecrets.
type ListSecretsRequest struct {
	Filters                []Filter `json:"Filters,omitempty"`
	MaxResults             int      `json:"MaxResults,omitempty"`
	NextToken              string   `json:"NextToken,omitempty"`
	SortOrder              string   `json:"SortOrder,omitempty"`
	IncludePlannedDeletion bool     `json:"IncludePlannedDeletion,omitempty"`
}

// Filter represents a filter.
type Filter struct {
	Key    string   `json:"Key"`
	Values []string `json:"Values"`
}

// ListSecretsResponse is the response for ListSecrets.
type ListSecretsResponse struct {
	SecretList []SecretListEntry `json:"SecretList"`
	NextToken  string            `json:"NextToken,omitempty"`
}

// SecretListEntry represents a secret in list response.
type SecretListEntry struct {
	ARN                            string                               `json:"ARN"`
	Name                           string                               `json:"Name"`
	Description                    string                               `json:"Description,omitempty"`
	KmsKeyID                       string                               `json:"KmsKeyId,omitempty"`
	RotationEnabled                bool                                 `json:"RotationEnabled"`
	RotationLambdaARN              string                               `json:"RotationLambdaARN,omitempty"`
	RotationRules                  *RotationRules                       `json:"RotationRules,omitempty"`
	LastRotatedDate                *float64                             `json:"LastRotatedDate,omitempty"`
	LastChangedDate                *float64                             `json:"LastChangedDate,omitempty"`
	LastAccessedDate               *float64                             `json:"LastAccessedDate,omitempty"`
	DeletedDate                    *float64                             `json:"DeletedDate,omitempty"`
	NextRotationDate               *float64                             `json:"NextRotationDate,omitempty"`
	Tags                           []Tag                                `json:"Tags,omitempty"`
	SecretVersionsToStages         map[string][]string                  `json:"SecretVersionsToStages,omitempty"`
	OwningService                  string                               `json:"OwningService,omitempty"`
	CreatedDate                    float64                              `json:"CreatedDate"`
	PrimaryRegion                  string                               `json:"PrimaryRegion,omitempty"`
	Type                           string                               `json:"Type,omitempty"`
	ExternalSecretRotationMetadata []ExternalSecretRotationMetadataItem `json:"ExternalSecretRotationMetadata,omitempty"`
	ExternalSecretRotationRoleArn  string                               `json:"ExternalSecretRotationRoleArn,omitempty"`
}

// DescribeSecretRequest is the request for DescribeSecret.
type DescribeSecretRequest struct {
	SecretID string `json:"SecretId"`
}

// DescribeSecretResponse is the response for DescribeSecret.
type DescribeSecretResponse struct {
	ARN                string              `json:"ARN"`
	Name               string              `json:"Name"`
	Description        string              `json:"Description,omitempty"`
	KmsKeyID           string              `json:"KmsKeyId,omitempty"`
	RotationEnabled    bool                `json:"RotationEnabled"`
	RotationLambdaARN  string              `json:"RotationLambdaARN,omitempty"`
	RotationRules      *RotationRules      `json:"RotationRules,omitempty"`
	LastRotatedDate    *float64            `json:"LastRotatedDate,omitempty"`
	LastChangedDate    *float64            `json:"LastChangedDate,omitempty"`
	LastAccessedDate   *float64            `json:"LastAccessedDate,omitempty"`
	DeletedDate        *float64            `json:"DeletedDate,omitempty"`
	NextRotationDate   *float64            `json:"NextRotationDate,omitempty"`
	Tags               []Tag               `json:"Tags,omitempty"`
	VersionIDsToStages map[string][]string `json:"VersionIdsToStages,omitempty"`
	OwningService      string              `json:"OwningService,omitempty"`
	CreatedDate        float64             `json:"CreatedDate"`
	PrimaryRegion      string              `json:"PrimaryRegion,omitempty"`
	ReplicationStatus  []ReplicationStatus `json:"ReplicationStatus,omitempty"`
}

// UpdateSecretRequest is the request for UpdateSecret.
type UpdateSecretRequest struct {
	SecretID           string `json:"SecretId"`
	ClientRequestToken string `json:"ClientRequestToken,omitempty"`
	Description        string `json:"Description,omitempty"`
	KmsKeyID           string `json:"KmsKeyId,omitempty"`
	SecretBinary       []byte `json:"SecretBinary,omitempty"`
	SecretString       string `json:"SecretString,omitempty"`
}

// UpdateSecretResponse is the response for UpdateSecret.
type UpdateSecretResponse struct {
	ARN       string `json:"ARN"`
	Name      string `json:"Name"`
	VersionID string `json:"VersionId,omitempty"`
}

// GetRandomPasswordRequest is the request for GetRandomPassword.
type GetRandomPasswordRequest struct {
	ExcludeCharacters       string `json:"ExcludeCharacters,omitempty"`
	ExcludeLowercase        bool   `json:"ExcludeLowercase,omitempty"`
	ExcludeNumbers          bool   `json:"ExcludeNumbers,omitempty"`
	ExcludePunctuation      bool   `json:"ExcludePunctuation,omitempty"`
	ExcludeUppercase        bool   `json:"ExcludeUppercase,omitempty"`
	IncludeSpace            bool   `json:"IncludeSpace,omitempty"`
	PasswordLength          int64  `json:"PasswordLength,omitempty"`
	RequireEachIncludedType bool   `json:"RequireEachIncludedType,omitempty"`
}

// GetRandomPasswordResponse is the response for GetRandomPassword.
type GetRandomPasswordResponse struct {
	RandomPassword string `json:"RandomPassword"`
}

// ErrorResponse represents a Secrets Manager error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// SecretError represents a Secrets Manager error.
type SecretError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *SecretError) Error() string {
	return e.Message
}

// ExternalSecretRotationMetadataItem represents the metadata for external secret rotation.
type ExternalSecretRotationMetadataItem struct {
	Key   string `json:"Key,omitempty"`
	Value string `json:"Value,omitempty"`
}

// GetResourcePolicyRequest is the request for GetResourcePolicy.
type GetResourcePolicyRequest struct {
	SecretID string `json:"SecretId"`
}

// GetResourcePolicyResponse is the response for GetResourcePolicy.
//
// AWS responds with ARN + Name and omits the ResourcePolicy field when no
// policy is attached. terraform-provider-aws treats the missing field as
// "no policy attached".
type GetResourcePolicyResponse struct {
	ARN            string `json:"ARN"`
	Name           string `json:"Name"`
	ResourcePolicy string `json:"ResourcePolicy,omitempty"`
}

// PutResourcePolicyRequest is the request for PutResourcePolicy.
type PutResourcePolicyRequest struct {
	SecretID       string `json:"SecretId"`
	ResourcePolicy string `json:"ResourcePolicy"`
}

// PutResourcePolicyResponse is the response for PutResourcePolicy.
type PutResourcePolicyResponse struct {
	ARN  string `json:"ARN"`
	Name string `json:"Name"`
}

// DeleteResourcePolicyRequest is the request for DeleteResourcePolicy.
type DeleteResourcePolicyRequest struct {
	SecretID string `json:"SecretId"`
}

// DeleteResourcePolicyResponse is the response for DeleteResourcePolicy.
type DeleteResourcePolicyResponse struct {
	ARN  string `json:"ARN"`
	Name string `json:"Name"`
}
