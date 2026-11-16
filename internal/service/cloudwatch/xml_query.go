package cloudwatch

import (
	"encoding/xml"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// CloudWatch's Query protocol (used by terraform-provider-aws and other
// pre-CBOR clients) wraps responses in <ActionResponse> XML envelopes.
// DispatchAction serves both AWS JSON 1.0 clients and Query clients reached
// via the unified Query→JSON dispatcher; handlers use these XML wrappers
// only when service.IsQueryProtocol reports the latter.

const cloudWatchXMLNS = "http://monitoring.amazonaws.com/doc/2010-08-01/"

// writeCloudWatchXML writes an XML response with HTTP 200 OK.
func writeCloudWatchXML(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

// xmlResponseMetadata is the standard ResponseMetadata block AWS includes
// in every Query-protocol response.
type xmlResponseMetadata struct {
	RequestID string `xml:"RequestId"`
}

// writeCloudWatchXMLError writes a Query-protocol ErrorResponse with the
// given HTTP status.
func writeCloudWatchXMLError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(xmlErrorResponse{
		Xmlns:     cloudWatchXMLNS,
		Error:     xmlError{Type: errorFaultType(status), Code: code, Message: message},
		RequestID: requestID,
	})
}

type xmlErrorResponse struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	Xmlns     string   `xml:"xmlns,attr"`
	Error     xmlError `xml:"Error"`
	RequestID string   `xml:"RequestId"`
}

type xmlError struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// xmlTimestamp formats t as the ISO 8601 timestamp the Query protocol uses,
// at the same millisecond precision as the JSON path.
func xmlTimestamp(t time.Time) string {
	return t.UTC().Round(time.Millisecond).Format(time.RFC3339Nano)
}

// xmlStringList wraps scalar members in the AWS Query list shape.
type xmlStringList struct {
	Members []string `xml:"member"`
}

// xmlPutMetricDataResponse is the empty PutMetricData response envelope.
type xmlPutMetricDataResponse struct {
	XMLName          xml.Name            `xml:"PutMetricDataResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ResponseMetadata xmlResponseMetadata `xml:"ResponseMetadata"`
}

// xmlGetMetricDataResponse wraps the GetMetricData result.
type xmlGetMetricDataResponse struct {
	XMLName             xml.Name               `xml:"GetMetricDataResponse"`
	Xmlns               string                 `xml:"xmlns,attr"`
	GetMetricDataResult xmlGetMetricDataResult `xml:"GetMetricDataResult"`
	ResponseMetadata    xmlResponseMetadata    `xml:"ResponseMetadata"`
}

type xmlGetMetricDataResult struct {
	MetricDataResults xmlMetricDataResultList `xml:"MetricDataResults"`
	NextToken         string                  `xml:"NextToken,omitempty"`
}

type xmlMetricDataResultList struct {
	Members []xmlMetricDataResult `xml:"member"`
}

type xmlMetricDataResult struct {
	ID         string        `xml:"Id"`
	Label      string        `xml:"Label"`
	Timestamps xmlStringList `xml:"Timestamps"`
	Values     xmlFloatList  `xml:"Values"`
	StatusCode string        `xml:"StatusCode"`
}

type xmlFloatList struct {
	Members []float64 `xml:"member"`
}

// xmlGetMetricStatisticsResponse wraps the GetMetricStatistics result.
type xmlGetMetricStatisticsResponse struct {
	XMLName                   xml.Name                     `xml:"GetMetricStatisticsResponse"`
	Xmlns                     string                       `xml:"xmlns,attr"`
	GetMetricStatisticsResult xmlGetMetricStatisticsResult `xml:"GetMetricStatisticsResult"`
	ResponseMetadata          xmlResponseMetadata          `xml:"ResponseMetadata"`
}

type xmlGetMetricStatisticsResult struct {
	Label      string           `xml:"Label"`
	Datapoints xmlDatapointList `xml:"Datapoints"`
}

type xmlDatapointList struct {
	Members []xmlDatapoint `xml:"member"`
}

type xmlDatapoint struct {
	Timestamp   string   `xml:"Timestamp,omitempty"`
	SampleCount *float64 `xml:"SampleCount,omitempty"`
	Average     *float64 `xml:"Average,omitempty"`
	Sum         *float64 `xml:"Sum,omitempty"`
	Minimum     *float64 `xml:"Minimum,omitempty"`
	Maximum     *float64 `xml:"Maximum,omitempty"`
	Unit        string   `xml:"Unit,omitempty"`
}

// xmlListMetricsResponse wraps the ListMetrics result.
type xmlListMetricsResponse struct {
	XMLName           xml.Name             `xml:"ListMetricsResponse"`
	Xmlns             string               `xml:"xmlns,attr"`
	ListMetricsResult xmlListMetricsResult `xml:"ListMetricsResult"`
	ResponseMetadata  xmlResponseMetadata  `xml:"ResponseMetadata"`
}

type xmlListMetricsResult struct {
	Metrics        xmlMetricList `xml:"Metrics"`
	NextToken      string        `xml:"NextToken,omitempty"`
	OwningAccounts xmlStringList `xml:"OwningAccounts"`
}

type xmlMetricList struct {
	Members []xmlMetric `xml:"member"`
}

type xmlMetric struct {
	Namespace  string           `xml:"Namespace"`
	MetricName string           `xml:"MetricName"`
	Dimensions xmlDimensionList `xml:"Dimensions"`
}

type xmlDimensionList struct {
	Members []Dimension `xml:"member"`
}

// xmlSetAlarmStateResponse is the empty SetAlarmState response envelope.
type xmlSetAlarmStateResponse struct {
	XMLName          xml.Name            `xml:"SetAlarmStateResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ResponseMetadata xmlResponseMetadata `xml:"ResponseMetadata"`
}

// xmlPutMetricAlarmResponse is the empty PutMetricAlarm response envelope.
type xmlPutMetricAlarmResponse struct {
	XMLName          xml.Name            `xml:"PutMetricAlarmResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ResponseMetadata xmlResponseMetadata `xml:"ResponseMetadata"`
}

// xmlDeleteAlarmsResponse is the empty DeleteAlarms response envelope.
type xmlDeleteAlarmsResponse struct {
	XMLName          xml.Name            `xml:"DeleteAlarmsResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ResponseMetadata xmlResponseMetadata `xml:"ResponseMetadata"`
}

// xmlDescribeAlarmsResponse wraps the DescribeAlarms result.
type xmlDescribeAlarmsResponse struct {
	XMLName              xml.Name                `xml:"DescribeAlarmsResponse"`
	Xmlns                string                  `xml:"xmlns,attr"`
	DescribeAlarmsResult xmlDescribeAlarmsResult `xml:"DescribeAlarmsResult"`
	ResponseMetadata     xmlResponseMetadata     `xml:"ResponseMetadata"`
}

type xmlDescribeAlarmsResult struct {
	MetricAlarms xmlMetricAlarmList `xml:"MetricAlarms"`
	NextToken    string             `xml:"NextToken,omitempty"`
}

// xmlMetricAlarmList wraps the alarm members in the AWS Query shape
// (<MetricAlarms><member>...</member>...</MetricAlarms>).
type xmlMetricAlarmList struct {
	Members []xmlMetricAlarm `xml:"member"`
}

// xmlMetricAlarm is a single metric alarm entry. Field set kept tight to
// what terraform-provider-aws reads on refresh.
type xmlMetricAlarm struct {
	AlarmName                          string  `xml:"AlarmName"`
	AlarmArn                           string  `xml:"AlarmArn"`
	AlarmDescription                   string  `xml:"AlarmDescription,omitempty"`
	MetricName                         string  `xml:"MetricName"`
	Namespace                          string  `xml:"Namespace"`
	Statistic                          string  `xml:"Statistic,omitempty"`
	Period                             int32   `xml:"Period"`
	EvaluationPeriods                  int32   `xml:"EvaluationPeriods"`
	Threshold                          float64 `xml:"Threshold"`
	ComparisonOperator                 string  `xml:"ComparisonOperator"`
	ActionsEnabled                     bool    `xml:"ActionsEnabled"`
	StateValue                         string  `xml:"StateValue"`
	StateReason                        string  `xml:"StateReason,omitempty"`
	StateUpdatedTimestamp              string  `xml:"StateUpdatedTimestamp,omitempty"`
	AlarmConfigurationUpdatedTimestamp string  `xml:"AlarmConfigurationUpdatedTimestamp,omitempty"`
}

// xmlListTagsForResourceResponse echoes the AWS Query shape
// (`<ListTagsForResourceResponse><ListTagsForResourceResult><Tags>...</Tags></ListTagsForResourceResult></...>`).
type xmlListTagsForResourceResponse struct {
	XMLName                   xml.Name                     `xml:"ListTagsForResourceResponse"`
	Xmlns                     string                       `xml:"xmlns,attr"`
	ListTagsForResourceResult xmlListTagsForResourceResult `xml:"ListTagsForResourceResult"`
	ResponseMetadata          xmlResponseMetadata          `xml:"ResponseMetadata"`
}

type xmlListTagsForResourceResult struct {
	Tags xmlTagList `xml:"Tags"`
}

type xmlTagList struct {
	Members []xmlTagMember `xml:"member"`
}

type xmlTagMember struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// xmlTagResourceResponse is the empty envelope returned for TagResource.
type xmlTagResourceResponse struct {
	XMLName          xml.Name            `xml:"TagResourceResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ResponseMetadata xmlResponseMetadata `xml:"ResponseMetadata"`
}

// xmlUntagResourceResponse is the empty envelope returned for UntagResource.
type xmlUntagResourceResponse struct {
	XMLName          xml.Name            `xml:"UntagResourceResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ResponseMetadata xmlResponseMetadata `xml:"ResponseMetadata"`
}

// metricAlarmsToXML converts the internal MetricAlarm slice to the XML
// member list shape.
func metricAlarmsToXML(alarms []MetricAlarm) []xmlMetricAlarm {
	out := make([]xmlMetricAlarm, 0, len(alarms))

	for i := range alarms {
		a := alarms[i]
		out = append(out, xmlMetricAlarm{
			AlarmName:                          a.AlarmName,
			AlarmArn:                           a.AlarmArn,
			AlarmDescription:                   a.AlarmDescription,
			MetricName:                         a.MetricName,
			Namespace:                          a.Namespace,
			Statistic:                          a.Statistic,
			Period:                             a.Period,
			EvaluationPeriods:                  a.EvaluationPeriods,
			Threshold:                          a.Threshold,
			ComparisonOperator:                 a.ComparisonOperator,
			ActionsEnabled:                     a.ActionsEnabled,
			StateValue:                         a.StateValue,
			StateReason:                        a.StateReason,
			StateUpdatedTimestamp:              a.StateUpdatedTimestamp.Format(time.RFC3339),
			AlarmConfigurationUpdatedTimestamp: a.AlarmConfigurationUpdatedTimestamp.Format(time.RFC3339),
		})
	}

	return out
}

func metricDataResultsToXML(results []MetricDataResult) []xmlMetricDataResult {
	out := make([]xmlMetricDataResult, 0, len(results))

	for i := range results {
		result := results[i]

		timestamps := make([]string, 0, len(result.Timestamps))
		for _, ts := range result.Timestamps {
			timestamps = append(timestamps, xmlTimestamp(ts.Time))
		}

		out = append(out, xmlMetricDataResult{
			ID:         result.ID,
			Label:      result.Label,
			Timestamps: xmlStringList{Members: timestamps},
			Values:     xmlFloatList{Members: result.Values},
			StatusCode: result.StatusCode,
		})
	}

	return out
}

func datapointsToXML(datapoints []Datapoint) []xmlDatapoint {
	out := make([]xmlDatapoint, 0, len(datapoints))

	for i := range datapoints {
		dp := datapoints[i]

		var timestamp string
		if !dp.Timestamp.IsZero() {
			timestamp = xmlTimestamp(dp.Timestamp.Time)
		}

		out = append(out, xmlDatapoint{
			Timestamp:   timestamp,
			SampleCount: dp.SampleCount,
			Average:     dp.Average,
			Sum:         dp.Sum,
			Minimum:     dp.Minimum,
			Maximum:     dp.Maximum,
			Unit:        dp.Unit,
		})
	}

	return out
}

func metricsToXML(metrics []Metric) []xmlMetric {
	out := make([]xmlMetric, 0, len(metrics))

	for _, m := range metrics {
		out = append(out, xmlMetric{
			Namespace:  m.Namespace,
			MetricName: m.MetricName,
			Dimensions: xmlDimensionList{Members: m.Dimensions},
		})
	}

	return out
}
