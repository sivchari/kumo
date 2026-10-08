// Package awsauth builds the static SigV4 artifacts kumo's internal
// clients attach when calling another kumo service over HTTP.
package awsauth

// ScopeHeader returns an Authorization header whose credential scope names
// signingName, pinning the request to that service's scope router. kumo
// never verifies signatures, so every other field is a fixed placeholder.
func ScopeHeader(signingName string) string {
	return "AWS4-HMAC-SHA256 Credential=kumo/19700101/us-east-1/" + signingName +
		"/aws4_request, SignedHeaders=host, Signature=internal"
}
