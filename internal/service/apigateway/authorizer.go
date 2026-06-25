package apigateway

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sivchari/kumo/internal/service/execapi"
)

const (
	// authTypeCustom marks a method whose authorization is delegated to a
	// custom (REQUEST-type Lambda) authorizer.
	authTypeCustom = "CUSTOM"
	// defaultIdentitySource is assumed when an authorizer defines none. It
	// matches the migration target, whose REQUEST authorizers read the
	// Authorization header.
	defaultIdentitySource = "method.request.header.Authorization"
	identityHeaderPrefix  = "method.request.header."
	// executeAPIRegion and executeAPIAccount are kumo's conventional values,
	// matching the Lambda service defaults. They appear in the methodArn that
	// the authorizer's returned policy is matched against.
	executeAPIRegion  = "us-east-1"
	executeAPIAccount = "000000000000"
	effectAllow       = "Allow"
	effectDeny        = "Deny"
)

// authorize runs the CUSTOM (REQUEST-type Lambda) authorizer for a resolved
// method. It returns 0 to let the request proceed to the integration, or a
// non-zero HTTP status to return instead: 401 when required credentials are
// missing, 403 when the authorizer denies (or cannot be run safely), 500 when
// the authorizer Lambda could not be executed.
func (s *Service) authorize(r *http.Request, apiID, stage string, resolved *resolvedTarget) int {
	authorizer, err := s.storage.GetAuthorizer(r.Context(), apiID, resolved.method.AuthorizerID)
	if err != nil {
		slog.Warn("execute-api: authorizer not found; denying",
			"apiId", apiID, "authorizerId", resolved.method.AuthorizerID)

		return http.StatusForbidden
	}

	if identitySourceMissing(authorizer.IdentitySource, r) {
		return http.StatusUnauthorized
	}

	if s.invoker == nil {
		slog.Error("execute-api: no lambda invoker wired; denying CUSTOM authorization", "apiId", apiID)

		return http.StatusForbidden
	}

	fn := execapi.LambdaFunctionNameFromURI(authorizer.AuthorizerURI)
	if fn == "" {
		slog.Error("execute-api: cannot resolve authorizer function", "uri", authorizer.AuthorizerURI)

		return http.StatusForbidden
	}

	methodArn := buildMethodArn(apiID, stage, r.Method, resolved.resource.Path)

	event, err := buildAuthorizerEvent(r, apiID, stage, resolved, methodArn)
	if err != nil {
		slog.Error("execute-api: build authorizer event failed", "error", err)

		return http.StatusForbidden
	}

	payload, err := s.invoker.InvokeSync(r.Context(), fn, event)
	if err != nil {
		slog.Error("execute-api: authorizer invoke failed", "function", fn, "error", err)

		return http.StatusInternalServerError
	}

	if !policyAllows(payload, methodArn) {
		return http.StatusForbidden
	}

	return 0
}

// identitySourceMissing reports whether any header named by the authorizer's
// identity source is absent. API Gateway returns 401 before invoking the
// authorizer in that case. An empty identity source defaults to the
// Authorization header.
func identitySourceMissing(identitySource string, r *http.Request) bool {
	if strings.TrimSpace(identitySource) == "" {
		identitySource = defaultIdentitySource
	}

	for src := range strings.SplitSeq(identitySource, ",") {
		src = strings.TrimSpace(src)
		if !strings.HasPrefix(src, identityHeaderPrefix) {
			continue
		}

		name := strings.TrimPrefix(src, identityHeaderPrefix)
		if r.Header.Get(name) == "" {
			return true
		}
	}

	return false
}

// buildMethodArn assembles the execute-api ARN identifying the invoked method.
// It is both sent to the authorizer (as methodArn) and matched against the
// Resource of the policy the authorizer returns.
func buildMethodArn(apiID, stage, httpMethod, resourcePath string) string {
	return fmt.Sprintf("arn:aws:execute-api:%s:%s:%s/%s/%s%s",
		executeAPIRegion, executeAPIAccount, apiID, stage, httpMethod, resourcePath)
}

// buildAuthorizerEvent builds the REQUEST-type authorizer input event.
func buildAuthorizerEvent(r *http.Request, apiID, stage string, resolved *resolvedTarget, methodArn string) ([]byte, error) {
	event := AuthorizerEvent{
		Type:                  "REQUEST",
		MethodArn:             methodArn,
		Resource:              resolved.resource.Path,
		Path:                  "/" + stage + resolved.resource.Path,
		HTTPMethod:            r.Method,
		Headers:               singleValueHeaders(r),
		QueryStringParameters: singleValueQuery(r),
		PathParameters:        resolved.pathParams,
		RequestContext: AuthorizerRequestCtx{
			APIID:        apiID,
			Stage:        stage,
			HTTPMethod:   r.Method,
			ResourcePath: resolved.resource.Path,
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal authorizer event: %w", err)
	}

	return data, nil
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

// singleValueQuery collapses query parameters to a single (last) value each,
// returning nil when there are none.
func singleValueQuery(r *http.Request) map[string]string {
	q := r.URL.Query()
	if len(q) == 0 {
		return nil
	}

	out := make(map[string]string, len(q))

	for k, vs := range q {
		if len(vs) > 0 {
			out[k] = vs[len(vs)-1]
		}
	}

	return out
}

// policyAllows reports whether the authorizer response permits methodArn. A
// response that cannot be parsed, or that contains no matching Allow (or any
// matching Deny), denies — matching API Gateway, which treats an unparseable or
// failing authorizer as a denial.
func policyAllows(payload []byte, methodArn string) bool {
	var out AuthorizerOutput
	if err := json.Unmarshal(payload, &out); err != nil {
		slog.Warn("execute-api: authorizer response not interpretable; denying", "error", err)

		return false
	}

	return evaluatePolicy(out.PolicyDocument, methodArn)
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
