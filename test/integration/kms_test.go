//go:build integration

package integration

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/sivchari/golden"
)

func newKMSClient(t *testing.T) *kms.Client {
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

	return kms.NewFromConfig(cfg, func(o *kms.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestKMS_CreateAndDescribeKey(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Create key.
	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test key"),
		KeyUsage:    types.KeyUsageTypeEncryptDecrypt,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "KeyId", "Arn", "CreationDate")).Assert(t.Name()+"_create", createOutput)

	keyID := *createOutput.KeyMetadata.KeyId

	// Describe key.
	describeOutput, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "KeyId", "Arn", "CreationDate")).Assert(t.Name()+"_describe", describeOutput)
}

func TestKMS_ListKeys(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Create a key first.
	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test list key"),
	})
	if err != nil {
		t.Fatal(err)
	}

	keyID := *createOutput.KeyMetadata.KeyId

	// List keys.
	listOutput, err := client.ListKeys(ctx, &kms.ListKeysInput{
		Limit: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Find our key.
	found := false
	for _, key := range listOutput.Keys {
		if *key.KeyId == keyID {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("created key %s not found in list", keyID)
	}
}

func TestKMS_EnableDisableKey(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Create key.
	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test enable/disable key"),
	})
	if err != nil {
		t.Fatal(err)
	}

	keyID := *createOutput.KeyMetadata.KeyId

	// Disable key.
	_, err = client.DisableKey(ctx, &kms.DisableKeyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify disabled.
	describeAfterDisable, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "KeyId", "Arn", "CreationDate")).Assert(t.Name()+"_disabled", describeAfterDisable)

	// Enable key.
	_, err = client.EnableKey(ctx, &kms.EnableKeyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify enabled.
	describeAfterEnable, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "KeyId", "Arn", "CreationDate")).Assert(t.Name()+"_enabled", describeAfterEnable)
}

func TestKMS_ScheduleKeyDeletion(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Create key.
	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test deletion key"),
	})
	if err != nil {
		t.Fatal(err)
	}

	keyID := *createOutput.KeyMetadata.KeyId

	// Schedule deletion.
	deleteOutput, err := client.ScheduleKeyDeletion(ctx, &kms.ScheduleKeyDeletionInput{
		KeyId:               aws.String(keyID),
		PendingWindowInDays: aws.Int32(7),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "KeyId", "DeletionDate")).Assert(t.Name()+"_schedule", deleteOutput)

	// Verify pending deletion.
	describeOutput, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "KeyId", "Arn", "CreationDate", "DeletionDate")).Assert(t.Name()+"_describe", describeOutput)
}

func TestKMS_EncryptDecrypt(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Create key.
	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test encrypt/decrypt key"),
		KeyUsage:    types.KeyUsageTypeEncryptDecrypt,
	})
	if err != nil {
		t.Fatal(err)
	}

	keyID := *createOutput.KeyMetadata.KeyId
	plaintext := []byte("Hello, KMS!")

	// Encrypt.
	encryptOutput, err := client.Encrypt(ctx, &kms.EncryptInput{
		KeyId:     aws.String(keyID),
		Plaintext: plaintext,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(encryptOutput.CiphertextBlob) == 0 {
		t.Fatal("ciphertext is empty")
	}

	// Decrypt.
	decryptOutput, err := client.Decrypt(ctx, &kms.DecryptInput{
		CiphertextBlob: encryptOutput.CiphertextBlob,
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(decryptOutput.Plaintext) != string(plaintext) {
		t.Errorf("plaintext mismatch: got %s, want %s", decryptOutput.Plaintext, plaintext)
	}
}

func TestKMS_GenerateDataKey(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Create key.
	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test generate data key"),
		KeyUsage:    types.KeyUsageTypeEncryptDecrypt,
	})
	if err != nil {
		t.Fatal(err)
	}

	keyID := *createOutput.KeyMetadata.KeyId

	// Generate data key.
	dataKeyOutput, err := client.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{
		KeyId:   aws.String(keyID),
		KeySpec: types.DataKeySpecAes256,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(dataKeyOutput.Plaintext) == 0 {
		t.Fatal("plaintext data key is empty")
	}

	if len(dataKeyOutput.CiphertextBlob) == 0 {
		t.Fatal("ciphertext data key is empty")
	}

	if len(dataKeyOutput.Plaintext) != 32 {
		t.Errorf("plaintext data key should be 32 bytes, got %d", len(dataKeyOutput.Plaintext))
	}

	// Verify we can decrypt the ciphertext blob.
	decryptOutput, err := client.Decrypt(ctx, &kms.DecryptInput{
		CiphertextBlob: dataKeyOutput.CiphertextBlob,
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(decryptOutput.Plaintext) != string(dataKeyOutput.Plaintext) {
		t.Error("decrypted data key does not match original plaintext")
	}
}

func TestKMS_CreateAndDeleteAlias(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Create key.
	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test alias key"),
	})
	if err != nil {
		t.Fatal(err)
	}

	keyID := *createOutput.KeyMetadata.KeyId
	aliasName := "alias/test-alias"

	// Create alias.
	_, err = client.CreateAlias(ctx, &kms.CreateAliasInput{
		AliasName:   aws.String(aliasName),
		TargetKeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List aliases.
	listOutput, err := client.ListAliases(ctx, &kms.ListAliasesInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, alias := range listOutput.Aliases {
		if *alias.AliasName == aliasName {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("alias %s not found in list", aliasName)
	}

	// Delete alias.
	_, err = client.DeleteAlias(ctx, &kms.DeleteAliasInput{
		AliasName: aws.String(aliasName),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestKMS_KeyNotFound(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Try to describe a non-existent key.
	_, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: aws.String("non-existent-key-id"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent key")
	}
}

func TestKMS_EncryptWithAlias(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Create key.
	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test encrypt with alias"),
		KeyUsage:    types.KeyUsageTypeEncryptDecrypt,
	})
	if err != nil {
		t.Fatal(err)
	}

	keyID := *createOutput.KeyMetadata.KeyId
	aliasName := "alias/test-encrypt-alias"

	// Create alias.
	_, err = client.CreateAlias(ctx, &kms.CreateAliasInput{
		AliasName:   aws.String(aliasName),
		TargetKeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteAlias(context.Background(), &kms.DeleteAliasInput{
			AliasName: aws.String(aliasName),
		})
	})

	plaintext := []byte("Hello via alias!")

	// Encrypt using alias.
	encryptOutput, err := client.Encrypt(ctx, &kms.EncryptInput{
		KeyId:     aws.String(aliasName),
		Plaintext: plaintext,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Decrypt.
	decryptOutput, err := client.Decrypt(ctx, &kms.DecryptInput{
		CiphertextBlob: encryptOutput.CiphertextBlob,
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(decryptOutput.Plaintext) != string(plaintext) {
		t.Errorf("plaintext mismatch: got %s, want %s", decryptOutput.Plaintext, plaintext)
	}
}

func TestKMS_KeyPolicy(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	customPolicy := `{"Version":"2012-10-17","Statement":[{"Sid":"Custom","Effect":"Allow","Principal":{"AWS":"*"},"Action":"kms:*","Resource":"*"}]}`

	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("test-key-policy"),
		Policy:      aws.String(customPolicy),
	})
	if err != nil {
		t.Fatal(err)
	}

	keyID := *createOutput.KeyMetadata.KeyId

	t.Cleanup(func() {
		_, _ = client.ScheduleKeyDeletion(context.Background(), &kms.ScheduleKeyDeletionInput{
			KeyId:               aws.String(keyID),
			PendingWindowInDays: aws.Int32(7),
		})
	})

	// GetKeyPolicy should return the custom policy.
	getOutput, err := client.GetKeyPolicy(ctx, &kms.GetKeyPolicyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get", getOutput)

	// PutKeyPolicy with a new policy.
	newPolicy := `{"Version":"2012-10-17","Statement":[{"Sid":"Updated","Effect":"Allow","Principal":{"AWS":"*"},"Action":"kms:Encrypt","Resource":"*"}]}`
	_, err = client.PutKeyPolicy(ctx, &kms.PutKeyPolicyInput{
		KeyId:  aws.String(keyID),
		Policy: aws.String(newPolicy),
	})
	if err != nil {
		t.Fatal(err)
	}

	// GetKeyPolicy should return the updated policy.
	getOutput2, err := client.GetKeyPolicy(ctx, &kms.GetKeyPolicyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_updated", getOutput2)

	// ListKeyPolicies should always return ["default"].
	listOutput, err := client.ListKeyPolicies(ctx, &kms.ListKeyPoliciesInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_list", listOutput)
}

func TestKMS_SignVerify(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Create an RSA asymmetric signing key.
	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test RSA signing key"),
		KeyUsage:    types.KeyUsageTypeSignVerify,
		KeySpec:     types.KeySpecRsa2048,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "KeyId", "Arn", "CreationDate")).Assert(t.Name()+"_create", createOutput)

	keyID := *createOutput.KeyMetadata.KeyId

	t.Cleanup(func() {
		_, _ = client.ScheduleKeyDeletion(context.Background(), &kms.ScheduleKeyDeletionInput{
			KeyId:               aws.String(keyID),
			PendingWindowInDays: aws.Int32(7),
		})
	})

	message := []byte("CloudFront signed cookie payload")

	// Sign the message.
	signOutput, err := client.Sign(ctx, &kms.SignInput{
		KeyId:            aws.String(keyID),
		Message:          message,
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(signOutput.Signature) == 0 {
		t.Fatal("signature is empty")
	}

	// Verify the signature.
	verifyOutput, err := client.Verify(ctx, &kms.VerifyInput{
		KeyId:            aws.String(keyID),
		Message:          message,
		Signature:        signOutput.Signature,
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !verifyOutput.SignatureValid {
		t.Error("expected signature to be valid")
	}

	// Verifying a tampered message must fail.
	_, err = client.Verify(ctx, &kms.VerifyInput{
		KeyId:            aws.String(keyID),
		Message:          []byte("tampered payload"),
		Signature:        signOutput.Signature,
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	})
	if err == nil {
		t.Error("expected verification of tampered message to fail")
	}
}

func TestKMS_SignVerifyECC(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test ECC signing key"),
		KeyUsage:    types.KeyUsageTypeSignVerify,
		KeySpec:     types.KeySpecEccNistP256,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "KeyId", "Arn", "CreationDate")).Assert(t.Name()+"_create", createOutput)

	keyID := *createOutput.KeyMetadata.KeyId

	t.Cleanup(func() {
		_, _ = client.ScheduleKeyDeletion(context.Background(), &kms.ScheduleKeyDeletionInput{
			KeyId:               aws.String(keyID),
			PendingWindowInDays: aws.Int32(7),
		})
	})

	message := []byte("hello ecc")

	signOutput, err := client.Sign(ctx, &kms.SignInput{
		KeyId:            aws.String(keyID),
		Message:          message,
		SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	verifyOutput, err := client.Verify(ctx, &kms.VerifyInput{
		KeyId:            aws.String(keyID),
		Message:          message,
		Signature:        signOutput.Signature,
		SigningAlgorithm: types.SigningAlgorithmSpecEcdsaSha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !verifyOutput.SignatureValid {
		t.Error("expected signature to be valid")
	}
}

func TestKMS_GetPublicKey(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("Test get public key"),
		KeyUsage:    types.KeyUsageTypeSignVerify,
		KeySpec:     types.KeySpecRsa2048,
	})
	if err != nil {
		t.Fatal(err)
	}

	keyID := *createOutput.KeyMetadata.KeyId

	t.Cleanup(func() {
		_, _ = client.ScheduleKeyDeletion(context.Background(), &kms.ScheduleKeyDeletionInput{
			KeyId:               aws.String(keyID),
			PendingWindowInDays: aws.Int32(7),
		})
	})

	pubOutput, err := client.GetPublicKey(ctx, &kms.GetPublicKeyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(pubOutput.PublicKey) == 0 {
		t.Fatal("public key is empty")
	}

	// The returned public key must be a parseable DER SubjectPublicKeyInfo and
	// must verify a signature produced by the same key (offline verification).
	pubKey, err := x509.ParsePKIXPublicKey(pubOutput.PublicKey)
	if err != nil {
		t.Fatalf("failed to parse public key: %v", err)
	}

	rsaPub, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("expected RSA public key, got %T", pubKey)
	}

	message := []byte("offline verification")
	signOutput, err := client.Sign(ctx, &kms.SignInput{
		KeyId:            aws.String(keyID),
		Message:          message,
		SigningAlgorithm: types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	})
	if err != nil {
		t.Fatal(err)
	}

	digest := sha256.Sum256(message)
	if err := rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, digest[:], signOutput.Signature); err != nil {
		t.Errorf("offline verification failed: %v", err)
	}

	// PublicKey and key identifiers are dynamic; assert the stable shape only.
	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "KeyId", "PublicKey")).Assert(t.Name(), pubOutput)
}

func TestKMS_TagOperations(t *testing.T) {
	client := newKMSClient(t)
	ctx := t.Context()

	// Create a key with initial tags.
	createOutput, err := client.CreateKey(ctx, &kms.CreateKeyInput{
		Description: aws.String("test-tag-operations"),
		Tags: []types.Tag{
			{TagKey: aws.String("Env"), TagValue: aws.String("test")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	keyID := *createOutput.KeyMetadata.KeyId

	t.Cleanup(func() {
		_, _ = client.ScheduleKeyDeletion(context.Background(), &kms.ScheduleKeyDeletionInput{
			KeyId:               aws.String(keyID),
			PendingWindowInDays: aws.Int32(7),
		})
	})

	// ListResourceTags should return the initial tag.
	listOutput, err := client.ListResourceTags(ctx, &kms.ListResourceTagsInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_initial", listOutput)

	// TagResource adds more tags.
	_, err = client.TagResource(ctx, &kms.TagResourceInput{
		KeyId: aws.String(keyID),
		Tags: []types.Tag{
			{TagKey: aws.String("Team"), TagValue: aws.String("platform")},
			{TagKey: aws.String("Project"), TagValue: aws.String("kumo")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ListResourceTags should now return 3 tags.
	listOutput2, err := client.ListResourceTags(ctx, &kms.ListResourceTagsInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "Tags")).Assert(t.Name()+"_after_tag", listOutput2)

	// Verify that all 3 tags are present.
	if len(listOutput2.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(listOutput2.Tags))
	}

	// UntagResource removes one tag.
	_, err = client.UntagResource(ctx, &kms.UntagResourceInput{
		KeyId:   aws.String(keyID),
		TagKeys: []string{"Team"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ListResourceTags should now return 2 tags.
	listOutput3, err := client.ListResourceTags(ctx, &kms.ListResourceTagsInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata", "Tags")).Assert(t.Name()+"_after_untag", listOutput3)

	// Verify that 2 tags remain.
	if len(listOutput3.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(listOutput3.Tags))
	}
}
