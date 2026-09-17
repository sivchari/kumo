//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

const (
	functionURLOrigin   = "https://example.com"
	functionURLTestKey  = "AKIAFUNCTIONURLTEST"
	headerAllowOriginIT = "Access-Control-Allow-Origin"
	contentTypeJSONIT   = "application/json"
)

// functionURLStub is an InvokeEndpoint that answers every invocation with
// payload (and functionError as X-Amz-Function-Error when set), counting calls.
type functionURLStub struct {
	url         string
	invocations atomic.Int32
}

func newFunctionURLStub(t *testing.T, payload, functionError string) *functionURLStub {
	t.Helper()

	stub := &functionURLStub{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		stub.invocations.Add(1)

		if functionError != "" {
			w.Header().Set("X-Amz-Function-Error", functionError)
		}

		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(server.Close)

	stub.url = server.URL

	return stub
}

// newFunctionURLEcho is an InvokeEndpoint that returns the received event as
// the body of a 200 envelope, so tests can inspect the event kumo built.
func newFunctionURLEcho(t *testing.T) *functionURLStub {
	t.Helper()

	stub := &functionURLStub{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.invocations.Add(1)

		event, _ := io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"statusCode": 200,
			"headers":    map[string]string{"content-type": contentTypeJSONIT, "x-fn": "echo"},
			"body":       string(event),
		})
	}))
	t.Cleanup(server.Close)

	stub.url = server.URL

	return stub
}

// createFunctionURL creates name with the stub as InvokeEndpoint plus a
// function URL, and returns the url id.
func createFunctionURL(t *testing.T, name, invokeEndpoint string, authType types.FunctionUrlAuthType, cors *types.Cors) string {
	t.Helper()

	createLambdaWithEndpoint(t, name, invokeEndpoint)

	created, err := newLambdaClient(t).CreateFunctionUrlConfig(t.Context(), &lambda.CreateFunctionUrlConfigInput{
		FunctionName: aws.String(name),
		AuthType:     authType,
		Cors:         cors,
	})
	if err != nil {
		t.Fatalf("CreateFunctionUrlConfig: %v", err)
	}

	host := strings.TrimPrefix(aws.ToString(created.FunctionUrl), "https://")

	return host[:strings.Index(host, ".")]
}

func functionURLHost(urlID string) string {
	return urlID + ".lambda-url.localhost"
}

// callFunctionURL sends a request to kumo with the function URL virtual host.
func callFunctionURL(t *testing.T, req *http.Request) (*http.Response, []byte) {
	t.Helper()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("call function url: %v", err)
	}

	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	return resp, body
}

func newFunctionURLRequest(t *testing.T, method, urlID, path string, body io.Reader) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, kumoEndpoint+path, body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	req.Host = functionURLHost(urlID)

	return req
}

func decodeEvent(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var event map[string]any
	if err := json.Unmarshal(body, &event); err != nil {
		t.Fatalf("response is not the echoed event: %v: %s", err, body)
	}

	return event
}

func TestLambda_FunctionURLInvokeBuildsPayloadV2Event(t *testing.T) {
	echo := newFunctionURLEcho(t)
	urlID := createFunctionURL(t, "test-fnurl-event", echo.url, types.FunctionUrlAuthTypeNone, nil)

	req := newFunctionURLRequest(t, http.MethodPost, urlID, "/a%2Fb/c?x=1&x=2&y=3", strings.NewReader(`{"hello":"world"}`))
	req.Header.Set("Content-Type", contentTypeJSONIT)
	req.Header.Set("Cookie", "s=1; t=2")
	req.Header.Add("X-Custom", "A")
	req.Header.Add("X-Custom", "B")

	resp, body := callFunctionURL(t, req)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("x-fn") != "echo" {
		t.Fatalf("status %d headers %v body %s", resp.StatusCode, resp.Header, body)
	}

	event := decodeEvent(t, body)

	for key, want := range map[string]any{"version": "2.0", "routeKey": "$default", "rawPath": "/a%2Fb/c", "rawQueryString": "x=1&x=2&y=3", "body": `{"hello":"world"}`, "isBase64Encoded": false} {
		if event[key] != want {
			t.Errorf("%s = %v, want %v", key, event[key], want)
		}
	}

	if cookies, _ := event["cookies"].([]any); len(cookies) != 2 || cookies[0] != "s=1" {
		t.Errorf("cookies = %v", event["cookies"])
	}

	headers, _ := event["headers"].(map[string]any)
	if headers["x-custom"] != "A,B" || headers["host"] != functionURLHost(urlID) || headers["x-forwarded-proto"] != "http" {
		t.Errorf("headers = %v", headers)
	}

	if query, _ := event["queryStringParameters"].(map[string]any); query["x"] != "1,2" || query["y"] != "3" {
		t.Errorf("queryStringParameters = %v", event["queryStringParameters"])
	}

	ctx, _ := event["requestContext"].(map[string]any)
	httpCtx, _ := ctx["http"].(map[string]any)

	if ctx["apiId"] != urlID || ctx["domainName"] != functionURLHost(urlID) || ctx["domainPrefix"] != urlID || ctx["stage"] != "$default" {
		t.Errorf("requestContext = %v", ctx)
	}

	if authorizer, present := ctx["authorizer"]; !present || authorizer != nil {
		t.Errorf("requestContext.authorizer must be null for AuthType NONE, got %v", authorizer)
	}

	if httpCtx["method"] != http.MethodPost || httpCtx["path"] != "/a/b/c" || httpCtx["sourceIp"] == "" {
		t.Errorf("requestContext.http = %v", httpCtx)
	}
}

func TestLambda_FunctionURLInvokeEncodesBinaryBodies(t *testing.T) {
	echo := newFunctionURLEcho(t)
	urlID := createFunctionURL(t, "test-fnurl-binary", echo.url, types.FunctionUrlAuthTypeNone, nil)
	raw := []byte{0x00, 0x01, 0xff}

	req := newFunctionURLRequest(t, http.MethodPut, urlID, "/upload", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/octet-stream")

	_, body := callFunctionURL(t, req)
	event := decodeEvent(t, body)

	if event["isBase64Encoded"] != true || event["body"] != base64.StdEncoding.EncodeToString(raw) {
		t.Errorf("binary body = %v (base64=%v)", event["body"], event["isBase64Encoded"])
	}
}

func TestLambda_FunctionURLInvokeMapsResponses(t *testing.T) {
	cases := []struct {
		name, payload, functionError string
		wantStatus                   int
		wantBody                     string
		wantContentType              string
	}{
		{"envelope", `{"statusCode":201,"headers":{"x-a":"1","content-type":"text/plain"},"cookies":["a=1","b=2"],"body":"aGk=","isBase64Encoded":true}`, "", http.StatusCreated, "hi", "text/plain"},
		{"json without statusCode", `{"message":"hi"}`, "", http.StatusOK, `{"message":"hi"}`, contentTypeJSONIT},
		{"non-string body", `{"statusCode":200,"body":{"a":1}}`, "", http.StatusBadGateway, "Internal Server Error", "text/plain"},
		{"invalid json", `not json`, "", http.StatusBadGateway, "Internal Server Error", "text/plain"},
		{"function error", `{"errorMessage":"boom"}`, "Unhandled", http.StatusBadGateway, "Internal Server Error", "text/plain"},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newFunctionURLStub(t, tc.payload, tc.functionError)
			urlID := createFunctionURL(t, fmt.Sprintf("test-fnurl-resp-%d", i), stub.url, types.FunctionUrlAuthTypeNone, nil)

			resp, body := callFunctionURL(t, newFunctionURLRequest(t, http.MethodGet, urlID, "/", http.NoBody))

			if resp.StatusCode != tc.wantStatus || string(body) != tc.wantBody || resp.Header.Get("Content-Type") != tc.wantContentType {
				t.Fatalf("got %d %q %q, want %d %q %q", resp.StatusCode, body, resp.Header.Get("Content-Type"), tc.wantStatus, tc.wantBody, tc.wantContentType)
			}

			if tc.name == "envelope" {
				if cookies := resp.Header.Values("Set-Cookie"); len(cookies) != 2 || resp.Header.Get("x-a") != "1" {
					t.Errorf("Set-Cookie = %v, x-a = %q", cookies, resp.Header.Get("x-a"))
				}
			}
		})
	}
}

func TestLambda_FunctionURLInvokeUnknownOrDeletedURLIsForbidden(t *testing.T) {
	resp, body := callFunctionURL(t, newFunctionURLRequest(t, http.MethodGet, strings.Repeat("z", 32), "/", http.NoBody))
	if resp.StatusCode != http.StatusForbidden || resp.Header.Get("x-amzn-ErrorType") != "AccessDeniedException" || string(body) != `{"Message":null}` {
		t.Fatalf("unknown url: %d %v %s", resp.StatusCode, resp.Header, body)
	}

	stub := newFunctionURLStub(t, `{"statusCode":200}`, "")
	name := "test-fnurl-deleted"
	urlID := createFunctionURL(t, name, stub.url, types.FunctionUrlAuthTypeNone, nil)

	if _, err := newLambdaClient(t).DeleteFunctionUrlConfig(t.Context(), &lambda.DeleteFunctionUrlConfigInput{FunctionName: aws.String(name)}); err != nil {
		t.Fatalf("DeleteFunctionUrlConfig: %v", err)
	}

	resp, _ = callFunctionURL(t, newFunctionURLRequest(t, http.MethodGet, urlID, "/", http.NoBody))
	if resp.StatusCode != http.StatusForbidden || stub.invocations.Load() != 0 {
		t.Fatalf("deleted url: %d (invocations %d)", resp.StatusCode, stub.invocations.Load())
	}
}

// signFunctionURLRequest signs req with SigV4 the way an AWS SDK would.
func signFunctionURLRequest(t *testing.T, req *http.Request) {
	t.Helper()

	payloadHash := sha256.Sum256(nil)
	creds := aws.Credentials{AccessKeyID: functionURLTestKey, SecretAccessKey: "secret"}

	if err := v4.NewSigner().SignHTTP(context.Background(), creds, req, hex.EncodeToString(payloadHash[:]), "lambda", testRegion(), time.Now()); err != nil {
		t.Fatalf("SignHTTP: %v", err)
	}
}

func TestLambda_FunctionURLInvokeAWSIAMRequiresSigV4(t *testing.T) {
	echo := newFunctionURLEcho(t)
	urlID := createFunctionURL(t, "test-fnurl-iam", echo.url, types.FunctionUrlAuthTypeAwsIam, nil)

	resp, body := callFunctionURL(t, newFunctionURLRequest(t, http.MethodGet, urlID, "/", http.NoBody))
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "Forbidden") || echo.invocations.Load() != 0 {
		t.Fatalf("unsigned: %d %s (invocations %d)", resp.StatusCode, body, echo.invocations.Load())
	}

	signed := newFunctionURLRequest(t, http.MethodGet, urlID, "/", http.NoBody)
	signFunctionURLRequest(t, signed)

	resp, body = callFunctionURL(t, signed)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("signed: %d %s", resp.StatusCode, body)
	}

	ctx, _ := decodeEvent(t, body)["requestContext"].(map[string]any)
	authorizer, _ := ctx["authorizer"].(map[string]any)
	iam, _ := authorizer["iam"].(map[string]any)

	if iam["accessKey"] != functionURLTestKey || iam["userArn"] != "arn:aws:iam::000000000000:root" {
		t.Errorf("authorizer.iam = %v", iam)
	}

	// Presigned query form. The SDK signer leaves X-Amz-Expires to the caller.
	payloadHash := sha256.Sum256(nil)
	presigned := newFunctionURLRequest(t, http.MethodGet, urlID, "/?X-Amz-Expires=300", http.NoBody)

	signedURL, _, err := v4.NewSigner().PresignHTTP(context.Background(), aws.Credentials{AccessKeyID: functionURLTestKey, SecretAccessKey: "secret"},
		presigned, hex.EncodeToString(payloadHash[:]), "lambda", testRegion(), time.Now())
	if err != nil {
		t.Fatalf("PresignHTTP: %v", err)
	}

	query := signedURL[strings.Index(signedURL, "?"):]

	resp, _ = callFunctionURL(t, newFunctionURLRequest(t, http.MethodGet, urlID, "/"+query, http.NoBody))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("presigned: %d", resp.StatusCode)
	}
}

func TestLambda_FunctionURLInvokeCORS(t *testing.T) {
	stub := newFunctionURLStub(t, `{"statusCode":200,"headers":{"access-control-allow-origin":"https://from-function"},"body":"ok"}`, "")
	urlID := createFunctionURL(t, "test-fnurl-cors", stub.url, types.FunctionUrlAuthTypeNone, &types.Cors{
		AllowOrigins:  []string{functionURLOrigin},
		AllowMethods:  []string{"GET", "POST"},
		AllowHeaders:  []string{"content-type"},
		ExposeHeaders: []string{"x-request-id"},
		MaxAge:        aws.Int32(3600),
	})

	preflight := newFunctionURLRequest(t, http.MethodOptions, urlID, "/", http.NoBody)
	preflight.Header.Set("Origin", functionURLOrigin)
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflight.Header.Set("Access-Control-Request-Headers", "content-type")

	resp, body := callFunctionURL(t, preflight)
	if resp.StatusCode != http.StatusOK || len(body) != 0 || resp.Header.Get(headerAllowOriginIT) != functionURLOrigin ||
		resp.Header.Get("Access-Control-Allow-Methods") != "GET,POST" || resp.Header.Get("Access-Control-Max-Age") != "3600" || resp.Header.Get("Vary") != "Origin" {
		t.Fatalf("preflight: %d %v %s", resp.StatusCode, resp.Header, body)
	}

	preflight.Header.Set("Access-Control-Request-Method", http.MethodDelete)

	if resp, _ = callFunctionURL(t, preflight); resp.StatusCode != http.StatusForbidden || stub.invocations.Load() != 0 {
		t.Fatalf("disallowed preflight: %d (invocations %d)", resp.StatusCode, stub.invocations.Load())
	}

	get := newFunctionURLRequest(t, http.MethodGet, urlID, "/", http.NoBody)
	get.Header.Set("Origin", functionURLOrigin)

	resp, body = callFunctionURL(t, get)
	if origins := resp.Header.Values(headerAllowOriginIT); resp.StatusCode != http.StatusOK || string(body) != "ok" || len(origins) != 2 || resp.Header.Get("Access-Control-Expose-Headers") != "x-request-id" {
		t.Fatalf("cors response: %d %v %s", resp.StatusCode, resp.Header, body)
	}

	resp, _ = callFunctionURL(t, newFunctionURLRequest(t, http.MethodGet, urlID, "/", http.NoBody))
	if origins := resp.Header.Values(headerAllowOriginIT); len(origins) != 1 || origins[0] != "https://from-function" {
		t.Fatalf("without Origin only the function's header must remain: %v", origins)
	}
}

// TestLambda_FunctionURLInvokeHealthPathReachesFunction guards the dispatch
// order: on a function URL host even /health belongs to the function.
func TestLambda_FunctionURLInvokeHealthPathReachesFunction(t *testing.T) {
	echo := newFunctionURLEcho(t)
	urlID := createFunctionURL(t, "test-fnurl-health", echo.url, types.FunctionUrlAuthTypeNone, nil)

	resp, _ := callFunctionURL(t, newFunctionURLRequest(t, http.MethodGet, urlID, "/health", http.NoBody))
	if resp.StatusCode != http.StatusOK || resp.Header.Get("x-fn") != "echo" || echo.invocations.Load() != 1 {
		t.Fatalf("/health on a function url host: %d %v (invocations %d)", resp.StatusCode, resp.Header, echo.invocations.Load())
	}

	plain, err := http.Get(kumoEndpoint + "/health")
	if err != nil {
		t.Fatalf("plain /health: %v", err)
	}

	_ = plain.Body.Close()

	if plain.StatusCode != http.StatusOK || echo.invocations.Load() != 1 {
		t.Fatalf("plain /health must stay the server health check: %d (invocations %d)", plain.StatusCode, echo.invocations.Load())
	}
}
