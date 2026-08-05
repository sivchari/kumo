package cloudfront

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func minimalConfig(callerRef string) CreateDistributionRequest {
	return CreateDistributionRequest{
		CallerReference: callerRef,
		Comment:         "test",
		Enabled:         true,
		Origins: &OriginsXML{
			Quantity: 1,
			Items: &OriginList{Origin: []OriginXML{{
				ID:             "o1",
				DomainName:     "example.s3.amazonaws.com",
				S3OriginConfig: &S3OriginConfigXML{OriginAccessIdentity: ""},
			}}},
		},
		DefaultCacheBehavior: &DefaultCacheBehaviorXML{
			TargetOriginID:       "o1",
			ViewerProtocolPolicy: "allow-all",
		},
	}
}

func marshalXML(v any) []byte {
	raw, _ := xml.Marshal(v)

	return raw
}

func tagItemsToMap(items []Tag) map[string]string {
	m := make(map[string]string, len(items))
	for _, it := range items {
		m[it.Key] = it.Value
	}

	return m
}

func createDistViaHandler(t *testing.T, svc *Service, body []byte, withTags bool) *GetDistributionResult {
	t.Helper()

	target := "/2020-05-31/distribution"
	if withTags {
		target += "?WithTags"
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, bytes.NewReader(body))
	w := httptest.NewRecorder()
	svc.CreateDistribution(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("CreateDistribution status: got %d, want 201 (body=%s)", w.Code, w.Body.String())
	}

	var out GetDistributionResult
	if err := xml.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal create response: %v body=%s", err, w.Body.String())
	}

	return &out
}

func listTagsViaHandler(t *testing.T, svc *Service, arn string) map[string]string {
	t.Helper()

	target := "/2020-05-31/tagging?Resource=" + url.QueryEscape(arn)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	w := httptest.NewRecorder()
	svc.ListTagsForResource(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ListTagsForResource status: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	var out Tags
	if err := xml.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal tags: %v body=%s", err, w.Body.String())
	}

	return tagItemsToMap(out.Items)
}

func taggingViaHandler(t *testing.T, svc *Service, operation, arn string, body []byte) {
	t.Helper()

	target := "/2020-05-31/tagging?Operation=" + operation + "&Resource=" + url.QueryEscape(arn)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, bytes.NewReader(body))
	w := httptest.NewRecorder()
	svc.Tagging(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("%s status: got %d, want 204 (body=%s)", operation, w.Code, w.Body.String())
	}
}

func TestCreateDistribution_WithTags(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	wrap := DistributionConfigWithTags{
		DistributionConfig: minimalConfig("with-tags-ref"),
		Tags: &Tags{Items: []Tag{
			{Key: "env", Value: "local"},
			{Key: "team", Value: "idp"},
		}},
	}

	created := createDistViaHandler(t, svc, marshalXML(wrap), true)
	if created.ARN == "" {
		t.Fatal("created ARN is empty")
	}

	got := listTagsViaHandler(t, svc, created.ARN)
	if got["env"] != "local" || got["team"] != "idp" {
		t.Fatalf("tags: got %v, want env=local team=idp", got)
	}
}

func TestCreateDistribution_WithoutTags_NoRegression(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	created := createDistViaHandler(t, svc, marshalXML(minimalConfig("plain-ref")), false)
	if created.Status != statusInProgress {
		t.Fatalf("create status: got %q, want %q", created.Status, statusInProgress)
	}

	if got := listTagsViaHandler(t, svc, created.ARN); len(got) != 0 {
		t.Fatalf("plain create should have no tags, got %v", got)
	}
}

func TestTagging_TagListUntag(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())
	created := createDistViaHandler(t, svc, marshalXML(minimalConfig("tagging-ref")), false)

	taggingViaHandler(t, svc, "Tag", created.ARN,
		marshalXML(Tags{Items: []Tag{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}}}))

	if got := listTagsViaHandler(t, svc, created.ARN); got["a"] != "1" || got["b"] != "2" {
		t.Fatalf("after tag: got %v, want a=1 b=2", got)
	}

	taggingViaHandler(t, svc, "Untag", created.ARN, marshalXML(TagKeysBody{Items: []string{"a"}}))

	got := listTagsViaHandler(t, svc, created.ARN)
	if _, ok := got["a"]; ok {
		t.Fatalf("key a should be removed, got %v", got)
	}

	if got["b"] != "2" {
		t.Fatalf("key b should remain, got %v", got)
	}
}

func TestGetDistribution_TransitionsToDeployed(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())
	created := createDistViaHandler(t, svc, marshalXML(minimalConfig("deploy-ref")), false)

	if created.Status != statusInProgress {
		t.Fatalf("create status: got %q, want %q", created.Status, statusInProgress)
	}

	target := "/2020-05-31/distribution/" + created.ID
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	req.SetPathValue("id", created.ID)

	w := httptest.NewRecorder()
	svc.GetDistribution(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GetDistribution status: %d body=%s", w.Code, w.Body.String())
	}

	var got GetDistributionResult
	if err := xml.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}

	if got.Status != statusDeployed {
		t.Fatalf("get status: got %q, want %q", got.Status, statusDeployed)
	}
}
