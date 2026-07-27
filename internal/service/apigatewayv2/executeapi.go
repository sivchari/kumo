package apigatewayv2

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service/execapi"
)

// defaultStageName is the HTTP API auto-deploy catch-all stage.
const defaultStageName = "$default"

// defaultPayloadFormatVersion is the HTTP API default when an integration does
// not specify one.
const defaultPayloadFormatVersion = "2.0"

// HandleExecuteAPI implements service.ExecuteAPIHandler for HTTP APIs. It
// returns false when apiID is not an API managed by this service.
func (s *Service) HandleExecuteAPI(w http.ResponseWriter, r *http.Request, apiID, invokePath string) bool {
	if _, err := s.storage.GetAPI(r.Context(), apiID); err != nil {
		return false
	}

	stage, stageObj, routePath := s.resolveStage(r, apiID, invokePath)
	if stage == "" {
		writeExecuteErrorV2(w, http.StatusNotFound, "Not Found")

		return true
	}

	route, pathParams, ok := s.matchRequestRoute(r, apiID, routePath)
	if !ok {
		writeExecuteErrorV2(w, http.StatusNotFound, "Not Found")

		return true
	}

	authContext, ok := s.authorizeRoute(r, apiID, route)
	if !ok {
		writeExecuteErrorV2(w, http.StatusUnauthorized, "Unauthorized")

		return true
	}

	integration, ok := s.integrationForRoute(r, apiID, route)
	if !ok {
		writeExecuteErrorV2(w, http.StatusInternalServerError, "Internal server error")

		return true
	}

	execapi.Dispatch(w, r, targetFor(integration),
		&execapi.Request{
			BaseURL:        s.baseURLOrDefault(),
			APIID:          apiID,
			Stage:          stage,
			ResourcePath:   routePath,
			RouteKey:       route.RouteKey,
			PathParameters: pathParams,
			StageVariables: stageObj.StageVariables,
			Authorizer:     authContext,
		},
	)

	return true
}

// matchRequestRoute loads the API's routes and matches the request against
// them.
func (s *Service) matchRequestRoute(r *http.Request, apiID, routePath string) (*Route, map[string]string, bool) {
	routes, err := s.storage.GetRoutes(r.Context(), apiID)
	if err != nil {
		return nil, nil, false
	}

	return matchRoute(routes, r.Method, routePath)
}

// authorizeRoute enforces the route's JWT authorizer, if any. ok is false
// when the request must be rejected with 401 Unauthorized. Routes without a
// JWT authorizer (AuthorizationType != "JWT" or no AuthorizerID) are always
// authorized, with a nil authorizer context.
func (s *Service) authorizeRoute(r *http.Request, apiID string, route *Route) (*execapi.AuthorizerContext, bool) {
	if route.AuthorizationType != authorizerTypeJWT || route.AuthorizerID == "" {
		return nil, true
	}

	authorizer, err := s.storage.GetAuthorizer(r.Context(), apiID, route.AuthorizerID)
	if err != nil {
		return nil, false
	}

	return authorizeJWT(r, authorizer)
}

// targetFor builds the execapi.Target for an integration, defaulting the
// payload format version when the integration does not specify one.
func targetFor(integration *Integration) execapi.Target {
	payloadFormat := integration.PayloadFormatVersion
	if payloadFormat == "" {
		payloadFormat = defaultPayloadFormatVersion
	}

	return execapi.Target{
		Type:                 integration.IntegrationType,
		URI:                  integration.IntegrationURI,
		PayloadFormatVersion: payloadFormat,
	}
}

// resolveStage determines the stage, the resolved stage object, and the
// route path from the invoke path. The path is /{stage}/{route} for a named
// stage, or /{route} for $default.
func (s *Service) resolveStage(r *http.Request, apiID, invokePath string) (stage string, stageObj *Stage, routePath string) {
	segs := execapi.SplitPath(invokePath)

	if len(segs) > 0 {
		if st, err := s.storage.GetStage(r.Context(), apiID, segs[0]); err == nil {
			return segs[0], st, normalizeRoutePath(segs[1:])
		}
	}

	if st, err := s.storage.GetStage(r.Context(), apiID, defaultStageName); err == nil {
		return defaultStageName, st, normalizeRoutePath(segs)
	}

	return "", nil, ""
}

// integrationForRoute resolves the integration referenced by a route's target
// (of the form "integrations/{integrationId}").
func (s *Service) integrationForRoute(r *http.Request, apiID string, route *Route) (*Integration, bool) {
	id := strings.TrimPrefix(route.Target, "integrations/")
	if id == "" || id == route.Target {
		return nil, false
	}

	integration, err := s.storage.GetIntegration(r.Context(), apiID, id)
	if err != nil {
		return nil, false
	}

	return integration, true
}

// baseURLOrDefault returns the configured base URL, defaulting to the local
// kumo server when unset.
func (s *Service) baseURLOrDefault() string {
	if s.baseURL == "" {
		return execapi.DefaultBaseURL
	}

	return s.baseURL
}

// matchRoute selects the route matching the request method and path. An exact
// "METHOD /path" (or "ANY /path") match wins by specificity; the "$default"
// route is the catch-all fallback.
func matchRoute(routes []*Route, method, routePath string) (*Route, map[string]string, bool) {
	reqSegs := execapi.SplitPath(routePath)

	var (
		best     *Route
		bestVals map[string]string
		// bestScore starts below the $default sentinel so any real match wins.
		bestScore = -2
		fallback  *Route
	)

	for _, route := range routes {
		if route.RouteKey == defaultStageName { // "$default"
			fallback = route

			continue
		}

		routeMethod, routeTemplate, ok := splitRouteKey(route.RouteKey)
		if !ok || (routeMethod != method && routeMethod != "ANY") {
			continue
		}

		vals, score, ok := execapi.MatchPath(routeTemplate, reqSegs)
		if !ok {
			continue
		}

		if score > bestScore {
			best = route
			bestVals = vals
			bestScore = score
		}
	}

	if best != nil {
		return best, bestVals, true
	}

	if fallback != nil {
		return fallback, nil, true
	}

	return nil, nil, false
}

// splitRouteKey splits a route key like "GET /items" into method and path.
func splitRouteKey(routeKey string) (method, path string, ok bool) {
	parts := strings.SplitN(routeKey, " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// normalizeRoutePath joins remaining segments into a leading-slash path.
func normalizeRoutePath(segs []string) string {
	if len(segs) == 0 {
		return "/"
	}

	return "/" + strings.Join(segs, "/")
}

// writeExecuteErrorV2 writes an API Gateway style error body.
func writeExecuteErrorV2(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}
