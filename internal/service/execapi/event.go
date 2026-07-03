package execapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// proxyEventV1 is the API Gateway proxy integration input event for REST APIs
// and HTTP API payload format 1.0.
type proxyEventV1 struct {
	Resource                        string              `json:"resource"`
	Path                            string              `json:"path"`
	HTTPMethod                      string              `json:"httpMethod"`
	Headers                         map[string]string   `json:"headers"`
	MultiValueHeaders               map[string][]string `json:"multiValueHeaders"`
	QueryStringParameters           map[string]string   `json:"queryStringParameters"`
	MultiValueQueryStringParameters map[string][]string `json:"multiValueQueryStringParameters"`
	PathParameters                  map[string]string   `json:"pathParameters"`
	StageVariables                  map[string]string   `json:"stageVariables"`
	RequestContext                  requestContextV1    `json:"requestContext"`
	Body                            string              `json:"body"`
	IsBase64Encoded                 bool                `json:"isBase64Encoded"`
}

type requestContextV1 struct {
	ResourcePath string `json:"resourcePath"`
	HTTPMethod   string `json:"httpMethod"`
	Path         string `json:"path"`
	APIID        string `json:"apiId"`
	Stage        string `json:"stage"`
	RequestID    string `json:"requestId"`
}

// buildEventV1 builds the payload-1.0 proxy event from the HTTP request.
func buildEventV1(r *http.Request, req *Request, body []byte) ([]byte, error) {
	headers, multiHeaders := collectHeaders(r)
	query, multiQuery := collectQuery(r)

	event := proxyEventV1{
		Resource:                        req.ResourcePath,
		Path:                            req.ResourcePath,
		HTTPMethod:                      r.Method,
		Headers:                         headers,
		MultiValueHeaders:               multiHeaders,
		QueryStringParameters:           emptyToNil(query),
		MultiValueQueryStringParameters: emptyMultiToNil(multiQuery),
		PathParameters:                  emptyToNil(req.PathParameters),
		Body:                            string(body),
		RequestContext: requestContextV1{
			ResourcePath: req.ResourcePath,
			HTTPMethod:   r.Method,
			Path:         "/" + req.Stage + req.ResourcePath,
			APIID:        req.APIID,
			Stage:        req.Stage,
			RequestID:    uuid.New().String(),
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal v1 event: %w", err)
	}

	return data, nil
}

// proxyEventV2 is the HTTP API payload format 2.0 input event.
type proxyEventV2 struct {
	Version               string            `json:"version"`
	RouteKey              string            `json:"routeKey"`
	RawPath               string            `json:"rawPath"`
	RawQueryString        string            `json:"rawQueryString"`
	Headers               map[string]string `json:"headers"`
	QueryStringParameters map[string]string `json:"queryStringParameters,omitempty"`
	PathParameters        map[string]string `json:"pathParameters,omitempty"`
	StageVariables        map[string]string `json:"stageVariables,omitempty"`
	RequestContext        requestContextV2  `json:"requestContext"`
	Body                  string            `json:"body,omitempty"`
	IsBase64Encoded       bool              `json:"isBase64Encoded"`
}

type requestContextV2 struct {
	APIID     string `json:"apiId"`
	Stage     string `json:"stage"`
	RouteKey  string `json:"routeKey"`
	RequestID string `json:"requestId"`
	HTTP      httpV2 `json:"http"`
}

type httpV2 struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Protocol string `json:"protocol"`
	SourceIP string `json:"sourceIp"`
}

// buildEventV2 builds the payload-2.0 proxy event from the HTTP request.
func buildEventV2(r *http.Request, req *Request, body []byte) ([]byte, error) {
	headers, _ := collectHeaders(r)
	query, _ := collectQuery(r)

	path := "/" + req.Stage + req.ResourcePath

	event := proxyEventV2{
		Version:               "2.0",
		RouteKey:              req.RouteKey,
		RawPath:               path,
		RawQueryString:        r.URL.RawQuery,
		Headers:               headers,
		QueryStringParameters: emptyToNil(query),
		PathParameters:        emptyToNil(req.PathParameters),
		Body:                  string(body),
		RequestContext: requestContextV2{
			APIID:     req.APIID,
			Stage:     req.Stage,
			RouteKey:  req.RouteKey,
			RequestID: uuid.New().String(),
			HTTP: httpV2{
				Method:   r.Method,
				Path:     path,
				Protocol: r.Proto,
				SourceIP: r.RemoteAddr,
			},
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal v2 event: %w", err)
	}

	return data, nil
}

func collectHeaders(r *http.Request) (map[string]string, map[string][]string) {
	single := map[string]string{}
	multi := map[string][]string{}

	for k, vs := range r.Header {
		multi[k] = vs

		if len(vs) > 0 {
			single[k] = vs[len(vs)-1]
		}
	}

	return single, multi
}

func collectQuery(r *http.Request) (map[string]string, map[string][]string) {
	single := map[string]string{}
	multi := map[string][]string{}

	for k, vs := range r.URL.Query() {
		multi[k] = vs

		if len(vs) > 0 {
			single[k] = vs[len(vs)-1]
		}
	}

	return single, multi
}

func emptyToNil(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}

	return m
}

func emptyMultiToNil(m map[string][]string) map[string][]string {
	if len(m) == 0 {
		return nil
	}

	return m
}
