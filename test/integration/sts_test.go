//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/sivchari/golden"
)

func newSTSClient(t *testing.T) *sts.Client {
	t.Helper()

	return sts.NewFromConfig(awsConfig(t), func(o *sts.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestSTS_GetCallerIdentity(t *testing.T) {
	client := newSTSClient(t)
	ctx := t.Context()

	result, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("Account", "UserId", "Arn", "ResultMetadata")).Assert(t.Name(), result)
}

func TestSTS_AssumeRole(t *testing.T) {
	client := newSTSClient(t)
	ctx := t.Context()

	result, err := client.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         aws.String("arn:aws:iam::000000000000:role/test-role"),
		RoleSessionName: aws.String("test-session"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("AccessKeyId", "SecretAccessKey", "SessionToken", "Expiration", "AssumedRoleId", "ResultMetadata")).Assert(t.Name(), result)
}

func TestSTS_AssumeRoleWithWebIdentity(t *testing.T) {
	client := newSTSClient(t)
	ctx := t.Context()

	result, err := client.AssumeRoleWithWebIdentity(ctx, &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String("arn:aws:iam::000000000000:role/web-identity-role"),
		RoleSessionName:  aws.String("web-session"),
		WebIdentityToken: aws.String("mock-token"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("AccessKeyId", "SecretAccessKey", "SessionToken", "Expiration", "AssumedRoleId", "ResultMetadata")).Assert(t.Name(), result)
}

func TestSTS_GetSessionToken(t *testing.T) {
	client := newSTSClient(t)
	ctx := t.Context()

	result, err := client.GetSessionToken(ctx, &sts.GetSessionTokenInput{})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("AccessKeyId", "SecretAccessKey", "SessionToken", "Expiration", "ResultMetadata")).Assert(t.Name(), result)
}

func TestSTS_GetFederationToken(t *testing.T) {
	client := newSTSClient(t)
	ctx := t.Context()

	result, err := client.GetFederationToken(ctx, &sts.GetFederationTokenInput{
		Name: aws.String("test-federated-user"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("AccessKeyId", "SecretAccessKey", "SessionToken", "Expiration", "Arn", "FederatedUserId", "ResultMetadata")).Assert(t.Name(), result)
}

func TestSTS_AssumeRole_MissingRoleArn(t *testing.T) {
	client := newSTSClient(t)
	ctx := t.Context()

	_, err := client.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         aws.String(""),
		RoleSessionName: aws.String("test-session"),
	})
	if err == nil {
		t.Fatal("expected error for missing RoleArn")
	}
}
