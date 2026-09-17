package execapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Lambda function URL data plane: the payload format 2.0 event Lambda builds
// for a function URL and the response mapping it applies. See
// https://docs.aws.amazon.com/lambda/latest/dg/urls-invocation.html
const (
	functionURLRouteKey    = "$default"
	functionURLTimeLayout  = "02/Jan/2006:15:04:05 -0700"
	functionURLContentJSON = "application/json"
	msgInternalServerError = "Internal Server Error"

	headerCookie         = "cookie"
	headerForwardedFor   = "x-forwarded-for"
	headerForwardedProto = "x-forwarded-proto"
	headerForwardedPort  = "x-forwarded-port"
	headerContentType    = "Content-Type"
)

// FunctionURLRequest carries the function URL context the event needs.
type FunctionURLRequest struct {
	URLID     string
	AccountID string
	// Authorizer is nil for AuthType NONE, which Lambda renders as
	// requestContext.authorizer: null.
	Authorizer *FunctionURLAuthorizer
}

// FunctionURLAuthorizer is requestContext.authorizer of an AWS_IAM function URL.
type FunctionURLAuthorizer struct {
	IAM *FunctionURLIAMIdentity `json:"iam,omitempty"`
}

// FunctionURLIAMIdentity is requestContext.authorizer.iam.
type FunctionURLIAMIdentity struct {
	AccessKey       string  `json:"accessKey"`
	AccountID       string  `json:"accountId"`
	CallerID        string  `json:"callerId"`
	CognitoIdentity *string `json:"cognitoIdentity"`
	PrincipalOrgID  *string `json:"principalOrgId"`
	UserArn         string  `json:"userArn"`
	UserID          string  `json:"userId"`
}

type functionURLEvent struct {
	Version               string                    `json:"version"`
	RouteKey              string                    `json:"routeKey"`
	RawPath               string                    `json:"rawPath"`
	RawQueryString        string                    `json:"rawQueryString"`
	Cookies               []string                  `json:"cookies,omitempty"`
	Headers               map[string]string         `json:"headers"`
	QueryStringParameters map[string]string         `json:"queryStringParameters,omitempty"`
	RequestContext        functionURLRequestContext `json:"requestContext"`
	Body                  string                    `json:"body,omitempty"`
	PathParameters        *struct{}                 `json:"pathParameters"`
	IsBase64Encoded       bool                      `json:"isBase64Encoded"`
	StageVariables        *struct{}                 `json:"stageVariables"`
}

type functionURLRequestContext struct {
	AccountID      string                 `json:"accountId"`
	APIID          string                 `json:"apiId"`
	Authentication *struct{}              `json:"authentication"`
	Authorizer     *FunctionURLAuthorizer `json:"authorizer"`
	DomainName     string                 `json:"domainName"`
	DomainPrefix   string                 `json:"domainPrefix"`
	HTTP           functionURLHTTP        `json:"http"`
	RequestID      string                 `json:"requestId"`
	RouteKey       string                 `json:"routeKey"`
	Stage          string                 `json:"stage"`
	Time           string                 `json:"time"`
	TimeEpoch      int64                  `json:"timeEpoch"`
}

type functionURLHTTP struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Protocol  string `json:"protocol"`
	SourceIP  string `json:"sourceIp"`
	UserAgent string `json:"userAgent"`
}

// ServeFunctionURL invokes functionName synchronously with the function URL
// event built from r and writes the response the way Lambda does. Runtime and
// function failures are reported as 502.
func ServeFunctionURL(w http.ResponseWriter, r *http.Request, baseURL, functionName string, req *FunctionURLRequest) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeFunctionURLError(w)

		return
	}

	event, err := buildFunctionURLEvent(r, body, req, time.Now())
	if err != nil {
		writeFunctionURLError(w)

		return
	}

	result, err := invokeFunction(r.Context(), baseURL, functionName, event)
	if err != nil || result.FunctionError != "" {
		slog.Error("function url: invocation failed", "function", functionName, "urlId", req.URLID, "error", err)
		writeFunctionURLError(w)

		return
	}

	writeFunctionURLResponse(w, result.Body)
}

// buildFunctionURLEvent renders the payload format 2.0 event for a function
// URL request. now is injected so tests can pin time and timeEpoch.
func buildFunctionURLEvent(r *http.Request, body []byte, req *FunctionURLRequest, now time.Time) ([]byte, error) {
	host := strings.ToLower(r.Host)
	sourceIP := remoteIP(r.RemoteAddr)
	proto, port := forwardedProtoAndPort(r)
	headers, cookies := functionURLHeaders(r.Header, host, sourceIP, proto, port)

	event := functionURLEvent{
		Version:               "2.0",
		RouteKey:              functionURLRouteKey,
		RawPath:               r.URL.EscapedPath(),
		RawQueryString:        r.URL.RawQuery,
		Cookies:               cookies,
		Headers:               headers,
		QueryStringParameters: joinedQuery(r.URL.Query()),
		RequestContext: functionURLRequestContext{
			AccountID:    req.AccountID,
			APIID:        req.URLID,
			Authorizer:   req.Authorizer,
			DomainName:   hostWithoutPort(host),
			DomainPrefix: req.URLID,
			HTTP: functionURLHTTP{
				Method:    r.Method,
				Path:      r.URL.Path,
				Protocol:  r.Proto,
				SourceIP:  sourceIP,
				UserAgent: r.UserAgent(),
			},
			RequestID: uuid.New().String(),
			RouteKey:  functionURLRouteKey,
			Stage:     functionURLRouteKey,
			Time:      now.UTC().Format(functionURLTimeLayout),
			TimeEpoch: now.UnixMilli(),
		},
	}

	event.Body, event.IsBase64Encoded = functionURLBody(r.Header, body)

	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal function url event: %w", err)
	}

	return data, nil
}

// functionURLHeaders lower-cases the request headers, joins repeated values
// with commas, moves cookies into their own list, adds host and overwrites the
// x-forwarded-* headers with values derived from the connection so clients
// cannot spoof them.
func functionURLHeaders(header http.Header, host, sourceIP, proto, port string) (map[string]string, []string) {
	headers := make(map[string]string, len(header)+4)

	var cookies []string

	for name, values := range header {
		lower := strings.ToLower(name)
		if lower == headerCookie {
			cookies = append(cookies, splitCookies(values)...)

			continue
		}

		headers[lower] = strings.Join(values, ",")
	}

	headers["host"] = host
	headers[headerForwardedFor] = sourceIP
	headers[headerForwardedProto] = proto
	headers[headerForwardedPort] = port

	return headers, cookies
}

func splitCookies(values []string) []string {
	var cookies []string

	for _, value := range values {
		for _, cookie := range strings.Split(value, ";") {
			if cookie = strings.TrimSpace(cookie); cookie != "" {
				cookies = append(cookies, cookie)
			}
		}
	}

	return cookies
}

func joinedQuery(values url.Values) map[string]string {
	if len(values) == 0 {
		return nil
	}

	joined := make(map[string]string, len(values))
	for key, vs := range values {
		joined[key] = strings.Join(vs, ",")
	}

	return joined
}

// remoteIP strips the port from a RemoteAddr, keeping IPv6 literals intact.
func remoteIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}

	return addr
}

func hostWithoutPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}

	return host
}

// forwardedProtoAndPort derives x-forwarded-proto / x-forwarded-port from the
// connection: https when the request arrived over TLS, the Host port when
// present and the scheme default otherwise.
func forwardedProtoAndPort(r *http.Request) (string, string) {
	proto, port := "http", "80"
	if r.TLS != nil {
		proto, port = "https", "443"
	}

	if _, p, err := net.SplitHostPort(r.Host); err == nil && p != "" {
		port = p
	}

	return proto, port
}

// functionURLBody renders the request body: verbatim for textual content,
// base64 for anything else, and absent when empty.
func functionURLBody(header http.Header, body []byte) (string, bool) {
	if len(body) == 0 {
		return "", false
	}

	if isTextualBody(header, body) {
		return string(body), false
	}

	return base64.StdEncoding.EncodeToString(body), true
}

// isTextualBody reports whether the body can travel as a plain string: a
// textual media type, no content encoding and valid UTF-8. A missing content
// type is treated as binary.
func isTextualBody(header http.Header, body []byte) bool {
	if encoding := header.Get("Content-Encoding"); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(header.Get(headerContentType))
	if err != nil {
		return false
	}

	return isTextualMediaType(strings.ToLower(mediaType)) && utf8.Valid(body)
}

func isTextualMediaType(mediaType string) bool {
	switch mediaType {
	case "application/json", "application/xml", "application/javascript", "application/x-www-form-urlencoded":
		return true
	}

	return strings.HasPrefix(mediaType, "text/") || strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml")
}

// functionURLResponse is a parsed response envelope.
type functionURLResponse struct {
	status  int
	headers map[string]string
	cookies []string
	body    []byte
}

// writeFunctionURLResponse maps a function's payload to the HTTP response the
// way Lambda does: valid JSON without statusCode is returned as a 200 JSON
// body, an envelope is unwrapped, anything malformed is a 502.
func writeFunctionURLResponse(w http.ResponseWriter, payload []byte) {
	if !json.Valid(payload) {
		writeFunctionURLError(w)

		return
	}

	var envelope map[string]json.RawMessage
	if json.Unmarshal(payload, &envelope) != nil || envelope == nil {
		writeJSONPassthrough(w, payload)

		return
	}

	statusRaw, hasStatus := envelope["statusCode"]
	if !hasStatus {
		writeJSONPassthrough(w, payload)

		return
	}

	resp, err := parseFunctionURLEnvelope(statusRaw, envelope)
	if err != nil {
		writeFunctionURLError(w)

		return
	}

	for name, value := range resp.headers {
		w.Header().Add(name, value)
	}

	for _, cookie := range resp.cookies {
		w.Header().Add("Set-Cookie", cookie)
	}

	if w.Header().Get(headerContentType) == "" {
		w.Header().Set(headerContentType, functionURLContentJSON)
	}

	w.WriteHeader(resp.status)
	_, _ = w.Write(resp.body)
}

func writeJSONPassthrough(w http.ResponseWriter, payload []byte) {
	w.Header().Set(headerContentType, functionURLContentJSON)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func writeFunctionURLError(w http.ResponseWriter) {
	w.Header().Set(headerContentType, "text/plain")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusBadGateway)
	_, _ = io.WriteString(w, msgInternalServerError)
}

var errMalformedEnvelope = errors.New("malformed function url response envelope")

// parseFunctionURLEnvelope validates a response envelope. Every field must
// have its documented type; the body is a string (base64 when flagged).
func parseFunctionURLEnvelope(statusRaw json.RawMessage, envelope map[string]json.RawMessage) (*functionURLResponse, error) {
	status, err := parseFunctionURLStatus(statusRaw)
	if err != nil {
		return nil, err
	}

	resp := &functionURLResponse{status: status, headers: map[string]string{}}

	if err := unmarshalEnvelopeField(envelope, "headers", &resp.headers); err != nil {
		return nil, err
	}

	if err := unmarshalEnvelopeField(envelope, "cookies", &resp.cookies); err != nil {
		return nil, err
	}

	var (
		body     string
		isBase64 bool
	)

	if err := unmarshalEnvelopeField(envelope, "body", &body); err != nil {
		return nil, err
	}

	if err := unmarshalEnvelopeField(envelope, "isBase64Encoded", &isBase64); err != nil {
		return nil, err
	}

	resp.body = []byte(body)

	if isBase64 {
		if resp.body, err = base64.StdEncoding.DecodeString(body); err != nil {
			return nil, errMalformedEnvelope
		}
	}

	return resp, nil
}

// parseFunctionURLStatus accepts an integer status a server can send as its
// final response (200-599); 1xx cannot be final in net/http.
func parseFunctionURLStatus(raw json.RawMessage) (int, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] == '"' {
		return 0, errMalformedEnvelope
	}

	var number json.Number
	if json.Unmarshal(trimmed, &number) != nil {
		return 0, errMalformedEnvelope
	}

	status, err := number.Int64()
	if err != nil || status < http.StatusOK || status > 599 {
		return 0, errMalformedEnvelope
	}

	return int(status), nil
}

// unmarshalEnvelopeField decodes an optional envelope field into out; a
// missing or null field leaves out untouched, a wrong type is an error.
func unmarshalEnvelopeField(envelope map[string]json.RawMessage, key string, out any) error {
	raw, ok := envelope[key]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return errMalformedEnvelope
	}

	return nil
}
