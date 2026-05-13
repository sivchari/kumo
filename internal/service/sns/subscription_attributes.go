package sns

import (
	"net/http"

	"github.com/google/uuid"
)

// GetSubscriptionAttributes returns attributes for a subscription.
//
// terraform-provider-aws polls this after Subscribe to confirm the
// subscription is active. Without this handler, kumo returns
// InvalidAction and terraform apply fails on all
// aws_sns_topic_subscription resources.
func (s *Service) GetSubscriptionAttributes(w http.ResponseWriter, r *http.Request) {
	var req getSubscriptionAttributesRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.SubscriptionArn == "" {
		writeTopicError(w, errInvalidParameter, "SubscriptionArn is required", http.StatusBadRequest)

		return
	}

	sub, err := s.storage.GetSubscription(r.Context(), req.SubscriptionArn)
	if err != nil {
		handleTopicError(w, err)

		return
	}

	attrs := buildSubscriptionAttributeView(sub)

	entries := make([]XMLAttributeEntry, 0, len(attrs))
	for k, v := range attrs {
		entries = append(entries, XMLAttributeEntry{Key: k, Value: v})
	}

	writeXMLResponse(w, XMLGetSubscriptionAttributesResponse{
		Xmlns: snsXMLNS,
		GetSubscriptionAttributesResult: XMLGetSubscriptionAttributesResult{
			Attributes: XMLAttributesMap{Entry: entries},
		},
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// SetSubscriptionAttributes sets a single attribute on a subscription.
//
// terraform-provider-aws calls this after Subscribe to set attributes
// like RawMessageDelivery, FilterPolicy, etc.
func (s *Service) SetSubscriptionAttributes(w http.ResponseWriter, r *http.Request) {
	var req setSubscriptionAttributesRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.SubscriptionArn == "" {
		writeTopicError(w, errInvalidParameter, "SubscriptionArn is required", http.StatusBadRequest)

		return
	}

	if req.AttributeName == "" {
		writeTopicError(w, errInvalidParameter, "AttributeName is required", http.StatusBadRequest)

		return
	}

	if err := s.storage.SetSubscriptionAttribute(r.Context(), req.SubscriptionArn, req.AttributeName, req.AttributeValue.String()); err != nil {
		handleTopicError(w, err)

		return
	}

	writeXMLResponse(w, XMLSetSubscriptionAttributesResponse{
		Xmlns:            snsXMLNS,
		ResponseMetadata: ResponseMetadata{RequestID: uuid.New().String()},
	})
}

// buildSubscriptionAttributeView returns the attribute map terraform expects.
func buildSubscriptionAttributeView(sub *Subscription) map[string]string {
	attrs := map[string]string{
		"SubscriptionArn":              sub.ARN,
		"TopicArn":                     sub.TopicARN,
		"Protocol":                     sub.Protocol,
		"Endpoint":                     sub.Endpoint,
		"Owner":                        sub.Owner,
		"PendingConfirmation":          "false",
		"ConfirmationWasAuthenticated": "true",
	}

	for k, v := range sub.SubscriptionAttributes {
		attrs[k] = v
	}

	return attrs
}
