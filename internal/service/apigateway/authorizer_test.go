package apigateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sivchari/kumo/internal/service/execapi"
)

const testMethodArn = "arn:aws:execute-api:us-east-1:000000000000:abc123/dev/GET/items"

// The resolved method every authorizer case is evaluated against.
const (
	testResourcePath = "/items"
	testResourceID   = "res123"
	testHTTPMethod   = "GET"
	testStage        = "dev"
)

// Identity source expressions reused across cases.
const (
	authHeaderSource  = "method.request.header.Authorization"
	shortHeaderSource = "method.request.header.Auth"
)

// authorizerFnURI is a REST-form authorizerUri whose embedded function ARN
// resolves to "authz-fn".
const authorizerFnURI = "arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/" +
	"arn:aws:lambda:us-east-1:000000000000:function:authz-fn/invocations"

const (
	allowPolicyAll = `{"principalId":"u","policyDocument":{"Version":"2012-10-17",` +
		`"Statement":[{"Action":"execute-api:Invoke","Effect":"Allow","Resource":"*"}]}}`
	denyPolicyAll = `{"principalId":"u","policyDocument":{"Version":"2012-10-17",` +
		`"Statement":[{"Action":"execute-api:Invoke","Effect":"Deny","Resource":"*"}]}}`
)

// fakeLambda is a stand-in for kumo's own Lambda invoke endpoint, which is how
// the execute-api data plane reaches an authorizer function. It records every
// invocation and replays a fixed response.
type fakeLambda struct {
	server   *httptest.Server
	calls    atomic.Int32
	response string
	// functionError, when set, is returned as the X-Amz-Function-Error header
	// so the invocation looks like a failed function.
	functionError string
	// status, when non-zero, replaces the 200 a successful invoke returns.
	status int

	lastPath  string
	lastEvent []byte
}

// newFakeLambda starts a fake Lambda invoke endpoint returning response.
func newFakeLambda(t *testing.T, response string) *fakeLambda {
	t.Helper()

	f := &fakeLambda{response: response}

	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		f.lastPath = r.URL.Path

		f.lastEvent, _ = io.ReadAll(r.Body)

		if f.status != 0 {
			w.WriteHeader(f.status)

			return
		}

		if f.functionError != "" {
			w.Header().Set("X-Amz-Function-Error", f.functionError)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(f.response))
	}))
	t.Cleanup(f.server.Close)

	return f
}

// newAuthorizerService builds a Service whose self-calls reach lambda, backed by
// in-memory storage holding a single REST API, a "dev" stage and one authorizer.
func newAuthorizerService(
	t *testing.T, lambda *fakeLambda, authorizer *CreateAuthorizerRequest,
) (svc *Service, apiID, authID string) {
	t.Helper()

	store := NewMemoryStorage()
	svc = New(store)

	if lambda != nil {
		svc.baseURL = lambda.server.URL
	}

	api, err := store.CreateRestAPI(t.Context(), &CreateRestAPIRequest{Name: "test"})
	if err != nil {
		t.Fatalf("CreateRestAPI: %v", err)
	}

	authz, err := store.CreateAuthorizer(t.Context(), api.ID, authorizer)
	if err != nil {
		t.Fatalf("CreateAuthorizer: %v", err)
	}

	return svc, api.ID, authz.ID
}

// requestAuthorizer builds a REQUEST authorizer request with caching disabled,
// which is what most cases want so every call reaches the function.
func requestAuthorizer(uri, identitySource string) *CreateAuthorizerRequest {
	ttl := int32(0)

	return &CreateAuthorizerRequest{
		Name:                         "a",
		Type:                         authorizerTypeRequest,
		AuthorizerURI:                uri,
		IdentitySource:               identitySource,
		AuthorizerResultTTLInSeconds: &ttl,
	}
}

// authorizerTarget builds the resolved method a CUSTOM-authorized GET /items
// request resolves to.
func authorizerTarget(authID string, stageVars map[string]string) *resolvedTarget {
	return &resolvedTarget{
		resource: &Resource{ID: testResourceID, Path: testResourcePath},
		method:   Method{HTTPMethod: testHTTPMethod, AuthorizationType: authTypeCustom, AuthorizerID: authID},
		stage:    &Stage{StageName: testStage, Variables: stageVars},
	}
}

// authorizeCase is one authorizer outcome and the status it must map to.
type authorizeCase struct {
	name          string
	uri           string
	setAuthHeader bool
	wrongAuthID   bool
	authType      string
	response      string
	functionError string
	lambdaStatus  int
	want          int
}

// authorizeCases enumerates the outcomes AWS documents for a REQUEST authorizer.
func authorizeCases() []authorizeCase {
	return []authorizeCase{
		{name: "allow", uri: authorizerFnURI, setAuthHeader: true, response: allowPolicyAll, want: 0},
		{
			name: "explicit deny is forbidden", uri: authorizerFnURI, setAuthHeader: true,
			response: denyPolicyAll, want: http.StatusForbidden,
		},
		{
			name: "no matching statement is forbidden", uri: authorizerFnURI, setAuthHeader: true,
			response: `{"principalId":"u","policyDocument":{"Version":"2012-10-17","Statement":` +
				`[{"Effect":"Allow","Resource":"arn:aws:execute-api:us-east-1:000000000000:other/dev/GET/x"}]}}`,
			want: http.StatusForbidden,
		},
		{
			name: "missing identity source is unauthorized", uri: authorizerFnURI,
			response: allowPolicyAll, want: http.StatusUnauthorized,
		},
		{
			name: "authorizer not found is a configuration error", uri: authorizerFnURI,
			setAuthHeader: true, wrongAuthID: true, response: allowPolicyAll, want: http.StatusInternalServerError,
		},
		{
			name: "non-REQUEST type is a configuration error", uri: authorizerFnURI, setAuthHeader: true,
			authType: authorizerTypeToken, response: allowPolicyAll, want: http.StatusInternalServerError,
		},
		{
			name: "unresolvable uri is a configuration error", uri: "", setAuthHeader: true,
			response: allowPolicyAll, want: http.StatusInternalServerError,
		},
		{
			name: "invoke failure is an authorizer failure", uri: authorizerFnURI, setAuthHeader: true,
			lambdaStatus: http.StatusBadGateway, want: http.StatusInternalServerError,
		},
		{
			name: "Unauthorized error is unauthorized", uri: authorizerFnURI, setAuthHeader: true,
			response: `{"errorMessage":"Unauthorized","errorType":"Error"}`, functionError: "Unhandled",
			want: http.StatusUnauthorized,
		},
		{
			name: "other function error is an authorizer failure", uri: authorizerFnURI, setAuthHeader: true,
			response: `{"errorMessage":"boom","errorType":"Error"}`, functionError: "Unhandled",
			want: http.StatusInternalServerError,
		},
		{
			name: "response without a policy is an authorizer failure", uri: authorizerFnURI,
			setAuthHeader: true, response: `{"errorMessage":"boom"}`, want: http.StatusInternalServerError,
		},
		{
			name: "response that is not JSON is an authorizer failure", uri: authorizerFnURI,
			setAuthHeader: true, response: `not a policy`, want: http.StatusInternalServerError,
		},
		{
			name: "simple isAuthorized response is rejected for REST", uri: authorizerFnURI,
			setAuthHeader: true, response: `{"isAuthorized":true}`, want: http.StatusInternalServerError,
		},
	}
}

func TestAuthorize(t *testing.T) {
	t.Parallel()

	for _, tc := range authorizeCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lambda := newFakeLambda(t, tc.response)
			lambda.functionError = tc.functionError
			lambda.status = tc.lambdaStatus

			req := requestAuthorizer(tc.uri, authHeaderSource)
			if tc.authType != "" {
				req.Type = tc.authType
			}

			svc, apiID, authID := newAuthorizerService(t, lambda, req)

			if tc.wrongAuthID {
				authID = "does-not-exist"
			}

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, testResourcePath, http.NoBody)
			if tc.setAuthHeader {
				r.Header.Set("Authorization", "Bearer token")
			}

			if got := svc.authorize(r, apiID, testStage, authorizerTarget(authID, nil)); got != tc.want {
				t.Errorf("authorize = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestAuthorize_MissingIdentitySourceSkipsInvoke proves AWS's documented
// shortcut: a missing identity source is a 401 and the function is never run.
func TestAuthorize_MissingIdentitySourceSkipsInvoke(t *testing.T) {
	t.Parallel()

	lambda := newFakeLambda(t, allowPolicyAll)
	svc, apiID, authID := newAuthorizerService(t, lambda,
		requestAuthorizer(authorizerFnURI, authHeaderSource))

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, testResourcePath, http.NoBody)

	if got := svc.authorize(r, apiID, testStage, authorizerTarget(authID, nil)); got != http.StatusUnauthorized {
		t.Fatalf("authorize = %d, want 401", got)
	}

	if calls := lambda.calls.Load(); calls != 0 {
		t.Errorf("authorizer invoked %d times, want 0", calls)
	}
}

// TestAuthorize_IdentitySourceForms covers the four identity source expressions
// AWS accepts for a REQUEST authorizer, plus one it does not.
func TestAuthorize_IdentitySourceForms(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		identitySource string
		url            string
		header         string
		stageVars      map[string]string
		want           int
	}{
		{name: "header present", identitySource: authHeaderSource, url: testResourcePath, header: "x"},
		{
			name: "header missing", identitySource: authHeaderSource, url: testResourcePath,
			want: http.StatusUnauthorized,
		},
		{name: "querystring present", identitySource: "method.request.querystring.token", url: testResourcePath + "?token=x"},
		{
			name: "querystring missing", identitySource: "method.request.querystring.token", url: testResourcePath,
			want: http.StatusUnauthorized,
		},
		{
			name: "stage variable present", identitySource: "stageVariables.key", url: testResourcePath,
			stageVars: map[string]string{"key": "x"},
		},
		{name: "stage variable missing", identitySource: "stageVariables.key", url: testResourcePath, want: http.StatusUnauthorized},
		{name: "context value", identitySource: "context.accountId", url: testResourcePath},
		{
			name: "multiple sources, one missing", identitySource: authHeaderSource + ", context.stage",
			url: testResourcePath, want: http.StatusUnauthorized,
		},
		{
			name: "multiple sources present", identitySource: authHeaderSource + ", context.stage",
			url: testResourcePath, header: "x",
		},
		{
			name: "unsupported source is a configuration error", identitySource: "method.request.path.id",
			url: testResourcePath, want: http.StatusInternalServerError,
		},
		{name: "no identity source invokes the function", identitySource: "", url: testResourcePath},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lambda := newFakeLambda(t, allowPolicyAll)
			svc, apiID, authID := newAuthorizerService(t, lambda,
				requestAuthorizer(authorizerFnURI, tc.identitySource))

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.url, http.NoBody)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}

			if got := svc.authorize(r, apiID, testStage, authorizerTarget(authID, tc.stageVars)); got != tc.want {
				t.Errorf("authorize = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestAuthorize_CachesResultForTTL proves authorizerResultTtlInSeconds is
// honoured: a second request with the same identity source value reuses the
// cached response, and a different value re-invokes the function.
func TestAuthorize_CachesResultForTTL(t *testing.T) {
	t.Parallel()

	lambda := newFakeLambda(t, allowPolicyAll)

	ttl := int32(300)
	svc, apiID, authID := newAuthorizerService(t, lambda, &CreateAuthorizerRequest{
		Name:                         "cached",
		Type:                         authorizerTypeRequest,
		AuthorizerURI:                authorizerFnURI,
		IdentitySource:               authHeaderSource,
		AuthorizerResultTTLInSeconds: &ttl,
	})

	target := authorizerTarget(authID, nil)

	call := func(token string) int {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, testResourcePath, http.NoBody)
		r.Header.Set("Authorization", token)

		return svc.authorize(r, apiID, testStage, target)
	}

	for range 3 {
		if got := call("token-a"); got != 0 {
			t.Fatalf("authorize = %d, want 0", got)
		}
	}

	if calls := lambda.calls.Load(); calls != 1 {
		t.Errorf("authorizer invoked %d times for one identity, want 1", calls)
	}

	if got := call("token-b"); got != 0 {
		t.Fatalf("authorize = %d, want 0", got)
	}

	if calls := lambda.calls.Load(); calls != 2 {
		t.Errorf("authorizer invoked %d times for two identities, want 2", calls)
	}
}

// TestAuthorize_CachingDisabledInvokesEveryRequest proves a TTL of 0 disables
// caching.
func TestAuthorize_CachingDisabledInvokesEveryRequest(t *testing.T) {
	t.Parallel()

	lambda := newFakeLambda(t, allowPolicyAll)
	svc, apiID, authID := newAuthorizerService(t, lambda,
		requestAuthorizer(authorizerFnURI, authHeaderSource))

	target := authorizerTarget(authID, nil)

	for range 3 {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, testResourcePath, http.NoBody)
		r.Header.Set("Authorization", "token")

		if got := svc.authorize(r, apiID, testStage, target); got != 0 {
			t.Fatalf("authorize = %d, want 0", got)
		}
	}

	if calls := lambda.calls.Load(); calls != 3 {
		t.Errorf("authorizer invoked %d times, want 3", calls)
	}
}

// TestAuthorize_BuildsEventAndResolvesFunction checks the REQUEST authorizer
// event against the shape AWS documents for a REST API.
func TestAuthorize_BuildsEventAndResolvesFunction(t *testing.T) {
	t.Parallel()

	lambda := newFakeLambda(t, allowPolicyAll)
	svc, apiID, authID := newAuthorizerService(t, lambda,
		requestAuthorizer(authorizerFnURI, authHeaderSource))

	target := authorizerTarget(authID, map[string]string{"stageVar": "sv"})
	target.pathParams = map[string]string{"id": "1"}

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, testResourcePath+"?foo=bar", http.NoBody)
	r.Header.Set("Authorization", "Bearer x")

	if got := svc.authorize(r, apiID, testStage, target); got != 0 {
		t.Fatalf("authorize = %d, want 0", got)
	}

	if !strings.HasSuffix(lambda.lastPath, "/functions/authz-fn/invocations") {
		t.Errorf("invoke path = %q, want the authz-fn invoke endpoint", lambda.lastPath)
	}

	var ev AuthorizerEvent
	if err := json.Unmarshal(lambda.lastEvent, &ev); err != nil {
		t.Fatalf("authorizer event is not valid JSON: %v (%s)", err, lambda.lastEvent)
	}

	assertAuthorizerEnvelope(t, &ev, apiID)
	assertAuthorizerParameters(t, &ev)
	assertAuthorizerRequestContext(t, &ev.RequestContext, apiID)
}

// assertAuthorizerEnvelope checks the top-level fields of the authorizer event.
func assertAuthorizerEnvelope(t *testing.T, ev *AuthorizerEvent, apiID string) {
	t.Helper()

	wantArn := "arn:aws:execute-api:us-east-1:000000000000:" + apiID + "/dev/GET/items"
	if ev.MethodArn != wantArn {
		t.Errorf("methodArn = %q, want %q", ev.MethodArn, wantArn)
	}

	if ev.Type != authorizerTypeRequest || ev.HTTPMethod != testHTTPMethod || ev.Resource != testResourcePath {
		t.Errorf("unexpected event envelope: %+v", ev)
	}

	if ev.Path != "/dev/items" {
		t.Errorf("path = %q, want /dev/items", ev.Path)
	}
}

// assertAuthorizerParameters checks the request parameter groups AWS documents
// a REQUEST authorizer receives.
func assertAuthorizerParameters(t *testing.T, ev *AuthorizerEvent) {
	t.Helper()

	if ev.Headers["Authorization"] != "Bearer x" {
		t.Errorf("Authorization header not forwarded: %+v", ev.Headers)
	}

	if ev.QueryStringParameters["foo"] != "bar" {
		t.Errorf("query parameter not forwarded: %+v", ev.QueryStringParameters)
	}

	if ev.PathParameters["id"] != "1" {
		t.Errorf("path parameter not forwarded: %+v", ev.PathParameters)
	}

	if ev.StageVariables["stageVar"] != "sv" {
		t.Errorf("stage variable not forwarded: %+v", ev.StageVariables)
	}
}

// assertAuthorizerRequestContext checks the requestContext block.
func assertAuthorizerRequestContext(t *testing.T, ctx *AuthorizerRequestCtx, apiID string) {
	t.Helper()

	if ctx.APIID != apiID || ctx.Stage != testStage || ctx.ResourcePath != testResourcePath ||
		ctx.HTTPMethod != testHTTPMethod || ctx.Path != "/dev/items" || ctx.ResourceID != testResourceID ||
		ctx.AccountID != executeAPIAccount || ctx.RequestID == "" {
		t.Errorf("unexpected requestContext: %+v", ctx)
	}

	if ctx.Identity.SourceIP == "" {
		t.Errorf("requestContext.identity.sourceIp is empty")
	}
}

// TestBuildAuthorizerEvent_EmptyParameterGroupsAreObjects pins the documented
// `"pathParameters": {}` shape: absent parameter groups must not be null, so an
// authorizer can index them unconditionally.
func TestBuildAuthorizerEvent_EmptyParameterGroupsAreObjects(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, testResourcePath, http.NoBody)
	event := buildAuthorizerEvent(r, "abc123", "dev", authorizerTarget("auth1", nil), testMethodArn)

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	for _, key := range []string{"headers", "queryStringParameters", "pathParameters", "stageVariables"} {
		if strings.Contains(string(body), `"`+key+`":null`) {
			t.Errorf("%s is null, want {}: %s", key, body)
		}
	}
}

func TestEvaluateAuthorizerResponse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		want    int
	}{
		{"allow", allowPolicyAll, 0},
		{"deny", denyPolicyAll, http.StatusForbidden},
		{"lambda error envelope", `{"errorMessage":"boom","errorType":"Error"}`, http.StatusInternalServerError},
		{"invalid json", `not json at all`, http.StatusInternalServerError},
		{"empty", ``, http.StatusInternalServerError},
		{
			"principal without policy", `{"principalId":"u"}`,
			http.StatusInternalServerError,
		},
		{
			"policy without principal",
			`{"policyDocument":{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Resource":"*"}]}}`,
			http.StatusInternalServerError,
		},
		{
			"empty statement list",
			`{"principalId":"u","policyDocument":{"Version":"2012-10-17","Statement":[]}}`,
			http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := evaluateAuthorizerResponse([]byte(tc.payload), testMethodArn); got != tc.want {
				t.Errorf("evaluateAuthorizerResponse(%q) = %d, want %d", tc.payload, got, tc.want)
			}
		})
	}
}

func TestEvaluatePolicy(t *testing.T) {
	t.Parallel()

	const otherArn = "arn:aws:execute-api:us-east-1:000000000000:abc123/dev/POST/items"

	cases := []struct {
		name       string
		statements string // JSON array of IAM statements
		want       bool
	}{
		{"allow exact", `[{"Action":"execute-api:Invoke","Effect":"Allow","Resource":"` + testMethodArn + `"}]`, true},
		{"deny exact", `[{"Effect":"Deny","Resource":"` + testMethodArn + `"}]`, false},
		{
			"allow wildcard tail",
			`[{"Effect":"Allow","Resource":"arn:aws:execute-api:us-east-1:000000000000:abc123/dev/*"}]`, true,
		},
		{"allow star", `[{"Effect":"Allow","Resource":"*"}]`, true},
		{"deny precedence", `[{"Effect":"Allow","Resource":"*"},{"Effect":"Deny","Resource":"` + testMethodArn + `"}]`, false},
		{"no match", `[{"Effect":"Allow","Resource":"` + otherArn + `"}]`, false},
		{"resource array match", `[{"Effect":"Allow","Resource":["` + otherArn + `","` + testMethodArn + `"]}]`, true},
		{"empty statements", `[]`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var statements []PolicyStatement
			if err := json.Unmarshal([]byte(tc.statements), &statements); err != nil {
				t.Fatalf("unmarshal statements: %v", err)
			}

			doc := PolicyDocument{Version: "2012-10-17", Statement: statements}
			if got := evaluatePolicy(doc, testMethodArn); got != tc.want {
				t.Errorf("evaluatePolicy = %v, want %v", got, tc.want)
			}
		})
	}
}

// abc is the subject of the wildcard matching table.
const abc = "abc"

func TestWildcardMatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		pattern string
		s       string
		want    bool
	}{
		{"exact", abc, abc, true},
		{"mismatch", abc, "abd", false},
		{"star all", "*", "anything/at/all", true},
		{"star tail", abc + "/*", abc + "/def/ghi", true},
		{"star tail needs separator", abc + "/*", abc, false},
		{"star middle", "a*c", "abbbc", true},
		{"question single", "ab?", abc, true},
		{"question too long", "ab?", "abcd", false},
		{"trailing star empty", abc + "*", abc, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := wildcardMatch(tc.pattern, tc.s); got != tc.want {
				t.Errorf("wildcardMatch(%q, %q) = %v, want %v", tc.pattern, tc.s, got, tc.want)
			}
		})
	}
}

func TestBuildMethodArn(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		resourcePath string
		want         string
	}{
		{"sub resource", "/items", "arn:aws:execute-api:us-east-1:000000000000:abc123/dev/GET/items"},
		{"root", "/", "arn:aws:execute-api:us-east-1:000000000000:abc123/dev/GET/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := buildMethodArn("abc123", "dev", "GET", tc.resourcePath); got != tc.want {
				t.Errorf("buildMethodArn = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLambdaFunctionNameFromURI_Authorizer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		uri  string
		want string
	}{
		{
			"rest authorizer uri",
			"arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/" +
				"arn:aws:lambda:us-east-1:000000000000:function:my-authorizer/invocations",
			"my-authorizer",
		},
		{
			"bare function arn",
			"arn:aws:lambda:us-east-1:000000000000:function:my-authorizer",
			"my-authorizer",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := execapi.LambdaFunctionNameFromURI(tc.uri); got != tc.want {
				t.Errorf("LambdaFunctionNameFromURI = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateAuthorizer(t *testing.T) {
	t.Parallel()

	ttl := func(v int32) *int32 { return &v }

	cases := []struct {
		name    string
		req     *CreateAuthorizerRequest
		wantErr bool
	}{
		{
			"request with identity source and default ttl",
			&CreateAuthorizerRequest{Name: "a", Type: authorizerTypeRequest, IdentitySource: shortHeaderSource},
			false,
		},
		{
			"request without identity source and caching on",
			&CreateAuthorizerRequest{Name: "a", Type: authorizerTypeRequest},
			true,
		},
		{
			"request without identity source and caching off",
			&CreateAuthorizerRequest{Name: "a", Type: authorizerTypeRequest, AuthorizerResultTTLInSeconds: ttl(0)},
			false,
		},
		{
			"ttl above the maximum",
			&CreateAuthorizerRequest{
				Name: "a", Type: authorizerTypeRequest,
				IdentitySource: shortHeaderSource, AuthorizerResultTTLInSeconds: ttl(3601),
			},
			true,
		},
		{
			"negative ttl",
			&CreateAuthorizerRequest{
				Name: "a", Type: authorizerTypeRequest,
				IdentitySource: shortHeaderSource, AuthorizerResultTTLInSeconds: ttl(-1),
			},
			true,
		},
		{
			"token authorizer needs no identity source",
			&CreateAuthorizerRequest{Name: "a", Type: authorizerTypeToken},
			false,
		},
		{"unknown type", &CreateAuthorizerRequest{Name: "a", Type: "LAMBDA"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := validateAuthorizer(tc.req) != ""; gotErr != tc.wantErr {
				t.Errorf("validateAuthorizer error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestCreateAuthorizerRequest_ResolvedTTL(t *testing.T) {
	t.Parallel()

	if got := (&CreateAuthorizerRequest{}).resolvedTTL(); got != defaultAuthorizerResultTTL {
		t.Errorf("absent ttl = %d, want %d", got, defaultAuthorizerResultTTL)
	}

	zero := int32(0)
	if got := (&CreateAuthorizerRequest{AuthorizerResultTTLInSeconds: &zero}).resolvedTTL(); got != 0 {
		t.Errorf("explicit zero ttl = %d, want 0", got)
	}
}

func TestRawToStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"single string", `"a"`, []string{"a"}},
		{"array", `["a","b"]`, []string{"a", "b"}},
		{"empty", ``, nil},
		{"number", `123`, nil},
		{"object", `{}`, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := rawToStrings(json.RawMessage(tc.raw)); !slices.Equal(got, tc.want) {
				t.Errorf("rawToStrings(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
