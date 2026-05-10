package cloudwatch

import (
	"encoding/xml"
	"net/http"

	"github.com/google/uuid"
)

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

// xmlTagResourceResponse / xmlUntagResourceResponse are the empty
// envelopes returned for the (no-op) tag mutation actions.
type xmlTagResourceResponse struct {
	XMLName          xml.Name            `xml:"TagResourceResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ResponseMetadata xmlResponseMetadata `xml:"ResponseMetadata"`
}

type xmlUntagResourceResponse struct {
	XMLName          xml.Name            `xml:"UntagResourceResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ResponseMetadata xmlResponseMetadata `xml:"ResponseMetadata"`
}

// ListTagsForResource returns an empty Tags list for any alarm.
//
// terraform-provider-aws calls this on every refresh of
// aws_cloudwatch_metric_alarm; without it apply errors immediately after
// PutMetricAlarm. Tags are not modeled in storage yet — same shape as
// the other no-op tag stubs in kumo.
func (s *Service) ListTagsForResource(w http.ResponseWriter, _ *http.Request) {
	writeCloudWatchXML(w, xmlListTagsForResourceResponse{
		Xmlns: cloudWatchXMLNS,
		ListTagsForResourceResult: xmlListTagsForResourceResult{
			Tags: xmlTagList{Members: []xmlTagMember{}},
		},
		ResponseMetadata: xmlResponseMetadata{RequestID: uuid.New().String()},
	})
}

// TagResource accepts and discards tag attachments.
func (s *Service) TagResource(w http.ResponseWriter, _ *http.Request) {
	writeCloudWatchXML(w, xmlTagResourceResponse{
		Xmlns:            cloudWatchXMLNS,
		ResponseMetadata: xmlResponseMetadata{RequestID: uuid.New().String()},
	})
}

// UntagResource accepts and discards tag detachments.
func (s *Service) UntagResource(w http.ResponseWriter, _ *http.Request) {
	writeCloudWatchXML(w, xmlUntagResourceResponse{
		Xmlns:            cloudWatchXMLNS,
		ResponseMetadata: xmlResponseMetadata{RequestID: uuid.New().String()},
	})
}
