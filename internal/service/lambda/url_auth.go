package lambda

import (
	"net/http"
	"slices"
	"strings"

	"github.com/sivchari/kumo/internal/service/execapi"
)

const (
	sigV4Algorithm   = "AWS4-HMAC-SHA256"
	sigV4Terminal    = "aws4_request"
	sigV4Service     = "lambda"
	sigV4ScopeFields = 5
	sigV4DateLength  = 8

	headerAmzDate = "X-Amz-Date"
)

// sigV4Identity recognises a structurally complete SigV4 authentication on a
// request to an AWS_IAM function URL, in either the Authorization header or
// the presigned query form, and returns the synthetic local identity Lambda
// places in requestContext.authorizer.iam. kumo does not verify the
// signature, the credential or any IAM policy: a well-formed, dated signature
// scoped to the lambda service whose signed headers are present is accepted.
func sigV4Identity(r *http.Request, accountID string) (*execapi.FunctionURLIAMIdentity, bool) {
	credential, signedHeaders, ok := sigV4Parameters(r)
	if !ok || !signedHeadersPresent(r, signedHeaders) {
		return nil, false
	}

	accessKey, ok := sigV4AccessKey(credential)
	if !ok {
		return nil, false
	}

	return &execapi.FunctionURLIAMIdentity{
		AccessKey: accessKey,
		AccountID: accountID,
		CallerID:  accountID,
		UserArn:   "arn:aws:iam::" + accountID + ":root",
		UserID:    accountID,
	}, true
}

// sigV4Parameters extracts Credential and SignedHeaders from whichever form
// the request uses, requiring the signature and the request date as well.
func sigV4Parameters(r *http.Request) (string, string, bool) {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if r.Header.Get(headerAmzDate) == "" && r.Header.Get("Date") == "" {
			return "", "", false
		}

		return sigV4HeaderParameters(auth)
	}

	q := r.URL.Query()
	if q.Get("X-Amz-Algorithm") != sigV4Algorithm {
		return "", "", false
	}

	for _, key := range []string{headerAmzDate, "X-Amz-Expires", "X-Amz-Signature"} {
		if q.Get(key) == "" {
			return "", "", false
		}
	}

	credential, signedHeaders := q.Get("X-Amz-Credential"), q.Get("X-Amz-SignedHeaders")

	return credential, signedHeaders, credential != "" && signedHeaders != ""
}

// sigV4HeaderParameters parses
// "AWS4-HMAC-SHA256 Credential=..., SignedHeaders=..., Signature=...".
func sigV4HeaderParameters(auth string) (string, string, bool) {
	rest, ok := strings.CutPrefix(auth, sigV4Algorithm+" ")
	if !ok {
		return "", "", false
	}

	params := map[string]string{}

	for _, part := range strings.Split(rest, ",") {
		if key, value, found := strings.Cut(strings.TrimSpace(part), "="); found {
			params[key] = value
		}
	}

	if params["Signature"] == "" {
		return "", "", false
	}

	credential, signedHeaders := params["Credential"], params["SignedHeaders"]

	return credential, signedHeaders, credential != "" && signedHeaders != ""
}

// signedHeadersPresent reports whether the signed header list names host and
// only headers the request actually carries.
func signedHeadersPresent(r *http.Request, signedHeaders string) bool {
	names := strings.Split(strings.ToLower(signedHeaders), ";")
	if !slices.Contains(names, "host") || r.Host == "" {
		return false
	}

	for _, name := range names {
		if name != "host" && len(r.Header.Values(name)) == 0 {
			return false
		}
	}

	return true
}

// sigV4AccessKey validates the credential scope
// <access-key>/<YYYYMMDD>/<region>/lambda/aws4_request and returns the access key.
func sigV4AccessKey(credential string) (string, bool) {
	parts := strings.Split(credential, "/")
	if len(parts) != sigV4ScopeFields || parts[0] == "" || !isSigV4Date(parts[1]) || parts[2] == "" ||
		parts[3] != sigV4Service || parts[4] != sigV4Terminal {
		return "", false
	}

	return parts[0], true
}

func isSigV4Date(s string) bool {
	if len(s) != sigV4DateLength {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}
