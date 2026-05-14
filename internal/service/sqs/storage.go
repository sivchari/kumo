package sqs

import (
	"context"
	"crypto/md5" //nolint:gosec // MD5 is required by SQS spec for message body hash
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// DeduplicationEntry holds deduplication information for FIFO queues.
type DeduplicationEntry struct {
	MessageID string    `json:"messageId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Storage defines the interface for SQS storage operations.
type Storage interface {
	CreateQueue(ctx context.Context, name string, attributes, tags map[string]string) (*Queue, error)
	DeleteQueue(ctx context.Context, queueURL string) error
	ListQueues(ctx context.Context, prefix string) ([]string, error)
	GetQueueURL(ctx context.Context, name string) (string, error)
	GetQueue(ctx context.Context, queueURL string) (*Queue, error)
	ListQueueTags(ctx context.Context, queueURL string) (map[string]string, error)
	TagQueue(ctx context.Context, queueURL string, tags map[string]string) error
	UntagQueue(ctx context.Context, queueURL string, tagKeys []string) error
	SendMessage(ctx context.Context, queueURL, body string, delaySeconds int, messageAttributes map[string]MessageAttributeValue, messageGroupID, messageDeduplicationID string) (*Message, error)
	ReceiveMessage(ctx context.Context, queueURL string, maxMessages, visibilityTimeout, waitTimeSeconds int) ([]*Message, error)
	DeleteMessage(ctx context.Context, queueURL, receiptHandle string) error
	ChangeMessageVisibility(ctx context.Context, queueURL, receiptHandle string, visibilityTimeout int) error
	PurgeQueue(ctx context.Context, queueURL string) error
	GetQueueAttributes(ctx context.Context, queueURL string, attributeNames []string) (map[string]string, error)
	SetQueueAttributes(ctx context.Context, queueURL string, attributes map[string]string) error
}

// QueueError represents an SQS queue error.
type QueueError struct {
	Code    string
	Message string
}

func (e *QueueError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Attribute value for boolean true.
const attrValueTrue = "true"

// Common error codes.
var (
	ErrQueueAlreadyExists   = &QueueError{Code: "QueueAlreadyExists", Message: "A queue with this name already exists"}
	ErrQueueDoesNotExist    = &QueueError{Code: "AWS.SimpleQueueService.NonExistentQueue", Message: "The specified queue does not exist"}
	ErrReceiptHandleInvalid = &QueueError{Code: "ReceiptHandleIsInvalid", Message: "The receipt handle is not valid"}
	ErrMessageNotInflight   = &QueueError{Code: "MessageNotInflight", Message: "The message is not in flight"}
)

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

// MemoryStorage implements Storage using in-memory maps.
type MemoryStorage struct {
	mu      sync.RWMutex          `json:"-"`
	Queues  map[string]*QueueData `json:"queues"`
	baseURL string
	dataDir string
}

// QueueData holds all data associated with a single SQS queue.
type QueueData struct {
	Queue              *Queue                        `json:"queue"`
	Messages           []*Message                    `json:"messages"`
	Inflight           map[string]*Message           `json:"-"`               // receiptHandle -> message
	DeduplicationCache map[string]DeduplicationEntry `json:"-"`               // deduplicationID -> entry (FIFO only)
	SequenceCounter    uint64                        `json:"sequenceCounter"` // Per-queue sequence number (FIFO only)
	notify             chan struct{}                 // signals new message arrival for long polling
}

// NewMemoryStorage creates a new in-memory SQS storage.
func NewMemoryStorage(baseURL string, opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Queues:  make(map[string]*QueueData),
		baseURL: baseURL,
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "sqs", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(s)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Queues == nil {
		s.Queues = make(map[string]*QueueData)
	}

	// Non-persisted fields (json:"-" / unexported) are zero after unmarshal;
	// re-init so ReceiveMessage doesn't panic on a nil map.
	for _, qd := range s.Queues {
		if qd == nil {
			continue
		}

		if qd.Inflight == nil {
			qd.Inflight = make(map[string]*Message)
		}

		if qd.notify == nil {
			qd.notify = make(chan struct{}, 1)
		}

		if qd.Queue != nil && qd.Queue.FifoQueue && qd.DeduplicationCache == nil {
			qd.DeduplicationCache = make(map[string]DeduplicationEntry)
		}
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "sqs", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// resolveQueueData finds a queue by URL, tolerating hostname differences.
// It must be called with s.mu held.
func (s *MemoryStorage) resolveQueueData(queueURL string) (string, *QueueData, error) {
	// Fast path: exact match.
	if qd, exists := s.Queues[queueURL]; exists {
		return queueURL, qd, nil
	}

	// Slow path: match by URL path to handle hostname differences
	// (e.g., localhost:4566 vs kumo:4566).
	parsed, err := url.Parse(queueURL)
	if err != nil {
		return "", nil, ErrQueueDoesNotExist
	}

	for storedURL, qd := range s.Queues {
		storedParsed, err := url.Parse(storedURL)
		if err != nil {
			continue
		}

		if storedParsed.Path == parsed.Path {
			return storedURL, qd, nil
		}
	}

	return "", nil, ErrQueueDoesNotExist
}

// CreateQueue creates a new queue.
func (s *MemoryStorage) CreateQueue(_ context.Context, name string, attributes, tags map[string]string) (*Queue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	queueURL := fmt.Sprintf("%s/000000000000/%s", s.baseURL, name)

	if qd, exists := s.Queues[queueURL]; exists {
		return qd.Queue, nil
	}

	// Check FIFO queue requirements.
	isFifo := strings.HasSuffix(name, ".fifo")
	if attributes["FifoQueue"] == attrValueTrue && !isFifo {
		return nil, &QueueError{
			Code:    "InvalidParameterValue",
			Message: "The queue name must end with .fifo for FIFO queues",
		}
	}

	now := time.Now()
	queue := &Queue{
		Name:                      name,
		URL:                       queueURL,
		ARN:                       fmt.Sprintf("arn:aws:sqs:us-east-1:000000000000:%s", name),
		Tags:                      maps.Clone(tags),
		CreatedTimestamp:          now,
		LastModifiedTimestamp:     now,
		VisibilityTimeout:         30,
		MessageRetentionPeriod:    345600,
		DelaySeconds:              0,
		MaxMessageSize:            262144,
		ReceiveWaitTimeSeconds:    0,
		FifoQueue:                 isFifo,
		ContentBasedDeduplication: attributes["ContentBasedDeduplication"] == attrValueTrue,
	}

	// Apply attributes.
	applyQueueAttributes(queue, attributes)

	qd := &QueueData{
		Queue:    queue,
		Messages: make([]*Message, 0),
		Inflight: make(map[string]*Message),
		notify:   make(chan struct{}, 1),
	}

	if isFifo {
		qd.DeduplicationCache = make(map[string]DeduplicationEntry)
	}

	s.Queues[queueURL] = qd

	return queue, nil
}

// DeleteQueue deletes a queue.
func (s *MemoryStorage) DeleteQueue(_ context.Context, queueURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	storedURL, _, err := s.resolveQueueData(queueURL)
	if err != nil {
		return err
	}

	delete(s.Queues, storedURL)

	return nil
}

// ListQueues lists all queues.
func (s *MemoryStorage) ListQueues(_ context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	urls := make([]string, 0, len(s.Queues))

	for url, qd := range s.Queues {
		if prefix == "" || len(qd.Queue.Name) >= len(prefix) && qd.Queue.Name[:len(prefix)] == prefix {
			urls = append(urls, url)
		}
	}

	return urls, nil
}

// GetQueueURL gets the URL for a queue by name.
func (s *MemoryStorage) GetQueueURL(_ context.Context, name string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for url, qd := range s.Queues {
		if qd.Queue.Name == name {
			return url, nil
		}
	}

	return "", ErrQueueDoesNotExist
}

// GetQueue gets a queue by URL.
func (s *MemoryStorage) GetQueue(_ context.Context, queueURL string) (*Queue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return nil, err
	}

	return qd.Queue, nil
}

// ListQueueTags gets the tags for a queue.
func (s *MemoryStorage) ListQueueTags(_ context.Context, queueURL string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return nil, err
	}

	return maps.Clone(qd.Queue.Tags), nil
}

// TagQueue adds or updates tags for a queue.
func (s *MemoryStorage) TagQueue(_ context.Context, queueURL string, tags map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return err
	}

	if qd.Queue.Tags == nil {
		qd.Queue.Tags = make(map[string]string, len(tags))
	}

	maps.Copy(qd.Queue.Tags, tags)

	qd.Queue.LastModifiedTimestamp = time.Now()

	return nil
}

// UntagQueue removes tags from a queue.
func (s *MemoryStorage) UntagQueue(_ context.Context, queueURL string, tagKeys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return err
	}

	for _, key := range tagKeys {
		delete(qd.Queue.Tags, key)
	}

	qd.Queue.LastModifiedTimestamp = time.Now()

	return nil
}

// FifoResult holds the result of FIFO validation and deduplication.
type FifoResult struct {
	SequenceNumber string   `json:"sequenceNumber"`
	DedupID        string   `json:"dedupId"`
	ExistingMsg    *Message `json:"existingMsg"`
}

// validateFIFO validates FIFO queue requirements and handles deduplication.
func (qd *QueueData) validateFIFO(body, messageGroupID, messageDeduplicationID string, now time.Time) (*FifoResult, error) {
	if messageGroupID == "" {
		return nil, &QueueError{
			Code:    "MissingParameter",
			Message: "The request must contain the parameter MessageGroupId",
		}
	}

	dedupID := messageDeduplicationID
	if dedupID == "" {
		if qd.Queue.ContentBasedDeduplication {
			hash := sha256.Sum256([]byte(body))
			dedupID = hex.EncodeToString(hash[:])
		} else {
			return nil, &QueueError{
				Code:    "InvalidParameterValue",
				Message: "The queue should either have ContentBasedDeduplication enabled or MessageDeduplicationId provided explicitly",
			}
		}
	}

	// Clean up expired deduplication entries.
	for id, entry := range qd.DeduplicationCache {
		if now.After(entry.ExpiresAt) {
			delete(qd.DeduplicationCache, id)
		}
	}

	// Check deduplication cache (5-minute window).
	if entry, exists := qd.DeduplicationCache[dedupID]; exists {
		for _, msg := range qd.Messages {
			if msg.MessageID == entry.MessageID {
				return &FifoResult{ExistingMsg: msg}, nil
			}
		}
	}

	// Generate sequence number.
	qd.SequenceCounter++

	// Add to deduplication cache (5-minute TTL).
	qd.DeduplicationCache[dedupID] = DeduplicationEntry{
		MessageID: "",
		ExpiresAt: now.Add(5 * time.Minute),
	}

	return &FifoResult{
		SequenceNumber: fmt.Sprintf("%d", qd.SequenceCounter),
		DedupID:        dedupID,
	}, nil
}

// lockedMessageGroups returns the set of MessageGroupIDs that have in-flight
// messages. Returns nil for non-FIFO queues.
func (qd *QueueData) lockedMessageGroups() map[string]struct{} {
	if !qd.Queue.FifoQueue {
		return nil
	}

	locked := make(map[string]struct{})

	for _, msg := range qd.Inflight {
		if msg.MessageGroupID != "" {
			locked[msg.MessageGroupID] = struct{}{}
		}
	}

	return locked
}

// updateFIFOCache updates the deduplication cache with the message ID.
func (qd *QueueData) updateFIFOCache(dedupID, messageID string) {
	entry := qd.DeduplicationCache[dedupID]
	entry.MessageID = messageID
	qd.DeduplicationCache[dedupID] = entry
}

// SendMessage sends a message to a queue.
func (s *MemoryStorage) SendMessage(_ context.Context, queueURL, body string, delaySeconds int, messageAttributes map[string]MessageAttributeValue, messageGroupID, messageDeduplicationID string) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	delay := delaySeconds

	if delay == 0 {
		delay = qd.Queue.DelaySeconds
	}

	var sequenceNumber, dedupID string

	if qd.Queue.FifoQueue {
		result, err := qd.validateFIFO(body, messageGroupID, messageDeduplicationID, now)
		if err != nil {
			return nil, err
		}

		if result.ExistingMsg != nil {
			return result.ExistingMsg, nil
		}

		sequenceNumber = result.SequenceNumber
		dedupID = result.DedupID
	}

	// MD5 is required by SQS specification for message body hash.
	md5Hash := md5.Sum([]byte(body)) //nolint:gosec // MD5 is required by SQS spec
	msg := &Message{
		MessageID:              uuid.New().String(),
		Body:                   body,
		MD5OfBody:              hex.EncodeToString(md5Hash[:]),
		MessageAttributes:      messageAttributes,
		SentTimestamp:          now,
		VisibleAt:              now.Add(time.Duration(delay) * time.Second),
		MessageGroupID:         messageGroupID,
		MessageDeduplicationID: messageDeduplicationID,
		SequenceNumber:         sequenceNumber,
		Attributes: map[string]string{
			"SentTimestamp":                    fmt.Sprintf("%d", now.UnixMilli()),
			"ApproximateReceiveCount":          "0",
			"ApproximateFirstReceiveTimestamp": "",
		},
	}

	if qd.Queue.FifoQueue {
		qd.updateFIFOCache(dedupID, msg.MessageID)
	}

	qd.Messages = append(qd.Messages, msg)

	// Notify long-polling receivers.
	select {
	case qd.notify <- struct{}{}:
	default:
	}

	return msg, nil
}

// ReceiveMessage receives messages from a queue.
// If waitTimeSeconds > 0 and no messages are available, it waits for messages to arrive (long polling).
func (s *MemoryStorage) ReceiveMessage(ctx context.Context, queueURL string, maxMessages, visibilityTimeout, waitTimeSeconds int) ([]*Message, error) {
	// Try to receive messages immediately.
	result, notify, err := s.receiveMessagesLocked(queueURL, maxMessages, visibilityTimeout)
	if err != nil {
		return nil, err
	}

	if len(result) > 0 || waitTimeSeconds <= 0 {
		return result, nil
	}

	// Long polling: wait for messages or timeout.
	timer := time.NewTimer(time.Duration(waitTimeSeconds) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("receive message cancelled: %w", ctx.Err())
		case <-timer.C:
			return []*Message{}, nil
		case <-notify:
			result, _, err = s.receiveMessagesLocked(queueURL, maxMessages, visibilityTimeout)
			if err != nil {
				return nil, err
			}

			if len(result) > 0 {
				return result, nil
			}
		}
	}
}

// receiveMessagesLocked attempts to receive messages while holding the lock.
// Returns the notify channel for long polling if no messages are available.
func (s *MemoryStorage) receiveMessagesLocked(queueURL string, maxMessages, visibilityTimeout int) ([]*Message, <-chan struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return nil, nil, err
	}

	if visibilityTimeout == 0 {
		visibilityTimeout = qd.Queue.VisibilityTimeout
	}

	now := time.Now()

	// Re-enqueue inflight messages whose visibility timeout has expired.
	s.requeueExpiredMessages(qd, now)

	result := make([]*Message, 0, maxMessages)
	remaining := make([]*Message, 0, len(qd.Messages))

	// For FIFO queues, track which message groups are locked (have in-flight messages).
	lockedGroups := qd.lockedMessageGroups()

	for _, msg := range qd.Messages {
		if len(result) >= maxMessages {
			remaining = append(remaining, msg)

			continue
		}

		if msg.VisibleAt.After(now) {
			remaining = append(remaining, msg)

			continue
		}

		// FIFO: skip messages from groups that are locked (have in-flight messages).
		if lockedGroups != nil && msg.MessageGroupID != "" {
			if _, locked := lockedGroups[msg.MessageGroupID]; locked {
				remaining = append(remaining, msg)

				continue
			}
		}

		// Deliver the message: make invisible and add to inflight.
		if s.deliverMessage(qd, msg, now, visibilityTimeout) {
			result = append(result, msg)
		}
	}

	qd.Messages = remaining

	return result, qd.notify, nil
}

// deliverMessage makes a message invisible and adds it to inflight.
// Returns true if delivered, false if moved to DLQ. Must be called under lock.
func (s *MemoryStorage) deliverMessage(qd *QueueData, msg *Message, now time.Time, visibilityTimeout int) bool {
	msg.ReceiptHandle = uuid.New().String()
	msg.VisibleAt = now.Add(time.Duration(visibilityTimeout) * time.Second)
	msg.ReceiveCount++
	msg.Attributes["ApproximateReceiveCount"] = fmt.Sprintf("%d", msg.ReceiveCount)

	if msg.Attributes["ApproximateFirstReceiveTimestamp"] == "" {
		msg.Attributes["ApproximateFirstReceiveTimestamp"] = fmt.Sprintf("%d", now.UnixMilli())
	}

	if qd.Queue.MaxReceiveCount > 0 && msg.ReceiveCount > qd.Queue.MaxReceiveCount {
		s.moveToDeadLetterQueue(qd.Queue.DeadLetterTargetArn, msg)

		return false
	}

	qd.Inflight[msg.ReceiptHandle] = msg

	return true
}

// ChangeMessageVisibility changes the visibility timeout of an inflight message.
func (s *MemoryStorage) ChangeMessageVisibility(_ context.Context, queueURL, receiptHandle string, visibilityTimeout int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return err
	}

	msg, exists := qd.Inflight[receiptHandle]
	if !exists {
		return ErrReceiptHandleInvalid
	}

	msg.VisibleAt = time.Now().Add(time.Duration(visibilityTimeout) * time.Second)

	return nil
}

// DeleteMessage deletes a message from a queue.
func (s *MemoryStorage) DeleteMessage(_ context.Context, queueURL, receiptHandle string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return err
	}

	if _, exists := qd.Inflight[receiptHandle]; !exists {
		return ErrReceiptHandleInvalid
	}

	delete(qd.Inflight, receiptHandle)

	return nil
}

// PurgeQueue purges all messages from a queue.
func (s *MemoryStorage) PurgeQueue(_ context.Context, queueURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return err
	}

	qd.Messages = make([]*Message, 0)
	qd.Inflight = make(map[string]*Message)

	return nil
}

// GetQueueAttributes gets queue attributes.
func (s *MemoryStorage) GetQueueAttributes(_ context.Context, queueURL string, attributeNames []string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return nil, err
	}

	q := qd.Queue
	allAttrs := map[string]string{
		"QueueArn":                              q.ARN,
		"CreatedTimestamp":                      fmt.Sprintf("%d", q.CreatedTimestamp.Unix()),
		"LastModifiedTimestamp":                 fmt.Sprintf("%d", q.LastModifiedTimestamp.Unix()),
		"VisibilityTimeout":                     fmt.Sprintf("%d", q.VisibilityTimeout),
		"MessageRetentionPeriod":                fmt.Sprintf("%d", q.MessageRetentionPeriod),
		"DelaySeconds":                          fmt.Sprintf("%d", q.DelaySeconds),
		"MaximumMessageSize":                    fmt.Sprintf("%d", q.MaxMessageSize),
		"ReceiveMessageWaitTimeSeconds":         fmt.Sprintf("%d", q.ReceiveWaitTimeSeconds),
		"ApproximateNumberOfMessages":           fmt.Sprintf("%d", len(qd.Messages)),
		"ApproximateNumberOfMessagesNotVisible": fmt.Sprintf("%d", len(qd.Inflight)),
		"FifoQueue":                             fmt.Sprintf("%t", q.FifoQueue),
		"ContentBasedDeduplication":             fmt.Sprintf("%t", q.ContentBasedDeduplication),
	}

	if q.Policy != "" {
		allAttrs["Policy"] = q.Policy
	}

	if q.RedrivePolicy != "" {
		allAttrs["RedrivePolicy"] = q.RedrivePolicy
	}

	// Check if "All" is requested.
	if slices.Contains(attributeNames, "All") {
		return allAttrs, nil
	}

	result := make(map[string]string)

	for _, name := range attributeNames {
		if val, ok := allAttrs[name]; ok {
			result[name] = val
		}
	}

	return result, nil
}

// SetQueueAttributes sets queue attributes.
func (s *MemoryStorage) SetQueueAttributes(_ context.Context, queueURL string, attributes map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, qd, err := s.resolveQueueData(queueURL)
	if err != nil {
		return err
	}

	applyQueueAttributes(qd.Queue, attributes)
	qd.Queue.LastModifiedTimestamp = time.Now()

	return nil
}

func applyQueueAttributes(q *Queue, attrs map[string]string) {
	for key, val := range attrs {
		switch key {
		case "VisibilityTimeout":
			_, _ = fmt.Sscanf(val, "%d", &q.VisibilityTimeout)
		case "MessageRetentionPeriod":
			_, _ = fmt.Sscanf(val, "%d", &q.MessageRetentionPeriod)
		case "DelaySeconds":
			_, _ = fmt.Sscanf(val, "%d", &q.DelaySeconds)
		case "MaximumMessageSize":
			_, _ = fmt.Sscanf(val, "%d", &q.MaxMessageSize)
		case "ReceiveMessageWaitTimeSeconds":
			_, _ = fmt.Sscanf(val, "%d", &q.ReceiveWaitTimeSeconds)
		case "ContentBasedDeduplication":
			q.ContentBasedDeduplication = val == "true"
		case "Policy":
			q.Policy = val
		case "RedrivePolicy":
			q.RedrivePolicy = val
			parseRedrivePolicy(q, val)
		}
	}
}

// redrivePolicy is used for JSON unmarshaling of RedrivePolicy attribute.
type redrivePolicy struct {
	DeadLetterTargetArn string `json:"deadLetterTargetArn"`
	MaxReceiveCount     string `json:"maxReceiveCount"`
}

// requeueExpiredMessages moves inflight messages whose visibility timeout has
// expired back to the message queue. If the queue has a DLQ configured and the
// message's ReceiveCount has reached MaxReceiveCount, the message is moved to the
// DLQ instead. Must be called under lock.
func (s *MemoryStorage) requeueExpiredMessages(qd *QueueData, now time.Time) {
	for handle, msg := range qd.Inflight {
		if !msg.VisibleAt.Before(now) {
			continue
		}

		delete(qd.Inflight, handle)

		// Check DLQ redrive policy: if the message has already been received
		// MaxReceiveCount times, move it to the DLQ.
		if qd.Queue.MaxReceiveCount > 0 && msg.ReceiveCount >= qd.Queue.MaxReceiveCount {
			s.moveToDeadLetterQueue(qd.Queue.DeadLetterTargetArn, msg)

			continue
		}

		// Prepend the message so it is delivered first.
		qd.Messages = append([]*Message{msg}, qd.Messages...)
	}
}

// moveToDeadLetterQueue moves a message to the dead letter queue. Must be called under lock.
func (s *MemoryStorage) moveToDeadLetterQueue(dlqArn string, msg *Message) {
	if dlqArn == "" {
		return
	}

	// Find the DLQ by ARN.
	for _, qd := range s.Queues {
		if qd.Queue.ARN == dlqArn {
			// Reset message for DLQ.
			dlqMsg := &Message{
				MessageID:         msg.MessageID,
				Body:              msg.Body,
				MD5OfBody:         msg.MD5OfBody,
				Attributes:        maps.Clone(msg.Attributes),
				MessageAttributes: msg.MessageAttributes,
				SentTimestamp:     msg.SentTimestamp,
				VisibleAt:         time.Now(),
				ReceiveCount:      0,
			}

			qd.Messages = append(qd.Messages, dlqMsg)

			return
		}
	}
}

func parseRedrivePolicy(q *Queue, val string) {
	var rp redrivePolicy
	if err := json.Unmarshal([]byte(val), &rp); err != nil {
		return
	}

	q.DeadLetterTargetArn = rp.DeadLetterTargetArn
	_, _ = fmt.Sscanf(rp.MaxReceiveCount, "%d", &q.MaxReceiveCount)
}
