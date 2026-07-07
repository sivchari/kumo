package sts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultAccountID = "000000000000"

	// Default credential expiration duration (1 hour).
	defaultDurationSeconds = 3600
)

// Storage defines the STS storage interface.
type Storage interface {
	AssumeRole(ctx context.Context, input *AssumeRoleInput) (*AssumeRoleResult, error)
	AssumeRoleWithSAML(ctx context.Context, input *AssumeRoleWithSAMLInput) (*AssumeRoleResult, error)
	AssumeRoleWithWebIdentity(ctx context.Context, input *AssumeRoleWithWebIdentityInput) (*AssumeRoleResult, error)
	GetCallerIdentity(ctx context.Context) (*CallerIdentity, error)
	GetSessionToken(ctx context.Context, input *GetSessionTokenInput) (*Credentials, error)
	GetFederationToken(ctx context.Context, input *GetFederationTokenInput) (*FederationTokenResult, error)
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

// AssumeRoleResult represents the result of an AssumeRole operation.
type AssumeRoleResult struct {
	AssumedRoleUser  *AssumedRoleUser
	Credentials      *Credentials
	PackedPolicySize int32
}

// CallerIdentity represents the caller identity.
type CallerIdentity struct {
	Account string
	Arn     string
	UserID  string
}

// FederationTokenResult represents the result of a GetFederationToken operation.
type FederationTokenResult struct {
	Credentials      *Credentials
	FederatedUser    *FederatedUser
	PackedPolicySize int32
}

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	dataDir string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "sts", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
// STS is stateless, so we return an empty JSON object.
func (m *MemoryStorage) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}

// UnmarshalJSON restores the storage state from JSON.
// STS is stateless, so there is nothing to restore.
func (m *MemoryStorage) UnmarshalJSON(_ []byte) error {
	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "sts", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// AssumeRole generates temporary credentials for an assumed role.
func (m *MemoryStorage) AssumeRole(_ context.Context, input *AssumeRoleInput) (*AssumeRoleResult, error) {
	duration := resolveDuration(input.DurationSeconds)
	creds := generateCredentials(duration)
	roleSessionName := input.RoleSessionName
	assumedRoleID := generateAssumedRoleID(roleSessionName)
	partition, accountID, roleName := roleArnParts(input.RoleArn)

	return &AssumeRoleResult{
		AssumedRoleUser: &AssumedRoleUser{
			Arn:           fmt.Sprintf("arn:%s:sts::%s:assumed-role/%s/%s", partition, accountID, roleName, roleSessionName),
			AssumedRoleID: assumedRoleID,
		},
		Credentials:      creds,
		PackedPolicySize: 0,
	}, nil
}

// AssumeRoleWithSAML generates temporary credentials for a SAML-authenticated role assumption.
func (m *MemoryStorage) AssumeRoleWithSAML(_ context.Context, input *AssumeRoleWithSAMLInput) (*AssumeRoleResult, error) {
	duration := resolveDuration(input.DurationSeconds)
	creds := generateCredentials(duration)
	assumedRoleID := generateAssumedRoleID("SAMLSession")
	partition, accountID, roleName := roleArnParts(input.RoleArn)

	return &AssumeRoleResult{
		AssumedRoleUser: &AssumedRoleUser{
			Arn:           fmt.Sprintf("arn:%s:sts::%s:assumed-role/%s/SAMLSession", partition, accountID, roleName),
			AssumedRoleID: assumedRoleID,
		},
		Credentials:      creds,
		PackedPolicySize: 0,
	}, nil
}

// AssumeRoleWithWebIdentity generates temporary credentials for a web identity role assumption.
func (m *MemoryStorage) AssumeRoleWithWebIdentity(_ context.Context, input *AssumeRoleWithWebIdentityInput) (*AssumeRoleResult, error) {
	duration := resolveDuration(input.DurationSeconds)
	creds := generateCredentials(duration)
	roleSessionName := input.RoleSessionName
	assumedRoleID := generateAssumedRoleID(roleSessionName)
	partition, accountID, roleName := roleArnParts(input.RoleArn)

	return &AssumeRoleResult{
		AssumedRoleUser: &AssumedRoleUser{
			Arn:           fmt.Sprintf("arn:%s:sts::%s:assumed-role/%s/%s", partition, accountID, roleName, roleSessionName),
			AssumedRoleID: assumedRoleID,
		},
		Credentials:      creds,
		PackedPolicySize: 0,
	}, nil
}

// GetCallerIdentity returns the identity of the caller.
func (m *MemoryStorage) GetCallerIdentity(_ context.Context) (*CallerIdentity, error) {
	return &CallerIdentity{
		Account: defaultAccountID,
		Arn:     fmt.Sprintf("arn:aws:iam::%s:root", defaultAccountID),
		UserID:  defaultAccountID,
	}, nil
}

// GetSessionToken generates temporary session credentials.
func (m *MemoryStorage) GetSessionToken(_ context.Context, input *GetSessionTokenInput) (*Credentials, error) {
	duration := resolveDuration(input.DurationSeconds)

	return generateCredentials(duration), nil
}

// GetFederationToken generates temporary credentials for a federated user.
func (m *MemoryStorage) GetFederationToken(_ context.Context, input *GetFederationTokenInput) (*FederationTokenResult, error) {
	duration := resolveDuration(input.DurationSeconds)
	creds := generateCredentials(duration)

	return &FederationTokenResult{
		Credentials: creds,
		FederatedUser: &FederatedUser{
			Arn:             fmt.Sprintf("arn:aws:sts::%s:federated-user/%s", defaultAccountID, input.Name),
			FederatedUserID: fmt.Sprintf("%s:%s", defaultAccountID, input.Name),
		},
		PackedPolicySize: 0,
	}, nil
}

// Helper functions.

// roleArnParts extracts the partition, account ID, and role name from an IAM
// role ARN (arn:<partition>:iam::<account>:role/<path...>/<name>). Fallbacks
// keep AssumeRole permissive: unparseable input yields "aws" / defaultAccountID
// / "emulated-role", and an empty partition segment also defaults to "aws".
func roleArnParts(roleArn string) (string, string, string) {
	partition, accountID, roleName := "aws", defaultAccountID, "emulated-role"

	parts := strings.SplitN(roleArn, ":", 6)
	if len(parts) != 6 || parts[2] != "iam" {
		return partition, accountID, roleName
	}

	if parts[1] != "" {
		partition = parts[1]
	}

	if parts[4] != "" {
		accountID = parts[4]
	}

	res, ok := strings.CutPrefix(parts[5], "role/")
	if !ok || res == "" {
		return partition, accountID, roleName
	}

	if name := path.Base(res); name != "." && name != "/" {
		roleName = name
	}

	return partition, accountID, roleName
}

func resolveDuration(durationSeconds int32) int32 {
	if durationSeconds <= 0 {
		return defaultDurationSeconds
	}

	return durationSeconds
}

func generateCredentials(durationSeconds int32) *Credentials {
	expiration := time.Now().Add(time.Duration(durationSeconds) * time.Second)

	return &Credentials{
		AccessKeyID:     "ASIA" + randomHex(16),
		SecretAccessKey: randomHex(40),
		SessionToken:    randomHex(64),
		Expiration:      expiration.UTC().Format(time.RFC3339),
	}
}

func generateAssumedRoleID(sessionName string) string {
	return fmt.Sprintf("AROA%s:%s", uuid.New().String()[:12], sessionName)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)[:n]
}
