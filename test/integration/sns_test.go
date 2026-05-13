//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/sivchari/golden"
)

func newSNSClient(t *testing.T) *sns.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return sns.NewFromConfig(cfg, func(o *sns.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestSNS_CreateAndDeleteTopic(t *testing.T) {
	client := newSNSClient(t)
	ctx := t.Context()
	topicName := "test-topic-create-delete"

	// Create topic.
	createOutput, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("TopicArn", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Delete topic.
	_, err = client.DeleteTopic(ctx, &sns.DeleteTopicInput{
		TopicArn: createOutput.TopicArn,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSNS_ListTopics(t *testing.T) {
	client := newSNSClient(t)
	ctx := t.Context()
	topicName := "test-topic-list"

	// Create topic.
	createOutput, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTopic(context.Background(), &sns.DeleteTopicInput{
			TopicArn: createOutput.TopicArn,
		})
	})

	// List topics.
	listOutput, err := client.ListTopics(ctx, &sns.ListTopicsInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, topic := range listOutput.Topics {
		if topic.TopicArn != nil && *topic.TopicArn == *createOutput.TopicArn {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("topic %s not found in list", *createOutput.TopicArn)
	}
}

func TestSNS_SubscribeAndUnsubscribe(t *testing.T) {
	client := newSNSClient(t)
	ctx := t.Context()
	topicName := "test-topic-subscribe"

	// Create topic.
	createOutput, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTopic(context.Background(), &sns.DeleteTopicInput{
			TopicArn: createOutput.TopicArn,
		})
	})

	// Subscribe.
	subscribeOutput, err := client.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: createOutput.TopicArn,
		Protocol: aws.String("sqs"),
		Endpoint: aws.String("arn:aws:sqs:us-east-1:000000000000:test-queue"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("SubscriptionArn", "ResultMetadata")).Assert(t.Name()+"_subscribe", subscribeOutput)

	// Unsubscribe.
	_, err = client.Unsubscribe(ctx, &sns.UnsubscribeInput{
		SubscriptionArn: subscribeOutput.SubscriptionArn,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSNS_ListSubscriptions(t *testing.T) {
	client := newSNSClient(t)
	ctx := t.Context()
	topicName := "test-topic-list-subs"

	// Create topic.
	createOutput, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTopic(context.Background(), &sns.DeleteTopicInput{
			TopicArn: createOutput.TopicArn,
		})
	})

	// Subscribe.
	subscribeOutput, err := client.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: createOutput.TopicArn,
		Protocol: aws.String("sqs"),
		Endpoint: aws.String("arn:aws:sqs:us-east-1:000000000000:test-queue"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.Unsubscribe(context.Background(), &sns.UnsubscribeInput{
			SubscriptionArn: subscribeOutput.SubscriptionArn,
		})
	})

	// List subscriptions.
	listOutput, err := client.ListSubscriptions(ctx, &sns.ListSubscriptionsInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, sub := range listOutput.Subscriptions {
		if sub.SubscriptionArn != nil && *sub.SubscriptionArn == *subscribeOutput.SubscriptionArn {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("subscription %s not found in list", *subscribeOutput.SubscriptionArn)
	}
}

func TestSNS_ListSubscriptionsByTopic(t *testing.T) {
	client := newSNSClient(t)
	ctx := t.Context()
	topicName := "test-topic-list-subs-by-topic"

	// Create topic.
	createOutput, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTopic(context.Background(), &sns.DeleteTopicInput{
			TopicArn: createOutput.TopicArn,
		})
	})

	// Subscribe.
	subscribeOutput, err := client.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: createOutput.TopicArn,
		Protocol: aws.String("sqs"),
		Endpoint: aws.String("arn:aws:sqs:us-east-1:000000000000:test-queue"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.Unsubscribe(context.Background(), &sns.UnsubscribeInput{
			SubscriptionArn: subscribeOutput.SubscriptionArn,
		})
	})

	// List subscriptions by topic.
	listOutput, err := client.ListSubscriptionsByTopic(ctx, &sns.ListSubscriptionsByTopicInput{
		TopicArn: createOutput.TopicArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("SubscriptionArn", "TopicArn", "ResultMetadata")).Assert(t.Name(), listOutput)
}

func TestSNS_Publish(t *testing.T) {
	client := newSNSClient(t)
	ctx := t.Context()
	topicName := "test-topic-publish"
	message := "Hello, SNS!"

	// Create topic.
	createOutput, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTopic(context.Background(), &sns.DeleteTopicInput{
			TopicArn: createOutput.TopicArn,
		})
	})

	// Publish message.
	publishOutput, err := client.Publish(ctx, &sns.PublishInput{
		TopicArn: createOutput.TopicArn,
		Message:  aws.String(message),
		Subject:  aws.String("Test Subject"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("MessageId", "SequenceNumber", "ResultMetadata")).Assert(t.Name(), publishOutput)
}

func TestSNS_CreateTopicIdempotent(t *testing.T) {
	client := newSNSClient(t)
	ctx := t.Context()
	topicName := "test-topic-idempotent"

	// Create topic first time.
	createOutput1, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTopic(context.Background(), &sns.DeleteTopicInput{
			TopicArn: createOutput1.TopicArn,
		})
	})

	// Create topic second time (should return the same ARN).
	createOutput2, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatal(err)
	}

	if *createOutput1.TopicArn != *createOutput2.TopicArn {
		t.Errorf("topic ARN mismatch: first %s, second %s",
			*createOutput1.TopicArn, *createOutput2.TopicArn)
	}
}

func TestSNS_GetSubscriptionAttributes(t *testing.T) {
	client := newSNSClient(t)
	ctx := t.Context()
	topicName := "test-get-sub-attrs"

	// Create topic.
	topicOutput, err := client.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteTopic(context.Background(), &sns.DeleteTopicInput{
			TopicArn: topicOutput.TopicArn,
		})
	})

	// Subscribe with SQS protocol.
	subOutput, err := client.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: topicOutput.TopicArn,
		Protocol: aws.String("sqs"),
		Endpoint: aws.String("arn:aws:sqs:us-east-1:000000000000:test-queue"),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.Unsubscribe(context.Background(), &sns.UnsubscribeInput{
			SubscriptionArn: subOutput.SubscriptionArn,
		})
	})

	// GetSubscriptionAttributes.
	getOutput, err := client.GetSubscriptionAttributes(ctx, &sns.GetSubscriptionAttributesInput{
		SubscriptionArn: subOutput.SubscriptionArn,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields(
		"SubscriptionArn", "TopicArn", "ResultMetadata",
	)).Assert(t.Name(), getOutput)
}
