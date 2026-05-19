//go:build integration

package integration

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/sivchari/golden"
)

const minimalFunctionSource = `function handler(event) { return event.request; }`

func encodedFunctionCode() []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(minimalFunctionSource)))
}

func functionConfig() *types.FunctionConfig {
	return &types.FunctionConfig{
		Comment: aws.String("kumo integration test"),
		Runtime: types.FunctionRuntimeCloudfrontJs20,
	}
}

func cleanupFunction(t *testing.T, client *cloudfront.Client, name string) {
	t.Helper()

	describe, err := client.DescribeFunction(context.Background(), &cloudfront.DescribeFunctionInput{
		Name: aws.String(name),
	})
	if err != nil {
		return
	}

	_, _ = client.DeleteFunction(context.Background(), &cloudfront.DeleteFunctionInput{
		Name:    aws.String(name),
		IfMatch: describe.ETag,
	})
}

func TestCloudFrontFunctions_CreateFunction(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := "kumo-create-fn"

	t.Cleanup(func() { cleanupFunction(t, client, name) })

	result, err := client.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
		Name:           aws.String(name),
		FunctionCode:   encodedFunctionCode(),
		FunctionConfig: functionConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ETag",
		"Location",
		"FunctionMetadata",
		"ResultMetadata",
	)).Assert(t.Name(), result)
}

func TestCloudFrontFunctions_DescribeFunction(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := "kumo-describe-fn"

	t.Cleanup(func() { cleanupFunction(t, client, name) })

	if _, err := client.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
		Name:           aws.String(name),
		FunctionCode:   encodedFunctionCode(),
		FunctionConfig: functionConfig(),
	}); err != nil {
		t.Fatal(err)
	}

	described, err := client.DescribeFunction(ctx, &cloudfront.DescribeFunctionInput{
		Name: aws.String(name),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ETag",
		"FunctionMetadata",
		"ResultMetadata",
	)).Assert(t.Name(), described)
}

func TestCloudFrontFunctions_GetFunction(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := "kumo-get-fn"

	t.Cleanup(func() { cleanupFunction(t, client, name) })

	if _, err := client.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
		Name:           aws.String(name),
		FunctionCode:   encodedFunctionCode(),
		FunctionConfig: functionConfig(),
	}); err != nil {
		t.Fatal(err)
	}

	got, err := client.GetFunction(ctx, &cloudfront.GetFunctionInput{
		Name: aws.String(name),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ETag",
		"ResultMetadata",
	)).Assert(t.Name(), got)
}

func TestCloudFrontFunctions_ListFunctions(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := "kumo-list-fn"

	t.Cleanup(func() { cleanupFunction(t, client, name) })

	if _, err := client.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
		Name:           aws.String(name),
		FunctionCode:   encodedFunctionCode(),
		FunctionConfig: functionConfig(),
	}); err != nil {
		t.Fatal(err)
	}

	listed, err := client.ListFunctions(ctx, &cloudfront.ListFunctionsInput{})
	if err != nil {
		t.Fatal(err)
	}

	if listed.FunctionList == nil || len(listed.FunctionList.Items) == 0 {
		t.Fatalf("ListFunctions returned no items")
	}
}

func TestCloudFrontFunctions_UpdateFunction(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := "kumo-update-fn"

	t.Cleanup(func() { cleanupFunction(t, client, name) })

	created, err := client.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
		Name:           aws.String(name),
		FunctionCode:   encodedFunctionCode(),
		FunctionConfig: functionConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}

	updatedConfig := functionConfig()
	updatedConfig.Comment = aws.String("kumo integration test (updated)")

	result, err := client.UpdateFunction(ctx, &cloudfront.UpdateFunctionInput{
		Name:           aws.String(name),
		IfMatch:        created.ETag,
		FunctionCode:   encodedFunctionCode(),
		FunctionConfig: updatedConfig,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"ETag",
		"FunctionMetadata",
		"ResultMetadata",
	)).Assert(t.Name(), result)
}

func TestCloudFrontFunctions_PublishFunction(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := "kumo-publish-fn"

	t.Cleanup(func() { cleanupFunction(t, client, name) })

	created, err := client.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
		Name:           aws.String(name),
		FunctionCode:   encodedFunctionCode(),
		FunctionConfig: functionConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.PublishFunction(ctx, &cloudfront.PublishFunctionInput{
		Name:    aws.String(name),
		IfMatch: created.ETag,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"FunctionSummary",
		"ResultMetadata",
	)).Assert(t.Name(), result)
}

func TestCloudFrontFunctions_TestFunction(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := "kumo-test-fn"

	t.Cleanup(func() { cleanupFunction(t, client, name) })

	created, err := client.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
		Name:           aws.String(name),
		FunctionCode:   encodedFunctionCode(),
		FunctionConfig: functionConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}

	eventObject := base64.StdEncoding.EncodeToString([]byte(`{"version":"1.0","context":{"distributionId":"E1"},"viewer":{"ip":"1.2.3.4"},"request":{"uri":"/index.html","method":"GET","headers":{},"cookies":{},"querystring":{}}}`))

	result, err := client.TestFunction(ctx, &cloudfront.TestFunctionInput{
		Name:        aws.String(name),
		IfMatch:     created.ETag,
		EventObject: []byte(eventObject),
		Stage:       types.FunctionStageDevelopment,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.TestResult == nil || result.TestResult.ComputeUtilization == nil {
		t.Fatalf("TestFunction did not return TestResult.ComputeUtilization: %#v", result.TestResult)
	}
}

func TestCloudFrontFunctions_DeleteFunction(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := "kumo-delete-fn"

	created, err := client.CreateFunction(ctx, &cloudfront.CreateFunctionInput{
		Name:           aws.String(name),
		FunctionCode:   encodedFunctionCode(),
		FunctionConfig: functionConfig(),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.DeleteFunction(ctx, &cloudfront.DeleteFunctionInput{
		Name:    aws.String(name),
		IfMatch: created.ETag,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.DescribeFunction(ctx, &cloudfront.DescribeFunctionInput{
		Name: aws.String(name),
	}); err == nil {
		t.Fatalf("expected DescribeFunction to fail after delete")
	}
}
