package sns

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

const (
	testLambdaARN     = "arn:aws:lambda:us-east-1:000000000000:function:fn"
	testLambdaMessage = "hello lambda"
	testLambdaSubject = "hello subject"
	lambdaTimestamp   = "2006-01-02T15:04:05.000Z"
)

// capturingInvoker is a fake LambdaInvoker that records every InvokeAsync
// call, optionally returning an error.
type capturingInvoker struct {
	arns     []string
	payloads [][]byte
	err      error
}

func (c *capturingInvoker) InvokeAsync(_ context.Context, functionArn string, payload []byte) error {
	c.arns = append(c.arns, functionArn)
	c.payloads = append(c.payloads, payload)

	return c.err
}

// newTopicWithLambdaSubscription creates a topic with one lambda
// subscription and wires the invoker (nil leaves the storage unwired).
func newTopicWithLambdaSubscription(t *testing.T, invoker LambdaInvoker, subAttrs map[string]string) (*MemoryStorage, *Subscription) {
	t.Helper()

	storage := NewMemoryStorage("http://localhost:4566")
	if invoker != nil {
		storage.SetLambdaInvoker(invoker)
	}

	ctx := context.Background()

	topic, err := storage.CreateTopic(ctx, "lambda-topic", nil, nil)
	if err != nil {
		t.Fatalf("CreateTopic() error = %v", err)
	}

	sub, err := storage.Subscribe(ctx, topic.ARN, protocolLambda, testLambdaARN, nil)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if subAttrs != nil {
		sub.SubscriptionAttributes = subAttrs
	}

	return storage, sub
}

// publishToLambda publishes one message and returns the message id plus the
// decoded Lambda event the invoker received.
func publishToLambda(t *testing.T, subject string, attributes map[string]MessageAttribute) (string, *Subscription, map[string]any) {
	t.Helper()

	invoker := &capturingInvoker{}
	storage, sub := newTopicWithLambdaSubscription(t, invoker, nil)

	messageID, err := storage.Publish(context.Background(), sub.TopicARN, testLambdaMessage, subject, "", "", attributes)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if len(invoker.arns) != 1 || invoker.arns[0] != testLambdaARN {
		t.Fatalf("expected one invocation of %s, got %v", testLambdaARN, invoker.arns)
	}

	var event map[string]any
	if err := json.Unmarshal(invoker.payloads[0], &event); err != nil {
		t.Fatalf("payload is not JSON: %v\n%s", err, invoker.payloads[0])
	}

	return messageID, sub, event
}

// snsEntity returns Records[0].Sns of a decoded event.
func snsEntity(t *testing.T, event map[string]any) (map[string]any, map[string]any) {
	t.Helper()

	records, ok := event["Records"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("expected exactly one record, got %v", event["Records"])
	}

	record, ok := records[0].(map[string]any)
	if !ok {
		t.Fatalf("record is not an object: %v", records[0])
	}

	entity, ok := record["Sns"].(map[string]any)
	if !ok {
		t.Fatalf("Sns is not an object: %v", record["Sns"])
	}

	return record, entity
}

func TestPublish_LambdaEventRecordShape(t *testing.T) {
	t.Parallel()

	_, sub, event := publishToLambda(t, testLambdaSubject, nil)
	record, _ := snsEntity(t, event)

	if record["EventSource"] != "aws:sns" || record["EventVersion"] != "1.0" {
		t.Fatalf("unexpected record header: %v", record)
	}

	if record["EventSubscriptionArn"] != sub.ARN {
		t.Fatalf("EventSubscriptionArn = %v, want %s", record["EventSubscriptionArn"], sub.ARN)
	}
}

func TestPublish_LambdaEventSnsFields(t *testing.T) {
	t.Parallel()

	attributes := map[string]MessageAttribute{"orderId": {DataType: dataTypeString, StringValue: testTraceValueAbc}}
	messageID, sub, event := publishToLambda(t, testLambdaSubject, attributes)
	_, entity := snsEntity(t, event)

	want := map[string]any{
		"Type":             "Notification",
		"MessageId":        messageID,
		"TopicArn":         sub.TopicARN,
		"Subject":          testLambdaSubject,
		"Message":          testLambdaMessage,
		"SignatureVersion": "1",
	}

	for key, value := range want {
		if entity[key] != value {
			t.Errorf("Sns.%s = %v, want %v", key, entity[key], value)
		}
	}

	for _, key := range []string{"Signature", "SigningCertUrl", "UnsubscribeUrl", "Timestamp"} {
		if _, ok := entity[key].(string); !ok {
			t.Errorf("Sns.%s missing or not a string: %v", key, entity[key])
		}
	}

	for _, key := range []string{"SigningCertURL", "UnsubscribeURL"} {
		if _, ok := entity[key]; ok {
			t.Errorf("Sns must use the Lambda spelling, found %s", key)
		}
	}

	unsubscribe, _ := entity["UnsubscribeUrl"].(string)
	if !strings.Contains(unsubscribe, sub.ARN) {
		t.Errorf("UnsubscribeUrl %q should reference the subscription ARN %s", unsubscribe, sub.ARN)
	}

	attrs, ok := entity["MessageAttributes"].(map[string]any)
	if !ok {
		t.Fatalf("MessageAttributes missing: %v", entity["MessageAttributes"])
	}

	orderID, _ := attrs["orderId"].(map[string]any)
	if orderID["Type"] != dataTypeString || orderID["Value"] != testTraceValueAbc {
		t.Errorf("MessageAttributes.orderId = %v", attrs["orderId"])
	}
}

func TestPublish_LambdaEventTimestampHasMilliseconds(t *testing.T) {
	t.Parallel()

	_, _, event := publishToLambda(t, "", nil)
	_, entity := snsEntity(t, event)

	timestamp, _ := entity["Timestamp"].(string)
	if _, err := time.Parse(lambdaTimestamp, timestamp); err != nil {
		t.Fatalf("Timestamp %q does not match %s: %v", timestamp, lambdaTimestamp, err)
	}

	if !regexp.MustCompile(`\.\d{3}Z$`).MatchString(timestamp) {
		t.Fatalf("Timestamp %q must carry exactly three fractional digits", timestamp)
	}
}

func TestPublish_LambdaEventSubjectIsNullWhenAbsent(t *testing.T) {
	t.Parallel()

	_, _, event := publishToLambda(t, "", nil)
	_, entity := snsEntity(t, event)

	subject, present := entity["Subject"]
	if !present || subject != nil {
		t.Fatalf("Subject = %v (present=%v), want an explicit null", subject, present)
	}
}

func TestPublish_LambdaEventMessageAttributesIsEmptyObjectWhenAbsent(t *testing.T) {
	t.Parallel()

	_, _, event := publishToLambda(t, "", nil)
	_, entity := snsEntity(t, event)

	attrs, ok := entity["MessageAttributes"].(map[string]any)
	if !ok || len(attrs) != 0 {
		t.Fatalf("MessageAttributes = %v, want {}", entity["MessageAttributes"])
	}
}

func TestPublish_LambdaEventConvertsAttributeTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		attribute MessageAttribute
		wantType  string
		wantValue string
	}{
		{"string", MessageAttribute{DataType: dataTypeString, StringValue: "v"}, dataTypeString, "v"},
		{"missing type defaults to string", MessageAttribute{StringValue: "v"}, dataTypeString, "v"},
		{"number is passed as string", MessageAttribute{DataType: dataTypeNumber, StringValue: "42"}, dataTypeString, "42"},
		{"string array is passed as string", MessageAttribute{DataType: dataTypeStringArray, StringValue: `["a","b"]`}, dataTypeString, `["a","b"]`},
		{"binary is base64", MessageAttribute{DataType: dataTypeBinary, BinaryValue: []byte("hi")}, dataTypeBinary, "aGk="},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, event := publishToLambda(t, "", map[string]MessageAttribute{"attr": tc.attribute})
			_, entity := snsEntity(t, event)

			attrs, _ := entity["MessageAttributes"].(map[string]any)

			got, _ := attrs["attr"].(map[string]any)
			if got["Type"] != tc.wantType || got["Value"] != tc.wantValue {
				t.Fatalf("attr = %v, want Type=%s Value=%s", got, tc.wantType, tc.wantValue)
			}
		})
	}
}

func TestPublish_LambdaFilterPolicyMismatchSkipsInvocation(t *testing.T) {
	t.Parallel()

	invoker := &capturingInvoker{}
	storage, sub := newTopicWithLambdaSubscription(t, invoker, map[string]string{"FilterPolicy": `{"kind":["billing"]}`})

	attributes := map[string]MessageAttribute{"kind": {DataType: dataTypeString, StringValue: "audit"}}
	if _, err := storage.Publish(context.Background(), sub.TopicARN, testLambdaMessage, "", "", "", attributes); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if len(invoker.arns) != 0 {
		t.Fatalf("expected no invocation for a filtered-out message, got %v", invoker.arns)
	}
}

func TestPublish_LambdaWithoutInvokerIsANoop(t *testing.T) {
	t.Parallel()

	storage, sub := newTopicWithLambdaSubscription(t, nil, nil)

	messageID, err := storage.Publish(context.Background(), sub.TopicARN, testLambdaMessage, "", "", "", nil)
	if err != nil || messageID == "" {
		t.Fatalf("Publish() = %q, %v; want a message id and no error", messageID, err)
	}
}

func TestPublish_LambdaInvokerErrorDoesNotFailPublish(t *testing.T) {
	t.Parallel()

	invoker := &capturingInvoker{err: errors.New("boom")}
	storage, sub := newTopicWithLambdaSubscription(t, invoker, nil)

	messageID, err := storage.Publish(context.Background(), sub.TopicARN, testLambdaMessage, "", "", "", nil)
	if err != nil || messageID == "" {
		t.Fatalf("Publish() = %q, %v; delivery failures must not fail Publish", messageID, err)
	}
}

func TestPublish_DeliversToSQSAndLambdaSubscribers(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{}
	invoker := &capturingInvoker{}
	storage, sub := newTopicWithLambdaSubscription(t, invoker, nil)
	storage.SetSQSPublisher(publisher)

	if _, err := storage.Subscribe(context.Background(), sub.TopicARN, protocolSQS, "arn:aws:sqs:us-east-1:000000000000:q", nil); err != nil {
		t.Fatalf("Subscribe(sqs) error = %v", err)
	}

	if _, err := storage.Publish(context.Background(), sub.TopicARN, testLambdaMessage, "", "", "", nil); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if len(invoker.arns) != 1 || publisher.body == "" {
		t.Fatalf("expected both subscribers to be delivered: lambda=%v sqs=%q", invoker.arns, publisher.body)
	}
}

func TestMemoryStorage_SnapshotExcludesLambdaInvokerAndRestoresDelivery(t *testing.T) {
	t.Parallel()

	storage, sub := newTopicWithLambdaSubscription(t, &capturingInvoker{}, nil)

	snapshot, err := json.Marshal(storage)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	if strings.Contains(strings.ToLower(string(snapshot)), "invoker") {
		t.Fatalf("snapshot must not persist the invoker: %s", snapshot)
	}

	restored := NewMemoryStorage("http://localhost:4566")
	if err := json.Unmarshal(snapshot, restored); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	invoker := &capturingInvoker{}
	restored.SetLambdaInvoker(invoker)

	if _, err := restored.Publish(context.Background(), sub.TopicARN, testLambdaMessage, "", "", "", nil); err != nil {
		t.Fatalf("Publish() on restored storage error = %v", err)
	}

	if len(invoker.arns) != 1 {
		t.Fatalf("restored storage should deliver to the lambda subscription, got %v", invoker.arns)
	}
}
