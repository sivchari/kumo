//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// snsLambdaRecord is one Records[] entry of the event SNS delivers to Lambda.
type snsLambdaRecord struct {
	EventSource          string `json:"EventSource"`
	EventVersion         string `json:"EventVersion"`
	EventSubscriptionArn string `json:"EventSubscriptionArn"`
	Sns                  struct {
		Type              string  `json:"Type"`
		MessageID         string  `json:"MessageId"`
		TopicArn          string  `json:"TopicArn"`
		Subject           *string `json:"Subject"`
		Message           string  `json:"Message"`
		Timestamp         string  `json:"Timestamp"`
		SignatureVersion  string  `json:"SignatureVersion"`
		SigningCertURL    string  `json:"SigningCertUrl"`
		UnsubscribeURL    string  `json:"UnsubscribeUrl"`
		MessageAttributes map[string]struct {
			Type  string `json:"Type"`
			Value string `json:"Value"`
		} `json:"MessageAttributes"`
	} `json:"Sns"`
}

// lambdaSubscriberTopic is a topic with one lambda subscription whose
// function forwards every invocation payload to received.
type lambdaSubscriberTopic struct {
	client   *sns.Client
	topicARN string
	subARN   string
	received chan []byte
}

func newLambdaSubscriberTopic(t *testing.T, functionName, topicName string) lambdaSubscriberTopic {
	t.Helper()

	received := make(chan []byte, 4)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(target.Close)

	createLambdaWithEndpoint(t, functionName, target.URL)

	client := newSNSClient(t)
	ctx := t.Context()

	topic, err := client.CreateTopic(ctx, &sns.CreateTopicInput{Name: aws.String(topicName)})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTopic(context.Background(), &sns.DeleteTopicInput{TopicArn: topic.TopicArn})
	})

	sub, err := client.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn:              topic.TopicArn,
		Protocol:              aws.String("lambda"),
		Endpoint:              aws.String("arn:aws:lambda:us-east-1:000000000000:function:" + functionName),
		ReturnSubscriptionArn: true,
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.Unsubscribe(context.Background(), &sns.UnsubscribeInput{SubscriptionArn: sub.SubscriptionArn})
	})

	return lambdaSubscriberTopic{client: client, topicARN: *topic.TopicArn, subARN: *sub.SubscriptionArn, received: received}
}

// waitForLambdaEvent waits for the next invocation and decodes its single record.
func waitForLambdaEvent(t *testing.T, received <-chan []byte) snsLambdaRecord {
	t.Helper()

	var body []byte

	select {
	case body = <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("expected SNS Publish to invoke the lambda subscription, but it was never called")
	}

	var event struct {
		Records []snsLambdaRecord `json:"Records"`
	}

	if err := json.Unmarshal(body, &event); err != nil {
		t.Fatalf("decode lambda event %s: %v", body, err)
	}

	if len(event.Records) != 1 {
		t.Fatalf("expected exactly one record, got %d: %s", len(event.Records), body)
	}

	return event.Records[0]
}

func TestSNS_PublishInvokesLambdaSubscription(t *testing.T) {
	fixture := newLambdaSubscriberTopic(t, "sns-lambda-subscriber", "test-sns-lambda-topic")

	published, err := fixture.client.Publish(t.Context(), &sns.PublishInput{
		TopicArn: aws.String(fixture.topicARN),
		Message:  aws.String("hello lambda"),
		Subject:  aws.String("greeting"),
		MessageAttributes: map[string]snstypes.MessageAttributeValue{
			"count": {DataType: aws.String("Number"), StringValue: aws.String("42")},
			"blob":  {DataType: aws.String("Binary"), BinaryValue: []byte("hi")},
		},
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	record := waitForLambdaEvent(t, fixture.received)

	if record.EventSource != "aws:sns" || record.EventVersion != "1.0" || record.EventSubscriptionArn != fixture.subARN {
		t.Errorf("record header = %+v, want aws:sns / 1.0 / %s", record, fixture.subARN)
	}

	entity := record.Sns
	if entity.Type != "Notification" || entity.MessageID != *published.MessageId || entity.TopicArn != fixture.topicARN {
		t.Errorf("Sns = %+v, want Notification / %s / %s", entity, *published.MessageId, fixture.topicARN)
	}

	if entity.Subject == nil || *entity.Subject != "greeting" || entity.Message != "hello lambda" {
		t.Errorf("Sns subject/message = %v / %q", entity.Subject, entity.Message)
	}

	if entity.SignatureVersion != "1" || entity.SigningCertURL == "" || !strings.Contains(entity.UnsubscribeURL, fixture.subARN) {
		t.Errorf("Sns signature fields = %q / %q / %q", entity.SignatureVersion, entity.SigningCertURL, entity.UnsubscribeURL)
	}

	if _, err := time.Parse("2006-01-02T15:04:05.000Z", entity.Timestamp); err != nil {
		t.Errorf("Timestamp %q: %v", entity.Timestamp, err)
	}

	if count := entity.MessageAttributes["count"]; count.Type != "String" || count.Value != "42" {
		t.Errorf("Number attribute must be delivered as String: %+v", entity.MessageAttributes)
	}

	if blob := entity.MessageAttributes["blob"]; blob.Type != "Binary" || blob.Value != "aGk=" {
		t.Errorf("Binary attribute must be delivered base64-encoded: %+v", entity.MessageAttributes)
	}
}

func TestSNS_PublishWithoutSubjectSendsNullSubjectToLambda(t *testing.T) {
	fixture := newLambdaSubscriberTopic(t, "sns-lambda-subscriber-nosubject", "test-sns-lambda-nosubject")

	if _, err := fixture.client.Publish(t.Context(), &sns.PublishInput{
		TopicArn: aws.String(fixture.topicARN),
		Message:  aws.String("no subject"),
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	record := waitForLambdaEvent(t, fixture.received)

	if record.Sns.Subject != nil {
		t.Errorf("Subject = %q, want null", *record.Sns.Subject)
	}

	if record.Sns.MessageAttributes == nil {
		t.Error("MessageAttributes must be present as an empty object")
	}
}
