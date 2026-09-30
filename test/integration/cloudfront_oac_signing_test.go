//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// edgeAccessKeyID is the principal kumo's CloudFront edge signs Lambda
// origin requests with (internal/service/cloudfront/edge_oac_signing.go).
const edgeAccessKeyID = "AKIAKUMOCLOUDFRONTOA"

// createIAMFunctionURL creates an echo function whose URL requires AWS_IAM
// and returns its lambda-url domain.
func createIAMFunctionURL(t *testing.T, name string) (*functionURLStub, string) {
	t.Helper()

	echo := newFunctionURLEcho(t)
	urlID := createFunctionURL(t, name, echo.url, lambdatypes.FunctionUrlAuthTypeAwsIam, nil)

	return echo, urlID + ".lambda-url.us-east-1.on.aws"
}

// createLambdaOAC creates a lambda origin access control with the behavior.
func createLambdaOAC(t *testing.T, client *cloudfront.Client, name string, behavior cloudfronttypes.OriginAccessControlSigningBehaviors) *string {
	t.Helper()

	return createOAC(t, client, oacConfig(uniqueName(name), cloudfronttypes.OriginAccessControlOriginTypesLambda, behavior)).OriginAccessControl.Id
}

// createSignedOriginDistribution creates a distribution whose only origin is
// the function URL domain, optionally attached to an OAC, and whose default
// behavior optionally whitelists the Authorization header (legacy
// ForwardedValues) so a viewer signature is forwarded.
func createSignedOriginDistribution(t *testing.T, client *cloudfront.Client, domain string, oacID *string, forwardAuthorization bool) string {
	t.Helper()

	origin := customOrigin("fn", domain)
	origin.OriginAccessControlId = oacID

	behavior := &cloudfronttypes.DefaultCacheBehavior{
		TargetOriginId:       aws.String("fn"),
		ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
		CachePolicyId:        aws.String(cachingDisabledID),
	}

	if forwardAuthorization {
		behavior.CachePolicyId = nil
		behavior.MinTTL = aws.Int64(0)
		behavior.ForwardedValues = &cloudfronttypes.ForwardedValues{
			QueryString: aws.Bool(true),
			Cookies:     &cloudfronttypes.CookiePreference{Forward: cloudfronttypes.ItemSelectionNone},
			Headers:     &cloudfronttypes.Headers{Quantity: aws.Int32(1), Items: []string{"Authorization"}},
		}
	}

	created, err := client.CreateDistribution(t.Context(), &cloudfront.CreateDistributionInput{DistributionConfig: &cloudfronttypes.DistributionConfig{
		CallerReference:      aws.String(uniqueName("test-cf-oac-signing")),
		Comment:              aws.String("oac signing"),
		Enabled:              aws.Bool(true),
		Origins:              &cloudfronttypes.Origins{Quantity: aws.Int32(1), Items: []cloudfronttypes.Origin{origin}},
		DefaultCacheBehavior: behavior,
	}})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{Id: created.Distribution.Id, IfMatch: created.ETag})
	})

	return aws.ToString(created.Distribution.Id)
}

func eventIAMAccessKey(t *testing.T, body []byte) (string, map[string]any) {
	t.Helper()

	event := decodeEvent(t, body)
	ctx, _ := event["requestContext"].(map[string]any)
	authorizer, _ := ctx["authorizer"].(map[string]any)
	iam, _ := authorizer["iam"].(map[string]any)
	key, _ := iam["accessKey"].(string)

	return key, event
}

// TestCloudFront_EdgeSignsLambdaOACOrigin — with an OAC of type lambda and
// SigningBehavior always, the edge reaches an AWS_IAM function URL with its
// own SigV4 signature; the function sees the edge principal and the origin
// domain as host.
func TestCloudFront_EdgeSignsLambdaOACOrigin(t *testing.T) {
	client := newCloudFrontClient(t)
	echo, domain := createIAMFunctionURL(t, "test-cf-oac-sign-always")
	distID := createSignedOriginDistribution(t, client, domain, createLambdaOAC(t, client, "test-cf-oac-always", cloudfronttypes.OriginAccessControlSigningBehaviorsAlways), false)

	resp, body := callEdgePath(t, http.MethodGet, distID, "/hello?x=1", http.NoBody, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET through a signed origin: %d %s", resp.StatusCode, body)
	}

	key, event := eventIAMAccessKey(t, body)
	headers, _ := event["headers"].(map[string]any)
	authorization, _ := headers["authorization"].(string)

	if key != edgeAccessKeyID || headers["host"] != domain || !strings.Contains(authorization, "SignedHeaders=") || !strings.Contains(authorization, "host") {
		t.Errorf("accessKey=%q host=%v authorization=%q", key, headers["host"], authorization)
	}

	// A body needs the viewer's payload hash; without it the edge refuses.
	payload := `{"a":1}`
	digest := sha256.Sum256([]byte(payload))

	resp, _ = callEdgePath(t, http.MethodPost, distID, "/submit", strings.NewReader(payload), "application/json")
	if resp.StatusCode != http.StatusForbidden || echo.invocations.Load() != 1 {
		t.Fatalf("POST without x-amz-content-sha256: %d (invocations %d)", resp.StatusCode, echo.invocations.Load())
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, kumoEndpoint+"/kumo/cdn/"+distID+"/submit", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(digest[:]))

	resp, body = callFunctionURL(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST with x-amz-content-sha256: %d %s", resp.StatusCode, body)
	}

	if key, event := eventIAMAccessKey(t, body); key != edgeAccessKeyID || event["body"] != payload {
		t.Errorf("POST: accessKey=%q body=%v", key, event["body"])
	}
}

// TestCloudFront_EdgeWithoutSigningCannotReachIAMOrigin — no OAC, or an OAC
// that never signs, leaves the AWS_IAM function URL refusing the edge.
func TestCloudFront_EdgeWithoutSigningCannotReachIAMOrigin(t *testing.T) {
	client := newCloudFrontClient(t)
	echo, domain := createIAMFunctionURL(t, "test-cf-oac-sign-none")

	plain := createSignedOriginDistribution(t, client, domain, nil, false)
	never := createSignedOriginDistribution(t, client, domain, createLambdaOAC(t, client, "test-cf-oac-never", cloudfronttypes.OriginAccessControlSigningBehaviorsNever), false)

	for name, distID := range map[string]string{"no OAC": plain, "never": never} {
		resp, body := callEdgePath(t, http.MethodGet, distID, "/hello", http.NoBody, "")
		if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "Forbidden") {
			t.Errorf("%s: %d %s, want the function URL's 403", name, resp.StatusCode, body)
		}
	}

	if echo.invocations.Load() != 0 {
		t.Errorf("function must not have been invoked, got %d invocations", echo.invocations.Load())
	}
}

// TestCloudFront_EdgeNoOverrideKeepsForwardedViewerSignature — with
// no-override the edge passes a viewer signature through only when the
// behavior forwards Authorization; otherwise it signs itself.
func TestCloudFront_EdgeNoOverrideKeepsForwardedViewerSignature(t *testing.T) {
	client := newCloudFrontClient(t)
	_, domain := createIAMFunctionURL(t, "test-cf-oac-sign-nooverride")
	oacID := createLambdaOAC(t, client, "test-cf-oac-no-override", cloudfronttypes.OriginAccessControlSigningBehaviorsNoOverride)

	forwarding := createSignedOriginDistribution(t, client, domain, oacID, true)
	notForwarding := createSignedOriginDistribution(t, client, domain, oacID, false)

	cases := []struct {
		name    string
		distID  string
		signed  bool
		wantKey string
	}{
		{"forwarded viewer signature is kept", forwarding, true, functionURLTestKey},
		{"unsigned viewer request is signed by the edge", forwarding, false, edgeAccessKeyID},
		{"viewer signature the behavior does not forward is replaced", notForwarding, true, edgeAccessKeyID},
	}

	for _, tc := range cases {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, kumoEndpoint+"/kumo/cdn/"+tc.distID+"/hello", http.NoBody)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}

		if tc.signed {
			signFunctionURLRequest(t, req)
		}

		resp, body := callFunctionURL(t, req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d %s", tc.name, resp.StatusCode, body)
		}

		if key, _ := eventIAMAccessKey(t, body); key != tc.wantKey {
			t.Errorf("%s: accessKey=%q, want %q", tc.name, key, tc.wantKey)
		}
	}
}
