package kms

import (
	"time"
)

// KeyState represents the state of a KMS key.
type KeyState string

// Key states.
const (
	KeyStateEnabled         KeyState = "Enabled"
	KeyStateDisabled        KeyState = "Disabled"
	KeyStatePendingDeletion KeyState = "PendingDeletion"
	KeyStatePendingImport   KeyState = "PendingImport"
	KeyStateUnavailable     KeyState = "Unavailable"
)

// KeyUsage represents the cryptographic operations for which you can use the key.
type KeyUsage string

// Key usages.
const (
	KeyUsageEncryptDecrypt KeyUsage = "ENCRYPT_DECRYPT"
	KeyUsageSignVerify     KeyUsage = "SIGN_VERIFY"
	KeyUsageGenerateVerify KeyUsage = "GENERATE_VERIFY_MAC"
)

// KeySpec represents the type of KMS key.
type KeySpec string

// Key specs.
const (
	KeySpecSymmetricDefault KeySpec = "SYMMETRIC_DEFAULT"
	KeySpecRSA2048          KeySpec = "RSA_2048"
	KeySpecRSA3072          KeySpec = "RSA_3072"
	KeySpecRSA4096          KeySpec = "RSA_4096"
	KeySpecECCNistP256      KeySpec = "ECC_NIST_P256"
	KeySpecECCNistP384      KeySpec = "ECC_NIST_P384"
	KeySpecECCNistP521      KeySpec = "ECC_NIST_P521"
	KeySpecECCSecgP256K1    KeySpec = "ECC_SECG_P256K1"
	KeySpecHMAC224          KeySpec = "HMAC_224"
	KeySpecHMAC256          KeySpec = "HMAC_256"
	KeySpecHMAC384          KeySpec = "HMAC_384"
	KeySpecHMAC512          KeySpec = "HMAC_512"
)

// KeyManager represents who manages the key material.
type KeyManager string

// Key managers.
const (
	KeyManagerAWS      KeyManager = "AWS"
	KeyManagerCustomer KeyManager = "CUSTOMER"
)

// Key represents a KMS key.
type Key struct {
	KeyID                 string
	Arn                   string
	Alias                 string
	Description           string
	KeyState              KeyState
	KeyUsage              KeyUsage
	KeySpec               KeySpec
	KeyManager            KeyManager
	CreationDate          time.Time
	Enabled               bool
	DeletionDate          *time.Time
	ValidTo               *time.Time
	Origin                string
	ExpirationModel       string
	MultiRegion           bool
	MultiRegionConfig     *MultiRegionConfig
	PendingDeletionWindow int32
	Tags                  map[string]string
	Policy                string // IAM key policy document
	// Simulated key material for encryption/decryption.
	KeyMaterial []byte
}

// MultiRegionConfig represents multi-region key configuration.
type MultiRegionConfig struct {
	MultiRegionKeyType string
	PrimaryKey         *MultiRegionKey
	ReplicaKeys        []MultiRegionKey
}

// MultiRegionKey represents a multi-region key.
type MultiRegionKey struct {
	Arn    string
	Region string
}

// Alias represents a KMS key alias.
type Alias struct {
	AliasName       string
	AliasArn        string
	TargetKeyID     string
	CreationDate    time.Time
	LastUpdatedDate time.Time
}

// CreateKeyRequest is the request for CreateKey.
type CreateKeyRequest struct {
	Description         string `json:"Description,omitempty"`
	KeyUsage            string `json:"KeyUsage,omitempty"`
	KeySpec             string `json:"KeySpec,omitempty"`
	Origin              string `json:"Origin,omitempty"`
	CustomKeyStoreID    string `json:"CustomKeyStoreId,omitempty"`
	BypassPolicyLockout bool   `json:"BypassPolicyLockoutSafetyCheck,omitempty"`
	Policy              string `json:"Policy,omitempty"`
	Tags                []Tag  `json:"Tags,omitempty"`
	MultiRegion         bool   `json:"MultiRegion,omitempty"`
	XksKeyID            string `json:"XksKeyId,omitempty"`
}

// Tag represents a tag.
type Tag struct {
	TagKey   string `json:"TagKey"`
	TagValue string `json:"TagValue"`
}

// CreateKeyResponse is the response for CreateKey.
type CreateKeyResponse struct {
	KeyMetadata *KeyMetadata `json:"KeyMetadata"`
}

// KeyMetadata represents key metadata in API responses.
type KeyMetadata struct {
	AWSAccountID          string             `json:"AWSAccountId,omitempty"`
	KeyID                 string             `json:"KeyId"`
	Arn                   string             `json:"Arn"`
	CreationDate          float64            `json:"CreationDate"`
	Enabled               bool               `json:"Enabled"`
	Description           string             `json:"Description,omitempty"`
	KeyUsage              string             `json:"KeyUsage,omitempty"`
	KeyState              string             `json:"KeyState"`
	DeletionDate          *float64           `json:"DeletionDate,omitempty"`
	ValidTo               *float64           `json:"ValidTo,omitempty"`
	Origin                string             `json:"Origin,omitempty"`
	CustomKeyStoreID      string             `json:"CustomKeyStoreId,omitempty"`
	CloudHsmClusterID     string             `json:"CloudHsmClusterId,omitempty"`
	ExpirationModel       string             `json:"ExpirationModel,omitempty"`
	KeyManager            string             `json:"KeyManager,omitempty"`
	KeySpec               string             `json:"KeySpec,omitempty"`
	EncryptionAlgorithms  []string           `json:"EncryptionAlgorithms,omitempty"`
	SigningAlgorithms     []string           `json:"SigningAlgorithms,omitempty"`
	MultiRegion           bool               `json:"MultiRegion,omitempty"`
	MultiRegionConfig     *MultiRegionOutput `json:"MultiRegionConfiguration,omitempty"`
	PendingDeletionWindow *int32             `json:"PendingDeletionWindowInDays,omitempty"`
	MacAlgorithms         []string           `json:"MacAlgorithms,omitempty"`
	XksKeyConfig          *XksKeyConfigType  `json:"XksKeyConfiguration,omitempty"`
}

// MultiRegionOutput represents multi-region config in response.
type MultiRegionOutput struct {
	MultiRegionKeyType string               `json:"MultiRegionKeyType,omitempty"`
	PrimaryKey         *MultiRegionKeyInfo  `json:"PrimaryKey,omitempty"`
	ReplicaKeys        []MultiRegionKeyInfo `json:"ReplicaKeys,omitempty"`
}

// MultiRegionKeyInfo represents multi-region key info.
type MultiRegionKeyInfo struct {
	Arn    string `json:"Arn,omitempty"`
	Region string `json:"Region,omitempty"`
}

// XksKeyConfigType represents XKS key configuration.
type XksKeyConfigType struct {
	ID string `json:"Id,omitempty"`
}

// DescribeKeyRequest is the request for DescribeKey.
type DescribeKeyRequest struct {
	KeyID       string   `json:"KeyId"`
	GrantTokens []string `json:"GrantTokens,omitempty"`
}

// DescribeKeyResponse is the response for DescribeKey.
type DescribeKeyResponse struct {
	KeyMetadata *KeyMetadata `json:"KeyMetadata"`
}

// ListKeysRequest is the request for ListKeys.
type ListKeysRequest struct {
	Limit  int32  `json:"Limit,omitempty"`
	Marker string `json:"Marker,omitempty"`
}

// ListKeysResponse is the response for ListKeys.
type ListKeysResponse struct {
	Keys       []KeyListEntry `json:"Keys"`
	NextMarker string         `json:"NextMarker,omitempty"`
	Truncated  bool           `json:"Truncated"`
}

// KeyListEntry represents a key in list response.
type KeyListEntry struct {
	KeyID  string `json:"KeyId"`
	KeyArn string `json:"KeyArn"`
}

// EnableKeyRequest is the request for EnableKey.
type EnableKeyRequest struct {
	KeyID string `json:"KeyId"`
}

// EnableKeyResponse is the response for EnableKey.
type EnableKeyResponse struct{}

// DisableKeyRequest is the request for DisableKey.
type DisableKeyRequest struct {
	KeyID string `json:"KeyId"`
}

// DisableKeyResponse is the response for DisableKey.
type DisableKeyResponse struct{}

// ScheduleKeyDeletionRequest is the request for ScheduleKeyDeletion.
type ScheduleKeyDeletionRequest struct {
	KeyID               string `json:"KeyId"`
	PendingWindowInDays int32  `json:"PendingWindowInDays,omitempty"`
}

// ScheduleKeyDeletionResponse is the response for ScheduleKeyDeletion.
type ScheduleKeyDeletionResponse struct {
	KeyID               string  `json:"KeyId"`
	DeletionDate        float64 `json:"DeletionDate"`
	KeyState            string  `json:"KeyState"`
	PendingWindowInDays int32   `json:"PendingWindowInDays,omitempty"`
}

// EncryptRequest is the request for Encrypt.
type EncryptRequest struct {
	KeyID               string            `json:"KeyId"`
	Plaintext           []byte            `json:"Plaintext"`
	EncryptionContext   map[string]string `json:"EncryptionContext,omitempty"`
	GrantTokens         []string          `json:"GrantTokens,omitempty"`
	EncryptionAlgorithm string            `json:"EncryptionAlgorithm,omitempty"`
	DryRun              bool              `json:"DryRun,omitempty"`
}

// EncryptResponse is the response for Encrypt.
type EncryptResponse struct {
	CiphertextBlob      []byte `json:"CiphertextBlob"`
	KeyID               string `json:"KeyId"`
	EncryptionAlgorithm string `json:"EncryptionAlgorithm,omitempty"`
}

// DecryptRequest is the request for Decrypt.
type DecryptRequest struct {
	CiphertextBlob      []byte            `json:"CiphertextBlob"`
	EncryptionContext   map[string]string `json:"EncryptionContext,omitempty"`
	GrantTokens         []string          `json:"GrantTokens,omitempty"`
	KeyID               string            `json:"KeyId,omitempty"`
	EncryptionAlgorithm string            `json:"EncryptionAlgorithm,omitempty"`
	Recipient           *RecipientInfo    `json:"Recipient,omitempty"`
	DryRun              bool              `json:"DryRun,omitempty"`
}

// RecipientInfo represents recipient info.
type RecipientInfo struct {
	KeyEncryptionAlgorithm string `json:"KeyEncryptionAlgorithm,omitempty"`
	AttestationDocument    []byte `json:"AttestationDocument,omitempty"`
}

// DecryptResponse is the response for Decrypt.
type DecryptResponse struct {
	KeyID                  string `json:"KeyId"`
	Plaintext              []byte `json:"Plaintext,omitempty"`
	EncryptionAlgorithm    string `json:"EncryptionAlgorithm,omitempty"`
	CiphertextForRecipient []byte `json:"CiphertextForRecipient,omitempty"`
}

// GenerateDataKeyRequest is the request for GenerateDataKey.
type GenerateDataKeyRequest struct {
	KeyID             string            `json:"KeyId"`
	KeySpec           string            `json:"KeySpec,omitempty"`
	NumberOfBytes     int32             `json:"NumberOfBytes,omitempty"`
	EncryptionContext map[string]string `json:"EncryptionContext,omitempty"`
	GrantTokens       []string          `json:"GrantTokens,omitempty"`
	Recipient         *RecipientInfo    `json:"Recipient,omitempty"`
	DryRun            bool              `json:"DryRun,omitempty"`
}

// GenerateDataKeyResponse is the response for GenerateDataKey.
type GenerateDataKeyResponse struct {
	CiphertextBlob         []byte `json:"CiphertextBlob"`
	Plaintext              []byte `json:"Plaintext,omitempty"`
	KeyID                  string `json:"KeyId"`
	CiphertextForRecipient []byte `json:"CiphertextForRecipient,omitempty"`
}

// CreateAliasRequest is the request for CreateAlias.
type CreateAliasRequest struct {
	AliasName   string `json:"AliasName"`
	TargetKeyID string `json:"TargetKeyId"`
}

// CreateAliasResponse is the response for CreateAlias.
type CreateAliasResponse struct{}

// DeleteAliasRequest is the request for DeleteAlias.
type DeleteAliasRequest struct {
	AliasName string `json:"AliasName"`
}

// DeleteAliasResponse is the response for DeleteAlias.
type DeleteAliasResponse struct{}

// ListAliasesRequest is the request for ListAliases.
type ListAliasesRequest struct {
	KeyID  string `json:"KeyId,omitempty"`
	Limit  int32  `json:"Limit,omitempty"`
	Marker string `json:"Marker,omitempty"`
}

// ListAliasesResponse is the response for ListAliases.
type ListAliasesResponse struct {
	Aliases    []AliasListEntry `json:"Aliases"`
	NextMarker string           `json:"NextMarker,omitempty"`
	Truncated  bool             `json:"Truncated"`
}

// AliasListEntry represents an alias in list response.
type AliasListEntry struct {
	AliasName       string  `json:"AliasName"`
	AliasArn        string  `json:"AliasArn"`
	TargetKeyID     string  `json:"TargetKeyId,omitempty"`
	CreationDate    float64 `json:"CreationDate,omitempty"`
	LastUpdatedDate float64 `json:"LastUpdatedDate,omitempty"`
}

// ErrorResponse represents a KMS error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// ServiceError represents a KMS service error.
type ServiceError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *ServiceError) Error() string {
	return e.Message
}

// getKeyPolicyRequest is the wire shape of GetKeyPolicy.
type getKeyPolicyRequest struct {
	KeyID      string `json:"KeyId"`
	PolicyName string `json:"PolicyName,omitempty"`
}

// putKeyPolicyRequest is the wire shape of PutKeyPolicy.
type putKeyPolicyRequest struct {
	KeyID      string `json:"KeyId"`
	Policy     string `json:"Policy"`
	PolicyName string `json:"PolicyName,omitempty"`
}

// getKeyPolicyResponse is the wire shape of GetKeyPolicy.
type getKeyPolicyResponse struct {
	Policy     string `json:"Policy"`
	PolicyName string `json:"PolicyName"`
}

// listKeyPoliciesResponse is the wire shape of ListKeyPolicies.
type listKeyPoliciesResponse struct {
	PolicyNames []string `json:"PolicyNames"`
}

// listResourceTagsResponse mirrors AWS — the Tags field must be present
// even when empty so terraform-provider-aws can parse it.
type listResourceTagsResponse struct {
	Tags []map[string]string `json:"Tags"`
}

// getKeyRotationStatusResponse is the wire shape of GetKeyRotationStatus.
type getKeyRotationStatusResponse struct {
	KeyRotationEnabled bool `json:"KeyRotationEnabled"`
}
