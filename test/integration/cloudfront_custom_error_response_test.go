//go:build integration

package integration

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	customErrorOriginID = "pages-bucket"
	customErrorPage404  = "<h1>custom 404</h1>"
	customErrorPage403  = "<h1>custom 403</h1>"
)

// newCustomErrorBucket creates a bucket holding index.html and the custom
// error pages under errors/, and returns its virtual-hosted domain name.
func newCustomErrorBucket(t *testing.T) string {
	t.Helper()

	client := newS3Client(t)
	bucket := "cf-custom-errors-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	if _, err := client.CreateBucket(t.Context(), &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	objects := map[string]string{
		"index.html":      "<h1>home</h1>",
		"errors/404.html": customErrorPage404,
		"errors/403.html": customErrorPage403,
	}

	for key, body := range objects {
		if _, err := client.PutObject(t.Context(), &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(key),
			Body:        strings.NewReader(body),
			ContentType: aws.String("text/html"),
		}); err != nil {
			t.Fatalf("PutObject %s: %v", key, err)
		}
	}

	t.Cleanup(func() {
		for key := range objects {
			_, _ = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		}

		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	return bucket + ".s3.us-east-1.amazonaws.com"
}

// createCustomErrorDistribution creates a distribution over the bucket with
// the given default behaviour, an open `/errors/*` behaviour and the custom
// error responses.
func createCustomErrorDistribution(t *testing.T, domain string, defaultBehavior *cloudfronttypes.DefaultCacheBehavior, responses ...cloudfronttypes.CustomErrorResponse) (*cloudfront.Client, string) {
	t.Helper()

	client := newCloudFrontClient(t)

	defaultBehavior.TargetOriginId = aws.String(customErrorOriginID)
	defaultBehavior.ViewerProtocolPolicy = cloudfronttypes.ViewerProtocolPolicyAllowAll
	defaultBehavior.CachePolicyId = aws.String(cachingDisabledID)

	created, err := client.CreateDistribution(t.Context(), &cloudfront.CreateDistributionInput{DistributionConfig: &cloudfronttypes.DistributionConfig{
		CallerReference: aws.String("test-cf-custom-errors-" + strconv.FormatInt(time.Now().UnixNano(), 36)),
		Comment:         aws.String("custom error responses"),
		Enabled:         aws.Bool(true),
		Origins: &cloudfronttypes.Origins{Quantity: aws.Int32(1), Items: []cloudfronttypes.Origin{{
			Id:             aws.String(customErrorOriginID),
			DomainName:     aws.String(domain),
			S3OriginConfig: &cloudfronttypes.S3OriginConfig{OriginAccessIdentity: aws.String("")},
		}}},
		DefaultCacheBehavior: defaultBehavior,
		CacheBehaviors: &cloudfronttypes.CacheBehaviors{Quantity: aws.Int32(1), Items: []cloudfronttypes.CacheBehavior{{
			PathPattern:          aws.String("/errors/*"),
			TargetOriginId:       aws.String(customErrorOriginID),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
			CachePolicyId:        aws.String(cachingDisabledID),
		}}},
		CustomErrorResponses: &cloudfronttypes.CustomErrorResponses{Quantity: aws.Int32(int32(len(responses))), Items: responses},
	}})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{Id: created.Distribution.Id, IfMatch: created.ETag})
	})

	return client, aws.ToString(created.Distribution.Id)
}

func customErrorResponse(errorCode int32, pagePath, responseCode string) cloudfronttypes.CustomErrorResponse {
	return cloudfronttypes.CustomErrorResponse{
		ErrorCode:          aws.Int32(errorCode),
		ResponsePagePath:   aws.String(pagePath),
		ResponseCode:       aws.String(responseCode),
		ErrorCachingMinTTL: aws.Int64(0),
	}
}

// TestCloudFront_CustomErrorResponseRoundTrip — the configured entries read
// back from GetDistributionConfig.
func TestCloudFront_CustomErrorResponseRoundTrip(t *testing.T) {
	t.Parallel()

	client, distID := createCustomErrorDistribution(t, "roundtrip.s3.us-east-1.amazonaws.com", &cloudfronttypes.DefaultCacheBehavior{},
		customErrorResponse(404, "/errors/404.html", "200"),
		cloudfronttypes.CustomErrorResponse{ErrorCode: aws.Int32(503), ErrorCachingMinTTL: aws.Int64(5)})

	got, err := client.GetDistributionConfig(t.Context(), &cloudfront.GetDistributionConfigInput{Id: aws.String(distID)})
	if err != nil {
		t.Fatalf("GetDistributionConfig: %v", err)
	}

	responses := got.DistributionConfig.CustomErrorResponses
	if responses == nil || aws.ToInt32(responses.Quantity) != 2 || len(responses.Items) != 2 {
		t.Fatalf("CustomErrorResponses = %+v, want two entries", responses)
	}

	first, second := responses.Items[0], responses.Items[1]
	if aws.ToInt32(first.ErrorCode) != 404 || aws.ToString(first.ResponsePagePath) != "/errors/404.html" || aws.ToString(first.ResponseCode) != "200" || aws.ToInt64(first.ErrorCachingMinTTL) != 0 {
		t.Errorf("first entry = %+v", first)
	}

	if aws.ToInt32(second.ErrorCode) != 503 || aws.ToString(second.ResponsePagePath) != "" || aws.ToInt64(second.ErrorCachingMinTTL) != 5 {
		t.Errorf("second entry = %+v", second)
	}
}

// TestCloudFront_EdgeCustomErrorPageForOrigin404 — an S3 404 is replaced by
// the bucket's error page with the configured response code.
func TestCloudFront_EdgeCustomErrorPageForOrigin404(t *testing.T) {
	t.Parallel()

	domain := newCustomErrorBucket(t)

	for _, tc := range []struct {
		responseCode string
		want         int
	}{{"200", http.StatusOK}, {"404", http.StatusNotFound}, {"204", http.StatusNotFound}} { // 204 is not a documented response code: the origin status is kept.
		t.Run("response_code_"+tc.responseCode, func(t *testing.T) {
			t.Parallel()

			_, distID := createCustomErrorDistribution(t, domain, &cloudfronttypes.DefaultCacheBehavior{},
				customErrorResponse(404, "/errors/404.html", tc.responseCode))

			resp, body := callEdgePath(t, http.MethodGet, distID, "/no-such-page.html", http.NoBody, "")
			if resp.StatusCode != tc.want || string(body) != customErrorPage404 {
				t.Errorf("got %d %q, want %d %q", resp.StatusCode, body, tc.want, customErrorPage404)
			}

			if resp, body := callEdgePath(t, http.MethodGet, distID, "/index.html", http.NoBody, ""); resp.StatusCode != http.StatusOK || string(body) != "<h1>home</h1>" {
				t.Errorf("existing object: got %d %q", resp.StatusCode, body)
			}
		})
	}
}

// TestCloudFront_EdgeCustomErrorPageKeepsCORSHeaders — a cross-origin request
// that ends in the custom error page still gets the CORS headers the bucket
// answered the request with, so the browser lets the page through.
func TestCloudFront_EdgeCustomErrorPageKeepsCORSHeaders(t *testing.T) {
	t.Parallel()

	const viewerOrigin = "https://app.example"

	domain := newCustomErrorBucket(t)
	bucket, _, _ := strings.Cut(domain, ".")

	if _, err := newS3Client(t).PutBucketCors(t.Context(), &s3.PutBucketCorsInput{
		Bucket: aws.String(bucket),
		CORSConfiguration: &s3types.CORSConfiguration{CORSRules: []s3types.CORSRule{{
			AllowedOrigins: []string{viewerOrigin},
			AllowedMethods: []string{http.MethodGet},
		}}},
	}); err != nil {
		t.Fatalf("PutBucketCors: %v", err)
	}

	_, distID := createCustomErrorDistribution(t, domain, &cloudfronttypes.DefaultCacheBehavior{},
		customErrorResponse(404, "/errors/404.html", "200"))

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, kumoEndpoint+"/kumo/cdn/"+distID+"/no-such-page.html", http.NoBody)
	if err != nil {
		t.Fatalf("build edge request: %v", err)
	}

	req.Header.Set("Origin", viewerOrigin)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("edge request: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read edge response: %v", err)
	}

	if resp.StatusCode != http.StatusOK || string(body) != customErrorPage404 {
		t.Fatalf("got %d %q, want 200 %q", resp.StatusCode, body, customErrorPage404)
	}

	if got := resp.Header.Values("Access-Control-Allow-Origin"); len(got) != 1 || got[0] != viewerOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want [%q]", got, viewerOrigin)
	}
}

// localOrigin is a custom origin for an HTTP server of the test, reached over
// plain HTTP on its port.
func localOrigin(t *testing.T, id, serverURL string) cloudfronttypes.Origin {
	t.Helper()

	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse %s: %v", serverURL, err)
	}

	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("port of %s: %v", serverURL, err)
	}

	return cloudfronttypes.Origin{
		Id:         aws.String(id),
		DomainName: aws.String(parsed.Hostname()),
		CustomOriginConfig: &cloudfronttypes.CustomOriginConfig{
			HTTPPort:             aws.Int32(int32(port)),
			HTTPSPort:            aws.Int32(443),
			OriginProtocolPolicy: cloudfronttypes.OriginProtocolPolicyHttpOnly,
		},
	}
}

// createLocalCustomErrorDistribution creates a distribution over two HTTP
// servers of the test: contentURL behind a default behaviour that forwards
// cookies and lets the edge cache a response for an hour, and pagesURL behind
// an uncached `/errors/*` behaviour.
func createLocalCustomErrorDistribution(t *testing.T, contentURL, pagesURL string, responses ...cloudfronttypes.CustomErrorResponse) string {
	t.Helper()

	client := newCloudFrontClient(t)

	created, err := client.CreateDistribution(t.Context(), &cloudfront.CreateDistributionInput{DistributionConfig: &cloudfronttypes.DistributionConfig{
		CallerReference: aws.String("test-cf-local-custom-errors-" + strconv.FormatInt(time.Now().UnixNano(), 36)),
		Comment:         aws.String("custom error responses over local origins"),
		Enabled:         aws.Bool(true),
		Origins: &cloudfronttypes.Origins{Quantity: aws.Int32(2), Items: []cloudfronttypes.Origin{
			localOrigin(t, "content", contentURL),
			localOrigin(t, customErrorOriginID, pagesURL),
		}},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
			TargetOriginId:       aws.String("content"),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
			ForwardedValues: &cloudfronttypes.ForwardedValues{
				QueryString: aws.Bool(false),
				Cookies:     &cloudfronttypes.CookiePreference{Forward: cloudfronttypes.ItemSelectionAll},
			},
			MinTTL:     aws.Int64(0),
			DefaultTTL: aws.Int64(3600),
			MaxTTL:     aws.Int64(86400),
		},
		CacheBehaviors: &cloudfronttypes.CacheBehaviors{Quantity: aws.Int32(1), Items: []cloudfronttypes.CacheBehavior{{
			PathPattern:          aws.String("/errors/*"),
			TargetOriginId:       aws.String(customErrorOriginID),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
			CachePolicyId:        aws.String(cachingDisabledID),
		}}},
		CustomErrorResponses: &cloudfronttypes.CustomErrorResponses{Quantity: aws.Int32(int32(len(responses))), Items: responses},
	}})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{Id: created.Distribution.Id, IfMatch: created.ETag})
	})

	return aws.ToString(created.Distribution.Id)
}

// TestCloudFront_EdgeCustomErrorPageCarryingCookieIsNotCacheable — the custom
// error page that replaces an origin error setting a cookie carries the
// cookie, but no cache may store it although the page origin marked the page
// public, and the edge does not answer the next request from its cache.
func TestCloudFront_EdgeCustomErrorPageCarryingCookieIsNotCacheable(t *testing.T) {
	t.Parallel()

	const cookie = "session=viewer-1; Path=/; HttpOnly"

	var contentRequests atomic.Int32

	content := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		contentRequests.Add(1)
		w.Header().Set("Set-Cookie", cookie)
		w.Header().Set("Cache-Control", "private")
		http.Error(w, "content 404", http.StatusNotFound)
	}))
	t.Cleanup(content.Close)

	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("CDN-Cache-Control", "max-age=3600")
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, customErrorPage404)
	}))
	t.Cleanup(pages.Close)

	distID := createLocalCustomErrorDistribution(t, content.URL, pages.URL, customErrorResponse(404, "/errors/404.html", "200"))

	for i := range 2 {
		resp, body := callEdgePath(t, http.MethodGet, distID, "/no-such-page.html", http.NoBody, "")
		if resp.StatusCode != http.StatusOK || string(body) != customErrorPage404 {
			t.Fatalf("request %d: got %d %q, want 200 %q", i, resp.StatusCode, body, customErrorPage404)
		}

		if got := resp.Header.Values("Set-Cookie"); len(got) != 1 || got[0] != cookie {
			t.Errorf("request %d: Set-Cookie = %q, want [%q]", i, got, cookie)
		}

		if got := resp.Header.Values("Cache-Control"); len(got) != 1 || got[0] != "no-store" {
			t.Errorf("request %d: Cache-Control = %q, want [no-store]", i, got)
		}

		if got := resp.Header.Get("CDN-Cache-Control"); got != "" {
			t.Errorf("request %d: CDN-Cache-Control = %q, want none", i, got)
		}
	}

	if n := contentRequests.Load(); n != 2 {
		t.Errorf("content origin got %d requests, want 2: the cookie must not be served from the edge cache", n)
	}
}

// newTrustedKeyGroupBehavior creates a public key and key group and returns a
// default behaviour that requires signed URLs or cookies from that group. The
// key group's cleanup is registered before the caller creates the
// distribution, so it runs after the distribution's.
func newTrustedKeyGroupBehavior(t *testing.T) *cloudfronttypes.DefaultCacheBehavior {
	t.Helper()

	client := newCloudFrontClient(t)
	_, pubPEM := generateTestKeyPair(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)

	pk, err := client.CreatePublicKey(t.Context(), &cloudfront.CreatePublicKeyInput{PublicKeyConfig: &cloudfronttypes.PublicKeyConfig{
		CallerReference: aws.String("test-cf-custom-errors-pk-" + suffix),
		Name:            aws.String("custom-errors-key-" + suffix),
		EncodedKey:      aws.String(pubPEM),
	}})
	if err != nil {
		t.Fatalf("CreatePublicKey: %v", err)
	}

	kg, err := client.CreateKeyGroup(t.Context(), &cloudfront.CreateKeyGroupInput{KeyGroupConfig: &cloudfronttypes.KeyGroupConfig{
		Name:  aws.String("custom-errors-kg-" + suffix),
		Items: []string{aws.ToString(pk.PublicKey.Id)},
	}})
	if err != nil {
		t.Fatalf("CreateKeyGroup: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteKeyGroup(context.Background(), &cloudfront.DeleteKeyGroupInput{Id: kg.KeyGroup.Id, IfMatch: kg.ETag})
		_, _ = client.DeletePublicKey(context.Background(), &cloudfront.DeletePublicKeyInput{Id: pk.PublicKey.Id, IfMatch: pk.ETag})
	})

	return &cloudfronttypes.DefaultCacheBehavior{
		TrustedKeyGroups: &cloudfronttypes.TrustedKeyGroups{Enabled: aws.Bool(true), Quantity: aws.Int32(1), Items: []string{aws.ToString(kg.KeyGroup.Id)}},
	}
}

// TestCloudFront_EdgeCustomErrorPageForSignedCookie403 — the edge's own 403
// for a request to a trusted-key-group behaviour without signed cookies is
// replaced by the custom 403 page.
func TestCloudFront_EdgeCustomErrorPageForSignedCookie403(t *testing.T) {
	t.Parallel()

	_, distID := createCustomErrorDistribution(t, newCustomErrorBucket(t), newTrustedKeyGroupBehavior(t),
		customErrorResponse(403, "/errors/403.html", "403"))

	resp, body := callEdgePath(t, http.MethodGet, distID, "/index.html", http.NoBody, "")
	if resp.StatusCode != http.StatusForbidden || string(body) != customErrorPage403 {
		t.Errorf("got %d %q, want 403 %q", resp.StatusCode, body, customErrorPage403)
	}
}

// TestCloudFront_EdgeCustomErrorPageMissing — when the error page itself is
// missing, the viewer gets the status the bucket returned for the page (404)
// instead of the original 403 or the configured response code.
func TestCloudFront_EdgeCustomErrorPageMissing(t *testing.T) {
	t.Parallel()

	_, distID := createCustomErrorDistribution(t, newCustomErrorBucket(t), newTrustedKeyGroupBehavior(t),
		customErrorResponse(403, "/errors/deleted.html", "200"))

	resp, body := callEdgePath(t, http.MethodGet, distID, "/index.html", http.NoBody, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got %d %q, want the bucket's 404 for the missing page", resp.StatusCode, body)
	}
}
