//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/sivchari/golden"
)

func newCostExplorerClient(t *testing.T) *costexplorer.Client {
	t.Helper()

	return costexplorer.NewFromConfig(awsConfig(t), func(o *costexplorer.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestCostExplorer_GetCostAndUsage(t *testing.T) {
	client := newCostExplorerClient(t)
	ctx := t.Context()

	// Get cost and usage for last month
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, -1, 0).Format("2006-01-02")

	output, err := client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{"UnblendedCost", "UsageQuantity"},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "ResultsByTime"),
	)
	g.Assert(t.Name(), output)
}

func TestCostExplorer_GetCostAndUsageGrouped(t *testing.T) {
	client := newCostExplorerClient(t)
	ctx := t.Context()

	// Get cost and usage grouped by service
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, -1, 0).Format("2006-01-02")

	output, err := client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{"BlendedCost"},
		GroupBy: []types.GroupDefinition{
			{
				Type: types.GroupDefinitionTypeDimension,
				Key:  aws.String("SERVICE"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "ResultsByTime"),
	)
	g.Assert(t.Name(), output)
}

func TestCostExplorer_GetDimensionValues(t *testing.T) {
	client := newCostExplorerClient(t)
	ctx := t.Context()

	// Get dimension values for SERVICE
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, -1, 0).Format("2006-01-02")

	output, err := client.GetDimensionValues(ctx, &costexplorer.GetDimensionValuesInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
		Dimension: types.DimensionService,
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert(t.Name(), output)
}

func TestCostExplorer_GetDimensionValuesWithSearch(t *testing.T) {
	client := newCostExplorerClient(t)
	ctx := t.Context()

	// Get dimension values for SERVICE with search
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, -1, 0).Format("2006-01-02")

	output, err := client.GetDimensionValues(ctx, &costexplorer.GetDimensionValuesInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
		Dimension:    types.DimensionService,
		SearchString: aws.String("Elastic"),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert(t.Name(), output)
}

func TestCostExplorer_GetTags(t *testing.T) {
	client := newCostExplorerClient(t)
	ctx := t.Context()

	// Get tags
	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, -1, 0).Format("2006-01-02")

	output, err := client.GetTags(ctx, &costexplorer.GetTagsInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata"),
	)
	g.Assert(t.Name(), output)
}

func TestCostExplorer_GetCostForecast(t *testing.T) {
	client := newCostExplorerClient(t)
	ctx := t.Context()

	// Get cost forecast for next month
	start := time.Now().Format("2006-01-02")
	end := time.Now().AddDate(0, 1, 0).Format("2006-01-02")

	output, err := client.GetCostForecast(ctx, &costexplorer.GetCostForecastInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
		Metric:      types.MetricBlendedCost,
		Granularity: types.GranularityDaily,
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "Total", "ForecastResultsByTime"),
	)
	g.Assert(t.Name(), output)
}

func TestCostExplorer_CostCategoryDefinition(t *testing.T) {
	client := newCostExplorerClient(t)
	ctx := t.Context()

	// Create cost category
	createOutput, err := client.CreateCostCategoryDefinition(ctx, &costexplorer.CreateCostCategoryDefinitionInput{
		Name:        aws.String("test-cost-category"),
		RuleVersion: types.CostCategoryRuleVersionCostCategoryExpressionV1,
		Rules: []types.CostCategoryRule{
			{
				Value: aws.String("Development"),
				Rule: &types.Expression{
					Dimensions: &types.DimensionValues{
						Key:    types.DimensionLinkedAccount,
						Values: []string{"123456789012"},
					},
				},
			},
		},
		DefaultValue: aws.String("Other"),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "CostCategoryArn", "EffectiveStart"),
	)
	g.Assert(t.Name()+"_create", createOutput)

	// Describe cost category
	describeOutput, err := client.DescribeCostCategoryDefinition(ctx, &costexplorer.DescribeCostCategoryDefinitionInput{
		CostCategoryArn: createOutput.CostCategoryArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	g2 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "CostCategoryArn", "EffectiveStart"),
	)
	g2.Assert(t.Name()+"_describe", describeOutput)

	// List cost categories
	listOutput, err := client.ListCostCategoryDefinitions(ctx, &costexplorer.ListCostCategoryDefinitionsInput{})
	if err != nil {
		t.Fatal(err)
	}

	g3 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "CostCategoryReferences"),
	)
	g3.Assert(t.Name()+"_list", listOutput)

	// Delete cost category
	deleteOutput, err := client.DeleteCostCategoryDefinition(ctx, &costexplorer.DeleteCostCategoryDefinitionInput{
		CostCategoryArn: createOutput.CostCategoryArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	g4 := golden.New(t,
		golden.WithIgnoreFields("ResultMetadata", "CostCategoryArn", "EffectiveEnd"),
	)
	g4.Assert(t.Name()+"_delete", deleteOutput)
}
