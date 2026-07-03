//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/sivchari/golden"
)

func newEventBridgeClient(t *testing.T) *eventbridge.Client {
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

	return eventbridge.NewFromConfig(cfg, func(o *eventbridge.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestEventBridge_CreateAndDescribeEventBus(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Create event bus.
	createOutput, err := client.CreateEventBus(ctx, &eventbridge.CreateEventBusInput{
		Name: aws.String("test-event-bus"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("EventBusArn", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Describe event bus.
	describeOutput, err := client.DescribeEventBus(ctx, &eventbridge.DescribeEventBusInput{
		Name: aws.String("test-event-bus"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestEventBridge_ListEventBuses(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Create an event bus first.
	_, err := client.CreateEventBus(ctx, &eventbridge.CreateEventBusInput{
		Name: aws.String("test-list-event-bus"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List event buses.
	listOutput, err := client.ListEventBuses(ctx, &eventbridge.ListEventBusesInput{
		Limit: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Default event bus should always be present.
	foundDefault := false

	for _, eb := range listOutput.EventBuses {
		if *eb.Name == "default" {
			foundDefault = true

			break
		}
	}

	if !foundDefault {
		t.Error("default event bus not found in list")
	}
}

func TestEventBridge_PutAndDescribeRule(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Put rule on default event bus.
	putOutput, err := client.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("test-rule"),
		EventPattern: aws.String(`{"source": ["test.source"]}`),
		State:        types.RuleStateEnabled,
		Description:  aws.String("Test rule"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("RuleArn", "ResultMetadata")).Assert(t.Name()+"_put", putOutput)

	// Describe rule.
	describeOutput, err := client.DescribeRule(ctx, &eventbridge.DescribeRuleInput{
		Name: aws.String("test-rule"),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("Arn", "ResultMetadata")).Assert(t.Name()+"_describe", describeOutput)
}

func TestEventBridge_ListRules(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Create a rule first.
	_, err := client.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("test-list-rule"),
		EventPattern: aws.String(`{"source": ["test.source"]}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List rules.
	listOutput, err := client.ListRules(ctx, &eventbridge.ListRulesInput{
		Limit: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, rule := range listOutput.Rules {
		if *rule.Name == "test-list-rule" {
			found = true

			break
		}
	}

	if !found {
		t.Error("created rule not found in list")
	}
}

func TestEventBridge_PutAndListTargets(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Create a rule first.
	_, err := client.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("test-targets-rule"),
		EventPattern: aws.String(`{"source": ["test.source"]}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put targets.
	putTargetsOutput, err := client.PutTargets(ctx, &eventbridge.PutTargetsInput{
		Rule: aws.String("test-targets-rule"),
		Targets: []types.Target{
			{
				Id:  aws.String("target-1"),
				Arn: aws.String("arn:aws:lambda:us-east-1:000000000000:function:test-function"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_put_targets", putTargetsOutput)

	// List targets.
	listTargetsOutput, err := client.ListTargetsByRule(ctx, &eventbridge.ListTargetsByRuleInput{
		Rule: aws.String("test-targets-rule"),
	})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, target := range listTargetsOutput.Targets {
		if *target.Id == "target-1" {
			found = true

			break
		}
	}

	if !found {
		t.Error("created target not found in list")
	}
}

func TestEventBridge_RemoveTargets(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Create a rule and add a target.
	_, err := client.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("test-remove-targets-rule"),
		EventPattern: aws.String(`{"source": ["test.source"]}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.PutTargets(ctx, &eventbridge.PutTargetsInput{
		Rule: aws.String("test-remove-targets-rule"),
		Targets: []types.Target{
			{
				Id:  aws.String("target-to-remove"),
				Arn: aws.String("arn:aws:lambda:us-east-1:000000000000:function:test-function"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Remove targets.
	removeOutput, err := client.RemoveTargets(ctx, &eventbridge.RemoveTargetsInput{
		Rule: aws.String("test-remove-targets-rule"),
		Ids:  []string{"target-to-remove"},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), removeOutput)
}

func TestEventBridge_PutEvents(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Put events.
	putEventsOutput, err := client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				Source:     aws.String("test.source"),
				DetailType: aws.String("test.detail.type"),
				Detail:     aws.String(`{"key": "value"}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("EventId", "ResultMetadata")).Assert(t.Name(), putEventsOutput)
}

func TestEventBridge_DeleteRule(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Create a rule.
	_, err := client.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("test-delete-rule"),
		EventPattern: aws.String(`{"source": ["test.source"]}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete rule.
	_, err = client.DeleteRule(ctx, &eventbridge.DeleteRuleInput{
		Name: aws.String("test-delete-rule"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.DescribeRule(ctx, &eventbridge.DescribeRuleInput{
		Name: aws.String("test-delete-rule"),
	})
	if err == nil {
		t.Fatal("expected error for deleted rule")
	}
}

func TestEventBridge_DeleteEventBus(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Create an event bus.
	_, err := client.CreateEventBus(ctx, &eventbridge.CreateEventBusInput{
		Name: aws.String("test-delete-event-bus"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Delete event bus.
	_, err = client.DeleteEventBus(ctx, &eventbridge.DeleteEventBusInput{
		Name: aws.String("test-delete-event-bus"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion.
	_, err = client.DescribeEventBus(ctx, &eventbridge.DescribeEventBusInput{
		Name: aws.String("test-delete-event-bus"),
	})
	if err == nil {
		t.Fatal("expected error for deleted event bus")
	}
}

func TestEventBridge_PutEvents_Delivery(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Create rule with event pattern.
	_, err := client.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("delivery-test-rule"),
		EventPattern: aws.String(`{"source": ["order.service"], "detail-type": ["OrderCreated"]}`),
		State:        types.RuleStateEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add target to rule.
	_, err = client.PutTargets(ctx, &eventbridge.PutTargetsInput{
		Rule: aws.String("delivery-test-rule"),
		Targets: []types.Target{
			{
				Id:  aws.String("sqs-target"),
				Arn: aws.String("arn:aws:sqs:us-east-1:000000000000:order-queue"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put matching event.
	_, err = client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				Source:     aws.String("order.service"),
				DetailType: aws.String("OrderCreated"),
				Detail:     aws.String(`{"orderId": "123"}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put non-matching event.
	_, err = client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				Source:     aws.String("other.service"),
				DetailType: aws.String("SomethingElse"),
				Detail:     aws.String(`{"data": "ignored"}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check delivered events via kumo endpoint.
	resp, err := http.Get("http://localhost:4566/kumo/eventbridge/delivered-events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	var delivered []struct {
		Source     string `json:"Source"`
		DetailType string `json:"DetailType"`
		RuleName   string `json:"RuleName"`
		TargetID   string `json:"TargetId"`
		TargetArn  string `json:"TargetArn"`
	}

	if err := json.Unmarshal(body, &delivered); err != nil {
		t.Fatal(err)
	}

	// Find our delivery.
	found := false

	for _, d := range delivered {
		if d.Source == "order.service" && d.RuleName == "delivery-test-rule" && d.TargetID == "sqs-target" {
			found = true

			break
		}
	}

	if !found {
		t.Fatalf("expected matching event to be delivered to sqs-target, got: %s", string(body))
	}
}

func TestEventBridge_PutEvents_SQSDelivery(t *testing.T) {
	ebClient := newEventBridgeClient(t)
	sqsClient := newSQSClient(t)
	ctx := t.Context()

	queueName := "eb-sqs-delivery-test"

	// Create SQS queue.
	_, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create rule matching the event.
	_, err = ebClient.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("sqs-delivery-rule"),
		EventPattern: aws.String(`{"source": ["payment.service"], "detail-type": ["PaymentProcessed"]}`),
		State:        types.RuleStateEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add SQS target.
	_, err = ebClient.PutTargets(ctx, &eventbridge.PutTargetsInput{
		Rule: aws.String("sqs-delivery-rule"),
		Targets: []types.Target{
			{
				Id:  aws.String("sqs-delivery-target"),
				Arn: aws.String("arn:aws:sqs:us-east-1:000000000000:" + queueName),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put matching event.
	putOutput, err := ebClient.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				Source:     aws.String("payment.service"),
				DetailType: aws.String("PaymentProcessed"),
				Detail:     aws.String(`{"paymentId": "pay-001"}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("EventId", "ResultMetadata")).Assert(t.Name()+"_put_events", putOutput)

	// Receive message from SQS to confirm delivery.
	var recvOutput *sqs.ReceiveMessageOutput

	for range 10 {
		recvOutput, err = sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:        aws.String("http://localhost:4566/000000000000/" + queueName),
			WaitTimeSeconds: 1,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(recvOutput.Messages) > 0 {
			break
		}
	}

	if len(recvOutput.Messages) == 0 {
		t.Fatal("expected event to be delivered to SQS queue, but no message received")
	}

	// Parse the message body to verify it's a valid EventBridge event envelope.
	var envelope map[string]any
	if err := json.Unmarshal([]byte(*recvOutput.Messages[0].Body), &envelope); err != nil {
		t.Fatalf("failed to parse SQS message body as JSON: %v", err)
	}

	if envelope["source"] != "payment.service" {
		t.Errorf("expected source=payment.service, got %v", envelope["source"])
	}

	if envelope["detail-type"] != "PaymentProcessed" {
		t.Errorf("expected detail-type=PaymentProcessed, got %v", envelope["detail-type"])
	}
}

func TestEventBridge_PutEvents_InputPath(t *testing.T) {
	ebClient := newEventBridgeClient(t)
	sqsClient := newSQSClient(t)
	ctx := t.Context()

	queueName := "eb-inputpath-test"

	// Create SQS queue.
	_, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create rule.
	_, err = ebClient.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("inputpath-rule"),
		EventPattern: aws.String(`{"source": ["notif.service"], "detail-type": ["notifevent"]}`),
		State:        types.RuleStateEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add SQS target with InputPath.
	putTargetsOutput, err := ebClient.PutTargets(ctx, &eventbridge.PutTargetsInput{
		Rule: aws.String("inputpath-rule"),
		Targets: []types.Target{
			{
				Id:        aws.String("inputpath-target"),
				Arn:       aws.String("arn:aws:sqs:us-east-1:000000000000:" + queueName),
				InputPath: aws.String("$.detail"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_put_targets", putTargetsOutput)

	// Put matching event.
	_, err = ebClient.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				Source:     aws.String("notif.service"),
				DetailType: aws.String("notifevent"),
				Detail:     aws.String(`{"message": "hello", "userId": "u-001"}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Receive message from SQS.
	var recvOutput *sqs.ReceiveMessageOutput

	for range 10 {
		recvOutput, err = sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:        aws.String("http://localhost:4566/000000000000/" + queueName),
			WaitTimeSeconds: 1,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(recvOutput.Messages) > 0 {
			break
		}
	}

	if len(recvOutput.Messages) == 0 {
		t.Fatal("expected event to be delivered to SQS queue, but no message received")
	}

	// Verify that InputPath was applied: message body should be the detail only.
	var detail map[string]any
	if err := json.Unmarshal([]byte(*recvOutput.Messages[0].Body), &detail); err != nil {
		t.Fatalf("failed to parse SQS message body: %v (body: %s)", err, *recvOutput.Messages[0].Body)
	}

	if detail["message"] != "hello" {
		t.Errorf("expected message=hello, got %v", detail["message"])
	}

	if detail["userId"] != "u-001" {
		t.Errorf("expected userId=u-001, got %v", detail["userId"])
	}

	// Verify envelope fields are NOT present (InputPath extracts only $.detail).
	if _, hasVersion := detail["version"]; hasVersion {
		t.Errorf("expected InputPath to strip envelope, but found 'version' in: %s", *recvOutput.Messages[0].Body)
	}
}

func TestEventBridge_PutEvents_InputTransformer(t *testing.T) {
	ebClient := newEventBridgeClient(t)
	sqsClient := newSQSClient(t)
	ctx := t.Context()

	queueName := "eb-inputtransformer-test"

	// Create SQS queue.
	_, err := sqsClient.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	busName := "transform-bus"

	// Create custom event bus.
	_, err = ebClient.CreateEventBus(ctx, &eventbridge.CreateEventBusInput{
		Name: aws.String(busName),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create rule on custom bus.
	_, err = ebClient.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("transform-rule"),
		EventBusName: aws.String(busName),
		EventPattern: aws.String(`{"source": ["transform.service"], "detail-type": ["TransformEvent"]}`),
		State:        types.RuleStateEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add SQS target with InputTransformer.
	_, err = ebClient.PutTargets(ctx, &eventbridge.PutTargetsInput{
		Rule:         aws.String("transform-rule"),
		EventBusName: aws.String(busName),
		Targets: []types.Target{
			{
				Id:  aws.String("transform-target"),
				Arn: aws.String("arn:aws:sqs:us-east-1:000000000000:" + queueName),
				InputTransformer: &types.InputTransformer{
					InputPathsMap: map[string]string{
						"marker": "$.detail.marker",
					},
					InputTemplate: aws.String(`{"transformedMarker": <marker>, "source": "custom-bus"}`),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = ebClient.RemoveTargets(cleanupCtx, &eventbridge.RemoveTargetsInput{
			EventBusName: aws.String(busName),
			Ids:          []string{"transform-target"},
			Rule:         aws.String("transform-rule"),
		})
		_, _ = ebClient.DeleteRule(cleanupCtx, &eventbridge.DeleteRuleInput{
			EventBusName: aws.String(busName),
			Name:         aws.String("transform-rule"),
		})
		_, _ = ebClient.DeleteEventBus(cleanupCtx, &eventbridge.DeleteEventBusInput{
			Name: aws.String(busName),
		})
		_, _ = sqsClient.DeleteQueue(cleanupCtx, &sqs.DeleteQueueInput{
			QueueUrl: aws.String("http://localhost:4566/000000000000/" + queueName),
		})
	})

	// Put matching event.
	marker := "test-marker-" + t.Name()

	_, err = ebClient.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				EventBusName: aws.String(busName),
				Source:       aws.String("transform.service"),
				DetailType:   aws.String("TransformEvent"),
				Detail:       aws.String(`{"marker":"` + marker + `"}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Receive message from SQS and verify transformation was applied.
	var recvOutput *sqs.ReceiveMessageOutput

	for range 10 {
		recvOutput, err = sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:        aws.String("http://localhost:4566/000000000000/" + queueName),
			WaitTimeSeconds: 1,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(recvOutput.Messages) > 0 {
			break
		}
	}

	if len(recvOutput.Messages) == 0 {
		t.Fatal("expected transformed event to be delivered to SQS queue, but no message received")
	}

	body := *recvOutput.Messages[0].Body

	// Verify the message contains the transformed content.
	var transformed map[string]any
	if err := json.Unmarshal([]byte(body), &transformed); err != nil {
		t.Fatalf("failed to parse SQS message body as JSON: %v (body: %s)", err, body)
	}

	if transformed["transformedMarker"] != marker {
		t.Errorf("expected transformedMarker=%s, got %v", marker, transformed["transformedMarker"])
	}

	if transformed["source"] != "custom-bus" {
		t.Errorf("expected source=custom-bus, got %v", transformed["source"])
	}

	// Verify envelope fields are NOT present (InputTransformer produces custom output).
	if _, hasVersion := transformed["version"]; hasVersion {
		t.Errorf("expected InputTransformer to replace envelope, but found 'version' in: %s", body)
	}
}

func TestEventBridge_TagOperations(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()
	busName := "test-tag-bus"

	// Create event bus.
	createOut, err := client.CreateEventBus(ctx, &eventbridge.CreateEventBusInput{
		Name: aws.String(busName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteEventBus(context.Background(), &eventbridge.DeleteEventBusInput{
			Name: aws.String(busName),
		})
	})

	busARN := aws.ToString(createOut.EventBusArn)

	// ListTagsForResource should return empty initially.
	listOut, err := client.ListTagsForResource(ctx, &eventbridge.ListTagsForResourceInput{
		ResourceARN: aws.String(busARN),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_empty", listOut)

	// TagResource.
	_, err = client.TagResource(ctx, &eventbridge.TagResourceInput{
		ResourceARN: aws.String(busARN),
		Tags: []types.Tag{
			{Key: aws.String("env"), Value: aws.String("test")},
			{Key: aws.String("team"), Value: aws.String("platform")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ListTagsForResource should return the added tags.
	listOut, err = client.ListTagsForResource(ctx, &eventbridge.ListTagsForResourceInput{
		ResourceARN: aws.String(busARN),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_after_tag", listOut)

	// UntagResource.
	_, err = client.UntagResource(ctx, &eventbridge.UntagResourceInput{
		ResourceARN: aws.String(busARN),
		TagKeys:     []string{"env"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// ListTagsForResource should return only remaining tags.
	listOut, err = client.ListTagsForResource(ctx, &eventbridge.ListTagsForResourceInput{
		ResourceARN: aws.String(busARN),
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_after_untag", listOut)
}

func TestEventBridge_EventBusNotFound(t *testing.T) {
	client := newEventBridgeClient(t)
	ctx := t.Context()

	// Try to describe a non-existent event bus.
	_, err := client.DescribeEventBus(ctx, &eventbridge.DescribeEventBusInput{
		Name: aws.String("nonexistent-event-bus"),
	})
	if err == nil {
		t.Fatal("expected error for non-existent event bus")
	}
}

func TestEventBridge_PutEvents_LambdaDelivery(t *testing.T) {
	ebClient := newEventBridgeClient(t)
	ctx := t.Context()

	// Mock Lambda backend that records invocations.
	var (
		mu       sync.Mutex
		received [][]byte
	)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, body)
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(mockServer.Close)

	functionName := "eb-lambda-delivery-fn"

	// Register Lambda function with InvokeEndpoint so that the kumo Lambda emulator
	// forwards invocations to our mock server.
	createReq, _ := json.Marshal(map[string]any{
		"FunctionName":   functionName,
		"Runtime":        "python3.12",
		"Role":           "arn:aws:iam::000000000000:role/test-role",
		"Handler":        "index.handler",
		"InvokeEndpoint": mockServer.URL,
		"Code":           map[string]any{"ZipFile": []byte("fake-zip")},
	})

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://localhost:4566/lambda/2015-03-31/functions", bytes.NewReader(createReq))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create function: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create function status: %d", resp.StatusCode)
	}

	t.Cleanup(func() {
		delReq, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete,
			"http://localhost:4566/lambda/2015-03-31/functions/"+functionName, nil)

		delResp, _ := http.DefaultClient.Do(delReq)
		if delResp != nil {
			delResp.Body.Close()
		}
	})

	// Create rule using advanced pattern matchers (prefix + numeric).
	_, err = ebClient.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:         aws.String("eb-lambda-rule"),
		EventPattern: aws.String(`{"source": [{"prefix": "order."}], "detail": {"amount": [{"numeric": [">", 0]}]}}`),
		State:        types.RuleStateEnabled,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add Lambda target.
	_, err = ebClient.PutTargets(ctx, &eventbridge.PutTargetsInput{
		Rule: aws.String("eb-lambda-rule"),
		Targets: []types.Target{
			{
				Id:  aws.String("lambda-target"),
				Arn: aws.String("arn:aws:lambda:us-east-1:000000000000:function:" + functionName),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put matching event.
	_, err = ebClient.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				Source:     aws.String("order.service"),
				DetailType: aws.String("OrderCreated"),
				Detail:     aws.String(`{"orderId": "o-1", "amount": 42}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put non-matching event (amount <= 0 should be filtered out).
	_, err = ebClient.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				Source:     aws.String("order.service"),
				DetailType: aws.String("OrderCreated"),
				Detail:     aws.String(`{"orderId": "o-2", "amount": 0}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Wait for the asynchronous delivery to complete.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(received)
		mu.Unlock()

		if count >= 1 {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected exactly 1 Lambda invocation, got %d", len(received))
	}

	// Verify payload is the EventBridge event envelope.
	var envelope map[string]any
	if err := json.Unmarshal(received[0], &envelope); err != nil {
		t.Fatalf("invalid envelope JSON: %v (body=%s)", err, string(received[0]))
	}

	if envelope["source"] != "order.service" {
		t.Errorf("source=%v, want order.service", envelope["source"])
	}

	detail, _ := envelope["detail"].(map[string]any)
	if detail["orderId"] != "o-1" {
		t.Errorf("orderId=%v, want o-1", detail["orderId"])
	}
}
