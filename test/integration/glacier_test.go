//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glacier"
	"github.com/sivchari/golden"
)

func newGlacierClient(t *testing.T) *glacier.Client {
	t.Helper()

	return glacier.NewFromConfig(awsConfig(t), func(o *glacier.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestGlacier_CreateAndDeleteVault(t *testing.T) {
	client := newGlacierClient(t)
	ctx := t.Context()
	vaultName := "test-vault"

	_, err := client.CreateVault(ctx, &glacier.CreateVaultInput{
		AccountId: aws.String("-"),
		VaultName: aws.String(vaultName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete
	_, err = client.DeleteVault(ctx, &glacier.DeleteVaultInput{
		AccountId: aws.String("-"),
		VaultName: aws.String(vaultName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGlacier_DescribeVault(t *testing.T) {
	client := newGlacierClient(t)
	ctx := t.Context()
	vaultName := "test-describe-vault"

	_, err := client.CreateVault(ctx, &glacier.CreateVaultInput{
		AccountId: aws.String("-"),
		VaultName: aws.String(vaultName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteVault(t.Context(), &glacier.DeleteVaultInput{
			AccountId: aws.String("-"),
			VaultName: aws.String(vaultName),
		})
	})

	describeResult, err := client.DescribeVault(ctx, &glacier.DescribeVaultInput{
		AccountId: aws.String("-"),
		VaultName: aws.String(vaultName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("VaultARN", "CreationDate", "LastInventoryDate", "ResultMetadata")).Assert(t.Name(), describeResult)
}

func TestGlacier_ListVaults(t *testing.T) {
	client := newGlacierClient(t)
	ctx := t.Context()
	vaultName := "test-list-vault"

	_, err := client.CreateVault(ctx, &glacier.CreateVaultInput{
		AccountId: aws.String("-"),
		VaultName: aws.String(vaultName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteVault(t.Context(), &glacier.DeleteVaultInput{
			AccountId: aws.String("-"),
			VaultName: aws.String(vaultName),
		})
	})

	listResult, err := client.ListVaults(ctx, &glacier.ListVaultsInput{
		AccountId: aws.String("-"),
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, v := range listResult.VaultList {
		if *v.VaultName == vaultName {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected to find vault %s in list", vaultName)
	}
}

func TestGlacier_VaultNotFound(t *testing.T) {
	client := newGlacierClient(t)
	ctx := t.Context()

	_, err := client.DescribeVault(ctx, &glacier.DescribeVaultInput{
		AccountId: aws.String("-"),
		VaultName: aws.String("non-existent-vault"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent vault")
	}
}

func TestGlacier_CreateVaultIdempotent(t *testing.T) {
	client := newGlacierClient(t)
	ctx := t.Context()
	vaultName := "test-idempotent-vault"

	_, err := client.CreateVault(ctx, &glacier.CreateVaultInput{
		AccountId: aws.String("-"),
		VaultName: aws.String(vaultName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteVault(t.Context(), &glacier.DeleteVaultInput{
			AccountId: aws.String("-"),
			VaultName: aws.String(vaultName),
		})
	})

	// Create same vault again - should succeed (idempotent)
	_, err = client.CreateVault(ctx, &glacier.CreateVaultInput{
		AccountId: aws.String("-"),
		VaultName: aws.String(vaultName),
	})
	if err != nil {
		t.Fatal(err)
	}
}
