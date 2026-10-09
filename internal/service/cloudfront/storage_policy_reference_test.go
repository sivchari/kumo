package cloudfront

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

const (
	managedCachingOptimizedID   = "658327ea-f89d-4fab-a63d-7e88639e58f6"
	managedSecurityHeadersID    = "67f7725c-6f97-4210-82d7-5512b31e9d03"
	missingPolicyID             = "00000000-0000-0000-0000-000000000000"
	policyReferenceTestCallerID = "policy-reference"
)

// policyRefs names the policies one behavior references.
type policyRefs struct {
	cache, responseHeaders string
}

func (p policyRefs) behavior() DefaultCacheBehaviorXML {
	return DefaultCacheBehaviorXML{TargetOriginID: "o", CachePolicyID: p.cache, ResponseHeadersPolicyID: p.responseHeaders}
}

// distributionRequest references the policies from the default behavior and,
// when ordered is set, from one ordered behavior as well.
func distributionRequest(callerRef string, def policyRefs, ordered *policyRefs) *CreateDistributionRequest {
	req := &CreateDistributionRequest{
		CallerReference:      callerRef,
		Origins:              &OriginsXML{Quantity: 1, Items: &OriginList{Origin: []OriginXML{{ID: "o", DomainName: oacTestPlainDomain}}}},
		DefaultCacheBehavior: new(def.behavior()),
	}

	if ordered != nil {
		req.CacheBehaviors = &CacheBehaviorsXML{Quantity: 1, Items: []CacheBehaviorXML{{PathPattern: pathAPI, DefaultCacheBehaviorXML: ordered.behavior()}}}
	}

	return req
}

func TestCreateDistribution_PolicyReferences(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	cachePolicy, err := store.CreateCachePolicy(ctx, validCachePolicy("referenced"))
	if err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	headersPolicy, err := store.CreateResponseHeadersPolicy(ctx, validResponseHeadersPolicy("referenced"))
	if err != nil {
		t.Fatalf("CreateResponseHeadersPolicy: %v", err)
	}

	cases := map[string]struct {
		def     policyRefs
		ordered *policyRefs
		want    string
	}{
		"no policy":                         {},
		"managed cache policy":              {def: policyRefs{cache: managedCachingOptimizedID}},
		"managed response headers policy":   {def: policyRefs{responseHeaders: managedSecurityHeadersID}},
		"stored policies":                   {def: policyRefs{cache: cachePolicy.ID, responseHeaders: headersPolicy.ID}},
		"stored and managed on ordered":     {ordered: &policyRefs{cache: cachePolicy.ID, responseHeaders: managedSecurityHeadersID}},
		"unknown cache policy":              {def: policyRefs{cache: missingPolicyID}, want: errNoSuchCachePolicy},
		"unknown response headers policy":   {def: policyRefs{responseHeaders: missingPolicyID}, want: errNoSuchResponseHeadersPolicy},
		"unknown cache policy on ordered":   {ordered: &policyRefs{cache: missingPolicyID}, want: errNoSuchCachePolicy},
		"unknown headers policy on ordered": {ordered: &policyRefs{responseHeaders: missingPolicyID}, want: errNoSuchResponseHeadersPolicy},
		"response headers id as cache id":   {def: policyRefs{cache: headersPolicy.ID}, want: errNoSuchCachePolicy},
		"cache id as response headers id":   {def: policyRefs{responseHeaders: cachePolicy.ID}, want: errNoSuchResponseHeadersPolicy},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := store.CreateDistribution(ctx, distributionRequest(name, tc.def, tc.ordered))
			assertCreateOutcome(t, store, name, tc.want, err)
		})
	}
}

// assertCreateOutcome checks the create error against want ("" for success)
// and that the distribution exists exactly when the create succeeded.
func assertCreateOutcome(t *testing.T, store *MemoryStorage, callerRef, want string, err error) {
	t.Helper()

	dists, _, listErr := store.ListDistributions(t.Context(), "", 1000)
	if listErr != nil {
		t.Fatalf("ListDistributions: %v", listErr)
	}

	created := slices.ContainsFunc(dists, func(d *Distribution) bool { return d.DistributionConfig.CallerReference == callerRef })

	if want == "" {
		if err != nil || !created {
			t.Fatalf("CreateDistribution: created=%v, %v", created, err)
		}

		return
	}

	if got := errorCode(t, err); got != want {
		t.Errorf("error code = %s, want %s", got, want)
	}

	if created {
		t.Error("rejected request left a distribution behind")
	}
}

func TestUpdateDistribution_PolicyReferences(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		def     policyRefs
		ordered *policyRefs
		want    string
	}{
		"managed policies":                  {def: policyRefs{cache: managedCachingOptimizedID, responseHeaders: managedSecurityHeadersID}},
		"unknown cache policy":              {def: policyRefs{cache: missingPolicyID}, want: errNoSuchCachePolicy},
		"unknown response headers policy":   {def: policyRefs{responseHeaders: missingPolicyID}, want: errNoSuchResponseHeadersPolicy},
		"unknown cache policy on ordered":   {ordered: &policyRefs{cache: missingPolicyID}, want: errNoSuchCachePolicy},
		"unknown headers policy on ordered": {ordered: &policyRefs{responseHeaders: missingPolicyID}, want: errNoSuchResponseHeadersPolicy},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := NewMemoryStorage()
			ctx := context.Background()

			created, err := store.CreateDistribution(ctx, distributionRequest(policyReferenceTestCallerID, policyRefs{}, nil))
			if err != nil {
				t.Fatalf("CreateDistribution: %v", err)
			}

			before := *created

			updated, err := store.UpdateDistribution(ctx, created.ID, distributionRequest(policyReferenceTestCallerID, tc.def, tc.ordered), created.ETag)
			if tc.want == "" {
				if err != nil || updated.ETag == before.ETag {
					t.Fatalf("UpdateDistribution: %+v, %v", updated, err)
				}

				return
			}

			if got := errorCode(t, err); got != tc.want {
				t.Errorf("error code = %s, want %s", got, tc.want)
			}

			stored, err := store.GetDistribution(ctx, created.ID)
			if err != nil {
				t.Fatalf("GetDistribution: %v", err)
			}

			if stored.ETag != before.ETag || stored.DistributionConfig != before.DistributionConfig || stored.DistributionConfig.CacheBehaviors != nil {
				t.Errorf("rejected update modified the distribution: %+v", stored)
			}
		})
	}
}

func TestUpdateDistribution_StoredPolicyReference(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	policy, err := store.CreateCachePolicy(ctx, validCachePolicy("referenced"))
	if err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	created, err := store.CreateDistribution(ctx, distributionRequest(policyReferenceTestCallerID, policyRefs{}, nil))
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	updated, err := store.UpdateDistribution(ctx, created.ID, distributionRequest(policyReferenceTestCallerID, policyRefs{cache: policy.ID}, nil), created.ETag)
	if err != nil {
		t.Fatalf("UpdateDistribution: %v", err)
	}

	if got := updated.DistributionConfig.DefaultCacheBehavior.CachePolicyID; got != policy.ID {
		t.Errorf("CachePolicyID = %q, want %q", got, policy.ID)
	}

	if err := store.DeleteCachePolicy(ctx, policy.ID, policy.ETag); errorCode(t, err) != errCachePolicyInUse {
		t.Errorf("delete while referenced: %v", err)
	}
}

func TestManagedPolicyIDs_Disjoint(t *testing.T) {
	t.Parallel()

	for id := range managedCachePolicies {
		if _, dup := managedResponseHeadersPolicies[id]; dup {
			t.Errorf("%s is listed as both a cache policy and a response headers policy", id)
		}
	}
}

func TestPolicyReferences_HTTPStatus(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		refs     policyRefs
		wantCode string
	}{
		"cache policy":            {refs: policyRefs{cache: missingPolicyID}, wantCode: errNoSuchCachePolicy},
		"response headers policy": {refs: policyRefs{responseHeaders: missingPolicyID}, wantCode: errNoSuchResponseHeadersPolicy},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := NewMemoryStorage()
			svc := New(store)
			req := distributionRequest(policyReferenceTestCallerID, tc.refs, nil)

			plain, err := xml.Marshal(req)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			withTags, err := xml.Marshal(&DistributionConfigWithTags{DistributionConfig: *req})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			w := httptest.NewRecorder()
			svc.CreateDistribution(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/2020-05-31/distribution", strings.NewReader(string(plain))))
			assertPolicyNotFound(t, "CreateDistribution", w, tc.wantCode)

			w = httptest.NewRecorder()
			svc.CreateDistribution(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/2020-05-31/distribution?WithTags", strings.NewReader(string(withTags))))
			assertPolicyNotFound(t, "CreateDistributionWithTags", w, tc.wantCode)

			created, err := svc.storage.CreateDistribution(t.Context(), distributionRequest(policyReferenceTestCallerID, policyRefs{}, nil))
			if err != nil {
				t.Fatalf("CreateDistribution: %v", err)
			}

			update := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/2020-05-31/distribution/"+created.ID+"/config", strings.NewReader(string(plain)))
			update.SetPathValue("id", created.ID)
			update.Header.Set("If-Match", created.ETag)

			w = httptest.NewRecorder()
			svc.UpdateDistribution(w, update)
			assertPolicyNotFound(t, "UpdateDistribution", w, tc.wantCode)

			dists, _, err := store.ListDistributions(t.Context(), "", 1000)
			if err != nil || len(dists) != 1 || dists[0].ID != created.ID {
				t.Errorf("distributions = %v, %v; want only the one created directly", dists, err)
			}
		})
	}
}

func assertPolicyNotFound(t *testing.T, op string, w *httptest.ResponseRecorder, code string) {
	t.Helper()

	if w.Code != http.StatusNotFound {
		t.Errorf("%s: status %d, want 404 (body=%s)", op, w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "<Code>"+code+"</Code>") {
		t.Errorf("%s: body %s does not carry %s", op, w.Body.String(), code)
	}
}
