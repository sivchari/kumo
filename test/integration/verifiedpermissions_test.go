//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
	"github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
	"github.com/sivchari/golden"
)

const (
	vpSchema = `{"Issuer":{"entityTypes":{"User":{},"Program":{}},"actions":{}}}`

	vpReadPolicy = `permit (
    principal,
    action == Issuer::Action::"Read",
    resource
)
when {
    resource is Issuer::Program &&
    (context.permission_level == "read-only" || context.permission_level == "read-write")
};`

	vpWritePolicy = `permit (
    principal,
    action == Issuer::Action::"Write",
    resource
)
when {
    resource is Issuer::Program &&
    context.permission_level == "read-write"
};`
)

func newVerifiedPermissionsClient(t *testing.T) *verifiedpermissions.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return verifiedpermissions.NewFromConfig(cfg, func(o *verifiedpermissions.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

// TestVerifiedPermissions_IsAuthorized provisions the IdP's policy store,
// schema, and read/write Cedar policies, then asserts the authorization
// decision for each permission level.
func TestVerifiedPermissions_IsAuthorized(t *testing.T) {
	client := newVerifiedPermissionsClient(t)
	ctx := t.Context()

	store, err := client.CreatePolicyStore(ctx, &verifiedpermissions.CreatePolicyStoreInput{
		ValidationSettings: &types.ValidationSettings{Mode: types.ValidationModeOff},
	})
	if err != nil {
		t.Fatal(err)
	}

	storeID := store.PolicyStoreId

	if _, err := client.PutSchema(ctx, &verifiedpermissions.PutSchemaInput{
		PolicyStoreId: storeID,
		Definition:    &types.SchemaDefinitionMemberCedarJson{Value: vpSchema},
	}); err != nil {
		t.Fatal(err)
	}

	for _, statement := range []string{vpReadPolicy, vpWritePolicy} {
		if _, err := client.CreatePolicy(ctx, &verifiedpermissions.CreatePolicyInput{
			PolicyStoreId: storeID,
			Definition: &types.PolicyDefinitionMemberStatic{Value: types.StaticPolicyDefinition{
				Statement: aws.String(statement),
			}},
		}); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name   string
		action string
		level  string
		want   types.Decision
	}{
		{"read_by_read_only", "Read", "read-only", types.DecisionAllow},
		{"read_by_read_write", "Read", "read-write", types.DecisionAllow},
		{"write_by_read_write", "Write", "read-write", types.DecisionAllow},
		{"write_by_read_only", "Write", "read-only", types.DecisionDeny},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := client.IsAuthorized(ctx, &verifiedpermissions.IsAuthorizedInput{
				PolicyStoreId: storeID,
				Principal: &types.EntityIdentifier{
					EntityType: aws.String("Issuer::User"),
					EntityId:   aws.String("user-1"),
				},
				Action: &types.ActionIdentifier{
					ActionType: aws.String("Issuer::Action"),
					ActionId:   aws.String(tc.action),
				},
				Resource: &types.EntityIdentifier{
					EntityType: aws.String("Issuer::Program"),
					EntityId:   aws.String("program-1"),
				},
				Context: &types.ContextDefinitionMemberContextMap{Value: map[string]types.AttributeValue{
					"permission_level": &types.AttributeValueMemberString{Value: tc.level},
				}},
			})
			if err != nil {
				t.Fatal(err)
			}

			if out.Decision != tc.want {
				t.Fatalf("decision: got %q, want %q", out.Decision, tc.want)
			}

			golden.New(t, golden.WithIgnoreFields("ResultMetadata", "PolicyId")).Assert(t.Name(), out)
		})
	}
}
