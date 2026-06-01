//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigateway/types"
)

const kumoEndpoint = "http://localhost:4566"

// executeAPIClient builds an API Gateway client targeting kumoEndpoint, so
// resources are created on the same server the stage URL is invoked against.
func executeAPIClient(t *testing.T) *apigateway.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	return apigateway.NewFromConfig(cfg, func(o *apigateway.Options) {
		o.BaseEndpoint = aws.String(kumoEndpoint + "/apigateway")
	})
}

// rootResourceID returns the id of the "/" resource of a REST API.
func rootResourceID(t *testing.T, client *apigateway.Client, apiID *string) string {
	t.Helper()

	out, err := client.GetResources(t.Context(), &apigateway.GetResourcesInput{RestApiId: apiID})
	if err != nil {
		t.Fatalf("GetResources: %v", err)
	}

	for _, r := range out.Items {
		if r.Path != nil && *r.Path == "/" {
			return *r.Id
		}
	}

	t.Fatal("root resource not found")

	return ""
}

// createLambdaWithEndpoint creates a Lambda function whose InvokeEndpoint is
// the given URL, using the raw HTTP API (InvokeEndpoint is a kumo extension
// the SDK cannot send).
func createLambdaWithEndpoint(t *testing.T, name, invokeEndpoint string) {
	t.Helper()

	body, _ := json.Marshal(map[string]any{
		"FunctionName":   name,
		"Runtime":        "provided.al2",
		"Role":           "arn:aws:iam::000000000000:role/test-role",
		"Handler":        "index.handler",
		"InvokeEndpoint": invokeEndpoint,
		"Code":           map[string]any{"ZipFile": []byte("fake")},
	})

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost,
		kumoEndpoint+"/lambda/2015-03-31/functions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create lambda: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create lambda status %d: %s", resp.StatusCode, b)
	}

	t.Cleanup(func() {
		delReq, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete,
			kumoEndpoint+"/lambda/2015-03-31/functions/"+name, nil)

		if dr, _ := http.DefaultClient.Do(delReq); dr != nil {
			_ = dr.Body.Close()
		}
	})
}

// buildLambdaAPI wires a REST API with a single GET /items route backed by an
// AWS_PROXY Lambda integration, deployed to the given stage. Returns the API id.
func buildLambdaAPI(t *testing.T, client *apigateway.Client, apiName, functionName, stage string) string {
	t.Helper()

	api, err := client.CreateRestApi(t.Context(), &apigateway.CreateRestApiInput{Name: aws.String(apiName)})
	if err != nil {
		t.Fatalf("CreateRestApi: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRestApi(context.Background(), &apigateway.DeleteRestApiInput{RestApiId: api.Id})
	})

	root := rootResourceID(t, client, api.Id)

	res, err := client.CreateResource(t.Context(), &apigateway.CreateResourceInput{
		RestApiId: api.Id,
		ParentId:  aws.String(root),
		PathPart:  aws.String("items"),
	})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}

	if _, err := client.PutMethod(t.Context(), &apigateway.PutMethodInput{
		RestApiId:         api.Id,
		ResourceId:        res.Id,
		HttpMethod:        aws.String("GET"),
		AuthorizationType: aws.String("NONE"),
	}); err != nil {
		t.Fatalf("PutMethod: %v", err)
	}

	uri := fmt.Sprintf(
		"arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:000000000000:function:%s/invocations",
		functionName,
	)

	if _, err := client.PutIntegration(t.Context(), &apigateway.PutIntegrationInput{
		RestApiId:             api.Id,
		ResourceId:            res.Id,
		HttpMethod:            aws.String("GET"),
		Type:                  types.IntegrationTypeAwsProxy,
		IntegrationHttpMethod: aws.String("POST"),
		Uri:                   aws.String(uri),
	}); err != nil {
		t.Fatalf("PutIntegration: %v", err)
	}

	if _, err := client.CreateDeployment(t.Context(), &apigateway.CreateDeploymentInput{
		RestApiId: api.Id,
		StageName: aws.String(stage),
	}); err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	return *api.Id
}

// callStage invokes a deployed stage via the virtual-hosted execute-api
// endpoint: it connects to the kumo server but sets the Host header to
// {apiId}.execute-api.localhost so the router dispatches it as execute-api
// (this is how a real client reaches the api_endpoint).
func callStage(t *testing.T, method, apiID, stage, path string) (int, string) {
	t.Helper()

	url := fmt.Sprintf("%s/%s%s", kumoEndpoint, stage, path)

	req, _ := http.NewRequestWithContext(t.Context(), method, url, nil)
	req.Host = apiID + ".execute-api.localhost"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("call stage: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, string(body)
}

// TestExecuteAPI_LambdaProxy proves a Lambda function created in kumo is
// actually invoked through the deployed stage URL and its proxy response is
// returned to the caller.
func TestExecuteAPI_LambdaProxy(t *testing.T) {
	client := executeAPIClient(t)

	// Mock Lambda handler: returns a proxy-integration envelope and echoes
	// the received event path so we can assert the gateway shaped it.
	lambda := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event map[string]any
		_ = json.NewDecoder(r.Body).Decode(&event)

		resp := map[string]any{
			"statusCode": 201,
			"headers":    map[string]string{"X-Handler": "kumo-test"},
			"body":       fmt.Sprintf("hello from lambda: path=%v", event["path"]),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(lambda.Close)

	fn := "executeapi-lambda-fn"
	createLambdaWithEndpoint(t, fn, lambda.URL)

	apiID := buildLambdaAPI(t, client, "executeapi-lambda", fn, "dev")

	status, body := callStage(t, http.MethodGet, apiID, "dev", "/items")

	if status != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%q", status, body)
	}

	if want := "hello from lambda: path=/items"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

// TestExecuteAPI_HTTPProxy proves an HTTP_PROXY integration forwards the
// request to the backend and returns its response.
func TestExecuteAPI_HTTPProxy(t *testing.T) {
	client := executeAPIClient(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("from http backend"))
	}))
	t.Cleanup(backend.Close)

	api, err := client.CreateRestApi(t.Context(), &apigateway.CreateRestApiInput{Name: aws.String("executeapi-http")})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRestApi(context.Background(), &apigateway.DeleteRestApiInput{RestApiId: api.Id})
	})

	root := rootResourceID(t, client, api.Id)

	res, err := client.CreateResource(t.Context(), &apigateway.CreateResourceInput{
		RestApiId: api.Id, ParentId: aws.String(root), PathPart: aws.String("ping"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.PutMethod(t.Context(), &apigateway.PutMethodInput{
		RestApiId: api.Id, ResourceId: res.Id, HttpMethod: aws.String("GET"), AuthorizationType: aws.String("NONE"),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.PutIntegration(t.Context(), &apigateway.PutIntegrationInput{
		RestApiId:             api.Id,
		ResourceId:            res.Id,
		HttpMethod:            aws.String("GET"),
		Type:                  types.IntegrationTypeHttpProxy,
		IntegrationHttpMethod: aws.String("GET"),
		Uri:                   aws.String(backend.URL),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.CreateDeployment(t.Context(), &apigateway.CreateDeploymentInput{
		RestApiId: api.Id, StageName: aws.String("dev"),
	}); err != nil {
		t.Fatal(err)
	}

	status, body := callStage(t, http.MethodGet, *api.Id, "dev", "/ping")
	if status != http.StatusOK || body != "from http backend" {
		t.Errorf("status=%d body=%q, want 200 'from http backend'", status, body)
	}
}

// TestExecuteAPI_NoInvokeEndpoint asserts that without a wired InvokeEndpoint
// the Lambda echoes the event (no statusCode), so the gateway returns 502 —
// matching a malformed Lambda proxy response on real AWS.
func TestExecuteAPI_NoInvokeEndpoint(t *testing.T) {
	client := executeAPIClient(t)

	fn := "executeapi-noendpoint-fn"
	createLambdaWithEndpoint(t, fn, "") // empty -> echo stub

	apiID := buildLambdaAPI(t, client, "executeapi-noendpoint", fn, "dev")

	status, _ := callStage(t, http.MethodGet, apiID, "dev", "/items")
	if status != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", status)
	}
}

// TestExecuteAPI_UnknownRoute asserts an unmatched path returns the real AWS
// 403 "Missing Authentication Token".
func TestExecuteAPI_UnknownRoute(t *testing.T) {
	client := executeAPIClient(t)

	fn := "executeapi-unknownroute-fn"
	createLambdaWithEndpoint(t, fn, "")

	apiID := buildLambdaAPI(t, client, "executeapi-unknownroute", fn, "dev")

	status, body := callStage(t, http.MethodGet, apiID, "dev", "/does-not-exist")
	if status != http.StatusForbidden {
		t.Errorf("status = %d, want 403; body=%q", status, body)
	}
}
