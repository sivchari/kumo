package apigatewayv2

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sivchari/kumo/internal/service/execapi"
)

// authorizerTypeJWT is the only authorizer type kumo evaluates at execute
// time. Other types (e.g. REQUEST) are stored but never enforced.
const authorizerTypeJWT = "JWT"

// Registered JWT claim names used for authorizer validation, per "Control
// access to HTTP APIs with JWT authorizers in API Gateway":
// https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-jwt-authorizer.html
const (
	claimExp      = "exp"
	claimIss      = "iss"
	claimAud      = "aud"
	claimClientID = "client_id"
	claimScope    = "scope"
)

// authorizeJWT validates the token presented by the request against the
// route's JWT authorizer and, on success, returns the authorizer context to
// attach to the payload-2.0 event's requestContext. ok is false when the
// request must be rejected with 401 Unauthorized.
func authorizeJWT(r *http.Request, authorizer *Authorizer) (*execapi.AuthorizerContext, bool) {
	token, ok := bearerToken(r, authorizer.IdentitySource)
	if !ok {
		return nil, false
	}

	claims, err := decodeJWTPayload(token)
	if err != nil {
		return nil, false
	}

	if !validateJWTClaims(claims, authorizer.JWTConfiguration) {
		return nil, false
	}

	// Signature verification is intentionally skipped: kumo has no access to
	// the identity provider's JWKS. A real check would slot in here, after
	// claim validation and before the context is built, so a malformed or
	// unsigned token still fails fast regardless of its claims.

	return &execapi.AuthorizerContext{
		JWT: &execapi.JWTAuthorizerClaims{
			Claims: stringifyClaims(claims),
			Scopes: scopesFromClaims(claims),
		},
	}, true
}

// bearerToken extracts the bearer token from the request header named by
// identitySource, defaulting to Authorization. Only header-based identity
// sources (the common case) are recognized; anything else falls back to the
// Authorization header.
func bearerToken(r *http.Request, identitySource []string) (string, bool) {
	header := "Authorization"

	for _, src := range identitySource {
		if name, ok := strings.CutPrefix(src, "$request.header."); ok {
			header = name

			break
		}
	}

	const bearerPrefix = "Bearer "

	raw := r.Header.Get(header)
	if !strings.HasPrefix(raw, bearerPrefix) {
		return "", false
	}

	token := strings.TrimPrefix(raw, bearerPrefix)
	if token == "" {
		return "", false
	}

	return token, true
}

// decodeJWTPayload base64url-decodes and JSON-parses a JWT's payload (the
// second dot-separated segment) into its claim set. It does not verify the
// signature.
func decodeJWTPayload(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed JWT: expected 3 segments, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}

	claims := map[string]any{}

	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()

	if err := dec.Decode(&claims); err != nil {
		return nil, fmt.Errorf("unmarshal JWT payload: %w", err)
	}

	return claims, nil
}

// validateJWTClaims checks exp, iss, and aud/client_id against the
// authorizer configuration.
func validateJWTClaims(claims map[string]any, cfg *JWTConfiguration) bool {
	if !expirationValid(claims) {
		return false
	}

	if cfg == nil {
		return true
	}

	if cfg.Issuer != "" && claimString(claims, claimIss) != cfg.Issuer {
		return false
	}

	if len(cfg.Audience) > 0 && !audienceMatches(claims, cfg.Audience) {
		return false
	}

	return true
}

// expirationValid rejects tokens whose exp claim is missing, unparsable, or
// in the past.
func expirationValid(claims map[string]any) bool {
	num, ok := claims[claimExp].(json.Number)
	if !ok {
		return false
	}

	exp, err := num.Int64()
	if err != nil {
		return false
	}

	return time.Now().Before(time.Unix(exp, 0))
}

// audienceMatches implements API Gateway's aud/client_id precedence: aud is
// checked when present (a string or an array of strings per RFC 7519);
// client_id is checked only when aud is absent.
func audienceMatches(claims map[string]any, audience []string) bool {
	if aud, ok := claims[claimAud]; ok {
		return containsAny(audienceValues(aud), audience)
	}

	if clientID := claimString(claims, claimClientID); clientID != "" {
		return containsAny([]string{clientID}, audience)
	}

	return false
}

// audienceValues normalizes the aud claim into a slice of strings.
func audienceValues(aud any) []string {
	switch v := aud.(type) {
	case string:
		return []string{v}
	case []any:
		values := make([]string, 0, len(v))

		for _, item := range v {
			if s, ok := item.(string); ok {
				values = append(values, s)
			}
		}

		return values
	default:
		return nil
	}
}

// containsAny reports whether any of values is present in allowed.
func containsAny(values, allowed []string) bool {
	for _, v := range values {
		for _, a := range allowed {
			if v == a {
				return true
			}
		}
	}

	return false
}

// claimString returns a claim's value as a string, or "" if it is absent or
// not a string.
func claimString(claims map[string]any, key string) string {
	s, _ := claims[key].(string)

	return s
}

// stringifyClaims renders every claim value the way API Gateway does in
// requestContext.authorizer.jwt.claims: every value, including numbers and
// booleans, becomes a string.
func stringifyClaims(claims map[string]any) map[string]string {
	out := make(map[string]string, len(claims))

	for k, v := range claims {
		out[k] = stringifyClaimValue(v)
	}

	return out
}

// stringifyClaimValue converts a decoded JSON claim value to its string
// form. json.Number preserves the token's original numeric text instead of
// round-tripping through float64.
func stringifyClaimValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		return val.String()
	case bool:
		return strconv.FormatBool(val)
	case nil:
		return ""
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return ""
		}

		return string(data)
	}
}

// scopesFromClaims splits the space-delimited scope claim into individual
// scopes. Returns nil when the claim is absent.
func scopesFromClaims(claims map[string]any) []string {
	scope := claimString(claims, claimScope)
	if scope == "" {
		return nil
	}

	return strings.Fields(scope)
}
