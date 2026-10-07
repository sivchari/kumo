package cloudfront

import (
	"encoding/json"
	"encoding/xml"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	methodPost = "POST"
	pathAPI    = "api/*"
)

func newBareDistribution() *Distribution {
	return &Distribution{
		ID:                     "EDFDVBD6EXAMPLE",
		LastModifiedTime:       time.Unix(1, 0),
		ActiveTrustedSigners:   &ActiveTrustedSigners{},
		ActiveTrustedKeyGroups: &ActiveTrustedKeyGroups{},
		DistributionConfig: &DistributionConfig{
			Origins:              &Origins{Quantity: 1, Items: []Origin{{ID: "o", DomainName: oacTestPlainDomain}}},
			DefaultCacheBehavior: &DefaultCacheBehavior{TargetOriginID: "o", ViewerProtocolPolicy: "allow-all"},
			CacheBehaviors: &CacheBehaviors{Quantity: 1, Items: []CacheBehavior{{
				PathPattern:          pathAPI,
				DefaultCacheBehavior: DefaultCacheBehavior{TargetOriginID: "o", ViewerProtocolPolicy: "allow-all"},
			}}},
		},
	}
}

func marshalRoundTrip(t *testing.T, dist *Distribution) *GetDistributionResult {
	t.Helper()

	data, err := xml.Marshal(buildDistributionXML(dist))
	if err != nil {
		t.Fatalf("marshal distribution: %v", err)
	}

	var got GetDistributionResult
	if err := xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal distribution: %v", err)
	}

	return &got
}

// assertMandatoryElements checks the elements the Terraform AWS provider
// dereferences without nil checks.
func assertMandatoryElements(t *testing.T, b *DefaultCacheBehaviorXML) {
	t.Helper()

	if b.AllowedMethods == nil || b.AllowedMethods.CachedMethods == nil {
		t.Fatalf("AllowedMethods and its CachedMethods must always be present, got %+v", b.AllowedMethods)
	}

	if b.FunctionAssociations == nil || b.LambdaFunctionAssociations == nil {
		t.Error("FunctionAssociations and LambdaFunctionAssociations must always be present")
	}

	if b.TrustedKeyGroups == nil || b.TrustedKeyGroups.Enabled || b.TrustedKeyGroups.Quantity != 0 {
		t.Errorf("TrustedKeyGroups must default to disabled and empty, got %+v", b.TrustedKeyGroups)
	}

	if b.TrustedSigners == nil || b.TrustedSigners.Enabled || b.TrustedSigners.Quantity != 0 {
		t.Errorf("TrustedSigners must default to disabled and empty, got %+v", b.TrustedSigners)
	}
}

func TestBuildDistributionXML_AlwaysEmitsMandatoryElements(t *testing.T) {
	t.Parallel()

	got := marshalRoundTrip(t, newBareDistribution())

	cfg := got.DistributionConfig
	if cfg.OriginGroups == nil || cfg.OriginGroups.Quantity != 0 {
		t.Errorf("OriginGroups must always be present and empty, got %+v", cfg.OriginGroups)
	}

	assertMandatoryElements(t, cfg.DefaultCacheBehavior)

	if len(cfg.CacheBehaviors.Items) != 1 {
		t.Fatalf("got %d ordered behaviors, want 1", len(cfg.CacheBehaviors.Items))
	}

	assertMandatoryElements(t, &cfg.CacheBehaviors.Items[0].DefaultCacheBehaviorXML)

	methods := cfg.DefaultCacheBehavior.AllowedMethods
	if want := []string{methodGet, methodHead}; !slices.Equal(methods.Items, want) || methods.Quantity != len(want) {
		t.Errorf("default AllowedMethods = %+v, want %v", methods, want)
	}

	if want := []string{methodGet, methodHead}; !slices.Equal(methods.CachedMethods.Items, want) || methods.CachedMethods.Quantity != len(want) {
		t.Errorf("default CachedMethods = %+v, want %v", methods.CachedMethods, want)
	}

	if got.ActiveTrustedSigners == nil || got.ActiveTrustedKeyGroups == nil || got.LastModifiedTime == "" {
		t.Error("ActiveTrustedSigners, ActiveTrustedKeyGroups and LastModifiedTime must always be present")
	}
}

func TestBuildDistributionXML_PreservesExplicitMethods(t *testing.T) {
	t.Parallel()

	dist := newBareDistribution()
	explicit := []string{methodGet, methodHead, methodPost}
	dist.DistributionConfig.DefaultCacheBehavior.AllowedMethods = &AllowedMethods{Quantity: len(explicit), Items: explicit}
	dist.DistributionConfig.DefaultCacheBehavior.CachedMethods = &CachedMethods{Quantity: 1, Items: []string{methodGet}}

	methods := marshalRoundTrip(t, dist).DistributionConfig.DefaultCacheBehavior.AllowedMethods
	if !slices.Equal(methods.Items, explicit) || methods.Quantity != len(explicit) {
		t.Errorf("AllowedMethods = %+v, want %v", methods, explicit)
	}

	if want := []string{methodGet}; !slices.Equal(methods.CachedMethods.Items, want) || methods.CachedMethods.Quantity != 1 {
		t.Errorf("CachedMethods = %+v, want %v", methods.CachedMethods, want)
	}
}

func TestBuildDistributionListXML_AlwaysEmitsMandatoryElements(t *testing.T) {
	t.Parallel()

	data, err := xml.Marshal(buildDistributionListXML([]*Distribution{newBareDistribution()}, "", 100, ""))
	if err != nil {
		t.Fatalf("marshal distribution list: %v", err)
	}

	var got DistributionListXML
	if err := xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal distribution list: %v", err)
	}

	summary := got.Items.DistributionSummary[0]
	assertMandatoryElements(t, summary.DefaultCacheBehavior)

	if len(summary.CacheBehaviors.Items) != 1 {
		t.Fatalf("got %d ordered behaviors, want 1", len(summary.CacheBehaviors.Items))
	}

	assertMandatoryElements(t, &summary.CacheBehaviors.Items[0].DefaultCacheBehaviorXML)
}

func TestBuildDistributionConfigXML_OmittedLoggingReadsBackDisabled(t *testing.T) {
	t.Parallel()

	data, err := xml.Marshal(buildDistributionConfigXML(newBareDistribution().DistributionConfig))
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	want := "<Logging><Enabled>false</Enabled><IncludeCookies>false</IncludeCookies><Bucket></Bucket><Prefix></Prefix></Logging>"
	if !strings.Contains(string(data), want) {
		t.Errorf("config XML lacks %s:\n%s", want, data)
	}
}

func TestNewDistributionConfig_DisabledLoggingDropsBucketAndPrefix(t *testing.T) {
	t.Parallel()

	got := newDistributionConfig(&CreateDistributionRequest{Logging: &LoggingConfigXML{
		IncludeCookies: true,
		Bucket:         "logs.s3.amazonaws.com",
		Prefix:         "cf/",
	}}).Logging

	if want := (LoggingConfig{IncludeCookies: true}); got == nil || *got != want {
		t.Errorf("Logging = %+v, want %+v", got, want)
	}
}

func TestMemoryStorage_LoadsSnapshotWithoutLogging(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(map[string]any{"distributions": map[string]*Distribution{"EDFDVBD6EXAMPLE": newBareDistribution()}})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	legacy := strings.Replace(string(data), `,"Logging":null`, "", 1)
	if strings.Contains(legacy, "Logging") {
		t.Fatalf("snapshot still carries Logging: %s", legacy)
	}

	s := NewMemoryStorage()
	if err := s.UnmarshalJSON([]byte(legacy)); err != nil {
		t.Fatalf("load snapshot: %v", err)
	}

	dist, err := s.GetDistribution(t.Context(), "EDFDVBD6EXAMPLE")
	if err != nil {
		t.Fatalf("GetDistribution: %v", err)
	}

	if got := marshalRoundTrip(t, dist).DistributionConfig.Logging; got == nil || *got != (LoggingConfigXML{}) {
		t.Errorf("Logging = %+v, want the disabled form", got)
	}
}
