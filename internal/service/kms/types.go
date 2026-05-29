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

// SigningAlgorithm represents a signing algorithm spec.
type SigningAlgorithm string

// Signing algorithms.
const (
	SigningAlgorithmRSASSAPSSSha256      SigningAlgorithm = "RSASSA_PSS_SHA_256"
	SigningAlgorithmRSASSAPSSSha384      SigningAlgorithm = "RSASSA_PSS_SHA_384"
	SigningAlgorithmRSASSAPSSSha512      SigningAlgorithm = "RSASSA_PSS_SHA_512"
	SigningAlgorithmRSASSAPKCS1V15Sha256 SigningAlgorithm = "RSASSA_PKCS1_V1_5_SHA_256"
	SigningAlgorithmRSASSAPKCS1V15Sha384 SigningAlgorithm = "RSASSA_PKCS1_V1_5_SHA_384"
	SigningAlgorithmRSASSAPKCS1V15Sha512 SigningAlgorithm = "RSASSA_PKCS1_V1_5_SHA_512"
	SigningAlgorithmECDSASha256          SigningAlgorithm = "ECDSA_SHA_256"
	SigningAlgorithmECDSASha384          SigningAlgorithm = "ECDSA_SHA_384"
	SigningAlgorithmECDSASha512          SigningAlgorithm = "ECDSA_SHA_512"
)

// EncryptionAlgorithm represents an encryption algorithm spec.
type EncryptionAlgorithm string

// Encryption algorithms.
const (
	EncryptionAlgorithmSymmetricDefault EncryptionAlgorithm = "SYMMETRIC_DEFAULT"
	EncryptionAlgorithmRSAESOAEPSha1    EncryptionAlgorithm = "RSAES_OAEP_SHA_1"
	EncryptionAlgorithmRSAESOAEPSha256  EncryptionAlgorithm = "RSAES_OAEP_SHA_256"
)

// MessageType represents whether a Sign/Verify message is RAW or a DIGEST.
type MessageType string

// Message types.
const (
	MessageTypeRaw    MessageType = "RAW"
	MessageTypeDigest MessageType = "DIGEST"
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
	// Simulated key material for symmetric encryption/decryption (AES-GCM).
	KeyMaterial []byte
	// AsymmetricKey holds the PKCS#8 DER-encoded private key for asymmetric
	// (RSA/ECC) keys used by Sign/Verify/GetPublicKey. Nil for symmetric keys.
	AsymmetricKey []byte
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

// GetKeyPolicyRequest is the request for GetKeyPolicy.
type GetKeyPolicyRequest struct {
	KeyID      string `json:"KeyId"`
	PolicyName string `json:"PolicyName,omitempty"`
}

// GetKeyPolicyResponse is the response for GetKeyPolicy.
type GetKeyPolicyResponse struct {
	Policy     string `json:"Policy"`
	PolicyName string `json:"PolicyName"`
}

// PutKeyPolicyRequest is the request for PutKeyPolicy.
type PutKeyPolicyRequest struct {
	KeyID                          string `json:"KeyId"`
	Policy                         string `json:"Policy"`
	PolicyName                     string `json:"PolicyName,omitempty"`
	BypassPolicyLockoutSafetyCheck bool   `json:"BypassPolicyLockoutSafetyCheck,omitempty"`
}

// PutKeyPolicyResponse is the response for PutKeyPolicy.
type PutKeyPolicyResponse struct{}

// ListKeyPoliciesRequest is the request for ListKeyPolicies.
type ListKeyPoliciesRequest struct {
	KeyID  string `json:"KeyId"`
	Limit  int32  `json:"Limit,omitempty"`
	Marker string `json:"Marker,omitempty"`
}

// ListKeyPoliciesResponse is the response for ListKeyPolicies.
type ListKeyPoliciesResponse struct {
	PolicyNames []string `json:"PolicyNames"`
	NextMarker  string   `json:"NextMarker,omitempty"`
	Truncated   bool     `json:"Truncated"`
}

// ListResourceTagsRequest is the request for ListResourceTags.
type ListResourceTagsRequest struct {
	KeyID  string `json:"KeyId"`
	Limit  int32  `json:"Limit,omitempty"`
	Marker string `json:"Marker,omitempty"`
}

// ListResourceTagsResponse is the response for ListResourceTags.
// The Tags field must be present even when empty so terraform-provider-aws
// can parse it.
type ListResourceTagsResponse struct {
	Tags       []Tag  `json:"Tags"`
	NextMarker string `json:"NextMarker,omitempty"`
	Truncated  bool   `json:"Truncated"`
}

// TagResourceRequest is the request for TagResource.
type TagResourceRequest struct {
	KeyID string `json:"KeyId"`
	Tags  []Tag  `json:"Tags"`
}

// TagResourceResponse is the response for TagResource.
type TagResourceResponse struct{}

// UntagResourceRequest is the request for UntagResource.
type UntagResourceRequest struct {
	KeyID   string   `json:"KeyId"`
	TagKeys []string `json:"TagKeys"`
}

// UntagResourceResponse is the response for UntagResource.
type UntagResourceResponse struct{}

// GetKeyRotationStatusRequest is the request for GetKeyRotationStatus.
type GetKeyRotationStatusRequest struct {
	KeyID string `json:"KeyId"`
}

// GetKeyRotationStatusResponse is the response for GetKeyRotationStatus.
type GetKeyRotationStatusResponse struct {
	KeyRotationEnabled bool `json:"KeyRotationEnabled"`
}

// SignRequest is the request for Sign.
type SignRequest struct {
	KeyID            string   `json:"KeyId"`
	Message          []byte   `json:"Message"`
	MessageType      string   `json:"MessageType,omitempty"`
	SigningAlgorithm string   `json:"SigningAlgorithm"`
	GrantTokens      []string `json:"GrantTokens,omitempty"`
	DryRun           bool     `json:"DryRun,omitempty"`
}

// SignResponse is the response for Sign.
type SignResponse struct {
	KeyID            string `json:"KeyId"`
	Signature        []byte `json:"Signature"`
	SigningAlgorithm string `json:"SigningAlgorithm"`
}

// VerifyRequest is the request for Verify.
type VerifyRequest struct {
	KeyID            string   `json:"KeyId"`
	Message          []byte   `json:"Message"`
	Signature        []byte   `json:"Signature"`
	MessageType      string   `json:"MessageType,omitempty"`
	SigningAlgorithm string   `json:"SigningAlgorithm"`
	GrantTokens      []string `json:"GrantTokens,omitempty"`
	DryRun           bool     `json:"DryRun,omitempty"`
}

// VerifyResponse is the response for Verify.
type VerifyResponse struct {
	KeyID            string `json:"KeyId"`
	SignatureValid   bool   `json:"SignatureValid"`
	SigningAlgorithm string `json:"SigningAlgorithm"`
}

// GetPublicKeyRequest is the request for GetPublicKey.
type GetPublicKeyRequest struct {
	KeyID       string   `json:"KeyId"`
	GrantTokens []string `json:"GrantTokens,omitempty"`
}

// GetPublicKeyResponse is the response for GetPublicKey.
type GetPublicKeyResponse struct {
	KeyID                 string   `json:"KeyId"`
	PublicKey             []byte   `json:"PublicKey"`
	CustomerMasterKeySpec string   `json:"CustomerMasterKeySpec,omitempty"`
	KeySpec               string   `json:"KeySpec,omitempty"`
	KeyUsage              string   `json:"KeyUsage,omitempty"`
	EncryptionAlgorithms  []string `json:"EncryptionAlgorithms,omitempty"`
	SigningAlgorithms     []string `json:"SigningAlgorithms,omitempty"`
}
