//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/securitylake"
	"github.com/aws/aws-sdk-go-v2/service/securitylake/types"
	"github.com/sivchari/golden"
)

func newSecurityLakeClient(t *testing.T) *securitylake.Client {
	t.Helper()

	return securitylake.NewFromConfig(awsConfig(t), func(o *securitylake.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestSecurityLake_CreateAndDeleteDataLake(t *testing.T) {
	client := newSecurityLakeClient(t)
	ctx := t.Context()

	// Create data lake.
	createOutput, err := client.CreateDataLake(ctx, &securitylake.CreateDataLakeInput{
		MetaStoreManagerRoleArn: aws.String("arn:aws:iam::123456789012:role/AmazonSecurityLakeMetaStoreManager"),
		Configurations: []types.DataLakeConfiguration{
			{
				Region: aws.String("us-east-1"),
				EncryptionConfiguration: &types.DataLakeEncryptionConfiguration{
					KmsKeyId: aws.String("alias/aws/securitylake"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "DataLakeArn", "S3BucketArn"),
	)
	g.Assert("create", createOutput)

	// List data lakes.
	listOutput, err := client.ListDataLakes(ctx, &securitylake.ListDataLakesInput{
		Regions: []string{"us-east-1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "DataLakeArn", "S3BucketArn"),
	)
	g2.Assert("list", listOutput)

	// Delete data lake.
	_, err = client.DeleteDataLake(ctx, &securitylake.DeleteDataLakeInput{
		Regions: []string{"us-east-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSecurityLake_CreateAndDeleteSubscriber(t *testing.T) {
	client := newSecurityLakeClient(t)
	ctx := t.Context()

	// First create a data lake.
	_, err := client.CreateDataLake(ctx, &securitylake.CreateDataLakeInput{
		MetaStoreManagerRoleArn: aws.String("arn:aws:iam::123456789012:role/AmazonSecurityLakeMetaStoreManager"),
		Configurations: []types.DataLakeConfiguration{
			{
				Region: aws.String("us-west-1"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDataLake(t.Context(), &securitylake.DeleteDataLakeInput{
			Regions: []string{"us-west-1"},
		})
	})

	// Create subscriber.
	createOutput, err := client.CreateSubscriber(ctx, &securitylake.CreateSubscriberInput{
		SubscriberName: aws.String("test-subscriber"),
		SubscriberIdentity: &types.AwsIdentity{
			ExternalId: aws.String("test-external-id"),
			Principal:  aws.String("123456789012"),
		},
		Sources: []types.LogSourceResource{
			&types.LogSourceResourceMemberAwsLogSource{
				Value: types.AwsLogSourceResource{
					SourceName:    types.AwsLogSourceNameCloudTrailMgmt,
					SourceVersion: aws.String("1.0"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	subscriberID := *createOutput.Subscriber.SubscriberId

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "SubscriberId", "SubscriberArn", "CreatedAt", "UpdatedAt"),
	)
	g.Assert("create", createOutput)

	// Get subscriber.
	getOutput, err := client.GetSubscriber(ctx, &securitylake.GetSubscriberInput{
		SubscriberId: aws.String(subscriberID),
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "SubscriberId", "SubscriberArn", "CreatedAt", "UpdatedAt"),
	)
	g2.Assert("get", getOutput)

	// List subscribers.
	listOutput, err := client.ListSubscribers(ctx, &securitylake.ListSubscribersInput{})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "SubscriberId", "SubscriberArn", "CreatedAt", "UpdatedAt"),
	)
	g3.Assert("list", listOutput)

	// Delete subscriber.
	_, err = client.DeleteSubscriber(ctx, &securitylake.DeleteSubscriberInput{
		SubscriberId: aws.String(subscriberID),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's deleted - should return error.
	_, err = client.GetSubscriber(ctx, &securitylake.GetSubscriberInput{
		SubscriberId: aws.String(subscriberID),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSecurityLake_CreateAndDeleteAwsLogSource(t *testing.T) {
	client := newSecurityLakeClient(t)
	ctx := t.Context()

	// First create a data lake.
	_, err := client.CreateDataLake(ctx, &securitylake.CreateDataLakeInput{
		MetaStoreManagerRoleArn: aws.String("arn:aws:iam::123456789012:role/AmazonSecurityLakeMetaStoreManager"),
		Configurations: []types.DataLakeConfiguration{
			{
				Region: aws.String("eu-west-1"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteDataLake(t.Context(), &securitylake.DeleteDataLakeInput{
			Regions: []string{"eu-west-1"},
		})
	})

	// Create AWS log source.
	createOutput, err := client.CreateAwsLogSource(ctx, &securitylake.CreateAwsLogSourceInput{
		Sources: []types.AwsLogSourceConfiguration{
			{
				SourceName:    types.AwsLogSourceNameCloudTrailMgmt,
				SourceVersion: aws.String("1.0"),
				Regions:       []string{"eu-west-1"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert("create", createOutput)

	// List log sources.
	listOutput, err := client.ListLogSources(ctx, &securitylake.ListLogSourcesInput{
		Regions: []string{"eu-west-1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g2.Assert("list", listOutput)

	// Delete AWS log source.
	_, err = client.DeleteAwsLogSource(ctx, &securitylake.DeleteAwsLogSourceInput{
		Sources: []types.AwsLogSourceConfiguration{
			{
				SourceName:    types.AwsLogSourceNameCloudTrailMgmt,
				SourceVersion: aws.String("1.0"),
				Regions:       []string{"eu-west-1"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
