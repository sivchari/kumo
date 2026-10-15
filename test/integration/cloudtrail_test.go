//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

func newCloudTrailClient(t *testing.T) *cloudtrail.Client {
	t.Helper()

	return cloudtrail.NewFromConfig(awsConfig(t), func(o *cloudtrail.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestCloudTrail_CreateAndDeleteTrail(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	trailName := "test-trail-create-delete"
	bucketName := "test-bucket"

	// Create trail.
	createOutput, err := client.CreateTrail(ctx, &cloudtrail.CreateTrailInput{
		Name:         aws.String(trailName),
		S3BucketName: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTrail(context.Background(), &cloudtrail.DeleteTrailInput{
			Name: aws.String(trailName),
		})
	})

	golden.New(t, golden.WithIgnoreFields("TrailARN", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Verify trail was created.
	descOutput, err := client.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(descOutput.TrailList) < 1 {
		t.Fatal("expected at least one trail in DescribeTrails response")
	}

	found := false

	for _, trail := range descOutput.TrailList {
		if *trail.Name == trailName {
			found = true

			break
		}
	}

	if !found {
		t.Fatal("Trail not found in DescribeTrails response")
	}

	// Delete trail.
	_, err = client.DeleteTrail(ctx, &cloudtrail.DeleteTrailInput{
		Name: aws.String(trailName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify trail is deleted.
	descOutput, err = client.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{})
	if err != nil {
		t.Fatal(err)
	}

	for _, trail := range descOutput.TrailList {
		if *trail.Name == trailName {
			t.Fatal("Trail should have been deleted")
		}
	}
}

func TestCloudTrail_GetTrail(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	trailName := "test-trail-get"
	bucketName := "test-bucket"

	// Create trail.
	_, err := client.CreateTrail(ctx, &cloudtrail.CreateTrailInput{
		Name:         aws.String(trailName),
		S3BucketName: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTrail(context.Background(), &cloudtrail.DeleteTrailInput{
			Name: aws.String(trailName),
		})
	})

	// Get trail.
	getOutput, err := client.GetTrail(ctx, &cloudtrail.GetTrailInput{
		Name: aws.String(trailName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("TrailARN", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestCloudTrail_DescribeTrails(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	trailName := "test-trail-describe"
	bucketName := "test-bucket"

	// Create trail.
	_, err := client.CreateTrail(ctx, &cloudtrail.CreateTrailInput{
		Name:         aws.String(trailName),
		S3BucketName: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTrail(context.Background(), &cloudtrail.DeleteTrailInput{
			Name: aws.String(trailName),
		})
	})

	// Describe all trails.
	descOutput, err := client.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{})
	if err != nil {
		t.Fatal(err)
	}

	if len(descOutput.TrailList) < 1 {
		t.Fatal("expected at least one trail in DescribeTrails response")
	}

	// Describe specific trail.
	descOutput, err = client.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{
		TrailNameList: []string{trailName},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("TrailARN", "ResultMetadata")).Assert(t.Name(), descOutput)
}

func TestCloudTrail_StartAndStopLogging(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	trailName := "test-trail-logging"
	bucketName := "test-bucket"

	// Create trail.
	_, err := client.CreateTrail(ctx, &cloudtrail.CreateTrailInput{
		Name:         aws.String(trailName),
		S3BucketName: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTrail(context.Background(), &cloudtrail.DeleteTrailInput{
			Name: aws.String(trailName),
		})
	})

	// Start logging.
	_, err = client.StartLogging(ctx, &cloudtrail.StartLoggingInput{
		Name: aws.String(trailName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify logging started.
	statusOutput, err := client.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{
		Name: aws.String(trailName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("StartLoggingTime", "LatestDeliveryTime", "LatestNotificationTime", "LatestCloudWatchLogsDeliveryTime", "LatestDigestDeliveryTime", "ResultMetadata")).Assert(t.Name()+"_started", statusOutput)

	// Stop logging.
	_, err = client.StopLogging(ctx, &cloudtrail.StopLoggingInput{
		Name: aws.String(trailName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify logging stopped.
	statusOutput, err = client.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{
		Name: aws.String(trailName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("StartLoggingTime", "StopLoggingTime", "LatestDeliveryTime", "LatestNotificationTime", "LatestCloudWatchLogsDeliveryTime", "LatestDigestDeliveryTime", "ResultMetadata")).Assert(t.Name()+"_stopped", statusOutput)
}

func TestCloudTrail_StartLoggingByARN(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	trailName := "test-trail-arn-alias"
	bucketName := "test-bucket"

	// Create trail with the short name, as Terraform's CreateTrail does.
	createOutput, err := client.CreateTrail(ctx, &cloudtrail.CreateTrailInput{
		Name:         aws.String(trailName),
		S3BucketName: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTrail(context.Background(), &cloudtrail.DeleteTrailInput{
			Name: aws.String(trailName),
		})
	})

	// Start logging using the ARN, as Terraform does (it holds the ARN as resource ID).
	_, err = client.StartLogging(ctx, &cloudtrail.StartLoggingInput{
		Name: createOutput.TrailARN,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify logging started, referenced by the short name.
	statusOutput, err := client.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{
		Name: aws.String(trailName),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("StartLoggingTime", "LatestDeliveryTime", "LatestNotificationTime", "LatestCloudWatchLogsDeliveryTime", "LatestDigestDeliveryTime", "ResultMetadata")).Assert(t.Name(), statusOutput)
}

func TestCloudTrail_LookupEvents(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	// LookupEvents returns empty list for MVP.
	output, err := client.LookupEvents(ctx, &cloudtrail.LookupEventsInput{})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), output)
}

func TestCloudTrail_TrailNotFound(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	// Get non-existent trail.
	_, err := client.GetTrail(ctx, &cloudtrail.GetTrailInput{
		Name: aws.String("non-existent-trail"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent trail.
	_, err = client.DeleteTrail(ctx, &cloudtrail.DeleteTrailInput{
		Name: aws.String("non-existent-trail"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Start logging on non-existent trail.
	_, err = client.StartLogging(ctx, &cloudtrail.StartLoggingInput{
		Name: aws.String("non-existent-trail"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Stop logging on non-existent trail.
	_, err = client.StopLogging(ctx, &cloudtrail.StopLoggingInput{
		Name: aws.String("non-existent-trail"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestCloudTrail_DuplicateTrail(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	trailName := "test-trail-duplicate"
	bucketName := "test-bucket"

	// Create trail.
	_, err := client.CreateTrail(ctx, &cloudtrail.CreateTrailInput{
		Name:         aws.String(trailName),
		S3BucketName: aws.String(bucketName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTrail(context.Background(), &cloudtrail.DeleteTrailInput{
			Name: aws.String(trailName),
		})
	})

	// Try to create duplicate trail.
	_, err = client.CreateTrail(ctx, &cloudtrail.CreateTrailInput{
		Name:         aws.String(trailName),
		S3BucketName: aws.String(bucketName),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestCloudTrail_CreateTrailWithOptions(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	trailName := "test-trail-options"
	bucketName := "test-bucket"
	s3Prefix := "logs/"

	// Create trail with options.
	createOutput, err := client.CreateTrail(ctx, &cloudtrail.CreateTrailInput{
		Name:                       aws.String(trailName),
		S3BucketName:               aws.String(bucketName),
		S3KeyPrefix:                aws.String(s3Prefix),
		IncludeGlobalServiceEvents: aws.Bool(false),
		IsMultiRegionTrail:         aws.Bool(true),
		EnableLogFileValidation:    aws.Bool(true),
		IsOrganizationTrail:        aws.Bool(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTrail(context.Background(), &cloudtrail.DeleteTrailInput{
			Name: aws.String(trailName),
		})
	})

	golden.New(t, golden.WithIgnoreFields("TrailARN", "ResultMetadata")).Assert(t.Name(), createOutput)
}

// createTaggedTrail creates a trail with the given tags and returns its ARN,
// registering deletion for cleanup.
func createTaggedTrail(t *testing.T, client *cloudtrail.Client, trailName string, tags []cloudtrailtypes.Tag) string {
	t.Helper()

	createOutput, err := client.CreateTrail(t.Context(), &cloudtrail.CreateTrailInput{
		Name:         aws.String(trailName),
		S3BucketName: aws.String("test-bucket"),
		TagsList:     tags,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTrail(context.Background(), &cloudtrail.DeleteTrailInput{
			Name: aws.String(trailName),
		})
	})

	return aws.ToString(createOutput.TrailARN)
}

// assertTrailTags lists the tags of trailARN and golden-asserts them. ResourceId
// is checked here rather than in the golden because the ARN embeds the
// configured region and account.
func assertTrailTags(t *testing.T, client *cloudtrail.Client, trailARN string) {
	t.Helper()

	output, err := client.ListTags(t.Context(), &cloudtrail.ListTagsInput{
		ResourceIdList: []string{trailARN},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(output.ResourceTagList) != 1 {
		t.Fatalf("ResourceTagList length: got %d, want 1", len(output.ResourceTagList))
	}

	if got := aws.ToString(output.ResourceTagList[0].ResourceId); got != trailARN {
		t.Errorf("ResourceId: got %q, want %q", got, trailARN)
	}

	golden.New(t, golden.WithIgnoreFields("ResourceId", "ResultMetadata")).Assert(t.Name(), output)
}

func TestCloudTrail_ListTags(t *testing.T) {
	client := newCloudTrailClient(t)

	// Terraform supplies tags on create, then reads them back with ListTags.
	trailARN := createTaggedTrail(t, client, "test-trail-list-tags", []cloudtrailtypes.Tag{
		{Key: aws.String("env"), Value: aws.String("local")},
		{Key: aws.String("app"), Value: aws.String("idp")},
	})

	assertTrailTags(t, client, trailARN)
}

func TestCloudTrail_AddTags(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	trailARN := createTaggedTrail(t, client, "test-trail-add-tags", nil)

	_, err := client.AddTags(ctx, &cloudtrail.AddTagsInput{
		ResourceId: aws.String(trailARN),
		TagsList: []cloudtrailtypes.Tag{
			{Key: aws.String("env"), Value: aws.String("local")},
			{Key: aws.String("team"), Value: aws.String("sec")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertTrailTags(t, client, trailARN)
}

func TestCloudTrail_RemoveTags(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	trailARN := createTaggedTrail(t, client, "test-trail-remove-tags", []cloudtrailtypes.Tag{
		{Key: aws.String("env"), Value: aws.String("local")},
		{Key: aws.String("team"), Value: aws.String("sec")},
	})

	// RemoveTags matches on key, so the value is irrelevant.
	_, err := client.RemoveTags(ctx, &cloudtrail.RemoveTagsInput{
		ResourceId: aws.String(trailARN),
		TagsList:   []cloudtrailtypes.Tag{{Key: aws.String("team")}},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertTrailTags(t, client, trailARN)
}

// TestCloudTrail_ListTagsUnknownResource pins kumo's deliberate divergence from
// AWS: ListTags on an unknown ARN yields an empty tag list instead of
// ResourceNotFoundException, so a Terraform refresh over a trail that no longer
// exists does not fail.
func TestCloudTrail_ListTagsUnknownResource(t *testing.T) {
	client := newCloudTrailClient(t)

	output, err := client.ListTags(t.Context(), &cloudtrail.ListTagsInput{
		ResourceIdList: []string{"arn:aws:cloudtrail:us-east-1:123456789012:trail/absent-trail"},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), output)
}

// TestCloudTrail_TagsListRequired covers the AddTags / RemoveTags guard: AWS
// marks TagsList required, so an empty list must be rejected rather than
// silently succeeding as a no-op.
func TestCloudTrail_TagsListRequired(t *testing.T) {
	client := newCloudTrailClient(t)
	ctx := t.Context()

	trailARN := createTaggedTrail(t, client, "test-trail-empty-tags", nil)

	_, err := client.AddTags(ctx, &cloudtrail.AddTagsInput{
		ResourceId: aws.String(trailARN),
		TagsList:   []cloudtrailtypes.Tag{},
	})
	assertAPIErrorCode(t, err, "ValidationException")

	_, err = client.RemoveTags(ctx, &cloudtrail.RemoveTagsInput{
		ResourceId: aws.String(trailARN),
		TagsList:   []cloudtrailtypes.Tag{},
	})
	assertAPIErrorCode(t, err, "ValidationException")
}

// assertAPIErrorCode fails unless err is an API error carrying wantCode.
func assertAPIErrorCode(t *testing.T, err error, wantCode string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error with code %q, got nil", wantCode)
	}

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected smithy.APIError, got %T: %v", err, err)
	}

	if apiErr.ErrorCode() != wantCode {
		t.Errorf("error code: got %q, want %q", apiErr.ErrorCode(), wantCode)
	}
}
