//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/sivchari/golden"
)

func newBackupClient(t *testing.T) *backup.Client {
	t.Helper()

	return backup.NewFromConfig(awsConfig(t), func(o *backup.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestBackup_CreateBackupVault(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	result, err := client.CreateBackupVault(ctx, &backup.CreateBackupVaultInput{
		BackupVaultName: aws.String("test-vault"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("BackupVaultArn", "CreationDate", "ResultMetadata")).Assert(t.Name(), result)
}

func TestBackup_DescribeBackupVault(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	_, err := client.CreateBackupVault(ctx, &backup.CreateBackupVaultInput{
		BackupVaultName: aws.String("describe-vault"),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.DescribeBackupVault(ctx, &backup.DescribeBackupVaultInput{
		BackupVaultName: aws.String("describe-vault"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("BackupVaultArn", "CreationDate", "CreatorRequestId", "ResultMetadata")).Assert(t.Name(), result)
}

func TestBackup_ListBackupVaults(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	_, err := client.CreateBackupVault(ctx, &backup.CreateBackupVaultInput{
		BackupVaultName: aws.String("list-vault"),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListBackupVaults(ctx, &backup.ListBackupVaultsInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.BackupVaultList) == 0 {
		t.Error("expected at least one backup vault")
	}
}

func TestBackup_DeleteBackupVault(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	_, err := client.CreateBackupVault(ctx, &backup.CreateBackupVaultInput{
		BackupVaultName: aws.String("delete-vault"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteBackupVault(ctx, &backup.DeleteBackupVaultInput{
		BackupVaultName: aws.String("delete-vault"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DescribeBackupVault(ctx, &backup.DescribeBackupVaultInput{
		BackupVaultName: aws.String("delete-vault"),
	})
	if err == nil {
		t.Fatal("expected error for deleted vault")
	}
}

func TestBackup_CreateBackupPlan(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	result, err := client.CreateBackupPlan(ctx, &backup.CreateBackupPlanInput{
		BackupPlan: &types.BackupPlanInput{
			BackupPlanName: aws.String("test-plan"),
			Rules: []types.BackupRuleInput{
				{
					RuleName:              aws.String("daily-rule"),
					TargetBackupVaultName: aws.String("test-vault"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("BackupPlanId", "BackupPlanArn", "CreationDate", "VersionId", "ResultMetadata")).Assert(t.Name(), result)
}

func TestBackup_GetBackupPlan(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	createResult, err := client.CreateBackupPlan(ctx, &backup.CreateBackupPlanInput{
		BackupPlan: &types.BackupPlanInput{
			BackupPlanName: aws.String("get-plan"),
			Rules: []types.BackupRuleInput{
				{
					RuleName:              aws.String("rule-1"),
					TargetBackupVaultName: aws.String("test-vault"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.GetBackupPlan(ctx, &backup.GetBackupPlanInput{
		BackupPlanId: createResult.BackupPlanId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("BackupPlanId", "BackupPlanArn", "CreationDate", "LastExecutionDate", "VersionId", "RuleId", "ResultMetadata")).Assert(t.Name(), result)
}

func TestBackup_ListBackupPlans(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	_, err := client.CreateBackupPlan(ctx, &backup.CreateBackupPlanInput{
		BackupPlan: &types.BackupPlanInput{
			BackupPlanName: aws.String("list-plan"),
			Rules: []types.BackupRuleInput{
				{
					RuleName:              aws.String("rule-1"),
					TargetBackupVaultName: aws.String("test-vault"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListBackupPlans(ctx, &backup.ListBackupPlansInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.BackupPlansList) == 0 {
		t.Error("expected at least one backup plan")
	}
}

func TestBackup_DeleteBackupPlan(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	createResult, err := client.CreateBackupPlan(ctx, &backup.CreateBackupPlanInput{
		BackupPlan: &types.BackupPlanInput{
			BackupPlanName: aws.String("delete-plan"),
			Rules: []types.BackupRuleInput{
				{
					RuleName:              aws.String("rule-1"),
					TargetBackupVaultName: aws.String("test-vault"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteBackupPlan(ctx, &backup.DeleteBackupPlanInput{
		BackupPlanId: createResult.BackupPlanId,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetBackupPlan(ctx, &backup.GetBackupPlanInput{
		BackupPlanId: createResult.BackupPlanId,
	})
	if err == nil {
		t.Fatal("expected error for deleted plan")
	}
}

func TestBackup_CreateBackupSelection(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	planResult, err := client.CreateBackupPlan(ctx, &backup.CreateBackupPlanInput{
		BackupPlan: &types.BackupPlanInput{
			BackupPlanName: aws.String("selection-plan"),
			Rules: []types.BackupRuleInput{
				{
					RuleName:              aws.String("rule-1"),
					TargetBackupVaultName: aws.String("test-vault"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.CreateBackupSelection(ctx, &backup.CreateBackupSelectionInput{
		BackupPlanId: planResult.BackupPlanId,
		BackupSelection: &types.BackupSelection{
			SelectionName: aws.String("test-selection"),
			IamRoleArn:    aws.String("arn:aws:iam::000000000000:role/test-role"),
			Resources:     []string{"arn:aws:ec2:us-east-1:000000000000:volume/*"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("SelectionId", "BackupPlanId", "CreationDate", "ResultMetadata")).Assert(t.Name(), result)
}

func TestBackup_GetBackupSelection(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	planResult, err := client.CreateBackupPlan(ctx, &backup.CreateBackupPlanInput{
		BackupPlan: &types.BackupPlanInput{
			BackupPlanName: aws.String("get-selection-plan"),
			Rules: []types.BackupRuleInput{
				{
					RuleName:              aws.String("rule-1"),
					TargetBackupVaultName: aws.String("test-vault"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	selResult, err := client.CreateBackupSelection(ctx, &backup.CreateBackupSelectionInput{
		BackupPlanId: planResult.BackupPlanId,
		BackupSelection: &types.BackupSelection{
			SelectionName: aws.String("get-selection"),
			IamRoleArn:    aws.String("arn:aws:iam::000000000000:role/test-role"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.GetBackupSelection(ctx, &backup.GetBackupSelectionInput{
		BackupPlanId: planResult.BackupPlanId,
		SelectionId:  selResult.SelectionId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("SelectionId", "BackupPlanId", "CreationDate", "ResultMetadata")).Assert(t.Name(), result)
}

func TestBackup_ListBackupSelections(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	planResult, err := client.CreateBackupPlan(ctx, &backup.CreateBackupPlanInput{
		BackupPlan: &types.BackupPlanInput{
			BackupPlanName: aws.String("list-selection-plan"),
			Rules: []types.BackupRuleInput{
				{
					RuleName:              aws.String("rule-1"),
					TargetBackupVaultName: aws.String("test-vault"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CreateBackupSelection(ctx, &backup.CreateBackupSelectionInput{
		BackupPlanId: planResult.BackupPlanId,
		BackupSelection: &types.BackupSelection{
			SelectionName: aws.String("list-selection"),
			IamRoleArn:    aws.String("arn:aws:iam::000000000000:role/test-role"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListBackupSelections(ctx, &backup.ListBackupSelectionsInput{
		BackupPlanId: planResult.BackupPlanId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("SelectionId", "BackupPlanId", "CreationDate", "ResultMetadata")).Assert(t.Name(), result)
}

func TestBackup_DeleteBackupSelection(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	planResult, err := client.CreateBackupPlan(ctx, &backup.CreateBackupPlanInput{
		BackupPlan: &types.BackupPlanInput{
			BackupPlanName: aws.String("delete-selection-plan"),
			Rules: []types.BackupRuleInput{
				{
					RuleName:              aws.String("rule-1"),
					TargetBackupVaultName: aws.String("test-vault"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	selResult, err := client.CreateBackupSelection(ctx, &backup.CreateBackupSelectionInput{
		BackupPlanId: planResult.BackupPlanId,
		BackupSelection: &types.BackupSelection{
			SelectionName: aws.String("delete-selection"),
			IamRoleArn:    aws.String("arn:aws:iam::000000000000:role/test-role"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteBackupSelection(ctx, &backup.DeleteBackupSelectionInput{
		BackupPlanId: planResult.BackupPlanId,
		SelectionId:  selResult.SelectionId,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetBackupSelection(ctx, &backup.GetBackupSelectionInput{
		BackupPlanId: planResult.BackupPlanId,
		SelectionId:  selResult.SelectionId,
	})
	if err == nil {
		t.Fatal("expected error for deleted selection")
	}
}

func TestBackup_VaultNotFound(t *testing.T) {
	client := newBackupClient(t)
	ctx := t.Context()

	_, err := client.DescribeBackupVault(ctx, &backup.DescribeBackupVaultInput{
		BackupVaultName: aws.String("nonexistent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent vault")
	}
}
