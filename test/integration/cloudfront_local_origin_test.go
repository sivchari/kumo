//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// createCustomOriginDistribution creates a distribution whose only origin is
// the custom domain, deletes it on cleanup, and returns the distribution id.
// The caller reference is unique per run so the test can be repeated against
// the same server.
func createCustomOriginDistribution(t *testing.T, name, domain, originPath string) string {
	t.Helper()

	client := newCloudFrontClient(t)
	ref := name + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	out, err := client.CreateDistribution(t.Context(), &cloudfront.CreateDistributionInput{
		DistributionConfig: &cloudfronttypes.DistributionConfig{
			CallerReference: aws.String(ref),
			Comment:         aws.String(name),
			Enabled:         aws.Bool(true),
			Origins: &cloudfronttypes.Origins{
				Quantity: aws.Int32(1),
				Items: []cloudfronttypes.Origin{{
					Id:         aws.String("origin"),
					DomainName: aws.String(domain),
					OriginPath: aws.String(originPath),
					CustomOriginConfig: &cloudfronttypes.CustomOriginConfig{
						HTTPPort:             aws.Int32(80),
						HTTPSPort:            aws.Int32(443),
						OriginProtocolPolicy: cloudfronttypes.OriginProtocolPolicyHttpsOnly,
					},
				}},
			},
			DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
				TargetOriginId:       aws.String("origin"),
				ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("4135ea2d-6df8-44a3-9df3-4b5a84be39ad"),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      out.Distribution.Id,
			IfMatch: out.ETag,
		})
	})

	return aws.ToString(out.Distribution.Id)
}

// callEdgePath sends a request through kumo's CloudFront edge for distID.
func callEdgePath(t *testing.T, method, distID, path string, body io.Reader, contentType string) (*http.Response, []byte) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, kumoEndpoint+"/kumo/cdn/"+distID+path, body)
	if err != nil {
		t.Fatalf("build edge request: %v", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("edge request: %v", err)
	}

	payload, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	return resp, payload
}

// TestCloudFront_EdgeServesLambdaFunctionURLOrigin — a distribution whose
// custom origin is a Lambda function URL that kumo serves is resolved
// in-process: the function receives the function URL event with the origin
// domain as host, and its response comes back through the edge.
func TestCloudFront_EdgeServesLambdaFunctionURLOrigin(t *testing.T) {
	echo := newFunctionURLEcho(t)
	urlID := createFunctionURL(t, "test-cf-fnurl-origin", echo.url, lambdatypes.FunctionUrlAuthTypeNone, nil)
	domain := urlID + ".lambda-url.us-east-1.on.aws"
	distID := createCustomOriginDistribution(t, "test-cf-fnurl-origin", domain, "")

	resp, body := callEdgePath(t, http.MethodGet, distID, "/hello?x=1", http.NoBody, "")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Cache") == "" {
		t.Fatalf("edge GET: %d %v %s", resp.StatusCode, resp.Header, body)
	}

	event := decodeEvent(t, body)
	ctx, _ := event["requestContext"].(map[string]any)
	headers, _ := event["headers"].(map[string]any)

	if ctx["domainName"] != domain || headers["host"] != domain {
		t.Errorf("function must see the origin domain as host: domainName=%v host=%v", ctx["domainName"], headers["host"])
	}

	if event["rawPath"] != "/hello" || event["rawQueryString"] != "x=1" {
		t.Errorf("rawPath=%v rawQueryString=%v", event["rawPath"], event["rawQueryString"])
	}

	// Non-cacheable methods pass through with their body.
	resp, body = callEdgePath(t, http.MethodPost, distID, "/submit", strings.NewReader(`{"a":1}`), "application/json")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("edge POST: %d %s", resp.StatusCode, body)
	}

	event = decodeEvent(t, body)
	httpCtx, _ := event["requestContext"].(map[string]any)["http"].(map[string]any)

	if httpCtx["method"] != http.MethodPost || event["body"] != `{"a":1}` {
		t.Errorf("POST event: method=%v body=%v", httpCtx["method"], event["body"])
	}

	if echo.invocations.Load() != 2 {
		t.Errorf("invocations = %d, want 2", echo.invocations.Load())
	}
}

// TestCloudFront_EdgeServesExecuteAPIOrigin — an HTTP API behind an
// execute-api custom origin (with the stage in OriginPath) is reached through
// the edge and its Lambda proxy integration answers.
func TestCloudFront_EdgeServesExecuteAPIOrigin(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event map[string]any
		_ = json.NewDecoder(r.Body).Decode(&event)

		_ = json.NewEncoder(w).Encode(map[string]any{
			"statusCode": 200,
			"body":       fmt.Sprintf("via execute-api: rawPath=%v", event["rawPath"]),
		})
	}))
	t.Cleanup(stub.Close)

	const fn = "test-cf-execapi-origin-fn"

	createLambdaWithEndpoint(t, fn, stub.URL)

	client := executeAPIV2Client(t)

	api, err := client.CreateApi(t.Context(), &apigatewayv2.CreateApiInput{
		Name:         aws.String("test-cf-execapi-origin"),
		ProtocolType: apigatewayv2types.ProtocolTypeHttp,
	})
	if err != nil {
		t.Fatalf("CreateApi: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteApi(t.Context(), &apigatewayv2.DeleteApiInput{ApiId: api.ApiId})
	})

	integ, err := client.CreateIntegration(t.Context(), &apigatewayv2.CreateIntegrationInput{
		ApiId:                api.ApiId,
		IntegrationType:      apigatewayv2types.IntegrationTypeAwsProxy,
		IntegrationUri:       aws.String("arn:aws:lambda:us-east-1:000000000000:function:" + fn),
		PayloadFormatVersion: aws.String("2.0"),
	})
	if err != nil {
		t.Fatalf("CreateIntegration: %v", err)
	}

	if _, err := client.CreateRoute(t.Context(), &apigatewayv2.CreateRouteInput{
		ApiId:    api.ApiId,
		RouteKey: aws.String("GET /items"),
		Target:   aws.String("integrations/" + aws.ToString(integ.IntegrationId)),
	}); err != nil {
		t.Fatalf("CreateRoute: %v", err)
	}

	if _, err := client.CreateStage(t.Context(), &apigatewayv2.CreateStageInput{
		ApiId:      api.ApiId,
		StageName:  aws.String("dev"),
		AutoDeploy: aws.Bool(true),
	}); err != nil {
		t.Fatalf("CreateStage: %v", err)
	}

	domain := aws.ToString(api.ApiId) + ".execute-api.us-east-1.amazonaws.com"
	distID := createCustomOriginDistribution(t, "test-cf-execapi-origin", domain, "/dev")

	resp, body := callEdgePath(t, http.MethodGet, distID, "/items", http.NoBody, "")
	if resp.StatusCode != http.StatusOK || string(body) != "via execute-api: rawPath=/dev/items" {
		t.Fatalf("edge GET through execute-api origin: %d %q", resp.StatusCode, body)
	}
}
