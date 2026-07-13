package cloudfront

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"
)

func getDistViaHandler(t *testing.T, svc *Service, id string) *GetDistributionResult {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/2020-05-31/distribution/"+id, http.NoBody)
	req.SetPathValue("id", id)

	w := httptest.NewRecorder()
	svc.GetDistribution(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GetDistribution status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	var out GetDistributionResult
	if err := xml.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal get response: %v body=%s", err, w.Body.String())
	}

	return &out
}

func assertTrustedDefaults(t *testing.T, dcb *DefaultCacheBehaviorXML) {
	t.Helper()

	if dcb == nil {
		t.Fatal("DefaultCacheBehavior is nil")
	}

	if dcb.TrustedSigners == nil {
		t.Fatal("TrustedSigners is nil, want present with Enabled=false/Quantity=0")
	}

	if dcb.TrustedSigners.Enabled || dcb.TrustedSigners.Quantity != 0 {
		t.Errorf("TrustedSigners: got Enabled=%v Quantity=%d, want false/0",
			dcb.TrustedSigners.Enabled, dcb.TrustedSigners.Quantity)
	}

	if dcb.TrustedKeyGroups == nil {
		t.Fatal("TrustedKeyGroups is nil, want present with Enabled=false/Quantity=0")
	}

	if dcb.TrustedKeyGroups.Enabled || dcb.TrustedKeyGroups.Quantity != 0 {
		t.Errorf("TrustedKeyGroups: got Enabled=%v Quantity=%d, want false/0",
			dcb.TrustedKeyGroups.Enabled, dcb.TrustedKeyGroups.Quantity)
	}
}

func assertCachedMethods(t *testing.T, dcb *DefaultCacheBehaviorXML) {
	t.Helper()

	if dcb == nil || dcb.AllowedMethods == nil {
		t.Fatal("AllowedMethods is nil")
	}

	cm := dcb.AllowedMethods.CachedMethods
	if cm == nil {
		t.Fatal("CachedMethods is nil, want preserved in response")
	}

	if cm.Quantity != 2 {
		t.Errorf("CachedMethods.Quantity: got %d, want 2", cm.Quantity)
	}

	if len(cm.Items) != 2 || cm.Items[0] != "GET" || cm.Items[1] != "HEAD" {
		t.Errorf("CachedMethods.Items: got %v, want [GET HEAD]", cm.Items)
	}
}

// TestDefaultCacheBehavior_TrustedDefaultsWhenSigningAbsent verifies that a
// CachePolicy-style distribution without TrustedSigners/TrustedKeyGroups still
// returns both elements with Enabled=false/Quantity=0, matching real CloudFront
// and avoiding the Terraform provider nil-pointer crash.
func TestDefaultCacheBehavior_TrustedDefaultsWhenSigningAbsent(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	created := createDistViaHandler(t, svc, marshalXML(minimalConfig("cache-policy-ref")), false)
	assertTrustedDefaults(t, created.DistributionConfig.DefaultCacheBehavior)

	got := getDistViaHandler(t, svc, created.ID)
	assertTrustedDefaults(t, got.DistributionConfig.DefaultCacheBehavior)
}

// TestDefaultCacheBehavior_CachedMethodsRoundTrip verifies that AllowedMethods'
// nested CachedMethods present in the request survives into both the create and
// get responses.
func TestDefaultCacheBehavior_CachedMethodsRoundTrip(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	config := minimalConfig("cached-methods-ref")
	config.DefaultCacheBehavior.AllowedMethods = &AllowedMethodsXML{
		Quantity: 2,
		Items:    []string{"GET", "HEAD"},
		CachedMethods: &CachedMethodsXML{
			Quantity: 2,
			Items:    []string{"GET", "HEAD"},
		},
	}

	created := createDistViaHandler(t, svc, marshalXML(config), false)
	assertCachedMethods(t, created.DistributionConfig.DefaultCacheBehavior)

	got := getDistViaHandler(t, svc, created.ID)
	assertCachedMethods(t, got.DistributionConfig.DefaultCacheBehavior)
}
