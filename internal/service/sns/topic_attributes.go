package sns

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// flexString accepts a JSON string, number, or boolean and stores its
// canonical string form. Used for fields that are nominally strings on the
// wire but the AWS Query→JSON conversion has promoted to a typed literal.
type flexString string

// UnmarshalJSON implements json.Unmarshaler.
func (f *flexString) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*f = ""

		return nil
	}

	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("flexString: %w", err)
		}

		*f = flexString(s)

		return nil
	}

	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexString(n.String())

		return nil
	}

	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*f = flexString(strconv.FormatBool(b))

		return nil
	}

	return &json.UnsupportedTypeError{}
}

// String returns the underlying string.
func (f flexString) String() string { return string(f) }

// defaultTopicPolicy is the AWS-default access policy returned for any topic
// when none has been set explicitly. terraform-provider-aws hashes this for
// drift detection, so it must be a stable JSON document.
const defaultTopicPolicy = `{"Version":"2012-10-17","Id":"__default_policy_ID","Statement":[{"Sid":"__default_statement_ID","Effect":"Allow","Principal":{"AWS":"*"},"Action":["SNS:GetTopicAttributes","SNS:SetTopicAttributes","SNS:AddPermission","SNS:RemovePermission","SNS:DeleteTopic","SNS:Subscribe","SNS:ListSubscriptionsByTopic","SNS:Publish"],"Resource":"*"}]}`

// SetTopicAttributes accepts AttributeName + AttributeValue and writes them
// onto the topic's attribute map.
//
// terraform-provider-aws issues this once per attribute it wants to set
// after CreateTopic — and crucially, it sets *all* the optional feedback
// attributes (FirehoseSuccessFeedbackSampleRate, etc.) regardless of whether
// the resource block specifies them. Without an accepting handler, the
// first such call fails with InvalidAction and apply errors before any
// subsequent SetTopicAttributes is even tried.
func (s *Service) SetTopicAttributes(w http.ResponseWriter, r *http.Request) {
	var req setTopicAttributesRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TopicArn == "" {
		writeTopicError(w, errInvalidParameter, "TopicArn is required", http.StatusBadRequest)

		return
	}

	if req.AttributeName == "" {
		writeTopicError(w, errInvalidParameter, "AttributeName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.SetTopicAttribute(r.Context(), req.TopicArn, req.AttributeName, req.AttributeValue.String()); err != nil {
		handleTopicError(w, err)

		return
	}

	writeXMLResponse(w, XMLSetTopicAttributesResponse{
		Xmlns:            snsXMLNS,
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// GetTopicAttributes returns the topic's stored attributes plus the standard
// AWS-managed fields (TopicArn / Owner / Policy / SubscriptionsConfirmed /
// SubscriptionsPending / SubscriptionsDeleted) that terraform-provider-aws
// reads on every refresh.
func (s *Service) GetTopicAttributes(w http.ResponseWriter, r *http.Request) {
	var req getTopicAttributesRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TopicArn == "" {
		writeTopicError(w, errInvalidParameter, "TopicArn is required", http.StatusBadRequest)

		return
	}

	topic, err := s.storage.GetTopic(r.Context(), req.TopicArn)
	if err != nil {
		handleTopicError(w, err)

		return
	}

	attrs := buildTopicAttributeView(topic)

	entries := make([]XMLAttributeEntry, 0, len(attrs))
	for k, v := range attrs {
		entries = append(entries, XMLAttributeEntry{Key: k, Value: v})
	}

	writeXMLResponse(w, XMLGetTopicAttributesResponse{
		Xmlns: snsXMLNS,
		GetTopicAttributesResult: XMLGetTopicAttributesResult{
			Attributes: XMLAttributesMap{Entry: entries},
		},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ListTagsForResource returns an empty tag list for any topic.
func (s *Service) ListTagsForResource(w http.ResponseWriter, _ *http.Request) {
	writeXMLResponse(w, XMLListTagsForResourceResponse{
		Xmlns:            snsXMLNS,
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// TagResource accepts and discards tag attachments.
func (s *Service) TagResource(w http.ResponseWriter, _ *http.Request) {
	writeXMLResponse(w, XMLTagResourceResponse{
		Xmlns:            snsXMLNS,
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// UntagResource accepts and discards tag detachments.
func (s *Service) UntagResource(w http.ResponseWriter, _ *http.Request) {
	writeXMLResponse(w, XMLUntagResourceResponse{
		Xmlns:            snsXMLNS,
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// handleTopicError converts a storage error into the appropriate XML error.
func handleTopicError(w http.ResponseWriter, err error) {
	var tErr *TopicError
	if errors.As(err, &tErr) {
		status := http.StatusBadRequest
		if tErr.Code == "NotFound" {
			status = http.StatusNotFound
		}

		writeTopicError(w, tErr.Code, tErr.Message, status)

		return
	}

	writeTopicError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)
}

// buildTopicAttributeView merges the topic's stored attributes with the
// AWS-managed defaults terraform-provider-aws expects to read after refresh.
func buildTopicAttributeView(topic *Topic) map[string]string {
	attrs := map[string]string{
		"TopicArn":                topic.ARN,
		"Owner":                   "000000000000",
		"DisplayName":             topic.DisplayName,
		"Policy":                  defaultTopicPolicy,
		"SubscriptionsConfirmed":  "0",
		"SubscriptionsPending":    "0",
		"SubscriptionsDeleted":    "0",
		"DeliveryPolicy":          "",
		"EffectiveDeliveryPolicy": "",
	}

	for k, v := range topic.Attributes {
		attrs[k] = v
	}

	// Always reflect the canonical fields back from the topic record, even
	// if the user wrote a stale value into the attribute map.
	attrs["TopicArn"] = topic.ARN
	if topic.DisplayName != "" {
		attrs["DisplayName"] = topic.DisplayName
	}

	return attrs
}
