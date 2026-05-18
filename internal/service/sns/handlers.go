package sns

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const snsXMLNS = "http://sns.amazonaws.com/doc/2010-03-31/"

// Error codes for SNS.
const (
	errNotFound             = "NotFound"
	errInvalidParameter     = "InvalidParameter"
	errInternalServiceError = "InternalError"
	errInvalidAction        = "InvalidAction"
)

// CreateTopic handles the CreateTopic action.
func (s *Service) CreateTopic(w http.ResponseWriter, r *http.Request) {
	var req CreateTopicRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeTopicError(w, errInvalidParameter, "Topic name is required", http.StatusBadRequest)

		return
	}

	topic, err := s.storage.CreateTopic(r.Context(), req.Name, req.Attributes)
	if err != nil {
		var sErr *TopicError
		if errors.As(err, &sErr) {
			writeTopicError(w, sErr.Code, sErr.Message, http.StatusBadRequest)

			return
		}

		writeTopicError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLCreateTopicResponse{
		Xmlns: snsXMLNS,
		CreateTopicResult: XMLCreateTopicResult{
			TopicArn: topic.ARN,
		},
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// DeleteTopic handles the DeleteTopic action.
func (s *Service) DeleteTopic(w http.ResponseWriter, r *http.Request) {
	var req DeleteTopicRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TopicARN == "" {
		writeTopicError(w, errInvalidParameter, "TopicArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteTopic(r.Context(), req.TopicARN)
	if err != nil {
		var sErr *TopicError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeTopicError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeTopicError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	// DeleteTopic returns empty response on success.
	writeXMLResponse(w, XMLDeleteTopicResponse{
		Xmlns: snsXMLNS,
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// ListTopics handles the ListTopics action.
func (s *Service) ListTopics(w http.ResponseWriter, r *http.Request) {
	var req ListTopicsRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	topics, nextToken, err := s.storage.ListTopics(r.Context(), req.NextToken)
	if err != nil {
		writeTopicError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	topicMembers := make([]XMLTopicMember, 0, len(topics))
	for _, topic := range topics {
		topicMembers = append(topicMembers, XMLTopicMember{
			TopicArn: topic.ARN,
		})
	}

	writeXMLResponse(w, XMLListTopicsResponse{
		Xmlns: snsXMLNS,
		ListTopicsResult: XMLListTopicsResult{
			Topics: XMLTopics{
				Member: topicMembers,
			},
			NextToken: nextToken,
		},
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// Subscribe handles the Subscribe action.
func (s *Service) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req SubscribeRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TopicARN == "" {
		writeTopicError(w, errInvalidParameter, "TopicArn is required", http.StatusBadRequest)

		return
	}

	if req.Protocol == "" {
		writeTopicError(w, errInvalidParameter, "Protocol is required", http.StatusBadRequest)

		return
	}

	subscription, err := s.storage.Subscribe(r.Context(), req.TopicARN, req.Protocol, req.Endpoint, req.Attributes)
	if err != nil {
		var sErr *TopicError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeTopicError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeTopicError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLSubscribeResponse{
		Xmlns: snsXMLNS,
		SubscribeResult: XMLSubscribeResult{
			SubscriptionArn: subscription.ARN,
		},
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// Unsubscribe handles the Unsubscribe action.
func (s *Service) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req UnsubscribeRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.SubscriptionARN == "" {
		writeTopicError(w, errInvalidParameter, "SubscriptionArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.Unsubscribe(r.Context(), req.SubscriptionARN)
	if err != nil {
		var sErr *TopicError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeTopicError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeTopicError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	// Unsubscribe returns empty response on success.
	writeXMLResponse(w, XMLUnsubscribeResponse{
		Xmlns: snsXMLNS,
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// Publish handles the Publish action.
func (s *Service) Publish(w http.ResponseWriter, r *http.Request) {
	var req PublishRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	topicARN := req.TopicARN
	if topicARN == "" {
		topicARN = req.TargetARN
	}

	if topicARN == "" {
		writeTopicError(w, errInvalidParameter, "TopicArn or TargetArn is required", http.StatusBadRequest)

		return
	}

	if req.Message == "" {
		writeTopicError(w, errInvalidParameter, "Message is required", http.StatusBadRequest)

		return
	}

	messageID, err := s.storage.Publish(r.Context(), topicARN, req.Message, req.Subject, req.MessageGroupID, req.MessageDeduplicationID, req.MessageAttributes)
	if err != nil {
		var sErr *TopicError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeTopicError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeTopicError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	writeXMLResponse(w, XMLPublishResponse{
		Xmlns: snsXMLNS,
		PublishResult: XMLPublishResult{
			MessageID: messageID,
		},
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// ListSubscriptions handles the ListSubscriptions action.
func (s *Service) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	var req ListSubscriptionsRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	subscriptions, nextToken, err := s.storage.ListSubscriptions(r.Context(), req.NextToken)
	if err != nil {
		writeTopicError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	members := convertSubscriptionsToXMLMembers(subscriptions)

	writeXMLResponse(w, XMLListSubscriptionsResponse{
		Xmlns: snsXMLNS,
		ListSubscriptionsResult: XMLListSubscriptionsResult{
			Subscriptions: XMLSubscriptions{
				Member: members,
			},
			NextToken: nextToken,
		},
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// ListSubscriptionsByTopic handles the ListSubscriptionsByTopic action.
func (s *Service) ListSubscriptionsByTopic(w http.ResponseWriter, r *http.Request) {
	var req ListSubscriptionsByTopicRequest
	if err := readJSONRequest(r, &req); err != nil {
		writeTopicError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TopicARN == "" {
		writeTopicError(w, errInvalidParameter, "TopicArn is required", http.StatusBadRequest)

		return
	}

	subscriptions, nextToken, err := s.storage.ListSubscriptionsByTopic(r.Context(), req.TopicARN, req.NextToken)
	if err != nil {
		var sErr *TopicError
		if errors.As(err, &sErr) {
			status := http.StatusBadRequest
			if sErr.Code == errNotFound {
				status = http.StatusNotFound
			}

			writeTopicError(w, sErr.Code, sErr.Message, status)

			return
		}

		writeTopicError(w, errInternalServiceError, "Internal server error", http.StatusInternalServerError)

		return
	}

	members := convertSubscriptionsToXMLMembers(subscriptions)

	writeXMLResponse(w, XMLListSubscriptionsByTopicResponse{
		Xmlns: snsXMLNS,
		ListSubscriptionsByTopicResult: XMLListSubscriptionsByTopicResult{
			Subscriptions: XMLSubscriptions{
				Member: members,
			},
			NextToken: nextToken,
		},
		ResponseMetadata: ResponseMetadata{
			RequestID: uuid.New().String(),
		},
	})
}

// convertSubscriptionsToXMLMembers converts subscriptions to XML members.
func convertSubscriptionsToXMLMembers(subscriptions []*Subscription) []XMLSubscriptionMember {
	members := make([]XMLSubscriptionMember, 0, len(subscriptions))

	for _, sub := range subscriptions {
		members = append(members, XMLSubscriptionMember{
			SubscriptionArn: sub.ARN,
			Owner:           sub.Owner,
			Protocol:        sub.Protocol,
			Endpoint:        sub.Endpoint,
			TopicArn:        sub.TopicARN,
		})
	}

	return members
}

// readJSONRequest reads and decodes JSON request body.
func readJSONRequest(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// writeXMLResponse writes an XML response with HTTP 200 OK.
func writeXMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

// writeTopicError writes an SNS error response in XML format.
func writeTopicError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(XMLErrorResponse{
		Xmlns: snsXMLNS,
		Error: XMLErrorDetail{
			Type:    "Sender",
			Code:    code,
			Message: message,
		},
		RequestID: requestID,
	})
}

// actionHandlers returns a map of action names to handler functions.
func (s *Service) actionHandlers() map[string]func(http.ResponseWriter, *http.Request) {
	return map[string]func(http.ResponseWriter, *http.Request){
		"CreateTopic":               s.CreateTopic,
		"DeleteTopic":               s.DeleteTopic,
		"ListTopics":                s.ListTopics,
		"Subscribe":                 s.Subscribe,
		"Unsubscribe":               s.Unsubscribe,
		"Publish":                   s.Publish,
		"ListSubscriptions":         s.ListSubscriptions,
		"ListSubscriptionsByTopic":  s.ListSubscriptionsByTopic,
		"GetSubscriptionAttributes": s.GetSubscriptionAttributes,
		"SetSubscriptionAttributes": s.SetSubscriptionAttributes,
		"GetTopicAttributes":        s.GetTopicAttributes,
		"SetTopicAttributes":        s.SetTopicAttributes,
		"ListTagsForResource":       s.ListTagsForResource,
		"TagResource":               s.TagResource,
		"UntagResource":             s.UntagResource,
	}
}

// DispatchAction routes the request to the appropriate handler based on X-Amz-Target header.
// This method implements the JSONProtocolService interface.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	target := r.Header.Get("X-Amz-Target")
	action := strings.TrimPrefix(target, "AmazonSimpleNotificationService.")

	handlers := s.actionHandlers()
	if handler, ok := handlers[action]; ok {
		handler(w, r)

		return
	}

	writeTopicError(w, errInvalidAction, "The action "+action+" is not valid", http.StatusBadRequest)
}
