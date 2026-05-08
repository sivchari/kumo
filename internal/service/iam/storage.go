package iam

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultPath     = "/"
	defaultMaxItems = 100
	accessKeyActive = "Active"
)

// Error codes.
const (
	errEntityAlreadyExists = "EntityAlreadyExists"
	errNoSuchEntity        = "NoSuchEntity"
	errDeleteConflict      = "DeleteConflict"
	errLimitExceeded       = "LimitExceeded"
)

// Storage defines the IAM storage interface.
type Storage interface {
	CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error)
	DeleteUser(ctx context.Context, userName string) error
	GetUser(ctx context.Context, userName string) (*User, error)
	ListUsers(ctx context.Context, pathPrefix string, maxItems int) ([]User, error)

	CreateRole(ctx context.Context, req *CreateRoleRequest) (*Role, error)
	DeleteRole(ctx context.Context, roleName string) error
	GetRole(ctx context.Context, roleName string) (*Role, error)
	ListRoles(ctx context.Context, pathPrefix string, maxItems int) ([]Role, error)

	CreatePolicy(ctx context.Context, req *CreatePolicyRequest) (*Policy, error)
	DeletePolicy(ctx context.Context, policyArn string) error
	GetPolicy(ctx context.Context, policyArn string) (*Policy, error)
	ListPolicies(ctx context.Context, pathPrefix string, maxItems int, onlyAttached bool) ([]Policy, error)

	AttachUserPolicy(ctx context.Context, userName, policyArn string) error
	DetachUserPolicy(ctx context.Context, userName, policyArn string) error
	AttachRolePolicy(ctx context.Context, roleName, policyArn string) error
	DetachRolePolicy(ctx context.Context, roleName, policyArn string) error
	ListAttachedRolePolicies(ctx context.Context, roleName string) ([]AttachedPolicy, error)

	PutRolePolicy(ctx context.Context, roleName, policyName, policyDocument string) error
	GetRolePolicy(ctx context.Context, roleName, policyName string) (string, error)
	DeleteRolePolicy(ctx context.Context, roleName, policyName string) error
	ListRolePolicies(ctx context.Context, roleName string) ([]string, error)

	CreateOIDCProvider(ctx context.Context, url string, clientIDs, thumbprints []string) (*OIDCProvider, error)
	GetOIDCProvider(ctx context.Context, arn string) (*OIDCProvider, error)
	DeleteOIDCProvider(ctx context.Context, arn string) error
	ListOIDCProviders(ctx context.Context) ([]string, error)
	UpdateOIDCProviderThumbprint(ctx context.Context, arn string, thumbprints []string) error

	CreateAccessKey(ctx context.Context, userName string) (*AccessKey, error)
	DeleteAccessKey(ctx context.Context, userName, accessKeyID string) error
	ListAccessKeys(ctx context.Context, userName string, maxItems int) ([]AccessKeyMetadata, error)

	CreateInstanceProfile(ctx context.Context, name, path string) (*InstanceProfile, error)
	DeleteInstanceProfile(ctx context.Context, name string) error
	GetInstanceProfile(ctx context.Context, name string) (*InstanceProfile, error)
	ListInstanceProfiles(ctx context.Context, pathPrefix string, maxItems int) ([]InstanceProfile, error)
	ListInstanceProfilesForRoleReal(ctx context.Context, roleName string) ([]InstanceProfile, error)
	AddRoleToInstanceProfile(ctx context.Context, profileName, roleName string) error
	RemoveRoleFromInstanceProfile(ctx context.Context, profileName, roleName string) error
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
	mu               sync.RWMutex                     `json:"-"`
	Users            map[string]*User                 `json:"users"`
	Roles            map[string]*Role                 `json:"roles"`
	Policies         map[string]*Policy               `json:"policies"`         // key is ARN
	AccessKeys       map[string]map[string]*AccessKey `json:"accessKeys"`       // userName -> accessKeyID -> AccessKey
	OIDCProviders    map[string]*OIDCProvider         `json:"oidcProviders"`    // key is ARN
	InstanceProfiles map[string]*InstanceProfile      `json:"instanceProfiles"` // key is name
	accountID        string
	dataDir          string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Users:            make(map[string]*User),
		Roles:            make(map[string]*Role),
		Policies:         make(map[string]*Policy),
		AccessKeys:       make(map[string]map[string]*AccessKey),
		OIDCProviders:    make(map[string]*OIDCProvider),
		InstanceProfiles: make(map[string]*InstanceProfile),
		accountID:        "123456789012",
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "iam", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(s)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Users == nil {
		s.Users = make(map[string]*User)
	}

	if s.Roles == nil {
		s.Roles = make(map[string]*Role)
	}

	if s.Policies == nil {
		s.Policies = make(map[string]*Policy)
	}

	if s.AccessKeys == nil {
		s.AccessKeys = make(map[string]map[string]*AccessKey)
	}

	if s.OIDCProviders == nil {
		s.OIDCProviders = make(map[string]*OIDCProvider)
	}

	if s.InstanceProfiles == nil {
		s.InstanceProfiles = make(map[string]*InstanceProfile)
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "iam", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateUser creates a new IAM user.
func (s *MemoryStorage) CreateUser(_ context.Context, req *CreateUserRequest) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Users[req.UserName]; exists {
		return nil, &Error{
			Code:    errEntityAlreadyExists,
			Message: fmt.Sprintf("User with name %s already exists.", req.UserName),
		}
	}

	path := req.Path
	if path == "" {
		path = defaultPath
	}

	user := &User{
		UserName:         req.UserName,
		UserID:           generateID("AIDA"),
		Arn:              fmt.Sprintf("arn:aws:iam::%s:user%s%s", s.accountID, path, req.UserName),
		Path:             path,
		CreateDate:       time.Now().UTC(),
		Tags:             req.Tags,
		AttachedPolicies: []AttachedPolicy{},
	}

	s.Users[req.UserName] = user
	s.AccessKeys[req.UserName] = make(map[string]*AccessKey)

	return user, nil
}

// DeleteUser deletes an IAM user.
func (s *MemoryStorage) DeleteUser(_ context.Context, userName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.Users[userName]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The user with name %s cannot be found.", userName),
		}
	}

	if len(user.AttachedPolicies) > 0 {
		return &Error{
			Code:    errDeleteConflict,
			Message: "Cannot delete entity, must detach all policies first.",
		}
	}

	if keys, ok := s.AccessKeys[userName]; ok && len(keys) > 0 {
		return &Error{
			Code:    errDeleteConflict,
			Message: "Cannot delete entity, must delete access keys first.",
		}
	}

	delete(s.Users, userName)
	delete(s.AccessKeys, userName)

	return nil
}

// GetUser gets an IAM user.
func (s *MemoryStorage) GetUser(_ context.Context, userName string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.Users[userName]
	if !exists {
		return nil, &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The user with name %s cannot be found.", userName),
		}
	}

	return user, nil
}

// ListUsers lists IAM users.
func (s *MemoryStorage) ListUsers(_ context.Context, pathPrefix string, maxItems int) ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxItems <= 0 {
		maxItems = defaultMaxItems
	}

	if pathPrefix == "" {
		pathPrefix = defaultPath
	}

	users := make([]User, 0)

	for _, user := range s.Users {
		if strings.HasPrefix(user.Path, pathPrefix) {
			users = append(users, *user)
			if len(users) >= maxItems {
				break
			}
		}
	}

	return users, nil
}

// CreateRole creates a new IAM role.
func (s *MemoryStorage) CreateRole(_ context.Context, req *CreateRoleRequest) (*Role, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Roles[req.RoleName]; exists {
		return nil, &Error{
			Code:    errEntityAlreadyExists,
			Message: fmt.Sprintf("Role with name %s already exists.", req.RoleName),
		}
	}

	path := req.Path
	if path == "" {
		path = defaultPath
	}

	maxSessionDuration := req.MaxSessionDuration
	if maxSessionDuration == 0 {
		maxSessionDuration = 3600 // 1 hour default
	}

	role := &Role{
		RoleName:                 req.RoleName,
		RoleID:                   generateID("AROA"),
		Arn:                      fmt.Sprintf("arn:aws:iam::%s:role%s%s", s.accountID, path, req.RoleName),
		Path:                     path,
		CreateDate:               time.Now().UTC(),
		AssumeRolePolicyDocument: req.AssumeRolePolicyDocument,
		Description:              req.Description,
		MaxSessionDuration:       maxSessionDuration,
		Tags:                     req.Tags,
		AttachedPolicies:         []AttachedPolicy{},
	}

	s.Roles[req.RoleName] = role

	return role, nil
}

// DeleteRole deletes an IAM role.
func (s *MemoryStorage) DeleteRole(_ context.Context, roleName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, exists := s.Roles[roleName]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role with name %s cannot be found.", roleName),
		}
	}

	if len(role.AttachedPolicies) > 0 {
		return &Error{
			Code:    errDeleteConflict,
			Message: "Cannot delete entity, must detach all policies first.",
		}
	}

	delete(s.Roles, roleName)

	return nil
}

// GetRole gets an IAM role.
func (s *MemoryStorage) GetRole(_ context.Context, roleName string) (*Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	role, exists := s.Roles[roleName]
	if !exists {
		return nil, &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role with name %s cannot be found.", roleName),
		}
	}

	return role, nil
}

// ListRoles lists IAM roles.
func (s *MemoryStorage) ListRoles(_ context.Context, pathPrefix string, maxItems int) ([]Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxItems <= 0 {
		maxItems = defaultMaxItems
	}

	if pathPrefix == "" {
		pathPrefix = defaultPath
	}

	roles := make([]Role, 0)

	for _, role := range s.Roles {
		if strings.HasPrefix(role.Path, pathPrefix) {
			roles = append(roles, *role)
			if len(roles) >= maxItems {
				break
			}
		}
	}

	return roles, nil
}

// CreatePolicy creates a new IAM policy.
func (s *MemoryStorage) CreatePolicy(_ context.Context, req *CreatePolicyRequest) (*Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := req.Path
	if path == "" {
		path = defaultPath
	}

	arn := fmt.Sprintf("arn:aws:iam::%s:policy%s%s", s.accountID, path, req.PolicyName)

	if _, exists := s.Policies[arn]; exists {
		return nil, &Error{
			Code:    errEntityAlreadyExists,
			Message: fmt.Sprintf("A policy called %s already exists. Duplicate names are not allowed.", req.PolicyName),
		}
	}

	now := time.Now().UTC()

	policy := &Policy{
		PolicyName:       req.PolicyName,
		PolicyID:         generateID("ANPA"),
		Arn:              arn,
		Path:             path,
		DefaultVersionID: "v1",
		AttachmentCount:  0,
		IsAttachable:     true,
		CreateDate:       now,
		UpdateDate:       now,
		Description:      req.Description,
		Tags:             req.Tags,
		PolicyDocument:   req.PolicyDocument,
	}

	s.Policies[arn] = policy

	return policy, nil
}

// DeletePolicy deletes an IAM policy.
func (s *MemoryStorage) DeletePolicy(_ context.Context, policyArn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	policy, exists := s.Policies[policyArn]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("Policy %s does not exist.", policyArn),
		}
	}

	if policy.AttachmentCount > 0 {
		return &Error{
			Code:    errDeleteConflict,
			Message: "Cannot delete a policy attached to entities.",
		}
	}

	delete(s.Policies, policyArn)

	return nil
}

// GetPolicy gets an IAM policy.
func (s *MemoryStorage) GetPolicy(_ context.Context, policyArn string) (*Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, exists := s.Policies[policyArn]
	if !exists {
		return nil, &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("Policy %s does not exist.", policyArn),
		}
	}

	return policy, nil
}

// ListPolicies lists IAM policies.
func (s *MemoryStorage) ListPolicies(_ context.Context, pathPrefix string, maxItems int, onlyAttached bool) ([]Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxItems <= 0 {
		maxItems = defaultMaxItems
	}

	if pathPrefix == "" {
		pathPrefix = defaultPath
	}

	policies := make([]Policy, 0)

	for _, policy := range s.Policies {
		if !strings.HasPrefix(policy.Path, pathPrefix) {
			continue
		}

		if onlyAttached && policy.AttachmentCount == 0 {
			continue
		}

		policies = append(policies, *policy)

		if len(policies) >= maxItems {
			break
		}
	}

	return policies, nil
}

// AttachUserPolicy attaches a policy to a user.
func (s *MemoryStorage) AttachUserPolicy(_ context.Context, userName, policyArn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.Users[userName]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The user with name %s cannot be found.", userName),
		}
	}

	policy, exists := s.Policies[policyArn]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("Policy %s does not exist.", policyArn),
		}
	}

	for _, ap := range user.AttachedPolicies {
		if ap.PolicyArn == policyArn {
			return nil // Already attached
		}
	}

	user.AttachedPolicies = append(user.AttachedPolicies, AttachedPolicy{
		PolicyName: policy.PolicyName,
		PolicyArn:  policyArn,
	})
	policy.AttachmentCount++

	return nil
}

// DetachUserPolicy detaches a policy from a user.
func (s *MemoryStorage) DetachUserPolicy(_ context.Context, userName, policyArn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.Users[userName]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The user with name %s cannot be found.", userName),
		}
	}

	policy, exists := s.Policies[policyArn]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("Policy %s does not exist.", policyArn),
		}
	}

	found := false

	for i, ap := range user.AttachedPolicies {
		if ap.PolicyArn == policyArn {
			user.AttachedPolicies = append(user.AttachedPolicies[:i], user.AttachedPolicies[i+1:]...)
			found = true

			break
		}
	}

	if !found {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("Policy %s is not attached to user %s.", policyArn, userName),
		}
	}

	policy.AttachmentCount--

	return nil
}

// AttachRolePolicy attaches a policy to a role.
func (s *MemoryStorage) AttachRolePolicy(_ context.Context, roleName, policyArn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, exists := s.Roles[roleName]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role with name %s cannot be found.", roleName),
		}
	}

	policy, exists := s.Policies[policyArn]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("Policy %s does not exist.", policyArn),
		}
	}

	for _, ap := range role.AttachedPolicies {
		if ap.PolicyArn == policyArn {
			return nil // Already attached
		}
	}

	role.AttachedPolicies = append(role.AttachedPolicies, AttachedPolicy{
		PolicyName: policy.PolicyName,
		PolicyArn:  policyArn,
	})
	policy.AttachmentCount++

	return nil
}

// DetachRolePolicy detaches a policy from a role.
func (s *MemoryStorage) DetachRolePolicy(_ context.Context, roleName, policyArn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, exists := s.Roles[roleName]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role with name %s cannot be found.", roleName),
		}
	}

	policy, exists := s.Policies[policyArn]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("Policy %s does not exist.", policyArn),
		}
	}

	found := false

	for i, ap := range role.AttachedPolicies {
		if ap.PolicyArn == policyArn {
			role.AttachedPolicies = append(role.AttachedPolicies[:i], role.AttachedPolicies[i+1:]...)
			found = true

			break
		}
	}

	if !found {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("Policy %s is not attached to role %s.", policyArn, roleName),
		}
	}

	policy.AttachmentCount--

	return nil
}

// CreateAccessKey creates a new access key for a user.
func (s *MemoryStorage) CreateAccessKey(_ context.Context, userName string) (*AccessKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Users[userName]; !exists {
		return nil, &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The user with name %s cannot be found.", userName),
		}
	}

	keys := s.AccessKeys[userName]
	if len(keys) >= 2 {
		return nil, &Error{
			Code:    errLimitExceeded,
			Message: "Cannot exceed quota for AccessKeysPerUser: 2",
		}
	}

	accessKey := &AccessKey{
		AccessKeyID:     generateAccessKeyID(),
		SecretAccessKey: generateSecretAccessKey(),
		Status:          accessKeyActive,
		UserName:        userName,
		CreateDate:      time.Now().UTC(),
	}

	s.AccessKeys[userName][accessKey.AccessKeyID] = accessKey

	return accessKey, nil
}

// DeleteAccessKey deletes an access key.
func (s *MemoryStorage) DeleteAccessKey(_ context.Context, userName, accessKeyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Users[userName]; !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The user with name %s cannot be found.", userName),
		}
	}

	keys, exists := s.AccessKeys[userName]
	if !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The Access Key with id %s cannot be found.", accessKeyID),
		}
	}

	if _, exists := keys[accessKeyID]; !exists {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The Access Key with id %s cannot be found.", accessKeyID),
		}
	}

	delete(s.AccessKeys[userName], accessKeyID)

	return nil
}

// ListAccessKeys lists access keys for a user.
func (s *MemoryStorage) ListAccessKeys(_ context.Context, userName string, maxItems int) ([]AccessKeyMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.Users[userName]; !exists {
		return nil, &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The user with name %s cannot be found.", userName),
		}
	}

	if maxItems <= 0 {
		maxItems = defaultMaxItems
	}

	keys := make([]AccessKeyMetadata, 0)

	for _, key := range s.AccessKeys[userName] {
		keys = append(keys, AccessKeyMetadata{
			AccessKeyID: key.AccessKeyID,
			Status:      key.Status,
			UserName:    key.UserName,
			CreateDate:  key.CreateDate,
		})

		if len(keys) >= maxItems {
			break
		}
	}

	return keys, nil
}

// generateID generates a unique ID with a prefix.
func generateID(prefix string) string {
	return prefix + strings.ToUpper(uuid.New().String()[:17])
}

// generateAccessKeyID generates an AWS-style access key ID.
func generateAccessKeyID() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)

	return "AKIA" + strings.ToUpper(hex.EncodeToString(b))[:16]
}

// generateSecretAccessKey generates an AWS-style secret access key.
func generateSecretAccessKey() string {
	b := make([]byte, 30)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)[:40]
}

// PutRolePolicy upserts an inline policy on a role.
func (s *MemoryStorage) PutRolePolicy(_ context.Context, roleName, policyName, policyDocument string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, ok := s.Roles[roleName]
	if !ok {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role with name %s cannot be found.", roleName),
		}
	}

	if role.InlinePolicies == nil {
		role.InlinePolicies = make(map[string]string)
	}

	role.InlinePolicies[policyName] = policyDocument

	return nil
}

// GetRolePolicy returns the document for an inline policy on a role.
func (s *MemoryStorage) GetRolePolicy(_ context.Context, roleName, policyName string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	role, ok := s.Roles[roleName]
	if !ok {
		return "", &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role with name %s cannot be found.", roleName),
		}
	}

	doc, ok := role.InlinePolicies[policyName]
	if !ok {
		return "", &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role policy with name %s cannot be found.", policyName),
		}
	}

	return doc, nil
}

// DeleteRolePolicy removes an inline policy from a role.
func (s *MemoryStorage) DeleteRolePolicy(_ context.Context, roleName, policyName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	role, ok := s.Roles[roleName]
	if !ok {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role with name %s cannot be found.", roleName),
		}
	}

	if _, ok := role.InlinePolicies[policyName]; !ok {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role policy with name %s cannot be found.", policyName),
		}
	}

	delete(role.InlinePolicies, policyName)

	return nil
}

// ListRolePolicies returns the names of inline policies attached to a role.
func (s *MemoryStorage) ListRolePolicies(_ context.Context, roleName string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	role, ok := s.Roles[roleName]
	if !ok {
		return nil, &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role with name %s cannot be found.", roleName),
		}
	}

	names := make([]string, 0, len(role.InlinePolicies))
	for name := range role.InlinePolicies {
		names = append(names, name)
	}

	sort.Strings(names)

	return names, nil
}

// ListAttachedRolePolicies returns managed policies attached to a role.
func (s *MemoryStorage) ListAttachedRolePolicies(_ context.Context, roleName string) ([]AttachedPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	role, ok := s.Roles[roleName]
	if !ok {
		return nil, &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("The role with name %s cannot be found.", roleName),
		}
	}

	out := make([]AttachedPolicy, len(role.AttachedPolicies))
	copy(out, role.AttachedPolicies)

	return out, nil
}

// CreateOIDCProvider stores a new OpenID Connect provider.
// The provider ARN is derived from the URL by stripping the scheme, matching
// the AWS IAM convention: arn:aws:iam::<account>:oidc-provider/<host>/<path>.
func (s *MemoryStorage) CreateOIDCProvider(_ context.Context, url string, clientIDs, thumbprints []string) (*OIDCProvider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	host := stripScheme(url)
	arn := fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", s.accountID, host)

	if _, exists := s.OIDCProviders[arn]; exists {
		return nil, &Error{
			Code:    errEntityAlreadyExists,
			Message: fmt.Sprintf("OpenID Connect provider %s already exists.", arn),
		}
	}

	provider := &OIDCProvider{
		Arn:            arn,
		URL:            url,
		ClientIDList:   append([]string(nil), clientIDs...),
		ThumbprintList: append([]string(nil), thumbprints...),
		CreateDate:     time.Now().UTC(),
	}
	s.OIDCProviders[arn] = provider

	return provider, nil
}

// GetOIDCProvider returns the provider with the given ARN.
func (s *MemoryStorage) GetOIDCProvider(_ context.Context, arn string) (*OIDCProvider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provider, ok := s.OIDCProviders[arn]
	if !ok {
		return nil, &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("OpenID Connect provider %s does not exist.", arn),
		}
	}

	return provider, nil
}

// DeleteOIDCProvider removes a provider by ARN.
func (s *MemoryStorage) DeleteOIDCProvider(_ context.Context, arn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.OIDCProviders[arn]; !ok {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("OpenID Connect provider %s does not exist.", arn),
		}
	}

	delete(s.OIDCProviders, arn)

	return nil
}

// ListOIDCProviders returns all provider ARNs.
func (s *MemoryStorage) ListOIDCProviders(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	arns := make([]string, 0, len(s.OIDCProviders))
	for arn := range s.OIDCProviders {
		arns = append(arns, arn)
	}

	sort.Strings(arns)

	return arns, nil
}

// UpdateOIDCProviderThumbprint replaces the thumbprint list of a provider.
func (s *MemoryStorage) UpdateOIDCProviderThumbprint(_ context.Context, arn string, thumbprints []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider, ok := s.OIDCProviders[arn]
	if !ok {
		return &Error{
			Code:    errNoSuchEntity,
			Message: fmt.Sprintf("OpenID Connect provider %s does not exist.", arn),
		}
	}

	provider.ThumbprintList = append([]string(nil), thumbprints...)

	return nil
}

func stripScheme(url string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if rest, ok := strings.CutPrefix(url, prefix); ok {
			return rest
		}
	}

	return url
}

// CreateInstanceProfile creates a new instance profile.
func (s *MemoryStorage) CreateInstanceProfile(_ context.Context, name, path string) (*InstanceProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.InstanceProfiles[name]; exists {
		return nil, &Error{
			Code:    errEntityAlreadyExists,
			Message: fmt.Sprintf("Instance profile with name %s already exists.", name),
		}
	}

	if path == "" {
		path = defaultPath
	}

	profile := &InstanceProfile{
		InstanceProfileName: name,
		InstanceProfileID:   generateID("AIP"),
		Arn:                 fmt.Sprintf("arn:aws:iam::%s:instance-profile%s%s", s.accountID, path, name),
		Path:                path,
		CreateDate:          time.Now().UTC(),
	}
	s.InstanceProfiles[name] = profile

	return profile, nil
}

// DeleteInstanceProfile removes an instance profile by name.
func (s *MemoryStorage) DeleteInstanceProfile(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile, ok := s.InstanceProfiles[name]
	if !ok {
		return &Error{Code: errNoSuchEntity, Message: fmt.Sprintf("Instance profile %s does not exist.", name)}
	}

	if len(profile.Roles) > 0 {
		return &Error{Code: errDeleteConflict, Message: fmt.Sprintf("Cannot delete instance profile %s; remove roles first.", name)}
	}

	delete(s.InstanceProfiles, name)

	return nil
}

// GetInstanceProfile returns the instance profile by name.
func (s *MemoryStorage) GetInstanceProfile(_ context.Context, name string) (*InstanceProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, ok := s.InstanceProfiles[name]
	if !ok {
		return nil, &Error{Code: errNoSuchEntity, Message: fmt.Sprintf("Instance profile %s does not exist.", name)}
	}

	return profile, nil
}

// ListInstanceProfiles returns all instance profiles, optionally filtered by path prefix.
func (s *MemoryStorage) ListInstanceProfiles(_ context.Context, pathPrefix string, _ int) ([]InstanceProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]InstanceProfile, 0, len(s.InstanceProfiles))

	for _, p := range s.InstanceProfiles {
		if pathPrefix != "" && !strings.HasPrefix(p.Path, pathPrefix) {
			continue
		}

		out = append(out, *p)
	}

	return out, nil
}

// ListInstanceProfilesForRoleReal returns all profiles that contain the role.
// (Suffix Real to avoid collision with the existing stub method that returns
// an empty list — the stub is kept for backward-compat with the original PR.)
func (s *MemoryStorage) ListInstanceProfilesForRoleReal(_ context.Context, roleName string) ([]InstanceProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.Roles[roleName]; !ok {
		return nil, &Error{Code: errNoSuchEntity, Message: fmt.Sprintf("The role with name %s cannot be found.", roleName)}
	}

	out := make([]InstanceProfile, 0)

	for _, p := range s.InstanceProfiles {
		for i := range p.Roles {
			if p.Roles[i].RoleName == roleName {
				out = append(out, *p)

				break
			}
		}
	}

	return out, nil
}

// AddRoleToInstanceProfile attaches a role to a profile.
func (s *MemoryStorage) AddRoleToInstanceProfile(_ context.Context, profileName, roleName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile, ok := s.InstanceProfiles[profileName]
	if !ok {
		return &Error{Code: errNoSuchEntity, Message: fmt.Sprintf("Instance profile %s does not exist.", profileName)}
	}

	if len(profile.Roles) > 0 {
		return &Error{Code: errLimitExceeded, Message: fmt.Sprintf("Instance profile %s already has a role attached.", profileName)}
	}

	role, ok := s.Roles[roleName]
	if !ok {
		return &Error{Code: errNoSuchEntity, Message: fmt.Sprintf("The role with name %s cannot be found.", roleName)}
	}

	profile.Roles = append(profile.Roles, *role)

	return nil
}

// RemoveRoleFromInstanceProfile detaches a role from a profile.
func (s *MemoryStorage) RemoveRoleFromInstanceProfile(_ context.Context, profileName, roleName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile, ok := s.InstanceProfiles[profileName]
	if !ok {
		return &Error{Code: errNoSuchEntity, Message: fmt.Sprintf("Instance profile %s does not exist.", profileName)}
	}

	for i := range profile.Roles {
		if profile.Roles[i].RoleName == roleName {
			profile.Roles = append(profile.Roles[:i], profile.Roles[i+1:]...)

			return nil
		}
	}

	return &Error{Code: errNoSuchEntity, Message: fmt.Sprintf("Role %s is not attached to instance profile %s.", roleName, profileName)}
}
