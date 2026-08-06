//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/finspace"
	"github.com/sivchari/golden"
)

func newFinSpaceClient(t *testing.T) *finspace.Client {
	t.Helper()

	return finspace.NewFromConfig(awsConfig(t), func(o *finspace.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestFinSpace_CreateAndDeleteKxEnvironment(t *testing.T) {
	client := newFinSpaceClient(t)
	ctx := t.Context()

	// Create environment.
	createOutput, err := client.CreateKxEnvironment(ctx, &finspace.CreateKxEnvironmentInput{
		Name:     aws.String("test-environment"),
		KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
	})
	if err != nil {
		t.Fatal(err)
	}

	environmentID := *createOutput.EnvironmentId

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "EnvironmentId", "EnvironmentArn", "CreationTimestamp"),
	)
	g.Assert("create", createOutput)

	// Get environment.
	getOutput, err := client.GetKxEnvironment(ctx, &finspace.GetKxEnvironmentInput{
		EnvironmentId: aws.String(environmentID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "EnvironmentId", "EnvironmentArn", "CreationTimestamp", "UpdateTimestamp"),
	)
	g2.Assert("get", getOutput)

	// List environments.
	listOutput, err := client.ListKxEnvironments(ctx, &finspace.ListKxEnvironmentsInput{})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "EnvironmentId", "EnvironmentArn", "CreationTimestamp", "UpdateTimestamp"),
	)
	g3.Assert("list", listOutput)

	// Delete environment.
	_, err = client.DeleteKxEnvironment(ctx, &finspace.DeleteKxEnvironmentInput{
		EnvironmentId: aws.String(environmentID),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's deleted - should return error.
	_, err = client.GetKxEnvironment(ctx, &finspace.GetKxEnvironmentInput{
		EnvironmentId: aws.String(environmentID),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFinSpace_CreateAndDeleteKxDatabase(t *testing.T) {
	client := newFinSpaceClient(t)
	ctx := t.Context()

	// First create an environment.
	createEnvOutput, err := client.CreateKxEnvironment(ctx, &finspace.CreateKxEnvironmentInput{
		Name:     aws.String("test-env-for-db"),
		KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
	})
	if err != nil {
		t.Fatal(err)
	}

	environmentID := *createEnvOutput.EnvironmentId

	t.Cleanup(func() {
		_, _ = client.DeleteKxEnvironment(t.Context(), &finspace.DeleteKxEnvironmentInput{
			EnvironmentId: aws.String(environmentID),
		})
	})

	// Create database.
	createOutput, err := client.CreateKxDatabase(ctx, &finspace.CreateKxDatabaseInput{
		EnvironmentId: aws.String(environmentID),
		DatabaseName:  aws.String("test-database"),
		Description:   aws.String("Test database"),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "EnvironmentId", "DatabaseArn", "CreatedTimestamp"),
	)
	g.Assert("create", createOutput)

	// Get database.
	getOutput, err := client.GetKxDatabase(ctx, &finspace.GetKxDatabaseInput{
		EnvironmentId: aws.String(environmentID),
		DatabaseName:  aws.String("test-database"),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "EnvironmentId", "DatabaseArn", "CreatedTimestamp", "LastModifiedTimestamp"),
	)
	g2.Assert("get", getOutput)

	// List databases.
	listOutput, err := client.ListKxDatabases(ctx, &finspace.ListKxDatabasesInput{
		EnvironmentId: aws.String(environmentID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "EnvironmentId", "DatabaseArn", "CreatedTimestamp", "LastModifiedTimestamp"),
	)
	g3.Assert("list", listOutput)

	// Delete database.
	_, err = client.DeleteKxDatabase(ctx, &finspace.DeleteKxDatabaseInput{
		EnvironmentId: aws.String(environmentID),
		DatabaseName:  aws.String("test-database"),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFinSpace_CreateAndDeleteKxUser(t *testing.T) {
	client := newFinSpaceClient(t)
	ctx := t.Context()

	// First create an environment.
	createEnvOutput, err := client.CreateKxEnvironment(ctx, &finspace.CreateKxEnvironmentInput{
		Name:     aws.String("test-env-for-user"),
		KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
	})
	if err != nil {
		t.Fatal(err)
	}

	environmentID := *createEnvOutput.EnvironmentId

	t.Cleanup(func() {
		_, _ = client.DeleteKxEnvironment(t.Context(), &finspace.DeleteKxEnvironmentInput{
			EnvironmentId: aws.String(environmentID),
		})
	})

	// Create user.
	createOutput, err := client.CreateKxUser(ctx, &finspace.CreateKxUserInput{
		EnvironmentId: aws.String(environmentID),
		UserName:      aws.String("test-user"),
		IamRole:       aws.String("arn:aws:iam::123456789012:role/TestRole"),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "EnvironmentId", "UserArn"),
	)
	g.Assert("create", createOutput)

	// Get user.
	getOutput, err := client.GetKxUser(ctx, &finspace.GetKxUserInput{
		EnvironmentId: aws.String(environmentID),
		UserName:      aws.String("test-user"),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "EnvironmentId", "UserArn"),
	)
	g2.Assert("get", getOutput)

	// List users.
	listOutput, err := client.ListKxUsers(ctx, &finspace.ListKxUsersInput{
		EnvironmentId: aws.String(environmentID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "UserArn", "CreateTimestamp", "UpdateTimestamp"),
	)
	g3.Assert("list", listOutput)

	// Delete user.
	_, err = client.DeleteKxUser(ctx, &finspace.DeleteKxUserInput{
		EnvironmentId: aws.String(environmentID),
		UserName:      aws.String("test-user"),
	})
	if err != nil {
		t.Fatal(err)
	}
}
