//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
)

func executeAPIV2Client(t *testing.T) *apigatewayv2.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	return apigatewayv2.NewFromConfig(cfg, func(o *apigatewayv2.Options) {
		o.BaseEndpoint = aws.String(kumoEndpoint + "/apigatewayv2")
	})
}

// TestExecuteAPIV2_LambdaProxy proves an HTTP API (v2) route backed by an
// AWS_PROXY Lambda is invoked through the deployed stage URL, using the
// payload format 2.0 event.
func TestExecuteAPIV2_LambdaProxy(t *testing.T) {
	client := executeAPIV2Client(t)

	// Mock Lambda: returns a v2 proxy envelope and echoes rawPath so we can
	// assert the gateway built the 2.0 event.
	lambda := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event map[string]any
		_ = json.NewDecoder(r.Body).Decode(&event)

		resp := map[string]any{
			"statusCode": 200,
			"body":       fmt.Sprintf("v2 hello: rawPath=%v version=%v", event["rawPath"], event["version"]),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(lambda.Close)

	fn := "executeapi-v2-fn"
	createLambdaWithEndpoint(t, fn, lambda.URL)

	api, err := client.CreateApi(t.Context(), &apigatewayv2.CreateApiInput{
		Name:         aws.String("executeapi-v2"),
		ProtocolType: types.ProtocolTypeHttp,
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApi(t.Context(), &apigatewayv2.DeleteApiInput{ApiId: api.ApiId})
	})

	integ, err := client.CreateIntegration(t.Context(), &apigatewayv2.CreateIntegrationInput{
		ApiId:                api.ApiId,
		IntegrationType:      types.IntegrationTypeAwsProxy,
		IntegrationUri:       aws.String("arn:aws:lambda:us-east-1:000000000000:function:" + fn),
		PayloadFormatVersion: aws.String("2.0"),
	})
	if err != nil {
		t.Fatalf("CreateIntegration: %v", err)
	}

	if _, err := client.CreateRoute(t.Context(), &apigatewayv2.CreateRouteInput{
		ApiId:    api.ApiId,
		RouteKey: aws.String("GET /items"),
		Target:   aws.String("integrations/" + *integ.IntegrationId),
	}); err != nil {
		t.Fatalf("CreateRoute: %v", err)
	}

	if _, err := client.CreateStage(t.Context(), &apigatewayv2.CreateStageInput{
		ApiId:      api.ApiId,
		StageName:  aws.String("dev"),
		AutoDeploy: aws.Bool(true),
	}); err != nil {
		t.Fatalf("CreateStage: %v", err)
	}

	status, body := callStage(t, http.MethodGet, *api.ApiId, "dev", "/items")

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", status, body)
	}

	if want := "v2 hello: rawPath=/dev/items version=2.0"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

// TestExecuteAPIV2_UnknownRoute asserts an unmatched route on a real API
// returns 404 (and a known API id is owned by the v2 service).
func TestExecuteAPIV2_UnknownRoute(t *testing.T) {
	client := executeAPIV2Client(t)

	api, err := client.CreateApi(t.Context(), &apigatewayv2.CreateApiInput{
		Name:         aws.String("executeapi-v2-unknown"),
		ProtocolType: types.ProtocolTypeHttp,
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApi(t.Context(), &apigatewayv2.DeleteApiInput{ApiId: api.ApiId})
	})

	if _, err := client.CreateStage(t.Context(), &apigatewayv2.CreateStageInput{
		ApiId:     api.ApiId,
		StageName: aws.String("dev"),
	}); err != nil {
		t.Fatalf("CreateStage: %v", err)
	}

	status, _ := callStage(t, http.MethodGet, *api.ApiId, "dev", "/missing")
	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}
}
