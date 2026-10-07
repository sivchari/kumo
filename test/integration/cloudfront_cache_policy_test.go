//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/sivchari/golden"
)

// cachePolicyGoldenIgnores hides the per-run identifiers.
var cachePolicyGoldenIgnores = golden.WithIgnoreFields("ResultMetadata", "Id", "ETag", "Location", "LastModifiedTime", "Name")

const managedCachingOptimizedID = "658327ea-f89d-4fab-a63d-7e88639e58f6"

// cachePolicyConfig exercises every member of the configuration, including
// explicit zero and false values.
func cachePolicyConfig(name string) *cloudfronttypes.CachePolicyConfig {
	return &cloudfronttypes.CachePolicyConfig{
		Name:       aws.String(name),
		Comment:    aws.String("kumo integration test"),
		DefaultTTL: aws.Int64(3600),
		MaxTTL:     aws.Int64(86400),
		MinTTL:     aws.Int64(0),
		ParametersInCacheKeyAndForwardedToOrigin: &cloudfronttypes.ParametersInCacheKeyAndForwardedToOrigin{
			EnableAcceptEncodingGzip:   aws.Bool(false),
			EnableAcceptEncodingBrotli: aws.Bool(true),
			HeadersConfig: &cloudfronttypes.CachePolicyHeadersConfig{
				HeaderBehavior: cloudfronttypes.CachePolicyHeaderBehaviorWhitelist,
				Headers:        &cloudfronttypes.Headers{Quantity: aws.Int32(2), Items: []string{"Origin", "Accept-Language"}},
			},
			CookiesConfig: &cloudfronttypes.CachePolicyCookiesConfig{
				CookieBehavior: cloudfronttypes.CachePolicyCookieBehaviorAllExcept,
				Cookies:        &cloudfronttypes.CookieNames{Quantity: aws.Int32(1), Items: []string{"tracking"}},
			},
			QueryStringsConfig: &cloudfronttypes.CachePolicyQueryStringsConfig{
				QueryStringBehavior: cloudfronttypes.CachePolicyQueryStringBehaviorWhitelist,
				QueryStrings:        &cloudfronttypes.QueryStringNames{Quantity: aws.Int32(1), Items: []string{"v"}},
			},
		},
	}
}

// createCachePolicy creates a cache policy and deletes it on cleanup (with
// whatever ETag it has by then).
func createCachePolicy(t *testing.T, client *cloudfront.Client, cfg *cloudfronttypes.CachePolicyConfig) *cloudfront.CreateCachePolicyOutput {
	t.Helper()

	created, err := client.CreateCachePolicy(t.Context(), &cloudfront.CreateCachePolicyInput{CachePolicyConfig: cfg})
	if err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()

		current, err := client.GetCachePolicy(ctx, &cloudfront.GetCachePolicyInput{Id: created.CachePolicy.Id})
		if err != nil {
			return
		}

		_, _ = client.DeleteCachePolicy(ctx, &cloudfront.DeleteCachePolicyInput{Id: created.CachePolicy.Id, IfMatch: current.ETag})
	})

	return created
}

// assertCachePolicyUnchanged checks that a rejected mutation left the policy as it was.
func assertCachePolicyUnchanged(t *testing.T, client *cloudfront.Client, id, etag *string, name string) {
	t.Helper()

	got, err := client.GetCachePolicy(t.Context(), &cloudfront.GetCachePolicyInput{Id: id})
	if err != nil {
		t.Fatalf("GetCachePolicy: %v", err)
	}

	if aws.ToString(got.ETag) != aws.ToString(etag) || aws.ToString(got.CachePolicy.CachePolicyConfig.Name) != name {
		t.Fatalf("rejected mutation changed the policy: ETag %q (want %q), Name %q (want %q)", aws.ToString(got.ETag), aws.ToString(etag), aws.ToString(got.CachePolicy.CachePolicyConfig.Name), name)
	}
}

// cachePolicyDistribution creates a distribution whose default (or, with
// orderedPolicyID, ordered) cache behavior references the given policies.
func cachePolicyDistribution(t *testing.T, client *cloudfront.Client, enabled bool, defaultPolicyID, orderedPolicyID string) *cloudfront.CreateDistributionOutput {
	t.Helper()

	cfg := &cloudfronttypes.DistributionConfig{
		CallerReference:      aws.String(uniqueName("test-cf-cache-policy-ref")),
		Comment:              aws.String("references a cache policy"),
		Enabled:              aws.Bool(enabled),
		Origins:              &cloudfronttypes.Origins{Quantity: aws.Int32(1), Items: []cloudfronttypes.Origin{customOrigin("web", "www.example.com")}},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{TargetOriginId: aws.String("web"), ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll, CachePolicyId: aws.String(defaultPolicyID)},
	}

	if orderedPolicyID != "" {
		cfg.CacheBehaviors = &cloudfronttypes.CacheBehaviors{Quantity: aws.Int32(1), Items: []cloudfronttypes.CacheBehavior{{
			PathPattern:          aws.String(apiPathPattern),
			TargetOriginId:       aws.String("web"),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
			CachePolicyId:        aws.String(orderedPolicyID),
		}}}
	}

	dist, err := client.CreateDistribution(t.Context(), &cloudfront.CreateDistributionInput{DistributionConfig: cfg})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()

		current, err := client.GetDistribution(ctx, &cloudfront.GetDistributionInput{Id: dist.Distribution.Id})
		if err != nil {
			return
		}

		_, _ = client.DeleteDistribution(ctx, &cloudfront.DeleteDistributionInput{Id: dist.Distribution.Id, IfMatch: current.ETag})
	})

	return dist
}

func TestCloudFront_CachePolicyLifecycle(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := uniqueName("test-cf-cache-policy-lifecycle")

	created := createCachePolicy(t, client, cachePolicyConfig(name))
	golden.New(t, cachePolicyGoldenIgnores).Assert(t.Name()+"_create", created)

	id := created.CachePolicy.Id

	if aws.ToString(created.Location) != "/2020-05-31/cache-policy/"+aws.ToString(id) || aws.ToString(created.ETag) == "" || created.CachePolicy.LastModifiedTime == nil {
		t.Errorf("Location = %q, ETag = %q, LastModifiedTime = %v", aws.ToString(created.Location), aws.ToString(created.ETag), created.CachePolicy.LastModifiedTime)
	}

	got, err := client.GetCachePolicy(ctx, &cloudfront.GetCachePolicyInput{Id: id})
	if err != nil {
		t.Fatalf("GetCachePolicy: %v", err)
	}

	golden.New(t, cachePolicyGoldenIgnores).Assert(t.Name()+"_get", got)

	config, err := client.GetCachePolicyConfig(ctx, &cloudfront.GetCachePolicyConfigInput{Id: id})
	if err != nil {
		t.Fatalf("GetCachePolicyConfig: %v", err)
	}

	golden.New(t, cachePolicyGoldenIgnores).Assert(t.Name()+"_config", config)

	if aws.ToString(config.ETag) != aws.ToString(got.ETag) || aws.ToString(config.CachePolicyConfig.Name) != name {
		t.Errorf("config ETag %q != get ETag %q or name %q", aws.ToString(config.ETag), aws.ToString(got.ETag), aws.ToString(config.CachePolicyConfig.Name))
	}

	// Update the TTLs, the header list and the comment; drop the optional
	// Brotli flag and the cookie list.
	next := cachePolicyConfig(name)
	next.Comment = aws.String("updated")
	next.DefaultTTL = aws.Int64(0)
	next.MaxTTL = aws.Int64(600)
	next.MinTTL = aws.Int64(0)
	next.ParametersInCacheKeyAndForwardedToOrigin.EnableAcceptEncodingGzip = aws.Bool(true)
	next.ParametersInCacheKeyAndForwardedToOrigin.EnableAcceptEncodingBrotli = nil
	next.ParametersInCacheKeyAndForwardedToOrigin.HeadersConfig.Headers = &cloudfronttypes.Headers{Quantity: aws.Int32(1), Items: []string{"Host"}}
	next.ParametersInCacheKeyAndForwardedToOrigin.CookiesConfig = &cloudfronttypes.CachePolicyCookiesConfig{CookieBehavior: cloudfronttypes.CachePolicyCookieBehaviorNone}

	updated, err := client.UpdateCachePolicy(ctx, &cloudfront.UpdateCachePolicyInput{Id: id, IfMatch: got.ETag, CachePolicyConfig: next})
	if err != nil {
		t.Fatalf("UpdateCachePolicy: %v", err)
	}

	golden.New(t, cachePolicyGoldenIgnores).Assert(t.Name()+"_update", updated)

	if aws.ToString(updated.ETag) == aws.ToString(got.ETag) {
		t.Error("update must issue a new ETag")
	}

	_, err = client.UpdateCachePolicy(ctx, &cloudfront.UpdateCachePolicyInput{Id: id, IfMatch: got.ETag, CachePolicyConfig: cachePolicyConfig(name)})
	assertCloudFrontAPIError(t, err, "PreconditionFailed", http.StatusPreconditionFailed)

	listed, err := client.ListCachePolicies(ctx, &cloudfront.ListCachePoliciesInput{Type: cloudfronttypes.CachePolicyTypeCustom})
	if err != nil {
		t.Fatalf("ListCachePolicies: %v", err)
	}

	found := false

	for _, item := range listed.CachePolicyList.Items {
		if aws.ToString(item.CachePolicy.Id) == aws.ToString(id) {
			found = item.Type == cloudfronttypes.CachePolicyTypeCustom && aws.ToString(item.CachePolicy.CachePolicyConfig.Comment) == "updated"
		}
	}

	if !found {
		t.Errorf("updated policy missing from the list: %+v", listed.CachePolicyList)
	}

	if _, err := client.DeleteCachePolicy(ctx, &cloudfront.DeleteCachePolicyInput{Id: id, IfMatch: updated.ETag}); err != nil {
		t.Fatalf("DeleteCachePolicy: %v", err)
	}

	_, err = client.GetCachePolicy(ctx, &cloudfront.GetCachePolicyInput{Id: id})
	assertCloudFrontAPIError(t, err, "NoSuchCachePolicy", http.StatusNotFound)

	// The name is free again once the policy is gone.
	createCachePolicy(t, client, cachePolicyConfig(name))
}

func TestCloudFront_CachePolicyErrors(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := uniqueName("test-cf-cache-policy-errors")
	otherName := uniqueName("test-cf-cache-policy-other")

	created := createCachePolicy(t, client, cachePolicyConfig(name))
	other := createCachePolicy(t, client, cachePolicyConfig(otherName))
	id := created.CachePolicy.Id

	_, err := client.CreateCachePolicy(ctx, &cloudfront.CreateCachePolicyInput{CachePolicyConfig: cachePolicyConfig(name)})
	assertCloudFrontAPIError(t, err, "CachePolicyAlreadyExists", http.StatusConflict)

	_, err = client.UpdateCachePolicy(ctx, &cloudfront.UpdateCachePolicyInput{Id: other.CachePolicy.Id, IfMatch: other.ETag, CachePolicyConfig: cachePolicyConfig(name)})
	assertCloudFrontAPIError(t, err, "CachePolicyAlreadyExists", http.StatusConflict)
	assertCachePolicyUnchanged(t, client, other.CachePolicy.Id, other.ETag, otherName)

	renamed := cachePolicyConfig(uniqueName("test-cf-cache-policy-renamed"))

	_, err = client.UpdateCachePolicy(ctx, &cloudfront.UpdateCachePolicyInput{Id: id, CachePolicyConfig: renamed})
	assertCloudFrontAPIError(t, err, "InvalidIfMatchVersion", http.StatusBadRequest)

	_, err = client.UpdateCachePolicy(ctx, &cloudfront.UpdateCachePolicyInput{Id: id, IfMatch: aws.String("EStale"), CachePolicyConfig: renamed})
	assertCloudFrontAPIError(t, err, "PreconditionFailed", http.StatusPreconditionFailed)

	_, err = client.DeleteCachePolicy(ctx, &cloudfront.DeleteCachePolicyInput{Id: id})
	assertCloudFrontAPIError(t, err, "InvalidIfMatchVersion", http.StatusBadRequest)

	_, err = client.DeleteCachePolicy(ctx, &cloudfront.DeleteCachePolicyInput{Id: id, IfMatch: aws.String("EStale")})
	assertCloudFrontAPIError(t, err, "PreconditionFailed", http.StatusPreconditionFailed)

	assertCachePolicyUnchanged(t, client, id, created.ETag, name)

	missing := aws.String("00000000-0000-0000-0000-000000000000")

	_, err = client.GetCachePolicy(ctx, &cloudfront.GetCachePolicyInput{Id: missing})
	assertCloudFrontAPIError(t, err, "NoSuchCachePolicy", http.StatusNotFound)

	_, err = client.GetCachePolicyConfig(ctx, &cloudfront.GetCachePolicyConfigInput{Id: missing})
	assertCloudFrontAPIError(t, err, "NoSuchCachePolicy", http.StatusNotFound)

	_, err = client.UpdateCachePolicy(ctx, &cloudfront.UpdateCachePolicyInput{Id: missing, IfMatch: aws.String("E1"), CachePolicyConfig: renamed})
	assertCloudFrontAPIError(t, err, "NoSuchCachePolicy", http.StatusNotFound)

	_, err = client.DeleteCachePolicy(ctx, &cloudfront.DeleteCachePolicyInput{Id: missing, IfMatch: aws.String("E1")})
	assertCloudFrontAPIError(t, err, "NoSuchCachePolicy", http.StatusNotFound)

	inconsistent := cachePolicyConfig(uniqueName("test-cf-cache-policy-quantity"))
	inconsistent.ParametersInCacheKeyAndForwardedToOrigin.HeadersConfig.Headers.Quantity = aws.Int32(3)

	_, err = client.CreateCachePolicy(ctx, &cloudfront.CreateCachePolicyInput{CachePolicyConfig: inconsistent})
	assertCloudFrontAPIError(t, err, "InconsistentQuantities", http.StatusBadRequest)

	// Keeping its own name is not a conflict.
	same, err := client.UpdateCachePolicy(ctx, &cloudfront.UpdateCachePolicyInput{Id: id, IfMatch: created.ETag, CachePolicyConfig: cachePolicyConfig(name)})
	if err != nil {
		t.Fatalf("UpdateCachePolicy keeping its name: %v", err)
	}

	if aws.ToString(same.ETag) == aws.ToString(created.ETag) {
		t.Error("update must issue a new ETag")
	}
}

func TestCloudFront_CachePolicyInUse(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	t.Run("default behavior", func(t *testing.T) {
		policy := createCachePolicy(t, client, cachePolicyConfig(uniqueName("test-cf-cache-policy-default")))
		dist := cachePolicyDistribution(t, client, true, aws.ToString(policy.CachePolicy.Id), "")

		_, err := client.DeleteCachePolicy(ctx, &cloudfront.DeleteCachePolicyInput{Id: policy.CachePolicy.Id, IfMatch: policy.ETag})
		assertCloudFrontAPIError(t, err, "CachePolicyInUse", http.StatusConflict)
		assertCachePolicyUnchanged(t, client, policy.CachePolicy.Id, policy.ETag, aws.ToString(policy.CachePolicy.CachePolicyConfig.Name))

		// Point the behavior at a managed policy; the custom one is then free.
		current, err := client.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: dist.Distribution.Id})
		if err != nil {
			t.Fatalf("GetDistributionConfig: %v", err)
		}

		current.DistributionConfig.DefaultCacheBehavior.CachePolicyId = aws.String(managedCachingOptimizedID)

		if _, err := client.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{Id: dist.Distribution.Id, IfMatch: current.ETag, DistributionConfig: current.DistributionConfig}); err != nil {
			t.Fatalf("UpdateDistribution: %v", err)
		}

		if _, err := client.DeleteCachePolicy(ctx, &cloudfront.DeleteCachePolicyInput{Id: policy.CachePolicy.Id, IfMatch: policy.ETag}); err != nil {
			t.Fatalf("DeleteCachePolicy after detaching: %v", err)
		}
	})

	t.Run("ordered behavior of a disabled distribution", func(t *testing.T) {
		policy := createCachePolicy(t, client, cachePolicyConfig(uniqueName("test-cf-cache-policy-ordered")))
		dist := cachePolicyDistribution(t, client, false, managedCachingOptimizedID, aws.ToString(policy.CachePolicy.Id))

		_, err := client.DeleteCachePolicy(ctx, &cloudfront.DeleteCachePolicyInput{Id: policy.CachePolicy.Id, IfMatch: policy.ETag})
		assertCloudFrontAPIError(t, err, "CachePolicyInUse", http.StatusConflict)

		if _, err := client.DeleteDistribution(ctx, &cloudfront.DeleteDistributionInput{Id: dist.Distribution.Id, IfMatch: dist.ETag}); err != nil {
			t.Fatalf("DeleteDistribution: %v", err)
		}

		if _, err := client.DeleteCachePolicy(ctx, &cloudfront.DeleteCachePolicyInput{Id: policy.CachePolicy.Id, IfMatch: policy.ETag}); err != nil {
			t.Fatalf("DeleteCachePolicy after the distribution is gone: %v", err)
		}
	})
}

func TestCloudFront_ListCachePolicies(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	want := map[string]bool{}

	for range 3 {
		created := createCachePolicy(t, client, cachePolicyConfig(uniqueName("test-cf-cache-policy-list")))
		want[aws.ToString(created.CachePolicy.Id)] = true
	}

	var marker *string

	seen := map[string]bool{}

	for {
		page, err := client.ListCachePolicies(ctx, &cloudfront.ListCachePoliciesInput{Type: cloudfronttypes.CachePolicyTypeCustom, MaxItems: aws.Int32(1), Marker: marker})
		if err != nil {
			t.Fatalf("ListCachePolicies: %v", err)
		}

		list := page.CachePolicyList
		if aws.ToInt32(list.MaxItems) != 1 || aws.ToInt32(list.Quantity) != int32(len(list.Items)) || len(list.Items) > 1 {
			t.Fatalf("page shape: MaxItems %d, Quantity %d, %d items", aws.ToInt32(list.MaxItems), aws.ToInt32(list.Quantity), len(list.Items))
		}

		for _, item := range list.Items {
			if item.Type != cloudfronttypes.CachePolicyTypeCustom {
				t.Errorf("Type = %q, want custom", item.Type)
			}

			seen[aws.ToString(item.CachePolicy.Id)] = true
		}

		if list.NextMarker == nil {
			break
		}

		marker = list.NextMarker
	}

	for id := range want {
		if !seen[id] {
			t.Errorf("policy %s missing from the paginated list", id)
		}
	}

	managed, err := client.ListCachePolicies(ctx, &cloudfront.ListCachePoliciesInput{Type: cloudfronttypes.CachePolicyTypeManaged})
	if err != nil {
		t.Fatalf("ListCachePolicies managed: %v", err)
	}

	if aws.ToInt32(managed.CachePolicyList.Quantity) != 0 || len(managed.CachePolicyList.Items) != 0 {
		t.Errorf("managed list = %+v, want empty (kumo seeds no managed policies)", managed.CachePolicyList)
	}

	_, err = client.ListCachePolicies(ctx, &cloudfront.ListCachePoliciesInput{Type: cloudfronttypes.CachePolicyType("builtin")})
	assertCloudFrontAPIError(t, err, "InvalidArgument", http.StatusBadRequest)
}
