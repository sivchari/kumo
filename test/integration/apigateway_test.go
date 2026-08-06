//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/sivchari/golden"
)

func newAPIGatewayClient(t *testing.T) *apigateway.Client {
	t.Helper()

	return apigateway.NewFromConfig(awsConfig(t), func(o *apigateway.Options) {
		o.BaseEndpoint = aws.String(testEndpoint() + "/apigateway")
	})
}

func TestAPIGateway_CreateAndGetRestApi(t *testing.T) {
	client := newAPIGatewayClient(t)
	ctx := t.Context()

	apiName := "test-rest-api"

	// Create REST API.
	createOutput, err := client.CreateRestApi(ctx, &apigateway.CreateRestApiInput{
		Name:        aws.String(apiName),
		Description: aws.String("Test REST API"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "CreatedDate", "RootResourceId", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Get REST API.
	getOutput, err := client.GetRestApi(ctx, &apigateway.GetRestApiInput{
		RestApiId: createOutput.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "CreatedDate", "RootResourceId", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestAPIGateway_GetRestApis(t *testing.T) {
	client := newAPIGatewayClient(t)
	ctx := t.Context()

	// Create a REST API first.
	createOutput, err := client.CreateRestApi(ctx, &apigateway.CreateRestApiInput{
		Name: aws.String("test-list-api"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get REST APIs.
	listOutput, err := client.GetRestApis(ctx, &apigateway.GetRestApisInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, api := range listOutput.Items {
		if api.Id != nil && *api.Id == *createOutput.Id {
			found = true

			break
		}
	}

	if !found {
		t.Error("created REST API not found in list")
	}
}

func TestAPIGateway_CreateAndGetResource(t *testing.T) {
	client := newAPIGatewayClient(t)
	ctx := t.Context()

	// Create REST API.
	apiOutput, err := client.CreateRestApi(ctx, &apigateway.CreateRestApiInput{
		Name: aws.String("test-resource-api"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get resources to find root resource.
	resourcesOutput, err := client.GetResources(ctx, &apigateway.GetResourcesInput{
		RestApiId: apiOutput.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	var rootResourceID string

	for _, res := range resourcesOutput.Items {
		if res.Path != nil && *res.Path == "/" {
			rootResourceID = *res.Id

			break
		}
	}

	if rootResourceID == "" {
		t.Fatal("root resource not found")
	}

	// Create resource.
	createOutput, err := client.CreateResource(ctx, &apigateway.CreateResourceInput{
		RestApiId: apiOutput.Id,
		ParentId:  aws.String(rootResourceID),
		PathPart:  aws.String("users"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "ParentId", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Get resource.
	getOutput, err := client.GetResource(ctx, &apigateway.GetResourceInput{
		RestApiId:  apiOutput.Id,
		ResourceId: createOutput.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "ParentId", "ResultMetadata")).Assert(t.Name()+"_get", getOutput)
}

func TestAPIGateway_PutMethodAndIntegration(t *testing.T) {
	client := newAPIGatewayClient(t)
	ctx := t.Context()

	// Create REST API.
	apiOutput, err := client.CreateRestApi(ctx, &apigateway.CreateRestApiInput{
		Name: aws.String("test-method-api"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get root resource.
	resourcesOutput, err := client.GetResources(ctx, &apigateway.GetResourcesInput{
		RestApiId: apiOutput.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	var rootResourceID string

	for _, res := range resourcesOutput.Items {
		if res.Path != nil && *res.Path == "/" {
			rootResourceID = *res.Id

			break
		}
	}

	// Put method.
	methodOutput, err := client.PutMethod(ctx, &apigateway.PutMethodInput{
		RestApiId:         apiOutput.Id,
		ResourceId:        aws.String(rootResourceID),
		HttpMethod:        aws.String("GET"),
		AuthorizationType: aws.String("NONE"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_method", methodOutput)

	// Put integration.
	integrationOutput, err := client.PutIntegration(ctx, &apigateway.PutIntegrationInput{
		RestApiId:  apiOutput.Id,
		ResourceId: aws.String(rootResourceID),
		HttpMethod: aws.String("GET"),
		Type:       types.IntegrationTypeMock,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_integration", integrationOutput)
}

func TestAPIGateway_CreateDeploymentAndStage(t *testing.T) {
	client := newAPIGatewayClient(t)
	ctx := t.Context()

	// Create REST API.
	apiOutput, err := client.CreateRestApi(ctx, &apigateway.CreateRestApiInput{
		Name: aws.String("test-deployment-api"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create deployment.
	deploymentOutput, err := client.CreateDeployment(ctx, &apigateway.CreateDeploymentInput{
		RestApiId:   apiOutput.Id,
		Description: aws.String("Test deployment"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "CreatedDate", "ResultMetadata")).Assert(t.Name()+"_deployment", deploymentOutput)

	// Create stage.
	stageOutput, err := client.CreateStage(ctx, &apigateway.CreateStageInput{
		RestApiId:    apiOutput.Id,
		StageName:    aws.String("prod"),
		DeploymentId: deploymentOutput.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("DeploymentId", "CreatedDate", "LastUpdatedDate", "ResultMetadata")).Assert(t.Name()+"_stage", stageOutput)

	// Get stage.
	getStageOutput, err := client.GetStage(ctx, &apigateway.GetStageInput{
		RestApiId: apiOutput.Id,
		StageName: aws.String("prod"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("DeploymentId", "CreatedDate", "LastUpdatedDate", "ResultMetadata")).Assert(t.Name()+"_get_stage", getStageOutput)
}

func TestAPIGateway_DeleteRestApi(t *testing.T) {
	client := newAPIGatewayClient(t)
	ctx := t.Context()

	// Create REST API.
	createOutput, err := client.CreateRestApi(ctx, &apigateway.CreateRestApiInput{
		Name: aws.String("test-delete-api"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete REST API.
	_, err = client.DeleteRestApi(ctx, &apigateway.DeleteRestApiInput{
		RestApiId: createOutput.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.GetRestApi(ctx, &apigateway.GetRestApiInput{
		RestApiId: createOutput.Id,
	})
	if err == nil {
		t.Error("expected error for deleted REST API")
	}
}

func TestAPIGateway_RestApiNotFound(t *testing.T) {
	client := newAPIGatewayClient(t)
	ctx := t.Context()

	// Try to get non-existent REST API.
	_, err := client.GetRestApi(ctx, &apigateway.GetRestApiInput{
		RestApiId: aws.String("nonexistent-api"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent REST API")
	}
}
