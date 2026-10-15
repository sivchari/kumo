package apigateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/service/execapi"
)

const (
	// authTypeCustom marks a method whose authorization is delegated to a
	// Lambda authorizer.
	authTypeCustom = "CUSTOM"
	// authorizerTypeRequest is the only authorizer type the execute-api data
	// plane evaluates; TOKEN and COGNITO_USER_POOLS are stored but not run.
	authorizerTypeRequest = "REQUEST"
	authorizerTypeToken   = "TOKEN"
	authorizerTypeCognito = "COGNITO_USER_POOLS"

	// Identity source expression prefixes accepted for a REQUEST authorizer.
	identityPrefixHeader      = "method.request.header."
	identityPrefixQueryString = "method.request.querystring."
	identityPrefixStageVar    = "stageVariables."
	identityPrefixContext     = "context."

	// executeAPIRegion and executeAPIAccount are kumo's conventional values,
	// matching the Lambda service defaults. They appear in the methodArn that
	// the authorizer's returned policy is matched against.
	executeAPIRegion  = "us-east-1"
	executeAPIAccount = "000000000000"

	effectAllow = "Allow"
	effectDeny  = "Deny"

	// unauthorizedErrorMessage is the error a Lambda authorizer raises to ask
	// for a 401 instead of a 403.
	unauthorizedErrorMessage = "Unauthorized"
)

// authorize runs the CUSTOM (REQUEST-type Lambda) authorizer for a resolved
// method. It returns 0 to let the request proceed to the integration, or the
// HTTP status to return instead.
//
// The statuses follow the gateway response types AWS maps these outcomes to:
// 401 (UNAUTHORIZED) when an identity source is missing or the function raises
// "Unauthorized", 403 (ACCESS_DENIED) when the returned policy does not allow
// the method, and 500 (AUTHORIZER_FAILURE / AUTHORIZER_CONFIGURATION_ERROR)
// when the authorizer cannot be run or answers in an invalid format.
func (s *Service) authorize(r *http.Request, apiID, stage string, resolved *resolvedTarget) int {
	authorizer, err := s.storage.GetAuthorizer(r.Context(), apiID, resolved.method.AuthorizerID)
	if err != nil {
		slog.Error("execute-api: authorizer not found",
			"apiId", apiID, "authorizerId", resolved.method.AuthorizerID)

		return http.StatusInternalServerError
	}

	if authorizer.Type != authorizerTypeRequest {
		slog.Error("execute-api: only REQUEST authorizers are evaluated",
			"apiId", apiID, "authorizerId", authorizer.ID, "type", authorizer.Type)

		return http.StatusInternalServerError
	}

	fn := execapi.LambdaFunctionNameFromURI(authorizer.AuthorizerURI)
	if fn == "" {
		slog.Error("execute-api: cannot resolve authorizer function", "uri", authorizer.AuthorizerURI)

		return http.StatusInternalServerError
	}

	methodArn := buildMethodArn(apiID, stage, r.Method, resolved.resource.Path)
	event := buildAuthorizerEvent(r, apiID, stage, resolved, methodArn)

	identity, status := resolveIdentitySources(authorizer.IdentitySource, r, resolved, &event.RequestContext)
	if status != 0 {
		return status
	}

	payload, status := s.authorizerPayload(r, authorizer, fn, identity, event)
	if status != 0 {
		return status
	}

	return evaluateAuthorizerResponse(payload, methodArn)
}

// authorizerPayload returns the authorizer function's response, served from the
// result cache when caching is enabled and the entry is still live.
func (s *Service) authorizerPayload(
	r *http.Request, authorizer *Authorizer, fn string, identity []string, event *AuthorizerEvent,
) ([]byte, int) {
	ttl := authorizer.cacheTTL()

	var key string
	if ttl > 0 {
		key = authorizerCacheKey(authorizer, identity)

		if cached, ok := s.authorizerResults.get(key); ok {
			return cached, 0
		}
	}

	body, err := json.Marshal(event)
	if err != nil {
		slog.Error("execute-api: marshal authorizer event failed", "error", err)

		return nil, http.StatusInternalServerError
	}

	result, err := execapi.InvokeFunctionSync(r.Context(), s.baseURLOrDefault(), fn, body)
	if err != nil {
		slog.Error("execute-api: authorizer invoke failed", "function", fn, "error", err)

		return nil, http.StatusInternalServerError
	}

	if result.FunctionError != "" {
		return nil, authorizerFunctionErrorStatus(result.Payload)
	}

	if ttl > 0 {
		s.authorizerResults.put(key, result.Payload, ttl)
	}

	return result.Payload, 0
}

// authorizerFunctionErrorStatus maps a failed authorizer invocation to a status.
// A function that raises the literal "Unauthorized" error asks for a 401;
// any other failure is an authorizer failure and surfaces as 500.
func authorizerFunctionErrorStatus(payload []byte) int {
	var envelope struct {
		ErrorMessage string `json:"errorMessage"`
	}

	if err := json.Unmarshal(payload, &envelope); err == nil &&
		strings.TrimSpace(envelope.ErrorMessage) == unauthorizedErrorMessage {
		return http.StatusUnauthorized
	}

	slog.Warn("execute-api: authorizer function failed", "payload", string(payload))

	return http.StatusInternalServerError
}

// authorizerCacheKey derives the cache key from the authorizer and its identity
// source values, whose order AWS preserves when deriving the key.
func authorizerCacheKey(authorizer *Authorizer, identity []string) string {
	return authorizer.RestAPIID + "\x00" + authorizer.ID + "\x00" + strings.Join(identity, "\x00")
}

// resolveIdentitySources resolves each configured identity source to its value,
// preserving the configured order so the values can derive the cache key.
//
// A source that is missing, null or empty is a 401 and the authorizer function
// is not invoked at all. A source kumo cannot evaluate is a configuration error
// rather than a silent allow.
func resolveIdentitySources(
	identitySource string, r *http.Request, resolved *resolvedTarget, reqCtx *AuthorizerRequestCtx,
) ([]string, int) {
	if strings.TrimSpace(identitySource) == "" {
		// Optional for a REQUEST authorizer when caching is disabled: the
		// request is passed straight to the function.
		return nil, 0
	}

	var values []string

	for src := range strings.SplitSeq(identitySource, ",") {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}

		value, ok := identitySourceValue(src, r, resolved, reqCtx)
		if !ok {
			slog.Error("execute-api: unsupported authorizer identity source", "source", src)

			return nil, http.StatusInternalServerError
		}

		if value == "" {
			return nil, http.StatusUnauthorized
		}

		values = append(values, value)
	}

	return values, 0
}

// identitySourceValue resolves a single identity source expression. ok reports
// whether the expression is one of the four forms AWS accepts for a REQUEST
// authorizer; the value is empty when the expression resolves to nothing.
func identitySourceValue(
	src string, r *http.Request, resolved *resolvedTarget, reqCtx *AuthorizerRequestCtx,
) (string, bool) {
	switch {
	case strings.HasPrefix(src, identityPrefixHeader):
		return r.Header.Get(strings.TrimPrefix(src, identityPrefixHeader)), true
	case strings.HasPrefix(src, identityPrefixQueryString):
		return r.URL.Query().Get(strings.TrimPrefix(src, identityPrefixQueryString)), true
	case strings.HasPrefix(src, identityPrefixStageVar):
		if resolved.stage == nil {
			return "", true
		}

		return resolved.stage.Variables[strings.TrimPrefix(src, identityPrefixStageVar)], true
	case strings.HasPrefix(src, identityPrefixContext):
		return requestContextValue(reqCtx, strings.TrimPrefix(src, identityPrefixContext))
	default:
		return "", false
	}
}

// requestContextValue resolves a $context.<key> identity source against the
// request context sent to the authorizer.
func requestContextValue(reqCtx *AuthorizerRequestCtx, key string) (string, bool) {
	switch key {
	case "accountId":
		return reqCtx.AccountID, true
	case "apiId":
		return reqCtx.APIID, true
	case "httpMethod":
		return reqCtx.HTTPMethod, true
	case "path":
		return reqCtx.Path, true
	case "requestId":
		return reqCtx.RequestID, true
	case "resourceId":
		return reqCtx.ResourceID, true
	case "resourcePath":
		return reqCtx.ResourcePath, true
	case "stage":
		return reqCtx.Stage, true
	default:
		return "", false
	}
}

// buildMethodArn assembles the execute-api ARN identifying the invoked method.
// It is both sent to the authorizer (as methodArn) and matched against the
// Resource of the policy the authorizer returns.
func buildMethodArn(apiID, stage, httpMethod, resourcePath string) string {
	return fmt.Sprintf("arn:aws:execute-api:%s:%s:%s/%s/%s%s",
		executeAPIRegion, executeAPIAccount, apiID, stage, httpMethod, resourcePath)
}

// buildAuthorizerEvent builds the REQUEST-type authorizer input event.
func buildAuthorizerEvent(
	r *http.Request, apiID, stage string, resolved *resolvedTarget, methodArn string,
) *AuthorizerEvent {
	var (
		resourceID   string
		resourcePath string
	)

	if resolved.resource != nil {
		resourceID = resolved.resource.ID
		resourcePath = resolved.resource.Path
	}

	var stageVariables map[string]string
	if resolved.stage != nil {
		stageVariables = resolved.stage.Variables
	}

	path := "/" + stage + resourcePath

	return &AuthorizerEvent{
		Type:                  authorizerTypeRequest,
		MethodArn:             methodArn,
		Resource:              resourcePath,
		Path:                  path,
		HTTPMethod:            r.Method,
		Headers:               singleValueHeaders(r),
		QueryStringParameters: singleValueQuery(r),
		PathParameters:        nonNilMap(resolved.pathParams),
		StageVariables:        nonNilMap(stageVariables),
		RequestContext: AuthorizerRequestCtx{
			Path:         path,
			AccountID:    executeAPIAccount,
			ResourceID:   resourceID,
			Stage:        stage,
			RequestID:    uuid.New().String(),
			Identity:     AuthorizerRequestIdentity{APIKey: r.Header.Get("X-Api-Key"), SourceIP: sourceIP(r)},
			ResourcePath: resourcePath,
			HTTPMethod:   r.Method,
			APIID:        apiID,
		},
	}
}

// sourceIP returns the caller's IP, dropping the port RemoteAddr carries.
func sourceIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

// nonNilMap returns m, or an empty map when m is nil, so the authorizer event
// carries {} instead of null for absent parameter groups.
func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}

	return m
}

// singleValueHeaders collapses request headers to a single (last) value each.
func singleValueHeaders(r *http.Request) map[string]string {
	out := make(map[string]string, len(r.Header))

	for k, vs := range r.Header {
		if len(vs) > 0 {
			out[k] = vs[len(vs)-1]
		}
	}

	return out
}

// singleValueQuery collapses query parameters to a single (last) value each.
func singleValueQuery(r *http.Request) map[string]string {
	q := r.URL.Query()
	out := make(map[string]string, len(q))

	for k, vs := range q {
		if len(vs) > 0 {
			out[k] = vs[len(vs)-1]
		}
	}

	return out
}

// evaluateAuthorizerResponse maps an authorizer response to the status the
// gateway returns: 0 to allow, 403 when the policy does not allow methodArn,
// and 500 when the response is not a principal plus a policy document, which
// AWS requires of every REST authorizer.
func evaluateAuthorizerResponse(payload []byte, methodArn string) int {
	var out AuthorizerOutput

	if err := json.Unmarshal(payload, &out); err != nil {
		slog.Error("execute-api: authorizer response is not valid JSON", "error", err)

		return http.StatusInternalServerError
	}

	if out.PrincipalID == "" || len(out.PolicyDocument.Statement) == 0 {
		slog.Error("execute-api: authorizer response is missing principalId or policyDocument",
			"payload", string(payload))

		return http.StatusInternalServerError
	}

	if !evaluatePolicy(out.PolicyDocument, methodArn) {
		return http.StatusForbidden
	}

	return 0
}

// evaluatePolicy applies IAM semantics: an explicit matching Deny wins; a
// matching Allow with no matching Deny permits; anything else denies.
func evaluatePolicy(doc PolicyDocument, methodArn string) bool {
	allowed := false

	for _, st := range doc.Statement {
		if !statementMatches(st, methodArn) {
			continue
		}

		if st.Effect == effectDeny {
			return false
		}

		if st.Effect == effectAllow {
			allowed = true
		}
	}

	return allowed
}

// statementMatches reports whether any Resource in the statement matches
// methodArn, expanding AWS-style wildcards.
func statementMatches(st PolicyStatement, methodArn string) bool {
	for _, res := range rawToStrings(st.Resource) {
		if wildcardMatch(res, methodArn) {
			return true
		}
	}

	return false
}

// rawToStrings normalizes a JSON value that may be a string or an array of
// strings into a slice.
func rawToStrings(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		return many
	}

	return nil
}

// wildcardMatch reports whether s matches an AWS resource pattern where '*'
// matches any sequence of characters and '?' matches any single character.
func wildcardMatch(pattern, s string) bool {
	var (
		i, j int
		star = -1
		mark int
	)

	for i < len(s) {
		switch {
		case j < len(pattern) && (pattern[j] == '?' || pattern[j] == s[i]):
			i++
			j++
		case j < len(pattern) && pattern[j] == '*':
			star = j
			mark = i
			j++
		case star != -1:
			j = star + 1
			mark++
			i = mark
		default:
			return false
		}
	}

	for j < len(pattern) && pattern[j] == '*' {
		j++
	}

	return j == len(pattern)
}
