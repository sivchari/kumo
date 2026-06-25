//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigateway/types"
)

// authorizerPolicyServer returns an httptest server that acts as a REQUEST-type
// Lambda authorizer: it reads the Authorization header from the event and
// returns an Allow policy for "allow", a Deny policy for "deny", and a
// non-policy error envelope (an authorizer exception) for anything else.
func authorizerPolicyServer(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event map[string]any
		_ = json.NewDecoder(r.Body).Decode(&event)

		headers, _ := event["headers"].(map[string]any)
		auth, _ := headers["Authorization"].(string)
		methodArn, _ := event["methodArn"].(string)

		w.Header().Set("Content-Type", "application/json")

		switch auth {
		case "allow":
			_ = json.NewEncoder(w).Encode(authPolicy("user-allow", "Allow", methodArn))
		case "deny":
			_ = json.NewEncoder(w).Encode(authPolicy("user-deny", "Deny", methodArn))
		default:
			// Simulate an authorizer that raises an exception: the body is not
			// a policy, which the gateway must treat as a denial.
			_, _ = w.Write([]byte(`{"errorMessage":"boom","errorType":"Error"}`))
		}
	}))
	t.Cleanup(srv.Close)

	return srv
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
// integration, deployed to the given stage. Returns the API id.
func buildAuthorizerAPI(t *testing.T, client *apigateway.Client, apiName, authFn, backendFn, stage string) string {
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

	authURI := fmt.Sprintf(
		"arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:000000000000:function:%s/invocations",
		authFn,
	)

	authorizer, err := client.CreateAuthorizer(t.Context(), &apigateway.CreateAuthorizerInput{
		RestApiId:                    api.Id,
		Name:                         aws.String("req-authorizer"),
		Type:                         types.AuthorizerTypeRequest,
		AuthorizerUri:                aws.String(authURI),
		IdentitySource:               aws.String("method.request.header.Authorization"),
		AuthorizerResultTtlInSeconds: aws.Int32(0),
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

	backendURI := fmt.Sprintf(
		"arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:000000000000:function:%s/invocations",
		backendFn,
	)

	if _, err := client.PutIntegration(t.Context(), &apigateway.PutIntegrationInput{
		RestApiId:             api.Id,
		ResourceId:            res.Id,
		HttpMethod:            aws.String("GET"),
		Type:                  types.IntegrationTypeAwsProxy,
		IntegrationHttpMethod: aws.String("POST"),
		Uri:                   aws.String(backendURI),
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

// TestExecuteAPIAuthorizer proves the execute-api path runs a REQUEST-type
// Lambda authorizer before the integration: Allow reaches the backend (200),
// Deny is rejected (403), a missing identity-source header is unauthorized
// (401, authorizer not invoked), and an authorizer exception denies (403).
func TestExecuteAPIAuthorizer(t *testing.T) {
	client := executeAPIClient(t)

	authSrv := authorizerPolicyServer(t)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"statusCode": 200, "body": "reached-backend"})
	}))
	t.Cleanup(backend.Close)

	authFn := "authorizer-fn"
	backendFn := "authorizer-backend-fn"

	createLambdaWithEndpoint(t, authFn, authSrv.URL)
	createLambdaWithEndpoint(t, backendFn, backend.URL)

	apiID := buildAuthorizerAPI(t, client, "authorizer-api", authFn, backendFn, "dev")

	cases := []struct {
		name       string
		authHeader string // "" means no Authorization header
		wantStatus int
	}{
		{"allow reaches backend", "allow", http.StatusOK},
		{"deny is forbidden", "deny", http.StatusForbidden},
		{"missing token is unauthorized", "", http.StatusUnauthorized},
		{"authorizer exception is forbidden", "boom", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := ""
			if tc.authHeader != "" {
				key = "Authorization"
			}

			status, body := callStageWithHeader(t, http.MethodGet, apiID, "dev", "/items", key, tc.authHeader)
			if status != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body=%q)", status, tc.wantStatus, body)
			}

			if tc.wantStatus == http.StatusOK && body != "reached-backend" {
				t.Errorf("body = %q, want %q", body, "reached-backend")
			}
		})
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
