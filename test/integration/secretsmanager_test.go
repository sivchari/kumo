//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/sivchari/golden"
)

func newSecretsManagerClient(t *testing.T) *secretsmanager.Client {
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

	return secretsmanager.NewFromConfig(cfg, func(o *secretsmanager.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestSecretsManager_CreateAndDeleteSecret(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-create-delete"
	secretValue := "super-secret-value"

	// Create secret.
	createOutput, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(secretValue),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "VersionId", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Delete secret.
	deleteOutput, err := client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(secretName),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "DeletionDate", "ResultMetadata")).Assert(t.Name()+"_delete", deleteOutput)
}

func TestSecretsManager_GetSecretValue(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-get-value"
	secretValue := "my-secret-value-123"

	// Create secret.
	createOutput, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(secretValue),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "VersionId", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// Get secret value.
	getOutput, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "VersionId", "SecretString", "CreatedDate", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestSecretsManager_PutSecretValue(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-put-value"
	initialValue := "initial-value"
	updatedValue := "updated-value"

	// Create secret.
	_, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(initialValue),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// Put new secret value.
	putOutput, err := client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(updatedValue),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "VersionId", "ResultMetadata")).Assert(t.Name()+"_put", putOutput)

	// Get and verify updated value.
	getOutput, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "VersionId", "SecretString", "CreatedDate", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestSecretsManager_ListSecrets(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-list"

	// Create secret.
	_, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String("test-value"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// List secrets.
	listOutput, err := client.ListSecrets(ctx, &secretsmanager.ListSecretsInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, secret := range listOutput.SecretList {
		if *secret.Name == secretName {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("secret %s not found in list", secretName)
	}
}

func TestSecretsManager_DescribeSecret(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-describe"
	description := "This is a test secret"

	// Create secret with description.
	_, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String("test-value"),
		Description:  aws.String(description),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// Describe secret.
	describeOutput, err := client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "CreatedDate", "LastChangedDate", "VersionIdsToStages", "ResultMetadata")).Assert(t.Name(), describeOutput)
}

func TestSecretsManager_UpdateSecret(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-update"
	initialDescription := "Initial description"
	updatedDescription := "Updated description"
	updatedValue := "updated-secret-value"

	// Create secret.
	_, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String("initial-value"),
		Description:  aws.String(initialDescription),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// Update secret.
	updateOutput, err := client.UpdateSecret(ctx, &secretsmanager.UpdateSecretInput{
		SecretId:     aws.String(secretName),
		Description:  aws.String(updatedDescription),
		SecretString: aws.String(updatedValue),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "VersionId", "ResultMetadata")).Assert(t.Name()+"_update", updateOutput)

	// Verify description updated.
	describeOutput, err := client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "CreatedDate", "LastChangedDate", "VersionIdsToStages", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)

	// Verify value updated.
	getOutput, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "VersionId", "SecretString", "CreatedDate", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestSecretsManager_DeleteWithRecoveryWindow(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-recovery"

	// Create secret.
	_, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String("test-value"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete with recovery window.
	deleteOutput, err := client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:             aws.String(secretName),
		RecoveryWindowInDays: aws.Int64(7),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "DeletionDate", "ResultMetadata")).Assert(t.Name()+"_delete", deleteOutput)

	// Verify secret is not accessible.
	_, err = client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err == nil {
		t.Fatal("expected error when getting deleted secret")
	}

	// Force delete for cleanup.
	_, _ = client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(secretName),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
}

func TestSecretsManager_SecretWithBinary(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-binary"
	secretBinary := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	// Create secret with binary value.
	_, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretBinary: secretBinary,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// Get secret value.
	getOutput, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(getOutput.SecretBinary) != string(secretBinary) {
		t.Errorf("secret binary mismatch: got %v, want %v", getOutput.SecretBinary, secretBinary)
	}
}

func TestSecretsManager_SecretWithJSON(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-json"

	secretData := map[string]string{
		"username": "admin",
		"password": "supersecret123",
		"host":     "db.example.com",
	}

	secretJSON, err := json.Marshal(secretData)
	if err != nil {
		t.Fatal(err)
	}

	// Create secret with JSON value.
	_, err = client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(string(secretJSON)),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// Get secret value.
	getOutput, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Parse and verify JSON.
	var retrievedData map[string]string
	if err := json.Unmarshal([]byte(*getOutput.SecretString), &retrievedData); err != nil {
		t.Fatal(err)
	}

	if retrievedData["username"] != secretData["username"] {
		t.Errorf("username mismatch: got %s, want %s", retrievedData["username"], secretData["username"])
	}

	if retrievedData["password"] != secretData["password"] {
		t.Errorf("password mismatch: got %s, want %s", retrievedData["password"], secretData["password"])
	}
}

func TestSecretsManager_VersionStages(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-versions"

	// Create secret with initial value.
	_, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String("version-1"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// Put new version.
	_, err = client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String("version-2"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get current version (should be version-2).
	currentOutput, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if *currentOutput.SecretString != "version-2" {
		t.Errorf("current version mismatch: got %s, want version-2", *currentOutput.SecretString)
	}

	// Get previous version (should be version-1).
	previousOutput, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSPREVIOUS"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if *previousOutput.SecretString != "version-1" {
		t.Errorf("previous version mismatch: got %s, want version-1", *previousOutput.SecretString)
	}
}

func TestSecretsManager_GetSecretValueByPartialARN(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-partial-arn"
	secretValue := "partial-arn-value"

	// Create secret.
	_, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(secretValue),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// Get by partial ARN (without the random suffix that AWS appends).
	partialARN := "arn:aws:secretsmanager:us-east-1:000000000000:secret:" + secretName
	getOutput, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(partialARN),
	})
	if err != nil {
		t.Fatalf("GetSecretValue with partial ARN failed: %v", err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "VersionId", "CreatedDate", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestSecretsManager_GetSecretValueByExactARN(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-exact-arn"
	secretValue := "exact-arn-value"

	// Create secret and capture the ARN.
	createOutput, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(secretValue),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// Get by exact ARN (with the random suffix).
	getOutput, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: createOutput.ARN,
	})
	if err != nil {
		t.Fatalf("GetSecretValue with exact ARN failed: %v", err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "VersionId", "CreatedDate", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestSecretsManager_ResourcePolicy(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()
	secretName := "test-secret-resource-policy"

	// Create secret.
	_, err := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String("policy-test-value"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteSecret(context.Background(), &secretsmanager.DeleteSecretInput{
			SecretId:                   aws.String(secretName),
			ForceDeleteWithoutRecovery: aws.Bool(true),
		})
	})

	// GetResourcePolicy (empty - no policy attached yet).
	getOutput1, err := client.GetResourcePolicy(ctx, &secretsmanager.GetResourcePolicyInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "ResultMetadata")).Assert(t.Name()+"_get_empty", getOutput1)

	// PutResourcePolicy.
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"secretsmanager:GetSecretValue","Resource":"*"}]}`
	putOutput, err := client.PutResourcePolicy(ctx, &secretsmanager.PutResourcePolicyInput{
		SecretId:       aws.String(secretName),
		ResourcePolicy: aws.String(policy),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "ResultMetadata")).Assert(t.Name()+"_put", putOutput)

	// GetResourcePolicy (should contain the policy).
	getOutput2, err := client.GetResourcePolicy(ctx, &secretsmanager.GetResourcePolicyInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "ResultMetadata")).Assert(t.Name()+"_get_with_policy", getOutput2)

	// DeleteResourcePolicy.
	deleteOutput, err := client.DeleteResourcePolicy(ctx, &secretsmanager.DeleteResourcePolicyInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "ResultMetadata")).Assert(t.Name()+"_delete", deleteOutput)

	// GetResourcePolicy (empty again after deletion).
	getOutput3, err := client.GetResourcePolicy(ctx, &secretsmanager.GetResourcePolicyInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ARN", "ResultMetadata")).Assert(t.Name()+"_get_after_delete", getOutput3)
}

func TestSecretsManager_GetRandomPassword(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()

	// Default parameters.
	output, err := client.GetRandomPassword(ctx, &secretsmanager.GetRandomPasswordInput{})
	if err != nil {
		t.Fatal(err)
	}

	if output.RandomPassword == nil {
		t.Fatal("expected non-nil RandomPassword")
	}

	if len(*output.RandomPassword) != 32 {
		t.Errorf("expected default length 32, got %d", len(*output.RandomPassword))
	}
}

func TestSecretsManager_GetRandomPassword_CustomLength(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()

	output, err := client.GetRandomPassword(ctx, &secretsmanager.GetRandomPasswordInput{
		PasswordLength: aws.Int64(64),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(*output.RandomPassword) != 64 {
		t.Errorf("expected length 64, got %d", len(*output.RandomPassword))
	}
}

func TestSecretsManager_GetRandomPassword_ExcludeTypes(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()

	// Only lowercase letters.
	output, err := client.GetRandomPassword(ctx, &secretsmanager.GetRandomPasswordInput{
		PasswordLength:     aws.Int64(100),
		ExcludeUppercase:   aws.Bool(true),
		ExcludeNumbers:     aws.Bool(true),
		ExcludePunctuation: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	password := *output.RandomPassword
	for _, c := range password {
		if !unicode.IsLower(c) {
			t.Errorf("expected only lowercase, got %c", c)

			break
		}
	}
}

func TestSecretsManager_GetRandomPassword_ExcludeCharacters(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()

	output, err := client.GetRandomPassword(ctx, &secretsmanager.GetRandomPasswordInput{
		PasswordLength:    aws.Int64(100),
		ExcludeCharacters: aws.String("aeiouAEIOU"),
	})
	if err != nil {
		t.Fatal(err)
	}

	password := *output.RandomPassword
	if strings.ContainsAny(password, "aeiouAEIOU") {
		t.Errorf("password contains excluded vowels: %s", password)
	}
}

func TestSecretsManager_GetRandomPassword_RequireEachType(t *testing.T) {
	client := newSecretsManagerClient(t)
	ctx := t.Context()

	output, err := client.GetRandomPassword(ctx, &secretsmanager.GetRandomPasswordInput{
		PasswordLength:          aws.Int64(20),
		RequireEachIncludedType: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	password := *output.RandomPassword
	hasUpper := strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	hasLower := strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz")
	hasDigit := strings.ContainsAny(password, "0123456789")

	if !hasUpper {
		t.Error("expected at least one uppercase letter")
	}

	if !hasLower {
		t.Error("expected at least one lowercase letter")
	}

	if !hasDigit {
		t.Error("expected at least one digit")
	}
}
