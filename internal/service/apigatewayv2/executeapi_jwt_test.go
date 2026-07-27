package apigatewayv2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"
)

const (
	testJWTIssuer   = "https://issuer.example.com"
	testJWTAudience = "test-audience"
)

// newJWTTestAPI wires up an API with a $default stage, an AWS_PROXY
// integration pointing at lambdaURL, and a GET /items route protected by a
// JWT authorizer configured with testJWTIssuer/testJWTAudience. It returns
// the service and the API's id.
func newJWTTestAPI(t *testing.T, lambdaURL string) (*Service, string) {
	t.Helper()

	storage := NewMemoryStorage()
	svc := New(storage)
	svc.baseURL = lambdaURL

	api, err := storage.CreateAPI(t.Context(), &CreateAPIRequest{Name: "jwt-api", ProtocolType: "HTTP"})
	if err != nil {
		t.Fatalf("CreateAPI() error = %v", err)
	}

	authorizer, err := storage.CreateAuthorizer(t.Context(), api.APIID, &CreateAuthorizerRequest{
		Name:           "jwt-authorizer",
		AuthorizerType: authorizerTypeJWT,
		JWTConfiguration: &JWTConfiguration{
			Issuer:   testJWTIssuer,
			Audience: []string{testJWTAudience},
		},
	})
	if err != nil {
		t.Fatalf("CreateAuthorizer() error = %v", err)
	}

	integration, err := storage.CreateIntegration(t.Context(), api.APIID, &CreateIntegrationRequest{
		IntegrationType:      "AWS_PROXY",
		IntegrationURI:       "arn:aws:lambda:us-east-1:000000000000:function:jwt-test-fn",
		PayloadFormatVersion: "2.0",
	})
	if err != nil {
		t.Fatalf("CreateIntegration() error = %v", err)
	}

	_, err = storage.CreateRoute(t.Context(), api.APIID, &CreateRouteRequest{
		RouteKey:          "GET /items",
		Target:            "integrations/" + integration.IntegrationID,
		AuthorizationType: authorizerTypeJWT,
		AuthorizerID:      authorizer.AuthorizerID,
	})
	if err != nil {
		t.Fatalf("CreateRoute() error = %v", err)
	}

	if _, err := storage.CreateStage(t.Context(), api.APIID, &CreateStageRequest{
		StageName:  defaultStageName,
		AutoDeploy: true,
	}); err != nil {
		t.Fatalf("CreateStage() error = %v", err)
	}

	return svc, api.APIID
}

// TestHandleExecuteAPI_JWTAuthorizer_Unauthorized covers the 401 cases: a
// missing header, a malformed token, an expired token, a wrong issuer, and a
// wrong audience must all be rejected before the integration is invoked.
func TestHandleExecuteAPI_JWTAuthorizer_Unauthorized(t *testing.T) {
	future := time.Now().Add(time.Hour).Unix()
	past := time.Now().Add(-time.Hour).Unix()

	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "missing Authorization header",
			header: "",
		},
		{
			name:   "malformed token (no Bearer prefix)",
			header: "sometoken",
		},
		{
			name: "expired token",
			header: "Bearer " + buildTestJWT(t, map[string]any{
				"iss": testJWTIssuer,
				"aud": testJWTAudience,
				"exp": past,
			}),
		},
		{
			name: "wrong issuer",
			header: "Bearer " + buildTestJWT(t, map[string]any{
				"iss": "https://wrong-issuer.example.com",
				"aud": testJWTAudience,
				"exp": future,
			}),
		},
		{
			name: "wrong audience",
			header: "Bearer " + buildTestJWT(t, map[string]any{
				"iss": testJWTIssuer,
				"aud": "wrong-audience",
				"exp": future,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkJWTUnauthorized(t, tt.header)
		})
	}
}

func checkJWTUnauthorized(t *testing.T, header string) {
	t.Helper()

	lambdaInvoked := false

	lambda := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		lambdaInvoked = true
	}))
	t.Cleanup(lambda.Close)

	svc, apiID := newJWTTestAPI(t, lambda.URL)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	if header != "" {
		r.Header.Set("Authorization", header)
	}

	w := httptest.NewRecorder()

	if !svc.HandleExecuteAPI(w, r, apiID, "/items") {
		t.Fatal("HandleExecuteAPI() returned false, want true")
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d; body=%q", w.Code, http.StatusUnauthorized, w.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error body: %v", err)
	}

	if body["message"] != "Unauthorized" {
		t.Errorf("error body message = %q, want %q", body["message"], "Unauthorized")
	}

	if lambdaInvoked {
		t.Error("lambda integration was invoked despite failed authorization")
	}
}

// jwtEventCapture decodes the requestContext.authorizer.jwt block of a
// payload-2.0 Lambda event.
type jwtEventCapture struct {
	RequestContext struct {
		Authorizer struct {
			JWT struct {
				Claims map[string]string `json:"claims"`
				Scopes []string          `json:"scopes"`
			} `json:"jwt"`
		} `json:"authorizer"`
	} `json:"requestContext"`
}

// newCapturingLambda starts a mock Lambda invoke endpoint that decodes each
// request body into captured and always responds with a 200.
func newCapturingLambda(t *testing.T, captured *jwtEventCapture) *httptest.Server {
	t.Helper()

	lambda := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(captured); err != nil {
			t.Fatalf("decode lambda event: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"statusCode": 200, "body": "ok"})
	}))
	t.Cleanup(lambda.Close)

	return lambda
}

// TestHandleExecuteAPI_JWTAuthorizer_Valid proves a valid token's claims and
// scopes are propagated to the Lambda event's
// requestContext.authorizer.jwt block.
func TestHandleExecuteAPI_JWTAuthorizer_Valid(t *testing.T) {
	future := time.Now().Add(time.Hour).Unix()

	var captured jwtEventCapture

	lambda := newCapturingLambda(t, &captured)
	svc, apiID := newJWTTestAPI(t, lambda.URL)

	token := buildTestJWT(t, map[string]any{
		"sub":   "user-1",
		"iss":   testJWTIssuer,
		"aud":   testJWTAudience,
		"exp":   future,
		"scope": "read write",
	})

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()

	if !svc.HandleExecuteAPI(w, r, apiID, "/items") {
		t.Fatal("HandleExecuteAPI() returned false, want true")
	}

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", w.Code, w.Body.String())
	}

	checkCapturedJWTClaims(t, captured, future)
}

func checkCapturedJWTClaims(t *testing.T, captured jwtEventCapture, wantExpUnix int64) {
	t.Helper()

	claims := captured.RequestContext.Authorizer.JWT.Claims
	if claims["sub"] != "user-1" {
		t.Errorf("claims[sub] = %q, want %q", claims["sub"], "user-1")
	}

	if claims["iss"] != testJWTIssuer {
		t.Errorf("claims[iss] = %q, want %q", claims["iss"], testJWTIssuer)
	}

	wantExp := strconv.FormatInt(wantExpUnix, 10)
	if claims["exp"] != wantExp {
		t.Errorf("claims[exp] = %q, want %q (stringified number)", claims["exp"], wantExp)
	}

	wantScopes := []string{"read", "write"}
	if gotScopes := captured.RequestContext.Authorizer.JWT.Scopes; !slices.Equal(gotScopes, wantScopes) {
		t.Errorf("scopes = %v, want %v", gotScopes, wantScopes)
	}
}

// TestHandleExecuteAPI_NoAuthorizer proves routes without a JWT authorizer
// are unaffected by this feature: no Authorization header is required and no
// authorizer block appears in the Lambda event.
func TestHandleExecuteAPI_NoAuthorizer(t *testing.T) {
	var capturedEvent map[string]any

	lambda := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedEvent); err != nil {
			t.Fatalf("decode lambda event: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"statusCode": 200, "body": "ok"})
	}))
	t.Cleanup(lambda.Close)

	storage := NewMemoryStorage()
	svc := New(storage)
	svc.baseURL = lambda.URL

	api, err := storage.CreateAPI(t.Context(), &CreateAPIRequest{Name: "no-auth-api", ProtocolType: "HTTP"})
	if err != nil {
		t.Fatalf("CreateAPI() error = %v", err)
	}

	integration, err := storage.CreateIntegration(t.Context(), api.APIID, &CreateIntegrationRequest{
		IntegrationType:      "AWS_PROXY",
		IntegrationURI:       "arn:aws:lambda:us-east-1:000000000000:function:no-auth-fn",
		PayloadFormatVersion: "2.0",
	})
	if err != nil {
		t.Fatalf("CreateIntegration() error = %v", err)
	}

	if _, err := storage.CreateRoute(t.Context(), api.APIID, &CreateRouteRequest{
		RouteKey: "GET /items",
		Target:   "integrations/" + integration.IntegrationID,
	}); err != nil {
		t.Fatalf("CreateRoute() error = %v", err)
	}

	if _, err := storage.CreateStage(t.Context(), api.APIID, &CreateStageRequest{
		StageName:  defaultStageName,
		AutoDeploy: true,
	}); err != nil {
		t.Fatalf("CreateStage() error = %v", err)
	}

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/items", nil)
	w := httptest.NewRecorder()

	if !svc.HandleExecuteAPI(w, r, api.APIID, "/items") {
		t.Fatal("HandleExecuteAPI() returned false, want true")
	}

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", w.Code, w.Body.String())
	}

	requestContext, _ := capturedEvent["requestContext"].(map[string]any)
	if _, present := requestContext["authorizer"]; present {
		t.Error("requestContext.authorizer is present for a route without an authorizer")
	}
}
