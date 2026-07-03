//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/sivchari/golden"
)

func newCloudWatchClient(t *testing.T) *cloudwatch.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return cloudwatch.NewFromConfig(cfg, func(o *cloudwatch.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestCloudWatch_PutMetricData(t *testing.T) {
	client := newCloudWatchClient(t)
	ctx := t.Context()

	// Put metric data.
	_, err := client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String("TestNamespace"),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String("TestMetric"),
				Value:      aws.Float64(100.0),
				Unit:       types.StandardUnitCount,
				Dimensions: []types.Dimension{
					{
						Name:  aws.String("Environment"),
						Value: aws.String("Test"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCloudWatch_ListMetrics(t *testing.T) {
	client := newCloudWatchClient(t)
	ctx := t.Context()

	namespace := "TestListMetrics"
	metricName := "ListTestMetric"

	// Put metric data first.
	_, err := client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String(namespace),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(metricName),
				Value:      aws.Float64(42.0),
				Unit:       types.StandardUnitCount,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// List metrics.
	output, err := client.ListMetrics(ctx, &cloudwatch.ListMetricsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, m := range output.Metrics {
		if *m.Namespace == namespace && *m.MetricName == metricName {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("metric %s/%s not found in list", namespace, metricName)
	}
}

func TestCloudWatch_GetMetricStatistics(t *testing.T) {
	client := newCloudWatchClient(t)
	ctx := t.Context()

	namespace := "TestGetStats"
	metricName := "StatsTestMetric"

	// Put some metric data.
	_, err := client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String(namespace),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(metricName),
				Value:      aws.Float64(10.0),
				Unit:       types.StandardUnitCount,
			},
			{
				MetricName: aws.String(metricName),
				Value:      aws.Float64(20.0),
				Unit:       types.StandardUnitCount,
			},
			{
				MetricName: aws.String(metricName),
				Value:      aws.Float64(30.0),
				Unit:       types.StandardUnitCount,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get metric statistics.
	now := time.Now()
	output, err := client.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		StartTime:  aws.Time(now.Add(-1 * time.Hour)),
		EndTime:    aws.Time(now.Add(1 * time.Hour)),
		Period:     aws.Int32(60),
		Statistics: []types.Statistic{
			types.StatisticSum,
			types.StatisticAverage,
			types.StatisticMinimum,
			types.StatisticMaximum,
			types.StatisticSampleCount,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Timestamp", "ResultMetadata")).Assert(t.Name(), output)
}

func TestCloudWatch_PutMetricAlarm(t *testing.T) {
	client := newCloudWatchClient(t)
	ctx := t.Context()

	alarmName := "test-alarm"

	// Put metric alarm.
	_, err := client.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(alarmName),
		MetricName:         aws.String("TestMetric"),
		Namespace:          aws.String("TestNamespace"),
		Statistic:          types.StatisticAverage,
		Period:             aws.Int32(60),
		EvaluationPeriods:  aws.Int32(1),
		Threshold:          aws.Float64(80.0),
		ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
		ActionsEnabled:     aws.Bool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteAlarms(context.Background(), &cloudwatch.DeleteAlarmsInput{
			AlarmNames: []string{alarmName},
		})
	})
}

func TestCloudWatch_DescribeAlarms(t *testing.T) {
	client := newCloudWatchClient(t)
	ctx := t.Context()

	alarmName := "test-describe-alarm"

	// Put metric alarm.
	_, err := client.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(alarmName),
		MetricName:         aws.String("DescribeTestMetric"),
		Namespace:          aws.String("TestNamespace"),
		Statistic:          types.StatisticAverage,
		Period:             aws.Int32(60),
		EvaluationPeriods:  aws.Int32(1),
		Threshold:          aws.Float64(90.0),
		ComparisonOperator: types.ComparisonOperatorGreaterThanOrEqualToThreshold,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteAlarms(context.Background(), &cloudwatch.DeleteAlarmsInput{
			AlarmNames: []string{alarmName},
		})
	})

	// Describe alarms.
	output, err := client.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{
		AlarmNames: []string{alarmName},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("AlarmArn", "AlarmConfigurationUpdatedTimestamp", "StateTransitionedTimestamp", "StateUpdatedTimestamp", "ResultMetadata")).Assert(t.Name(), output)
}

func TestCloudWatch_DeleteAlarms(t *testing.T) {
	client := newCloudWatchClient(t)
	ctx := t.Context()

	alarmName := "test-delete-alarm"

	// Put metric alarm.
	_, err := client.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(alarmName),
		MetricName:         aws.String("DeleteTestMetric"),
		Namespace:          aws.String("TestNamespace"),
		Statistic:          types.StatisticAverage,
		Period:             aws.Int32(60),
		EvaluationPeriods:  aws.Int32(1),
		Threshold:          aws.Float64(50.0),
		ComparisonOperator: types.ComparisonOperatorLessThanThreshold,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete alarm.
	_, err = client.DeleteAlarms(ctx, &cloudwatch.DeleteAlarmsInput{
		AlarmNames: []string{alarmName},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify alarm is deleted.
	output, err := client.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{
		AlarmNames: []string{alarmName},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(output.MetricAlarms) != 0 {
		t.Errorf("expected alarm to be deleted, but found %d alarms", len(output.MetricAlarms))
	}
}

func TestCloudWatch_GetMetricData(t *testing.T) {
	client := newCloudWatchClient(t)
	ctx := t.Context()

	namespace := "TestGetMetricData"
	metricName := "DataTestMetric"

	// Put metric data.
	_, err := client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String(namespace),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(metricName),
				Value:      aws.Float64(100.0),
				Unit:       types.StandardUnitCount,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get metric data.
	now := time.Now()
	output, err := client.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
		StartTime: aws.Time(now.Add(-1 * time.Hour)),
		EndTime:   aws.Time(now.Add(1 * time.Hour)),
		MetricDataQueries: []types.MetricDataQuery{
			{
				Id: aws.String("m1"),
				MetricStat: &types.MetricStat{
					Metric: &types.Metric{
						Namespace:  aws.String(namespace),
						MetricName: aws.String(metricName),
					},
					Period: aws.Int32(60),
					Stat:   aws.String("Average"),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Timestamps", "ResultMetadata")).Assert(t.Name(), output)
}

func TestCloudWatch_PutMetricDataWithDimensions(t *testing.T) {
	client := newCloudWatchClient(t)
	ctx := t.Context()

	namespace := "TestDimensions"
	metricName := "DimensionMetric"

	// Put metric data with dimensions.
	_, err := client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: aws.String(namespace),
		MetricData: []types.MetricDatum{
			{
				MetricName: aws.String(metricName),
				Value:      aws.Float64(50.0),
				Unit:       types.StandardUnitPercent,
				Dimensions: []types.Dimension{
					{
						Name:  aws.String("InstanceId"),
						Value: aws.String("i-12345"),
					},
					{
						Name:  aws.String("InstanceType"),
						Value: aws.String("t2.micro"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// List metrics with dimension filter.
	output, err := client.ListMetrics(ctx, &cloudwatch.ListMetricsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		Dimensions: []types.DimensionFilter{
			{
				Name:  aws.String("InstanceId"),
				Value: aws.String("i-12345"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(output.Metrics) == 0 {
		t.Fatal("expected at least one metric with matching dimensions, got none")
	}
}

func TestCloudWatch_SetAlarmState(t *testing.T) {
	client := newCloudWatchClient(t)
	ctx := t.Context()
	alarmName := "test-set-alarm-state"

	// Create alarm.
	_, err := client.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(alarmName),
		MetricName:         aws.String("TestMetric"),
		Namespace:          aws.String("TestNamespace"),
		Statistic:          types.StatisticAverage,
		Period:             aws.Int32(60),
		EvaluationPeriods:  aws.Int32(1),
		Threshold:          aws.Float64(50.0),
		ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteAlarms(context.Background(), &cloudwatch.DeleteAlarmsInput{
			AlarmNames: []string{alarmName},
		})
	})

	// SetAlarmState.
	_, err = client.SetAlarmState(ctx, &cloudwatch.SetAlarmStateInput{
		AlarmName:   aws.String(alarmName),
		StateValue:  types.StateValueAlarm,
		StateReason: aws.String("Test alarm"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// DescribeAlarms should reflect the new state.
	descOutput, err := client.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{
		AlarmNames: []string{alarmName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields(
		"AlarmArn", "StateUpdatedTimestamp", "AlarmConfigurationUpdatedTimestamp", "ResultMetadata",
	)).Assert(t.Name(), descOutput)
}

func TestCloudWatch_TagOperations(t *testing.T) {
	client := newCloudWatchClient(t)
	ctx := t.Context()

	alarmName := "test-tag-operations-alarm"

	// Create an alarm to tag.
	_, err := client.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(alarmName),
		MetricName:         aws.String("TagTestMetric"),
		Namespace:          aws.String("TestNamespace"),
		Statistic:          types.StatisticAverage,
		Period:             aws.Int32(60),
		EvaluationPeriods:  aws.Int32(1),
		Threshold:          aws.Float64(70.0),
		ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteAlarms(context.Background(), &cloudwatch.DeleteAlarmsInput{
			AlarmNames: []string{alarmName},
		})
	})

	// Get the alarm ARN from DescribeAlarms.
	descOutput, err := client.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{
		AlarmNames: []string{alarmName},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(descOutput.MetricAlarms) == 0 {
		t.Fatal("expected alarm to exist after PutMetricAlarm")
	}

	alarmARN := *descOutput.MetricAlarms[0].AlarmArn

	// TagResource — attach two tags.
	_, err = client.TagResource(ctx, &cloudwatch.TagResourceInput{
		ResourceARN: aws.String(alarmARN),
		Tags: []types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("Production")},
			{Key: aws.String("Team"), Value: aws.String("Platform")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ListTagsForResource — verify both tags are returned.
	listOutput, err := client.ListTagsForResource(ctx, &cloudwatch.ListTagsForResourceInput{
		ResourceARN: aws.String(alarmARN),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_after_tag", listOutput)

	// UntagResource — remove one tag.
	_, err = client.UntagResource(ctx, &cloudwatch.UntagResourceInput{
		ResourceARN: aws.String(alarmARN),
		TagKeys:     []string{"Team"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ListTagsForResource — verify only the remaining tag is returned.
	listOutput2, err := client.ListTagsForResource(ctx, &cloudwatch.ListTagsForResourceInput{
		ResourceARN: aws.String(alarmARN),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_after_untag", listOutput2)
}
