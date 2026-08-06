//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

func newCloudFrontClient(t *testing.T) *cloudfront.Client {
	t.Helper()

	return cloudfront.NewFromConfig(awsConfig(t), func(o *cloudfront.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestCloudFront_CreateDistribution(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	result, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-create-distribution"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"ARN",
		"DomainName",
		"LastModifiedTime",
		"ETag",
		"Location",
		"ResultMetadata",
	)).Assert(t.Name(), result)

	// Clean up.
	_, err = client.DeleteDistribution(ctx, &cloudfront.DeleteDistributionInput{
		Id:      result.Distribution.Id,
		IfMatch: result.ETag,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCloudFront_GetDistribution(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	// Create distribution first.
	createResult, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-get-distribution"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      createResult.Distribution.Id,
			IfMatch: createResult.ETag,
		})
	})

	// Get distribution.
	getResult, err := client.GetDistribution(ctx, &cloudfront.GetDistributionInput{
		Id: createResult.Distribution.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"ARN",
		"DomainName",
		"LastModifiedTime",
		"ETag",
		"Location",
		"ResultMetadata",
	)).Assert(t.Name(), getResult)
}

func TestCloudFront_ListDistributions(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	// Create distribution first.
	createResult, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-list-distributions"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      createResult.Distribution.Id,
			IfMatch: createResult.ETag,
		})
	})

	// List distributions.
	listResult, err := client.ListDistributions(ctx, &cloudfront.ListDistributionsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if listResult == nil {
		t.Fatal("listResult is nil")
	}
	if listResult.DistributionList == nil {
		t.Fatal("listResult.DistributionList is nil")
	}

	// Find our distribution.
	found := false
	for _, dist := range listResult.DistributionList.Items {
		if *dist.Id == *createResult.Distribution.Id {
			found = true

			break
		}
	}
	if !found {
		t.Error("Distribution should be in list")
	}
}

func TestCloudFront_UpdateDistribution(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	// Create distribution first.
	createResult, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-update-distribution"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update distribution.
	updateResult, err := client.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{
		Id:      createResult.Distribution.Id,
		IfMatch: createResult.ETag,
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-update-distribution"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Updated comment"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"ARN",
		"DomainName",
		"LastModifiedTime",
		"ETag",
		"Location",
		"ResultMetadata",
	)).Assert(t.Name(), updateResult)

	// Clean up with new ETag.
	_, err = client.DeleteDistribution(ctx, &cloudfront.DeleteDistributionInput{
		Id:      createResult.Distribution.Id,
		IfMatch: updateResult.ETag,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCloudFront_CreateInvalidation(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	// Create distribution first.
	createResult, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-create-invalidation"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      createResult.Distribution.Id,
			IfMatch: createResult.ETag,
		})
	})

	// Create invalidation.
	invResult, err := client.CreateInvalidation(ctx, &cloudfront.CreateInvalidationInput{
		DistributionId: createResult.Distribution.Id,
		InvalidationBatch: &types.InvalidationBatch{
			CallerReference: aws.String("test-invalidation-1"),
			Paths: &types.Paths{
				Quantity: aws.Int32(1),
				Items:    []string{"/*"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"CreateTime",
		"Location",
		"ResultMetadata",
	)).Assert(t.Name(), invResult)
}

func TestCloudFront_GetInvalidation(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	// Create distribution first.
	createResult, err := client.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &types.DistributionConfig{
			CallerReference: aws.String("test-get-invalidation"),
			Origins: &types.Origins{
				Quantity: aws.Int32(1),
				Items: []types.Origin{
					{
						Id:         aws.String("myS3Origin"),
						DomainName: aws.String("mybucket.s3.amazonaws.com"),
						S3OriginConfig: &types.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
					},
				},
			},
			DefaultCacheBehavior: &types.DefaultCacheBehavior{
				TargetOriginId:       aws.String("myS3Origin"),
				ViewerProtocolPolicy: types.ViewerProtocolPolicyAllowAll,
				CachePolicyId:        aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			},
			Comment: aws.String("Test distribution"),
			Enabled: aws.Bool(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDistribution(context.Background(), &cloudfront.DeleteDistributionInput{
			Id:      createResult.Distribution.Id,
			IfMatch: createResult.ETag,
		})
	})

	// Create invalidation.
	invResult, err := client.CreateInvalidation(ctx, &cloudfront.CreateInvalidationInput{
		DistributionId: createResult.Distribution.Id,
		InvalidationBatch: &types.InvalidationBatch{
			CallerReference: aws.String("test-get-invalidation-1"),
			Paths: &types.Paths{
				Quantity: aws.Int32(1),
				Items:    []string{"/images/*"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get invalidation.
	getResult, err := client.GetInvalidation(ctx, &cloudfront.GetInvalidationInput{
		DistributionId: createResult.Distribution.Id,
		Id:             invResult.Invalidation.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"CreateTime",
		"ResultMetadata",
	)).Assert(t.Name(), getResult)
}

func TestCloudFront_PublicKeyCRUD(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	_, pubPEM := generateTestKeyPair(t)

	createResult, err := client.CreatePublicKey(ctx, &cloudfront.CreatePublicKeyInput{
		PublicKeyConfig: &types.PublicKeyConfig{
			CallerReference: aws.String("test-pk-crud"),
			Name:            aws.String("test-pk-crud-key"),
			EncodedKey:      aws.String(pubPEM),
			Comment:         aws.String("CRUD test key"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"CreatedTime",
		"ETag",
		"EncodedKey",
		"ResultMetadata",
	)).Assert(t.Name()+"_create", createResult)

	// Get.
	getResult, err := client.GetPublicKey(ctx, &cloudfront.GetPublicKeyInput{
		Id: createResult.PublicKey.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"CreatedTime",
		"ETag",
		"EncodedKey",
		"ResultMetadata",
	)).Assert(t.Name()+"_get", getResult)

	// List: find our key by ID rather than asserting the whole list,
	// since other parallel tests create PublicKeys too.
	listResult, err := client.ListPublicKeys(ctx, &cloudfront.ListPublicKeysInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	if listResult.PublicKeyList != nil {
		for _, item := range listResult.PublicKeyList.Items {
			if aws.ToString(item.Id) == aws.ToString(createResult.PublicKey.Id) {
				found = true

				break
			}
		}
	}

	if !found {
		t.Error("PublicKey should be in list")
	}

	// Delete.
	_, err = client.DeletePublicKey(ctx, &cloudfront.DeletePublicKeyInput{
		Id:      createResult.PublicKey.Id,
		IfMatch: createResult.ETag,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get-after-delete is NoSuchPublicKey.
	_, err = client.GetPublicKey(ctx, &cloudfront.GetPublicKeyInput{
		Id: createResult.PublicKey.Id,
	})
	if err == nil {
		t.Fatal("expected NoSuchPublicKey error after delete")
	}

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "NoSuchPublicKey" {
		t.Fatalf("expected NoSuchPublicKey, got: %T: %v", err, err)
	}
}

func TestCloudFront_KeyGroupCRUD(t *testing.T) {
	t.Parallel()

	client := newCloudFrontClient(t)
	ctx := t.Context()

	_, pubPEM := generateTestKeyPair(t)

	pkResult, err := client.CreatePublicKey(ctx, &cloudfront.CreatePublicKeyInput{
		PublicKeyConfig: &types.PublicKeyConfig{
			CallerReference: aws.String("test-kg-crud-pk"),
			Name:            aws.String("test-kg-crud-key"),
			EncodedKey:      aws.String(pubPEM),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeletePublicKey(context.Background(), &cloudfront.DeletePublicKeyInput{
			Id:      pkResult.PublicKey.Id,
			IfMatch: pkResult.ETag,
		})
	})

	createResult, err := client.CreateKeyGroup(ctx, &cloudfront.CreateKeyGroupInput{
		KeyGroupConfig: &types.KeyGroupConfig{
			Name:    aws.String("test-kg-crud-group"),
			Items:   []string{*pkResult.PublicKey.Id},
			Comment: aws.String("CRUD test group"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"LastModifiedTime",
		"ETag",
		"Items",
		"ResultMetadata",
	)).Assert(t.Name()+"_create", createResult)

	// Get.
	getResult, err := client.GetKeyGroup(ctx, &cloudfront.GetKeyGroupInput{
		Id: createResult.KeyGroup.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields(
		"Id",
		"LastModifiedTime",
		"ETag",
		"Items",
		"ResultMetadata",
	)).Assert(t.Name()+"_get", getResult)

	// List: find our group by ID rather than asserting the whole list.
	listResult, err := client.ListKeyGroups(ctx, &cloudfront.ListKeyGroupsInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	if listResult.KeyGroupList != nil {
		for _, item := range listResult.KeyGroupList.Items {
			if item.KeyGroup != nil && aws.ToString(item.KeyGroup.Id) == aws.ToString(createResult.KeyGroup.Id) {
				found = true

				break
			}
		}
	}

	if !found {
		t.Error("KeyGroup should be in list")
	}

	// Deleting the referenced PublicKey must fail with PublicKeyInUse.
	_, err = client.DeletePublicKey(ctx, &cloudfront.DeletePublicKeyInput{
		Id:      pkResult.PublicKey.Id,
		IfMatch: pkResult.ETag,
	})
	if err == nil {
		t.Fatal("expected PublicKeyInUse error while KeyGroup references it")
	}

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "PublicKeyInUse" {
		t.Fatalf("expected PublicKeyInUse, got: %T: %v", err, err)
	}

	// Delete KeyGroup.
	_, err = client.DeleteKeyGroup(ctx, &cloudfront.DeleteKeyGroupInput{
		Id:      createResult.KeyGroup.Id,
		IfMatch: createResult.ETag,
	})
	if err != nil {
		t.Fatal(err)
	}
}
