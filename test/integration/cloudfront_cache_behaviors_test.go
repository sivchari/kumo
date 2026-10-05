//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

// cacheBehaviorGoldenIgnores hides the fields that differ between runs.
var cacheBehaviorGoldenIgnores = golden.WithIgnoreFields("ResultMetadata", "Id", "ARN", "DomainName", "ETag", "LastModifiedTime", "CallerReference", "Location")

const (
	apiPathPattern        = "/api/*"
	cachingDisabledID     = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
	allViewerExceptHostID = "b689b0a8-53d0-40ab-baf2-68738e2966ac"
)

func assertCloudFrontAPIError(t *testing.T, err error, wantCode string, wantStatus int) {
	t.Helper()

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != wantCode {
		t.Fatalf("expected %s, got %v", wantCode, err)
	}

	var respErr *awshttp.ResponseError
	if !errors.As(err, &respErr) || respErr.HTTPStatusCode() != wantStatus {
		t.Fatalf("expected HTTP %d, got %v", wantStatus, err)
	}
}

func customOrigin(id, domain string) cloudfronttypes.Origin {
	return cloudfronttypes.Origin{
		Id:         aws.String(id),
		DomainName: aws.String(domain),
		CustomOriginConfig: &cloudfronttypes.CustomOriginConfig{
			HTTPPort:             aws.Int32(80),
			HTTPSPort:            aws.Int32(443),
			OriginProtocolPolicy: cloudfronttypes.OriginProtocolPolicyHttpsOnly,
		},
	}
}

// apiCacheBehavior is an ordered behaviour exercising every field the
// Terraform provider reads back.
func apiCacheBehavior(pattern, target string) cloudfronttypes.CacheBehavior {
	return cloudfronttypes.CacheBehavior{
		PathPattern:             aws.String(pattern),
		TargetOriginId:          aws.String(target),
		ViewerProtocolPolicy:    cloudfronttypes.ViewerProtocolPolicyRedirectToHttps,
		AllowedMethods:          &cloudfronttypes.AllowedMethods{Quantity: aws.Int32(3), Items: []cloudfronttypes.Method{cloudfronttypes.MethodGet, cloudfronttypes.MethodHead, cloudfronttypes.MethodOptions}, CachedMethods: &cloudfronttypes.CachedMethods{Quantity: aws.Int32(2), Items: []cloudfronttypes.Method{cloudfronttypes.MethodGet, cloudfronttypes.MethodHead}}},
		Compress:                aws.Bool(true),
		SmoothStreaming:         aws.Bool(false),
		CachePolicyId:           aws.String(cachingDisabledID),
		OriginRequestPolicyId:   aws.String(allViewerExceptHostID),
		ResponseHeadersPolicyId: aws.String("67f7725c-6f97-4210-82d7-5512b31e9d03"),
		TrustedSigners:          &cloudfronttypes.TrustedSigners{Enabled: aws.Bool(false), Quantity: aws.Int32(0)},
		TrustedKeyGroups:        &cloudfronttypes.TrustedKeyGroups{Enabled: aws.Bool(false), Quantity: aws.Int32(0)},
	}
}

// legacyCacheBehavior uses the legacy ForwardedValues shape, including the
// nested cookie and query string whitelists.
func legacyCacheBehavior(pattern, target string) cloudfronttypes.CacheBehavior {
	return cloudfronttypes.CacheBehavior{
		PathPattern:          aws.String(pattern),
		TargetOriginId:       aws.String(target),
		ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
		MinTTL:               aws.Int64(0),
		DefaultTTL:           aws.Int64(300),
		MaxTTL:               aws.Int64(3600),
		ForwardedValues: &cloudfronttypes.ForwardedValues{
			QueryString:          aws.Bool(true),
			QueryStringCacheKeys: &cloudfronttypes.QueryStringCacheKeys{Quantity: aws.Int32(1), Items: []string{"v"}},
			Cookies:              &cloudfronttypes.CookiePreference{Forward: cloudfronttypes.ItemSelectionWhitelist, WhitelistedNames: &cloudfronttypes.CookieNames{Quantity: aws.Int32(1), Items: []string{"session"}}},
			Headers:              &cloudfronttypes.Headers{Quantity: aws.Int32(1), Items: []string{"Authorization"}},
		},
	}
}

func TestCloudFront_OrderedCacheBehaviorsRoundTrip(t *testing.T) {
	client := newCloudFrontClient(t)
	ctx := t.Context()
	ref := "test-cf-cache-behaviors-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	config := &cloudfronttypes.DistributionConfig{
		CallerReference: aws.String(ref),
		Comment:         aws.String("ordered cache behaviors"),
		Enabled:         aws.Bool(true),
		Origins: &cloudfronttypes.Origins{Quantity: aws.Int32(2), Items: []cloudfronttypes.Origin{
			customOrigin("assets", "assets.example.com"),
			customOrigin("api", "api.example.com"),
		}},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
			TargetOriginId:          aws.String("assets"),
			ViewerProtocolPolicy:    cloudfronttypes.ViewerProtocolPolicyRedirectToHttps,
			Compress:                aws.Bool(true),
			CachePolicyId:           aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			ResponseHeadersPolicyId: aws.String("67f7725c-6f97-4210-82d7-5512b31e9d03"),
		},
		CacheBehaviors: &cloudfronttypes.CacheBehaviors{Quantity: aws.Int32(2), Items: []cloudfronttypes.CacheBehavior{
			apiCacheBehavior(apiPathPattern, "api"),
			legacyCacheBehavior("/legacy/*", "assets"),
		}},
	}

	created, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{DistributionConfig: config})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{Id: created.Distribution.Id, IfMatch: created.ETag})
	})

	golden.New(t, cacheBehaviorGoldenIgnores).Assert(t.Name()+"_create", created)

	got, err := client.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: created.Distribution.Id})
	if err != nil {
		t.Fatalf("GetDistributionConfig: %v", err)
	}

	golden.New(t, cacheBehaviorGoldenIgnores).Assert(t.Name()+"_config", got)

	// Reorder and rename: the update must replace the list wholesale.
	config.CacheBehaviors = &cloudfronttypes.CacheBehaviors{Quantity: aws.Int32(1), Items: []cloudfronttypes.CacheBehavior{apiCacheBehavior("/v2/*", "api")}}

	updated, err := client.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{Id: created.Distribution.Id, IfMatch: got.ETag, DistributionConfig: config})
	if err != nil {
		t.Fatalf("UpdateDistribution: %v", err)
	}

	golden.New(t, cacheBehaviorGoldenIgnores).Assert(t.Name()+"_update", updated)

	listed, err := client.ListDistributions(ctx, &cloudfront.ListDistributionsInput{})
	if err != nil {
		t.Fatalf("ListDistributions: %v", err)
	}

	for _, item := range listed.DistributionList.Items {
		if aws.ToString(item.Id) == aws.ToString(created.Distribution.Id) {
			if aws.ToInt32(item.CacheBehaviors.Quantity) != 1 || aws.ToString(item.CacheBehaviors.Items[0].PathPattern) != "/v2/*" {
				t.Errorf("summary CacheBehaviors = %+v", item.CacheBehaviors)
			}

			return
		}
	}

	t.Fatal("distribution missing from ListDistributions")
}

func TestCloudFront_CacheBehaviorTargetMustExist(t *testing.T) {
	client := newCloudFrontClient(t)

	_, err := client.CreateDistribution(t.Context(), &cloudfront.CreateDistributionInput{DistributionConfig: &cloudfronttypes.DistributionConfig{
		CallerReference: aws.String("test-cf-missing-target-" + strconv.FormatInt(time.Now().UnixNano(), 36)),
		Comment:         aws.String("missing target"),
		Enabled:         aws.Bool(true),
		Origins:         &cloudfronttypes.Origins{Quantity: aws.Int32(1), Items: []cloudfronttypes.Origin{customOrigin("assets", "assets.example.com")}},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
			TargetOriginId:       aws.String("assets"),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
			CachePolicyId:        aws.String(cachingDisabledID),
		},
		CacheBehaviors: &cloudfronttypes.CacheBehaviors{Quantity: aws.Int32(1), Items: []cloudfronttypes.CacheBehavior{apiCacheBehavior(apiPathPattern, "missing")}},
	}})
	assertCloudFrontAPIError(t, err, "NoSuchOrigin", http.StatusNotFound)
}

func TestCloudFront_CacheBehaviorPathPatternInvalid(t *testing.T) {
	client := newCloudFrontClient(t)

	for name, patterns := range map[string][]string{
		"duplicate": {apiPathPattern, apiPathPattern},
		"empty":     {apiPathPattern, ""},
	} {
		items := make([]cloudfronttypes.CacheBehavior, 0, len(patterns))
		for _, pattern := range patterns {
			items = append(items, apiCacheBehavior(pattern, "assets"))
		}

		_, err := client.CreateDistribution(t.Context(), &cloudfront.CreateDistributionInput{DistributionConfig: &cloudfronttypes.DistributionConfig{
			CallerReference: aws.String("test-cf-" + name + "-pattern-" + strconv.FormatInt(time.Now().UnixNano(), 36)),
			Comment:         aws.String(name + " path pattern"),
			Enabled:         aws.Bool(true),
			Origins:         &cloudfronttypes.Origins{Quantity: aws.Int32(1), Items: []cloudfronttypes.Origin{customOrigin("assets", "assets.example.com")}},
			DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
				TargetOriginId:       aws.String("assets"),
				ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String(cachingDisabledID),
			},
			CacheBehaviors: &cloudfronttypes.CacheBehaviors{Quantity: aws.Int32(int32(len(items))), Items: items},
		}})
		assertCloudFrontAPIError(t, err, "InvalidArgument", http.StatusBadRequest)
	}
}

// TestCloudFront_EdgeRoutesOrderedCacheBehaviors —`/api/*` reaches the api
// function URL, everything else the default one, through the edge.
func TestCloudFront_EdgeRoutesOrderedCacheBehaviors(t *testing.T) {
	assets := newFunctionURLEcho(t)
	api := newFunctionURLEcho(t)
	assetsDomain := createFunctionURL(t, "test-cf-behaviors-assets", assets.url, lambdatypes.FunctionUrlAuthTypeNone, nil) + ".lambda-url.us-east-1.on.aws"
	apiDomain := createFunctionURL(t, "test-cf-behaviors-api", api.url, lambdatypes.FunctionUrlAuthTypeNone, nil) + ".lambda-url.us-east-1.on.aws"

	client := newCloudFrontClient(t)

	created, err := client.CreateDistribution(t.Context(), &cloudfront.CreateDistributionInput{DistributionConfig: &cloudfronttypes.DistributionConfig{
		CallerReference: aws.String("test-cf-behaviors-edge-" + strconv.FormatInt(time.Now().UnixNano(), 36)),
		Comment:         aws.String("edge routing"),
		Enabled:         aws.Bool(true),
		Origins:         &cloudfronttypes.Origins{Quantity: aws.Int32(2), Items: []cloudfronttypes.Origin{customOrigin("assets", assetsDomain), customOrigin("api", apiDomain)}},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
			TargetOriginId:       aws.String("assets"),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
			CachePolicyId:        aws.String(cachingDisabledID),
		},
		CacheBehaviors: &cloudfronttypes.CacheBehaviors{Quantity: aws.Int32(1), Items: []cloudfronttypes.CacheBehavior{apiCacheBehavior(apiPathPattern, "api")}},
	}})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{Id: created.Distribution.Id, IfMatch: created.ETag})
	})

	distID := aws.ToString(created.Distribution.Id)

	for path, want := range map[string]string{"/api/items": apiDomain, "/index.html": assetsDomain, "/apix": assetsDomain} {
		resp, body := callEdgePath(t, http.MethodGet, distID, path, http.NoBody, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d %s", path, resp.StatusCode, body)
		}

		ctx, _ := decodeEvent(t, body)["requestContext"].(map[string]any)
		if ctx["domainName"] != want {
			t.Errorf("%s reached %v, want %s", path, ctx["domainName"], want)
		}
	}

	if assets.invocations.Load() != 2 || api.invocations.Load() != 1 {
		t.Errorf("invocations assets=%d api=%d, want 2 and 1", assets.invocations.Load(), api.invocations.Load())
	}
}
