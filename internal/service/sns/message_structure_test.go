package sns

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

const (
	testDefaultMessage = "default message"
	testSQSMessage     = "for the queue"
	testLambdaEntry    = "for the function"
	testStructuredBody = `{"default":"default message","sqs":"for the queue","lambda":"for the function"}`
)

func assertStructureError(t *testing.T, err error, wantMessage string) {
	t.Helper()

	var topicErr *TopicError
	if !errors.As(err, &topicErr) {
		t.Fatalf("expected *TopicError, got %T: %v", err, err)
	}

	if topicErr.Code != errInvalidParameter || topicErr.Message != wantMessage {
		t.Fatalf("got %s %q, want %s %q", topicErr.Code, topicErr.Message, errInvalidParameter, wantMessage)
	}
}

func TestParseJSONMessageStructure_SelectsPerProtocol(t *testing.T) {
	t.Parallel()

	messages, err := messagesForStructure(testStructuredBody, messageStructureJSON)
	if err != nil {
		t.Fatalf("messagesForStructure() error = %v", err)
	}

	cases := map[string]string{
		protocolSQS:    testSQSMessage,
		protocolLambda: testLambdaEntry,
		"http":         testDefaultMessage,
		"unknown":      testDefaultMessage,
	}

	for protocol, want := range cases {
		if got := messages.forProtocol(protocol); got != want {
			t.Errorf("forProtocol(%s) = %q, want %q", protocol, got, want)
		}
	}
}

func TestParseJSONMessageStructure_Rules(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		body     string
		protocol string
		want     string
	}{
		{"unknown key is ignored even if a protocol matches it", `{"default":"d","foo":"x"}`, "foo", "d"},
		{"non-string protocol value drops the key", `{"default":"d","sqs":{"a":1}}`, protocolSQS, "d"},
		{"explicit empty string overrides default", `{"default":"d","sqs":""}`, protocolSQS, ""},
		{"duplicate keys are accepted, last wins", `{"default":"first","default":"second"}`, protocolSQS, "second"},
		{"escapes are unescaped once", `{"default":"a\"b\\n"}`, protocolSQS, `a"b\n`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			messages, err := messagesForStructure(tc.body, messageStructureJSON)
			if err != nil {
				t.Fatalf("messagesForStructure(%s) error = %v", tc.body, err)
			}

			if got := messages.forProtocol(tc.protocol); got != tc.want {
				t.Fatalf("forProtocol(%s) = %q, want %q", tc.protocol, got, tc.want)
			}
		})
	}
}

func TestParseJSONMessageStructure_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		want string
	}{
		{"missing default", `{"sqs":"s"}`, msgStructureNoDefault},
		{"non-string default", `{"default":{"object":"test"}}`, msgStructureNoDefault},
		{"empty object", `{}`, msgStructureNoDefault},
		{"invalid json", `{"default": "x"} }`, msgStructureParseFailed},
		{"null", `null`, msgStructureParseFailed},
		{"array", `["default"]`, msgStructureParseFailed},
		{"bare string", `"default"`, msgStructureParseFailed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := messagesForStructure(tc.body, messageStructureJSON)
			assertStructureError(t, err, tc.want)
		})
	}
}

func TestMessagesForStructure_PlainMessageIsTheDefault(t *testing.T) {
	t.Parallel()

	messages, err := messagesForStructure(testStructuredBody, "")
	if err != nil {
		t.Fatalf("messagesForStructure() error = %v", err)
	}

	if got := messages.forProtocol(protocolSQS); got != testStructuredBody {
		t.Fatalf("plain message must be delivered verbatim, got %q", got)
	}
}

// newStructuredTopic creates a topic with a raw-delivery sqs subscriber and a
// lambda subscriber, wired to the given fakes.
func newStructuredTopic(t *testing.T, publisher *capturingPublisher, invoker *capturingInvoker, rawDelivery bool) (*MemoryStorage, string) {
	t.Helper()

	storage, sub := newTopicWithLambdaSubscription(t, invoker, nil)
	storage.SetSQSPublisher(publisher)

	var attrs map[string]string
	if rawDelivery {
		attrs = map[string]string{subscriptionAttrRawMessageDelivery: testAttrValueTrue}
	}

	if _, err := storage.Subscribe(context.Background(), sub.TopicARN, protocolSQS, "arn:aws:sqs:us-east-1:000000000000:q", attrs); err != nil {
		t.Fatalf("Subscribe(sqs) error = %v", err)
	}

	return storage, sub.TopicARN
}

func lambdaMessageOf(t *testing.T, invoker *capturingInvoker) string {
	t.Helper()

	if len(invoker.payloads) != 1 {
		t.Fatalf("expected one lambda invocation, got %d", len(invoker.payloads))
	}

	var event snsLambdaEvent
	if err := json.Unmarshal(invoker.payloads[0], &event); err != nil {
		t.Fatalf("decode lambda event: %v", err)
	}

	return event.Records[0].Sns.Message
}

func TestPublish_JSONStructureDeliversProtocolEntries(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{}
	invoker := &capturingInvoker{}
	storage, topicARN := newStructuredTopic(t, publisher, invoker, true)

	attributes := map[string]MessageAttribute{"team": {DataType: dataTypeString, StringValue: "infra"}}

	if _, err := storage.Publish(context.Background(), topicARN, testStructuredBody, messageStructureJSON, "", "g1", "d1", attributes); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if publisher.body != testSQSMessage || publisher.groupID != "g1" || publisher.dedupID != "d1" {
		t.Errorf("sqs delivery = body %q group %q dedup %q", publisher.body, publisher.groupID, publisher.dedupID)
	}

	if _, ok := publisher.attrs["team"]; !ok {
		t.Errorf("sqs attributes must be forwarded, got %v", publisher.attrs)
	}

	if got := lambdaMessageOf(t, invoker); got != testLambdaEntry {
		t.Errorf("lambda Message = %q, want %q", got, testLambdaEntry)
	}
}

func TestPublish_JSONStructureFallsBackToDefault(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{}
	invoker := &capturingInvoker{}
	storage, topicARN := newStructuredTopic(t, publisher, invoker, true)

	if _, err := storage.Publish(context.Background(), topicARN, `{"default":"default message"}`, messageStructureJSON, "", "", "", nil); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if publisher.body != testDefaultMessage || lambdaMessageOf(t, invoker) != testDefaultMessage {
		t.Errorf("both subscribers must receive the default entry: sqs=%q", publisher.body)
	}
}

func TestPublish_JSONStructureEnvelopeCarriesSelectedMessage(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{}
	storage, topicARN := newStructuredTopic(t, publisher, &capturingInvoker{}, false)

	if _, err := storage.Publish(context.Background(), topicARN, testStructuredBody, messageStructureJSON, "", "", "", nil); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	var envelope snsNotificationEnvelope
	if err := json.Unmarshal([]byte(publisher.body), &envelope); err != nil {
		t.Fatalf("sqs body is not an envelope: %v\n%s", err, publisher.body)
	}

	if envelope.Message != testSQSMessage {
		t.Fatalf("envelope Message = %q, want %q", envelope.Message, testSQSMessage)
	}
}

func TestPublish_JSONStructureValidationFailsBeforeDelivery(t *testing.T) {
	t.Parallel()

	publisher := &capturingPublisher{}
	invoker := &capturingInvoker{}
	storage, topicARN := newStructuredTopic(t, publisher, invoker, true)

	messageID, err := storage.Publish(context.Background(), topicARN, `{"sqs":"no default"}`, messageStructureJSON, "", "", "", nil)
	assertStructureError(t, err, msgStructureNoDefault)

	if messageID != "" || publisher.calls != 0 || len(invoker.arns) != 0 {
		t.Fatalf("nothing may be delivered after a validation failure: id=%q sqs=%d lambda=%d", messageID, publisher.calls, len(invoker.arns))
	}
}

func TestPublish_JSONStructureIsValidatedBeforeTopicLookup(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage("http://localhost:4566")

	_, err := storage.Publish(context.Background(), "arn:aws:sns:us-east-1:000000000000:missing", `not json`, messageStructureJSON, "", "", "", nil)
	assertStructureError(t, err, msgStructureParseFailed)
}
