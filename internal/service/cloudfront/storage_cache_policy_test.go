package cloudfront

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const cachePolicyTestHeader = "Origin"

func ptr[T any](v T) *T { return &v }

func validCachePolicy(name string) *CachePolicyConfig {
	return &CachePolicyConfig{
		Name:       name,
		DefaultTTL: ptr(int64(60)),
		MaxTTL:     ptr(int64(120)),
		MinTTL:     ptr(int64(0)),
		Parameters: &CachePolicyParameters{
			EnableAcceptEncodingGzip: ptr(false),
			HeadersConfig:            &CachePolicyHeadersConfig{HeaderBehavior: cachePolicyBehaviorWhitelist, Headers: &CachePolicyNames{Quantity: ptr(1), Items: &CachePolicyNameItems{Name: []string{cachePolicyTestHeader}}}},
			CookiesConfig:            &CachePolicyCookiesConfig{CookieBehavior: cachePolicyBehaviorNone},
			QueryStringsConfig:       &CachePolicyQueryStrings{QueryStringBehavior: cachePolicyBehaviorAll},
		},
	}
}

func TestValidateCachePolicyConfig(t *testing.T) {
	t.Parallel()

	mutate := func(f func(*CachePolicyConfig)) *CachePolicyConfig {
		cfg := validCachePolicy("n")
		f(cfg)

		return cfg
	}

	cases := map[string]struct {
		cfg  *CachePolicyConfig
		want string
	}{
		"empty name":        {mutate(func(c *CachePolicyConfig) { c.Name = "" }), errInvalidArgument},
		"missing MinTTL":    {mutate(func(c *CachePolicyConfig) { c.MinTTL = nil }), errInvalidArgument},
		"long comment":      {mutate(func(c *CachePolicyConfig) { c.Comment = strings.Repeat("c", cachePolicyCommentMaxLength+1) }), errInvalidArgument},
		"missing gzip":      {mutate(func(c *CachePolicyConfig) { c.Parameters.EnableAcceptEncodingGzip = nil }), errInvalidArgument},
		"missing cookies":   {mutate(func(c *CachePolicyConfig) { c.Parameters.CookiesConfig = nil }), errInvalidArgument},
		"bad header mode":   {mutate(func(c *CachePolicyConfig) { c.Parameters.HeadersConfig.HeaderBehavior = cachePolicyBehaviorAll }), errInvalidArgument},
		"bad query mode":    {mutate(func(c *CachePolicyConfig) { c.Parameters.QueryStringsConfig.QueryStringBehavior = "some" }), errInvalidArgument},
		"missing quantity":  {mutate(func(c *CachePolicyConfig) { c.Parameters.HeadersConfig.Headers.Quantity = nil }), errInvalidArgument},
		"quantity mismatch": {mutate(func(c *CachePolicyConfig) { c.Parameters.HeadersConfig.Headers.Quantity = ptr(2) }), errInconsistentQuantities},
		"cookie items extra": {mutate(func(c *CachePolicyConfig) {
			c.Parameters.CookiesConfig.Cookies = &CachePolicyNames{Quantity: ptr(0), Items: &CachePolicyNameItems{Name: []string{"a"}}}
		}), errInconsistentQuantities},
	}

	for name, tc := range cases {
		if err := validateCachePolicyConfig(tc.cfg); err == nil || errorCode(t, err) != tc.want {
			t.Errorf("%s: err = %v, want %s", name, err, tc.want)
		}
	}

	if err := validateCachePolicyConfig(&CachePolicyConfig{Name: "n", MinTTL: ptr(int64(0))}); err != nil {
		t.Errorf("parameters are optional: %v", err)
	}
}

func TestNormalizeCachePolicyConfig_Defaults(t *testing.T) {
	t.Parallel()

	cases := []struct {
		min, wantDefault, wantMax int64
	}{
		{0, cachePolicyDefaultTTL, cachePolicyDefaultMaxTTL},
		{100000, 100000, cachePolicyDefaultMaxTTL},
		{40000000, 40000000, 40000000},
	}

	for _, tc := range cases {
		got, err := normalizeCachePolicyConfig(&CachePolicyConfig{Name: "n", MinTTL: ptr(tc.min)})
		if err != nil {
			t.Fatalf("normalize: %v", err)
		}

		if *got.DefaultTTL != tc.wantDefault || *got.MaxTTL != tc.wantMax {
			t.Errorf("MinTTL %d: DefaultTTL %d MaxTTL %d, want %d %d", tc.min, *got.DefaultTTL, *got.MaxTTL, tc.wantDefault, tc.wantMax)
		}
	}

	explicit, err := normalizeCachePolicyConfig(&CachePolicyConfig{Name: "n", MinTTL: ptr(int64(0)), DefaultTTL: ptr(int64(0)), MaxTTL: ptr(int64(0))})
	if err != nil || *explicit.DefaultTTL != 0 || *explicit.MaxTTL != 0 {
		t.Errorf("explicit zero TTLs must be kept: %+v, %v", explicit, err)
	}
}

func TestCachePolicy_StoredStateIsIsolated(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	cfg := validCachePolicy("isolated")

	created, err := store.CreateCachePolicy(ctx, cfg)
	if err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	cfg.Parameters.HeadersConfig.Headers.Items.Name[0] = "mutated-request"
	*cfg.MinTTL = 99
	created.Config.Parameters.HeadersConfig.Headers.Items.Name[0] = "mutated-result"
	*created.Config.Parameters.EnableAcceptEncodingGzip = true

	got, err := store.GetCachePolicy(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetCachePolicy: %v", err)
	}

	if got.Config.Parameters.HeadersConfig.Headers.Items.Name[0] != cachePolicyTestHeader || *got.Config.MinTTL != 0 || *got.Config.Parameters.EnableAcceptEncodingGzip {
		t.Errorf("stored policy was mutated through a caller's pointer: %+v", got.Config.Parameters)
	}
}

func TestCachePolicy_WriteProtocol(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	first, err := store.CreateCachePolicy(ctx, validCachePolicy("first"))
	if err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	second, err := store.CreateCachePolicy(ctx, validCachePolicy("second"))
	if err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	if _, err := store.CreateCachePolicy(ctx, validCachePolicy("first")); errorCode(t, err) != errCachePolicyAlreadyExists {
		t.Errorf("duplicate name: %v", err)
	}

	if _, err := store.UpdateCachePolicy(ctx, second.ID, validCachePolicy("first"), second.ETag); errorCode(t, err) != errCachePolicyAlreadyExists {
		t.Errorf("rename to a taken name: %v", err)
	}

	if _, err := store.UpdateCachePolicy(ctx, first.ID, validCachePolicy("renamed"), ""); errorCode(t, err) != errInvalidIfMatchVersion {
		t.Errorf("missing If-Match: %v", err)
	}

	if _, err := store.UpdateCachePolicy(ctx, first.ID, validCachePolicy("renamed"), "stale"); errorCode(t, err) != errPreconditionFailed {
		t.Errorf("stale If-Match: %v", err)
	}

	if _, err := store.UpdateCachePolicy(ctx, "missing", validCachePolicy("renamed"), "E1"); errorCode(t, err) != errNoSuchCachePolicy {
		t.Errorf("missing id: %v", err)
	}

	unchanged, _ := store.GetCachePolicy(ctx, second.ID)
	if unchanged.ETag != second.ETag || unchanged.Config.Name != "second" {
		t.Errorf("rejected update changed state: %+v", unchanged)
	}

	same, err := store.UpdateCachePolicy(ctx, first.ID, validCachePolicy("first"), first.ETag)
	if err != nil || same.ETag == first.ETag {
		t.Fatalf("update keeping its own name: %+v, %v", same, err)
	}

	if err := store.DeleteCachePolicy(ctx, first.ID, first.ETag); errorCode(t, err) != errPreconditionFailed {
		t.Errorf("delete with the old ETag: %v", err)
	}

	if err := store.DeleteCachePolicy(ctx, first.ID, same.ETag); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := store.CreateCachePolicy(ctx, validCachePolicy("first")); err != nil {
		t.Errorf("name reuse after delete: %v", err)
	}
}

func TestCachePolicy_DeleteInUse(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	policy, err := store.CreateCachePolicy(ctx, validCachePolicy("in-use"))
	if err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	origins := &OriginsXML{Quantity: 1, Items: &OriginList{Origin: []OriginXML{{ID: "o", DomainName: oacTestPlainDomain}}}}

	ordered, err := store.CreateDistribution(ctx, &CreateDistributionRequest{
		CallerReference:      "ordered",
		Enabled:              false,
		Origins:              origins,
		DefaultCacheBehavior: &DefaultCacheBehaviorXML{TargetOriginID: "o", CachePolicyID: "658327ea-f89d-4fab-a63d-7e88639e58f6"},
		CacheBehaviors:       &CacheBehaviorsXML{Quantity: 1, Items: []CacheBehaviorXML{{PathPattern: pathAPI, DefaultCacheBehaviorXML: DefaultCacheBehaviorXML{TargetOriginID: "o", CachePolicyID: policy.ID}}}},
	})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	if err := store.DeleteCachePolicy(ctx, policy.ID, policy.ETag); errorCode(t, err) != errCachePolicyInUse {
		t.Errorf("delete while an ordered behavior of a disabled distribution references it: %v", err)
	}

	if err := store.DeleteDistribution(ctx, ordered.ID, ordered.ETag); err != nil {
		t.Fatalf("DeleteDistribution: %v", err)
	}

	if err := store.DeleteCachePolicy(ctx, policy.ID, policy.ETag); err != nil {
		t.Errorf("delete after the last reference is gone: %v", err)
	}
}

func TestCachePolicy_ListFilterAndPagination(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	for _, name := range []string{"a", "b", "c"} {
		if _, err := store.CreateCachePolicy(ctx, validCachePolicy(name)); err != nil {
			t.Fatalf("CreateCachePolicy(%s): %v", name, err)
		}
	}

	first, next, err := store.ListCachePolicies(ctx, cachePolicyTypeCustom, "", 2)
	if err != nil || len(first) != 2 || next != first[1].ID {
		t.Fatalf("first page = %d items, next %q, err %v", len(first), next, err)
	}

	second, next, err := store.ListCachePolicies(ctx, "", next, 2)
	if err != nil || len(second) != 1 || next != "" || second[0].ID <= first[1].ID {
		t.Fatalf("second page = %d items, next %q, err %v", len(second), next, err)
	}

	managed, next, err := store.ListCachePolicies(ctx, cachePolicyTypeManaged, "", 0)
	if err != nil || len(managed) != 0 || next != "" {
		t.Fatalf("managed = %d items, next %q, err %v", len(managed), next, err)
	}
}

func TestCachePolicy_SnapshotCompatibility(t *testing.T) {
	t.Parallel()

	legacy := NewMemoryStorage()
	if err := json.Unmarshal([]byte(`{"distributions":{},"invalidations":{}}`), legacy); err != nil {
		t.Fatalf("unmarshal legacy snapshot: %v", err)
	}

	if _, err := legacy.CreateCachePolicy(context.Background(), validCachePolicy("after-load")); err != nil {
		t.Fatalf("create after loading a snapshot without cache policies: %v", err)
	}

	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	restored := NewMemoryStorage()
	if err := json.Unmarshal(data, restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	list, _, _ := restored.ListCachePolicies(context.Background(), "", "", 0)
	if len(list) != 1 || list[0].Config.Name != "after-load" || list[0].Config.Parameters.HeadersConfig.Headers.Items.Name[0] != cachePolicyTestHeader {
		t.Fatalf("restored policies = %+v", list)
	}
}

func TestCachePolicyXML_Shape(t *testing.T) {
	t.Parallel()

	body := `<CachePolicyConfig xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/"><Name>n</Name><MinTTL>0</MinTTL>` +
		`<ParametersInCacheKeyAndForwardedToOrigin><EnableAcceptEncodingGzip>false</EnableAcceptEncodingGzip>` +
		`<HeadersConfig><HeaderBehavior>none</HeaderBehavior></HeadersConfig>` +
		`<CookiesConfig><CookieBehavior>whitelist</CookieBehavior><Cookies><Quantity>1</Quantity><Items><Name>session</Name></Items></Cookies></CookiesConfig>` +
		`<QueryStringsConfig><QueryStringBehavior>none</QueryStringBehavior></QueryStringsConfig></ParametersInCacheKeyAndForwardedToOrigin></CachePolicyConfig>`

	var cfg CachePolicyConfig
	if err := xml.Unmarshal([]byte(body), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	stored, err := normalizeCachePolicyConfig(&cfg)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	out, err := xml.Marshal(buildCachePolicyXML(&CachePolicy{ID: "id", Config: *stored}, cloudfrontXmlns))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got := string(out)

	for _, want := range []string{
		`<CachePolicy xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/"><Id>id</Id>`,
		`<CachePolicyConfig><Name>n</Name><DefaultTTL>86400</DefaultTTL><MaxTTL>31536000</MaxTTL><MinTTL>0</MinTTL>`,
		`<EnableAcceptEncodingGzip>false</EnableAcceptEncodingGzip><HeadersConfig><HeaderBehavior>none</HeaderBehavior></HeadersConfig>`,
		`<Cookies><Quantity>1</Quantity><Items><Name>session</Name></Items></Cookies>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}

	for _, unwanted := range []string{"Comment", "EnableAcceptEncodingBrotli", "<Headers>"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("omitted element %s was emitted: %s", unwanted, got)
		}
	}
}

func TestCachePolicy_CommentLengthCountsCharacters(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	withComment := func(name string, runes int) *CachePolicyConfig {
		cfg := validCachePolicy(name)
		cfg.Comment = strings.Repeat("あ", runes)

		return cfg
	}

	created, err := store.CreateCachePolicy(ctx, withComment("max", cachePolicyCommentMaxLength))
	if err != nil {
		t.Fatalf("create with %d multibyte characters: %v", cachePolicyCommentMaxLength, err)
	}

	if _, err := store.CreateCachePolicy(ctx, withComment("over", cachePolicyCommentMaxLength+1)); err == nil || errorCode(t, err) != errInvalidArgument {
		t.Errorf("create with %d characters: err = %v, want %s", cachePolicyCommentMaxLength+1, err, errInvalidArgument)
	}

	if _, err := store.UpdateCachePolicy(ctx, created.ID, withComment("max", cachePolicyCommentMaxLength+1), created.ETag); err == nil || errorCode(t, err) != errInvalidArgument {
		t.Errorf("update with %d characters: err = %v, want %s", cachePolicyCommentMaxLength+1, err, errInvalidArgument)
	}

	if _, err := store.UpdateCachePolicy(ctx, created.ID, withComment("max", cachePolicyCommentMaxLength), created.ETag); err != nil {
		t.Errorf("update with %d multibyte characters: %v", cachePolicyCommentMaxLength, err)
	}
}

func TestCachePolicyXML_ItemsPresence(t *testing.T) {
	t.Parallel()

	cfg := validCachePolicy("n")
	cfg.Parameters.HeadersConfig.Headers = &CachePolicyNames{Quantity: ptr(0)}

	out, err := xml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(out), "<Items>") {
		t.Errorf("omitted Items was emitted: %s", out)
	}

	body := `<CachePolicyConfig><Name>n</Name><MinTTL>0</MinTTL>` +
		`<ParametersInCacheKeyAndForwardedToOrigin><EnableAcceptEncodingGzip>false</EnableAcceptEncodingGzip>` +
		`<HeadersConfig><HeaderBehavior>none</HeaderBehavior><Headers><Quantity>0</Quantity></Headers></HeadersConfig>` +
		`<CookiesConfig><CookieBehavior>none</CookieBehavior><Cookies><Quantity>0</Quantity><Items/></Cookies></CookiesConfig>` +
		`<QueryStringsConfig><QueryStringBehavior>none</QueryStringBehavior></QueryStringsConfig></ParametersInCacheKeyAndForwardedToOrigin></CachePolicyConfig>`

	var req CachePolicyConfig
	if err := xml.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	store := NewMemoryStorage()

	created, err := store.CreateCachePolicy(context.Background(), &req)
	if err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	got, err := store.GetCachePolicy(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetCachePolicy: %v", err)
	}

	out, err = xml.Marshal(buildCachePolicyXML(got, cloudfrontXmlns))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, want := range []string{
		`<Headers><Quantity>0</Quantity></Headers>`,
		`<Cookies><Quantity>0</Quantity><Items></Items></Cookies>`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
}

func TestListCachePolicies_EmptyOmitsItems(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	w := httptest.NewRecorder()
	svc.ListCachePolicies(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/2020-05-31/cache-policy", http.NoBody))

	if w.Code != http.StatusOK {
		t.Fatalf("ListCachePolicies: status %d body=%s", w.Code, w.Body.String())
	}

	if got := w.Body.String(); strings.Contains(got, "<Items") || !strings.Contains(got, "<Quantity>0</Quantity>") {
		t.Errorf("empty list body = %s", got)
	}

	if _, err := svc.storage.CreateCachePolicy(t.Context(), validCachePolicy("listed")); err != nil {
		t.Fatalf("CreateCachePolicy: %v", err)
	}

	w = httptest.NewRecorder()
	svc.ListCachePolicies(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/2020-05-31/cache-policy", http.NoBody))

	if got := w.Body.String(); !strings.Contains(got, "<Quantity>1</Quantity><Items><CachePolicySummary><Type>custom</Type>") {
		t.Errorf("list body = %s", got)
	}
}
