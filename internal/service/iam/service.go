// Package iam provides IAM service emulation for kumo.
package iam

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	service.Register(New(NewMemoryStorage(opts...)))
}

// Service implements the IAM service.
type Service struct {
	storage        Storage
	actionHandlers map[string]http.HandlerFunc
}

// New creates a new IAM service.
func New(storage Storage) *Service {
	s := &Service{
		storage: storage,
	}
	s.initActionHandlers()

	return s
}

// initActionHandlers initializes the action handlers map.
func (s *Service) initActionHandlers() {
	s.actionHandlers = map[string]http.HandlerFunc{
		// User management
		"CreateUser": s.CreateUser,
		"DeleteUser": s.DeleteUser,
		"GetUser":    s.GetUser,
		"ListUsers":  s.ListUsers,
		// Role management
		"CreateRole": s.CreateRole,
		"DeleteRole": s.DeleteRole,
		"GetRole":    s.GetRole,
		"ListRoles":  s.ListRoles,
		// Policy management
		"CreatePolicy": s.CreatePolicy,
		"DeletePolicy": s.DeletePolicy,
		"GetPolicy":    s.GetPolicy,
		"ListPolicies": s.ListPolicies,
		// Policy attachments
		"AttachUserPolicy": s.AttachUserPolicy,
		"DetachUserPolicy": s.DetachUserPolicy,
		"AttachRolePolicy": s.AttachRolePolicy,
		"DetachRolePolicy": s.DetachRolePolicy,
		// Access keys
		"CreateAccessKey": s.CreateAccessKey,
		"DeleteAccessKey": s.DeleteAccessKey,
		"ListAccessKeys":  s.ListAccessKeys,
		// Inline role policies
		"PutRolePolicy":               s.PutRolePolicy,
		"GetRolePolicy":               s.GetRolePolicy,
		"DeleteRolePolicy":            s.DeleteRolePolicy,
		"ListRolePolicies":            s.ListRolePolicies,
		"ListAttachedRolePolicies":    s.ListAttachedRolePolicies,
		"ListInstanceProfilesForRole": s.ListInstanceProfilesForRole,
		// OpenID Connect provider
		"CreateOpenIDConnectProvider":           s.CreateOpenIDConnectProvider,
		"GetOpenIDConnectProvider":              s.GetOpenIDConnectProvider,
		"DeleteOpenIDConnectProvider":           s.DeleteOpenIDConnectProvider,
		"ListOpenIDConnectProviders":            s.ListOpenIDConnectProviders,
		"UpdateOpenIDConnectProviderThumbprint": s.UpdateOpenIDConnectProviderThumbprint,
		// Instance profiles
		"CreateInstanceProfile":         s.CreateInstanceProfile,
		"DeleteInstanceProfile":         s.DeleteInstanceProfile,
		"GetInstanceProfile":            s.GetInstanceProfile,
		"ListInstanceProfiles":          s.ListInstanceProfiles,
		"AddRoleToInstanceProfile":      s.AddRoleToInstanceProfile,
		"RemoveRoleFromInstanceProfile": s.RemoveRoleFromInstanceProfile,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "iam"
}

// RegisterRoutes registers the IAM routes.
func (s *Service) RegisterRoutes(r service.Router) {
	// IAM uses a single endpoint with Action parameter.
	// Register both with and without trailing slash for SDK compatibility.
	r.HandleFunc("POST", "/iam/", s.DispatchAction)
	r.HandleFunc("GET", "/iam/", s.DispatchAction)
	r.HandleFunc("POST", "/iam", s.DispatchAction)
	r.HandleFunc("GET", "/iam", s.DispatchAction)
}

// Close saves the storage state if persistence is enabled.
func (s *Service) Close() error {
	if c, ok := s.storage.(io.Closer); ok {
		if err := c.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
	}

	return nil
}
