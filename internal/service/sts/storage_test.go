package sts

import (
	"testing"
)

type roleArnPartsCase struct {
	name          string
	roleArn       string
	wantPartition string
	wantAccount   string
	wantName      string
}

func roleArnPartsTestCases() []roleArnPartsCase {
	return []roleArnPartsCase{
		{
			name:          "simple role",
			roleArn:       "arn:aws:iam::123456789012:role/my-role",
			wantPartition: "aws",
			wantAccount:   "123456789012",
			wantName:      "my-role",
		},
		{
			name:          "role with path",
			roleArn:       "arn:aws:iam::123456789012:role/path/to/my-role",
			wantPartition: "aws",
			wantAccount:   "123456789012",
			wantName:      "my-role",
		},
		{
			name:          "role with trailing slash",
			roleArn:       "arn:aws:iam::123456789012:role/",
			wantPartition: "aws",
			wantAccount:   "123456789012",
			wantName:      "emulated-role",
		},
		{
			name:          "not an arn",
			roleArn:       "not-an-arn",
			wantPartition: "aws",
			wantAccount:   defaultAccountID,
			wantName:      "emulated-role",
		},
		{
			name:          "empty",
			roleArn:       "",
			wantPartition: "aws",
			wantAccount:   defaultAccountID,
			wantName:      "emulated-role",
		},
		{
			name:          "govcloud partition",
			roleArn:       "arn:aws-us-gov:iam::123456789012:role/gov-role",
			wantPartition: "aws-us-gov",
			wantAccount:   "123456789012",
			wantName:      "gov-role",
		},
		{
			name:          "china partition",
			roleArn:       "arn:aws-cn:iam::123456789012:role/cn-role",
			wantPartition: "aws-cn",
			wantAccount:   "123456789012",
			wantName:      "cn-role",
		},
	}
}

func assertRoleArnParts(t *testing.T, tt *roleArnPartsCase) {
	t.Helper()

	gotPartition, gotAccount, gotName := roleArnParts(tt.roleArn)
	if gotPartition != tt.wantPartition {
		t.Errorf("roleArnParts(%q) partition = %q, want %q", tt.roleArn, gotPartition, tt.wantPartition)
	}

	if gotAccount != tt.wantAccount {
		t.Errorf("roleArnParts(%q) account = %q, want %q", tt.roleArn, gotAccount, tt.wantAccount)
	}

	if gotName != tt.wantName {
		t.Errorf("roleArnParts(%q) name = %q, want %q", tt.roleArn, gotName, tt.wantName)
	}
}

func TestRoleArnParts(t *testing.T) {
	t.Parallel()

	for _, tt := range roleArnPartsTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertRoleArnParts(t, &tt)
		})
	}
}

func TestAssumeRoleReflectsRoleArn(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()

	result, err := storage.AssumeRole(t.Context(), &AssumeRoleInput{
		RoleArn:         "arn:aws:iam::123456789012:role/deploy-role",
		RoleSessionName: "ci",
	})
	if err != nil {
		t.Fatalf("AssumeRole() error = %v", err)
	}

	want := "arn:aws:sts::123456789012:assumed-role/deploy-role/ci"
	if result.AssumedRoleUser.Arn != want {
		t.Errorf("AssumedRoleUser.Arn = %q, want %q", result.AssumedRoleUser.Arn, want)
	}
}

func TestAssumeRoleReflectsGovCloudPartition(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage()

	result, err := storage.AssumeRole(t.Context(), &AssumeRoleInput{
		RoleArn:         "arn:aws-us-gov:iam::123456789012:role/gov-role",
		RoleSessionName: "ci",
	})
	if err != nil {
		t.Fatalf("AssumeRole() error = %v", err)
	}

	want := "arn:aws-us-gov:sts::123456789012:assumed-role/gov-role/ci"
	if result.AssumedRoleUser.Arn != want {
		t.Errorf("AssumedRoleUser.Arn = %q, want %q", result.AssumedRoleUser.Arn, want)
	}
}
