package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/sivchari/kumo/internal/awsauth"
)

// sesTargetPath is an arbitrary AWS-faithful SES v2 path used as the request
// target across the signing-name cases.
const sesTargetPath = "/v2/email/identities"

var sigV4SigningNameCases = []struct {
	name   string
	auth   string
	target string
	want   string
}{
	{
		name:   "sdk authorization header",
		auth:   "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20261007/us-east-1/ses/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc",
		target: sesTargetPath,
		want:   "ses",
	},
	{
		name:   "awsauth scope header",
		auth:   awsauth.ScopeHeader("lambda"),
		target: sesTargetPath,
		want:   "lambda",
	},
	{
		name:   "space separated credential",
		auth:   "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20261007/eu-west-1/scheduler/aws4_request SignedHeaders=host Signature=abc",
		target: "/schedules/test",
		want:   "scheduler",
	},
	{
		name:   "presigned url query parameter",
		auth:   "",
		target: "/?X-Amz-Credential=" + url.QueryEscape("AKIDEXAMPLE/20261007/us-east-1/lambda/aws4_request"),
		want:   "lambda",
	},
	{
		name:   "unsigned request",
		auth:   "",
		target: sesTargetPath,
		want:   "",
	},
	{
		name:   "non sigv4 authorization",
		auth:   "Bearer token",
		target: sesTargetPath,
		want:   "",
	},
	{
		name:   "malformed scope too few parts",
		auth:   "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20261007/ses/aws4_request, Signature=abc",
		target: sesTargetPath,
		want:   "",
	},
	{
		name:   "scope without terminator",
		auth:   "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20261007/us-east-1/ses/other, Signature=abc",
		target: sesTargetPath,
		want:   "",
	},
	{
		name:   "empty service in scope",
		auth:   "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20261007/us-east-1//aws4_request, Signature=abc",
		target: sesTargetPath,
		want:   "",
	},
}

func TestSigV4SigningName(t *testing.T) {
	t.Parallel()

	for _, tc := range sigV4SigningNameCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}

			if got := sigV4SigningName(req); got != tc.want {
				t.Fatalf("sigV4SigningName() = %q, want %q", got, tc.want)
			}
		})
	}
}
