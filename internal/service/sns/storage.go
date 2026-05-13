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

// deliverMessage delivers a message to a subscription.
func (m *MemoryStorage) deliverMessage(ctx context.Context, sub *Subscription, message, subject, messageID, messageGroupID, messageDeduplicationID string, _ map[string]MessageAttribute) error {
	switch sub.Protocol {
	case "sqs":
		if m.SqsPublisher != nil {
			attrs := map[string]string{
				"MessageId": messageID,
			}
			if subject != "" {
				attrs["Subject"] = subject
			}

			if err := m.SqsPublisher.PublishToSQS(ctx, sub.Endpoint, message, messageGroupID, messageDeduplicationID, attrs); err != nil {
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
