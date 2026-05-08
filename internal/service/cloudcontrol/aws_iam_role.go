package cloudcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sivchari/kumo/internal/service/iam"
)

// awsIAMRole adapts AWS::IAM::Role to kumo's IAM storage. The
// CloudFormation surface accepts AssumeRolePolicyDocument as either a
// JSON string or a structured object; AWS clients send the structured
// form, but the IAM storage stores it as a string, so we re-marshal on
// the way in and just echo the stored string on the way out.
type awsIAMRole struct{}

func init() {
	registerDefaultHandler(&awsIAMRole{})
}

// roleProperties is the JSON shape AWS::IAM::Role uses on the wire. The
// AssumeRolePolicyDocument field is RawMessage so we accept both string
// and object forms without losing structure.
type roleProperties struct {
	RoleName                 string          `json:"RoleName,omitempty"`
	Arn                      string          `json:"Arn,omitempty"`
	RoleID                   string          `json:"RoleId,omitempty"`
	Path                     string          `json:"Path,omitempty"`
	Description              string          `json:"Description,omitempty"`
	MaxSessionDuration       int             `json:"MaxSessionDuration,omitempty"`
	AssumeRolePolicyDocument json.RawMessage `json:"AssumeRolePolicyDocument,omitempty"`
}

func (*awsIAMRole) TypeName() string { return "AWS::IAM::Role" }

func (*awsIAMRole) storage() (iam.Storage, error) {
	return lookupStorage[iam.Storage]("iam")
}

func (h *awsIAMRole) Create(ctx context.Context, desired []byte) (string, []byte, error) {
	var props roleProperties
	if err := json.Unmarshal(desired, &props); err != nil {
		return "", nil, fmt.Errorf("invalid AWS::IAM::Role properties: %w", err)
	}

	if props.RoleName == "" {
		return "", nil, errors.New("RoleName is required")
	}

	policyDoc, err := assumeRolePolicyAsString(props.AssumeRolePolicyDocument)
	if err != nil {
		return "", nil, err
	}

	storage, err := h.storage()
	if err != nil {
		return "", nil, err
	}

	role, err := storage.CreateRole(ctx, &iam.CreateRoleRequest{
		RoleName:                 props.RoleName,
		AssumeRolePolicyDocument: policyDoc,
		Path:                     props.Path,
		Description:              props.Description,
		MaxSessionDuration:       props.MaxSessionDuration,
	})
	if err != nil {
		return "", nil, err
	}

	state, err := roleStateJSON(role)
	if err != nil {
		return "", nil, err
	}

	return role.RoleName, state, nil
}

func (h *awsIAMRole) Read(ctx context.Context, identifier string) ([]byte, error) {
	storage, err := h.storage()
	if err != nil {
		return nil, err
	}

	role, err := storage.GetRole(ctx, identifier)
	if err != nil {
		// IAM storage surfaces NoSuchEntity-style errors as Go errors with
		// a name in the message; treat anything containing "not exist" or
		// "no such" as NotFound. The exact wording can be normalised here
		// once the IAM error type is exported.
		if isIAMNotFound(err) {
			return nil, &NotFoundError{Message: "role " + identifier + " does not exist"}
		}

		return nil, err
	}

	return roleStateJSON(role)
}

func (h *awsIAMRole) Update(ctx context.Context, identifier string, _ []byte) ([]byte, error) {
	return h.Read(ctx, identifier)
}

func (h *awsIAMRole) Delete(ctx context.Context, identifier string) error {
	storage, err := h.storage()
	if err != nil {
		return err
	}

	if _, err := storage.GetRole(ctx, identifier); err != nil {
		if isIAMNotFound(err) {
			return &NotFoundError{Message: "role " + identifier + " does not exist"}
		}

		return err
	}

	return storage.DeleteRole(ctx, identifier)
}

func (h *awsIAMRole) List(ctx context.Context) ([]ResourceDescription, error) {
	storage, err := h.storage()
	if err != nil {
		return nil, err
	}

	roles, err := storage.ListRoles(ctx, "", 0)
	if err != nil {
		return nil, err
	}

	out := make([]ResourceDescription, 0, len(roles))

	for i := range roles {
		props, err := roleStateJSON(&roles[i])
		if err != nil {
			return nil, err
		}

		out = append(out, ResourceDescription{Identifier: roles[i].RoleName, Properties: props})
	}

	return out, nil
}

// roleStateJSON serialises a Role for read responses. The full
// CloudFormation schema is emitted (with null / empty defaults for what
// kumo doesn't model — managed policies, permissions boundary, inline
// policies, tags) because terraform-provider-awscc treats every Computed
// property as "must be known after apply".
func roleStateJSON(r *iam.Role) ([]byte, error) {
	var policy any
	if r.AssumeRolePolicyDocument != "" {
		policy = json.RawMessage(r.AssumeRolePolicyDocument)
	}

	state := map[string]any{
		"RoleName":                 r.RoleName,
		"Arn":                      r.Arn,
		"RoleId":                   r.RoleID,
		"Path":                     r.Path,
		"Description":              r.Description,
		"MaxSessionDuration":       r.MaxSessionDuration,
		"AssumeRolePolicyDocument": policy,
		"ManagedPolicyArns":        []any{},
		"PermissionsBoundary":      nil,
		"Policies":                 []any{},
		"Tags":                     []any{},
	}

	return json.Marshal(state)
}

// assumeRolePolicyAsString accepts either a JSON string or a structured
// object and returns the canonical string form the IAM storage stores.
func assumeRolePolicyAsString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("AssumeRolePolicyDocument is required")
	}

	// If it's a JSON string, decode it and use the unwrapped contents.
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}

	// Otherwise it's an object/array — keep the raw bytes.
	return string(raw), nil
}

// isIAMNotFound returns true when err looks like a NoSuchEntity error
// from the IAM storage. Until the IAM package exports a typed error,
// we match on the message — the storage uses consistent wording.
func isIAMNotFound(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "NoSuchEntity") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "not found")
}
