package sns

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "000000000000"
)

// SQSPublisher is an interface for publishing messages to SQS.
type SQSPublisher interface {
	PublishToSQS(ctx context.Context, queueURL, messageBody, messageGroupID, messageDeduplicationID string, attributes map[string]string) error
}

// Storage defines the SNS storage interface.
type Storage interface {
	CreateTopic(ctx context.Context, name string, attributes map[string]string) (*Topic, error)
	GetTopic(ctx context.Context, topicARN string) (*Topic, error)
	SetTopicAttribute(ctx context.Context, topicARN, name, value string) error
	DeleteTopic(ctx context.Context, topicARN string) error
	ListTopics(ctx context.Context, nextToken string) ([]*Topic, string, error)
	Subscribe(ctx context.Context, topicARN, protocol, endpoint string, attributes map[string]string) (*Subscription, error)
	GetSubscription(ctx context.Context, subscriptionARN string) (*Subscription, error)
	SetSubscriptionAttribute(ctx context.Context, subscriptionARN, name, value string) error
	Unsubscribe(ctx context.Context, subscriptionARN string) error
	Publish(ctx context.Context, topicARN, message, subject, messageGroupID, messageDeduplicationID string, attributes map[string]MessageAttribute) (string, error)
	ListSubscriptions(ctx context.Context, nextToken string) ([]*Subscription, string, error)
	ListSubscriptionsByTopic(ctx context.Context, topicARN, nextToken string) ([]*Subscription, string, error)
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu            sync.RWMutex             `json:"-"`
	Topics        map[string]*Topic        `json:"topics"`        // keyed by ARN
	Subscriptions map[string]*Subscription `json:"subscriptions"` // keyed by ARN
	baseURL       string
	SqsPublisher  SQSPublisher `json:"-"`
	dataDir       string
}

// NewMemoryStorage creates a new in-memory SNS storage.
func NewMemoryStorage(baseURL string, opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Topics:        make(map[string]*Topic),
		Subscriptions: make(map[string]*Subscription),
		baseURL:       baseURL,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "sns", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (m *MemoryStorage) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(m)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (m *MemoryStorage) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(m)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if m.Topics == nil {
		m.Topics = make(map[string]*Topic)
	}

	if m.Subscriptions == nil {
		m.Subscriptions = make(map[string]*Subscription)
	}

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (m *MemoryStorage) saveLocked() {
	if m.dataDir == "" {
		return
	}

	storage.ScheduleSave(m.dataDir, "sns", m.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "sns", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// SetSQSPublisher sets the SQS publisher for SNS to SQS integration.
func (m *MemoryStorage) SetSQSPublisher(publisher SQSPublisher) {
	m.SqsPublisher = publisher
}

// CreateTopic creates a new topic.
func (m *MemoryStorage) CreateTopic(_ context.Context, name string, attributes map[string]string) (*Topic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	arn := m.buildTopicARN(name)

	// Return existing topic if it exists.
	if topic, exists := m.Topics[arn]; exists {
		return topic, nil
	}

	topic := &Topic{
		ARN:           arn,
		Name:          name,
		CreatedTime:   time.Now(),
		Attributes:    attributes,
		Subscriptions: make(map[string]*Subscription),
	}

	if attributes != nil {
		if displayName, ok := attributes["DisplayName"]; ok {
			topic.DisplayName = displayName
		}
	}

	m.Topics[arn] = topic

	m.saveLocked()

	return topic, nil
}

// GetTopic returns the topic with the given ARN, or NotFound.
func (m *MemoryStorage) GetTopic(_ context.Context, topicARN string) (*Topic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	topic, exists := m.Topics[topicARN]
	if !exists {
		return nil, &TopicError{Code: "NotFound", Message: "Topic does not exist: " + topicARN}
	}

	return topic, nil
}

// SetTopicAttribute writes name=value into the topic's attribute map.
//
// AWS exposes a small set of mutable topic attributes via SetTopicAttributes
// (DisplayName, Policy, DeliveryPolicy, plus the *FeedbackRoleArn and
// *FeedbackSampleRate families). kumo does not enforce or validate any of
// them; we just persist the write so subsequent GetTopicAttributes reflects
// it. The DisplayName field on the topic is also updated so existing code
// paths that read it directly stay consistent.
func (m *MemoryStorage) SetTopicAttribute(_ context.Context, topicARN, name, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	topic, exists := m.Topics[topicARN]
	if !exists {
		return &TopicError{Code: "NotFound", Message: "Topic does not exist: " + topicARN}
	}

	if topic.Attributes == nil {
		topic.Attributes = make(map[string]string)
	}

	topic.Attributes[name] = value

	if name == "DisplayName" {
		topic.DisplayName = value
	}

	m.saveLocked()

	return nil
}

// DeleteTopic deletes a topic.
func (m *MemoryStorage) DeleteTopic(_ context.Context, topicARN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	topic, exists := m.Topics[topicARN]
	if !exists {
		return &TopicError{
			Code:    "NotFound",
			Message: fmt.Sprintf("Topic does not exist: %s", topicARN),
		}
	}

	// Delete all subscriptions for this topic.
	for subARN := range topic.Subscriptions {
		delete(m.Subscriptions, subARN)
	}

	delete(m.Topics, topicARN)

	m.saveLocked()

	return nil
}

// ListTopics returns all topics.
func (m *MemoryStorage) ListTopics(_ context.Context, nextToken string) ([]*Topic, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect all topics.
	allTopics := make([]*Topic, 0, len(m.Topics))
	for _, topic := range m.Topics {
		allTopics = append(allTopics, topic)
	}

	// Sort by ARN for consistent ordering.
	sort.Slice(allTopics, func(i, j int) bool {
		return allTopics[i].ARN < allTopics[j].ARN
	})

	// Handle pagination.
	startIdx := 0
	maxResults := 100

	if nextToken != "" {
		for i, t := range allTopics {
			if t.ARN == nextToken {
				startIdx = i

				break
			}
		}
	}

	endIdx := min(startIdx+maxResults, len(allTopics))
	result := allTopics[startIdx:endIdx]

	var newNextToken string
	if endIdx < len(allTopics) {
		newNextToken = allTopics[endIdx].ARN
	}

	return result, newNextToken, nil
}

// Subscribe creates a subscription.
func (m *MemoryStorage) Subscribe(_ context.Context, topicARN, protocol, endpoint string, attributes map[string]string) (*Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	topic, exists := m.Topics[topicARN]
	if !exists {
		return nil, &TopicError{
			Code:    "NotFound",
			Message: fmt.Sprintf("Topic does not exist: %s", topicARN),
		}
	}

	// Validate protocol.
	validProtocols := map[string]bool{
		"http": true, "https": true, "email": true, "email-json": true,
		"sms": true, "sqs": true, "application": true, "lambda": true,
		"firehose": true,
	}

	if !validProtocols[protocol] {
		return nil, &TopicError{
			Code:    "InvalidParameter",
			Message: fmt.Sprintf("Invalid parameter: Protocol %s is not supported", protocol),
		}
	}

	subscriptionARN := m.buildSubscriptionARN(topicARN)

	subscription := &Subscription{
		ARN:                    subscriptionARN,
		TopicARN:               topicARN,
		Protocol:               protocol,
		Endpoint:               endpoint,
		Owner:                  defaultAccountID,
		SubscriptionAttributes: attributes,
	}

	// For SQS and Lambda protocols, auto-confirm.
	if protocol == "sqs" || protocol == "lambda" {
		subscription.ConfirmationWasAuthenticated = true
	}

	m.Subscriptions[subscriptionARN] = subscription
	topic.Subscriptions[subscriptionARN] = subscription

	m.saveLocked()

	return subscription, nil
}

// GetSubscription returns a subscription by ARN.
func (m *MemoryStorage) GetSubscription(_ context.Context, subscriptionARN string) (*Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subscription, exists := m.Subscriptions[subscriptionARN]
	if !exists {
		return nil, &TopicError{
			Code:    "NotFound",
			Message: fmt.Sprintf("Subscription does not exist: %s", subscriptionARN),
		}
	}

	return subscription, nil
}

// SetSubscriptionAttribute sets a single attribute on a subscription.
func (m *MemoryStorage) SetSubscriptionAttribute(_ context.Context, subscriptionARN, name, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subscription, exists := m.Subscriptions[subscriptionARN]
	if !exists {
		return &TopicError{
			Code:    "NotFound",
			Message: fmt.Sprintf("Subscription does not exist: %s", subscriptionARN),
		}
	}

	if subscription.SubscriptionAttributes == nil {
		subscription.SubscriptionAttributes = make(map[string]string)
	}

	subscription.SubscriptionAttributes[name] = value

	m.saveLocked()

	return nil
}

// Unsubscribe removes a subscription.
func (m *MemoryStorage) Unsubscribe(_ context.Context, subscriptionARN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subscription, exists := m.Subscriptions[subscriptionARN]
	if !exists {
		return &TopicError{
			Code:    "NotFound",
			Message: fmt.Sprintf("Subscription does not exist: %s", subscriptionARN),
		}
	}

	// Remove from topic's subscriptions.
	if topic, exists := m.Topics[subscription.TopicARN]; exists {
		delete(topic.Subscriptions, subscriptionARN)
	}

	delete(m.Subscriptions, subscriptionARN)

	m.saveLocked()

	return nil
}

// Publish publishes a message to a topic.
func (m *MemoryStorage) Publish(ctx context.Context, topicARN, message, subject, messageGroupID, messageDeduplicationID string, attributes map[string]MessageAttribute) (string, error) {
	m.mu.RLock()

	topic, exists := m.Topics[topicARN]
	if !exists {
		m.mu.RUnlock()

		return "", &TopicError{
			Code:    "NotFound",
			Message: fmt.Sprintf("Topic does not exist: %s", topicARN),
		}
	}

	// Copy subscriptions while holding read lock.
	subscriptions := make([]*Subscription, 0, len(topic.Subscriptions))
	for _, sub := range topic.Subscriptions {
		subscriptions = append(subscriptions, sub)
	}
	m.mu.RUnlock()

	messageID := uuid.New().String()

	// Deliver to all subscriptions.
	for _, sub := range subscriptions {
		if err := m.deliverMessage(ctx, sub, message, subject, messageID, messageGroupID, messageDeduplicationID, attributes); err != nil {
			// Log error but continue delivering to other subscriptions.
			continue
		}
	}

	return messageID, nil
}

// matchesFilterPolicy checks whether the given message attributes satisfy the
// subscription's FilterPolicy.  A FilterPolicy is a JSON object whose keys
// are attribute names and whose values are arrays of allowed values.
//
// AWS supports several filter policy operators (exact match, prefix,
// anything-but, numeric, exists, etc.).  This implementation covers:
//   - exact string match  (e.g. ["billing"])
//   - "exists": true/false
//   - "prefix"
//   - "anything-but"  (string list)
//
// If the subscription has no FilterPolicy, all messages match.
func matchesFilterPolicy(filterPolicyJSON string, attributes map[string]MessageAttribute) bool {
	if filterPolicyJSON == "" {
		return true
	}

	// Parse the filter policy.
	var policy map[string][]json.RawMessage
	if err := json.Unmarshal([]byte(filterPolicyJSON), &policy); err != nil {
		// Malformed policy -- deliver to be safe (same as AWS fallback).
		return true
	}

	// Every key in the policy must match at least one condition in its array.
	for key, conditions := range policy {
		attr, exists := attributes[key]

		if !matchesConditions(conditions, attr.StringValue, exists) {
			return false
		}
	}

	return true
}

// matchesConditions evaluates a single filter-policy key's condition array.
// The attribute must satisfy at least one condition in the array.
func matchesConditions(conditions []json.RawMessage, value string, exists bool) bool {
	for _, raw := range conditions {
		if matchesSingleCondition(raw, value, exists) {
			return true
		}
	}

	return false
}

// matchesSingleCondition checks one condition entry.  It can be a plain
// string (exact match) or an object with an operator key.
func matchesSingleCondition(raw json.RawMessage, value string, exists bool) bool {
	// Try plain string (exact match).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return exists && value == s
	}

	// Try number (exact numeric match against StringValue).
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return exists && value == n.String()
	}

	// Try boolean -- the only useful boolean in a condition array is
	// a bare true/false which AWS does not actually support at the
	// top level (it needs {"exists": true/false}), but handle it
	// gracefully.
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		if b {
			return exists
		}

		return !exists
	}

	// Try object (operator form).
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}

	return matchesOperator(obj, value, exists)
}

// matchesOperator handles object-form conditions like {"exists": true},
// {"prefix": "pay"}, {"anything-but": ["x"]}.
//
// matchesOperator evaluates a single SNS filter-policy operator object. Each
// operator returns (result, matched); an unparseable operator value is treated
// as not-matched so evaluation falls through, preserving the original behavior.
func matchesOperator(obj map[string]json.RawMessage, value string, exists bool) bool {
	if result, matched := matchExists(obj, exists); matched {
		return result
	}

	if result, matched := matchPrefix(obj, value, exists); matched {
		return result
	}

	if result, matched := matchAnythingBut(obj, value, exists); matched {
		return result
	}

	return false
}

// matchExists handles {"exists": true/false}.
func matchExists(obj map[string]json.RawMessage, exists bool) (bool, bool) {
	raw, ok := obj["exists"]
	if !ok {
		return false, false
	}

	var want bool
	if err := json.Unmarshal(raw, &want); err != nil {
		return false, false
	}

	return want == exists, true
}

// matchPrefix handles {"prefix": "val"}.
func matchPrefix(obj map[string]json.RawMessage, value string, exists bool) (bool, bool) {
	raw, ok := obj["prefix"]
	if !ok {
		return false, false
	}

	var prefix string
	if err := json.Unmarshal(raw, &prefix); err != nil {
		return false, false
	}

	return exists && strings.HasPrefix(value, prefix), true
}

// matchAnythingBut handles {"anything-but": ["v1",...]} or {"anything-but": "v1"}.
func matchAnythingBut(obj map[string]json.RawMessage, value string, exists bool) (bool, bool) {
	raw, ok := obj["anything-but"]
	if !ok {
		return false, false
	}

	if !exists {
		return false, true
	}

	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, deny := range arr {
			if value == deny {
				return false, true
			}
		}

		return true, true
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return value != single, true
	}

	return false, false
}

// deliverMessage delivers a message to a subscription.
func (m *MemoryStorage) deliverMessage(ctx context.Context, sub *Subscription, message, subject, messageID, messageGroupID, messageDeduplicationID string, attributes map[string]MessageAttribute) error {
	// Check FilterPolicy before delivering.
	if sub.SubscriptionAttributes != nil {
		if fp, ok := sub.SubscriptionAttributes["FilterPolicy"]; ok {
			if !matchesFilterPolicy(fp, attributes) {
				return nil
			}
		}
	}

	switch sub.Protocol {
	case "sqs":
		if m.SqsPublisher != nil {
			// Build the message body based on RawMessageDelivery setting.
			body := message
			if !isRawMessageDelivery(sub) {
				body = buildSNSNotificationEnvelope(sub.TopicARN, message, subject, messageID, attributes)
			}

			attrs := map[string]string{
				"MessageId": messageID,
			}
			if subject != "" {
				attrs["Subject"] = subject
			}

			if err := m.SqsPublisher.PublishToSQS(ctx, sub.Endpoint, body, messageGroupID, messageDeduplicationID, attrs); err != nil {
				return fmt.Errorf("failed to publish to SQS: %w", err)
			}

			return nil
		}
	case "http", "https":
		// HTTP delivery not implemented in emulator.
		return nil
	default:
		// Other protocols not implemented.
		return nil
	}

	return nil
}

// isRawMessageDelivery checks whether the subscription has RawMessageDelivery enabled.
func isRawMessageDelivery(sub *Subscription) bool {
	if sub.SubscriptionAttributes == nil {
		return false
	}

	return sub.SubscriptionAttributes["RawMessageDelivery"] == "true"
}

// buildSNSNotificationEnvelope wraps a message in the SNS notification JSON
// envelope that AWS sends to SQS when RawMessageDelivery is not enabled.
func buildSNSNotificationEnvelope(topicARN, message, subject, messageID string, attributes map[string]MessageAttribute) string {
	now := time.Now().UTC().Format(time.RFC3339)

	envelope := snsNotificationEnvelope{
		Type:             "Notification",
		MessageID:        messageID,
		TopicArn:         topicARN,
		Message:          message,
		Timestamp:        now,
		SignatureVersion: "1",
		Signature:        "EXAMPLE",
		SigningCertURL:   "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-0000000000000000000000.pem",
		UnsubscribeURL:   fmt.Sprintf("https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=%s", topicARN),
	}

	if subject != "" {
		envelope.Subject = subject
	}

	if len(attributes) > 0 {
		envelope.MessageAttributes = make(map[string]snsNotificationAttribute, len(attributes))
		for k, v := range attributes {
			envelope.MessageAttributes[k] = snsNotificationAttribute{
				Type:  v.DataType,
				Value: v.StringValue,
			}
		}
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		// Fallback to raw message if marshaling fails.
		return message
	}

	return string(data)
}

// ListSubscriptions returns all subscriptions.
func (m *MemoryStorage) ListSubscriptions(_ context.Context, nextToken string) ([]*Subscription, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect all subscriptions.
	allSubs := make([]*Subscription, 0, len(m.Subscriptions))
	for _, sub := range m.Subscriptions {
		allSubs = append(allSubs, sub)
	}

	// Sort by ARN for consistent ordering.
	sort.Slice(allSubs, func(i, j int) bool {
		return allSubs[i].ARN < allSubs[j].ARN
	})

	// Handle pagination.
	startIdx := 0
	maxResults := 100

	if nextToken != "" {
		for i, s := range allSubs {
			if s.ARN == nextToken {
				startIdx = i

				break
			}
		}
	}

	endIdx := min(startIdx+maxResults, len(allSubs))
	result := allSubs[startIdx:endIdx]

	var newNextToken string
	if endIdx < len(allSubs) {
		newNextToken = allSubs[endIdx].ARN
	}

	return result, newNextToken, nil
}

// ListSubscriptionsByTopic returns subscriptions for a specific topic.
func (m *MemoryStorage) ListSubscriptionsByTopic(_ context.Context, topicARN, nextToken string) ([]*Subscription, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	topic, exists := m.Topics[topicARN]
	if !exists {
		return nil, "", &TopicError{
			Code:    "NotFound",
			Message: fmt.Sprintf("Topic does not exist: %s", topicARN),
		}
	}

	// Collect subscriptions for this topic.
	allSubs := make([]*Subscription, 0, len(topic.Subscriptions))
	for _, sub := range topic.Subscriptions {
		allSubs = append(allSubs, sub)
	}

	// Sort by ARN for consistent ordering.
	sort.Slice(allSubs, func(i, j int) bool {
		return allSubs[i].ARN < allSubs[j].ARN
	})

	// Handle pagination.
	startIdx := 0
	maxResults := 100

	if nextToken != "" {
		for i, s := range allSubs {
			if s.ARN == nextToken {
				startIdx = i

				break
			}
		}
	}

	endIdx := min(startIdx+maxResults, len(allSubs))
	result := allSubs[startIdx:endIdx]

	var newNextToken string
	if endIdx < len(allSubs) {
		newNextToken = allSubs[endIdx].ARN
	}

	return result, newNextToken, nil
}

// buildTopicARN builds an ARN for a topic.
func (m *MemoryStorage) buildTopicARN(name string) string {
	return fmt.Sprintf("arn:aws:sns:%s:%s:%s", defaultRegion, defaultAccountID, name)
}

// buildSubscriptionARN builds an ARN for a subscription.
func (m *MemoryStorage) buildSubscriptionARN(topicARN string) string {
	// Extract topic name from ARN.
	parts := strings.Split(topicARN, ":")
	topicName := parts[len(parts)-1]

	return fmt.Sprintf("arn:aws:sns:%s:%s:%s:%s",
		defaultRegion, defaultAccountID, topicName, uuid.New().String())
}
