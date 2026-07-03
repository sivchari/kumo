//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/sivchari/golden"
)

func newAPIGatewayV2Client(t *testing.T) *apigatewayv2.Client {
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

	return apigatewayv2.NewFromConfig(cfg, func(o *apigatewayv2.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566/apigatewayv2")
	})
}

func TestAPIGatewayV2_CreateAndGetApi(t *testing.T) {
	client := newAPIGatewayV2Client(t)
	ctx := t.Context()

	createOutput, err := client.CreateApi(ctx, &apigatewayv2.CreateApiInput{
		Name:         aws.String("test-http-api"),
		ProtocolType: types.ProtocolTypeHttp,
		Description:  aws.String("Test HTTP API"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ApiId", "ApiEndpoint", "CreatedDate", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	getOutput, err := client.GetApi(ctx, &apigatewayv2.GetApiInput{
		ApiId: createOutput.ApiId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ApiId", "ApiEndpoint", "CreatedDate", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestAPIGatewayV2_GetApis(t *testing.T) {
	client := newAPIGatewayV2Client(t)
	ctx := t.Context()

	createOutput, err := client.CreateApi(ctx, &apigatewayv2.CreateApiInput{
		Name:         aws.String("test-list-http-api"),
		ProtocolType: types.ProtocolTypeHttp,
	})
	if err != nil {
		t.Fatal(err)
	}

	listOutput, err := client.GetApis(ctx, &apigatewayv2.GetApisInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, api := range listOutput.Items {
		if api.ApiId != nil && *api.ApiId == *createOutput.ApiId {
			found = true

			break
		}
	}

	if !found {
		t.Error("created API not found in list")
	}
}

func TestAPIGatewayV2_UpdateApi(t *testing.T) {
	client := newAPIGatewayV2Client(t)
	ctx := t.Context()

	createOutput, err := client.CreateApi(ctx, &apigatewayv2.CreateApiInput{
		Name:         aws.String("test-update-http-api"),
		ProtocolType: types.ProtocolTypeHttp,
	})
	if err != nil {
		t.Fatal(err)
	}

	updateOutput, err := client.UpdateApi(ctx, &apigatewayv2.UpdateApiInput{
		ApiId:       createOutput.ApiId,
		Description: aws.String("updated description"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ApiId", "ApiEndpoint", "CreatedDate", "ResultMetadata")).Assert(t.Name()+"_update", updateOutput)
}

func TestAPIGatewayV2_CreateRoute(t *testing.T) {
	client := newAPIGatewayV2Client(t)
	ctx := t.Context()

	apiOutput, err := client.CreateApi(ctx, &apigatewayv2.CreateApiInput{
		Name:         aws.String("test-route-api"),
		ProtocolType: types.ProtocolTypeHttp,
	})
	if err != nil {
		t.Fatal(err)
	}

	createOutput, err := client.CreateRoute(ctx, &apigatewayv2.CreateRouteInput{
		ApiId:    apiOutput.ApiId,
		RouteKey: aws.String("GET /pets"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("RouteId", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	getOutput, err := client.GetRoute(ctx, &apigatewayv2.GetRouteInput{
		ApiId:   apiOutput.ApiId,
		RouteId: createOutput.RouteId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("RouteId", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestAPIGatewayV2_CreateIntegration(t *testing.T) {
	client := newAPIGatewayV2Client(t)
	ctx := t.Context()

	apiOutput, err := client.CreateApi(ctx, &apigatewayv2.CreateApiInput{
		Name:         aws.String("test-integration-api"),
		ProtocolType: types.ProtocolTypeHttp,
	})
	if err != nil {
		t.Fatal(err)
	}

	createOutput, err := client.CreateIntegration(ctx, &apigatewayv2.CreateIntegrationInput{
		ApiId:                apiOutput.ApiId,
		IntegrationType:      types.IntegrationTypeHttpProxy,
		IntegrationMethod:    aws.String("GET"),
		IntegrationUri:       aws.String("https://example.com"),
		PayloadFormatVersion: aws.String("1.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("IntegrationId", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	getOutput, err := client.GetIntegration(ctx, &apigatewayv2.GetIntegrationInput{
		ApiId:         apiOutput.ApiId,
		IntegrationId: createOutput.IntegrationId,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("IntegrationId", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestAPIGatewayV2_CreateStageAndDeployment(t *testing.T) {
	client := newAPIGatewayV2Client(t)
	ctx := t.Context()

	apiOutput, err := client.CreateApi(ctx, &apigatewayv2.CreateApiInput{
		Name:         aws.String("test-stage-api"),
		ProtocolType: types.ProtocolTypeHttp,
	})
	if err != nil {
		t.Fatal(err)
	}

	stageOutput, err := client.CreateStage(ctx, &apigatewayv2.CreateStageInput{
		ApiId:      apiOutput.ApiId,
		StageName:  aws.String("dev"),
		AutoDeploy: aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("CreatedDate", "LastUpdatedDate", "ResultMetadata")).Assert(t.Name()+"_stage", stageOutput)

	deploymentOutput, err := client.CreateDeployment(ctx, &apigatewayv2.CreateDeploymentInput{
		ApiId:       apiOutput.ApiId,
		StageName:   aws.String("dev"),
		Description: aws.String("first deployment"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("DeploymentId", "CreatedDate", "ResultMetadata")).Assert(t.Name()+"_deployment", deploymentOutput)
}

func TestAPIGatewayV2_Tags(t *testing.T) {
	client := newAPIGatewayV2Client(t)
	ctx := t.Context()

	apiOutput, err := client.CreateApi(ctx, &apigatewayv2.CreateApiInput{
		Name:         aws.String("test-tags-api"),
		ProtocolType: types.ProtocolTypeHttp,
		Tags: map[string]string{
			"env": "test",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	getOutput, err := client.GetTags(ctx, &apigatewayv2.GetTagsInput{
		ResourceArn: aws.String("arn:aws:apigateway:us-east-1::/apis/" + *apiOutput.ApiId),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestAPIGatewayV2_DeleteApi(t *testing.T) {
	client := newAPIGatewayV2Client(t)
	ctx := t.Context()

	createOutput, err := client.CreateApi(ctx, &apigatewayv2.CreateApiInput{
		Name:         aws.String("test-delete-http-api"),
		ProtocolType: types.ProtocolTypeHttp,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteApi(ctx, &apigatewayv2.DeleteApiInput{
		ApiId: createOutput.ApiId,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetApi(ctx, &apigatewayv2.GetApiInput{
		ApiId: createOutput.ApiId,
	})
	if err == nil {
		t.Error("expected error for deleted API")
	}
}

func TestAPIGatewayV2_ApiNotFound(t *testing.T) {
	client := newAPIGatewayV2Client(t)
	ctx := t.Context()

	_, err := client.GetApi(ctx, &apigatewayv2.GetApiInput{
		ApiId: aws.String("nonexistent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent API")
	}
}
