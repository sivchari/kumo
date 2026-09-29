package sns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const (
	testAttrValueTrue = "true"
	testTraceValueAbc = "abc"
)

// capturingPublisher is a fake SQSPublisher that records the arguments of
// the most recent PublishToSQS call, optionally returning an error.
type capturingPublisher struct {
	endpoint string
	body     string
	groupID  string
	dedupID  string
	attrs    map[string]MessageAttribute
	calls    int
	err      error
}

func (c *capturingPublisher) PublishToSQS(_ context.Context, endpoint, body, groupID, dedupID string, attrs map[string]MessageAttribute) error {
	c.endpoint = endpoint
	c.body = body
	c.groupID = groupID
	c.dedupID = dedupID
	c.attrs = attrs
	c.calls++

	return c.err
}

// newTopicWithSQSSubscription creates a topic with a single sqs
// subscription, wires up the given publisher, and returns the topic ARN.
func newTopicWithSQSSubscription(t *testing.T, publisher SQSPublisher, subAttrs map[string]string) (*MemoryStorage, string) {
	t.Helper()

	storage := NewMemoryStorage("http://localhost:4566")
	storage.SetSQSPublisher(publisher)

	ctx := context.Background()

	topic, err := storage.CreateTopic(ctx, "test-topic", nil, nil)
	if err != nil {
		t.Fatalf("CreateTopic() error = %v", err)
	}

	sub, err := storage.Subscribe(ctx, topic.ARN, protocolSQS, "arn:aws:sqs:us-east-1:000000000000:test-queue", nil)
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
	storage, topicARN := newTopicWithSQSSubscription(t, publisher, map[string]string{subscriptionAttrRawMessageDelivery: testAttrValueTrue})

	attributes := map[string]MessageAttribute{
		"traceId": {DataType: dataTypeString, StringValue: testTraceValueAbc},
	}

	messageID, err := storage.Publish(context.Background(), topicARN, "hello", "", "", "", "", attributes)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	traceID, ok := publisher.attrs["traceId"]
	if !ok {
		t.Fatalf("expected captured attrs to contain traceId, got %v", publisher.attrs)
	}

	if traceID.DataType != dataTypeString || traceID.StringValue != testTraceValueAbc {
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
	storage, topicARN := newTopicWithSQSSubscription(t, publisher, map[string]string{subscriptionAttrRawMessageDelivery: testAttrValueTrue})

	attributes := map[string]MessageAttribute{
		"count": {DataType: "Number", StringValue: "42"},
		"blob":  {DataType: "Binary", BinaryValue: []byte{1, 2}},
	}

	_, err := storage.Publish(context.Background(), topicARN, "hello", "", "", "", "", attributes)
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
		"traceId": {DataType: dataTypeString, StringValue: testTraceValueAbc},
	}

	_, err := storage.Publish(context.Background(), topicARN, "hello", "", "", "", "", attributes)
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

func TestPublish_EnvelopeUnsubscribeURLUsesSubscriptionARN(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{}
	// No RawMessageDelivery attribute -> envelope mode.
	storage, topicARN := newTopicWithSQSSubscription(t, publisher, nil)

	ctx := context.Background()

	subs, _, err := storage.ListSubscriptionsByTopic(ctx, topicARN, "")
	if err != nil {
		t.Fatalf("ListSubscriptionsByTopic() error = %v", err)
	}

	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}

	sub := subs[0]

	// Give the subscription an ARN in a distinct region to verify the
	// UnsubscribeURL host is derived from the subscription's own region,
	// not the topic's or a hardcoded default.
	sub.ARN = "arn:aws:sns:ap-northeast-1:000000000000:test-topic:11111111-1111-1111-1111-111111111111"

	if _, err := storage.Publish(ctx, topicARN, "hello", "", "", "", "", nil); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	var envelope snsNotificationEnvelope
	if err := json.Unmarshal([]byte(publisher.body), &envelope); err != nil {
		t.Fatalf("sqs body is not an envelope: %v\n%s", err, publisher.body)
	}

	wantURL := "https://sns.ap-northeast-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=" + sub.ARN
	if envelope.UnsubscribeURL != wantURL {
		t.Errorf("envelope.UnsubscribeURL = %q, want %q", envelope.UnsubscribeURL, wantURL)
	}

	if envelope.TopicArn != topicARN {
		t.Errorf("envelope.TopicArn = %q, want %q", envelope.TopicArn, topicARN)
	}
}

func TestPublish_SubscriberErrorDoesNotFailPublish(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{err: errors.New("boom")}
	storage, topicARN := newTopicWithSQSSubscription(t, publisher, map[string]string{subscriptionAttrRawMessageDelivery: testAttrValueTrue})

	messageID, err := storage.Publish(context.Background(), topicARN, "hello", "", "", "", "", nil)
	if err != nil {
		t.Fatalf("Publish() error = %v, want nil (fire-and-forget contract)", err)
	}

	if messageID == "" {
		t.Errorf("Publish() messageID = %q, want non-empty", messageID)
	}
}
