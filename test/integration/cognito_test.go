//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/sivchari/golden"
)

func newCognitoClient(t *testing.T) *cognitoidentityprovider.Client {
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

	return cognitoidentityprovider.NewFromConfig(cfg, func(o *cognitoidentityprovider.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestCognito_CreateAndDescribeUserPool(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	// Create user pool.
	createOutput, err := client.CreateUserPool(ctx, &cognitoidentityprovider.CreateUserPoolInput{
		PoolName: aws.String("test-user-pool"),
		Policies: &types.UserPoolPolicyType{
			PasswordPolicy: &types.PasswordPolicyType{
				MinimumLength:    aws.Int32(8),
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumbers:   true,
				RequireSymbols:   false,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreationDate", "LastModifiedDate", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	userPoolID := *createOutput.UserPool.Id

	// Describe user pool.
	describeOutput, err := client.DescribeUserPool(ctx, &cognitoidentityprovider.DescribeUserPoolInput{
		UserPoolId: aws.String(userPoolID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreationDate", "LastModifiedDate", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestCognito_ListUserPools(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	// Create a user pool first.
	_, err := client.CreateUserPool(ctx, &cognitoidentityprovider.CreateUserPoolInput{
		PoolName: aws.String("test-list-user-pool"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List user pools.
	listOutput, err := client.ListUserPools(ctx, &cognitoidentityprovider.ListUserPoolsInput{
		MaxResults: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreationDate", "LastModifiedDate", "ResultMetadata")).Assert(t.Name(), listOutput)
}

func TestCognito_CreateAndDescribeUserPoolClient(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	// Create user pool first.
	poolOutput, err := client.CreateUserPool(ctx, &cognitoidentityprovider.CreateUserPoolInput{
		PoolName: aws.String("test-client-user-pool"),
	})
	if err != nil {
		t.Fatal(err)
	}

	userPoolID := *poolOutput.UserPool.Id

	// Create user pool client.
	clientOutput, err := client.CreateUserPoolClient(ctx, &cognitoidentityprovider.CreateUserPoolClientInput{
		UserPoolId: aws.String(userPoolID),
		ClientName: aws.String("test-client"),
		ExplicitAuthFlows: []types.ExplicitAuthFlowsType{
			types.ExplicitAuthFlowsTypeAllowUserPasswordAuth,
			types.ExplicitAuthFlowsTypeAllowRefreshTokenAuth,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ClientId", "UserPoolId", "CreationDate", "LastModifiedDate", "ResultMetadata")).Assert(t.Name()+"_create", clientOutput)

	clientID := *clientOutput.UserPoolClient.ClientId

	// Describe user pool client.
	describeOutput, err := client.DescribeUserPoolClient(ctx, &cognitoidentityprovider.DescribeUserPoolClientInput{
		UserPoolId: aws.String(userPoolID),
		ClientId:   aws.String(clientID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ClientId", "UserPoolId", "CreationDate", "LastModifiedDate", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestCognito_AdminCreateAndGetUser(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	// Create user pool.
	poolOutput, err := client.CreateUserPool(ctx, &cognitoidentityprovider.CreateUserPoolInput{
		PoolName: aws.String("test-admin-user-pool"),
	})
	if err != nil {
		t.Fatal(err)
	}

	userPoolID := *poolOutput.UserPool.Id

	// Admin create user.
	createUserOutput, err := client.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId:        aws.String(userPoolID),
		Username:          aws.String("testuser"),
		TemporaryPassword: aws.String("TempPass123!"),
		UserAttributes: []types.AttributeType{
			{
				Name:  aws.String("email"),
				Value: aws.String("testuser@example.com"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("UserCreateDate", "UserLastModifiedDate", "ResultMetadata")).Assert(t.Name()+"_create", createUserOutput)

	// Admin get user.
	getUserOutput, err := client.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(userPoolID),
		Username:   aws.String("testuser"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("UserCreateDate", "UserLastModifiedDate", "ResultMetadata")).Assert(t.Name()+"_get", getUserOutput)
}

func TestCognito_ListUsers(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	// Create user pool.
	poolOutput, err := client.CreateUserPool(ctx, &cognitoidentityprovider.CreateUserPoolInput{
		PoolName: aws.String("test-list-users-pool"),
	})
	if err != nil {
		t.Fatal(err)
	}

	userPoolID := *poolOutput.UserPool.Id

	// Create a user.
	_, err = client.AdminCreateUser(ctx, &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId:        aws.String(userPoolID),
		Username:          aws.String("listuser1"),
		TemporaryPassword: aws.String("TempPass123!"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List users.
	listOutput, err := client.ListUsers(ctx, &cognitoidentityprovider.ListUsersInput{
		UserPoolId: aws.String(userPoolID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("UserCreateDate", "UserLastModifiedDate", "ResultMetadata")).Assert(t.Name(), listOutput)
}

func TestCognito_SignUpAndConfirm(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	// Create user pool.
	poolOutput, err := client.CreateUserPool(ctx, &cognitoidentityprovider.CreateUserPoolInput{
		PoolName: aws.String("test-signup-pool"),
	})
	if err != nil {
		t.Fatal(err)
	}

	userPoolID := *poolOutput.UserPool.Id

	// Create user pool client.
	clientOutput, err := client.CreateUserPoolClient(ctx, &cognitoidentityprovider.CreateUserPoolClientInput{
		UserPoolId: aws.String(userPoolID),
		ClientName: aws.String("signup-client"),
		ExplicitAuthFlows: []types.ExplicitAuthFlowsType{
			types.ExplicitAuthFlowsTypeAllowUserPasswordAuth,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	clientID := *clientOutput.UserPoolClient.ClientId

	// Sign up.
	signUpOutput, err := client.SignUp(ctx, &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(clientID),
		Username: aws.String("signupuser"),
		Password: aws.String("Password123!"),
		UserAttributes: []types.AttributeType{
			{
				Name:  aws.String("email"),
				Value: aws.String("signupuser@example.com"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("UserSub", "ResultMetadata")).Assert(t.Name()+"_signup", signUpOutput)

	// Confirm sign up.
	_, err = client.ConfirmSignUp(ctx, &cognitoidentityprovider.ConfirmSignUpInput{
		ClientId:         aws.String(clientID),
		Username:         aws.String("signupuser"),
		ConfirmationCode: aws.String("123456"),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCognito_InitiateAuth(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	// Create user pool.
	poolOutput, err := client.CreateUserPool(ctx, &cognitoidentityprovider.CreateUserPoolInput{
		PoolName: aws.String("test-auth-pool"),
	})
	if err != nil {
		t.Fatal(err)
	}

	userPoolID := *poolOutput.UserPool.Id

	// Create user pool client.
	clientOutput, err := client.CreateUserPoolClient(ctx, &cognitoidentityprovider.CreateUserPoolClientInput{
		UserPoolId: aws.String(userPoolID),
		ClientName: aws.String("auth-client"),
		ExplicitAuthFlows: []types.ExplicitAuthFlowsType{
			types.ExplicitAuthFlowsTypeAllowUserPasswordAuth,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	clientID := *clientOutput.UserPoolClient.ClientId

	// Sign up and confirm user.
	_, err = client.SignUp(ctx, &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(clientID),
		Username: aws.String("authuser"),
		Password: aws.String("Password123!"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.ConfirmSignUp(ctx, &cognitoidentityprovider.ConfirmSignUpInput{
		ClientId:         aws.String(clientID),
		Username:         aws.String("authuser"),
		ConfirmationCode: aws.String("123456"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Initiate auth.
	authOutput, err := client.InitiateAuth(ctx, &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		ClientId: aws.String(clientID),
		AuthParameters: map[string]string{
			"USERNAME": "authuser",
			"PASSWORD": "Password123!",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AccessToken", "IdToken", "RefreshToken", "NewDeviceMetadata", "ResultMetadata")).Assert(t.Name(), authOutput)
}

func TestCognito_MfaConfig(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	// Create user pool.
	createOutput, err := client.CreateUserPool(ctx, &cognitoidentityprovider.CreateUserPoolInput{
		PoolName: aws.String("test-mfa-config-pool"),
	})
	if err != nil {
		t.Fatal(err)
	}

	userPoolID := *createOutput.UserPool.Id

	t.Cleanup(func() {
		_, _ = client.DeleteUserPool(ctx, &cognitoidentityprovider.DeleteUserPoolInput{
			UserPoolId: aws.String(userPoolID),
		})
	})

	// Get default MFA config (should be OFF).
	getOutput, err := client.GetUserPoolMfaConfig(ctx, &cognitoidentityprovider.GetUserPoolMfaConfigInput{
		UserPoolId: aws.String(userPoolID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get_default", getOutput)

	// Set MFA config to OPTIONAL with software token.
	setOutput, err := client.SetUserPoolMfaConfig(ctx, &cognitoidentityprovider.SetUserPoolMfaConfigInput{
		UserPoolId:       aws.String(userPoolID),
		MfaConfiguration: types.UserPoolMfaTypeOptional,
		SoftwareTokenMfaConfiguration: &types.SoftwareTokenMfaConfigType{
			Enabled: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_set", setOutput)

	// Get MFA config after setting it (should reflect the change).
	getOutput2, err := client.GetUserPoolMfaConfig(ctx, &cognitoidentityprovider.GetUserPoolMfaConfigInput{
		UserPoolId: aws.String(userPoolID),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get_after_set", getOutput2)
}

func TestCognito_UserPoolNotFound(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	// Try to describe a non-existent user pool.
	_, err := client.DescribeUserPool(ctx, &cognitoidentityprovider.DescribeUserPoolInput{
		UserPoolId: aws.String("us-east-1_nonexistent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent user pool")
	}
}

func TestCognito_DeleteUserPool(t *testing.T) {
	client := newCognitoClient(t)
	ctx := t.Context()

	// Create user pool.
	createOutput, err := client.CreateUserPool(ctx, &cognitoidentityprovider.CreateUserPoolInput{
		PoolName: aws.String("test-delete-pool"),
	})
	if err != nil {
		t.Fatal(err)
	}

	userPoolID := *createOutput.UserPool.Id

	// Delete user pool.
	_, err = client.DeleteUserPool(ctx, &cognitoidentityprovider.DeleteUserPoolInput{
		UserPoolId: aws.String(userPoolID),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.DescribeUserPool(ctx, &cognitoidentityprovider.DescribeUserPoolInput{
		UserPoolId: aws.String(userPoolID),
	})
	if err == nil {
		t.Fatal("expected error for deleted user pool")
	}
}
