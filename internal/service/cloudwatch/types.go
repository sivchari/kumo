// Package cloudwatch provides CloudWatch metrics service emulation for kumo.
package cloudwatch

import (
	"time"
)

// CBORTime wraps time.Time for CBOR serialization.
// CBOR uses Tag 1 for timestamps (epoch time).
type CBORTime struct {
	time.Time
}

// ToRFC3339 returns the time as an RFC3339 string.
func (t CBORTime) ToRFC3339() string {
	return t.Format(time.RFC3339)
}

// Metric represents a CloudWatch metric.
type Metric struct {
	Namespace  string      `json:"Namespace"`
	MetricName string      `json:"MetricName"`
	Dimensions []Dimension `json:"Dimensions,omitempty"`
}

// Dimension represents a CloudWatch dimension.
type Dimension struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

// MetricDatum represents a single metric data point.
type MetricDatum struct {
	MetricName        string        `json:"MetricName"`
	Dimensions        []Dimension   `json:"Dimensions,omitempty"`
	Timestamp         string        `json:"Timestamp,omitempty"`
	Value             *float64      `json:"Value,omitempty"`
	StatisticValues   *StatisticSet `json:"StatisticValues,omitempty"`
	Values            []float64     `json:"Values,omitempty"`
	Counts            []float64     `json:"Counts,omitempty"`
	Unit              string        `json:"Unit,omitempty"`
	StorageResolution *int32        `json:"StorageResolution,omitempty"`
}

// StatisticSet represents a set of statistics.
type StatisticSet struct {
	SampleCount float64 `json:"SampleCount"`
	Sum         float64 `json:"Sum"`
	Minimum     float64 `json:"Minimum"`
	Maximum     float64 `json:"Maximum"`
}

// MetricDatapoint represents a metric data point in storage.
type MetricDatapoint struct {
	Timestamp string
	Value     float64
	Unit      string
}

// Alarm represents a CloudWatch alarm.
type Alarm struct {
	AlarmName          string
	AlarmARN           string
	AlarmDescription   string
	MetricName         string
	Namespace          string
	Statistic          string
	Dimensions         []Dimension
	Period             int32
	EvaluationPeriods  int32
	Threshold          float64
	ComparisonOperator string
	ActionsEnabled     bool
	AlarmActions       []string
	OKActions          []string
	StateValue         string
	StateReason        string
	StateUpdatedAt     string
	CreatedAt          string
}

// PutMetricDataRequest is the request for PutMetricData.
type PutMetricDataRequest struct {
	Namespace  string        `json:"Namespace"`
	MetricData []MetricDatum `json:"MetricData"`
}

// GetMetricDataRequest is the request for GetMetricData.
type GetMetricDataRequest struct {
	MetricDataQueries []MetricDataQuery `json:"MetricDataQueries"`
	StartTime         string            `json:"StartTime"`
	EndTime           string            `json:"EndTime"`
	NextToken         string            `json:"NextToken,omitempty"`
	MaxDatapoints     *int32            `json:"MaxDatapoints,omitempty"`
}

// MetricDataQuery represents a metric data query.
type MetricDataQuery struct {
	ID         string      `json:"Id"`
	MetricStat *MetricStat `json:"MetricStat,omitempty"`
	Expression string      `json:"Expression,omitempty"`
	Label      string      `json:"Label,omitempty"`
	ReturnData *bool       `json:"ReturnData,omitempty"`
	Period     *int32      `json:"Period,omitempty"`
}

// MetricStat defines the metric and stat to return.
type MetricStat struct {
	Metric Metric `json:"Metric"`
	Period int32  `json:"Period"`
	Stat   string `json:"Stat"`
	Unit   string `json:"Unit,omitempty"`
}

// GetMetricStatisticsRequest is the request for GetMetricStatistics.
type GetMetricStatisticsRequest struct {
	Namespace  string      `json:"Namespace"`
	MetricName string      `json:"MetricName"`
	Dimensions []Dimension `json:"Dimensions,omitempty"`
	StartTime  string      `json:"StartTime"`
	EndTime    string      `json:"EndTime"`
	Period     int32       `json:"Period"`
	Statistics []string    `json:"Statistics,omitempty"`
	Unit       string      `json:"Unit,omitempty"`
}

// ListMetricsRequest is the request for ListMetrics.
type ListMetricsRequest struct {
	Namespace          string            `json:"Namespace,omitempty"`
	MetricName         string            `json:"MetricName,omitempty"`
	Dimensions         []DimensionFilter `json:"Dimensions,omitempty"`
	NextToken          string            `json:"NextToken,omitempty"`
	RecentlyActive     string            `json:"RecentlyActive,omitempty"`
	IncludeLinkedAccts *bool             `json:"IncludeLinkedAccounts,omitempty"`
	OwnAcctOnly        *bool             `json:"OwningAccount,omitempty"`
}

// DimensionFilter is used for filtering metrics by dimension.
type DimensionFilter struct {
	Name  string `json:"Name"`
	Value string `json:"Value,omitempty"`
}

// PutMetricAlarmRequest is the request for PutMetricAlarm.
type PutMetricAlarmRequest struct {
	AlarmName          string      `json:"AlarmName"`
	AlarmDescription   string      `json:"AlarmDescription,omitempty"`
	MetricName         string      `json:"MetricName"`
	Namespace          string      `json:"Namespace"`
	Statistic          string      `json:"Statistic,omitempty"`
	Dimensions         []Dimension `json:"Dimensions,omitempty"`
	Period             int32       `json:"Period"`
	EvaluationPeriods  int32       `json:"EvaluationPeriods"`
	Threshold          float64     `json:"Threshold"`
	ComparisonOperator string      `json:"ComparisonOperator"`
	ActionsEnabled     *bool       `json:"ActionsEnabled,omitempty"`
	AlarmActions       []string    `json:"AlarmActions,omitempty"`
	OKActions          []string    `json:"OKActions,omitempty"`
}

// SetAlarmStateRequest is the request for SetAlarmState.
type SetAlarmStateRequest struct {
	AlarmName   string `json:"AlarmName"`
	StateValue  string `json:"StateValue"`
	StateReason string `json:"StateReason"`
}

// SetAlarmStateCBORRequest is the CBOR request for SetAlarmState.
type SetAlarmStateCBORRequest struct {
	AlarmName   string `cbor:"AlarmName"`
	StateValue  string `cbor:"StateValue"`
	StateReason string `cbor:"StateReason"`
}

// DeleteAlarmsRequest is the request for DeleteAlarms.
type DeleteAlarmsRequest struct {
	AlarmNames []string `json:"AlarmNames"`
}

// DescribeAlarmsRequest is the request for DescribeAlarms.
type DescribeAlarmsRequest struct {
	AlarmNames      []string `json:"AlarmNames,omitempty"`
	AlarmNamePrefix string   `json:"AlarmNamePrefix,omitempty"`
	StateValue      string   `json:"StateValue,omitempty"`
	ActionPrefix    string   `json:"ActionPrefix,omitempty"`
	MaxRecords      *int32   `json:"MaxRecords,omitempty"`
	NextToken       string   `json:"NextToken,omitempty"`
}

// JSON Response types for CloudWatch JSON protocol.

// GetMetricDataResponse is the response for GetMetricData.
type GetMetricDataResponse struct {
	MetricDataResults []MetricDataResult `json:"MetricDataResults"`
	NextToken         string             `json:"NextToken,omitempty"`
}

// MetricDataResult represents a single metric data result.
type MetricDataResult struct {
	ID         string    `json:"Id"`
	Label      string    `json:"Label"`
	Timestamps []string  `json:"Timestamps"`
	Values     []float64 `json:"Values"`
	StatusCode string    `json:"StatusCode"`
}

// GetMetricStatisticsResponse is the response for GetMetricStatistics.
type GetMetricStatisticsResponse struct {
	Label      string      `json:"Label"`
	Datapoints []Datapoint `json:"Datapoints"`
}

// Datapoint represents a single datapoint.
type Datapoint struct {
	Timestamp   string   `json:"Timestamp"`
	SampleCount *float64 `json:"SampleCount,omitempty"`
	Average     *float64 `json:"Average,omitempty"`
	Sum         *float64 `json:"Sum,omitempty"`
	Minimum     *float64 `json:"Minimum,omitempty"`
	Maximum     *float64 `json:"Maximum,omitempty"`
	Unit        string   `json:"Unit,omitempty"`
}

// ListMetricsResponse is the response for ListMetrics.
type ListMetricsResponse struct {
	Metrics        []Metric `json:"Metrics"`
	NextToken      string   `json:"NextToken,omitempty"`
	OwningAccounts []string `json:"OwningAccounts,omitempty"`
}

// DescribeAlarmsResponse is the response for DescribeAlarms.
type DescribeAlarmsResponse struct {
	MetricAlarms []MetricAlarm `json:"MetricAlarms"`
	NextToken    string        `json:"NextToken,omitempty"`
}

// MetricAlarm represents a single metric alarm in JSON response.
type MetricAlarm struct {
	AlarmName                          string      `json:"AlarmName"`
	AlarmArn                           string      `json:"AlarmArn"`
	AlarmDescription                   string      `json:"AlarmDescription,omitempty"`
	MetricName                         string      `json:"MetricName"`
	Namespace                          string      `json:"Namespace"`
	Statistic                          string      `json:"Statistic,omitempty"`
	Dimensions                         []Dimension `json:"Dimensions,omitempty"`
	Period                             int32       `json:"Period"`
	EvaluationPeriods                  int32       `json:"EvaluationPeriods"`
	Threshold                          float64     `json:"Threshold"`
	ComparisonOperator                 string      `json:"ComparisonOperator"`
	ActionsEnabled                     bool        `json:"ActionsEnabled"`
	AlarmActions                       []string    `json:"AlarmActions,omitempty"`
	OKActions                          []string    `json:"OKActions,omitempty"`
	StateValue                         string      `json:"StateValue"`
	StateReason                        string      `json:"StateReason"`
	StateUpdatedTimestamp              string      `json:"StateUpdatedTimestamp"`
	AlarmConfigurationUpdatedTimestamp string      `json:"AlarmConfigurationUpdatedTimestamp"`
}

// ErrorResponse represents a CloudWatch error response in JSON format.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// Error represents a CloudWatch error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}

// CBOR Request types for RPC v2 CBOR protocol.
// These types use time.Time for timestamps which are sent as CBOR Tag (ID: 1).

// GetMetricDataCBORRequest is the CBOR request for GetMetricData.
type GetMetricDataCBORRequest struct {
	MetricDataQueries []MetricDataQuery `cbor:"MetricDataQueries"`
	StartTime         time.Time         `cbor:"StartTime"`
	EndTime           time.Time         `cbor:"EndTime"`
	NextToken         string            `cbor:"NextToken,omitempty"`
	MaxDatapoints     *int32            `cbor:"MaxDatapoints,omitempty"`
}

// GetMetricStatisticsCBORRequest is the CBOR request for GetMetricStatistics.
type GetMetricStatisticsCBORRequest struct {
	Namespace  string      `cbor:"Namespace"`
	MetricName string      `cbor:"MetricName"`
	Dimensions []Dimension `cbor:"Dimensions,omitempty"`
	StartTime  time.Time   `cbor:"StartTime"`
	EndTime    time.Time   `cbor:"EndTime"`
	Period     int32       `cbor:"Period"`
	Statistics []string    `cbor:"Statistics,omitempty"`
	Unit       string      `cbor:"Unit,omitempty"`
}

// CBOR Response types for RPC v2 CBOR protocol.

// GetMetricDataCBORResponse is the CBOR response for GetMetricData.
type GetMetricDataCBORResponse struct {
	MetricDataResults []MetricDataCBORResult `cbor:"MetricDataResults"`
	NextToken         string                 `cbor:"NextToken,omitempty"`
}

// MetricDataCBORResult represents a single metric data result for CBOR.
type MetricDataCBORResult struct {
	ID         string      `cbor:"Id"`
	Label      string      `cbor:"Label"`
	Timestamps []time.Time `cbor:"Timestamps"`
	Values     []float64   `cbor:"Values"`
	StatusCode string      `cbor:"StatusCode"`
}

// GetMetricStatisticsCBORResponse is the CBOR response for GetMetricStatistics.
type GetMetricStatisticsCBORResponse struct {
	Label      string          `cbor:"Label"`
	Datapoints []CBORDatapoint `cbor:"Datapoints"`
}

// CBORDatapoint represents a single datapoint for CBOR.
type CBORDatapoint struct {
	Timestamp   time.Time `cbor:"Timestamp"`
	SampleCount *float64  `cbor:"SampleCount,omitempty"`
	Average     *float64  `cbor:"Average,omitempty"`
	Sum         *float64  `cbor:"Sum,omitempty"`
	Minimum     *float64  `cbor:"Minimum,omitempty"`
	Maximum     *float64  `cbor:"Maximum,omitempty"`
	Unit        string    `cbor:"Unit,omitempty"`
}

// DescribeAlarmsCBORResponse is the CBOR response for DescribeAlarms.
type DescribeAlarmsCBORResponse struct {
	MetricAlarms []MetricAlarmCBOR `cbor:"MetricAlarms"`
	NextToken    string            `cbor:"NextToken,omitempty"`
}

// MetricAlarmCBOR represents a single metric alarm for CBOR response.
type MetricAlarmCBOR struct {
	AlarmName                          string      `cbor:"AlarmName"`
	AlarmArn                           string      `cbor:"AlarmArn"`
	AlarmDescription                   string      `cbor:"AlarmDescription,omitempty"`
	MetricName                         string      `cbor:"MetricName"`
	Namespace                          string      `cbor:"Namespace"`
	Statistic                          string      `cbor:"Statistic,omitempty"`
	Dimensions                         []Dimension `cbor:"Dimensions,omitempty"`
	Period                             int32       `cbor:"Period"`
	EvaluationPeriods                  int32       `cbor:"EvaluationPeriods"`
	Threshold                          float64     `cbor:"Threshold"`
	ComparisonOperator                 string      `cbor:"ComparisonOperator"`
	ActionsEnabled                     bool        `cbor:"ActionsEnabled"`
	AlarmActions                       []string    `cbor:"AlarmActions,omitempty"`
	OKActions                          []string    `cbor:"OKActions,omitempty"`
	StateValue                         string      `cbor:"StateValue"`
	StateReason                        string      `cbor:"StateReason"`
	StateUpdatedTimestamp              time.Time   `cbor:"StateUpdatedTimestamp"`
	AlarmConfigurationUpdatedTimestamp time.Time   `cbor:"AlarmConfigurationUpdatedTimestamp"`
}

// GetMetricDataResult is the result for GetMetricData storage operation.
type GetMetricDataResult struct {
	MetricDataResults []MetricDataResult
	NextToken         string
}

// GetMetricStatisticsResult is the result for GetMetricStatistics storage operation.
type GetMetricStatisticsResult struct {
	Label      string
	Datapoints []Datapoint
}

// ListMetricsResult is the result for ListMetrics storage operation.
type ListMetricsResult struct {
	Metrics        []Metric
	NextToken      string
	OwningAccounts []string
}

// DescribeAlarmsResult is the result for DescribeAlarms storage operation.
type DescribeAlarmsResult struct {
	MetricAlarms []MetricAlarm
	NextToken    string
}
