//go:build integration

package integration

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/sivchari/golden"
)

// oacGoldenIgnores hides the per-run identifiers.
var oacGoldenIgnores = golden.WithIgnoreFields("ResultMetadata", "Id", "ETag", "Location", "Name")

func uniqueName(prefix string) string {
	return prefix + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func oacConfig(name string, originType cloudfronttypes.OriginAccessControlOriginTypes, behavior cloudfronttypes.OriginAccessControlSigningBehaviors) *cloudfronttypes.OriginAccessControlConfig {
	return &cloudfronttypes.OriginAccessControlConfig{
		Name:                          aws.String(name),
		Description:                   aws.String("kumo integration test"),
		OriginAccessControlOriginType: originType,
		SigningBehavior:               behavior,
		SigningProtocol:               cloudfronttypes.OriginAccessControlSigningProtocolsSigv4,
	}
}

// createOAC creates an origin access control and deletes it on cleanup
// (with whatever ETag it has by then).
func createOAC(t *testing.T, client *cloudfront.Client, cfg *cloudfronttypes.OriginAccessControlConfig) *cloudfront.CreateOriginAccessControlOutput {
	t.Helper()

	created, err := client.CreateOriginAccessControl(t.Context(), &cloudfront.CreateOriginAccessControlInput{OriginAccessControlConfig: cfg})
	if err != nil {
		t.Fatalf("CreateOriginAccessControl: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()

		current, err := client.GetOriginAccessControl(ctx, &cloudfront.GetOriginAccessControlInput{Id: created.OriginAccessControl.Id})
		if err != nil {
			return
		}

		_, _ = client.DeleteOriginAccessControl(ctx, &cloudfront.DeleteOriginAccessControlInput{Id: created.OriginAccessControl.Id, IfMatch: current.ETag})
	})

	return created
}

func TestCloudFront_OriginAccessControlLifecycle(t *testing.T) {
	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := uniqueName("test-cf-oac-lifecycle")

	created := createOAC(t, client, oacConfig(name, cloudfronttypes.OriginAccessControlOriginTypesLambda, cloudfronttypes.OriginAccessControlSigningBehaviorsAlways))
	golden.New(t, oacGoldenIgnores).Assert(t.Name()+"_create", created)

	id := created.OriginAccessControl.Id

	if aws.ToString(created.Location) != "/2020-05-31/origin-access-control/"+aws.ToString(id) || aws.ToString(created.ETag) == "" {
		t.Errorf("Location = %q, ETag = %q", aws.ToString(created.Location), aws.ToString(created.ETag))
	}

	got, err := client.GetOriginAccessControl(ctx, &cloudfront.GetOriginAccessControlInput{Id: id})
	if err != nil {
		t.Fatalf("GetOriginAccessControl: %v", err)
	}

	golden.New(t, oacGoldenIgnores).Assert(t.Name()+"_get", got)

	config, err := client.GetOriginAccessControlConfig(ctx, &cloudfront.GetOriginAccessControlConfigInput{Id: id})
	if err != nil {
		t.Fatalf("GetOriginAccessControlConfig: %v", err)
	}

	golden.New(t, oacGoldenIgnores).Assert(t.Name()+"_config", config)

	if aws.ToString(config.ETag) != aws.ToString(got.ETag) {
		t.Errorf("config ETag %q != get ETag %q", aws.ToString(config.ETag), aws.ToString(got.ETag))
	}

	updated, err := client.UpdateOriginAccessControl(ctx, &cloudfront.UpdateOriginAccessControlInput{
		Id:                        id,
		IfMatch:                   got.ETag,
		OriginAccessControlConfig: oacConfig(name, cloudfronttypes.OriginAccessControlOriginTypesLambda, cloudfronttypes.OriginAccessControlSigningBehaviorsNoOverride),
	})
	if err != nil {
		t.Fatalf("UpdateOriginAccessControl: %v", err)
	}

	golden.New(t, oacGoldenIgnores).Assert(t.Name()+"_update", updated)

	if aws.ToString(updated.ETag) == aws.ToString(got.ETag) {
		t.Error("update must issue a new ETag")
	}

	listed, err := client.ListOriginAccessControls(ctx, &cloudfront.ListOriginAccessControlsInput{})
	if err != nil {
		t.Fatalf("ListOriginAccessControls: %v", err)
	}

	found := false

	for _, item := range listed.OriginAccessControlList.Items {
		if aws.ToString(item.Id) == aws.ToString(id) {
			found = item.SigningBehavior == cloudfronttypes.OriginAccessControlSigningBehaviorsNoOverride && aws.ToString(item.Name) == name
		}
	}

	if !found {
		t.Errorf("updated control missing from the list: %+v", listed.OriginAccessControlList)
	}

	if _, err := client.DeleteOriginAccessControl(ctx, &cloudfront.DeleteOriginAccessControlInput{Id: id, IfMatch: updated.ETag}); err != nil {
		t.Fatalf("DeleteOriginAccessControl: %v", err)
	}

	_, err = client.GetOriginAccessControl(ctx, &cloudfront.GetOriginAccessControlInput{Id: id})
	assertCloudFrontAPIError(t, err, "NoSuchOriginAccessControl", http.StatusNotFound)
}

func TestCloudFront_OriginAccessControlErrors(t *testing.T) {
	client := newCloudFrontClient(t)
	ctx := t.Context()
	name := uniqueName("test-cf-oac-errors")

	created := createOAC(t, client, oacConfig(name, cloudfronttypes.OriginAccessControlOriginTypesS3, cloudfronttypes.OriginAccessControlSigningBehaviorsAlways))
	id := created.OriginAccessControl.Id

	_, err := client.CreateOriginAccessControl(ctx, &cloudfront.CreateOriginAccessControlInput{OriginAccessControlConfig: oacConfig(name, cloudfronttypes.OriginAccessControlOriginTypesS3, cloudfronttypes.OriginAccessControlSigningBehaviorsAlways)})
	assertCloudFrontAPIError(t, err, "OriginAccessControlAlreadyExists", http.StatusConflict)

	_, err = client.CreateOriginAccessControl(ctx, &cloudfront.CreateOriginAccessControlInput{OriginAccessControlConfig: &cloudfronttypes.OriginAccessControlConfig{
		Name:                          aws.String(uniqueName("test-cf-oac-invalid")),
		OriginAccessControlOriginType: cloudfronttypes.OriginAccessControlOriginTypes("ec2"),
		SigningBehavior:               cloudfronttypes.OriginAccessControlSigningBehaviorsAlways,
		SigningProtocol:               cloudfronttypes.OriginAccessControlSigningProtocolsSigv4,
	}})
	assertCloudFrontAPIError(t, err, "InvalidArgument", http.StatusBadRequest)

	renamed := oacConfig(uniqueName("test-cf-oac-renamed"), cloudfronttypes.OriginAccessControlOriginTypesS3, cloudfronttypes.OriginAccessControlSigningBehaviorsNever)

	_, err = client.UpdateOriginAccessControl(ctx, &cloudfront.UpdateOriginAccessControlInput{Id: id, OriginAccessControlConfig: renamed})
	assertCloudFrontAPIError(t, err, "InvalidIfMatchVersion", http.StatusBadRequest)

	_, err = client.UpdateOriginAccessControl(ctx, &cloudfront.UpdateOriginAccessControlInput{Id: id, IfMatch: aws.String("EStale"), OriginAccessControlConfig: renamed})
	assertCloudFrontAPIError(t, err, "PreconditionFailed", http.StatusPreconditionFailed)

	_, err = client.GetOriginAccessControl(ctx, &cloudfront.GetOriginAccessControlInput{Id: aws.String("EDOESNOTEXIST1")})
	assertCloudFrontAPIError(t, err, "NoSuchOriginAccessControl", http.StatusNotFound)

	// A distribution that references the control pins it.
	dist, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{DistributionConfig: &cloudfronttypes.DistributionConfig{
		CallerReference: aws.String(uniqueName("test-cf-oac-ref")),
		Comment:         aws.String("references an OAC"),
		Enabled:         aws.Bool(true),
		Origins: &cloudfronttypes.Origins{Quantity: aws.Int32(1), Items: []cloudfronttypes.Origin{{
			Id:                    aws.String("assets"),
			DomainName:            aws.String("assets.s3.us-east-1.amazonaws.com"),
			S3OriginConfig:        &cloudfronttypes.S3OriginConfig{OriginAccessIdentity: aws.String("")},
			OriginAccessControlId: id,
		}}},
		DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{TargetOriginId: aws.String("assets"), ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll, CachePolicyId: aws.String(cachingDisabledID)},
	}})
	if err != nil {
		t.Fatalf("CreateDistribution: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{Id: dist.Distribution.Id, IfMatch: dist.ETag})
	})

	_, err = client.DeleteOriginAccessControl(ctx, &cloudfront.DeleteOriginAccessControlInput{Id: id, IfMatch: created.ETag})
	assertCloudFrontAPIError(t, err, "OriginAccessControlInUse", http.StatusConflict)
}

func TestCloudFront_DistributionRejectsInvalidOriginAccess(t *testing.T) {
	client := newCloudFrontClient(t)
	ctx := t.Context()

	lambdaOAC := createOAC(t, client, oacConfig(uniqueName("test-cf-oac-lambda"), cloudfronttypes.OriginAccessControlOriginTypesLambda, cloudfronttypes.OriginAccessControlSigningBehaviorsAlways))
	s3OAC := createOAC(t, client, oacConfig(uniqueName("test-cf-oac-s3"), cloudfronttypes.OriginAccessControlOriginTypesS3, cloudfronttypes.OriginAccessControlSigningBehaviorsAlways))

	create := func(origin cloudfronttypes.Origin) error {
		_, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{DistributionConfig: &cloudfronttypes.DistributionConfig{
			CallerReference:      aws.String(uniqueName("test-cf-oac-bad")),
			Comment:              aws.String("invalid origin access"),
			Enabled:              aws.Bool(true),
			Origins:              &cloudfronttypes.Origins{Quantity: aws.Int32(1), Items: []cloudfronttypes.Origin{origin}},
			DefaultCacheBehavior: &cloudfronttypes.DefaultCacheBehavior{TargetOriginId: origin.Id, ViewerProtocolPolicy: cloudfronttypes.ViewerProtocolPolicyAllowAll, CachePolicyId: aws.String(cachingDisabledID)},
		}})

		return err
	}

	s3Origin := func(oacID *string, oai string) cloudfronttypes.Origin {
		return cloudfronttypes.Origin{Id: aws.String("s3"), DomainName: aws.String("assets.s3.us-east-1.amazonaws.com"), S3OriginConfig: &cloudfronttypes.S3OriginConfig{OriginAccessIdentity: aws.String(oai)}, OriginAccessControlId: oacID}
	}

	unknown := s3Origin(aws.String("EDOESNOTEXIST1"), "")
	assertCloudFrontAPIError(t, create(unknown), "InvalidOriginAccessControl", http.StatusBadRequest)

	withOAI := s3Origin(s3OAC.OriginAccessControl.Id, "origin-access-identity/cloudfront/E1")
	assertCloudFrontAPIError(t, create(withOAI), "IllegalOriginAccessConfiguration", http.StatusBadRequest)

	lambdaOnS3 := s3Origin(lambdaOAC.OriginAccessControl.Id, "")
	assertCloudFrontAPIError(t, create(lambdaOnS3), "InvalidDomainNameForOriginAccessControl", http.StatusBadRequest)

	custom := customOrigin("web", "www.example.com")
	custom.OriginAccessControlId = s3OAC.OriginAccessControl.Id
	assertCloudFrontAPIError(t, create(custom), "InvalidDomainNameForOriginAccessControl", http.StatusBadRequest)
}
