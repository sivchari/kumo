//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigateway/types"
)

// authorizerPolicyServer returns an httptest server that acts as a REQUEST-type
// Lambda authorizer, keyed off the Authorization header of the event:
// "allow" returns an Allow policy, "deny" a Deny policy, "unauthorized" the
// "Unauthorized" error a function raises to ask for a 401, and anything else a
// generic error envelope. It counts how many times it was invoked.
func authorizerPolicyServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		var event map[string]any
		_ = json.NewDecoder(r.Body).Decode(&event)

		headers, _ := event["headers"].(map[string]any)
		auth, _ := headers["Authorization"].(string)
		methodArn, _ := event["methodArn"].(string)

		w.Header().Set("Content-Type", "application/json")

		switch auth {
		case "deny":
			_ = json.NewEncoder(w).Encode(authPolicy("user-deny", "Deny", methodArn))
		case "unauthorized":
			// A function that raises the literal "Unauthorized" error.
			w.Header().Set("X-Amz-Function-Error", "Unhandled")
			_, _ = w.Write([]byte(`{"errorMessage":"Unauthorized","errorType":"Error"}`))
		case "boom":
			// Any other exception from the authorizer function.
			w.Header().Set("X-Amz-Function-Error", "Unhandled")
			_, _ = w.Write([]byte(`{"errorMessage":"boom","errorType":"Error"}`))
		case "malformed":
			// A 200 response that is not a principal plus a policy document.
			_, _ = w.Write([]byte(`{"isAuthorized":true}`))
		default:
			_ = json.NewEncoder(w).Encode(authPolicy("user-allow", "Allow", methodArn))
		}
	}))
	t.Cleanup(srv.Close)

	return srv, &calls
}

// authPolicy builds a REQUEST-authorizer response with a single-statement IAM
// policy for the given effect and resource.
func authPolicy(principal, effect, resource string) map[string]any {
	return map[string]any{
		"principalId": principal,
		"policyDocument": map[string]any{
			"Version": "2012-10-17",
			"Statement": []map[string]any{
				{
					"Action":   "execute-api:Invoke",
					"Effect":   effect,
					"Resource": resource,
				},
			},
		},
	}
}

// buildAuthorizerAPI wires a REST API with a single GET /items route guarded by
// a CUSTOM (REQUEST-type) Lambda authorizer and backed by an AWS_PROXY Lambda
// integration, deployed to the given stage. resultTTL is the authorizer's
// authorizerResultTtlInSeconds; 0 disables result caching. Returns the API id.
func buildAuthorizerAPI(
	t *testing.T, client *apigateway.Client, apiName, authFn, backendFn, stage string, resultTTL int32,
) string {
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
		RestApiId: api.Id, ParentId: aws.String(root), PathPart: aws.String("items"),
	})
	if err != nil {
		t.Fatalf("CreateResource: %v", err)
	}

	authorizer, err := client.CreateAuthorizer(t.Context(), &apigateway.CreateAuthorizerInput{
		RestApiId:                    api.Id,
		Name:                         aws.String("req-authorizer"),
		Type:                         types.AuthorizerTypeRequest,
		AuthorizerUri:                aws.String(lambdaAuthorizerURI(authFn)),
		IdentitySource:               aws.String("method.request.header.Authorization"),
		AuthorizerResultTtlInSeconds: aws.Int32(resultTTL),
	})
	if err != nil {
		t.Fatalf("CreateAuthorizer: %v", err)
	}

	if _, err := client.PutMethod(t.Context(), &apigateway.PutMethodInput{
		RestApiId:         api.Id,
		ResourceId:        res.Id,
		HttpMethod:        aws.String("GET"),
		AuthorizationType: aws.String("CUSTOM"),
		AuthorizerId:      authorizer.Id,
	}); err != nil {
		t.Fatalf("PutMethod: %v", err)
	}

	if _, err := client.PutIntegration(t.Context(), &apigateway.PutIntegrationInput{
		RestApiId:             api.Id,
		ResourceId:            res.Id,
		HttpMethod:            aws.String("GET"),
		Type:                  types.IntegrationTypeAwsProxy,
		IntegrationHttpMethod: aws.String("POST"),
		Uri:                   aws.String(lambdaAuthorizerURI(backendFn)),
	}); err != nil {
		t.Fatalf("PutIntegration: %v", err)
	}

	if _, err := client.CreateDeployment(t.Context(), &apigateway.CreateDeploymentInput{
		RestApiId: api.Id, StageName: aws.String(stage),
	}); err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}

	return *api.Id
}

// lambdaAuthorizerURI builds the REST-form Lambda URI API Gateway requires for
// an authorizerUri or an AWS_PROXY integration uri.
func lambdaAuthorizerURI(fn string) string {
	return fmt.Sprintf(
		"arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/"+
			"arn:aws:lambda:us-east-1:000000000000:function:%s/invocations",
		fn,
	)
}

// callStageWithHeader invokes a deployed stage via the virtual-hosted
// execute-api endpoint, optionally setting a single request header.
func callStageWithHeader(t *testing.T, method, apiID, stage, path, headerKey, headerVal string) (int, string) {
	t.Helper()

	url := fmt.Sprintf("%s/%s%s", kumoEndpoint, stage, path)

	req, _ := http.NewRequestWithContext(t.Context(), method, url, nil)
	req.Host = apiID + ".execute-api.localhost"

	if headerKey != "" {
		req.Header.Set(headerKey, headerVal)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("call stage: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, string(body)
}

// authorizerBackend returns an httptest server standing in for the integration
// Lambda behind the authorizer.
func authorizerBackend(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"statusCode": 200, "body": "reached-backend"})
	}))
	t.Cleanup(srv.Close)

	return srv
}

// TestExecuteAPIAuthorizer proves the execute-api path runs a REQUEST-type
// Lambda authorizer before the integration, and maps each outcome to the status
// AWS documents: Allow reaches the backend (200), a Deny policy is 403, a
// missing identity source is 401 without invoking the function, the
// "Unauthorized" error is 401, and an authorizer exception or an invalid
// authorizer response is an authorizer failure (500).
func TestExecuteAPIAuthorizer(t *testing.T) {
	client := executeAPIClient(t)

	authSrv, authCalls := authorizerPolicyServer(t)
	backend := authorizerBackend(t)

	authFn := "authorizer-fn"
	backendFn := "authorizer-backend-fn"

	createLambdaWithEndpoint(t, authFn, authSrv.URL)
	createLambdaWithEndpoint(t, backendFn, backend.URL)

	apiID := buildAuthorizerAPI(t, client, "authorizer-api", authFn, backendFn, "dev", 0)

	cases := []struct {
		name       string
		authHeader string // "" means no Authorization header
		wantStatus int
	}{
		{"allow reaches backend", "allow", http.StatusOK},
		{"deny policy is forbidden", "deny", http.StatusForbidden},
		{"missing identity source is unauthorized", "", http.StatusUnauthorized},
		{"Unauthorized error is unauthorized", "unauthorized", http.StatusUnauthorized},
		{"authorizer exception is an authorizer failure", "boom", http.StatusInternalServerError},
		{"invalid authorizer response is an authorizer failure", "malformed", http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := ""
			if tc.authHeader != "" {
				key = "Authorization"
			}

			before := authCalls.Load()

			status, body := callStageWithHeader(t, http.MethodGet, apiID, "dev", "/items", key, tc.authHeader)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body=%q)", status, tc.wantStatus, body)
			}

			if tc.wantStatus == http.StatusOK && body != "reached-backend" {
				t.Errorf("body = %q, want %q", body, "reached-backend")
			}

			// A missing identity source short-circuits before the function.
			if tc.authHeader == "" && authCalls.Load() != before {
				t.Errorf("authorizer was invoked for a request with no identity source")
			}
		})
	}
}

// TestExecuteAPIAuthorizer_ResultCaching proves authorizerResultTtlInSeconds is
// honoured: repeated requests carrying the same identity source value reuse the
// cached authorizer response, and a different value invokes the function again.
func TestExecuteAPIAuthorizer_ResultCaching(t *testing.T) {
	client := executeAPIClient(t)

	authSrv, authCalls := authorizerPolicyServer(t)
	backend := authorizerBackend(t)

	authFn := "authorizer-cache-fn"
	backendFn := "authorizer-cache-backend-fn"

	createLambdaWithEndpoint(t, authFn, authSrv.URL)
	createLambdaWithEndpoint(t, backendFn, backend.URL)

	apiID := buildAuthorizerAPI(t, client, "authorizer-cache-api", authFn, backendFn, "dev", 300)

	for range 3 {
		status, body := callStageWithHeader(t, http.MethodGet, apiID, "dev", "/items", "Authorization", "allow-cached")
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%q)", status, body)
		}
	}

	if calls := authCalls.Load(); calls != 1 {
		t.Errorf("authorizer invoked %d times for one identity, want 1", calls)
	}

	status, body := callStageWithHeader(t, http.MethodGet, apiID, "dev", "/items", "Authorization", "allow-other")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%q)", status, body)
	}

	if calls := authCalls.Load(); calls != 2 {
		t.Errorf("authorizer invoked %d times for two identities, want 2", calls)
	}
}

// TestAuthorizerCRUD proves an authorizer can be created and read back via the
// management API.
func TestAuthorizerCRUD(t *testing.T) {
	client := executeAPIClient(t)

	api, err := client.CreateRestApi(t.Context(), &apigateway.CreateRestApiInput{Name: aws.String("authorizer-crud")})
	if err != nil {
		t.Fatalf("CreateRestApi: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRestApi(context.Background(), &apigateway.DeleteRestApiInput{RestApiId: api.Id})
	})

	created, err := client.CreateAuthorizer(t.Context(), &apigateway.CreateAuthorizerInput{
		RestApiId:      api.Id,
		Name:           aws.String("crud-authorizer"),
		Type:           types.AuthorizerTypeRequest,
		IdentitySource: aws.String("method.request.header.Authorization"),
	})
	if err != nil {
		t.Fatalf("CreateAuthorizer: %v", err)
	}

	// An absent authorizerResultTtlInSeconds defaults to 300 seconds.
	if aws.ToInt32(created.AuthorizerResultTtlInSeconds) != 300 {
		t.Errorf("AuthorizerResultTtlInSeconds = %d, want 300",
			aws.ToInt32(created.AuthorizerResultTtlInSeconds))
	}

	got, err := client.GetAuthorizer(t.Context(), &apigateway.GetAuthorizerInput{
		RestApiId: api.Id, AuthorizerId: created.Id,
	})
	if err != nil {
		t.Fatalf("GetAuthorizer: %v", err)
	}

	if aws.ToString(got.Id) != aws.ToString(created.Id) {
		t.Errorf("GetAuthorizer id = %q, want %q", aws.ToString(got.Id), aws.ToString(created.Id))
	}

	if got.Type != types.AuthorizerTypeRequest {
		t.Errorf("GetAuthorizer type = %q, want REQUEST", got.Type)
	}

	list, err := client.GetAuthorizers(t.Context(), &apigateway.GetAuthorizersInput{RestApiId: api.Id})
	if err != nil {
		t.Fatalf("GetAuthorizers: %v", err)
	}

	if len(list.Items) != 1 {
		t.Errorf("GetAuthorizers returned %d items, want 1", len(list.Items))
	}
}

// TestAuthorizerCRUD_RequiresIdentitySourceWhenCachingEnabled proves the
// documented CreateAuthorizer constraint: a REQUEST authorizer with caching on
// must declare the identity sources its cache key is derived from.
func TestAuthorizerCRUD_RequiresIdentitySourceWhenCachingEnabled(t *testing.T) {
	client := executeAPIClient(t)

	api, err := client.CreateRestApi(t.Context(), &apigateway.CreateRestApiInput{Name: aws.String("authorizer-validation")})
	if err != nil {
		t.Fatalf("CreateRestApi: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteRestApi(context.Background(), &apigateway.DeleteRestApiInput{RestApiId: api.Id})
	})

	if _, err := client.CreateAuthorizer(t.Context(), &apigateway.CreateAuthorizerInput{
		RestApiId: api.Id,
		Name:      aws.String("no-identity-source"),
		Type:      types.AuthorizerTypeRequest,
	}); err == nil {
		t.Error("CreateAuthorizer without an identity source and with caching enabled should fail")
	}

	// The same authorizer is accepted once caching is disabled.
	if _, err := client.CreateAuthorizer(t.Context(), &apigateway.CreateAuthorizerInput{
		RestApiId:                    api.Id,
		Name:                         aws.String("no-identity-source-uncached"),
		Type:                         types.AuthorizerTypeRequest,
		AuthorizerResultTtlInSeconds: aws.Int32(0),
	}); err != nil {
		t.Errorf("CreateAuthorizer with caching disabled: %v", err)
	}
}
