//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

// assertS3ErrorCode fails the test unless err is a smithy.APIError with
// the given code.
func assertS3ErrorCode(t *testing.T, err error, want string) {
	t.Helper()

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != want {
		t.Fatalf("expected error code %s, got: %T: %v", want, err, err)
	}
}

func TestS3_BucketWebsite(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-bucket-website"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	})

	// GET before PUT returns NoSuchWebsiteConfiguration.
	_, err = client.GetBucketWebsite(ctx, &s3.GetBucketWebsiteInput{Bucket: aws.String(bucketName)})
	if err == nil {
		t.Fatal("expected error before website configuration is set, got nil")
	}
	assertS3ErrorCode(t, err, "NoSuchWebsiteConfiguration")

	_, err = client.PutBucketWebsite(ctx, &s3.PutBucketWebsiteInput{
		Bucket: aws.String(bucketName),
		WebsiteConfiguration: &types.WebsiteConfiguration{
			IndexDocument: &types.IndexDocument{Suffix: aws.String("index.html")},
			ErrorDocument: &types.ErrorDocument{Key: aws.String("error.html")},
		},
	})
	if err != nil {
		t.Fatalf("failed to put bucket website: %v", err)
	}

	getResult, err := client.GetBucketWebsite(ctx, &s3.GetBucketWebsiteInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to get bucket website: %v", err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), getResult)

	_, err = client.DeleteBucketWebsite(ctx, &s3.DeleteBucketWebsiteInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to delete bucket website: %v", err)
	}

	_, err = client.GetBucketWebsite(ctx, &s3.GetBucketWebsiteInput{Bucket: aws.String(bucketName)})
	if err == nil {
		t.Fatal("expected error after website configuration is deleted, got nil")
	}
	assertS3ErrorCode(t, err, "NoSuchWebsiteConfiguration")
}

func TestS3_BucketLifecycleConfiguration(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-bucket-lifecycle"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	})

	// GET before PUT returns NoSuchLifecycleConfiguration.
	_, err = client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucketName)})
	if err == nil {
		t.Fatal("expected error before lifecycle configuration is set, got nil")
	}
	assertS3ErrorCode(t, err, "NoSuchLifecycleConfiguration")

	_, err = client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:         aws.String("expire-logs"),
					Status:     types.ExpirationStatusEnabled,
					Filter:     &types.LifecycleRuleFilter{Prefix: aws.String("logs/")},
					Expiration: &types.LifecycleExpiration{Days: aws.Int32(30)},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to put bucket lifecycle configuration: %v", err)
	}

	getResult, err := client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to get bucket lifecycle configuration: %v", err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), getResult)

	_, err = client.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to delete bucket lifecycle configuration: %v", err)
	}

	_, err = client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucketName)})
	if err == nil {
		t.Fatal("expected error after lifecycle configuration is deleted, got nil")
	}
	assertS3ErrorCode(t, err, "NoSuchLifecycleConfiguration")
}

func TestS3_BucketLogging(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-bucket-logging"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	})

	// GET before PUT returns an empty status, not an error.
	initial, err := client.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to get bucket logging: %v", err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_initial", initial)

	_, err = client.PutBucketLogging(ctx, &s3.PutBucketLoggingInput{
		Bucket: aws.String(bucketName),
		BucketLoggingStatus: &types.BucketLoggingStatus{
			LoggingEnabled: &types.LoggingEnabled{
				TargetBucket: aws.String("log-target"),
				TargetPrefix: aws.String("logs/"),
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to put bucket logging: %v", err)
	}

	enabled, err := client.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to get bucket logging: %v", err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_enabled", enabled)

	// PUT with an empty status opts back out of logging.
	_, err = client.PutBucketLogging(ctx, &s3.PutBucketLoggingInput{
		Bucket:              aws.String(bucketName),
		BucketLoggingStatus: &types.BucketLoggingStatus{},
	})
	if err != nil {
		t.Fatalf("failed to opt out of bucket logging: %v", err)
	}

	disabled, err := client.GetBucketLogging(ctx, &s3.GetBucketLoggingInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to get bucket logging: %v", err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_disabled", disabled)
}

func TestS3_BucketPolicy(t *testing.T) {
	client := newS3Client(t)
	ctx := t.Context()
	bucketName := "test-bucket-policy"

	// GET / PUT on a missing bucket both return NoSuchBucket.
	_, err := client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String("no-such-bucket-policy")})
	if err == nil {
		t.Fatal("expected error for missing bucket, got nil")
	}
	assertS3ErrorCode(t, err, "NoSuchBucket")

	_, err = client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String("no-such-bucket-policy"),
		Policy: aws.String("{}"),
	})
	if err == nil {
		t.Fatal("expected error for missing bucket, got nil")
	}
	assertS3ErrorCode(t, err, "NoSuchBucket")

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucketName)})
	})

	// GET before PUT returns NoSuchBucketPolicy.
	_, err = client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucketName)})
	if err == nil {
		t.Fatal("expected error before policy is set, got nil")
	}
	assertS3ErrorCode(t, err, "NoSuchBucketPolicy")

	const doc = `{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Principal":"*","Action":"s3:*","Resource":"*","Condition":{"Bool":{"aws:SecureTransport":"false"}}}]}`

	_, err = client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucketName),
		Policy: aws.String(doc),
	})
	if err != nil {
		t.Fatalf("failed to put bucket policy: %v", err)
	}

	getResult, err := client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to get bucket policy: %v", err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), getResult)

	_, err = client.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{Bucket: aws.String(bucketName)})
	if err != nil {
		t.Fatalf("failed to delete bucket policy: %v", err)
	}

	_, err = client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucketName)})
	if err == nil {
		t.Fatal("expected error after policy is deleted, got nil")
	}
	assertS3ErrorCode(t, err, "NoSuchBucketPolicy")
}
