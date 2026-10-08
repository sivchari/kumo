//go:build integration

package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/sivchari/golden"
)

// responseHeadersPolicyGoldenIgnores hides the per-run identifiers.
var responseHeadersPolicyGoldenIgnores = golden.WithIgnoreFields("ResultMetadata", "Id", "ETag", "Location", "LastModifiedTime", "Name")

// responseHeadersPolicyConfig exercises every member of the configuration,
// including explicit zero and false values.
func responseHeadersPolicyConfig(name string) *cloudfronttypes.ResponseHeadersPolicyConfig {
	return &cloudfronttypes.ResponseHeadersPolicyConfig{
		Name:    aws.String(name),
		Comment: aws.String("kumo integration test"),
		CorsConfig: &cloudfronttypes.ResponseHeadersPolicyCorsConfig{
			AccessControlAllowCredentials: aws.Bool(false),
			AccessControlAllowHeaders:     &cloudfronttypes.ResponseHeadersPolicyAccessControlAllowHeaders{Quantity: aws.Int32(2), Items: []string{"Authorization", "Content-Type"}},
			AccessControlAllowMethods: &cloudfronttypes.ResponseHeadersPolicyAccessControlAllowMethods{Quantity: aws.Int32(2), Items: []cloudfronttypes.ResponseHeadersPolicyAccessControlAllowMethodsValues{
				cloudfronttypes.ResponseHeadersPolicyAccessControlAllowMethodsValuesGet,
				cloudfronttypes.ResponseHeadersPolicyAccessControlAllowMethodsValuesOptions,
			}},
			AccessControlAllowOrigins:  &cloudfronttypes.ResponseHeadersPolicyAccessControlAllowOrigins{Quantity: aws.Int32(1), Items: []string{"https://www.example.com"}},
			AccessControlExposeHeaders: &cloudfronttypes.ResponseHeadersPolicyAccessControlExposeHeaders{Quantity: aws.Int32(1), Items: []string{"X-Request-Id"}},
			AccessControlMaxAgeSec:     aws.Int32(0),
			OriginOverride:             aws.Bool(true),
		},
		CustomHeadersConfig: &cloudfronttypes.ResponseHeadersPolicyCustomHeadersConfig{Quantity: aws.Int32(2), Items: []cloudfronttypes.ResponseHeadersPolicyCustomHeader{
			{Header: aws.String("X-Kumo"), Value: aws.String("1"), Override: aws.Bool(false)},
			{Header: aws.String("X-Empty"), Value: aws.String(""), Override: aws.Bool(true)},
		}},
		RemoveHeadersConfig: &cloudfronttypes.ResponseHeadersPolicyRemoveHeadersConfig{Quantity: aws.Int32(1), Items: []cloudfronttypes.ResponseHeadersPolicyRemoveHeader{
			{Header: aws.String("X-Powered-By")},
		}},
		SecurityHeadersConfig: &cloudfronttypes.ResponseHeadersPolicySecurityHeadersConfig{
			ContentSecurityPolicy: &cloudfronttypes.ResponseHeadersPolicyContentSecurityPolicy{ContentSecurityPolicy: aws.String("default-src 'self'"), Override: aws.Bool(true)},
			ContentTypeOptions:    &cloudfronttypes.ResponseHeadersPolicyContentTypeOptions{Override: aws.Bool(false)},
			FrameOptions:          &cloudfronttypes.ResponseHeadersPolicyFrameOptions{FrameOption: cloudfronttypes.FrameOptionsListSameorigin, Override: aws.Bool(true)},
			ReferrerPolicy:        &cloudfronttypes.ResponseHeadersPolicyReferrerPolicy{ReferrerPolicy: cloudfronttypes.ReferrerPolicyListStrictOriginWhenCrossOrigin, Override: aws.Bool(false)},
			StrictTransportSecurity: &cloudfronttypes.ResponseHeadersPolicyStrictTransportSecurity{
				AccessControlMaxAgeSec: aws.Int32(31536000),
				IncludeSubdomains:      aws.Bool(true),
				Preload:                aws.Bool(false),
				Override:               aws.Bool(true),
			},
			XSSProtection: &cloudfronttypes.ResponseHeadersPolicyXSSProtection{Override: aws.Bool(false), Protection: aws.Bool(true), ModeBlock: aws.Bool(false), ReportUri: aws.String("https://www.example.com/xss")},
		},
		ServerTimingHeadersConfig: &cloudfronttypes.ResponseHeadersPolicyServerTimingHeadersConfig{Enabled: aws.Bool(true), SamplingRate: aws.Float64(12.5)},
	}
}

// createResponseHeadersPolicy creates a response headers policy and deletes
// it on cleanup (with whatever ETag it has by then).
func createResponseHeadersPolicy(t *testing.T, client *cloudfront.Client, cfg *cloudfronttypes.ResponseHeadersPolicyConfig) *cloudfront.CreateResponseHeadersPolicyOutput {
	t.Helper()

	created, err := client.CreateResponseHeadersPolicy(t.Context(), &cloudfront.CreateResponseHeadersPolicyInput{ResponseHeadersPolicyConfig: cfg})
	if err != nil {
		t.Fatalf("CreateResponseHeadersPolicy: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()

		current, err := client.GetResponseHeadersPolicy(ctx, &cloudfront.GetResponseHeadersPolicyInput{Id: created.ResponseHeadersPolicy.Id})
		if err != nil {
			return
		}

		_, _ = client.DeleteResponseHeadersPolicy(ctx, &cloudfront.DeleteResponseHeadersPolicyInput{Id: created.ResponseHeadersPolicy.Id, IfMatch: current.ETag})
	})

	return created
}

// assertResponseHeadersPolicyUnchanged checks that a rejected mutation left the policy as it was.
func assertResponseHeadersPolicyUnchanged(t *testing.T, client *cloudfront.Client, id, etag *string, name string) {
	t.Helper()

	got, err := client.GetResponseHeadersPolicy(t.Context(), &cloudfront.GetResponseHeadersPolicyInput{Id: id})
	if err != nil {
		t.Fatalf("GetResponseHeadersPolicy: %v", err)
	}

	cfg := got.ResponseHeadersPolicy.ResponseHeadersPolicyConfig
	if aws.ToString(got.ETag) != aws.ToString(etag) || aws.ToString(cfg.Name) != name || aws.ToInt32(cfg.CustomHeadersConfig.Quantity) != 2 {
		t.Fatalf("rejected mutation changed the policy: ETag %q (want %q), Name %q (want %q)", aws.ToString(got.ETag), aws.ToString(etag), aws.ToString(cfg.Name), name)
	}
}

// responseHeadersPolicyDistribution creates a distribution whose default (or,
// with orderedPolicyID, ordered) cache behavior references the given policies.
func responseHeadersPolicyDistribution(t *testing.T, client *cloudfront.Client, enabled bool, defaultPolicyID, orderedPolicyID string) *cloudfront.CreateDistributionOutput {
	t.Helper()

	cfg := &cloudfronttypes.DistributionConfig{
		CallerReference: aws.String(uniqueName("test-cf-rhp-ref")),
		Comment:         aws.String("references a response headers policy"),
		Enabled:         aws.Bool(enabled),
		Origins:         &cloudfronttypes.Origins{Quantity: aws.Int32(1), Items: []cloudfronttypes.Origin{customOrigin("web", "www.example.com")}},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{
			TargetOriginId:       aws.String("web"),
			ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll,
			CachePolicyId:        aws.String(managedCachingOptimizedID),
		},
	}

	if defaultPolicyID != "" {
		cfg.DefaultCacheBehavior.ResponseHeadersPolicyId = aws.String(defaultPolicyID)
	}

	if orderedPolicyID != "" {
		cfg.CacheBehaviors = &cloudfronttypes.CacheBehaviors{Quantity: aws.Int32(1), Items: []cloudfronttypes.CacheBehavior{{
			PathPattern:             aws.String(apiPathPattern),
			TargetOriginId:          aws.String("web"),
			ViewerProtocolPolicy:    cloudfronttypes.ViewerProtocolPolicyAllowAll,
			CachePolicyId:           aws.String(managedCachingOptimizedID),
			ResponseHeadersPolicyId: aws.String(orderedPolicyID),
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

func TestCloudFront_ResponseHeadersPolicyLifecycle(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := uniqueName("test-cf-rhp-lifecycle")

	created := createResponseHeadersPolicy(t, client, responseHeadersPolicyConfig(name))
	golden.New(t, responseHeadersPolicyGoldenIgnores).Assert(t.Name()+"_create", created)

	id := created.ResponseHeadersPolicy.Id

	if aws.ToString(created.Location) != "/2020-05-31/response-headers-policy/"+aws.ToString(id) || aws.ToString(created.ETag) == "" || created.ResponseHeadersPolicy.LastModifiedTime == nil {
		t.Errorf("Location = %q, ETag = %q, LastModifiedTime = %v", aws.ToString(created.Location), aws.ToString(created.ETag), created.ResponseHeadersPolicy.LastModifiedTime)
	}

	got, err := client.GetResponseHeadersPolicy(ctx, &cloudfront.GetResponseHeadersPolicyInput{Id: id})
	if err != nil {
		t.Fatalf("GetResponseHeadersPolicy: %v", err)
	}

	golden.New(t, responseHeadersPolicyGoldenIgnores).Assert(t.Name()+"_get", got)

	config, err := client.GetResponseHeadersPolicyConfig(ctx, &cloudfront.GetResponseHeadersPolicyConfigInput{Id: id})
	if err != nil {
		t.Fatalf("GetResponseHeadersPolicyConfig: %v", err)
	}

	golden.New(t, responseHeadersPolicyGoldenIgnores).Assert(t.Name()+"_config", config)

	if aws.ToString(config.ETag) != aws.ToString(got.ETag) || aws.ToString(config.ResponseHeadersPolicyConfig.Name) != name {
		t.Errorf("config ETag %q != get ETag %q or name %q", aws.ToString(config.ETag), aws.ToString(got.ETag), aws.ToString(config.ResponseHeadersPolicyConfig.Name))
	}

	// Change the custom headers, drop CORS, the removed headers and most
	// security headers, and turn Server-Timing off without a sampling rate.
	next := responseHeadersPolicyConfig(name)
	next.Comment = aws.String("updated")
	next.CorsConfig = nil
	next.RemoveHeadersConfig = nil
	next.CustomHeadersConfig = &cloudfronttypes.ResponseHeadersPolicyCustomHeadersConfig{Quantity: aws.Int32(1), Items: []cloudfronttypes.ResponseHeadersPolicyCustomHeader{
		{Header: aws.String("X-Updated"), Value: aws.String("yes"), Override: aws.Bool(true)},
	}}
	next.SecurityHeadersConfig = &cloudfronttypes.ResponseHeadersPolicySecurityHeadersConfig{
		XSSProtection: &cloudfronttypes.ResponseHeadersPolicyXSSProtection{Override: aws.Bool(true), Protection: aws.Bool(false)},
	}
	next.ServerTimingHeadersConfig = &cloudfronttypes.ResponseHeadersPolicyServerTimingHeadersConfig{Enabled: aws.Bool(false)}

	updated, err := client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: id, IfMatch: got.ETag, ResponseHeadersPolicyConfig: next})
	if err != nil {
		t.Fatalf("UpdateResponseHeadersPolicy: %v", err)
	}

	golden.New(t, responseHeadersPolicyGoldenIgnores).Assert(t.Name()+"_update", updated)

	if aws.ToString(updated.ETag) == aws.ToString(got.ETag) {
		t.Error("update must issue a new ETag")
	}

	_, err = client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: id, IfMatch: got.ETag, ResponseHeadersPolicyConfig: responseHeadersPolicyConfig(name)})
	assertCloudFrontAPIError(t, err, "PreconditionFailed", http.StatusPreconditionFailed)

	afterUpdate, err := client.GetResponseHeadersPolicy(ctx, &cloudfront.GetResponseHeadersPolicyInput{Id: id})
	if err != nil {
		t.Fatalf("GetResponseHeadersPolicy: %v", err)
	}

	if aws.ToString(afterUpdate.ETag) != aws.ToString(updated.ETag) || afterUpdate.ResponseHeadersPolicy.ResponseHeadersPolicyConfig.CorsConfig != nil {
		t.Errorf("Get after update: ETag %q (want %q), CorsConfig %+v", aws.ToString(afterUpdate.ETag), aws.ToString(updated.ETag), afterUpdate.ResponseHeadersPolicy.ResponseHeadersPolicyConfig.CorsConfig)
	}

	listed, err := client.ListResponseHeadersPolicies(ctx, &cloudfront.ListResponseHeadersPoliciesInput{Type: cloudfronttypes.ResponseHeadersPolicyTypeCustom})
	if err != nil {
		t.Fatalf("ListResponseHeadersPolicies: %v", err)
	}

	found := false

	for _, item := range listed.ResponseHeadersPolicyList.Items {
		if aws.ToString(item.ResponseHeadersPolicy.Id) == aws.ToString(id) {
			found = item.Type == cloudfronttypes.ResponseHeadersPolicyTypeCustom && aws.ToString(item.ResponseHeadersPolicy.ResponseHeadersPolicyConfig.Comment) == "updated"
		}
	}

	if !found {
		t.Errorf("updated policy missing from the list: %+v", listed.ResponseHeadersPolicyList)
	}

	if _, err := client.DeleteResponseHeadersPolicy(ctx, &cloudfront.DeleteResponseHeadersPolicyInput{Id: id, IfMatch: updated.ETag}); err != nil {
		t.Fatalf("DeleteResponseHeadersPolicy: %v", err)
	}

	_, err = client.GetResponseHeadersPolicy(ctx, &cloudfront.GetResponseHeadersPolicyInput{Id: id})
	assertCloudFrontAPIError(t, err, "NoSuchResponseHeadersPolicy", http.StatusNotFound)

	// The name is free again once the policy is gone.
	createResponseHeadersPolicy(t, client, responseHeadersPolicyConfig(name))
}

func TestCloudFront_ResponseHeadersPolicyErrors(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := uniqueName("test-cf-rhp-errors")
	otherName := uniqueName("test-cf-rhp-other")

	created := createResponseHeadersPolicy(t, client, responseHeadersPolicyConfig(name))
	other := createResponseHeadersPolicy(t, client, responseHeadersPolicyConfig(otherName))
	id := created.ResponseHeadersPolicy.Id

	_, err := client.CreateResponseHeadersPolicy(ctx, &cloudfront.CreateResponseHeadersPolicyInput{ResponseHeadersPolicyConfig: responseHeadersPolicyConfig(name)})
	assertCloudFrontAPIError(t, err, "ResponseHeadersPolicyAlreadyExists", http.StatusConflict)

	_, err = client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: other.ResponseHeadersPolicy.Id, IfMatch: other.ETag, ResponseHeadersPolicyConfig: responseHeadersPolicyConfig(name)})
	assertCloudFrontAPIError(t, err, "ResponseHeadersPolicyAlreadyExists", http.StatusConflict)
	assertResponseHeadersPolicyUnchanged(t, client, other.ResponseHeadersPolicy.Id, other.ETag, otherName)

	inconsistent := responseHeadersPolicyConfig(otherName)
	inconsistent.CustomHeadersConfig.Quantity = aws.Int32(3)

	_, err = client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: other.ResponseHeadersPolicy.Id, IfMatch: other.ETag, ResponseHeadersPolicyConfig: inconsistent})
	assertCloudFrontAPIError(t, err, "InconsistentQuantities", http.StatusBadRequest)

	invalid := responseHeadersPolicyConfig(otherName)
	invalid.ServerTimingHeadersConfig.SamplingRate = aws.Float64(150)

	_, err = client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: other.ResponseHeadersPolicy.Id, IfMatch: other.ETag, ResponseHeadersPolicyConfig: invalid})
	assertCloudFrontAPIError(t, err, "InvalidArgument", http.StatusBadRequest)
	assertResponseHeadersPolicyUnchanged(t, client, other.ResponseHeadersPolicy.Id, other.ETag, otherName)

	_, err = client.CreateResponseHeadersPolicy(ctx, &cloudfront.CreateResponseHeadersPolicyInput{ResponseHeadersPolicyConfig: invalid})
	assertCloudFrontAPIError(t, err, "InvalidArgument", http.StatusBadRequest)

	// Keeping its own name is not a conflict.
	sameName, err := client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: id, IfMatch: created.ETag, ResponseHeadersPolicyConfig: responseHeadersPolicyConfig(name)})
	if err != nil {
		t.Fatalf("UpdateResponseHeadersPolicy keeping its name: %v", err)
	}

	if aws.ToString(sameName.ETag) == aws.ToString(created.ETag) {
		t.Error("update must issue a new ETag")
	}

	renamed := responseHeadersPolicyConfig(uniqueName("test-cf-rhp-renamed"))

	_, err = client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: id, ResponseHeadersPolicyConfig: renamed})
	assertCloudFrontAPIError(t, err, "InvalidIfMatchVersion", http.StatusBadRequest)

	_, err = client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: id, IfMatch: created.ETag, ResponseHeadersPolicyConfig: renamed})
	assertCloudFrontAPIError(t, err, "PreconditionFailed", http.StatusPreconditionFailed)

	_, err = client.DeleteResponseHeadersPolicy(ctx, &cloudfront.DeleteResponseHeadersPolicyInput{Id: id})
	assertCloudFrontAPIError(t, err, "InvalidIfMatchVersion", http.StatusBadRequest)

	_, err = client.DeleteResponseHeadersPolicy(ctx, &cloudfront.DeleteResponseHeadersPolicyInput{Id: id, IfMatch: created.ETag})
	assertCloudFrontAPIError(t, err, "PreconditionFailed", http.StatusPreconditionFailed)

	assertResponseHeadersPolicyUnchanged(t, client, id, sameName.ETag, name)

	missing := aws.String("00000000-0000-0000-0000-000000000000")

	_, err = client.GetResponseHeadersPolicy(ctx, &cloudfront.GetResponseHeadersPolicyInput{Id: missing})
	assertCloudFrontAPIError(t, err, "NoSuchResponseHeadersPolicy", http.StatusNotFound)

	_, err = client.GetResponseHeadersPolicyConfig(ctx, &cloudfront.GetResponseHeadersPolicyConfigInput{Id: missing})
	assertCloudFrontAPIError(t, err, "NoSuchResponseHeadersPolicy", http.StatusNotFound)

	_, err = client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: missing, IfMatch: aws.String("E1"), ResponseHeadersPolicyConfig: renamed})
	assertCloudFrontAPIError(t, err, "NoSuchResponseHeadersPolicy", http.StatusNotFound)

	_, err = client.DeleteResponseHeadersPolicy(ctx, &cloudfront.DeleteResponseHeadersPolicyInput{Id: missing, IfMatch: aws.String("E1")})
	assertCloudFrontAPIError(t, err, "NoSuchResponseHeadersPolicy", http.StatusNotFound)
}

func TestCloudFront_ResponseHeadersPolicyInUse(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	t.Run("default behavior", func(t *testing.T) {
		policy := createResponseHeadersPolicy(t, client, responseHeadersPolicyConfig(uniqueName("test-cf-rhp-default")))
		dist := responseHeadersPolicyDistribution(t, client, true, aws.ToString(policy.ResponseHeadersPolicy.Id), "")

		if got := aws.ToString(dist.Distribution.DistributionConfig.DefaultCacheBehavior.ResponseHeadersPolicyId); got != aws.ToString(policy.ResponseHeadersPolicy.Id) {
			t.Fatalf("ResponseHeadersPolicyId = %q", got)
		}

		_, err := client.DeleteResponseHeadersPolicy(ctx, &cloudfront.DeleteResponseHeadersPolicyInput{Id: policy.ResponseHeadersPolicy.Id, IfMatch: policy.ETag})
		assertCloudFrontAPIError(t, err, "ResponseHeadersPolicyInUse", http.StatusConflict)
		assertResponseHeadersPolicyUnchanged(t, client, policy.ResponseHeadersPolicy.Id, policy.ETag, aws.ToString(policy.ResponseHeadersPolicy.ResponseHeadersPolicyConfig.Name))

		// Detach the policy from the behavior; it can then be deleted.
		current, err := client.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: dist.Distribution.Id})
		if err != nil {
			t.Fatalf("GetDistributionConfig: %v", err)
		}

		current.DistributionConfig.DefaultCacheBehavior.ResponseHeadersPolicyId = nil

		if _, err := client.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{Id: dist.Distribution.Id, IfMatch: current.ETag, DistributionConfig: current.DistributionConfig}); err != nil {
			t.Fatalf("UpdateDistribution: %v", err)
		}

		if _, err := client.DeleteResponseHeadersPolicy(ctx, &cloudfront.DeleteResponseHeadersPolicyInput{Id: policy.ResponseHeadersPolicy.Id, IfMatch: policy.ETag}); err != nil {
			t.Fatalf("DeleteResponseHeadersPolicy after detaching: %v", err)
		}
	})

	t.Run("ordered behavior of a disabled distribution", func(t *testing.T) {
		policy := createResponseHeadersPolicy(t, client, responseHeadersPolicyConfig(uniqueName("test-cf-rhp-ordered")))
		dist := responseHeadersPolicyDistribution(t, client, false, "", aws.ToString(policy.ResponseHeadersPolicy.Id))

		_, err := client.DeleteResponseHeadersPolicy(ctx, &cloudfront.DeleteResponseHeadersPolicyInput{Id: policy.ResponseHeadersPolicy.Id, IfMatch: policy.ETag})
		assertCloudFrontAPIError(t, err, "ResponseHeadersPolicyInUse", http.StatusConflict)

		if _, err := client.DeleteDistribution(ctx, &cloudfront.DeleteDistributionInput{Id: dist.Distribution.Id, IfMatch: dist.ETag}); err != nil {
			t.Fatalf("DeleteDistribution: %v", err)
		}

		if _, err := client.DeleteResponseHeadersPolicy(ctx, &cloudfront.DeleteResponseHeadersPolicyInput{Id: policy.ResponseHeadersPolicy.Id, IfMatch: policy.ETag}); err != nil {
			t.Fatalf("DeleteResponseHeadersPolicy after the distribution is gone: %v", err)
		}
	})
}

func TestCloudFront_ListResponseHeadersPolicies(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	want := map[string]bool{}

	for range 3 {
		created := createResponseHeadersPolicy(t, client, responseHeadersPolicyConfig(uniqueName("test-cf-rhp-list")))
		want[aws.ToString(created.ResponseHeadersPolicy.Id)] = true
	}

	var marker *string

	seen := map[string]bool{}

	for {
		page, err := client.ListResponseHeadersPolicies(ctx, &cloudfront.ListResponseHeadersPoliciesInput{Type: cloudfronttypes.ResponseHeadersPolicyTypeCustom, MaxItems: aws.Int32(1), Marker: marker})
		if err != nil {
			t.Fatalf("ListResponseHeadersPolicies: %v", err)
		}

		list := page.ResponseHeadersPolicyList
		if aws.ToInt32(list.MaxItems) != 1 || aws.ToInt32(list.Quantity) != int32(len(list.Items)) || len(list.Items) > 1 {
			t.Fatalf("page shape: MaxItems %d, Quantity %d, %d items", aws.ToInt32(list.MaxItems), aws.ToInt32(list.Quantity), len(list.Items))
		}

		for _, item := range list.Items {
			if item.Type != cloudfronttypes.ResponseHeadersPolicyTypeCustom {
				t.Errorf("Type = %q, want custom", item.Type)
			}

			seen[aws.ToString(item.ResponseHeadersPolicy.Id)] = true
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

	managed, err := client.ListResponseHeadersPolicies(ctx, &cloudfront.ListResponseHeadersPoliciesInput{Type: cloudfronttypes.ResponseHeadersPolicyTypeManaged})
	if err != nil {
		t.Fatalf("ListResponseHeadersPolicies managed: %v", err)
	}

	if aws.ToInt32(managed.ResponseHeadersPolicyList.Quantity) != 0 || len(managed.ResponseHeadersPolicyList.Items) != 0 {
		t.Errorf("managed list = %+v, want empty (kumo seeds no managed policies)", managed.ResponseHeadersPolicyList)
	}

	_, err = client.ListResponseHeadersPolicies(ctx, &cloudfront.ListResponseHeadersPoliciesInput{Type: cloudfronttypes.ResponseHeadersPolicyType("builtin")})
	assertCloudFrontAPIError(t, err, "InvalidArgument", http.StatusBadRequest)
}

func TestCloudFront_ResponseHeadersPolicyDocumentedLimits(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := uniqueName("test-cf-rhp-limits")

	created := createResponseHeadersPolicy(t, client, responseHeadersPolicyConfig(name))
	id := created.ResponseHeadersPolicy.Id

	withRemoveHeaders := func(headers ...string) *cloudfronttypes.ResponseHeadersPolicyConfig {
		cfg := responseHeadersPolicyConfig(name)
		cfg.RemoveHeadersConfig = &cloudfronttypes.ResponseHeadersPolicyRemoveHeadersConfig{Quantity: aws.Int32(int32(len(headers)))}

		for _, h := range headers {
			cfg.RemoveHeadersConfig.Items = append(cfg.RemoveHeadersConfig.Items, cloudfronttypes.ResponseHeadersPolicyRemoveHeader{Header: aws.String(h)})
		}

		return cfg
	}

	withCSP := func(length int) *cloudfronttypes.ResponseHeadersPolicyConfig {
		cfg := responseHeadersPolicyConfig(name)
		cfg.SecurityHeadersConfig.ContentSecurityPolicy.ContentSecurityPolicy = aws.String(strings.Repeat("a", length))

		return cfg
	}

	for _, header := range []string{"Content-Length", "content-length", "HOST", "Transfer-Encoding", "X-Amzn-RequestId", "X-Amz-Cf-Pop", "x-edge-location", "X-Real-Ip"} {
		cfg := withRemoveHeaders(header)
		cfg.Name = aws.String(uniqueName("test-cf-rhp-remove"))

		_, err := client.CreateResponseHeadersPolicy(ctx, &cloudfront.CreateResponseHeadersPolicyInput{ResponseHeadersPolicyConfig: cfg})
		assertCloudFrontAPIError(t, err, "InvalidArgument", http.StatusBadRequest)

		_, err = client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: id, IfMatch: created.ETag, ResponseHeadersPolicyConfig: withRemoveHeaders("X-Powered-By", header)})
		assertCloudFrontAPIError(t, err, "InvalidArgument", http.StatusBadRequest)
		assertResponseHeadersPolicyUnchanged(t, client, id, created.ETag, name)
	}

	_, err := client.CreateResponseHeadersPolicy(ctx, &cloudfront.CreateResponseHeadersPolicyInput{ResponseHeadersPolicyConfig: withCSP(1784)})
	assertCloudFrontAPIError(t, err, "TooLongCSPInResponseHeadersPolicy", http.StatusBadRequest)

	_, err = client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: id, IfMatch: created.ETag, ResponseHeadersPolicyConfig: withCSP(1784)})
	assertCloudFrontAPIError(t, err, "TooLongCSPInResponseHeadersPolicy", http.StatusBadRequest)
	assertResponseHeadersPolicyUnchanged(t, client, id, created.ETag, name)

	maxCSP, err := client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: id, IfMatch: created.ETag, ResponseHeadersPolicyConfig: withCSP(1783)})
	if err != nil {
		t.Fatalf("UpdateResponseHeadersPolicy with a 1783-character CSP: %v", err)
	}

	nearMisses := []string{"X-Amz-Meta-Foo", "X-Amzn-Custom", "X-Powered-By", "Server", "Date", "Vary"}

	accepted, err := client.UpdateResponseHeadersPolicy(ctx, &cloudfront.UpdateResponseHeadersPolicyInput{Id: id, IfMatch: maxCSP.ETag, ResponseHeadersPolicyConfig: withRemoveHeaders(nearMisses...)})
	if err != nil {
		t.Fatalf("UpdateResponseHeadersPolicy removing %v: %v", nearMisses, err)
	}

	if got := accepted.ResponseHeadersPolicy.ResponseHeadersPolicyConfig.RemoveHeadersConfig; aws.ToInt32(got.Quantity) != int32(len(nearMisses)) {
		t.Errorf("RemoveHeadersConfig = %+v, want %v", got, nearMisses)
	}
}
