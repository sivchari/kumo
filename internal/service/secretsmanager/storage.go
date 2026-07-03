package secretsmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion         = "us-east-1"
	defaultAccountID      = "000000000000"
	defaultRecoveryWindow = 30
	stageCurrent          = "AWSCURRENT"
	stagePrevious         = "AWSPREVIOUS"
)

// Storage defines the Secrets Manager storage interface.
type Storage interface {
	CreateSecret(ctx context.Context, req *CreateSecretRequest) (*Secret, error)
	GetSecretValue(ctx context.Context, secretID, versionID, versionStage string) (*Secret, *SecretVersion, error)
	PutSecretValue(ctx context.Context, secretID, clientToken, secretString string, secretBinary []byte, versionStages []string) (*Secret, *SecretVersion, error)
	DeleteSecret(ctx context.Context, secretID string, recoveryWindow int64, forceDelete bool) (*Secret, error)
	ListSecrets(ctx context.Context, maxResults int, nextToken string, includePlannedDeletion bool) ([]*Secret, string, error)
	DescribeSecret(ctx context.Context, secretID string) (*Secret, error)
	UpdateSecret(ctx context.Context, req *UpdateSecretRequest) (*Secret, *SecretVersion, error)
	GetResourcePolicy(ctx context.Context, secretID string) (*Secret, string, error)
	PutResourcePolicy(ctx context.Context, secretID, policy string) (*Secret, error)
	DeleteResourcePolicy(ctx context.Context, secretID string) (*Secret, error)
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
	mu       sync.RWMutex       `json:"-"`
	Secrets  map[string]*Secret `json:"secrets"`  // keyed by name
	Policies map[string]string  `json:"policies"` // keyed by secret name
	baseURL  string
	dataDir  string
}

// NewMemoryStorage creates a new in-memory Secrets Manager storage.
func NewMemoryStorage(baseURL string, opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Secrets:  make(map[string]*Secret),
		Policies: make(map[string]string),
		baseURL:  baseURL,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "secretsmanager", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (m *MemoryStorage) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(m)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (m *MemoryStorage) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(m)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if m.Secrets == nil {
		m.Secrets = make(map[string]*Secret)
	}

	if m.Policies == nil {
		m.Policies = make(map[string]string)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "secretsmanager", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "secretsmanager", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateSecret creates a new secret.
func (m *MemoryStorage) CreateSecret(_ context.Context, req *CreateSecretRequest) (*Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Secrets[req.Name]; exists {
		return nil, &SecretError{
			Code:    "ResourceExistsException",
			Message: fmt.Sprintf("The operation failed because the secret %s already exists.", req.Name),
		}
	}

	now := time.Now()
	versionID := uuid.New().String()

	secret := &Secret{
		ARN:             m.buildARN(req.Name),
		Name:            req.Name,
		Description:     req.Description,
		KmsKeyID:        req.KmsKeyID,
		VersionID:       versionID,
		SecretString:    req.SecretString,
		SecretBinary:    req.SecretBinary,
		CreatedDate:     now,
		LastChangedDate: now,
		Tags:            req.Tags,
		VersionIDs:      make(map[string]*SecretVersion),
	}

	// Create initial version if secret value is provided.
	if req.SecretString != "" || len(req.SecretBinary) > 0 {
		version := &SecretVersion{
			VersionID:     versionID,
			SecretString:  req.SecretString,
			SecretBinary:  req.SecretBinary,
			VersionStages: []string{stageCurrent},
			CreatedDate:   now,
			KmsKeyID:      req.KmsKeyID,
		}
		secret.VersionIDs[versionID] = version
	}

	m.Secrets[req.Name] = secret

	m.saveLocked()

	return secret, nil
}

// GetSecretValue retrieves a secret value.
func (m *MemoryStorage) GetSecretValue(_ context.Context, secretID, versionID, versionStage string) (*Secret, *SecretVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	secret := m.findSecret(secretID)
	if secret == nil {
		return nil, nil, &SecretError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Secrets Manager can't find the specified secret: %s", secretID),
		}
	}

	if secret.DeletedDate != nil {
		return nil, nil, &SecretError{
			Code:    "InvalidRequestException",
			Message: "You can't perform this operation on a secret that's scheduled for deletion.",
		}
	}

	// Default to AWSCURRENT if no version specified.
	if versionStage == "" && versionID == "" {
		versionStage = stageCurrent
	}

	var version *SecretVersion

	if versionID != "" {
		version = secret.VersionIDs[versionID]
	} else {
		// Find version by stage.
		for _, v := range secret.VersionIDs {
			if slices.Contains(v.VersionStages, versionStage) {
				version = v

				break
			}
		}
	}

	if version == nil {
		return nil, nil, &SecretError{
			Code:    "ResourceNotFoundException",
			Message: "Secrets Manager can't find the specified secret version.",
		}
	}

	// Update last accessed date.
	now := time.Now()
	secret.LastAccessedDate = &now

	return secret, version, nil
}

// PutSecretValue puts a new value into an existing secret.
func (m *MemoryStorage) PutSecretValue(_ context.Context, secretID, clientToken, secretString string, secretBinary []byte, versionStages []string) (*Secret, *SecretVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	secret := m.findSecret(secretID)
	if secret == nil {
		return nil, nil, &SecretError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Secrets Manager can't find the specified secret: %s", secretID),
		}
	}

	if secret.DeletedDate != nil {
		return nil, nil, &SecretError{
			Code:    "InvalidRequestException",
			Message: "You can't perform this operation on a secret that's scheduled for deletion.",
		}
	}

	now := time.Now()
	versionID := clientToken

	if versionID == "" {
		versionID = uuid.New().String()
	}

	// Default stages.
	if len(versionStages) == 0 {
		versionStages = []string{stageCurrent}
	}

	// Remove AWSCURRENT stage from previous version.
	m.rotateVersionStages(secret)

	version := &SecretVersion{
		VersionID:     versionID,
		SecretString:  secretString,
		SecretBinary:  secretBinary,
		VersionStages: versionStages,
		CreatedDate:   now,
		KmsKeyID:      secret.KmsKeyID,
	}

	secret.VersionIDs[versionID] = version
	secret.VersionID = versionID
	secret.SecretString = secretString
	secret.SecretBinary = secretBinary
	secret.LastChangedDate = now

	m.saveLocked()

	return secret, version, nil
}

// DeleteSecret marks a secret for deletion.
func (m *MemoryStorage) DeleteSecret(_ context.Context, secretID string, recoveryWindow int64, forceDelete bool) (*Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	secret := m.findSecret(secretID)
	if secret == nil {
		return nil, &SecretError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Secrets Manager can't find the specified secret: %s", secretID),
		}
	}

	now := time.Now()

	if forceDelete {
		// Immediately delete.
		delete(m.Secrets, secret.Name)
		delete(m.Policies, secret.Name)
		secret.DeletedDate = &now

		m.saveLocked()

		return secret, nil
	}

	if recoveryWindow == 0 {
		recoveryWindow = defaultRecoveryWindow
	}

	deletionDate := now.AddDate(0, 0, int(recoveryWindow))
	secret.DeletedDate = &now
	secret.ScheduledDeletionDate = &deletionDate
	secret.RecoveryWindowInDays = recoveryWindow

	m.saveLocked()

	return secret, nil
}

// ListSecrets returns all secrets.
func (m *MemoryStorage) ListSecrets(_ context.Context, maxResults int, nextToken string, includePlannedDeletion bool) ([]*Secret, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 100
	}

	// Collect all secrets.
	allSecrets := make([]*Secret, 0, len(m.Secrets))

	for _, secret := range m.Secrets {
		// Skip deleted secrets unless requested.
		if secret.DeletedDate != nil && !includePlannedDeletion {
			continue
		}

		allSecrets = append(allSecrets, secret)
	}

	// Sort by name for consistent ordering.
	sort.Slice(allSecrets, func(i, j int) bool {
		return allSecrets[i].Name < allSecrets[j].Name
	})

	// Handle pagination.
	startIdx := 0

	if nextToken != "" {
		for i, s := range allSecrets {
			if s.Name == nextToken {
				startIdx = i

				break
			}
		}
	}

	endIdx := min(startIdx+maxResults, len(allSecrets))
	result := allSecrets[startIdx:endIdx]

	var newNextToken string
	if endIdx < len(allSecrets) {
		newNextToken = allSecrets[endIdx].Name
	}

	return result, newNextToken, nil
}

// DescribeSecret returns secret metadata.
func (m *MemoryStorage) DescribeSecret(_ context.Context, secretID string) (*Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	secret := m.findSecret(secretID)
	if secret == nil {
		return nil, &SecretError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Secrets Manager can't find the specified secret: %s", secretID),
		}
	}

	return secret, nil
}

// UpdateSecret updates a secret.
func (m *MemoryStorage) UpdateSecret(_ context.Context, req *UpdateSecretRequest) (*Secret, *SecretVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	secret := m.findSecret(req.SecretID)
	if secret == nil {
		return nil, nil, &SecretError{
			Code:    "ResourceNotFoundException",
			Message: fmt.Sprintf("Secrets Manager can't find the specified secret: %s", req.SecretID),
		}
	}

	if secret.DeletedDate != nil {
		return nil, nil, &SecretError{
			Code:    "InvalidRequestException",
			Message: "You can't perform this operation on a secret that's scheduled for deletion.",
		}
	}

	now := time.Now()

	if req.Description != "" {
		secret.Description = req.Description
	}

	if req.KmsKeyID != "" {
		secret.KmsKeyID = req.KmsKeyID
	}

	version := m.createNewVersionIfNeeded(secret, req, now)
	secret.LastChangedDate = now

	m.saveLocked()

	return secret, version, nil
}

// createNewVersionIfNeeded creates a new secret version if value is provided.
func (m *MemoryStorage) createNewVersionIfNeeded(secret *Secret, req *UpdateSecretRequest, now time.Time) *SecretVersion {
	if req.SecretString == "" && len(req.SecretBinary) == 0 {
		return nil
	}

	versionID := req.ClientRequestToken
	if versionID == "" {
		versionID = uuid.New().String()
	}

	m.rotateVersionStages(secret)

	version := &SecretVersion{
		VersionID:     versionID,
		SecretString:  req.SecretString,
		SecretBinary:  req.SecretBinary,
		VersionStages: []string{stageCurrent},
		CreatedDate:   now,
		KmsKeyID:      secret.KmsKeyID,
	}

	secret.VersionIDs[versionID] = version
	secret.VersionID = versionID
	secret.SecretString = req.SecretString
	secret.SecretBinary = req.SecretBinary

	return version
}

// rotateVersionStages moves AWSCURRENT to AWSPREVIOUS for all versions.
func (m *MemoryStorage) rotateVersionStages(secret *Secret) {
	for _, v := range secret.VersionIDs {
		newStages := make([]string, 0)

		for _, stage := range v.VersionStages {
			if stage != stageCurrent {
				newStages = append(newStages, stage)
			} else {
				newStages = append(newStages, stagePrevious)
			}
		}

		v.VersionStages = newStages
	}
}

// findSecret finds a secret by name or ARN.
func (m *MemoryStorage) findSecret(secretID string) *Secret {
	// Try by name first.
	if secret, exists := m.Secrets[secretID]; exists {
		return secret
	}

	// Try by ARN (exact match).
	for _, secret := range m.Secrets {
		if secret.ARN == secretID {
			return secret
		}
	}

	// Try by partial ARN (without random suffix).
	// AWS allows GetSecretValue with "arn:aws:secretsmanager:REGION:ACCOUNT:secret:NAME"
	// even though the full ARN is "arn:aws:secretsmanager:REGION:ACCOUNT:secret:NAME-SUFFIX".
	if strings.HasPrefix(secretID, "arn:aws:secretsmanager:") {
		parts := strings.SplitN(secretID, ":secret:", 2)
		if len(parts) == 2 {
			name := parts[1]
			if secret, exists := m.Secrets[name]; exists {
				return secret
			}
		}
	}

	return nil
}

// GetResourcePolicy returns the resource policy for a secret.
func (m *MemoryStorage) GetResourcePolicy(_ context.Context, secretID string) (*Secret, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	secret := m.findSecret(secretID)
	if secret == nil {
		return nil, "", m.secretNotFoundError(secretID)
	}

	policy := m.Policies[secret.Name]

	return secret, policy, nil
}

// PutResourcePolicy attaches a resource policy to a secret.
func (m *MemoryStorage) PutResourcePolicy(_ context.Context, secretID, policy string) (*Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	secret := m.findSecret(secretID)
	if secret == nil {
		return nil, m.secretNotFoundError(secretID)
	}

	m.Policies[secret.Name] = policy

	m.saveLocked()

	return secret, nil
}

// DeleteResourcePolicy removes the resource policy from a secret.
func (m *MemoryStorage) DeleteResourcePolicy(_ context.Context, secretID string) (*Secret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	secret := m.findSecret(secretID)
	if secret == nil {
		return nil, m.secretNotFoundError(secretID)
	}

	delete(m.Policies, secret.Name)

	m.saveLocked()

	return secret, nil
}

// secretNotFoundError returns a ResourceNotFoundException for the given secret ID.
func (m *MemoryStorage) secretNotFoundError(secretID string) *SecretError {
	return &SecretError{
		Code:    errResourceNotFound,
		Message: fmt.Sprintf("Secrets Manager can't find the specified secret: %s", secretID),
	}
}

// buildARN builds an ARN for a secret.
func (m *MemoryStorage) buildARN(name string) string {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	suffix := uuid.New().String()[:6]

	return fmt.Sprintf("arn:aws:secretsmanager:%s:%s:secret:%s-%s",
		region, defaultAccountID, name, suffix)
}
