//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/sivchari/golden"
)

// These tests capture the Lambda proxy event execute-api builds, proving the
// stage-path construction ($default vs a named stage) and the stageVariables
// shape (v1 always encodes the field, null when unset; v2 omits it when
// unset) end to end. They deliberately echo back only this small, static
// subset of the event rather than the whole payload: the full event carries
// dynamic fields (requestId, sourceIp, the ephemeral client port in Host)
// that would make a byte-for-byte golden comparison flaky.

// writeProxyEnvelope wraps a captured event subset in the Lambda proxy
// response envelope execute-api expects back.
func writeProxyEnvelope(w http.ResponseWriter, body []byte) {
	resp := map[string]any{
		"statusCode": 200,
		"headers":    map[string]string{"Content-Type": "application/json"},
		"body":       string(body),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// captureStageVariables reports whether raw's top-level "stageVariables" key
// is present, and its raw JSON value when it is.
func captureStageVariables(raw map[string]json.RawMessage) (present bool, value json.RawMessage) {
	v, ok := raw["stageVariables"]

	return ok, v
}

// mockLambdaEchoV2Shape returns a mock Lambda that captures the
// payload-2.0 event's rawPath, requestContext.stage,
// requestContext.http.path, and stageVariables presence/value.
func mockLambdaEchoV2Shape() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&raw)

		var reqCtx map[string]json.RawMessage
		_ = json.Unmarshal(raw["requestContext"], &reqCtx)

		var httpCtx map[string]json.RawMessage
		_ = json.Unmarshal(reqCtx["http"], &httpCtx)

		present, value := captureStageVariables(raw)

		captured := map[string]any{
			"rawPath":                json.RawMessage(raw["rawPath"]),
			"requestContextStage":    json.RawMessage(reqCtx["stage"]),
			"requestContextHTTPPath": json.RawMessage(httpCtx["path"]),
			"stageVariablesPresent":  present,
		}
		if present {
			captured["stageVariables"] = value
		}

		body, _ := json.Marshal(captured)
		writeProxyEnvelope(w, body)
	}))
}

// mockLambdaEchoV1Shape returns a mock Lambda that captures the
// payload-1.0 event's requestContext.path, requestContext.stage, and
// stageVariables presence/value.
func mockLambdaEchoV1Shape() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&raw)

		var reqCtx map[string]json.RawMessage
		_ = json.Unmarshal(raw["requestContext"], &reqCtx)

		present, value := captureStageVariables(raw)

		captured := map[string]any{
			"requestContextPath":    json.RawMessage(reqCtx["path"]),
			"requestContextStage":   json.RawMessage(reqCtx["stage"]),
			"stageVariablesPresent": present,
		}
		if present {
			captured["stageVariables"] = value
		}

		body, _ := json.Marshal(captured)
		writeProxyEnvelope(w, body)
	}))
}

// TestExecuteAPIV2_EventShape_PayloadV2_DefaultStage proves a $default stage
// omits the stage segment from rawPath and requestContext.http.path, and
// configured stage variables are included in the payload-2.0 event.
func TestExecuteAPIV2_EventShape_PayloadV2_DefaultStage(t *testing.T) {
	client := executeAPIV2Client(t)

	lambda := mockLambdaEchoV2Shape()
	t.Cleanup(lambda.Close)

	fn := "executeapi-eventshape-v2-default-fn"
	createLambdaWithEndpoint(t, fn, lambda.URL)

	api, err := client.CreateApi(t.Context(), &apigatewayv2.CreateApiInput{
		Name:         aws.String("executeapi-eventshape-v2-default"),
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
		ApiId:          api.ApiId,
		StageName:      aws.String("$default"),
		AutoDeploy:     aws.Bool(true),
		StageVariables: map[string]string{"env": "dev"},
	}); err != nil {
		t.Fatalf("CreateStage: %v", err)
	}

	status, body := callDefaultStage(t, http.MethodGet, *api.ApiId, "/items")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", status, body)
	}

	golden.New(t).Assert(t.Name(), body)
}

// TestExecuteAPIV2_EventShape_PayloadV2_NamedStage proves a named stage is
// retained as a path segment in rawPath and requestContext.http.path, and
// unset stage variables are omitted from the payload-2.0 event.
func TestExecuteAPIV2_EventShape_PayloadV2_NamedStage(t *testing.T) {
	client := executeAPIV2Client(t)

	lambda := mockLambdaEchoV2Shape()
	t.Cleanup(lambda.Close)

	fn := "executeapi-eventshape-v2-named-fn"
	createLambdaWithEndpoint(t, fn, lambda.URL)

	api, err := client.CreateApi(t.Context(), &apigatewayv2.CreateApiInput{
		Name:         aws.String("executeapi-eventshape-v2-named"),
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
		StageName:  aws.String("prod"),
		AutoDeploy: aws.Bool(true),
	}); err != nil {
		t.Fatalf("CreateStage: %v", err)
	}

	status, body := callStage(t, http.MethodGet, *api.ApiId, "prod", "/items")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", status, body)
	}

	golden.New(t).Assert(t.Name(), body)
}

// TestExecuteAPIV2_EventShape_PayloadV1_DefaultStage proves a $default stage
// omits the stage segment from requestContext.path, and unset stage
// variables are still present as a null field in the payload-1.0 event.
func TestExecuteAPIV2_EventShape_PayloadV1_DefaultStage(t *testing.T) {
	client := executeAPIV2Client(t)

	lambda := mockLambdaEchoV1Shape()
	t.Cleanup(lambda.Close)

	fn := "executeapi-eventshape-v1-default-fn"
	createLambdaWithEndpoint(t, fn, lambda.URL)

	api, err := client.CreateApi(t.Context(), &apigatewayv2.CreateApiInput{
		Name:         aws.String("executeapi-eventshape-v1-default"),
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
		PayloadFormatVersion: aws.String("1.0"),
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
		StageName:  aws.String("$default"),
		AutoDeploy: aws.Bool(true),
	}); err != nil {
		t.Fatalf("CreateStage: %v", err)
	}

	status, body := callDefaultStage(t, http.MethodGet, *api.ApiId, "/items")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", status, body)
	}

	golden.New(t).Assert(t.Name(), body)
}

// TestExecuteAPIV2_EventShape_PayloadV1_NamedStage proves a named stage is
// retained as a path segment in requestContext.path, and configured stage
// variables are included in the payload-1.0 event.
func TestExecuteAPIV2_EventShape_PayloadV1_NamedStage(t *testing.T) {
	client := executeAPIV2Client(t)

	lambda := mockLambdaEchoV1Shape()
	t.Cleanup(lambda.Close)

	fn := "executeapi-eventshape-v1-named-fn"
	createLambdaWithEndpoint(t, fn, lambda.URL)

	api, err := client.CreateApi(t.Context(), &apigatewayv2.CreateApiInput{
		Name:         aws.String("executeapi-eventshape-v1-named"),
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
		PayloadFormatVersion: aws.String("1.0"),
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
		ApiId:          api.ApiId,
		StageName:      aws.String("dev"),
		AutoDeploy:     aws.Bool(true),
		StageVariables: map[string]string{"env": "staging"},
	}); err != nil {
		t.Fatalf("CreateStage: %v", err)
	}

	status, body := callStage(t, http.MethodGet, *api.ApiId, "dev", "/items")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", status, body)
	}

	golden.New(t).Assert(t.Name(), body)
}
