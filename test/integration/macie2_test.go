//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/macie2"
	"github.com/aws/aws-sdk-go-v2/service/macie2/types"
	smithymiddleware "github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/sivchari/golden"
)

func newMacie2Client(t *testing.T) *macie2.Client {
	t.Helper()

	return macie2.NewFromConfig(awsConfig(t), func(o *macie2.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
		// Disable host prefix to route requests to localhost.
		o.APIOptions = append(o.APIOptions, func(stack *smithymiddleware.Stack) error {
			return stack.Serialize.Add(smithymiddleware.SerializeMiddlewareFunc(
				"DisableHostPrefix",
				func(ctx context.Context, in smithymiddleware.SerializeInput, next smithymiddleware.SerializeHandler) (smithymiddleware.SerializeOutput, smithymiddleware.Metadata, error) {
					ctx = smithyhttp.DisableEndpointHostPrefix(ctx, true)
					return next.HandleSerialize(ctx, in)
				},
			), smithymiddleware.Before)
		})
	})
}

func TestMacie2_EnableAndGetSession(t *testing.T) {
	client := newMacie2Client(t)
	ctx := t.Context()

	_, err := client.EnableMacie(ctx, &macie2.EnableMacieInput{
		FindingPublishingFrequency: types.FindingPublishingFrequencyFifteenMinutes,
		Status:                     types.MacieStatusEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DisableMacie(context.Background(), &macie2.DisableMacieInput{})
	})

	getOutput, err := client.GetMacieSession(ctx, &macie2.GetMacieSessionInput{})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("CreatedAt", "UpdatedAt", "ServiceRole", "ResultMetadata"))
	g.Assert(t.Name(), getOutput)
}

func TestMacie2_CreateAndDeleteAllowList(t *testing.T) {
	client := newMacie2Client(t)
	ctx := t.Context()

	createOutput, err := client.CreateAllowList(ctx, &macie2.CreateAllowListInput{
		ClientToken: aws.String("test-token"),
		Name:        aws.String("test-allow-list"),
		Description: aws.String("Test allow list"),
		Criteria: &types.AllowListCriteria{
			Regex: aws.String("^test.*"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	allowListID := createOutput.Id

	t.Cleanup(func() {
		_, _ = client.DeleteAllowList(context.Background(), &macie2.DeleteAllowListInput{
			Id: allowListID,
		})
	})

	g := golden.New(t, golden.WithIgnoreFields("Id", "Arn", "ResultMetadata"))
	g.Assert(t.Name()+"/CreateAllowList", createOutput)

	// Get allow list.
	getOutput, err := client.GetAllowList(ctx, &macie2.GetAllowListInput{
		Id: allowListID,
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreatedAt", "UpdatedAt", "ResultMetadata"))
	g2.Assert(t.Name()+"/GetAllowList", getOutput)

	// Delete allow list.
	_, err = client.DeleteAllowList(ctx, &macie2.DeleteAllowListInput{
		Id: allowListID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify allow list is deleted.
	_, err = client.GetAllowList(ctx, &macie2.GetAllowListInput{
		Id: allowListID,
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestMacie2_CreateAndDescribeClassificationJob(t *testing.T) {
	client := newMacie2Client(t)
	ctx := t.Context()

	createOutput, err := client.CreateClassificationJob(ctx, &macie2.CreateClassificationJobInput{
		ClientToken: aws.String("test-token"),
		Name:        aws.String("test-classification-job"),
		JobType:     types.JobTypeOneTime,
		S3JobDefinition: &types.S3JobDefinition{
			BucketDefinitions: []types.S3BucketDefinitionForJob{
				{
					AccountId: aws.String("123456789012"),
					Buckets:   []string{"test-bucket"},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t, golden.WithIgnoreFields("JobId", "JobArn", "ResultMetadata"))
	g.Assert(t.Name()+"/CreateClassificationJob", createOutput)

	// Describe classification job.
	descOutput, err := client.DescribeClassificationJob(ctx, &macie2.DescribeClassificationJobInput{
		JobId: createOutput.JobId,
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t, golden.WithIgnoreFields("JobId", "JobArn", "CreatedAt", "ResultMetadata"))
	g2.Assert(t.Name()+"/DescribeClassificationJob", descOutput)
}

func TestMacie2_CreateAndDeleteCustomDataIdentifier(t *testing.T) {
	client := newMacie2Client(t)
	ctx := t.Context()

	createOutput, err := client.CreateCustomDataIdentifier(ctx, &macie2.CreateCustomDataIdentifierInput{
		ClientToken: aws.String("test-token"),
		Name:        aws.String("test-custom-data-id"),
		Regex:       aws.String("[0-9]{3}-[0-9]{2}-[0-9]{4}"),
		Keywords:    []string{"SSN", "social security"},
	})
	if err != nil {
		t.Fatal(err)
	}

	cdiID := createOutput.CustomDataIdentifierId

	t.Cleanup(func() {
		_, _ = client.DeleteCustomDataIdentifier(context.Background(), &macie2.DeleteCustomDataIdentifierInput{
			Id: cdiID,
		})
	})

	g := golden.New(t, golden.WithIgnoreFields("CustomDataIdentifierId", "ResultMetadata"))
	g.Assert(t.Name()+"/CreateCustomDataIdentifier", createOutput)

	// Get custom data identifier.
	getOutput, err := client.GetCustomDataIdentifier(ctx, &macie2.GetCustomDataIdentifierInput{
		Id: cdiID,
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreatedAt", "ResultMetadata"))
	g2.Assert(t.Name()+"/GetCustomDataIdentifier", getOutput)

	// Delete custom data identifier.
	_, err = client.DeleteCustomDataIdentifier(ctx, &macie2.DeleteCustomDataIdentifierInput{
		Id: cdiID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify custom data identifier is deleted.
	_, err = client.GetCustomDataIdentifier(ctx, &macie2.GetCustomDataIdentifierInput{
		Id: cdiID,
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestMacie2_CreateAndDeleteFindingsFilter(t *testing.T) {
	client := newMacie2Client(t)
	ctx := t.Context()

	createOutput, err := client.CreateFindingsFilter(ctx, &macie2.CreateFindingsFilterInput{
		ClientToken: aws.String("test-token"),
		Name:        aws.String("test-findings-filter"),
		Action:      types.FindingsFilterActionArchive,
		FindingCriteria: &types.FindingCriteria{
			Criterion: map[string]types.CriterionAdditionalProperties{
				"severity.description": {
					Eq: []string{"High"},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	filterID := createOutput.Id

	t.Cleanup(func() {
		_, _ = client.DeleteFindingsFilter(context.Background(), &macie2.DeleteFindingsFilterInput{
			Id: filterID,
		})
	})

	g := golden.New(t, golden.WithIgnoreFields("Id", "Arn", "ResultMetadata"))
	g.Assert(t.Name()+"/CreateFindingsFilter", createOutput)

	// Get findings filter.
	getOutput, err := client.GetFindingsFilter(ctx, &macie2.GetFindingsFilterInput{
		Id: filterID,
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t, golden.WithIgnoreFields("Id", "Arn", "ResultMetadata"))
	g2.Assert(t.Name()+"/GetFindingsFilter", getOutput)

	// Delete findings filter.
	_, err = client.DeleteFindingsFilter(ctx, &macie2.DeleteFindingsFilterInput{
		Id: filterID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify findings filter is deleted.
	_, err = client.GetFindingsFilter(ctx, &macie2.GetFindingsFilterInput{
		Id: filterID,
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestMacie2_AllowListNotFound(t *testing.T) {
	client := newMacie2Client(t)
	ctx := t.Context()

	_, err := client.GetAllowList(ctx, &macie2.GetAllowListInput{
		Id: aws.String("non-existent-id"),
	})
	if err == nil {
		t.Error("expected error")
	}

	_, err = client.DeleteAllowList(ctx, &macie2.DeleteAllowListInput{
		Id: aws.String("non-existent-id"),
	})
	if err == nil {
		t.Error("expected error")
	}
}
