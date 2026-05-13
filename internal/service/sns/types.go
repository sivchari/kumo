// Package sns provides SNS service emulation for kumo.
package sns

import (
	"time"
)

// Topic represents an SNS topic.
type Topic struct {
	ARN           string
	Name          string
	DisplayName   string
	CreatedTime   time.Time
	Attributes    map[string]string
	Subscriptions map[string]*Subscription
}

// Subscription represents an SNS subscription.
type Subscription struct {
	ARN                          string
	TopicARN                     string
	Protocol                     string
	Endpoint                     string
	Owner                        string
	ConfirmationWasAuthenticated bool
	SubscriptionAttributes       map[string]string
}

// CreateTopicRequest is the request for CreateTopic.
type CreateTopicRequest struct {
	Name       string            `json:"Name"`
	Attributes map[string]string `json:"Attributes,omitempty"`
	Tags       []Tag             `json:"Tags,omitempty"`
}

// CreateTopicResponse is the response for CreateTopic.
type CreateTopicResponse struct {
	TopicARN string `json:"TopicArn"`
}

// DeleteTopicRequest is the request for DeleteTopic.
type DeleteTopicRequest struct {
	TopicARN string `json:"TopicArn"`
}

// ListTopicsRequest is the request for ListTopics.
type ListTopicsRequest struct {
	NextToken string `json:"NextToken,omitempty"`
}

// ListTopicsResponse is the response for ListTopics.
type ListTopicsResponse struct {
	Topics    []TopicEntry `json:"Topics"`
	NextToken string       `json:"NextToken,omitempty"`
}

// TopicEntry represents a topic in list response.
type TopicEntry struct {
	TopicARN string `json:"TopicArn"`
}

// SubscribeRequest is the request for Subscribe.
type SubscribeRequest struct {
	TopicARN              string            `json:"TopicArn"`
	Protocol              string            `json:"Protocol"`
	Endpoint              string            `json:"Endpoint,omitempty"`
	Attributes            map[string]string `json:"Attributes,omitempty"`
	ReturnSubscriptionArn bool              `json:"ReturnSubscriptionArn,omitempty"`
}

// SubscribeResponse is the response for Subscribe.
type SubscribeResponse struct {
	SubscriptionARN string `json:"SubscriptionArn"`
}

// UnsubscribeRequest is the request for Unsubscribe.
type UnsubscribeRequest struct {
	SubscriptionARN string `json:"SubscriptionArn"`
}

// PublishRequest is the request for Publish.
type PublishRequest struct {
	TopicARN               string                      `json:"TopicArn,omitempty"`
	TargetARN              string                      `json:"TargetArn,omitempty"`
	Message                string                      `json:"Message"`
	Subject                string                      `json:"Subject,omitempty"`
	MessageStructure       string                      `json:"MessageStructure,omitempty"`
	MessageAttributes      map[string]MessageAttribute `json:"MessageAttributes,omitempty"`
	MessageDeduplicationID string                      `json:"MessageDeduplicationId,omitempty"`
	MessageGroupID         string                      `json:"MessageGroupId,omitempty"`
}

// PublishResponse is the response for Publish.
type PublishResponse struct {
	MessageID      string `json:"MessageId"`
	SequenceNumber string `json:"SequenceNumber,omitempty"`
}

// ListSubscriptionsRequest is the request for ListSubscriptions.
type ListSubscriptionsRequest struct {
	NextToken string `json:"NextToken,omitempty"`
}

// ListSubscriptionsResponse is the response for ListSubscriptions.
type ListSubscriptionsResponse struct {
	Subscriptions []SubscriptionEntry `json:"Subscriptions"`
	NextToken     string              `json:"NextToken,omitempty"`
}

// ListSubscriptionsByTopicRequest is the request for ListSubscriptionsByTopic.
type ListSubscriptionsByTopicRequest struct {
	TopicARN  string `json:"TopicArn"`
	NextToken string `json:"NextToken,omitempty"`
}

// ListSubscriptionsByTopicResponse is the response for ListSubscriptionsByTopic.
type ListSubscriptionsByTopicResponse struct {
	Subscriptions []SubscriptionEntry `json:"Subscriptions"`
	NextToken     string              `json:"NextToken,omitempty"`
}

// SubscriptionEntry represents a subscription in list response.
type SubscriptionEntry struct {
	SubscriptionARN string `json:"SubscriptionArn"`
	Owner           string `json:"Owner"`
	Protocol        string `json:"Protocol"`
	Endpoint        string `json:"Endpoint"`
	TopicARN        string `json:"TopicArn"`
}

// MessageAttribute represents an SNS message attribute.
type MessageAttribute struct {
	DataType    string `json:"DataType"`
	StringValue string `json:"StringValue,omitempty"`
	BinaryValue []byte `json:"BinaryValue,omitempty"`
}

// Tag represents a tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// ErrorResponse represents an SNS error response.
type ErrorResponse struct {
	Type    string `json:"__type"`
	Message string `json:"Message"`
}

// XML response types for Query protocol.

// XMLCreateTopicResponse is the XML response for CreateTopic.
type XMLCreateTopicResponse struct {
	XMLName           struct{}             `xml:"CreateTopicResponse"`
	Xmlns             string               `xml:"xmlns,attr"`
	CreateTopicResult XMLCreateTopicResult `xml:"CreateTopicResult"`
	ResponseMetadata  ResponseMetadata     `xml:"ResponseMetadata"`
}

// XMLCreateTopicResult contains the CreateTopic result.
type XMLCreateTopicResult struct {
	TopicArn string `xml:"TopicArn"`
}

// XMLDeleteTopicResponse is the XML response for DeleteTopic.
type XMLDeleteTopicResponse struct {
	XMLName          struct{}         `xml:"DeleteTopicResponse"`
	Xmlns            string           `xml:"xmlns,attr"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// XMLListTopicsResponse is the XML response for ListTopics.
type XMLListTopicsResponse struct {
	XMLName          struct{}            `xml:"ListTopicsResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	ListTopicsResult XMLListTopicsResult `xml:"ListTopicsResult"`
	ResponseMetadata ResponseMetadata    `xml:"ResponseMetadata"`
}

// XMLListTopicsResult contains the ListTopics result.
type XMLListTopicsResult struct {
	Topics    XMLTopics `xml:"Topics"`
	NextToken string    `xml:"NextToken,omitempty"`
}

// XMLTopics is a wrapper for topic members.
type XMLTopics struct {
	Member []XMLTopicMember `xml:"member"`
}

// XMLTopicMember represents a topic in the list.
type XMLTopicMember struct {
	TopicArn string `xml:"TopicArn"`
}

// XMLSubscribeResponse is the XML response for Subscribe.
type XMLSubscribeResponse struct {
	XMLName          struct{}           `xml:"SubscribeResponse"`
	Xmlns            string             `xml:"xmlns,attr"`
	SubscribeResult  XMLSubscribeResult `xml:"SubscribeResult"`
	ResponseMetadata ResponseMetadata   `xml:"ResponseMetadata"`
}

// XMLSubscribeResult contains the Subscribe result.
type XMLSubscribeResult struct {
	SubscriptionArn string `xml:"SubscriptionArn"`
}

// XMLUnsubscribeResponse is the XML response for Unsubscribe.
type XMLUnsubscribeResponse struct {
	XMLName          struct{}         `xml:"UnsubscribeResponse"`
	Xmlns            string           `xml:"xmlns,attr"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// XMLPublishResponse is the XML response for Publish.
type XMLPublishResponse struct {
	XMLName          struct{}         `xml:"PublishResponse"`
	Xmlns            string           `xml:"xmlns,attr"`
	PublishResult    XMLPublishResult `xml:"PublishResult"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// XMLPublishResult contains the Publish result.
type XMLPublishResult struct {
	MessageID      string `xml:"MessageId"`
	SequenceNumber string `xml:"SequenceNumber,omitempty"`
}

// XMLListSubscriptionsResponse is the XML response for ListSubscriptions.
type XMLListSubscriptionsResponse struct {
	XMLName                 struct{}                   `xml:"ListSubscriptionsResponse"`
	Xmlns                   string                     `xml:"xmlns,attr"`
	ListSubscriptionsResult XMLListSubscriptionsResult `xml:"ListSubscriptionsResult"`
	ResponseMetadata        ResponseMetadata           `xml:"ResponseMetadata"`
}

// XMLListSubscriptionsResult contains the ListSubscriptions result.
type XMLListSubscriptionsResult struct {
	Subscriptions XMLSubscriptions `xml:"Subscriptions"`
	NextToken     string           `xml:"NextToken,omitempty"`
}

// XMLListSubscriptionsByTopicResponse is the XML response for ListSubscriptionsByTopic.
type XMLListSubscriptionsByTopicResponse struct {
	XMLName                        struct{}                          `xml:"ListSubscriptionsByTopicResponse"`
	Xmlns                          string                            `xml:"xmlns,attr"`
	ListSubscriptionsByTopicResult XMLListSubscriptionsByTopicResult `xml:"ListSubscriptionsByTopicResult"`
	ResponseMetadata               ResponseMetadata                  `xml:"ResponseMetadata"`
}

// XMLListSubscriptionsByTopicResult contains the ListSubscriptionsByTopic result.
type XMLListSubscriptionsByTopicResult struct {
	Subscriptions XMLSubscriptions `xml:"Subscriptions"`
	NextToken     string           `xml:"NextToken,omitempty"`
}

// XMLSubscriptions is a wrapper for subscription members.
type XMLSubscriptions struct {
	Member []XMLSubscriptionMember `xml:"member"`
}

// XMLSubscriptionMember represents a subscription in the list.
type XMLSubscriptionMember struct {
	SubscriptionArn string `xml:"SubscriptionArn"`
	Owner           string `xml:"Owner"`
	Protocol        string `xml:"Protocol"`
	Endpoint        string `xml:"Endpoint"`
	TopicArn        string `xml:"TopicArn"`
}

// ResponseMetadata contains the response metadata.
type ResponseMetadata struct {
	RequestID string `xml:"RequestId"`
}

// XMLErrorResponse is the XML error response.
type XMLErrorResponse struct {
	XMLName   struct{}       `xml:"ErrorResponse"`
	Xmlns     string         `xml:"xmlns,attr"`
	Error     XMLErrorDetail `xml:"Error"`
	RequestID string         `xml:"RequestId"`
}

// XMLErrorDetail contains error details.
type XMLErrorDetail struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// TopicError represents an SNS error.
type TopicError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *TopicError) Error() string {
	return e.Message
}

// XMLSetTopicAttributesResponse is the XML response for SetTopicAttributes.
type XMLSetTopicAttributesResponse struct {
	XMLName          struct{}         `xml:"SetTopicAttributesResponse"`
	Xmlns            string           `xml:"xmlns,attr"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// XMLGetTopicAttributesResponse is the XML response for GetTopicAttributes.
type XMLGetTopicAttributesResponse struct {
	XMLName                  struct{}                    `xml:"GetTopicAttributesResponse"`
	Xmlns                    string                      `xml:"xmlns,attr"`
	GetTopicAttributesResult XMLGetTopicAttributesResult `xml:"GetTopicAttributesResult"`
	ResponseMetadata         ResponseMetadata            `xml:"ResponseMetadata"`
}

// XMLGetTopicAttributesResult contains the attribute map.
type XMLGetTopicAttributesResult struct {
	Attributes XMLAttributesMap `xml:"Attributes"`
}

// XMLAttributesMap wraps the attribute entries as the AWS Query protocol
// expects: <Attributes><entry><key/><value/></entry>...</Attributes>.
type XMLAttributesMap struct {
	Entry []XMLAttributeEntry `xml:"entry"`
}

// XMLAttributeEntry is a single attribute key/value entry.
type XMLAttributeEntry struct {
	Key   string `xml:"key"`
	Value string `xml:"value"`
}

// XMLListTagsForResourceResponse mirrors the AWS shape; the Tags member is
// always an empty member list because tag storage is not modeled.
type XMLListTagsForResourceResponse struct {
	XMLName                   struct{}                     `xml:"ListTagsForResourceResponse"`
	Xmlns                     string                       `xml:"xmlns,attr"`
	ListTagsForResourceResult XMLListTagsForResourceResult `xml:"ListTagsForResourceResult"`
	ResponseMetadata          ResponseMetadata             `xml:"ResponseMetadata"`
}

// XMLListTagsForResourceResult contains the (always empty) tag list.
type XMLListTagsForResourceResult struct {
	Tags XMLTagList `xml:"Tags"`
}

// XMLTagList wraps the tag members.
type XMLTagList struct {
	Member []XMLTag `xml:"member"`
}

// XMLTag is a single tag.
type XMLTag struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// XMLTagResourceResponse is the XML response for TagResource.
type XMLTagResourceResponse struct {
	XMLName          struct{}         `xml:"TagResourceResponse"`
	Xmlns            string           `xml:"xmlns,attr"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// XMLUntagResourceResponse is the XML response for UntagResource.
type XMLUntagResourceResponse struct {
	XMLName          struct{}         `xml:"UntagResourceResponse"`
	Xmlns            string           `xml:"xmlns,attr"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// getSubscriptionAttributesRequest is the request for GetSubscriptionAttributes.
type getSubscriptionAttributesRequest struct {
	SubscriptionArn string `json:"SubscriptionArn"`
}

// setSubscriptionAttributesRequest is the request for SetSubscriptionAttributes.
type setSubscriptionAttributesRequest struct {
	SubscriptionArn string     `json:"SubscriptionArn"`
	AttributeName   string     `json:"AttributeName"`
	AttributeValue  flexString `json:"AttributeValue"`
}

// XMLSetSubscriptionAttributesResponse is the XML response for SetSubscriptionAttributes.
type XMLSetSubscriptionAttributesResponse struct {
	XMLName          struct{}         `xml:"SetSubscriptionAttributesResponse"`
	Xmlns            string           `xml:"xmlns,attr"`
	ResponseMetadata ResponseMetadata `xml:"ResponseMetadata"`
}

// XMLGetSubscriptionAttributesResponse is the XML response for GetSubscriptionAttributes.
type XMLGetSubscriptionAttributesResponse struct {
	XMLName                         struct{}                           `xml:"GetSubscriptionAttributesResponse"`
	Xmlns                           string                             `xml:"xmlns,attr"`
	GetSubscriptionAttributesResult XMLGetSubscriptionAttributesResult `xml:"GetSubscriptionAttributesResult"`
	ResponseMetadata                ResponseMetadata                   `xml:"ResponseMetadata"`
}

// XMLGetSubscriptionAttributesResult contains the attribute map.
type XMLGetSubscriptionAttributesResult struct {
	Attributes XMLAttributesMap `xml:"Attributes"`
}

// setTopicAttributesRequest is the wire shape after the Query dispatcher
// converts terraform's form-encoded body to JSON.
//
// AttributeValue uses the lenient flexString helper because the dispatcher's
// formToJSON heuristically promotes "0" / "1" / "true" / "false" to numeric
// or boolean JSON literals, but AWS itself sends every SetTopicAttributes
// value as a string and clients (terraform-provider-aws) produce
// numeric-looking sample-rate values that must round-trip as strings.
type setTopicAttributesRequest struct {
	TopicArn       string     `json:"TopicArn"`
	AttributeName  string     `json:"AttributeName"`
	AttributeValue flexString `json:"AttributeValue"`
}

// getTopicAttributesRequest mirrors GetTopicAttributes after the Query
// dispatcher converts the form-encoded body to JSON.
type getTopicAttributesRequest struct {
	TopicArn string `json:"TopicArn"`
}
