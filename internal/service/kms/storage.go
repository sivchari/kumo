package kms

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "000000000000"
)

// defaultKeyPolicy is the AWS-default key policy returned for any key when
// no explicit policy has been set. terraform-provider-aws hashes this for
// drift detection so it must be a stable JSON document with the standard
// AccountRootEnable statement.
const defaultKeyPolicy = `{"Version":"2012-10-17","Id":"key-default-1","Statement":[{"Sid":"Enable IAM User Permissions","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::000000000000:root"},"Action":"kms:*","Resource":"*"}]}`

// Error codes.
const (
	errNotFound          = "NotFoundException"
	errInvalidKeyState   = "KMSInvalidStateException"
	errAlreadyExists     = "AlreadyExistsException"
	errInvalidAlias      = "InvalidAliasNameException"
	errDependencyTimeout = "DependencyTimeoutException"
	errInvalidCiphertext = "InvalidCiphertextException"
	errIncorrectKey      = "IncorrectKeyException"
	errDisabled          = "DisabledException"
	errInvalidKeyUsage   = "InvalidKeyUsageException"
)

// determineKeySize returns the key size based on key spec or number of bytes.
func determineKeySize(keySpec string, numberOfBytes int32) int32 {
	switch keySpec {
	case "AES_256":
		return 32
	case "AES_128":
		return 16
	default:
		if numberOfBytes > 0 {
			return numberOfBytes
		}

		return 32 // Default to AES-256
	}
}

// Storage defines the KMS storage interface.
type Storage interface {
	// Key operations.
	CreateKey(ctx context.Context, req *CreateKeyRequest) (*Key, error)
	GetKey(ctx context.Context, keyID string) (*Key, error)
	ListKeys(ctx context.Context, limit int32, marker string) ([]*Key, string, error)
	EnableKey(ctx context.Context, keyID string) error
	DisableKey(ctx context.Context, keyID string) error
	ScheduleKeyDeletion(ctx context.Context, keyID string, pendingWindowInDays int32) (*Key, error)

	// Cryptographic operations.
	Encrypt(ctx context.Context, keyID string, plaintext []byte, encryptionContext map[string]string) ([]byte, error)
	Decrypt(ctx context.Context, ciphertextBlob []byte, encryptionContext map[string]string, keyID string) ([]byte, string, error)
	GenerateDataKey(ctx context.Context, keyID string, keySpec string, numberOfBytes int32, encryptionContext map[string]string) ([]byte, []byte, error)

	// Policy operations.
	GetKeyPolicy(ctx context.Context, keyID string) (string, error)
	PutKeyPolicy(ctx context.Context, keyID, policy string) error

	// Tag operations.
	ListResourceTags(ctx context.Context, keyID string) ([]Tag, error)
	TagResource(ctx context.Context, keyID string, tags []Tag) error
	UntagResource(ctx context.Context, keyID string, tagKeys []string) error

	// Rotation operations.
	GetKeyRotationStatus(ctx context.Context, keyID string) (bool, error)

	// Alias operations.
	CreateAlias(ctx context.Context, aliasName, targetKeyID string) error
	DeleteAlias(ctx context.Context, aliasName string) error
	ListAliases(ctx context.Context, keyID string, limit int32, marker string) ([]*Alias, string, error)
	GetAlias(ctx context.Context, aliasName string) (*Alias, error)
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu      sync.RWMutex      `json:"-"`
	Keys    map[string]*Key   `json:"keys"`    // keyID -> Key
	Aliases map[string]*Alias `json:"aliases"` // aliasName -> Alias
	region  string
	dataDir string
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	s := &MemoryStorage{
		Keys:    make(map[string]*Key),
		Aliases: make(map[string]*Alias),
		region:  region,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "kms", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type MemStorageAlias MemoryStorage

	data, err := json.Marshal(&struct{ *MemStorageAlias }{MemStorageAlias: (*MemStorageAlias)(s)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type MemStorageAlias MemoryStorage

	aux := &struct{ *MemStorageAlias }{MemStorageAlias: (*MemStorageAlias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Keys == nil {
		s.Keys = make(map[string]*Key)
	}

	if s.Aliases == nil {
		s.Aliases = make(map[string]*Alias)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
// It uses a type alias to avoid calling MarshalJSON (which would deadlock).
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(s)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(s.dataDir, "kms", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "kms", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateKey creates a new KMS key.
func (s *MemoryStorage) CreateKey(_ context.Context, req *CreateKeyRequest) (*Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	keyID := uuid.New().String()
	arn := fmt.Sprintf("arn:aws:kms:%s:%s:key/%s", s.region, defaultAccountID, keyID)

	// Generate random key material (256-bit for AES-256).
	keyMaterial := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, keyMaterial); err != nil {
		return nil, &ServiceError{Code: errDependencyTimeout, Message: "Failed to generate key material"}
	}

	keyUsage := KeyUsageEncryptDecrypt
	if req.KeyUsage != "" {
		keyUsage = KeyUsage(req.KeyUsage)
	}

	keySpec := KeySpecSymmetricDefault
	if req.KeySpec != "" {
		keySpec = KeySpec(req.KeySpec)
	}

	origin := "AWS_KMS"
	if req.Origin != "" {
		origin = req.Origin
	}

	tags := make(map[string]string)
	for _, tag := range req.Tags {
		tags[tag.TagKey] = tag.TagValue
	}

	policy := req.Policy
	if policy == "" {
		policy = defaultKeyPolicy
	}

	key := &Key{
		KeyID:        keyID,
		Arn:          arn,
		Description:  req.Description,
		KeyState:     KeyStateEnabled,
		KeyUsage:     keyUsage,
		KeySpec:      keySpec,
		KeyManager:   KeyManagerCustomer,
		CreationDate: time.Now(),
		Enabled:      true,
		Origin:       origin,
		MultiRegion:  req.MultiRegion,
		Tags:         tags,
		Policy:       policy,
		KeyMaterial:  keyMaterial,
	}

	s.Keys[keyID] = key
	s.saveLocked()

	return key, nil
}

// GetKey retrieves a key by ID, ARN, or alias.
func (s *MemoryStorage) GetKey(_ context.Context, keyID string) (*Key, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getKeyLocked(keyID)
}

// getKeyLocked retrieves a key without locking (caller must hold lock).
func (s *MemoryStorage) getKeyLocked(keyID string) (*Key, error) {
	// Check if it's an alias.
	if len(keyID) > 6 && keyID[:6] == "alias/" {
		alias, ok := s.Aliases[keyID]
		if !ok {
			return nil, &ServiceError{Code: errNotFound, Message: "Alias " + keyID + " is not found."}
		}

		keyID = alias.TargetKeyID
	}

	// Check if it's an ARN.
	if len(keyID) > 8 && keyID[:8] == "arn:aws:" {
		// Extract key ID from ARN.
		for _, key := range s.Keys {
			if key.Arn == keyID {
				return key, nil
			}
		}

		return nil, &ServiceError{Code: errNotFound, Message: "Key " + keyID + " is not found."}
	}

	// Look up by key ID.
	key, ok := s.Keys[keyID]
	if !ok {
		return nil, &ServiceError{Code: errNotFound, Message: "Key " + keyID + " is not found."}
	}

	return key, nil
}

// ListKeys lists all keys.
func (s *MemoryStorage) ListKeys(_ context.Context, limit int32, _ string) ([]*Key, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	keys := make([]*Key, 0, len(s.Keys))
	maxKeys := int(limit)

	for _, key := range s.Keys {
		keys = append(keys, key)

		if len(keys) >= maxKeys {
			break
		}
	}

	return keys, "", nil
}

// EnableKey enables a key.
func (s *MemoryStorage) EnableKey(_ context.Context, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, err := s.getKeyLocked(keyID)
	if err != nil {
		return err
	}

	if key.KeyState == KeyStatePendingDeletion {
		return &ServiceError{
			Code:    errInvalidKeyState,
			Message: "Key " + keyID + " is pending deletion.",
		}
	}

	key.KeyState = KeyStateEnabled
	key.Enabled = true

	s.saveLocked()

	return nil
}

// DisableKey disables a key.
func (s *MemoryStorage) DisableKey(_ context.Context, keyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, err := s.getKeyLocked(keyID)
	if err != nil {
		return err
	}

	if key.KeyState == KeyStatePendingDeletion {
		return &ServiceError{
			Code:    errInvalidKeyState,
			Message: "Key " + keyID + " is pending deletion.",
		}
	}

	key.KeyState = KeyStateDisabled
	key.Enabled = false

	s.saveLocked()

	return nil
}

// ScheduleKeyDeletion schedules a key for deletion.
func (s *MemoryStorage) ScheduleKeyDeletion(_ context.Context, keyID string, pendingWindowInDays int32) (*Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, err := s.getKeyLocked(keyID)
	if err != nil {
		return nil, err
	}

	if key.KeyState == KeyStatePendingDeletion {
		return nil, &ServiceError{
			Code:    errInvalidKeyState,
			Message: "Key " + keyID + " is pending deletion.",
		}
	}

	if pendingWindowInDays < 7 || pendingWindowInDays > 30 {
		pendingWindowInDays = 30
	}

	deletionDate := time.Now().AddDate(0, 0, int(pendingWindowInDays))
	key.KeyState = KeyStatePendingDeletion
	key.Enabled = false
	key.DeletionDate = &deletionDate
	key.PendingDeletionWindow = pendingWindowInDays

	s.saveLocked()

	return key, nil
}

// Encrypt encrypts plaintext using a key.
func (s *MemoryStorage) Encrypt(_ context.Context, keyID string, plaintext []byte, _ map[string]string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, err := s.getKeyLocked(keyID)
	if err != nil {
		return nil, err
	}

	if key.KeyState != KeyStateEnabled {
		return nil, &ServiceError{
			Code:    errDisabled,
			Message: "Key " + keyID + " is disabled.",
		}
	}

	if key.KeyUsage != KeyUsageEncryptDecrypt {
		return nil, &ServiceError{
			Code:    errInvalidKeyUsage,
			Message: "Key " + keyID + " is not configured for encryption.",
		}
	}

	// Use AES-GCM for encryption.
	block, err := aes.NewCipher(key.KeyMaterial)
	if err != nil {
		return nil, &ServiceError{Code: errDependencyTimeout, Message: "Encryption failed"}
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, &ServiceError{Code: errDependencyTimeout, Message: "Encryption failed"}
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, &ServiceError{Code: errDependencyTimeout, Message: "Encryption failed"}
	}

	// Prepend key ID (36 bytes UUID) + nonce to ciphertext for decryption lookup.
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	result := make([]byte, 0, 36+len(nonce)+len(ciphertext))
	result = append(result, []byte(key.KeyID)...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	return result, nil
}

// Decrypt decrypts ciphertext.
func (s *MemoryStorage) Decrypt(_ context.Context, ciphertextBlob []byte, _ map[string]string, requestKeyID string) ([]byte, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Extract key ID from ciphertext (first 36 bytes).
	if len(ciphertextBlob) < 36 {
		return nil, "", &ServiceError{Code: errInvalidCiphertext, Message: "Invalid ciphertext"}
	}

	embeddedKeyID := string(ciphertextBlob[:36])

	// If a key ID was specified, verify it matches.
	if requestKeyID != "" {
		key, err := s.getKeyLocked(requestKeyID)
		if err != nil {
			return nil, "", err
		}

		if key.KeyID != embeddedKeyID {
			return nil, "", &ServiceError{Code: errIncorrectKey, Message: "The key ID in the ciphertext does not match the specified key."}
		}
	}

	key, err := s.getKeyLocked(embeddedKeyID)
	if err != nil {
		return nil, "", err
	}

	if key.KeyState != KeyStateEnabled {
		return nil, "", &ServiceError{
			Code:    errDisabled,
			Message: "Key " + embeddedKeyID + " is disabled.",
		}
	}

	// Use AES-GCM for decryption.
	block, err := aes.NewCipher(key.KeyMaterial)
	if err != nil {
		return nil, "", &ServiceError{Code: errDependencyTimeout, Message: "Decryption failed"}
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", &ServiceError{Code: errDependencyTimeout, Message: "Decryption failed"}
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertextBlob) < 36+nonceSize {
		return nil, "", &ServiceError{Code: errInvalidCiphertext, Message: "Invalid ciphertext"}
	}

	nonce := ciphertextBlob[36 : 36+nonceSize]
	ciphertext := ciphertextBlob[36+nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, "", &ServiceError{Code: errInvalidCiphertext, Message: "Invalid ciphertext"}
	}

	return plaintext, key.KeyID, nil
}

// GenerateDataKey generates a data key.
func (s *MemoryStorage) GenerateDataKey(_ context.Context, keyID, keySpec string, numberOfBytes int32, _ map[string]string) ([]byte, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, err := s.getKeyLocked(keyID)
	if err != nil {
		return nil, nil, err
	}

	if key.KeyState != KeyStateEnabled {
		return nil, nil, &ServiceError{
			Code:    errDisabled,
			Message: "Key " + keyID + " is disabled.",
		}
	}

	if key.KeyUsage != KeyUsageEncryptDecrypt {
		return nil, nil, &ServiceError{
			Code:    errInvalidKeyUsage,
			Message: "Key " + keyID + " is not configured for encryption.",
		}
	}

	keySize := determineKeySize(keySpec, numberOfBytes)

	// Generate plaintext data key.
	plaintext := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, plaintext); err != nil {
		return nil, nil, &ServiceError{Code: errDependencyTimeout, Message: "Failed to generate data key"}
	}

	// Encrypt the data key using the KMS key.
	block, err := aes.NewCipher(key.KeyMaterial)
	if err != nil {
		return nil, nil, &ServiceError{Code: errDependencyTimeout, Message: "Encryption failed"}
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, &ServiceError{Code: errDependencyTimeout, Message: "Encryption failed"}
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, &ServiceError{Code: errDependencyTimeout, Message: "Encryption failed"}
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	encryptedKey := make([]byte, 0, 36+len(nonce)+len(ciphertext))
	encryptedKey = append(encryptedKey, []byte(key.KeyID)...)
	encryptedKey = append(encryptedKey, nonce...)
	encryptedKey = append(encryptedKey, ciphertext...)

	return plaintext, encryptedKey, nil
}

// CreateAlias creates an alias for a key.
func (s *MemoryStorage) CreateAlias(_ context.Context, aliasName, targetKeyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate alias name.
	if len(aliasName) < 7 || aliasName[:6] != "alias/" {
		return &ServiceError{Code: errInvalidAlias, Message: "Alias must begin with 'alias/'"}
	}

	// Check if alias already exists.
	if _, ok := s.Aliases[aliasName]; ok {
		return &ServiceError{Code: errAlreadyExists, Message: "Alias " + aliasName + " already exists."}
	}

	// Verify target key exists.
	key, err := s.getKeyLocked(targetKeyID)
	if err != nil {
		return err
	}

	aliasArn := fmt.Sprintf("arn:aws:kms:%s:%s:%s", s.region, defaultAccountID, aliasName)
	now := time.Now()

	s.Aliases[aliasName] = &Alias{
		AliasName:       aliasName,
		AliasArn:        aliasArn,
		TargetKeyID:     key.KeyID,
		CreationDate:    now,
		LastUpdatedDate: now,
	}
	s.saveLocked()

	return nil
}

// DeleteAlias deletes an alias.
func (s *MemoryStorage) DeleteAlias(_ context.Context, aliasName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Aliases[aliasName]; !ok {
		return &ServiceError{Code: errNotFound, Message: "Alias " + aliasName + " is not found."}
	}

	delete(s.Aliases, aliasName)
	s.saveLocked()

	return nil
}

// ListAliases lists aliases.
func (s *MemoryStorage) ListAliases(_ context.Context, keyID string, limit int32, _ string) ([]*Alias, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	aliases := make([]*Alias, 0)
	maxAliases := int(limit)

	for _, alias := range s.Aliases {
		if keyID != "" && alias.TargetKeyID != keyID {
			// If keyID is specified, filter by it.
			// Also need to resolve keyID if it's an alias or ARN.
			resolvedKey, err := s.getKeyLocked(keyID)
			if err != nil {
				continue
			}

			if alias.TargetKeyID != resolvedKey.KeyID {
				continue
			}
		}

		aliases = append(aliases, alias)

		if len(aliases) >= maxAliases {
			break
		}
	}

	return aliases, "", nil
}

// GetAlias retrieves an alias by name.
func (s *MemoryStorage) GetAlias(_ context.Context, aliasName string) (*Alias, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alias, ok := s.Aliases[aliasName]
	if !ok {
		return nil, &ServiceError{Code: errNotFound, Message: "Alias " + aliasName + " is not found."}
	}

	return alias, nil
}

// GetKeyPolicy returns the policy for a key.
func (s *MemoryStorage) GetKeyPolicy(_ context.Context, keyID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, err := s.getKeyLocked(keyID)
	if err != nil {
		return "", err
	}

	policy := key.Policy
	if policy == "" {
		policy = defaultKeyPolicy
	}

	return policy, nil
}

// PutKeyPolicy sets the policy for a key.
func (s *MemoryStorage) PutKeyPolicy(_ context.Context, keyID, policy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, err := s.getKeyLocked(keyID)
	if err != nil {
		return err
	}

	key.Policy = policy

	s.saveLocked()

	return nil
}

// ListResourceTags returns the tags for a key.
func (s *MemoryStorage) ListResourceTags(_ context.Context, keyID string) ([]Tag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, err := s.getKeyLocked(keyID)
	if err != nil {
		return nil, err
	}

	tags := make([]Tag, 0, len(key.Tags))
	for k, v := range key.Tags {
		tags = append(tags, Tag{TagKey: k, TagValue: v})
	}

	return tags, nil
}

// TagResource adds tags to a key.
func (s *MemoryStorage) TagResource(_ context.Context, keyID string, tags []Tag) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, err := s.getKeyLocked(keyID)
	if err != nil {
		return err
	}

	if key.Tags == nil {
		key.Tags = make(map[string]string)
	}

	for _, tag := range tags {
		key.Tags[tag.TagKey] = tag.TagValue
	}

	s.saveLocked()

	return nil
}

// UntagResource removes tags from a key.
func (s *MemoryStorage) UntagResource(_ context.Context, keyID string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, err := s.getKeyLocked(keyID)
	if err != nil {
		return err
	}

	for _, tagKey := range tagKeys {
		delete(key.Tags, tagKey)
	}

	s.saveLocked()

	return nil
}

// GetKeyRotationStatus returns the rotation status for a key.
func (s *MemoryStorage) GetKeyRotationStatus(_ context.Context, keyID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, err := s.getKeyLocked(keyID)
	if err != nil {
		return false, err
	}

	// Rotation is not modeled in storage; always report disabled.
	return false, nil
}
