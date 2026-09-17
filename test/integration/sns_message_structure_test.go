//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/smithy-go"
)

const (
	structuredMessageBody = `{"default":"fallback","sqs":"for the queue","lambda":"for the function"}`
	messageStructureJSON  = "json"
)

// subscribeRawQueue creates an SQS queue, subscribes it to topicARN with
// RawMessageDelivery and returns the queue URL.
func subscribeRawQueue(t *testing.T, snsClient *sns.Client, topicARN, queueName string) string {
	t.Helper()

	sqsClient := newSQSClient(t)
	ctx := t.Context()

	queue, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(queueName)})
	if err != nil {
		t.Fatalf("CreateQueue: %v", err)
	}

	t.Cleanup(func() {
		_, _ = sqsClient.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{QueueUrl: queue.QueueUrl})
	})

	sub, err := snsClient.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: aws.String(topicARN),
		Protocol: aws.String("sqs"),
		Endpoint: aws.String("arn:aws:sqs:us-east-1:000000000000:" + queueName),
	})
	if err != nil {
		t.Fatalf("Subscribe(sqs): %v", err)
	}

	t.Cleanup(func() {
		_, _ = snsClient.Unsubscribe(context.Background(), &sns.UnsubscribeInput{SubscriptionArn: sub.SubscriptionArn})
	})

	if _, err := snsClient.SetSubscriptionAttributes(ctx, &sns.SetSubscriptionAttributesInput{
		SubscriptionArn: sub.SubscriptionArn,
		AttributeName:   aws.String("RawMessageDelivery"),
		AttributeValue:  aws.String("true"),
	}); err != nil {
		t.Fatalf("SetSubscriptionAttributes: %v", err)
	}

	return *queue.QueueUrl
}

func assertSNSInvalidParameter(t *testing.T, err error, wantMessage string) {
	t.Helper()

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an API error, got %v", err)
	}

	if apiErr.ErrorCode() != "InvalidParameter" || apiErr.ErrorMessage() != wantMessage {
		t.Fatalf("got %s %q, want InvalidParameter %q", apiErr.ErrorCode(), apiErr.ErrorMessage(), wantMessage)
	}

	var respErr *awshttp.ResponseError
	if !errors.As(err, &respErr) || respErr.HTTPStatusCode() != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400, got %v", err)
	}
}

func TestSNS_PublishJSONStructureSelectsProtocolEntries(t *testing.T) {
	fixture := newLambdaSubscriberTopic(t, "sns-structure-subscriber", "test-sns-structure-topic")
	queueURL := subscribeRawQueue(t, fixture.client, fixture.topicARN, "test-sns-structure-queue")

	if _, err := fixture.client.Publish(t.Context(), &sns.PublishInput{
		TopicArn:         aws.String(fixture.topicARN),
		Message:          aws.String(structuredMessageBody),
		MessageStructure: aws.String(messageStructureJSON),
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if record := waitForLambdaEvent(t, fixture.received); record.Sns.Message != "for the function" {
		t.Errorf("lambda Message = %q, want the lambda entry", record.Sns.Message)
	}

	received, err := newSQSClient(t).ReceiveMessage(t.Context(), &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     5,
	})
	if err != nil {
		t.Fatalf("ReceiveMessage: %v", err)
	}

	if len(received.Messages) != 1 || aws.ToString(received.Messages[0].Body) != "for the queue" {
		t.Errorf("sqs delivery = %+v, want the sqs entry", received.Messages)
	}
}

func TestSNS_PublishJSONStructureRejectsInvalidBodies(t *testing.T) {
	client := newSNSClient(t)

	topic, err := client.CreateTopic(t.Context(), &sns.CreateTopicInput{Name: aws.String("test-sns-structure-errors")})
	if err != nil {
		t.Fatalf("CreateTopic: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTopic(context.Background(), &sns.DeleteTopicInput{TopicArn: topic.TopicArn})
	})

	cases := []struct {
		name, message, want string
	}{
		{"missing default", `{"sqs":"only the queue"}`, "Invalid parameter: Message Structure - No default entry in JSON message body"},
		{"non-string default", `{"default":{"object":"test"}}`, "Invalid parameter: Message Structure - No default entry in JSON message body"},
		{"invalid json", `{"default": "x"} }`, "Invalid parameter: Message Structure - JSON message body failed to parse"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.Publish(t.Context(), &sns.PublishInput{
				TopicArn:         topic.TopicArn,
				Message:          aws.String(tc.message),
				MessageStructure: aws.String(messageStructureJSON),
			})
			assertSNSInvalidParameter(t, err, tc.want)
		})
	}
}

func TestSNS_PublishJSONStructureIsValidatedBeforeTopicLookup(t *testing.T) {
	_, err := newSNSClient(t).Publish(t.Context(), &sns.PublishInput{
		TopicArn:         aws.String("arn:aws:sns:us-east-1:000000000000:does-not-exist"),
		Message:          aws.String("not json"),
		MessageStructure: aws.String(messageStructureJSON),
	})
	assertSNSInvalidParameter(t, err, "Invalid parameter: Message Structure - JSON message body failed to parse")
}
