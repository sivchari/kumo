//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/sivchari/golden"
)

func newCloudTrailClient(t *testing.T) *cloudtrail.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatal(err)
	}

	return cloudtrail.NewFromConfig(cfg, func(o *cloudtrail.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
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
