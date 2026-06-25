package apigateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/sivchari/kumo/internal/service/execapi"
)

const testMethodArn = "arn:aws:execute-api:us-east-1:000000000000:abc123/dev/GET/items"

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

// fakeInvoker is a test LambdaInvoker that records the call and returns a
// preset payload/error.
type fakeInvoker struct {
	payload  []byte
	err      error
	gotFn    string
	gotEvent []byte
}

func (f *fakeInvoker) InvokeSync(_ context.Context, fn string, payload []byte) ([]byte, error) {
	f.gotFn = fn
	f.gotEvent = payload

	return f.payload, f.err
}

// newAuthorizerService builds a Service backed by in-memory storage holding a
// single REST API and one authorizer, returning the service, REST API id, and
// authorizer id.
func newAuthorizerService(t *testing.T, uri, identitySource string) (svc *Service, apiID, authID string) {
	t.Helper()

	store := NewMemoryStorage()
	svc = New(store)

	api, err := store.CreateRestAPI(t.Context(), &CreateRestAPIRequest{Name: "test"})
	if err != nil {
		t.Fatalf("CreateRestAPI: %v", err)
	}

	authz, err := store.CreateAuthorizer(t.Context(), api.ID, &CreateAuthorizerRequest{
		Name:           "a",
		Type:           "REQUEST",
		AuthorizerURI:  uri,
		IdentitySource: identitySource,
	})
	if err != nil {
		t.Fatalf("CreateAuthorizer: %v", err)
	}

	return svc, api.ID, authz.ID
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
		{"allow wildcard tail", `[{"Effect":"Allow","Resource":"arn:aws:execute-api:us-east-1:000000000000:abc123/dev/*"}]`, true},
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

func TestPolicyAllows_NotInterpretable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
	}{
		{"lambda error envelope", `{"errorMessage":"boom","errorType":"Error"}`},
		{"invalid json", `not json at all`},
		{"empty", ``},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if policyAllows([]byte(tc.payload), testMethodArn) {
				t.Errorf("payload %q should deny", tc.payload)
			}
		})
	}
}

func TestWildcardMatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		pattern string
		s       string
		want    bool
	}{
		{"exact", "abc", "abc", true},
		{"mismatch", "abc", "abd", false},
		{"star all", "*", "anything/at/all", true},
		{"star tail", "abc/*", "abc/def/ghi", true},
		{"star tail needs separator", "abc/*", "abc", false},
		{"star middle", "a*c", "abbbc", true},
		{"question single", "ab?", "abc", true},
		{"question too long", "ab?", "abcd", false},
		{"trailing star empty", "abc*", "abc", true},
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

func TestIdentitySourceMissing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		identitySource string
		headerKey      string // header to set; "" sets none
		headerVal      string
		want           bool
	}{
		{"default missing", "", "", "", true},
		{"default present", "", "Authorization", "Bearer x", false},
		{"explicit missing", "method.request.header.Authorization", "", "", true},
		{"explicit present", "method.request.header.Authorization", "Authorization", "Bearer x", false},
		{"custom header present", "method.request.header.X-Token", "X-Token", "abc", false},
		{"custom header missing", "method.request.header.X-Token", "", "", true},
		{"non-header source ignored", "method.request.querystring.token", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/items", http.NoBody)
			if tc.headerKey != "" {
				req.Header.Set(tc.headerKey, tc.headerVal)
			}

			if got := identitySourceMissing(tc.identitySource, req); got != tc.want {
				t.Errorf("identitySourceMissing = %v, want %v", got, tc.want)
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

func TestAuthorize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		uri           string
		setAuthHeader bool
		wrongAuthID   bool
		noInvoker     bool
		invokerResp   string
		invokerErr    bool
		want          int
	}{
		{"allow", authorizerFnURI, true, false, false, allowPolicyAll, false, 0},
		{"deny", authorizerFnURI, true, false, false, denyPolicyAll, false, http.StatusForbidden},
		{"missing identity header", authorizerFnURI, false, false, false, allowPolicyAll, false, http.StatusUnauthorized},
		{"authorizer not found", authorizerFnURI, true, true, false, allowPolicyAll, false, http.StatusForbidden},
		{"no invoker wired", authorizerFnURI, true, false, true, "", false, http.StatusForbidden},
		{"unresolvable uri", "", true, false, false, allowPolicyAll, false, http.StatusForbidden},
		{"invoker error", authorizerFnURI, true, false, false, "", true, http.StatusInternalServerError},
		{"uninterpretable response", authorizerFnURI, true, false, false, "not a policy", false, http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc, apiID, authID := newAuthorizerService(t, tc.uri, "")

			if !tc.noInvoker {
				var invErr error
				if tc.invokerErr {
					invErr = errors.New("invoke failed")
				}

				svc.SetLambdaInvoker(&fakeInvoker{payload: []byte(tc.invokerResp), err: invErr})
			}

			methodAuthID := authID
			if tc.wrongAuthID {
				methodAuthID = "does-not-exist"
			}

			resolved := &resolvedTarget{
				resource: &Resource{Path: "/items"},
				method:   Method{HTTPMethod: "GET", AuthorizationType: authTypeCustom, AuthorizerID: methodAuthID},
			}

			req := httptest.NewRequest(http.MethodGet, "/items", http.NoBody)
			if tc.setAuthHeader {
				req.Header.Set("Authorization", "Bearer token")
			}

			if got := svc.authorize(req, apiID, "dev", resolved); got != tc.want {
				t.Errorf("authorize = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestAuthorize_BuildsEventAndResolvesFunction(t *testing.T) {
	t.Parallel()

	svc, apiID, authID := newAuthorizerService(t, authorizerFnURI, "")
	inv := &fakeInvoker{payload: []byte(allowPolicyAll)}
	svc.SetLambdaInvoker(inv)

	resolved := &resolvedTarget{
		resource:   &Resource{Path: "/items"},
		method:     Method{HTTPMethod: "GET", AuthorizationType: authTypeCustom, AuthorizerID: authID},
		pathParams: map[string]string{"id": "1"},
	}

	req := httptest.NewRequest(http.MethodGet, "/items?foo=bar", http.NoBody)
	req.Header.Set("Authorization", "Bearer x")

	if got := svc.authorize(req, apiID, "dev", resolved); got != 0 {
		t.Fatalf("authorize = %d, want 0", got)
	}

	if inv.gotFn != "authz-fn" {
		t.Errorf("invoked function = %q, want authz-fn", inv.gotFn)
	}

	var ev AuthorizerEvent
	if err := json.Unmarshal(inv.gotEvent, &ev); err != nil {
		t.Fatalf("authorizer event is not valid JSON: %v", err)
	}

	wantArn := "arn:aws:execute-api:us-east-1:000000000000:" + apiID + "/dev/GET/items"
	if ev.MethodArn != wantArn {
		t.Errorf("methodArn = %q, want %q", ev.MethodArn, wantArn)
	}

	if ev.Type != "REQUEST" || ev.HTTPMethod != "GET" || ev.Resource != "/items" {
		t.Errorf("unexpected event envelope: %+v", ev)
	}

	if ev.Headers["Authorization"] != "Bearer x" {
		t.Errorf("Authorization header not forwarded: %+v", ev.Headers)
	}

	if ev.QueryStringParameters["foo"] != "bar" {
		t.Errorf("query parameter not forwarded: %+v", ev.QueryStringParameters)
	}

	if ev.PathParameters["id"] != "1" {
		t.Errorf("path parameter not forwarded: %+v", ev.PathParameters)
	}
}

func TestSingleValueQuery_Empty(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/items", http.NoBody)
	if got := singleValueQuery(req); got != nil {
		t.Errorf("singleValueQuery = %v, want nil", got)
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
