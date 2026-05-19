package cognito

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Error codes.
const (
	errUserPoolNotFound       = "ResourceNotFoundException"
	errUserPoolClientNotFound = "ResourceNotFoundException"
	errUserNotFound           = "UserNotFoundException"
	errUsernameExists         = "UsernameExistsException"
	errNotAuthorized          = "NotAuthorizedException"
	errInvalidParameter       = "InvalidParameterException"
)

// Default values.
const defaultMfaConfiguration = "OFF"

// Storage defines the Cognito storage interface.
type Storage interface {
	// User Pool operations.
	CreateUserPool(ctx context.Context, req *CreateUserPoolRequest) (*UserPool, error)
	GetUserPool(ctx context.Context, userPoolID string) (*UserPool, error)
	ListUserPools(ctx context.Context, maxResults int32, nextToken string) ([]*UserPool, string, error)
	DeleteUserPool(ctx context.Context, userPoolID string) error

	// User Pool Client operations.
	CreateUserPoolClient(ctx context.Context, req *CreateUserPoolClientRequest) (*UserPoolClient, error)
	GetUserPoolClient(ctx context.Context, userPoolID, clientID string) (*UserPoolClient, error)
	ListUserPoolClients(ctx context.Context, userPoolID string, maxResults int32, nextToken string) ([]*UserPoolClient, string, error)
	DeleteUserPoolClient(ctx context.Context, userPoolID, clientID string) error

	// User operations.
	AdminCreateUser(ctx context.Context, req *AdminCreateUserRequest) (*User, error)
	AdminGetUser(ctx context.Context, userPoolID, username string) (*User, error)
	AdminDeleteUser(ctx context.Context, userPoolID, username string) error
	ListUsers(ctx context.Context, userPoolID string, limit int32, paginationToken string) ([]*User, string, error)

	// Authentication operations.
	SignUp(ctx context.Context, req *SignUpRequest) (*User, error)
	ConfirmSignUp(ctx context.Context, clientID, username, code string) error
	InitiateAuth(ctx context.Context, req *InitiateAuthRequest) (*InitiateAuthResponse, error)

	// MFA configuration operations.
	GetUserPoolMfaConfig(ctx context.Context, userPoolID string) (*MfaConfig, error)
	SetUserPoolMfaConfig(ctx context.Context, userPoolID string, config *MfaConfig) error

	// Helper operations.
	GetUserPoolByClientID(ctx context.Context, clientID string) (*UserPool, error)
	GetUserPoolClientByID(ctx context.Context, clientID string) (*UserPoolClient, error)
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
	mu                sync.RWMutex                `json:"-"`
	UserPools         map[string]*UserPool        `json:"userPools"`
	UserPoolClients   map[string]*UserPoolClient  `json:"userPoolClients"`
	Users             map[string]map[string]*User `json:"users"`             // userPoolID -> username -> User
	ConfirmationCodes map[string]string           `json:"confirmationCodes"` // username -> code
	MfaConfigs        map[string]*MfaConfig       `json:"mfaConfigs"`        // userPoolID -> MfaConfig
	dataDir           string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		UserPools:         make(map[string]*UserPool),
		UserPoolClients:   make(map[string]*UserPoolClient),
		Users:             make(map[string]map[string]*User),
		ConfirmationCodes: make(map[string]string),
		MfaConfigs:        make(map[string]*MfaConfig),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "cognito-idp", s)
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

	if s.UserPools == nil {
		s.UserPools = make(map[string]*UserPool)
	}

	if s.UserPoolClients == nil {
		s.UserPoolClients = make(map[string]*UserPoolClient)
	}

	if s.Users == nil {
		s.Users = make(map[string]map[string]*User)
	}

	if s.ConfirmationCodes == nil {
		s.ConfirmationCodes = make(map[string]string)
	}

	if s.MfaConfigs == nil {
		s.MfaConfigs = make(map[string]*MfaConfig)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	type alias MemoryStorage

	data, err := json.Marshal(&struct{ *alias }{alias: (*alias)(s)})
	if err != nil {
		return
	}

	_ = storage.SaveBytes(s.dataDir, "cognito-idp", data)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "cognito-idp", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// CreateUserPool creates a new user pool.
func (s *MemoryStorage) CreateUserPool(_ context.Context, req *CreateUserPoolRequest) (*UserPool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := strings.ReplaceAll(uuid.New().String(), "-", "")[:9]
	poolID := fmt.Sprintf("%s_%s", req.Region, id)
	now := time.Now()

	pool := &UserPool{
		ID:               poolID,
		Name:             req.PoolName,
		Status:           UserPoolStatusEnabled,
		CreationDate:     now,
		LastModifiedDate: now,
		MFAConfiguration: req.MfaConfiguration,
	}

	if req.Policies != nil && req.Policies.PasswordPolicy != nil {
		pool.Policies = &UserPoolPolicies{
			PasswordPolicy: &PasswordPolicy{
				MinimumLength:                 req.Policies.PasswordPolicy.MinimumLength,
				RequireUppercase:              req.Policies.PasswordPolicy.RequireUppercase,
				RequireLowercase:              req.Policies.PasswordPolicy.RequireLowercase,
				RequireNumbers:                req.Policies.PasswordPolicy.RequireNumbers,
				RequireSymbols:                req.Policies.PasswordPolicy.RequireSymbols,
				TemporaryPasswordValidityDays: req.Policies.PasswordPolicy.TemporaryPasswordValidityDays,
			},
		}
	}

	if req.AutoVerifiedAttributes != nil {
		pool.AutoVerifiedAttrs = req.AutoVerifiedAttributes
	}

	if req.UsernameAttributes != nil {
		pool.UsernameAttributes = req.UsernameAttributes
	}

	if req.LambdaConfig != nil {
		pool.LambdaConfig = convertLambdaConfigInputToLambdaConfig(req.LambdaConfig)
	}

	if req.EmailConfiguration != nil {
		pool.EmailConfiguration = &EmailConfiguration{
			SourceArn:           req.EmailConfiguration.SourceArn,
			ReplyToEmailAddress: req.EmailConfiguration.ReplyToEmailAddress,
			EmailSendingAccount: req.EmailConfiguration.EmailSendingAccount,
		}
	}

	s.UserPools[poolID] = pool
	s.Users[poolID] = make(map[string]*User)

	s.saveLocked()

	return pool, nil
}

// GetUserPool retrieves a user pool by ID.
func (s *MemoryStorage) GetUserPool(_ context.Context, userPoolID string) (*UserPool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pool, ok := s.UserPools[userPoolID]
	if !ok {
		return nil, &ServiceError{Code: errUserPoolNotFound, Message: "User pool not found"}
	}

	return pool, nil
}

// ListUserPools lists all user pools.
func (s *MemoryStorage) ListUserPools(_ context.Context, maxResults int32, _ string) ([]*UserPool, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 60
	}

	pools := make([]*UserPool, 0, len(s.UserPools))

	for _, pool := range s.UserPools {
		pools = append(pools, pool)

		if len(pools) >= int(maxResults) {
			break
		}
	}

	return pools, "", nil
}

// DeleteUserPool deletes a user pool.
func (s *MemoryStorage) DeleteUserPool(_ context.Context, userPoolID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.UserPools[userPoolID]; !ok {
		return &ServiceError{Code: errUserPoolNotFound, Message: "User pool not found"}
	}

	// Delete associated clients.
	for clientID, client := range s.UserPoolClients {
		if client.UserPoolID == userPoolID {
			delete(s.UserPoolClients, clientID)
		}
	}

	// Delete associated users.
	delete(s.Users, userPoolID)
	delete(s.UserPools, userPoolID)

	s.saveLocked()

	return nil
}

// CreateUserPoolClient creates a new user pool client.
func (s *MemoryStorage) CreateUserPoolClient(_ context.Context, req *CreateUserPoolClientRequest) (*UserPoolClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.UserPools[req.UserPoolID]; !ok {
		return nil, &ServiceError{Code: errUserPoolNotFound, Message: "User pool not found"}
	}

	clientID := uuid.New().String()[:26]
	now := time.Now()

	client := &UserPoolClient{
		ClientID:                        clientID,
		ClientName:                      req.ClientName,
		UserPoolID:                      req.UserPoolID,
		CreationDate:                    now,
		LastModifiedDate:                now,
		RefreshTokenValidity:            req.RefreshTokenValidity,
		AccessTokenValidity:             req.AccessTokenValidity,
		IDTokenValidity:                 req.IDTokenValidity,
		ExplicitAuthFlows:               req.ExplicitAuthFlows,
		SupportedIdentityProviders:      req.SupportedIdentityProviders,
		CallbackURLs:                    req.CallbackURLs,
		LogoutURLs:                      req.LogoutURLs,
		AllowedOAuthFlows:               req.AllowedOAuthFlows,
		AllowedOAuthScopes:              req.AllowedOAuthScopes,
		AllowedOAuthFlowsUserPoolClient: req.AllowedOAuthFlowsUserPoolClient,
	}

	if req.GenerateSecret {
		client.ClientSecret = generateSecret()
	}

	// Set defaults.
	if client.RefreshTokenValidity == 0 {
		client.RefreshTokenValidity = 30
	}

	if client.AccessTokenValidity == 0 {
		client.AccessTokenValidity = 60
	}

	if client.IDTokenValidity == 0 {
		client.IDTokenValidity = 60
	}

	s.UserPoolClients[clientID] = client

	s.saveLocked()

	return client, nil
}

// GetUserPoolClient retrieves a user pool client.
func (s *MemoryStorage) GetUserPoolClient(_ context.Context, userPoolID, clientID string) (*UserPoolClient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.UserPoolClients[clientID]
	if !ok || client.UserPoolID != userPoolID {
		return nil, &ServiceError{Code: errUserPoolClientNotFound, Message: "User pool client not found"}
	}

	return client, nil
}

// ListUserPoolClients lists user pool clients.
func (s *MemoryStorage) ListUserPoolClients(_ context.Context, userPoolID string, maxResults int32, _ string) ([]*UserPoolClient, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxResults <= 0 {
		maxResults = 60
	}

	clients := make([]*UserPoolClient, 0)

	for _, client := range s.UserPoolClients {
		if client.UserPoolID == userPoolID {
			clients = append(clients, client)

			if len(clients) >= int(maxResults) {
				break
			}
		}
	}

	return clients, "", nil
}

// DeleteUserPoolClient deletes a user pool client.
func (s *MemoryStorage) DeleteUserPoolClient(_ context.Context, userPoolID, clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, ok := s.UserPoolClients[clientID]
	if !ok || client.UserPoolID != userPoolID {
		return &ServiceError{Code: errUserPoolClientNotFound, Message: "User pool client not found"}
	}

	delete(s.UserPoolClients, clientID)

	s.saveLocked()

	return nil
}

// AdminCreateUser creates a new user.
func (s *MemoryStorage) AdminCreateUser(_ context.Context, req *AdminCreateUserRequest) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.UserPools[req.UserPoolID]; !ok {
		return nil, &ServiceError{Code: errUserPoolNotFound, Message: "User pool not found"}
	}

	if _, ok := s.Users[req.UserPoolID][req.Username]; ok {
		return nil, &ServiceError{Code: errUsernameExists, Message: "User already exists"}
	}

	now := time.Now()
	user := &User{
		Username:         req.Username,
		UserPoolID:       req.UserPoolID,
		UserCreateDate:   now,
		UserLastModified: now,
		Enabled:          true,
		UserStatus:       UserStatusForceChangePassword,
		Password:         req.TemporaryPassword,
	}

	if req.UserAttributes != nil {
		user.Attributes = make([]UserAttribute, len(req.UserAttributes))

		for i, attr := range req.UserAttributes {
			user.Attributes[i] = UserAttribute(attr)
		}
	}

	s.Users[req.UserPoolID][req.Username] = user

	s.saveLocked()

	return user, nil
}

// AdminGetUser retrieves a user.
func (s *MemoryStorage) AdminGetUser(_ context.Context, userPoolID, username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users, ok := s.Users[userPoolID]
	if !ok {
		return nil, &ServiceError{Code: errUserPoolNotFound, Message: "User pool not found"}
	}

	user, ok := users[username]
	if !ok {
		return nil, &ServiceError{Code: errUserNotFound, Message: "User not found"}
	}

	return user, nil
}

// AdminDeleteUser deletes a user.
func (s *MemoryStorage) AdminDeleteUser(_ context.Context, userPoolID, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	users, ok := s.Users[userPoolID]
	if !ok {
		return &ServiceError{Code: errUserPoolNotFound, Message: "User pool not found"}
	}

	if _, ok := users[username]; !ok {
		return &ServiceError{Code: errUserNotFound, Message: "User not found"}
	}

	delete(users, username)

	s.saveLocked()

	return nil
}

// ListUsers lists users in a user pool.
func (s *MemoryStorage) ListUsers(_ context.Context, userPoolID string, limit int32, _ string) ([]*User, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 60
	}

	users, ok := s.Users[userPoolID]
	if !ok {
		return nil, "", &ServiceError{Code: errUserPoolNotFound, Message: "User pool not found"}
	}

	result := make([]*User, 0, len(users))

	for _, user := range users {
		result = append(result, user)

		if len(result) >= int(limit) {
			break
		}
	}

	return result, "", nil
}

// SignUp registers a new user.
func (s *MemoryStorage) SignUp(_ context.Context, req *SignUpRequest) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find user pool by client ID.
	var userPoolID string

	for _, client := range s.UserPoolClients {
		if client.ClientID == req.ClientID {
			userPoolID = client.UserPoolID

			break
		}
	}

	if userPoolID == "" {
		return nil, &ServiceError{Code: errInvalidParameter, Message: "Invalid client ID"}
	}

	if _, ok := s.Users[userPoolID][req.Username]; ok {
		return nil, &ServiceError{Code: errUsernameExists, Message: "User already exists"}
	}

	now := time.Now()
	user := &User{
		Username:         req.Username,
		UserPoolID:       userPoolID,
		UserCreateDate:   now,
		UserLastModified: now,
		Enabled:          true,
		UserStatus:       UserStatusUnconfirmed,
		Password:         req.Password,
	}

	if req.UserAttributes != nil {
		user.Attributes = make([]UserAttribute, len(req.UserAttributes))

		for i, attr := range req.UserAttributes {
			user.Attributes[i] = UserAttribute(attr)
		}
	}

	s.Users[userPoolID][req.Username] = user

	// Generate confirmation code (simulated).
	s.ConfirmationCodes[req.Username] = "123456"

	s.saveLocked()

	return user, nil
}

// ConfirmSignUp confirms a user registration.
func (s *MemoryStorage) ConfirmSignUp(_ context.Context, clientID, username, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find user pool by client ID.
	var userPoolID string

	for _, client := range s.UserPoolClients {
		if client.ClientID == clientID {
			userPoolID = client.UserPoolID

			break
		}
	}

	if userPoolID == "" {
		return &ServiceError{Code: errInvalidParameter, Message: "Invalid client ID"}
	}

	user, ok := s.Users[userPoolID][username]
	if !ok {
		return &ServiceError{Code: errUserNotFound, Message: "User not found"}
	}

	// In a real implementation, we would verify the code.
	// For testing, we accept any code or the default "123456".
	expectedCode := s.ConfirmationCodes[username]
	if expectedCode != "" && code != expectedCode && code != "123456" {
		return &ServiceError{Code: errInvalidParameter, Message: "Invalid confirmation code"}
	}

	user.UserStatus = UserStatusConfirmed
	user.UserLastModified = time.Now()

	delete(s.ConfirmationCodes, username)

	s.saveLocked()

	return nil
}

// InitiateAuth initiates authentication.
func (s *MemoryStorage) InitiateAuth(_ context.Context, req *InitiateAuthRequest) (*InitiateAuthResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find user pool by client ID.
	var userPoolID string

	for _, client := range s.UserPoolClients {
		if client.ClientID == req.ClientID {
			userPoolID = client.UserPoolID

			break
		}
	}

	if userPoolID == "" {
		return nil, &ServiceError{Code: errInvalidParameter, Message: "Invalid client ID"}
	}

	username := req.AuthParameters["USERNAME"]
	password := req.AuthParameters["PASSWORD"]

	user, ok := s.Users[userPoolID][username]
	if !ok {
		return nil, &ServiceError{Code: errUserNotFound, Message: "User not found"}
	}

	if user.Password != password {
		return nil, &ServiceError{Code: errNotAuthorized, Message: "Incorrect username or password"}
	}

	if user.UserStatus == UserStatusUnconfirmed {
		return nil, &ServiceError{Code: errNotAuthorized, Message: "User is not confirmed"}
	}

	// Generate tokens.
	accessToken := generateToken()
	idToken := generateToken()
	refreshToken := generateToken()

	return &InitiateAuthResponse{
		AuthenticationResult: &AuthenticationResult{
			AccessToken:  accessToken,
			ExpiresIn:    3600,
			TokenType:    "Bearer",
			RefreshToken: refreshToken,
			IDToken:      idToken,
		},
	}, nil
}

// GetUserPoolByClientID retrieves a user pool by client ID.
func (s *MemoryStorage) GetUserPoolByClientID(_ context.Context, clientID string) (*UserPool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.UserPoolClients[clientID]
	if !ok {
		return nil, &ServiceError{Code: errUserPoolClientNotFound, Message: "User pool client not found"}
	}

	pool, ok := s.UserPools[client.UserPoolID]
	if !ok {
		return nil, &ServiceError{Code: errUserPoolNotFound, Message: "User pool not found"}
	}

	return pool, nil
}

// GetUserPoolClientByID retrieves a user pool client by ID.
func (s *MemoryStorage) GetUserPoolClientByID(_ context.Context, clientID string) (*UserPoolClient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.UserPoolClients[clientID]
	if !ok {
		return nil, &ServiceError{Code: errUserPoolClientNotFound, Message: "User pool client not found"}
	}

	return client, nil
}

// generateSecret generates a random client secret.
func generateSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)

	return base64.StdEncoding.EncodeToString(b)
}

// generateToken generates a random token.
func generateToken() string {
	b := make([]byte, 64)
	_, _ = rand.Read(b)

	return base64.RawURLEncoding.EncodeToString(b)
}

// GetUserPoolMfaConfig retrieves the MFA configuration for a user pool.
func (s *MemoryStorage) GetUserPoolMfaConfig(_ context.Context, userPoolID string) (*MfaConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.UserPools[userPoolID]; !ok {
		return nil, &ServiceError{Code: errUserPoolNotFound, Message: "User pool not found"}
	}

	cfg, ok := s.MfaConfigs[userPoolID]
	if !ok {
		return &MfaConfig{MfaConfiguration: defaultMfaConfiguration}, nil
	}

	return cfg, nil
}

// SetUserPoolMfaConfig sets the MFA configuration for a user pool.
func (s *MemoryStorage) SetUserPoolMfaConfig(_ context.Context, userPoolID string, config *MfaConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.UserPools[userPoolID]; !ok {
		return &ServiceError{Code: errUserPoolNotFound, Message: "User pool not found"}
	}

	s.MfaConfigs[userPoolID] = config

	s.saveLocked()

	return nil
}

// convertLambdaConfigInputToLambdaConfig converts LambdaConfigInput to LambdaConfig.
func convertLambdaConfigInputToLambdaConfig(input *LambdaConfigInput) *LambdaConfig {
	if input == nil {
		return nil
	}

	return &LambdaConfig{
		PreSignUp:               input.PreSignUp,
		CustomMessage:           input.CustomMessage,
		PostConfirmation:        input.PostConfirmation,
		PreAuthentication:       input.PreAuthentication,
		PostAuthentication:      input.PostAuthentication,
		DefineAuthChallenge:     input.DefineAuthChallenge,
		CreateAuthChallenge:     input.CreateAuthChallenge,
		VerifyAuthChallengeResp: input.VerifyAuthChallengeResp,
		PreTokenGeneration:      input.PreTokenGeneration,
		UserMigration:           input.UserMigration,
	}
}
