//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/amplify"
	"github.com/sivchari/golden"
)

func newAmplifyClient(t *testing.T) *amplify.Client {
	t.Helper()

	return amplify.NewFromConfig(awsConfig(t), func(o *amplify.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestAmplify_CreateApp(t *testing.T) {
	client := newAmplifyClient(t)
	ctx := t.Context()

	result, err := client.CreateApp(ctx, &amplify.CreateAppInput{
		Name: aws.String("test-app"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AppId", "AppArn", "CreateTime", "UpdateTime", "DefaultDomain", "ResultMetadata")).Assert(t.Name(), result)
}

func TestAmplify_GetApp(t *testing.T) {
	client := newAmplifyClient(t)
	ctx := t.Context()

	createResult, err := client.CreateApp(ctx, &amplify.CreateAppInput{
		Name: aws.String("get-app-test"),
	})
	if err != nil {
		t.Fatal(err)
	}

	getResult, err := client.GetApp(ctx, &amplify.GetAppInput{
		AppId: createResult.App.AppId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AppId", "AppArn", "CreateTime", "UpdateTime", "DefaultDomain", "ResultMetadata")).Assert(t.Name(), getResult)
}

func TestAmplify_ListApps(t *testing.T) {
	client := newAmplifyClient(t)
	ctx := t.Context()

	_, err := client.CreateApp(ctx, &amplify.CreateAppInput{
		Name: aws.String("list-app-test"),
	})
	if err != nil {
		t.Fatal(err)
	}

	listResult, err := client.ListApps(ctx, &amplify.ListAppsInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(listResult.Apps) == 0 {
		t.Error("expected at least one app")
	}
}

func TestAmplify_DeleteApp(t *testing.T) {
	client := newAmplifyClient(t)
	ctx := t.Context()

	createResult, err := client.CreateApp(ctx, &amplify.CreateAppInput{
		Name: aws.String("delete-app-test"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteApp(ctx, &amplify.DeleteAppInput{
		AppId: createResult.App.AppId,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetApp(ctx, &amplify.GetAppInput{
		AppId: createResult.App.AppId,
	})
	if err == nil {
		t.Fatal("expected error for deleted app")
	}
}

func TestAmplify_UpdateApp(t *testing.T) {
	client := newAmplifyClient(t)
	ctx := t.Context()

	createResult, err := client.CreateApp(ctx, &amplify.CreateAppInput{
		Name: aws.String("update-app-test"),
	})
	if err != nil {
		t.Fatal(err)
	}

	updateResult, err := client.UpdateApp(ctx, &amplify.UpdateAppInput{
		AppId:       createResult.App.AppId,
		Description: aws.String("updated description"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AppId", "AppArn", "CreateTime", "UpdateTime", "DefaultDomain", "ResultMetadata")).Assert(t.Name(), updateResult)
}

func TestAmplify_CreateBranch(t *testing.T) {
	client := newAmplifyClient(t)
	ctx := t.Context()

	appResult, err := client.CreateApp(ctx, &amplify.CreateAppInput{
		Name: aws.String("branch-test-app"),
	})
	if err != nil {
		t.Fatal(err)
	}

	branchResult, err := client.CreateBranch(ctx, &amplify.CreateBranchInput{
		AppId:      appResult.App.AppId,
		BranchName: aws.String("main"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("BranchArn", "CreateTime", "UpdateTime", "ResultMetadata")).Assert(t.Name(), branchResult)
}

func TestAmplify_ListBranches(t *testing.T) {
	client := newAmplifyClient(t)
	ctx := t.Context()

	appResult, err := client.CreateApp(ctx, &amplify.CreateAppInput{
		Name: aws.String("list-branches-app"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CreateBranch(ctx, &amplify.CreateBranchInput{
		AppId:      appResult.App.AppId,
		BranchName: aws.String("develop"),
	})
	if err != nil {
		t.Fatal(err)
	}

	listResult, err := client.ListBranches(ctx, &amplify.ListBranchesInput{
		AppId: appResult.App.AppId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("BranchArn", "CreateTime", "UpdateTime", "ResultMetadata")).Assert(t.Name(), listResult)
}

func TestAmplify_DeleteBranch(t *testing.T) {
	client := newAmplifyClient(t)
	ctx := t.Context()

	appResult, err := client.CreateApp(ctx, &amplify.CreateAppInput{
		Name: aws.String("delete-branch-app"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CreateBranch(ctx, &amplify.CreateBranchInput{
		AppId:      appResult.App.AppId,
		BranchName: aws.String("feature"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteBranch(ctx, &amplify.DeleteBranchInput{
		AppId:      appResult.App.AppId,
		BranchName: aws.String("feature"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetBranch(ctx, &amplify.GetBranchInput{
		AppId:      appResult.App.AppId,
		BranchName: aws.String("feature"),
	})
	if err == nil {
		t.Fatal("expected error for deleted branch")
	}
}

func TestAmplify_AppNotFound(t *testing.T) {
	client := newAmplifyClient(t)
	ctx := t.Context()

	_, err := client.GetApp(ctx, &amplify.GetAppInput{
		AppId: aws.String("nonexistent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent app")
	}
}
