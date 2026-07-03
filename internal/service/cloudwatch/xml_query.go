package cloudwatch

import (
	"encoding/xml"
	"net/http"

	"github.com/google/uuid"
)

// CloudWatch's Query protocol (used by terraform-provider-aws and other
// pre-CBOR clients) wraps responses in <ActionResponse> XML envelopes.
// The JSON DispatchAction path was originally written for non-existent
// JSON clients; its handlers now route through these XML wrappers when
// reached via the unified Query→JSON dispatcher.

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
			StateUpdatedTimestamp:              a.StateUpdatedTimestamp,
			AlarmConfigurationUpdatedTimestamp: a.AlarmConfigurationUpdatedTimestamp,
		})
	}

	return out
}
