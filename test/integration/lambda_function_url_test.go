//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

// functionURLGoldenIgnores hides the fields that differ between runs.
var functionURLGoldenIgnores = golden.WithIgnoreFields("ResultMetadata", "FunctionUrl", "CreationTime", "LastModifiedTime")

// createURLTestFunction creates a function for the function URL tests and
// deletes it (and, through the cascade, its URL) on cleanup.
func createURLTestFunction(t *testing.T, client *lambda.Client, name string) {
	t.Helper()

	if _, err := client.CreateFunction(t.Context(), &lambda.CreateFunctionInput{
		FunctionName: aws.String(name),
		Runtime:      types.RuntimeProvidedal2,
		Role:         aws.String("arn:aws:iam::000000000000:role/test-role"),
		Handler:      aws.String("index.handler"),
		Code:         &types.FunctionCode{ZipFile: []byte("fake-zip-content")},
	}); err != nil {
		t.Fatalf("CreateFunction: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteFunction(context.Background(), &lambda.DeleteFunctionInput{FunctionName: aws.String(name)})
	})
}

func assertLambdaAPIError(t *testing.T, err error, wantCode string, wantStatus int) {
	t.Helper()

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != wantCode {
		t.Fatalf("expected %s, got %v", wantCode, err)
	}

	var respErr *awshttp.ResponseError
	if !errors.As(err, &respErr) || respErr.HTTPStatusCode() != wantStatus {
		t.Fatalf("expected HTTP %d, got %v", wantStatus, err)
	}
}

func TestLambda_FunctionURLConfigLifecycle(t *testing.T) {
	client := newLambdaClient(t)
	ctx := t.Context()
	name := "test-function-url-lifecycle"
	createURLTestFunction(t, client, name)

	created, err := client.CreateFunctionUrlConfig(ctx, &lambda.CreateFunctionUrlConfigInput{
		FunctionName: aws.String(name),
		AuthType:     types.FunctionUrlAuthTypeNone,
		Cors: &types.Cors{
			AllowOrigins:  []string{"https://example.com"},
			AllowMethods:  []string{"GET", "POST"},
			AllowHeaders:  []string{"content-type"},
			ExposeHeaders: []string{"x-request-id"},
			MaxAge:        aws.Int32(3600),
		},
	})
	if err != nil {
		t.Fatalf("CreateFunctionUrlConfig: %v", err)
	}

	golden.New(t, functionURLGoldenIgnores).Assert(t.Name()+"_create", created)

	url := aws.ToString(created.FunctionUrl)
	if !strings.HasPrefix(url, "https://") || !strings.Contains(url, ".lambda-url.us-east-1.on.aws/") {
		t.Errorf("FunctionUrl = %q", url)
	}

	got, err := client.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{FunctionName: aws.String(name)})
	if err != nil {
		t.Fatalf("GetFunctionUrlConfig: %v", err)
	}

	golden.New(t, functionURLGoldenIgnores).Assert(t.Name()+"_get", got)

	if aws.ToString(got.FunctionUrl) != url {
		t.Errorf("Get returned a different URL: %q vs %q", aws.ToString(got.FunctionUrl), url)
	}

	updated, err := client.UpdateFunctionUrlConfig(ctx, &lambda.UpdateFunctionUrlConfigInput{
		FunctionName: aws.String(name),
		AuthType:     types.FunctionUrlAuthTypeAwsIam,
	})
	if err != nil {
		t.Fatalf("UpdateFunctionUrlConfig: %v", err)
	}

	golden.New(t, functionURLGoldenIgnores).Assert(t.Name()+"_update", updated)

	// Terraform removes a cors block by sending an empty Cors object.
	cleared, err := client.UpdateFunctionUrlConfig(ctx, &lambda.UpdateFunctionUrlConfigInput{
		FunctionName: aws.String(name),
		Cors:         &types.Cors{},
	})
	if err != nil {
		t.Fatalf("UpdateFunctionUrlConfig(clear cors): %v", err)
	}

	if cleared.Cors != nil {
		t.Errorf("an empty Cors object must clear the configuration, got %+v", cleared.Cors)
	}

	listed, err := client.ListFunctionUrlConfigs(ctx, &lambda.ListFunctionUrlConfigsInput{FunctionName: aws.String(name)})
	if err != nil {
		t.Fatalf("ListFunctionUrlConfigs: %v", err)
	}

	golden.New(t, functionURLGoldenIgnores).Assert(t.Name()+"_list", listed)

	if _, err := client.DeleteFunctionUrlConfig(ctx, &lambda.DeleteFunctionUrlConfigInput{FunctionName: aws.String(name)}); err != nil {
		t.Fatalf("DeleteFunctionUrlConfig: %v", err)
	}

	_, err = client.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{FunctionName: aws.String(name)})
	assertLambdaAPIError(t, err, "ResourceNotFoundException", http.StatusNotFound)
}

func TestLambda_FunctionURLConfigConflictsAndNotFound(t *testing.T) {
	client := newLambdaClient(t)
	ctx := t.Context()
	name := "test-function-url-conflict"
	createURLTestFunction(t, client, name)

	create := &lambda.CreateFunctionUrlConfigInput{FunctionName: aws.String(name), AuthType: types.FunctionUrlAuthTypeNone}

	if _, err := client.CreateFunctionUrlConfig(ctx, create); err != nil {
		t.Fatalf("CreateFunctionUrlConfig: %v", err)
	}

	_, err := client.CreateFunctionUrlConfig(ctx, create)
	assertLambdaAPIError(t, err, "ResourceConflictException", http.StatusConflict)

	_, err = client.GetFunctionUrlConfig(ctx, &lambda.GetFunctionUrlConfigInput{FunctionName: aws.String("test-function-url-missing")})
	assertLambdaAPIError(t, err, "ResourceNotFoundException", http.StatusNotFound)

	// kumo has no aliases, so a qualified target never exists.
	_, err = client.CreateFunctionUrlConfig(ctx, &lambda.CreateFunctionUrlConfigInput{
		FunctionName: aws.String(name),
		Qualifier:    aws.String("prod"),
		AuthType:     types.FunctionUrlAuthTypeNone,
	})
	assertLambdaAPIError(t, err, "ResourceNotFoundException", http.StatusNotFound)

	_, err = client.ListFunctionUrlConfigs(ctx, &lambda.ListFunctionUrlConfigsInput{FunctionName: aws.String(name), MaxItems: aws.Int32(0)})
	assertLambdaAPIError(t, err, "InvalidParameterValueException", http.StatusBadRequest)
}

// TestLambda_FunctionURLConfigRejectsMalformedBodies exercises both route
// prefixes (SDK BaseEndpoint style and CLI style) with raw HTTP.
func TestLambda_FunctionURLConfigRejectsMalformedBodies(t *testing.T) {
	client := newLambdaClient(t)
	name := "test-function-url-malformed"
	createURLTestFunction(t, client, name)

	for _, prefix := range []string{"", "/lambda"} {
		for _, body := range []string{`{not json`, `{}`, `{"AuthType":"MAYBE"}`} {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
				testEndpoint()+prefix+"/2021-10-31/functions/"+name+"/url", bytes.NewReader([]byte(body)))
			if err != nil {
				t.Fatalf("build request: %v", err)
			}

			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("POST %s: %v", prefix, err)
			}

			payload, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest || resp.Header.Get("X-Amzn-Errortype") != "InvalidParameterValueException" {
				t.Errorf("prefix %q body %s: got %d %s %s", prefix, body, resp.StatusCode, resp.Header.Get("X-Amzn-Errortype"), payload)
			}
		}
	}
}

func TestLambda_FunctionURLPermissionConditionRoundTrips(t *testing.T) {
	client := newLambdaClient(t)
	ctx := t.Context()
	name := "test-function-url-permission"
	createURLTestFunction(t, client, name)

	if _, err := client.AddPermission(ctx, &lambda.AddPermissionInput{
		FunctionName:        aws.String(name),
		StatementId:         aws.String("AllowPublicFunctionUrl"),
		Action:              aws.String("lambda:InvokeFunctionUrl"),
		Principal:           aws.String("*"),
		FunctionUrlAuthType: types.FunctionUrlAuthTypeNone,
	}); err != nil {
		t.Fatalf("AddPermission: %v", err)
	}

	policy, err := client.GetPolicy(ctx, &lambda.GetPolicyInput{FunctionName: aws.String(name)})
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}

	if !strings.Contains(aws.ToString(policy.Policy), `"lambda:FunctionUrlAuthType":"NONE"`) {
		t.Fatalf("policy must carry the FunctionUrlAuthType condition: %s", aws.ToString(policy.Policy))
	}

	_, err = client.AddPermission(ctx, &lambda.AddPermissionInput{
		FunctionName:        aws.String(name),
		StatementId:         aws.String("BadAuthType"),
		Action:              aws.String("lambda:InvokeFunctionUrl"),
		Principal:           aws.String("*"),
		FunctionUrlAuthType: types.FunctionUrlAuthType("MAYBE"),
	})
	assertLambdaAPIError(t, err, "InvalidParameterValueException", http.StatusBadRequest)
}
