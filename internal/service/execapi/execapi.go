// Package execapi provides the shared execute-api data-plane engine used by
// the API Gateway v1 (REST) and v2 (HTTP) services: it builds the Lambda
// proxy-integration event, invokes the function via kumo's own Lambda invoke
// endpoint (delegating execution to the function's InvokeEndpoint), unwraps
// the proxy response envelope, and forwards HTTP_PROXY integrations.
package execapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Integration types.
const (
	TypeAWSProxy  = "AWS_PROXY"
	TypeHTTPProxy = "HTTP_PROXY"
)

// payloadFormat2 is the HTTP API payload format version with the simplified
// request/response shape and optional statusCode.
const payloadFormat2 = "2.0"

// Target is the resolved integration to dispatch to.
type Target struct {
	Type string
	URI  string
	// PayloadFormatVersion selects the Lambda event/response shape: "2.0"
	// uses the HTTP API v2 format; anything else uses the 1.0 / REST proxy
	// format.
	PayloadFormatVersion string
}

// Request carries the metadata needed to build the Lambda event.
type Request struct {
	BaseURL        string
	APIID          string
	Stage          string
	ResourcePath   string // matched resource path template (v1) or route path
	RouteKey       string // v2 route key, e.g. "GET /items" or "$default"
	PathParameters map[string]string
}

// Dispatch resolves the integration target and writes the HTTP response. It is
// the single entry point shared by the v1 and v2 execute-api handlers.
func Dispatch(w http.ResponseWriter, r *http.Request, t Target, req *Request) {
	switch t.Type {
	case TypeAWSProxy:
		invokeLambda(w, r, t, req)
	case TypeHTTPProxy:
		forwardHTTP(w, r, t.URI)
	default:
		// AWS / HTTP (non-proxy) and MOCK require VTL mapping templates,
		// which kumo execute-api does not yet emulate.
		slog.Warn("execute-api: unsupported integration type", "type", t.Type, "apiId", req.APIID)
		writeError(w, http.StatusInternalServerError)
	}
}

// invokeLambda builds the proxy event, invokes the Lambda, and writes the
// unwrapped response.
func invokeLambda(w http.ResponseWriter, r *http.Request, t Target, req *Request) {
	name := lambdaFunctionNameFromURI(t.URI)
	if name == "" {
		writeError(w, http.StatusInternalServerError)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError)

		return
	}

	var event []byte
	if t.PayloadFormatVersion == payloadFormat2 {
		event, err = buildEventV2(r, req, body)
	} else {
		event, err = buildEventV1(r, req, body)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError)

		return
	}

	respBody, err := invoke(r, req.BaseURL, name, event)
	if err != nil {
		slog.Error("execute-api: lambda invoke failed", "function", name, "error", err)
		writeError(w, http.StatusInternalServerError)

		return
	}

	writeProxyResponse(w, respBody, t.PayloadFormatVersion == payloadFormat2)
}

// invoke POSTs the event to kumo's own Lambda invoke endpoint and returns the
// function's response body.
func invoke(r *http.Request, baseURL, name string, event []byte) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/lambda/2015-03-31/functions/%s/invocations", baseURL, name)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(event))
	if err != nil {
		return nil, fmt.Errorf("build invoke request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Amz-Invocation-Type", "RequestResponse")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("invoke lambda: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read invoke response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lambda invoke returned status %d", resp.StatusCode)
	}

	return respBody, nil
}

// proxyResponseEnvelope is the response a Lambda proxy integration returns.
// StatusCode is a pointer to distinguish "absent" from zero.
type proxyResponseEnvelope struct {
	StatusCode        *int                `json:"statusCode"`
	Headers           map[string]string   `json:"headers"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders"`
	Body              string              `json:"body"`
	IsBase64Encoded   bool                `json:"isBase64Encoded"`
}

// writeProxyResponse unwraps the Lambda proxy response envelope and writes it.
//
// For 1.0 a missing statusCode is a malformed response -> 502 (matching real
// API Gateway). For 2.0, per the HTTP API spec, a response without statusCode
// that is valid JSON is returned as a 200 with that JSON as the body.
func writeProxyResponse(w http.ResponseWriter, respBody []byte, v2 bool) {
	var env proxyResponseEnvelope

	err := json.Unmarshal(respBody, &env)
	if err != nil || env.StatusCode == nil {
		if v2 && err == nil {
			// Valid JSON without statusCode: 2.0 returns it as the body.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(respBody)

			return
		}

		writeError(w, http.StatusBadGateway)

		return
	}

	out := []byte(env.Body)

	if env.IsBase64Encoded {
		decoded, derr := base64.StdEncoding.DecodeString(env.Body)
		if derr != nil {
			writeError(w, http.StatusBadGateway)

			return
		}

		out = decoded
	}

	for k, v := range env.Headers {
		w.Header().Set(k, v)
	}

	for k, vs := range env.MultiValueHeaders {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(*env.StatusCode)
	_, _ = w.Write(out)
}

// forwardHTTP proxies the request verbatim to an HTTP_PROXY integration URI.
func forwardHTTP(w http.ResponseWriter, r *http.Request, uri string) {
	if uri == "" {
		writeError(w, http.StatusInternalServerError)

		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, uri, r.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway)

		return
	}

	for k, vs := range r.Header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway)

		return
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway)

		return
	}

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

// lambdaFunctionNameFromURI extracts the Lambda function name from an
// AWS_PROXY integration URI. It accepts both the v1 REST form
//
//	arn:aws:apigateway:{region}:lambda:path/2015-03-31/functions/{lambdaArn}/invocations
//
// and the v2 HTTP API form, which is the bare Lambda function ARN
//
//	arn:aws:lambda:{region}:{account}:function:{name}[:qualifier]
func lambdaFunctionNameFromURI(uri string) string {
	const (
		marker = "/functions/"
		suffix = "/invocations"
	)

	rest := uri

	if start := strings.Index(uri, marker); start >= 0 {
		rest = uri[start+len(marker):]
		if end := strings.Index(rest, suffix); end >= 0 {
			rest = rest[:end]
		}
	}

	// rest is the Lambda ARN (or bare name).
	if strings.Contains(rest, ":function:") {
		parts := strings.Split(rest, ":")
		if len(parts) >= 7 {
			return parts[6]
		}
	}

	return rest
}

// writeError writes an API Gateway style error body.
func writeError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Internal server error"})
}
