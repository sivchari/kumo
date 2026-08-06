//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/forecast"
	"github.com/aws/aws-sdk-go-v2/service/forecast/types"
	"github.com/sivchari/golden"
)

func newForecastClient(t *testing.T) *forecast.Client {
	t.Helper()

	return forecast.NewFromConfig(awsConfig(t), func(o *forecast.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestForecast_CreateAndDeleteDataset(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	datasetName := "test-dataset-create-delete"

	// Create dataset.
	createOutput, err := client.CreateDataset(ctx, &forecast.CreateDatasetInput{
		DatasetName: aws.String(datasetName),
		DatasetType: types.DatasetTypeTargetTimeSeries,
		Domain:      types.DomainRetail,
		Schema: &types.Schema{
			Attributes: []types.SchemaAttribute{
				{
					AttributeName: aws.String("item_id"),
					AttributeType: types.AttributeTypeString,
				},
				{
					AttributeName: aws.String("timestamp"),
					AttributeType: types.AttributeTypeTimestamp,
				},
				{
					AttributeName: aws.String("demand"),
					AttributeType: types.AttributeTypeFloat,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetArn := createOutput.DatasetArn

	t.Cleanup(func() {
		_, _ = client.DeleteDataset(context.Background(), &forecast.DeleteDatasetInput{
			DatasetArn: datasetArn,
		})
	})

	// Describe dataset.
	descOutput, err := client.DescribeDataset(ctx, &forecast.DescribeDatasetInput{
		DatasetArn: datasetArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DatasetArn", "CreationTime", "LastModificationTime")).Assert(t.Name()+"/DescribeDataset", descOutput)

	// Delete dataset.
	_, err = client.DeleteDataset(context.Background(), &forecast.DeleteDatasetInput{
		DatasetArn: datasetArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify dataset is deleted.
	_, err = client.DescribeDataset(ctx, &forecast.DescribeDatasetInput{
		DatasetArn: datasetArn,
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestForecast_ListDatasets(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	// Create datasets.
	var datasetArns []*string

	for i := 0; i < 2; i++ {
		createOutput, err := client.CreateDataset(ctx, &forecast.CreateDatasetInput{
			DatasetName: aws.String("test-dataset-list-" + string(rune('a'+i))),
			DatasetType: types.DatasetTypeTargetTimeSeries,
			Domain:      types.DomainRetail,
			Schema: &types.Schema{
				Attributes: []types.SchemaAttribute{
					{
						AttributeName: aws.String("item_id"),
						AttributeType: types.AttributeTypeString,
					},
					{
						AttributeName: aws.String("timestamp"),
						AttributeType: types.AttributeTypeTimestamp,
					},
					{
						AttributeName: aws.String("demand"),
						AttributeType: types.AttributeTypeFloat,
					},
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		datasetArns = append(datasetArns, createOutput.DatasetArn)
	}

	t.Cleanup(func() {
		for _, arn := range datasetArns {
			_, _ = client.DeleteDataset(context.Background(), &forecast.DeleteDatasetInput{
				DatasetArn: arn,
			})
		}
	})

	// List datasets.
	listOutput, err := client.ListDatasets(ctx, &forecast.ListDatasetsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOutput.Datasets) < 2 {
		t.Errorf("expected at least 2 datasets, got %d", len(listOutput.Datasets))
	}
}

func TestForecast_CreateAndDeleteDatasetGroup(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	datasetGroupName := "test-dataset-group"

	// Create dataset group.
	createOutput, err := client.CreateDatasetGroup(ctx, &forecast.CreateDatasetGroupInput{
		DatasetGroupName: aws.String(datasetGroupName),
		Domain:           types.DomainRetail,
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetGroupArn := createOutput.DatasetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteDatasetGroup(context.Background(), &forecast.DeleteDatasetGroupInput{
			DatasetGroupArn: datasetGroupArn,
		})
	})

	// Describe dataset group.
	descOutput, err := client.DescribeDatasetGroup(ctx, &forecast.DescribeDatasetGroupInput{
		DatasetGroupArn: datasetGroupArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DatasetGroupArn", "CreationTime", "LastModificationTime")).Assert(t.Name()+"/DescribeDatasetGroup", descOutput)

	// Delete dataset group.
	_, err = client.DeleteDatasetGroup(context.Background(), &forecast.DeleteDatasetGroupInput{
		DatasetGroupArn: datasetGroupArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify dataset group is deleted.
	_, err = client.DescribeDatasetGroup(ctx, &forecast.DescribeDatasetGroupInput{
		DatasetGroupArn: datasetGroupArn,
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestForecast_UpdateDatasetGroup(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	// Create a dataset.
	datasetOutput, err := client.CreateDataset(ctx, &forecast.CreateDatasetInput{
		DatasetName: aws.String("test-dataset-for-update-group"),
		DatasetType: types.DatasetTypeTargetTimeSeries,
		Domain:      types.DomainRetail,
		Schema: &types.Schema{
			Attributes: []types.SchemaAttribute{
				{
					AttributeName: aws.String("item_id"),
					AttributeType: types.AttributeTypeString,
				},
				{
					AttributeName: aws.String("timestamp"),
					AttributeType: types.AttributeTypeTimestamp,
				},
				{
					AttributeName: aws.String("demand"),
					AttributeType: types.AttributeTypeFloat,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetArn := datasetOutput.DatasetArn

	t.Cleanup(func() {
		_, _ = client.DeleteDataset(context.Background(), &forecast.DeleteDatasetInput{
			DatasetArn: datasetArn,
		})
	})

	// Create a dataset group.
	dgOutput, err := client.CreateDatasetGroup(ctx, &forecast.CreateDatasetGroupInput{
		DatasetGroupName: aws.String("test-dataset-group-update"),
		Domain:           types.DomainRetail,
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetGroupArn := dgOutput.DatasetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteDatasetGroup(context.Background(), &forecast.DeleteDatasetGroupInput{
			DatasetGroupArn: datasetGroupArn,
		})
	})

	// Update dataset group with dataset.
	_, err = client.UpdateDatasetGroup(ctx, &forecast.UpdateDatasetGroupInput{
		DatasetGroupArn: datasetGroupArn,
		DatasetArns:     []string{*datasetArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify the update.
	descOutput, err := client.DescribeDatasetGroup(ctx, &forecast.DescribeDatasetGroupInput{
		DatasetGroupArn: datasetGroupArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("DatasetGroupArn", "DatasetArns", "CreationTime", "LastModificationTime")).Assert(t.Name()+"/DescribeDatasetGroup", descOutput)
	if len(descOutput.DatasetArns) != 1 {
		t.Errorf("expected 1 dataset ARN, got %d", len(descOutput.DatasetArns))
	}
}

func TestForecast_CreatePredictor(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	// Create a dataset.
	datasetOutput, err := client.CreateDataset(ctx, &forecast.CreateDatasetInput{
		DatasetName: aws.String("test-dataset-for-predictor"),
		DatasetType: types.DatasetTypeTargetTimeSeries,
		Domain:      types.DomainRetail,
		Schema: &types.Schema{
			Attributes: []types.SchemaAttribute{
				{
					AttributeName: aws.String("item_id"),
					AttributeType: types.AttributeTypeString,
				},
				{
					AttributeName: aws.String("timestamp"),
					AttributeType: types.AttributeTypeTimestamp,
				},
				{
					AttributeName: aws.String("demand"),
					AttributeType: types.AttributeTypeFloat,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetArn := datasetOutput.DatasetArn

	t.Cleanup(func() {
		_, _ = client.DeleteDataset(context.Background(), &forecast.DeleteDatasetInput{
			DatasetArn: datasetArn,
		})
	})

	// Create a dataset group with the dataset.
	dgOutput, err := client.CreateDatasetGroup(ctx, &forecast.CreateDatasetGroupInput{
		DatasetGroupName: aws.String("test-dataset-group-for-predictor"),
		Domain:           types.DomainRetail,
		DatasetArns:      []string{*datasetArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetGroupArn := dgOutput.DatasetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteDatasetGroup(context.Background(), &forecast.DeleteDatasetGroupInput{
			DatasetGroupArn: datasetGroupArn,
		})
	})

	// Create a predictor.
	predictorOutput, err := client.CreatePredictor(ctx, &forecast.CreatePredictorInput{
		PredictorName:   aws.String("test-predictor"),
		ForecastHorizon: aws.Int32(30),
		InputDataConfig: &types.InputDataConfig{
			DatasetGroupArn: datasetGroupArn,
		},
		FeaturizationConfig: &types.FeaturizationConfig{
			ForecastFrequency: aws.String("D"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	predictorArn := predictorOutput.PredictorArn

	t.Cleanup(func() {
		_, _ = client.DeletePredictor(context.Background(), &forecast.DeletePredictorInput{
			PredictorArn: predictorArn,
		})
	})

	// Describe predictor.
	descOutput, err := client.DescribePredictor(ctx, &forecast.DescribePredictorInput{
		PredictorArn: predictorArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("PredictorArn", "DatasetImportJobArns", "CreationTime", "LastModificationTime", "InputDataConfig")).Assert(t.Name()+"/DescribePredictor", descOutput)

	// Delete predictor.
	_, err = client.DeletePredictor(context.Background(), &forecast.DeletePredictorInput{
		PredictorArn: predictorArn,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestForecast_CreateForecast(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	// Create a dataset.
	datasetOutput, err := client.CreateDataset(ctx, &forecast.CreateDatasetInput{
		DatasetName: aws.String("test-dataset-for-forecast"),
		DatasetType: types.DatasetTypeTargetTimeSeries,
		Domain:      types.DomainRetail,
		Schema: &types.Schema{
			Attributes: []types.SchemaAttribute{
				{
					AttributeName: aws.String("item_id"),
					AttributeType: types.AttributeTypeString,
				},
				{
					AttributeName: aws.String("timestamp"),
					AttributeType: types.AttributeTypeTimestamp,
				},
				{
					AttributeName: aws.String("demand"),
					AttributeType: types.AttributeTypeFloat,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetArn := datasetOutput.DatasetArn

	t.Cleanup(func() {
		_, _ = client.DeleteDataset(context.Background(), &forecast.DeleteDatasetInput{
			DatasetArn: datasetArn,
		})
	})

	// Create a dataset group.
	dgOutput, err := client.CreateDatasetGroup(ctx, &forecast.CreateDatasetGroupInput{
		DatasetGroupName: aws.String("test-dataset-group-for-forecast"),
		Domain:           types.DomainRetail,
		DatasetArns:      []string{*datasetArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetGroupArn := dgOutput.DatasetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteDatasetGroup(context.Background(), &forecast.DeleteDatasetGroupInput{
			DatasetGroupArn: datasetGroupArn,
		})
	})

	// Create a predictor.
	predictorOutput, err := client.CreatePredictor(ctx, &forecast.CreatePredictorInput{
		PredictorName:   aws.String("test-predictor-for-forecast"),
		ForecastHorizon: aws.Int32(30),
		InputDataConfig: &types.InputDataConfig{
			DatasetGroupArn: datasetGroupArn,
		},
		FeaturizationConfig: &types.FeaturizationConfig{
			ForecastFrequency: aws.String("D"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	predictorArn := predictorOutput.PredictorArn

	t.Cleanup(func() {
		_, _ = client.DeletePredictor(context.Background(), &forecast.DeletePredictorInput{
			PredictorArn: predictorArn,
		})
	})

	// Create a forecast.
	forecastOutput, err := client.CreateForecast(ctx, &forecast.CreateForecastInput{
		ForecastName: aws.String("test-forecast"),
		PredictorArn: predictorArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	forecastArn := forecastOutput.ForecastArn

	t.Cleanup(func() {
		_, _ = client.DeleteForecast(context.Background(), &forecast.DeleteForecastInput{
			ForecastArn: forecastArn,
		})
	})

	// Describe forecast.
	descOutput, err := client.DescribeForecast(ctx, &forecast.DescribeForecastInput{
		ForecastArn: forecastArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ForecastArn", "PredictorArn", "DatasetGroupArn", "CreationTime", "LastModificationTime")).Assert(t.Name()+"/DescribeForecast", descOutput)

	// Delete forecast.
	_, err = client.DeleteForecast(context.Background(), &forecast.DeleteForecastInput{
		ForecastArn: forecastArn,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestForecast_DatasetNotFound(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	// Describe non-existent dataset.
	_, err := client.DescribeDataset(ctx, &forecast.DescribeDatasetInput{
		DatasetArn: aws.String("arn:aws:forecast:us-east-1:123456789012:dataset/non-existent"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent dataset.
	_, err = client.DeleteDataset(context.Background(), &forecast.DeleteDatasetInput{
		DatasetArn: aws.String("arn:aws:forecast:us-east-1:123456789012:dataset/non-existent"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestForecast_DatasetGroupNotFound(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	// Describe non-existent dataset group.
	_, err := client.DescribeDatasetGroup(ctx, &forecast.DescribeDatasetGroupInput{
		DatasetGroupArn: aws.String("arn:aws:forecast:us-east-1:123456789012:dataset-group/non-existent"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent dataset group.
	_, err = client.DeleteDatasetGroup(context.Background(), &forecast.DeleteDatasetGroupInput{
		DatasetGroupArn: aws.String("arn:aws:forecast:us-east-1:123456789012:dataset-group/non-existent"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestForecast_PredictorNotFound(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	// Describe non-existent predictor.
	_, err := client.DescribePredictor(ctx, &forecast.DescribePredictorInput{
		PredictorArn: aws.String("arn:aws:forecast:us-east-1:123456789012:predictor/non-existent"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent predictor.
	_, err = client.DeletePredictor(context.Background(), &forecast.DeletePredictorInput{
		PredictorArn: aws.String("arn:aws:forecast:us-east-1:123456789012:predictor/non-existent"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestForecast_ForecastNotFound(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	// Describe non-existent forecast.
	_, err := client.DescribeForecast(ctx, &forecast.DescribeForecastInput{
		ForecastArn: aws.String("arn:aws:forecast:us-east-1:123456789012:forecast/non-existent"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent forecast.
	_, err = client.DeleteForecast(context.Background(), &forecast.DeleteForecastInput{
		ForecastArn: aws.String("arn:aws:forecast:us-east-1:123456789012:forecast/non-existent"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestForecast_DuplicateDatasetName(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	datasetName := "test-dataset-duplicate"

	// Create first dataset.
	createOutput, err := client.CreateDataset(ctx, &forecast.CreateDatasetInput{
		DatasetName: aws.String(datasetName),
		DatasetType: types.DatasetTypeTargetTimeSeries,
		Domain:      types.DomainRetail,
		Schema: &types.Schema{
			Attributes: []types.SchemaAttribute{
				{
					AttributeName: aws.String("item_id"),
					AttributeType: types.AttributeTypeString,
				},
				{
					AttributeName: aws.String("timestamp"),
					AttributeType: types.AttributeTypeTimestamp,
				},
				{
					AttributeName: aws.String("demand"),
					AttributeType: types.AttributeTypeFloat,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetArn := createOutput.DatasetArn

	t.Cleanup(func() {
		_, _ = client.DeleteDataset(context.Background(), &forecast.DeleteDatasetInput{
			DatasetArn: datasetArn,
		})
	})

	// Try to create a dataset with the same name.
	_, err = client.CreateDataset(ctx, &forecast.CreateDatasetInput{
		DatasetName: aws.String(datasetName),
		DatasetType: types.DatasetTypeTargetTimeSeries,
		Domain:      types.DomainRetail,
		Schema: &types.Schema{
			Attributes: []types.SchemaAttribute{
				{
					AttributeName: aws.String("item_id"),
					AttributeType: types.AttributeTypeString,
				},
				{
					AttributeName: aws.String("timestamp"),
					AttributeType: types.AttributeTypeTimestamp,
				},
				{
					AttributeName: aws.String("demand"),
					AttributeType: types.AttributeTypeFloat,
				},
			},
		},
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestForecast_ResourceInUse(t *testing.T) {
	client := newForecastClient(t)
	ctx := t.Context()

	// Create a dataset.
	datasetOutput, err := client.CreateDataset(ctx, &forecast.CreateDatasetInput{
		DatasetName: aws.String("test-dataset-in-use"),
		DatasetType: types.DatasetTypeTargetTimeSeries,
		Domain:      types.DomainRetail,
		Schema: &types.Schema{
			Attributes: []types.SchemaAttribute{
				{
					AttributeName: aws.String("item_id"),
					AttributeType: types.AttributeTypeString,
				},
				{
					AttributeName: aws.String("timestamp"),
					AttributeType: types.AttributeTypeTimestamp,
				},
				{
					AttributeName: aws.String("demand"),
					AttributeType: types.AttributeTypeFloat,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetArn := datasetOutput.DatasetArn

	// Create a dataset group that uses the dataset.
	dgOutput, err := client.CreateDatasetGroup(ctx, &forecast.CreateDatasetGroupInput{
		DatasetGroupName: aws.String("test-dataset-group-in-use"),
		Domain:           types.DomainRetail,
		DatasetArns:      []string{*datasetArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	datasetGroupArn := dgOutput.DatasetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteDatasetGroup(context.Background(), &forecast.DeleteDatasetGroupInput{
			DatasetGroupArn: datasetGroupArn,
		})
		_, _ = client.DeleteDataset(context.Background(), &forecast.DeleteDatasetInput{
			DatasetArn: datasetArn,
		})
	})

	// Try to delete the dataset (should fail because it's in use).
	_, err = client.DeleteDataset(context.Background(), &forecast.DeleteDatasetInput{
		DatasetArn: datasetArn,
	})
	if err == nil {
		t.Error("expected error")
	}
}
