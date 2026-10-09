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

const responseHeadersPolicyTestHeader = "X-Kumo"

func validResponseHeadersPolicy(name string) *ResponseHeadersPolicyConfig {
	return &ResponseHeadersPolicyConfig{
		Name: name,
		CorsConfig: &ResponseHeadersPolicyCorsConfig{
			AccessControlAllowCredentials: new(false),
			AccessControlAllowHeaders:     &ResponseHeadersPolicyHeaderList{Quantity: new(1), Items: &ResponseHeadersPolicyHeaders{Header: []string{"*"}}},
			AccessControlAllowMethods:     &ResponseHeadersPolicyMethodList{Quantity: new(2), Items: &ResponseHeadersPolicyMethods{Method: []string{methodGet, methodHead}}},
			AccessControlAllowOrigins:     &ResponseHeadersPolicyOriginList{Quantity: new(1), Items: &ResponseHeadersPolicyOrigins{Origin: []string{"https://example.com"}}},
			AccessControlMaxAgeSec:        new(int32(0)),
			OriginOverride:                new(true),
		},
		CustomHeadersConfig: &ResponseHeadersPolicyCustomHeadersConfig{Quantity: new(1), Items: &ResponseHeadersPolicyCustomHeaderList{ResponseHeadersPolicyCustomHeader: []ResponseHeadersPolicyCustomHeader{
			{Header: new(responseHeadersPolicyTestHeader), Override: new(false), Value: new("1")},
		}}},
		SecurityHeadersConfig: &ResponseHeadersPolicySecurityHeadersConfig{
			FrameOptions:  &ResponseHeadersPolicyFrameOptions{FrameOption: "DENY", Override: new(true)},
			XSSProtection: &ResponseHeadersPolicyXSSProtection{Override: new(false), Protection: new(true), ModeBlock: new(false), ReportURI: new("https://example.com/report")},
		},
		ServerTimingHeadersConfig: &ResponseHeadersPolicyServerTimingHeadersConfig{Enabled: new(false), SamplingRate: new(12.3456)},
	}
}

func TestValidateResponseHeadersPolicyConfig(t *testing.T) {
	t.Parallel()

	mutate := func(f func(*ResponseHeadersPolicyConfig)) *ResponseHeadersPolicyConfig {
		cfg := validResponseHeadersPolicy("n")
		f(cfg)

		return cfg
	}

	cases := map[string]struct {
		cfg  *ResponseHeadersPolicyConfig
		want string
	}{
		"no name": {mutate(func(c *ResponseHeadersPolicyConfig) { c.Name = "" }), errInvalidArgument},
		"long comment": {mutate(func(c *ResponseHeadersPolicyConfig) {
			c.Comment = strings.Repeat("c", responseHeadersPolicyCommentMaxLength+1)
		}), errInvalidArgument},
		"missing credentials": {mutate(func(c *ResponseHeadersPolicyConfig) { c.CorsConfig.AccessControlAllowCredentials = nil }), errInvalidArgument},
		"missing override":    {mutate(func(c *ResponseHeadersPolicyConfig) { c.CorsConfig.OriginOverride = nil }), errInvalidArgument},
		"missing origins":     {mutate(func(c *ResponseHeadersPolicyConfig) { c.CorsConfig.AccessControlAllowOrigins = nil }), errInvalidArgument},
		"missing items":       {mutate(func(c *ResponseHeadersPolicyConfig) { c.CorsConfig.AccessControlAllowHeaders.Items = nil }), errInvalidArgument},
		"bad method": {mutate(func(c *ResponseHeadersPolicyConfig) {
			c.CorsConfig.AccessControlAllowMethods.Items.Method[0] = "CONNECT"
		}), errInvalidArgument},
		"method quantity": {mutate(func(c *ResponseHeadersPolicyConfig) { c.CorsConfig.AccessControlAllowMethods.Quantity = new(3) }), errInconsistentQuantities},
		"expose quantity": {mutate(func(c *ResponseHeadersPolicyConfig) {
			c.CorsConfig.AccessControlExposeHeaders = &ResponseHeadersPolicyHeaderList{Quantity: new(1)}
		}), errInconsistentQuantities},
		"custom no value": {mutate(func(c *ResponseHeadersPolicyConfig) {
			c.CustomHeadersConfig.Items.ResponseHeadersPolicyCustomHeader[0].Value = nil
		}), errInvalidArgument},
		"custom quantity":    {mutate(func(c *ResponseHeadersPolicyConfig) { c.CustomHeadersConfig.Quantity = new(0) }), errInconsistentQuantities},
		"custom no quantity": {mutate(func(c *ResponseHeadersPolicyConfig) { c.CustomHeadersConfig.Quantity = nil }), errInvalidArgument},
		"remove no header": {mutate(func(c *ResponseHeadersPolicyConfig) {
			c.RemoveHeadersConfig = &ResponseHeadersPolicyRemoveHeadersConfig{Quantity: new(1), Items: &ResponseHeadersPolicyRemoveHeaderList{ResponseHeadersPolicyRemoveHeader: []ResponseHeadersPolicyRemoveHeader{{}}}}
		}), errInvalidArgument},
		"bad frame option": {mutate(func(c *ResponseHeadersPolicyConfig) { c.SecurityHeadersConfig.FrameOptions.FrameOption = "ALLOW-FROM" }), errInvalidArgument},
		"bad referrer policy": {mutate(func(c *ResponseHeadersPolicyConfig) {
			c.SecurityHeadersConfig.ReferrerPolicy = &ResponseHeadersPolicyReferrerPolicy{Override: new(true), ReferrerPolicy: "never"}
		}), errInvalidArgument},
		"hsts no max age": {mutate(func(c *ResponseHeadersPolicyConfig) {
			c.SecurityHeadersConfig.StrictTransportSecurity = &ResponseHeadersPolicyStrictTransportSecurity{Override: new(true)}
		}), errInvalidArgument},
		"xss no protection":   {mutate(func(c *ResponseHeadersPolicyConfig) { c.SecurityHeadersConfig.XSSProtection.Protection = nil }), errInvalidArgument},
		"xss block reporting": {mutate(func(c *ResponseHeadersPolicyConfig) { c.SecurityHeadersConfig.XSSProtection.ModeBlock = new(true) }), errInvalidArgument},
		"timing no enabled":   {mutate(func(c *ResponseHeadersPolicyConfig) { c.ServerTimingHeadersConfig.Enabled = nil }), errInvalidArgument},
		"timing above 100":    {mutate(func(c *ResponseHeadersPolicyConfig) { c.ServerTimingHeadersConfig.SamplingRate = new(100.5) }), errInvalidArgument},
		"timing negative":     {mutate(func(c *ResponseHeadersPolicyConfig) { c.ServerTimingHeadersConfig.SamplingRate = new(-1.0) }), errInvalidArgument},
		"timing precision":    {mutate(func(c *ResponseHeadersPolicyConfig) { c.ServerTimingHeadersConfig.SamplingRate = new(0.12345) }), errInvalidArgument},
	}

	for name, tc := range cases {
		if err := validateResponseHeadersPolicyConfig(tc.cfg); err == nil || errorCode(t, err) != tc.want {
			t.Errorf("%s: err = %v, want %s", name, err, tc.want)
		}
	}
}

func TestValidateResponseHeadersPolicyConfig_Accepts(t *testing.T) {
	t.Parallel()

	maxRate := validResponseHeadersPolicy("n")
	maxRate.ServerTimingHeadersConfig.SamplingRate = new(100.0)

	for name, cfg := range map[string]*ResponseHeadersPolicyConfig{
		"name only":      {Name: "n"},
		"full":           validResponseHeadersPolicy("n"),
		"empty lists":    {Name: "n", CustomHeadersConfig: &ResponseHeadersPolicyCustomHeadersConfig{Quantity: new(0)}, RemoveHeadersConfig: &ResponseHeadersPolicyRemoveHeadersConfig{Quantity: new(0)}},
		"sampling rates": maxRate,
	} {
		if err := validateResponseHeadersPolicyConfig(cfg); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestResponseHeadersPolicy_StoredStateIsIsolated(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	cfg := validResponseHeadersPolicy("isolated")

	created, err := store.CreateResponseHeadersPolicy(ctx, cfg)
	if err != nil {
		t.Fatalf("CreateResponseHeadersPolicy: %v", err)
	}

	*cfg.CustomHeadersConfig.Items.ResponseHeadersPolicyCustomHeader[0].Header = "changed-request"
	cfg.CorsConfig.AccessControlAllowOrigins.Items.Origin[0] = "changed-request"
	*created.Config.SecurityHeadersConfig.FrameOptions.Override = false
	*created.Config.ServerTimingHeadersConfig.SamplingRate = 1

	got, err := store.GetResponseHeadersPolicy(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetResponseHeadersPolicy: %v", err)
	}

	if *got.Config.CustomHeadersConfig.Items.ResponseHeadersPolicyCustomHeader[0].Header != responseHeadersPolicyTestHeader ||
		got.Config.CorsConfig.AccessControlAllowOrigins.Items.Origin[0] != "https://example.com" ||
		!*got.Config.SecurityHeadersConfig.FrameOptions.Override ||
		*got.Config.ServerTimingHeadersConfig.SamplingRate != 12.3456 {
		t.Errorf("stored policy was mutated through a caller's pointer: %+v", got.Config)
	}
}

func TestResponseHeadersPolicy_WriteProtocol(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	first, err := store.CreateResponseHeadersPolicy(ctx, validResponseHeadersPolicy("first"))
	if err != nil {
		t.Fatalf("CreateResponseHeadersPolicy: %v", err)
	}

	second, err := store.CreateResponseHeadersPolicy(ctx, validResponseHeadersPolicy("second"))
	if err != nil {
		t.Fatalf("CreateResponseHeadersPolicy: %v", err)
	}

	if _, err := store.CreateResponseHeadersPolicy(ctx, validResponseHeadersPolicy("first")); errorCode(t, err) != errResponseHeadersPolicyAlreadyExists {
		t.Errorf("duplicate name: %v", err)
	}

	if _, err := store.UpdateResponseHeadersPolicy(ctx, second.ID, validResponseHeadersPolicy("first"), second.ETag); errorCode(t, err) != errResponseHeadersPolicyAlreadyExists {
		t.Errorf("rename to a taken name: %v", err)
	}

	if _, err := store.UpdateResponseHeadersPolicy(ctx, first.ID, validResponseHeadersPolicy("renamed"), ""); errorCode(t, err) != errInvalidIfMatchVersion {
		t.Errorf("missing If-Match: %v", err)
	}

	if _, err := store.UpdateResponseHeadersPolicy(ctx, first.ID, validResponseHeadersPolicy("renamed"), "stale"); errorCode(t, err) != errPreconditionFailed {
		t.Errorf("stale If-Match: %v", err)
	}

	if _, err := store.UpdateResponseHeadersPolicy(ctx, "missing", validResponseHeadersPolicy("renamed"), "E1"); errorCode(t, err) != errNoSuchResponseHeadersPolicy {
		t.Errorf("missing id: %v", err)
	}

	invalid := validResponseHeadersPolicy("second")
	invalid.CustomHeadersConfig.Quantity = new(5)

	if _, err := store.UpdateResponseHeadersPolicy(ctx, second.ID, invalid, second.ETag); errorCode(t, err) != errInconsistentQuantities {
		t.Errorf("invalid update: %v", err)
	}

	unchanged, _ := store.GetResponseHeadersPolicy(ctx, second.ID)
	if unchanged.ETag != second.ETag || unchanged.Config.Name != "second" || *unchanged.Config.CustomHeadersConfig.Quantity != 1 {
		t.Errorf("rejected update changed state: %+v", unchanged)
	}
}

func TestResponseHeadersPolicy_DeleteProtocol(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	first, err := store.CreateResponseHeadersPolicy(ctx, validResponseHeadersPolicy("first"))
	if err != nil {
		t.Fatalf("CreateResponseHeadersPolicy: %v", err)
	}

	same, err := store.UpdateResponseHeadersPolicy(ctx, first.ID, validResponseHeadersPolicy("first"), first.ETag)
	if err != nil || same.ETag == first.ETag {
		t.Fatalf("update keeping its own name: %+v, %v", same, err)
	}

	if err := store.DeleteResponseHeadersPolicy(ctx, first.ID, ""); errorCode(t, err) != errInvalidIfMatchVersion {
		t.Errorf("delete without If-Match: %v", err)
	}

	if err := store.DeleteResponseHeadersPolicy(ctx, first.ID, first.ETag); errorCode(t, err) != errPreconditionFailed {
		t.Errorf("delete with the old ETag: %v", err)
	}

	if err := store.DeleteResponseHeadersPolicy(ctx, first.ID, same.ETag); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := store.CreateResponseHeadersPolicy(ctx, validResponseHeadersPolicy("first")); err != nil {
		t.Errorf("name reuse after delete: %v", err)
	}
}

func TestResponseHeadersPolicy_DeleteInUse(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	policy, err := store.CreateResponseHeadersPolicy(ctx, validResponseHeadersPolicy("in-use"))
	if err != nil {
		t.Fatalf("CreateResponseHeadersPolicy: %v", err)
	}

	origins := &OriginsXML{Quantity: 1, Items: &OriginList{Origin: []OriginXML{{ID: "o", DomainName: oacTestPlainDomain}}}}

	byDefault, err := store.CreateDistribution(ctx, &CreateDistributionRequest{
		CallerReference:      "default",
		Enabled:              true,
		Origins:              origins,
		DefaultCacheBehavior: &DefaultCacheBehaviorXML{TargetOriginID: "o", ResponseHeadersPolicyID: policy.ID},
	})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	ordered, err := store.CreateDistribution(ctx, &CreateDistributionRequest{
		CallerReference:      "ordered",
		Enabled:              false,
		Origins:              origins,
		DefaultCacheBehavior: &DefaultCacheBehaviorXML{TargetOriginID: "o"},
		CacheBehaviors:       &CacheBehaviorsXML{Quantity: 1, Items: []CacheBehaviorXML{{PathPattern: pathAPI, DefaultCacheBehaviorXML: DefaultCacheBehaviorXML{TargetOriginID: "o", ResponseHeadersPolicyID: policy.ID}}}},
	})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	for _, dist := range []*Distribution{byDefault, ordered} {
		if err := store.DeleteResponseHeadersPolicy(ctx, policy.ID, policy.ETag); errorCode(t, err) != errResponseHeadersPolicyInUse {
			t.Errorf("delete while distribution %s references it: %v", dist.DistributionConfig.CallerReference, err)
		}

		if err := store.DeleteDistribution(ctx, dist.ID, dist.ETag); err != nil {
			t.Fatalf("DeleteDistribution: %v", err)
		}
	}

	if err := store.DeleteResponseHeadersPolicy(ctx, policy.ID, policy.ETag); err != nil {
		t.Errorf("delete after the last reference is gone: %v", err)
	}
}

func TestResponseHeadersPolicy_ListFilterAndPagination(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	for _, name := range []string{"a", "b", "c"} {
		if _, err := store.CreateResponseHeadersPolicy(ctx, validResponseHeadersPolicy(name)); err != nil {
			t.Fatalf("CreateResponseHeadersPolicy(%s): %v", name, err)
		}
	}

	first, next, err := store.ListResponseHeadersPolicies(ctx, responseHeadersPolicyTypeCustom, "", 2)
	if err != nil || len(first) != 2 || next != first[1].ID {
		t.Fatalf("first page = %d items, next %q, err %v", len(first), next, err)
	}

	second, next, err := store.ListResponseHeadersPolicies(ctx, "", next, 2)
	if err != nil || len(second) != 1 || next != "" || second[0].ID <= first[1].ID {
		t.Fatalf("second page = %d items, next %q, err %v", len(second), next, err)
	}

	managed, next, err := store.ListResponseHeadersPolicies(ctx, responseHeadersPolicyTypeManaged, "", 0)
	if err != nil || len(managed) != 0 || next != "" {
		t.Fatalf("managed = %d items, next %q, err %v", len(managed), next, err)
	}
}

func TestResponseHeadersPolicy_SnapshotCompatibility(t *testing.T) {
	t.Parallel()

	legacy := NewMemoryStorage()
	if err := json.Unmarshal([]byte(`{"distributions":{},"invalidations":{}}`), legacy); err != nil {
		t.Fatalf("unmarshal legacy snapshot: %v", err)
	}

	if _, err := legacy.CreateResponseHeadersPolicy(context.Background(), validResponseHeadersPolicy("after-load")); err != nil {
		t.Fatalf("create after loading a snapshot without response headers policies: %v", err)
	}

	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	restored := NewMemoryStorage()
	if err := json.Unmarshal(data, restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	list, _, _ := restored.ListResponseHeadersPolicies(context.Background(), "", "", 0)
	if len(list) != 1 || list[0].Config.Name != "after-load" || *list[0].Config.CustomHeadersConfig.Items.ResponseHeadersPolicyCustomHeader[0].Header != responseHeadersPolicyTestHeader ||
		*list[0].Config.CorsConfig.AccessControlMaxAgeSec != 0 || *list[0].Config.ServerTimingHeadersConfig.SamplingRate != 12.3456 {
		t.Fatalf("restored policies = %+v", list)
	}
}

func TestResponseHeadersPolicyXML_Shape(t *testing.T) {
	t.Parallel()

	body := `<ResponseHeadersPolicyConfig xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/"><Name>n</Name>` +
		`<CorsConfig><AccessControlAllowCredentials>false</AccessControlAllowCredentials>` +
		`<AccessControlAllowHeaders><Quantity>0</Quantity><Items></Items></AccessControlAllowHeaders>` +
		`<AccessControlAllowMethods><Quantity>1</Quantity><Items><Method>GET</Method></Items></AccessControlAllowMethods>` +
		`<AccessControlAllowOrigins><Quantity>1</Quantity><Items><Origin>*</Origin></Items></AccessControlAllowOrigins>` +
		`<OriginOverride>false</OriginOverride></CorsConfig>` +
		`<RemoveHeadersConfig><Quantity>1</Quantity><Items><ResponseHeadersPolicyRemoveHeader><Header>Server</Header></ResponseHeadersPolicyRemoveHeader></Items></RemoveHeadersConfig>` +
		`<SecurityHeadersConfig><StrictTransportSecurity><AccessControlMaxAgeSec>0</AccessControlMaxAgeSec><Override>false</Override></StrictTransportSecurity></SecurityHeadersConfig>` +
		`<ServerTimingHeadersConfig><Enabled>true</Enabled><SamplingRate>0.5</SamplingRate></ServerTimingHeadersConfig></ResponseHeadersPolicyConfig>`

	var cfg ResponseHeadersPolicyConfig
	if err := xml.Unmarshal([]byte(body), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	stored, err := normalizeResponseHeadersPolicyConfig(&cfg)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	out, err := xml.Marshal(buildResponseHeadersPolicyXML(&ResponseHeadersPolicy{ID: "id", Config: *stored}, cloudfrontXmlns))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got := string(out)

	for _, want := range []string{
		`<ResponseHeadersPolicy xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/"><Id>id</Id>`,
		`<ResponseHeadersPolicyConfig><CorsConfig><AccessControlAllowCredentials>false</AccessControlAllowCredentials>`,
		`<AccessControlAllowHeaders><Quantity>0</Quantity><Items></Items></AccessControlAllowHeaders>`,
		`<Items><Method>GET</Method></Items>`,
		`<OriginOverride>false</OriginOverride></CorsConfig><Name>n</Name>`,
		`<Items><ResponseHeadersPolicyRemoveHeader><Header>Server</Header></ResponseHeadersPolicyRemoveHeader></Items>`,
		`<StrictTransportSecurity><AccessControlMaxAgeSec>0</AccessControlMaxAgeSec><Override>false</Override></StrictTransportSecurity>`,
		`<ServerTimingHeadersConfig><Enabled>true</Enabled><SamplingRate>0.5</SamplingRate></ServerTimingHeadersConfig>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}

	for _, unwanted := range []string{"Comment", "AccessControlExposeHeaders", "AccessControlMaxAgeSec>0</AccessControlMaxAgeSec><OriginOverride", "CustomHeadersConfig", "IncludeSubdomains", "Preload", "XSSProtection"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("omitted element %s was emitted: %s", unwanted, got)
		}
	}
}

func TestResponseHeadersPolicy_CommentLengthCountsCharacters(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	withComment := func(name string, runes int) *ResponseHeadersPolicyConfig {
		cfg := validResponseHeadersPolicy(name)
		cfg.Comment = strings.Repeat("あ", runes)

		return cfg
	}

	created, err := store.CreateResponseHeadersPolicy(ctx, withComment("max", responseHeadersPolicyCommentMaxLength))
	if err != nil {
		t.Fatalf("create with %d multibyte characters: %v", responseHeadersPolicyCommentMaxLength, err)
	}

	if _, err := store.CreateResponseHeadersPolicy(ctx, withComment("over", responseHeadersPolicyCommentMaxLength+1)); err == nil || errorCode(t, err) != errInvalidArgument {
		t.Errorf("create with %d characters: err = %v, want %s", responseHeadersPolicyCommentMaxLength+1, err, errInvalidArgument)
	}

	if _, err := store.UpdateResponseHeadersPolicy(ctx, created.ID, withComment("max", responseHeadersPolicyCommentMaxLength+1), created.ETag); err == nil || errorCode(t, err) != errInvalidArgument {
		t.Errorf("update with %d characters: err = %v, want %s", responseHeadersPolicyCommentMaxLength+1, err, errInvalidArgument)
	}

	if _, err := store.UpdateResponseHeadersPolicy(ctx, created.ID, withComment("max", responseHeadersPolicyCommentMaxLength), created.ETag); err != nil {
		t.Errorf("update with %d multibyte characters: %v", responseHeadersPolicyCommentMaxLength, err)
	}
}

func TestResponseHeadersPolicyXML_OmittedItems(t *testing.T) {
	t.Parallel()

	cfg := validResponseHeadersPolicy("n")
	cfg.CorsConfig.AccessControlExposeHeaders = &ResponseHeadersPolicyHeaderList{Quantity: new(0)}
	cfg.CustomHeadersConfig = &ResponseHeadersPolicyCustomHeadersConfig{Quantity: new(0)}
	cfg.RemoveHeadersConfig = &ResponseHeadersPolicyRemoveHeadersConfig{Quantity: new(0)}

	out, err := xml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, want := range []string{
		`<AccessControlExposeHeaders><Quantity>0</Quantity></AccessControlExposeHeaders>`,
		`<CustomHeadersConfig><Quantity>0</Quantity></CustomHeadersConfig>`,
		`<RemoveHeadersConfig><Quantity>0</Quantity></RemoveHeadersConfig>`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("omitted Items was emitted, missing %s in %s", want, out)
		}
	}
}

func TestResponseHeadersPolicyXML_EmptyItemsRoundTrip(t *testing.T) {
	t.Parallel()

	body := `<ResponseHeadersPolicyConfig><Name>n</Name>` +
		`<CorsConfig><AccessControlAllowCredentials>false</AccessControlAllowCredentials>` +
		`<AccessControlAllowHeaders><Quantity>0</Quantity><Items/></AccessControlAllowHeaders>` +
		`<AccessControlAllowMethods><Quantity>1</Quantity><Items><Method>GET</Method></Items></AccessControlAllowMethods>` +
		`<AccessControlAllowOrigins><Quantity>1</Quantity><Items><Origin>*</Origin></Items></AccessControlAllowOrigins>` +
		`<AccessControlExposeHeaders><Quantity>0</Quantity><Items/></AccessControlExposeHeaders>` +
		`<OriginOverride>false</OriginOverride></CorsConfig>` +
		`<CustomHeadersConfig><Quantity>0</Quantity><Items/></CustomHeadersConfig>` +
		`<RemoveHeadersConfig><Quantity>0</Quantity></RemoveHeadersConfig></ResponseHeadersPolicyConfig>`

	var req ResponseHeadersPolicyConfig
	if err := xml.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	store := NewMemoryStorage()

	created, err := store.CreateResponseHeadersPolicy(context.Background(), &req)
	if err != nil {
		t.Fatalf("CreateResponseHeadersPolicy: %v", err)
	}

	got, err := store.GetResponseHeadersPolicy(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetResponseHeadersPolicy: %v", err)
	}

	out, err := xml.Marshal(buildResponseHeadersPolicyXML(got, cloudfrontXmlns))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, want := range []string{
		`<AccessControlAllowHeaders><Quantity>0</Quantity><Items></Items></AccessControlAllowHeaders>`,
		`<AccessControlExposeHeaders><Quantity>0</Quantity><Items></Items></AccessControlExposeHeaders>`,
		`<CustomHeadersConfig><Quantity>0</Quantity><Items></Items></CustomHeadersConfig>`,
		`<RemoveHeadersConfig><Quantity>0</Quantity></RemoveHeadersConfig>`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
}

func TestListResponseHeadersPolicies_EmptyOmitsItems(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())

	w := httptest.NewRecorder()
	svc.ListResponseHeadersPolicies(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/2020-05-31/response-headers-policy", http.NoBody))

	if w.Code != http.StatusOK {
		t.Fatalf("ListResponseHeadersPolicies: status %d body=%s", w.Code, w.Body.String())
	}

	if got := w.Body.String(); strings.Contains(got, "<Items") || !strings.Contains(got, "<Quantity>0</Quantity>") {
		t.Errorf("empty list body = %s", got)
	}

	if _, err := svc.storage.CreateResponseHeadersPolicy(t.Context(), validResponseHeadersPolicy("listed")); err != nil {
		t.Fatalf("CreateResponseHeadersPolicy: %v", err)
	}

	w = httptest.NewRecorder()
	svc.ListResponseHeadersPolicies(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/2020-05-31/response-headers-policy", http.NoBody))

	if got := w.Body.String(); !strings.Contains(got, "<Quantity>1</Quantity><Items><ResponseHeadersPolicySummary><Type>custom</Type>") {
		t.Errorf("list body = %s", got)
	}
}

func withRemoveHeader(name, header string) *ResponseHeadersPolicyConfig {
	cfg := validResponseHeadersPolicy(name)
	cfg.RemoveHeadersConfig = &ResponseHeadersPolicyRemoveHeadersConfig{Quantity: new(1), Items: &ResponseHeadersPolicyRemoveHeaderList{
		ResponseHeadersPolicyRemoveHeader: []ResponseHeadersPolicyRemoveHeader{{Header: new(header)}},
	}}

	return cfg
}

func TestResponseHeadersPolicy_UnremovableHeaders(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	rejected := []string{
		"Connection", "Content-Encoding", "Content-Length", "Expect", "Host", "Keep-Alive",
		"Proxy-Authenticate", "Proxy-Authorization", "Proxy-Connection", "Trailer", "Transfer-Encoding",
		"Upgrade", "Via", "Warning", "X-Accel-Buffering", "X-Accel-Charset", "X-Accel-Limit-Rate",
		"X-Accel-Redirect", "X-Amzn-Auth", "X-Amzn-Cf-Billing", "X-Amzn-Cf-Id", "X-Amzn-Cf-Xff",
		"X-Amzn-ErrorType", "X-Amzn-Fle-Profile", "X-Amzn-Header-Count", "X-Amzn-Header-Order",
		"X-Amzn-Lambda-Integration-Tag", "X-Amzn-RequestId", "X-Forwarded-Proto", "X-Real-Ip",
		"X-Amz-Cf-Pop", "X-Amz-Cf-Id", "x-amz-cf-", "X-Edge-Location", "x-edge-location", "X-EDGE-REQUEST-ID",
	}

	for _, header := range rejected {
		for _, variant := range []string{header, strings.ToLower(header), strings.ToUpper(header)} {
			if _, err := store.CreateResponseHeadersPolicy(ctx, withRemoveHeader("rejected", variant)); err == nil || errorCode(t, err) != errInvalidArgument {
				t.Errorf("remove %s: err = %v, want %s", variant, err, errInvalidArgument)
			}
		}
	}

	for _, header := range []string{"X-Amz-Meta-Foo", "X-Amzn-Custom", "X-Amzn-Cf-Id-Extra", "X-Amz-Cf", "X-Edge", "X-Powered-By", "Server", "Date", "vary"} {
		if _, err := store.CreateResponseHeadersPolicy(ctx, withRemoveHeader("accepted-"+header, header)); err != nil {
			t.Errorf("remove %s: %v", header, err)
		}
	}

	created, err := store.CreateResponseHeadersPolicy(ctx, withRemoveHeader("update", "Server"))
	if err != nil {
		t.Fatalf("CreateResponseHeadersPolicy: %v", err)
	}

	if _, err := store.UpdateResponseHeadersPolicy(ctx, created.ID, withRemoveHeader("update", "content-length"), created.ETag); err == nil || errorCode(t, err) != errInvalidArgument {
		t.Errorf("update removing content-length: err = %v, want %s", err, errInvalidArgument)
	}

	unchanged, _ := store.GetResponseHeadersPolicy(ctx, created.ID)
	if unchanged.ETag != created.ETag || *unchanged.Config.RemoveHeadersConfig.Items.ResponseHeadersPolicyRemoveHeader[0].Header != "Server" {
		t.Errorf("rejected update changed state: %+v", unchanged)
	}
}

func TestResponseHeadersPolicy_CSPLength(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	withCSP := func(name, csp string) *ResponseHeadersPolicyConfig {
		cfg := validResponseHeadersPolicy(name)
		cfg.SecurityHeadersConfig.ContentSecurityPolicy = &ResponseHeadersPolicyContentSecurityPolicy{ContentSecurityPolicy: new(csp), Override: new(true)}

		return cfg
	}

	created, err := store.CreateResponseHeadersPolicy(ctx, withCSP("max", strings.Repeat("a", responseHeadersPolicyCSPMaxLength)))
	if err != nil {
		t.Fatalf("create with %d characters: %v", responseHeadersPolicyCSPMaxLength, err)
	}

	if _, err := store.CreateResponseHeadersPolicy(ctx, withCSP("multibyte", strings.Repeat("あ", responseHeadersPolicyCSPMaxLength))); err != nil {
		t.Errorf("create with %d multibyte characters: %v", responseHeadersPolicyCSPMaxLength, err)
	}

	if _, err := store.CreateResponseHeadersPolicy(ctx, withCSP("over", strings.Repeat("a", responseHeadersPolicyCSPMaxLength+1))); err == nil || errorCode(t, err) != errTooLongCSPInResponseHeadersPolicy {
		t.Errorf("create with %d characters: err = %v, want %s", responseHeadersPolicyCSPMaxLength+1, err, errTooLongCSPInResponseHeadersPolicy)
	}

	if _, err := store.UpdateResponseHeadersPolicy(ctx, created.ID, withCSP("max", strings.Repeat("a", responseHeadersPolicyCSPMaxLength+1)), created.ETag); err == nil || errorCode(t, err) != errTooLongCSPInResponseHeadersPolicy {
		t.Errorf("update with %d characters: err = %v, want %s", responseHeadersPolicyCSPMaxLength+1, err, errTooLongCSPInResponseHeadersPolicy)
	}

	unchanged, _ := store.GetResponseHeadersPolicy(ctx, created.ID)
	if unchanged.ETag != created.ETag || len(*unchanged.Config.SecurityHeadersConfig.ContentSecurityPolicy.ContentSecurityPolicy) != responseHeadersPolicyCSPMaxLength {
		t.Errorf("rejected update changed state: ETag %s, want %s", unchanged.ETag, created.ETag)
	}
}

func TestCreateResponseHeadersPolicy_TooLongCSPStatus(t *testing.T) {
	t.Parallel()

	svc := New(NewMemoryStorage())
	body := `<ResponseHeadersPolicyConfig><Name>n</Name><SecurityHeadersConfig><ContentSecurityPolicy>` +
		`<ContentSecurityPolicy>` + strings.Repeat("a", responseHeadersPolicyCSPMaxLength+1) + `</ContentSecurityPolicy>` +
		`<Override>true</Override></ContentSecurityPolicy></SecurityHeadersConfig></ResponseHeadersPolicyConfig>`

	w := httptest.NewRecorder()
	svc.CreateResponseHeadersPolicy(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/2020-05-31/response-headers-policy", strings.NewReader(body)))

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "<Code>"+errTooLongCSPInResponseHeadersPolicy+"</Code>") {
		t.Errorf("status %d body=%s", w.Code, w.Body.String())
	}
}
