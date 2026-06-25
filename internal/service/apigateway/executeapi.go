package apigateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service/execapi"
)

// maxExecuteResources bounds how many resources are scanned when resolving a
// request path to a resource.
const maxExecuteResources = 10000

// HandleExecuteAPI implements service.ExecuteAPIHandler for REST APIs. It
// returns false when apiID is not a REST API managed by this service, so the
// router can try the v2 service.
func (s *Service) HandleExecuteAPI(w http.ResponseWriter, r *http.Request, apiID, invokePath string) bool {
	if _, err := s.storage.GetRestAPI(r.Context(), apiID); err != nil {
		return false
	}

	// invokePath is /{stage}/{resource-path}; the first segment is the stage.
	segs := execapi.SplitPath(invokePath)
	if len(segs) == 0 {
		writeExecuteError(w, http.StatusForbidden, "Missing Authentication Token")

		return true
	}

	stage := segs[0]
	reqPath := "/" + strings.Join(segs[1:], "/")

	resolved, status := s.resolveExecuteTarget(r, apiID, stage, reqPath)
	if status != 0 {
		writeExecuteError(w, status, executeErrorMessage(status))

		return true
	}

	// CUSTOM methods run a REQUEST-type Lambda authorizer before the
	// integration; only an Allow lets the request continue.
	if resolved.method.AuthorizationType == authTypeCustom {
		if status := s.authorize(r, apiID, stage, &resolved); status != 0 {
			writeExecuteError(w, status, authorizerErrorMessage(status))

			return true
		}
	}

	execapi.Dispatch(w, r,
		execapi.Target{
			Type: resolved.method.MethodIntegration.Type,
			URI:  resolved.method.MethodIntegration.URI,
		},
		&execapi.Request{
			BaseURL:        s.baseURLOrDefault(),
			APIID:          apiID,
			Stage:          stage,
			ResourcePath:   resolved.resource.Path,
			PathParameters: resolved.pathParams,
		},
	)

	return true
}

// resolvedTarget is the resource/method/params resolved for an execute-api
// request.
type resolvedTarget struct {
	resource   *Resource
	method     Method
	pathParams map[string]string
}

// resolveExecuteTarget validates the stage and resolves the resource, method,
// and path parameters. A non-zero status is the HTTP error to return.
func (s *Service) resolveExecuteTarget(r *http.Request, apiID, stage, reqPath string) (resolvedTarget, int) {
	if _, err := s.storage.GetStage(r.Context(), apiID, stage); err != nil {
		return resolvedTarget{}, http.StatusNotFound
	}

	resources, _, err := s.storage.GetResources(r.Context(), apiID, maxExecuteResources, "")
	if err != nil {
		return resolvedTarget{}, http.StatusNotFound
	}

	resource, pathParams, ok := matchResource(resources, reqPath)
	if !ok {
		return resolvedTarget{}, http.StatusForbidden
	}

	method, ok := matchMethod(resource, r.Method)
	if !ok {
		return resolvedTarget{}, http.StatusForbidden
	}

	if method.MethodIntegration == nil {
		return resolvedTarget{}, http.StatusInternalServerError
	}

	return resolvedTarget{resource: resource, method: method, pathParams: pathParams}, 0
}

// executeErrorMessage maps an execute-api error status to its AWS message.
func executeErrorMessage(status int) string {
	switch status {
	case http.StatusNotFound:
		return "Not Found"
	case http.StatusInternalServerError:
		return "Internal server error"
	default:
		return "Missing Authentication Token"
	}
}

// authorizerErrorMessage maps an authorizer rejection status to its AWS message.
func authorizerErrorMessage(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusInternalServerError:
		return "Internal server error"
	default:
		return "User is not authorized to access this resource"
	}
}

// baseURLOrDefault returns the configured base URL, defaulting to the local
// kumo server when unset.
func (s *Service) baseURLOrDefault() string {
	if s.baseURL == "" {
		return execapi.DefaultBaseURL
	}

	return s.baseURL
}

// matchResource finds the most specific resource whose Path matches reqPath,
// returning any captured path parameters. Specificity prefers literal
// segments over {param} and greedy {proxy+} segments.
func matchResource(resources []*Resource, reqPath string) (*Resource, map[string]string, bool) {
	reqSegs := execapi.SplitPath(reqPath)

	var (
		best      *Resource
		bestVals  map[string]string
		bestScore = -1
	)

	for _, res := range resources {
		vals, score, ok := execapi.MatchPath(res.Path, reqSegs)
		if !ok {
			continue
		}

		if score > bestScore {
			best = res
			bestVals = vals
			bestScore = score
		}
	}

	if best == nil {
		return nil, nil, false
	}

	return best, bestVals, true
}

// matchMethod returns the resource method for the request method, falling back
// to an "ANY" method when defined.
func matchMethod(resource *Resource, httpMethod string) (Method, bool) {
	if m, ok := resource.ResourceMethods[httpMethod]; ok {
		return m, true
	}

	if m, ok := resource.ResourceMethods["ANY"]; ok {
		return m, true
	}

	return Method{}, false
}

// writeExecuteError writes an API Gateway style error body for the pre-dispatch
// resolution failures (404 / 403 / 500).
func writeExecuteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}
