package verifiedpermissions

import (
	"testing"

	cedartypes "github.com/cedar-policy/cedar-go/types"
)

// The IdP's real Cedar policy assets. The resource-type check lives in the
// `when` clause (not the policy scope) to stay portable across engines.
const (
	readPolicyStatement = `permit (
    principal,
    action == Issuer::Action::"Read",
    resource
)
when {
    resource is Issuer::Program &&
    (context.permission_level == "read-only" || context.permission_level == "read-write")
};`

	writePolicyStatement = `permit (
    principal,
    action == Issuer::Action::"Write",
    resource
)
when {
    resource is Issuer::Program &&
    context.permission_level == "read-write"
};`
)

func ptr[T any](v T) *T {
	return &v
}

// idpPolicies returns the read and write policies keyed by id.
func idpPolicies() map[string]*Policy {
	return map[string]*Policy{
		"read":  {ID: "read", Statement: readPolicyStatement},
		"write": {ID: "write", Statement: writePolicyStatement},
	}
}

// evalIDP runs the full IsAuthorized path (buildPolicySet -> buildRequest ->
// decide) for the given action and permission level against the IdP policies.
func evalIDP(t *testing.T, policies map[string]*Policy, action, level string) (string, []DeterminingPolicyItem) {
	t.Helper()

	ps, err := buildPolicySet(policies)
	if err != nil {
		t.Fatalf("buildPolicySet: %v", err)
	}

	req := &IsAuthorizedRequest{
		Principal: &EntityIdentifier{EntityType: "Issuer::User", EntityID: "user-1"},
		Action:    &ActionIdentifier{ActionType: "Issuer::Action", ActionID: action},
		Resource:  &EntityIdentifier{EntityType: "Issuer::Program", EntityID: "prog-1"},
		Context: &ContextDefinition{ContextMap: map[string]AttributeValue{
			"permission_level": {String: ptr(level)},
		}},
	}

	cReq, err := buildRequest(req)
	if err != nil {
		t.Fatalf("buildRequest: %v", err)
	}

	decision, determining, _ := decide(ps, &cReq)

	return decision, determining
}

func TestDecide_IDPPolicies(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		action string
		level  string
		want   string
	}{
		{"read by read-only is allowed", "Read", "read-only", "ALLOW"},
		{"read by read-write is allowed", "Read", "read-write", "ALLOW"},
		{"write by read-write is allowed", "Write", "read-write", "ALLOW"},
		{"write by read-only is denied", "Write", "read-only", "DENY"},
		{"unknown action is denied", "Delete", "read-write", "DENY"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, _ := evalIDP(t, idpPolicies(), tc.action, tc.level)
			if got != tc.want {
				t.Fatalf("decision: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDecide_NoPoliciesDenies(t *testing.T) {
	t.Parallel()

	got, determining := evalIDP(t, map[string]*Policy{}, "Read", "read-write")
	if got != "DENY" {
		t.Fatalf("decision: got %q, want DENY", got)
	}

	if len(determining) != 0 {
		t.Fatalf("determining: got %v, want empty", determining)
	}
}

func TestDecide_ReportsDeterminingPolicy(t *testing.T) {
	t.Parallel()

	got, determining := evalIDP(t, idpPolicies(), "Read", "read-write")
	if got != "ALLOW" {
		t.Fatalf("decision: got %q, want ALLOW", got)
	}

	if len(determining) != 1 || determining[0].PolicyID != "read" {
		t.Fatalf("determining: got %v, want [{read}]", determining)
	}
}

func TestBuildPolicySet_InvalidStatement(t *testing.T) {
	t.Parallel()

	_, err := buildPolicySet(map[string]*Policy{
		"bad": {ID: "bad", Statement: "this is not cedar"},
	})
	if err == nil {
		t.Fatal("expected error for invalid cedar statement, got nil")
	}
}

func TestValidateStatement(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		statement string
		wantErr   bool
	}{
		{"valid permit", readPolicyStatement, false},
		{"garbage", "not a policy", true},
		{"empty", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateStatement(tc.statement)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateStatement(%q): err=%v, wantErr=%v", tc.statement, err, tc.wantErr)
			}
		})
	}
}

func TestAttrToValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		attr AttributeValue
		want cedartypes.Value
	}{
		{"string", AttributeValue{String: ptr("read-write")}, cedartypes.String("read-write")},
		{"long", AttributeValue{Long: ptr(int64(42))}, cedartypes.Long(42)},
		{"boolean", AttributeValue{Boolean: ptr(true)}, cedartypes.Boolean(true)},
		{
			"entityIdentifier",
			AttributeValue{EntityIdentifier: &EntityIdentifier{EntityType: "Issuer::User", EntityID: "u1"}},
			cedartypes.NewEntityUID("Issuer::User", "u1"),
		},
		{
			"set of strings",
			AttributeValue{Set: []AttributeValue{{String: ptr("a")}, {String: ptr("b")}}},
			cedartypes.NewSet(cedartypes.String("a"), cedartypes.String("b")),
		},
		{
			"record",
			AttributeValue{Record: map[string]AttributeValue{"k": {Long: ptr(int64(1))}}},
			cedartypes.NewRecord(cedartypes.RecordMap{"k": cedartypes.Long(1)}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := attrToValue(tc.attr)
			if err != nil {
				t.Fatalf("attrToValue: %v", err)
			}

			if !got.Equal(tc.want) {
				t.Fatalf("value: got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestAttrToValue_EmptyIsError(t *testing.T) {
	t.Parallel()

	if _, err := attrToValue(AttributeValue{}); err == nil {
		t.Fatal("expected error for empty attribute value, got nil")
	}
}
