package cloudfront

import (
	"context"
	"errors"
	"testing"
)

const (
	oacTestS3Domain     = "mybucket.s3.us-east-1.amazonaws.com"
	oacTestLambdaDomain = "abc123.lambda-url.us-east-1.on.aws"
	oacTestPlainDomain  = "example.com"
)

func validOAC(name, originType string) *OriginAccessControlConfig {
	return &OriginAccessControlConfig{Name: name, SigningProtocol: oacSigningProtocolSigV4, SigningBehavior: oacSigningAlways, OriginAccessControlOriginType: originType}
}

func errorCode(t *testing.T, err error) string {
	t.Helper()

	var cfErr *Error
	if !errors.As(err, &cfErr) {
		t.Fatalf("expected a CloudFront error, got %v", err)
	}

	return cfErr.Code
}

func TestValidateOriginAccessControlConfig(t *testing.T) {
	t.Parallel()

	cases := map[string]*OriginAccessControlConfig{
		"empty name":       {SigningProtocol: oacSigningProtocolSigV4, SigningBehavior: oacSigningAlways, OriginAccessControlOriginType: oacOriginTypeS3},
		"long name":        validOAC(string(make([]byte, oacNameMaxLength+1)), oacOriginTypeS3),
		"bad origin type":  validOAC("n", "ec2"),
		"bad behavior":     {Name: "n", SigningProtocol: oacSigningProtocolSigV4, SigningBehavior: "sometimes", OriginAccessControlOriginType: oacOriginTypeS3},
		"bad protocol":     {Name: "n", SigningProtocol: "sigv2", SigningBehavior: oacSigningAlways, OriginAccessControlOriginType: oacOriginTypeS3},
		"missing protocol": {Name: "n", SigningBehavior: oacSigningAlways, OriginAccessControlOriginType: oacOriginTypeS3},
	}

	for name, cfg := range cases {
		if err := validateOriginAccessControlConfig(cfg); err == nil || errorCode(t, err) != errInvalidArgument {
			t.Errorf("%s: err = %v, want InvalidArgument", name, err)
		}
	}

	for _, originType := range []string{oacOriginTypeS3, oacOriginTypeLambda, oacOriginTypeMediaStore, oacOriginTypeMediaPackageV2} {
		if err := validateOriginAccessControlConfig(validOAC("n", originType)); err != nil {
			t.Errorf("%s: unexpected error %v", originType, err)
		}
	}
}

func TestOriginDomainSupportsOAC(t *testing.T) {
	t.Parallel()

	cases := []struct {
		originType, domain string
		want               bool
	}{
		{oacOriginTypeS3, oacTestS3Domain, true},
		{oacOriginTypeS3, "mybucket.s3.amazonaws.com", true},
		{oacOriginTypeS3, "mybucket.s3-us-west-2.amazonaws.com", true},
		{oacOriginTypeS3, "mybucket.s3.dualstack.us-east-1.amazonaws.com", true},
		{oacOriginTypeS3, "my-ap-123456789012.s3-accesspoint.us-east-1.amazonaws.com", true},
		{oacOriginTypeS3, "mfzwi23gnjvgw.mrap.accesspoint.s3-global.amazonaws.com", true},
		{oacOriginTypeS3, "mybucket.s3.cn-north-1.amazonaws.com.cn", true},
		{oacOriginTypeS3, "mybucket.s3-website-us-east-1.amazonaws.com", false},
		{oacOriginTypeS3, "mybucket.s3-website.ap-northeast-1.amazonaws.com", false},
		{oacOriginTypeS3, "mybucket.s3-accelerate.amazonaws.com", false},
		{oacOriginTypeS3, oacTestLambdaDomain, false},
		{oacOriginTypeS3, oacTestPlainDomain, false},
		{oacOriginTypeLambda, oacTestLambdaDomain, true},
		{oacOriginTypeLambda, testLambdaURLLocal, true},
		{oacOriginTypeLambda, oacTestS3Domain, false},
		{oacOriginTypeLambda, "app.lambda-url.example.com", false},
		{oacOriginTypeMediaStore, oacTestPlainDomain, true},
		{oacOriginTypeMediaPackageV2, oacTestPlainDomain, true},
	}

	for _, tc := range cases {
		if got := originDomainSupportsOAC(tc.originType, tc.domain); got != tc.want {
			t.Errorf("originDomainSupportsOAC(%q, %q) = %v, want %v", tc.originType, tc.domain, got, tc.want)
		}
	}
}

func TestListOriginAccessControls_Pagination(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	for _, name := range []string{"a", "b", "c"} {
		if _, err := store.CreateOriginAccessControl(ctx, validOAC(name, oacOriginTypeS3)); err != nil {
			t.Fatalf("CreateOriginAccessControl(%s): %v", name, err)
		}
	}

	first, next, err := store.ListOriginAccessControls(ctx, "", 2)
	if err != nil || len(first) != 2 || next != first[1].ID {
		t.Fatalf("first page = %d items, next %q, err %v", len(first), next, err)
	}

	second, next, err := store.ListOriginAccessControls(ctx, next, 2)
	if err != nil || len(second) != 1 || next != "" || second[0].ID <= first[1].ID {
		t.Fatalf("second page = %d items, next %q, err %v", len(second), next, err)
	}

	all, _, _ := store.ListOriginAccessControls(ctx, "", oacListMaxItems)
	if len(all) != 3 {
		t.Fatalf("all = %d items", len(all))
	}

	// A non-positive page size falls back to the default instead of panicking.
	defaulted, next, err := store.ListOriginAccessControls(ctx, "", 0)
	if err != nil || len(defaulted) != 3 || next != "" {
		t.Fatalf("maxItems 0 = %d items, next %q, err %v", len(defaulted), next, err)
	}
}

func TestOriginAccessControl_WriteProtocol(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	created, err := store.CreateOriginAccessControl(ctx, validOAC("first", oacOriginTypeLambda))
	if err != nil {
		t.Fatalf("CreateOriginAccessControl: %v", err)
	}

	if _, err := store.CreateOriginAccessControl(ctx, validOAC("first", oacOriginTypeS3)); errorCode(t, err) != errOriginAccessControlAlreadyExists {
		t.Errorf("duplicate name: %v", err)
	}

	if _, err := store.UpdateOriginAccessControl(ctx, created.ID, validOAC("renamed", oacOriginTypeLambda), ""); errorCode(t, err) != errInvalidIfMatchVersion {
		t.Errorf("missing If-Match: %v", err)
	}

	if _, err := store.UpdateOriginAccessControl(ctx, created.ID, validOAC("renamed", oacOriginTypeLambda), "stale"); errorCode(t, err) != errPreconditionFailed {
		t.Errorf("stale If-Match: %v", err)
	}

	updated, err := store.UpdateOriginAccessControl(ctx, created.ID, validOAC("renamed", oacOriginTypeLambda), created.ETag)
	if err != nil || updated.ETag == created.ETag || updated.Config.Name != "renamed" {
		t.Fatalf("update: %+v, %v", updated, err)
	}

	if err := store.DeleteOriginAccessControl(ctx, created.ID, created.ETag); errorCode(t, err) != errPreconditionFailed {
		t.Errorf("delete with the old ETag: %v", err)
	}

	if err := store.DeleteOriginAccessControl(ctx, created.ID, updated.ETag); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := store.GetOriginAccessControl(ctx, created.ID); errorCode(t, err) != errNoSuchOriginAccessControl {
		t.Errorf("get after delete: %v", err)
	}
}

func TestOriginAccessControl_DistributionValidation(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	ctx := context.Background()

	s3OAC, err := store.CreateOriginAccessControl(ctx, validOAC("s3", oacOriginTypeS3))
	if err != nil {
		t.Fatalf("CreateOriginAccessControl: %v", err)
	}

	lambdaOAC, err := store.CreateOriginAccessControl(ctx, validOAC("lambda", oacOriginTypeLambda))
	if err != nil {
		t.Fatalf("CreateOriginAccessControl: %v", err)
	}

	create := func(origin OriginXML) error {
		_, err := store.CreateDistribution(ctx, &CreateDistributionRequest{
			CallerReference:      "oac-" + origin.ID + "-" + origin.OriginAccessControlID + origin.DomainName,
			Enabled:              true,
			Origins:              &OriginsXML{Quantity: 1, Items: &OriginList{Origin: []OriginXML{origin}}},
			DefaultCacheBehavior: &DefaultCacheBehaviorXML{TargetOriginID: origin.ID},
		})

		return err
	}

	cases := []struct {
		name   string
		origin OriginXML
		want   string
	}{
		{"unknown id", OriginXML{ID: "a", DomainName: oacTestS3Domain, S3OriginConfig: &S3OriginConfigXML{}, OriginAccessControlID: "EMISSING"}, errInvalidOriginAccessControl},
		{"oac with oai", OriginXML{ID: "b", DomainName: oacTestS3Domain, S3OriginConfig: &S3OriginConfigXML{OriginAccessIdentity: "origin-access-identity/cloudfront/E1"}, OriginAccessControlID: s3OAC.ID}, errIllegalOriginAccessConfiguration},
		{"lambda oac on s3 domain", OriginXML{ID: "c", DomainName: oacTestS3Domain, S3OriginConfig: &S3OriginConfigXML{}, OriginAccessControlID: lambdaOAC.ID}, errInvalidDomainNameForOriginAccessControl},
		{"s3 oac on lambda domain", OriginXML{ID: "d", DomainName: oacTestLambdaDomain, CustomOriginConfig: &CustomOriginConfigXML{OriginProtocolPolicy: originPolicyHTTPS}, OriginAccessControlID: s3OAC.ID}, errInvalidDomainNameForOriginAccessControl},
	}

	for _, tc := range cases {
		if err := create(tc.origin); errorCode(t, err) != tc.want {
			t.Errorf("%s: err = %v, want %s", tc.name, err, tc.want)
		}
	}

	if err := create(OriginXML{ID: "ok-s3", DomainName: oacTestS3Domain, S3OriginConfig: &S3OriginConfigXML{}, OriginAccessControlID: s3OAC.ID}); err != nil {
		t.Errorf("valid s3 origin: %v", err)
	}

	if err := create(OriginXML{ID: "ok-lambda", DomainName: oacTestLambdaDomain, CustomOriginConfig: &CustomOriginConfigXML{OriginProtocolPolicy: originPolicyHTTPS}, OriginAccessControlID: lambdaOAC.ID}); err != nil {
		t.Errorf("valid lambda origin: %v", err)
	}

	if err := store.DeleteOriginAccessControl(ctx, s3OAC.ID, s3OAC.ETag); errorCode(t, err) != errOriginAccessControlInUse {
		t.Errorf("delete while referenced: %v", err)
	}
}
