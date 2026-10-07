package server

import (
	"net/http"
	"strings"
)

const (
	// sigV4ScopeTerminator ends every SigV4 credential scope
	// (AKID/date/region/service/aws4_request).
	sigV4ScopeTerminator = "aws4_request"

	// sigV4ScopeHeaderPrefix introduces the credential scope inside the
	// Authorization header produced by AWS SDKs.
	sigV4ScopeHeaderPrefix = "Credential="

	// sigV4ScopeQueryParam carries the credential scope of a presigned URL.
	sigV4ScopeQueryParam = "X-Amz-Credential"
)

// sigV4SigningName returns the service signing name from the request's SigV4
// credential scope, or "" when the request carries no parseable scope.
//
// The scope is read from the Authorization header
// (AWS4-HMAC-SHA256 Credential=AKID/date/region/service/aws4_request, ...)
// or, for presigned URLs, from the X-Amz-Credential query parameter.
func sigV4SigningName(r *http.Request) string {
	if name := signingNameFromAuthHeader(r.Header.Get("Authorization")); name != "" {
		return name
	}

	return signingNameFromCredential(r.URL.Query().Get(sigV4ScopeQueryParam))
}

// signingNameFromAuthHeader extracts the signing name from a SigV4
// Authorization header value.
func signingNameFromAuthHeader(auth string) string {
	idx := strings.Index(auth, sigV4ScopeHeaderPrefix)
	if idx < 0 {
		return ""
	}

	cred := auth[idx+len(sigV4ScopeHeaderPrefix):]
	if end := strings.IndexAny(cred, ", "); end >= 0 {
		cred = cred[:end]
	}

	return signingNameFromCredential(cred)
}

// signingNameFromCredential extracts the service name from a credential of
// the form AKID/date/region/service/aws4_request.
func signingNameFromCredential(cred string) string {
	parts := strings.Split(cred, "/")
	if len(parts) != 5 || parts[4] != sigV4ScopeTerminator || parts[3] == "" {
		return ""
	}

	return parts[3]
}
