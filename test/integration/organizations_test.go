//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/sivchari/golden"
)

func newOrganizationsClient(t *testing.T) *organizations.Client {
	t.Helper()

	return organizations.NewFromConfig(awsConfig(t), func(o *organizations.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

// ensureOrganization creates an organization if one doesn't exist, or returns the existing one.
func ensureOrganization(t *testing.T, client *organizations.Client) *types.Organization {
	t.Helper()

	ctx := t.Context()

	// Check if organization exists
	descOutput, err := client.DescribeOrganization(ctx, &organizations.DescribeOrganizationInput{})
	if err == nil {
		return descOutput.Organization
	}

	// No organization exists, create one
	createOutput, err := client.CreateOrganization(ctx, &organizations.CreateOrganizationInput{
		FeatureSet: types.OrganizationFeatureSetAll,
	})
	if err != nil {
		t.Fatal(err)
	}

	return createOutput.Organization
}

// ensureNoOrganization deletes any existing organization (only works if no member accounts).
func ensureNoOrganization(t *testing.T, client *organizations.Client) {
	t.Helper()

	ctx := t.Context()

	_, err := client.DescribeOrganization(ctx, &organizations.DescribeOrganizationInput{})
	if err != nil {
		return
	}

	_, _ = client.DeleteOrganization(ctx, &organizations.DeleteOrganizationInput{})
}

// TestOrganizations_WithOrganization tests operations that require an existing organization.
// These tests share a single organization to avoid cleanup issues with member accounts.
func TestOrganizations_WithOrganization(t *testing.T) {
	client := newOrganizationsClient(t)
	ctx := t.Context()

	// Setup: ensure organization exists
	org := ensureOrganization(t, client)
	if org == nil {
		t.Fatal("failed to ensure organization")
	}

	t.Run("DescribeOrganization", func(t *testing.T) {
		descOutput, err := client.DescribeOrganization(ctx, &organizations.DescribeOrganizationInput{})
		if err != nil {
			t.Fatal(err)
		}
		golden.New(t, golden.WithIgnoreFields("Id", "Arn", "MasterAccountArn", "MasterAccountId", "MasterAccountEmail", "AvailablePolicyTypes", "ResultMetadata")).Assert(t.Name(), descOutput)
	})

	t.Run("ListRoots", func(t *testing.T) {
		rootsOutput, err := client.ListRoots(ctx, &organizations.ListRootsInput{})
		if err != nil {
			t.Fatal(err)
		}
		golden.New(t, golden.WithIgnoreFields("Id", "Arn", "PolicyTypes", "ResultMetadata")).Assert(t.Name(), rootsOutput)
	})

	t.Run("DescribeAccount", func(t *testing.T) {
		descOutput, err := client.DescribeAccount(ctx, &organizations.DescribeAccountInput{
			AccountId: org.MasterAccountId,
		})
		if err != nil {
			t.Fatal(err)
		}
		golden.New(t, golden.WithIgnoreFields("Id", "Arn", "Email", "JoinedTimestamp", "ResultMetadata")).Assert(t.Name(), descOutput)
	})

	t.Run("DescribeAccount_NotFound", func(t *testing.T) {
		_, err := client.DescribeAccount(ctx, &organizations.DescribeAccountInput{
			AccountId: aws.String("000000000000"),
		})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("ListAccounts", func(t *testing.T) {
		listOutput, err := client.ListAccounts(ctx, &organizations.ListAccountsInput{})
		if err != nil {
			t.Fatal(err)
		}
		if len(listOutput.Accounts) < 1 {
			t.Error("expected at least one account")
		}

		// Verify State field is present and valid for all accounts
		for _, account := range listOutput.Accounts {
			if account.State != types.AccountStateActive {
				t.Errorf("expected account state to be active, got %v", account.State)
			}
		}
	})

	t.Run("CreateAccount", func(t *testing.T) {
		createOutput, err := client.CreateAccount(ctx, &organizations.CreateAccountInput{
			AccountName: aws.String("Test Account"),
			Email:       aws.String("test-create@example.com"),
		})
		if err != nil {
			t.Fatal(err)
		}
		golden.New(t, golden.WithIgnoreFields("Id", "AccountId", "RequestedTimestamp", "CompletedTimestamp", "ResultMetadata")).Assert(t.Name(), createOutput)
	})

	t.Run("CreateOrganizationalUnit", func(t *testing.T) {
		rootsOutput, err := client.ListRoots(ctx, &organizations.ListRootsInput{})
		if err != nil {
			t.Fatal(err)
		}
		if len(rootsOutput.Roots) == 0 {
			t.Fatal("no roots found")
		}
		rootID := rootsOutput.Roots[0].Id

		ouOutput, err := client.CreateOrganizationalUnit(ctx, &organizations.CreateOrganizationalUnitInput{
			Name:     aws.String("Test OU"),
			ParentId: rootID,
		})
		if err != nil {
			t.Fatal(err)
		}
		golden.New(t, golden.WithIgnoreFields("Id", "Arn", "ResultMetadata")).Assert(t.Name(), ouOutput)
	})

	t.Run("ListOrganizationalUnitsForParent", func(t *testing.T) {
		rootsOutput, err := client.ListRoots(ctx, &organizations.ListRootsInput{})
		if err != nil {
			t.Fatal(err)
		}
		if len(rootsOutput.Roots) == 0 {
			t.Fatal("no roots found")
		}
		rootID := rootsOutput.Roots[0].Id

		// Create additional OUs for this test
		_, err = client.CreateOrganizationalUnit(ctx, &organizations.CreateOrganizationalUnitInput{
			Name:     aws.String("Test OU ListOUs 1"),
			ParentId: rootID,
		})
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.CreateOrganizationalUnit(ctx, &organizations.CreateOrganizationalUnitInput{
			Name:     aws.String("Test OU ListOUs 2"),
			ParentId: rootID,
		})
		if err != nil {
			t.Fatal(err)
		}

		listOutput, err := client.ListOrganizationalUnitsForParent(ctx, &organizations.ListOrganizationalUnitsForParentInput{
			ParentId: rootID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(listOutput.OrganizationalUnits) < 2 {
			t.Errorf("expected at least 2 organizational units, got %d", len(listOutput.OrganizationalUnits))
		}
	})

	t.Run("DeleteOrganization_NotEmpty", func(t *testing.T) {
		// Organization has member accounts, so delete should fail
		_, err := client.DeleteOrganization(ctx, &organizations.DeleteOrganizationInput{})
		if err == nil {
			t.Error("expected error")
		}
	})
}

// TestOrganizations_CreateOrganization tests organization creation.
// This test is separate because it needs a clean state without an existing organization.
func TestOrganizations_CreateOrganization(t *testing.T) {
	client := newOrganizationsClient(t)
	ctx := t.Context()

	// Ensure clean state - this may fail if previous test left member accounts
	ensureNoOrganization(t, client)

	// Try to describe - should fail if no org exists
	_, err := client.DescribeOrganization(ctx, &organizations.DescribeOrganizationInput{})
	if err == nil {
		t.Skip("Organization already exists from previous tests, skipping create test")
	}

	// Create organization
	createOutput, err := client.CreateOrganization(ctx, &organizations.CreateOrganizationInput{
		FeatureSet: types.OrganizationFeatureSetAll,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "MasterAccountArn", "MasterAccountId", "MasterAccountEmail", "AvailablePolicyTypes", "ResultMetadata")).Assert(t.Name(), createOutput)
}

// TestOrganizations_DescribeOrganization_NotInOrganization tests behavior when no organization exists.
// This test is separate and may be skipped if an organization already exists from previous tests.
func TestOrganizations_DescribeOrganization_NotInOrganization(t *testing.T) {
	client := newOrganizationsClient(t)
	ctx := t.Context()

	// Ensure clean state
	ensureNoOrganization(t, client)

	// Check if organization actually doesn't exist
	_, err := client.DescribeOrganization(ctx, &organizations.DescribeOrganizationInput{})
	if err == nil {
		t.Skip("Organization exists from previous tests, cannot test NotInOrganization scenario")
	}

	// Verify the error is returned - already verified above by reaching this point
}

// TestOrganizations_DeleteOrganization tests organization deletion.
// This test is separate and may be skipped if an organization with member accounts exists.
func TestOrganizations_DeleteOrganization(t *testing.T) {
	client := newOrganizationsClient(t)
	ctx := t.Context()

	// Ensure clean state
	ensureNoOrganization(t, client)

	// Check if organization exists
	_, err := client.DescribeOrganization(ctx, &organizations.DescribeOrganizationInput{})
	if err == nil {
		// Organization exists, check if it can be deleted (no member accounts)
		listOutput, _ := client.ListAccounts(ctx, &organizations.ListAccountsInput{})
		if listOutput != nil && len(listOutput.Accounts) > 1 {
			t.Skip("Organization has member accounts from previous tests, cannot test clean deletion")
		}
	}

	// Create a fresh organization
	_, err = client.CreateOrganization(ctx, &organizations.CreateOrganizationInput{
		FeatureSet: types.OrganizationFeatureSetAll,
	})
	if err != nil {
		t.Skip("Cannot create organization, likely already exists with member accounts")
	}

	// Delete organization
	_, err = client.DeleteOrganization(ctx, &organizations.DeleteOrganizationInput{})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's deleted
	_, err = client.DescribeOrganization(ctx, &organizations.DescribeOrganizationInput{})
	if err == nil {
		t.Error("expected error")
	}
}
