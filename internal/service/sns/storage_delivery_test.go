package sns

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// capturingPublisher is a fake SQSPublisher that records the arguments of
// the most recent PublishToSQS call, optionally returning an error.
type capturingPublisher struct {
	endpoint string
	body     string
	attrs    map[string]MessageAttribute
	err      error
}

func (c *capturingPublisher) PublishToSQS(_ context.Context, endpoint, body, _, _ string, attrs map[string]MessageAttribute) error {
	c.endpoint = endpoint
	c.body = body
	c.attrs = attrs

	return c.err
}

// newTopicWithSQSSubscription creates a topic with a single sqs
// subscription, wires up the given publisher, and returns the topic ARN.
func newTopicWithSQSSubscription(t *testing.T, publisher SQSPublisher, subAttrs map[string]string) (*MemoryStorage, string) {
	t.Helper()

	storage := NewMemoryStorage("http://localhost:4566")
	storage.SetSQSPublisher(publisher)

	ctx := context.Background()

	topic, err := storage.CreateTopic(ctx, "test-topic", nil)
	if err != nil {
		t.Fatalf("CreateTopic() error = %v", err)
	}

	sub, err := storage.Subscribe(ctx, topic.ARN, "sqs", "arn:aws:sqs:us-east-1:000000000000:test-queue", nil)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if subAttrs != nil {
		sub.SubscriptionAttributes = subAttrs
	}

	return storage, topic.ARN
}

func TestPublish_RawDeliveryForwardsAttributes(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{}
	storage, topicARN := newTopicWithSQSSubscription(t, publisher, map[string]string{"RawMessageDelivery": "true"})

	attributes := map[string]MessageAttribute{
		"traceId": {DataType: "String", StringValue: "abc"},
	}

	messageID, err := storage.Publish(context.Background(), topicARN, "hello", "", "", "", attributes)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	traceID, ok := publisher.attrs["traceId"]
	if !ok {
		t.Fatalf("expected captured attrs to contain traceId, got %v", publisher.attrs)
	}

	if traceID.DataType != "String" || traceID.StringValue != "abc" {
		t.Errorf("traceId attribute = %+v, want DataType=String StringValue=abc", traceID)
	}

	msgIDAttr, ok := publisher.attrs["MessageId"]
	if !ok {
		t.Fatalf("expected captured attrs to contain MessageId, got %v", publisher.attrs)
	}

	if msgIDAttr.StringValue != messageID {
		t.Errorf("MessageId attribute StringValue = %q, want %q", msgIDAttr.StringValue, messageID)
	}
}

func TestPublish_RawDeliveryPreservesTypedAttributes(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{}
	storage, topicARN := newTopicWithSQSSubscription(t, publisher, map[string]string{"RawMessageDelivery": "true"})

	attributes := map[string]MessageAttribute{
		"count": {DataType: "Number", StringValue: "42"},
		"blob":  {DataType: "Binary", BinaryValue: []byte{1, 2}},
	}

	_, err := storage.Publish(context.Background(), topicARN, "hello", "", "", "", attributes)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	count, ok := publisher.attrs["count"]
	if !ok {
		t.Fatalf("expected captured attrs to contain count, got %v", publisher.attrs)
	}

	if count.DataType != "Number" || count.StringValue != "42" {
		t.Errorf("count attribute = %+v, want DataType=Number StringValue=42", count)
	}

	blob, ok := publisher.attrs["blob"]
	if !ok {
		t.Fatalf("expected captured attrs to contain blob, got %v", publisher.attrs)
	}

	if blob.DataType != "Binary" || !bytes.Equal(blob.BinaryValue, []byte{1, 2}) {
		t.Errorf("blob attribute = %+v, want DataType=Binary BinaryValue=[1 2]", blob)
	}
}

func TestPublish_EnvelopeDeliveryDoesNotDuplicateAttributes(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{}
	// No RawMessageDelivery attribute set -> defaults to envelope mode.
	storage, topicARN := newTopicWithSQSSubscription(t, publisher, nil)

	attributes := map[string]MessageAttribute{
		"traceId": {DataType: "String", StringValue: "abc"},
	}

	_, err := storage.Publish(context.Background(), topicARN, "hello", "", "", "", attributes)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if _, ok := publisher.attrs["traceId"]; ok {
		t.Errorf("expected captured attrs to NOT contain traceId in envelope mode, got %v", publisher.attrs)
	}

	if !strings.Contains(publisher.body, `"traceId"`) {
		t.Errorf("expected envelope body to contain traceId, got %q", publisher.body)
	}
}

func TestPublish_SubscriberErrorDoesNotFailPublish(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{err: errors.New("boom")}
	storage, topicARN := newTopicWithSQSSubscription(t, publisher, map[string]string{"RawMessageDelivery": "true"})

	messageID, err := storage.Publish(context.Background(), topicARN, "hello", "", "", "", nil)
	if err != nil {
		t.Fatalf("Publish() error = %v, want nil (fire-and-forget contract)", err)
	}

	if messageID == "" {
		t.Errorf("Publish() messageID = %q, want non-empty", messageID)
	}
}
