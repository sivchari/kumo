//go:build integration

package integration

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dataexchange"
	"github.com/aws/aws-sdk-go-v2/service/dataexchange/types"
	"github.com/sivchari/golden"
)

func newDataExchangeClient(t *testing.T) *dataexchange.Client {
	t.Helper()

	return dataexchange.NewFromConfig(awsConfig(t), func(o *dataexchange.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestDataExchange_CreateDataSet(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	result, err := client.CreateDataSet(ctx, &dataexchange.CreateDataSetInput{
		Name:        aws.String("test-dataset"),
		Description: aws.String("test description"),
		AssetType:   types.AssetTypeS3Snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestDataExchange_GetDataSet(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	createResult, err := client.CreateDataSet(ctx, &dataexchange.CreateDataSetInput{
		Name:        aws.String("get-dataset"),
		Description: aws.String("get test"),
		AssetType:   types.AssetTypeS3Snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.GetDataSet(ctx, &dataexchange.GetDataSetInput{
		DataSetId: createResult.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestDataExchange_ListDataSets(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	_, err := client.CreateDataSet(ctx, &dataexchange.CreateDataSetInput{
		Name:        aws.String("list-dataset"),
		Description: aws.String("list test"),
		AssetType:   types.AssetTypeS3Snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListDataSets(ctx, &dataexchange.ListDataSetsInput{})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestDataExchange_UpdateDataSet(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	createResult, err := client.CreateDataSet(ctx, &dataexchange.CreateDataSetInput{
		Name:        aws.String("update-dataset"),
		Description: aws.String("original"),
		AssetType:   types.AssetTypeS3Snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.UpdateDataSet(ctx, &dataexchange.UpdateDataSetInput{
		DataSetId:   createResult.Id,
		Description: aws.String("updated"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestDataExchange_DeleteDataSet(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	createResult, err := client.CreateDataSet(ctx, &dataexchange.CreateDataSetInput{
		Name:        aws.String("delete-dataset"),
		Description: aws.String("delete test"),
		AssetType:   types.AssetTypeS3Snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteDataSet(ctx, &dataexchange.DeleteDataSetInput{
		DataSetId: createResult.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetDataSet(ctx, &dataexchange.GetDataSetInput{
		DataSetId: createResult.Id,
	})
	if err == nil {
		t.Fatal("expected error for deleted data set")
	}
}

func TestDataExchange_CreateRevision(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	dsResult, err := client.CreateDataSet(ctx, &dataexchange.CreateDataSetInput{
		Name:        aws.String("revision-dataset"),
		Description: aws.String("revision test"),
		AssetType:   types.AssetTypeS3Snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.CreateRevision(ctx, &dataexchange.CreateRevisionInput{
		DataSetId: dsResult.Id,
		Comment:   aws.String("initial revision"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "DataSetId", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestDataExchange_GetRevision(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	dsResult, err := client.CreateDataSet(ctx, &dataexchange.CreateDataSetInput{
		Name:        aws.String("get-revision-dataset"),
		Description: aws.String("get revision test"),
		AssetType:   types.AssetTypeS3Snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}

	revResult, err := client.CreateRevision(ctx, &dataexchange.CreateRevisionInput{
		DataSetId: dsResult.Id,
		Comment:   aws.String("get revision"),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.GetRevision(ctx, &dataexchange.GetRevisionInput{
		DataSetId:  dsResult.Id,
		RevisionId: revResult.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "DataSetId", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestDataExchange_ListRevisions(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	dsResult, err := client.CreateDataSet(ctx, &dataexchange.CreateDataSetInput{
		Name:        aws.String("list-revision-dataset"),
		Description: aws.String("list revision test"),
		AssetType:   types.AssetTypeS3Snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.CreateRevision(ctx, &dataexchange.CreateRevisionInput{
		DataSetId: dsResult.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListDataSetRevisions(ctx, &dataexchange.ListDataSetRevisionsInput{
		DataSetId: dsResult.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "DataSetId", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestDataExchange_DeleteRevision(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	dsResult, err := client.CreateDataSet(ctx, &dataexchange.CreateDataSetInput{
		Name:        aws.String("delete-revision-dataset"),
		Description: aws.String("delete revision test"),
		AssetType:   types.AssetTypeS3Snapshot,
	})
	if err != nil {
		t.Fatal(err)
	}

	revResult, err := client.CreateRevision(ctx, &dataexchange.CreateRevisionInput{
		DataSetId: dsResult.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.DeleteRevision(ctx, &dataexchange.DeleteRevisionInput{
		DataSetId:  dsResult.Id,
		RevisionId: revResult.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.GetRevision(ctx, &dataexchange.GetRevisionInput{
		DataSetId:  dsResult.Id,
		RevisionId: revResult.Id,
	})
	if err == nil {
		t.Fatal("expected error for deleted revision")
	}
}

func TestDataExchange_CreateJob(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	result, err := client.CreateJob(ctx, &dataexchange.CreateJobInput{
		Type: types.TypeImportAssetsFromS3,
		Details: &types.RequestDetails{
			ImportAssetsFromS3: &types.ImportAssetsFromS3RequestDetails{
				DataSetId:  aws.String("test-dataset-id"),
				RevisionId: aws.String("test-revision-id"),
				AssetSources: []types.AssetSourceEntry{
					{
						Bucket: aws.String("source-bucket"),
						Key:    aws.String("source-key"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestDataExchange_GetJob(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	createResult, err := client.CreateJob(ctx, &dataexchange.CreateJobInput{
		Type: types.TypeImportAssetsFromS3,
		Details: &types.RequestDetails{
			ImportAssetsFromS3: &types.ImportAssetsFromS3RequestDetails{
				DataSetId:  aws.String("test-dataset-id"),
				RevisionId: aws.String("test-revision-id"),
				AssetSources: []types.AssetSourceEntry{
					{
						Bucket: aws.String("source-bucket"),
						Key:    aws.String("source-key"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.GetJob(ctx, &dataexchange.GetJobInput{
		JobId: createResult.Id,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestDataExchange_ListJobs(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	_, err := client.CreateJob(ctx, &dataexchange.CreateJobInput{
		Type: types.TypeImportAssetsFromS3,
		Details: &types.RequestDetails{
			ImportAssetsFromS3: &types.ImportAssetsFromS3RequestDetails{
				DataSetId:  aws.String("test-dataset-id"),
				RevisionId: aws.String("test-revision-id"),
				AssetSources: []types.AssetSourceEntry{
					{
						Bucket: aws.String("source-bucket"),
						Key:    aws.String("source-key"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.ListJobs(ctx, &dataexchange.ListJobsInput{})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Id", "Arn", "CreatedAt", "UpdatedAt", "ResultMetadata")).Assert(t.Name(), result)
}

func TestDataExchange_DataSetNotFound(t *testing.T) {
	client := newDataExchangeClient(t)
	ctx := t.Context()

	_, err := client.GetDataSet(ctx, &dataexchange.GetDataSetInput{
		DataSetId: aws.String("nonexistent"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent data set")
	}
}
