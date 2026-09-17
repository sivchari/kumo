package execapi

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	testURLID       = "abc123"
	testURLHost     = "abc123.lambda-url.localhost:4566"
	testAccountID   = "000000000000"
	testRemoteAddr  = "192.0.2.1:5555"
	fixedTime       = "2020-03-12T19:03:58.390Z"
	contentTypeText = "text/plain"
	jsonNull        = "null"
)

func fixedNow(t *testing.T) time.Time {
	t.Helper()

	now, err := time.Parse(time.RFC3339Nano, fixedTime)
	if err != nil {
		t.Fatalf("parse fixed time: %v", err)
	}

	return now
}

// buildEvent runs the function URL event builder and decodes the result.
func buildEvent(t *testing.T, r *http.Request, body []byte, req *FunctionURLRequest) map[string]any {
	t.Helper()

	raw, err := buildFunctionURLEvent(r, body, req, fixedNow(t))
	if err != nil {
		t.Fatalf("buildFunctionURLEvent() error = %v", err)
	}

	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("event is not JSON: %v\n%s", err, raw)
	}

	return event
}

func newURLRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody)
	r.RemoteAddr = testRemoteAddr

	return r
}

func TestBuildFunctionURLEvent_Shape(t *testing.T) {
	t.Parallel()

	r := newURLRequest(t, http.MethodGet, "http://"+testURLHost+"/a%2Fb/c?x=1&x=2&y=3")
	r.Header.Add("X-Custom", "A")
	r.Header.Add("X-Custom", "B")
	r.Header.Set("Cookie", "s=1; t=2")
	r.Header.Set("User-Agent", "ua/1.0")
	r.Header.Set("X-Forwarded-For", "203.0.113.9") // spoofed: must be overwritten

	event := buildEvent(t, r, nil, &FunctionURLRequest{URLID: testURLID, AccountID: testAccountID})

	want := map[string]any{"version": "2.0", "routeKey": defaultStage, "rawPath": "/a%2Fb/c", "rawQueryString": "x=1&x=2&y=3", "isBase64Encoded": false}
	for key, value := range want {
		if event[key] != value {
			t.Errorf("%s = %v, want %v", key, event[key], value)
		}
	}

	for _, key := range []string{"pathParameters", "stageVariables"} {
		if value, present := event[key]; !present || value != nil {
			t.Errorf("%s must be present as null, got %v (present=%v)", key, value, present)
		}
	}

	if _, present := event["body"]; present {
		t.Error("an empty body must be omitted")
	}

	cookies, _ := event["cookies"].([]any)
	if len(cookies) != 2 || cookies[0] != "s=1" || cookies[1] != "t=2" {
		t.Errorf("cookies = %v", event["cookies"])
	}

	query, _ := event["queryStringParameters"].(map[string]any)
	if query["x"] != "1,2" || query["y"] != "3" {
		t.Errorf("queryStringParameters = %v", query)
	}

	headers, _ := event["headers"].(map[string]any)
	wantHeaders := map[string]any{"x-custom": "A,B", "host": testURLHost, "user-agent": "ua/1.0", "x-forwarded-for": "192.0.2.1", "x-forwarded-proto": "http", "x-forwarded-port": "4566"}

	for key, value := range wantHeaders {
		if headers[key] != value {
			t.Errorf("headers[%s] = %v, want %v", key, headers[key], value)
		}
	}

	if _, present := headers["cookie"]; present {
		t.Error("the cookie header must move into cookies")
	}
}

func TestBuildFunctionURLEvent_RequestContext(t *testing.T) {
	t.Parallel()

	r := newURLRequest(t, http.MethodPost, "http://"+testURLHost+"/a%2Fb")
	r.Header.Set("User-Agent", "ua/1.0")

	event := buildEvent(t, r, nil, &FunctionURLRequest{URLID: testURLID, AccountID: testAccountID})

	ctx, _ := event["requestContext"].(map[string]any)
	wantCtx := map[string]any{
		"accountId": testAccountID, "apiId": testURLID, "domainName": "abc123.lambda-url.localhost", "domainPrefix": testURLID,
		"routeKey": "$default", "stage": "$default", "time": "12/Mar/2020:19:03:58 +0000", "timeEpoch": float64(1584039838390),
	}

	for key, value := range wantCtx {
		if ctx[key] != value {
			t.Errorf("requestContext.%s = %v, want %v", key, ctx[key], value)
		}
	}

	for _, key := range []string{"authentication", "authorizer"} {
		if value, present := ctx[key]; !present || value != nil {
			t.Errorf("requestContext.%s must be present as null, got %v", key, value)
		}
	}

	if requestID, _ := ctx["requestId"].(string); requestID == "" {
		t.Error("requestId must be set")
	}

	httpCtx, _ := ctx["http"].(map[string]any)
	wantHTTP := map[string]any{"method": "POST", "path": "/a/b", "protocol": "HTTP/1.1", "sourceIp": "192.0.2.1", "userAgent": "ua/1.0"}

	for key, value := range wantHTTP {
		if httpCtx[key] != value {
			t.Errorf("requestContext.http.%s = %v, want %v", key, httpCtx[key], value)
		}
	}
}

func TestBuildFunctionURLEvent_IAMAuthorizerAndTLS(t *testing.T) {
	t.Parallel()

	r := newURLRequest(t, http.MethodGet, "https://abc123.lambda-url.us-east-1.on.aws/")
	r.RemoteAddr = "[2001:db8::1]:443"
	r.TLS = &tls.ConnectionState{}

	iam := &FunctionURLIAMIdentity{AccessKey: "AKIATEST", AccountID: testAccountID, CallerID: testAccountID, UserArn: "arn:aws:iam::000000000000:root", UserID: testAccountID}
	event := buildEvent(t, r, nil, &FunctionURLRequest{URLID: testURLID, AccountID: testAccountID, Authorizer: &FunctionURLAuthorizer{IAM: iam}})

	headers, _ := event["headers"].(map[string]any)
	if headers["x-forwarded-proto"] != "https" || headers["x-forwarded-port"] != "443" || headers["x-forwarded-for"] != "2001:db8::1" {
		t.Errorf("forwarded headers = %v", headers)
	}

	ctx, _ := event["requestContext"].(map[string]any)
	httpCtx, _ := ctx["http"].(map[string]any)

	if httpCtx["sourceIp"] != "2001:db8::1" {
		t.Errorf("sourceIp = %v", httpCtx["sourceIp"])
	}

	authorizer, _ := ctx["authorizer"].(map[string]any)
	got, _ := authorizer["iam"].(map[string]any)

	if got["accessKey"] != "AKIATEST" || got["userArn"] != "arn:aws:iam::000000000000:root" {
		t.Errorf("authorizer.iam = %v", got)
	}

	for _, key := range []string{"cognitoIdentity", "principalOrgId"} {
		if value, present := got[key]; !present || value != nil {
			t.Errorf("authorizer.iam.%s must be present as null, got %v", key, value)
		}
	}
}

func TestBuildFunctionURLEvent_Body(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		contentType string
		encoding    string
		body        []byte
		wantBody    string
		wantBase64  bool
	}{
		{"json text", "Application/JSON; charset=utf-8", "", []byte(`{"a":1}`), `{"a":1}`, false},
		{"form text", "application/x-www-form-urlencoded", "", []byte("a=1"), "a=1", false},
		{"structured suffix", "application/vnd.api+json", "", []byte(`{}`), `{}`, false},
		{"octet stream", "application/octet-stream", "", []byte{0xff, 0x00}, "/wA=", true},
		{"missing content type", "", "", []byte("hi"), "aGk=", true},
		{"gzip encoded text", contentTypeText, "gzip", []byte("hi"), "aGk=", true},
		{"invalid utf-8 text", contentTypeText, "", []byte{0xff, 0xfe}, "//4=", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newURLRequest(t, http.MethodPost, "http://"+testURLHost+"/")
			if tc.contentType != "" {
				r.Header.Set("Content-Type", tc.contentType)
			}

			if tc.encoding != "" {
				r.Header.Set("Content-Encoding", tc.encoding)
			}

			event := buildEvent(t, r, tc.body, &FunctionURLRequest{URLID: testURLID, AccountID: testAccountID})

			if event["body"] != tc.wantBody || event["isBase64Encoded"] != tc.wantBase64 {
				t.Fatalf("body = %v (base64=%v), want %q (%v)", event["body"], event["isBase64Encoded"], tc.wantBody, tc.wantBase64)
			}
		})
	}
}

func TestWriteFunctionURLResponse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		payload     string
		wantStatus  int
		wantBody    string
		wantContent string
	}{
		{"envelope", `{"statusCode":201,"headers":{"x-a":"1","content-type":"text/plain"},"body":"hello","isBase64Encoded":false}`, 201, "hello", contentTypeText},
		{"base64 body", `{"statusCode":200,"body":"aGk=","isBase64Encoded":true}`, 200, "hi", functionURLContentJSON},
		{"object without statusCode", `{"message":"hi"}`, 200, `{"message":"hi"}`, functionURLContentJSON},
		{"string", `"Hello"`, 200, `"Hello"`, functionURLContentJSON},
		{"array", `[1,2]`, 200, `[1,2]`, functionURLContentJSON},
		{"number", `42`, 200, `42`, functionURLContentJSON},
		{jsonNull, jsonNull, 200, jsonNull, functionURLContentJSON},
		{"invalid json", `not json`, 502, "", ""},
		{"status below range", `{"statusCode":99}`, 502, "", ""},
		{"status above range", `{"statusCode":600}`, 502, "", ""},
		{"informational status cannot be final", `{"statusCode":150}`, 502, "", ""},
		{"string status", `{"statusCode":"200"}`, 502, "", ""},
		{"fractional status", `{"statusCode":200.5}`, 502, "", ""},
		{"headers wrong type", `{"statusCode":200,"headers":["x"]}`, 502, "", ""},
		{"header value wrong type", `{"statusCode":200,"headers":{"a":1}}`, 502, "", ""},
		{"cookies wrong type", `{"statusCode":200,"cookies":"a=1"}`, 502, "", ""},
		{"non-string body", `{"statusCode":200,"body":{"a":1}}`, 502, "", ""},
		{"isBase64Encoded wrong type", `{"statusCode":200,"body":"x","isBase64Encoded":"yes"}`, 502, "", ""},
		{"bad base64", `{"statusCode":200,"body":"%%%","isBase64Encoded":true}`, 502, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			writeFunctionURLResponse(rec, []byte(tc.payload))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tc.wantStatus, rec.Body.String())
			}

			if tc.wantStatus == http.StatusBadGateway {
				return
			}

			if rec.Body.String() != tc.wantBody || rec.Header().Get("Content-Type") != tc.wantContent {
				t.Fatalf("body %q content-type %q, want %q %q", rec.Body.String(), rec.Header().Get("Content-Type"), tc.wantBody, tc.wantContent)
			}
		})
	}
}

func TestWriteFunctionURLResponse_CookiesAndHeaders(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	writeFunctionURLResponse(rec, []byte(`{"statusCode":200,"headers":{"Set-Cookie":"h=1","X-A":"1"},"cookies":["a=1","b=2"],"body":""}`))

	if got := rec.Header().Values("Set-Cookie"); len(got) != 3 {
		t.Fatalf("Set-Cookie = %v, want the header value plus both cookies", got)
	}

	if rec.Header().Get("X-A") != "1" || rec.Header().Get("Content-Type") != functionURLContentJSON {
		t.Fatalf("headers = %v", rec.Header())
	}
}

func TestInvokeFunction_ReportsFunctionErrors(t *testing.T) {
	t.Parallel()

	lambda := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/functions/boom/invocations"):
			w.Header().Set("X-Amz-Function-Error", "Unhandled")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"errorMessage":"boom"}`))
		case strings.HasSuffix(r.URL.Path, "/functions/down/invocations"):
			w.WriteHeader(http.StatusBadGateway)
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	t.Cleanup(lambda.Close)

	ok, err := invokeFunction(t.Context(), lambda.URL, "fine", []byte(`{}`))
	if err != nil || ok.FunctionError != "" || string(ok.Body) != `{"ok":true}` {
		t.Fatalf("fine = %+v, %v", ok, err)
	}

	failed, err := invokeFunction(t.Context(), lambda.URL, "boom", []byte(`{}`))
	if err != nil || failed.FunctionError != "Unhandled" {
		t.Fatalf("boom = %+v, %v", failed, err)
	}

	if _, err := invokeFunction(t.Context(), lambda.URL, "down", []byte(`{}`)); err == nil {
		t.Fatal("a non-200 invoke response must be an error")
	}
}

func TestServeFunctionURL_MapsFunctionErrorsTo502(t *testing.T) {
	t.Parallel()

	var received []byte

	lambda := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read event: %v", err)
		}

		received = body

		if strings.HasSuffix(r.URL.Path, "/functions/boom/invocations") {
			w.Header().Set("X-Amz-Function-Error", "Unhandled")
		}

		_, _ = w.Write([]byte(`{"statusCode":204}`))
	}))
	t.Cleanup(lambda.Close)

	r := newURLRequest(t, http.MethodGet, "http://"+testURLHost+"/ping")
	rec := httptest.NewRecorder()
	ServeFunctionURL(rec, r, lambda.URL, "fine", &FunctionURLRequest{URLID: testURLID, AccountID: testAccountID})

	if rec.Code != http.StatusNoContent || !strings.Contains(string(received), `"apiId":"abc123"`) {
		t.Fatalf("fine: status %d, event %s", rec.Code, received)
	}

	rec = httptest.NewRecorder()
	ServeFunctionURL(rec, r, lambda.URL, "boom", &FunctionURLRequest{URLID: testURLID, AccountID: testAccountID})

	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "Internal Server Error") {
		t.Fatalf("boom: status %d body %q", rec.Code, rec.Body.String())
	}
}
