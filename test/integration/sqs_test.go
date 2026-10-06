//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go"
	"github.com/sivchari/golden"
)

func newSQSClient(t *testing.T) *sqs.Client {
	t.Helper()

	return sqs.NewFromConfig(awsConfig(t), func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestSQS_CreateAndDeleteQueue(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-create-delete"

	// Create queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("QueueUrl", "ResultMetadata")).Assert(t.Name()+"_create", createOutput)

	// Delete queue.
	_, err = client.DeleteQueue(ctx, &sqs.DeleteQueueInput{
		QueueUrl: createOutput.QueueUrl,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSQS_ListQueues(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-list"

	// Create queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// List queues.
	listOutput, err := client.ListQueues(ctx, &sqs.ListQueuesInput{})
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, url := range listOutput.QueueUrls {
		if url == *createOutput.QueueUrl {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("queue %s not found in list", *createOutput.QueueUrl)
	}
}

func TestSQS_GetQueueUrl(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-get-url"

	// Create queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Get queue URL.
	getOutput, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("QueueUrl", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestSQS_SendAndReceiveMessage(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-send-receive"
	messageBody := "Hello, SQS!"

	// Create queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send message.
	sendOutput, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    createOutput.QueueUrl,
		MessageBody: aws.String(messageBody),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("MessageId", "MD5OfMessageBody", "SequenceNumber", "ResultMetadata")).Assert(t.Name()+"_send", sendOutput)

	// Receive message.
	receiveOutput, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            createOutput.QueueUrl,
		MaxNumberOfMessages: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("MessageId", "ReceiptHandle", "MD5OfBody", "ApproximateFirstReceiveTimestamp", "SentTimestamp", "ResultMetadata")).Assert(t.Name()+"_receive", receiveOutput)

	// Delete message.
	_, err = client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      createOutput.QueueUrl,
		ReceiptHandle: receiveOutput.Messages[0].ReceiptHandle,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSQS_PurgeQueue(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-purge"

	// Create queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send multiple messages.
	for i := 0; i < 3; i++ {
		_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    createOutput.QueueUrl,
			MessageBody: aws.String("test message"),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Purge queue.
	_, err = client.PurgeQueue(ctx, &sqs.PurgeQueueInput{
		QueueUrl: createOutput.QueueUrl,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify queue is empty.
	receiveOutput, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            createOutput.QueueUrl,
		MaxNumberOfMessages: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(receiveOutput.Messages) != 0 {
		t.Errorf("expected 0 messages after purge, got %d", len(receiveOutput.Messages))
	}
}

func TestSQS_GetQueueAttributes(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-attributes"

	// Create queue with custom attributes.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"VisibilityTimeout": "60",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Get queue attributes.
	getOutput, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: createOutput.QueueUrl,
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameAll,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("QueueArn", "CreatedTimestamp", "LastModifiedTimestamp", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestSQS_SetQueueAttributes(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-set-attributes"

	// Create queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Set queue attributes.
	_, err = client.SetQueueAttributes(ctx, &sqs.SetQueueAttributesInput{
		QueueUrl: createOutput.QueueUrl,
		Attributes: map[string]string{
			"VisibilityTimeout": "120",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify attributes.
	getOutput, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: createOutput.QueueUrl,
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameVisibilityTimeout,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestSQS_QueueTags(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-tags"

	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Tags: map[string]string{
			"key1": "val1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	listOutput, err := client.ListQueueTags(ctx, &sqs.ListQueueTagsInput{
		QueueUrl: createOutput.QueueUrl,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_initial", listOutput)

	_, err = client.TagQueue(ctx, &sqs.TagQueueInput{
		QueueUrl: createOutput.QueueUrl,
		Tags: map[string]string{
			"key2": "val2",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.UntagQueue(ctx, &sqs.UntagQueueInput{
		QueueUrl: createOutput.QueueUrl,
		TagKeys:  []string{"key1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	listOutput, err = client.ListQueueTags(ctx, &sqs.ListQueueTagsInput{
		QueueUrl: createOutput.QueueUrl,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_updated", listOutput)
}

func TestSQS_FIFOQueue_CreateAndSendMessage(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-fifo.fifo"

	// Create FIFO queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"FifoQueue":                 "true",
			"ContentBasedDeduplication": "true",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send message with MessageGroupId.
	sendOutput, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:       createOutput.QueueUrl,
		MessageBody:    aws.String("FIFO message"),
		MessageGroupId: aws.String("group1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("MessageId", "MD5OfMessageBody", "SequenceNumber", "ResultMetadata")).Assert(t.Name(), sendOutput)
}

func TestSQS_FIFOQueue_GetAttributes(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-fifo-attrs.fifo"

	// Create FIFO queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"FifoQueue":                 "true",
			"ContentBasedDeduplication": "true",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Get queue attributes.
	getOutput, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: createOutput.QueueUrl,
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameAll,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("QueueArn", "CreatedTimestamp", "LastModifiedTimestamp", "ResultMetadata")).Assert(t.Name(), getOutput)
}

func TestSQS_FIFOQueue_MissingMessageGroupId(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-fifo-no-group.fifo"

	// Create FIFO queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"FifoQueue":                 "true",
			"ContentBasedDeduplication": "true",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send message without MessageGroupId (should fail).
	_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    createOutput.QueueUrl,
		MessageBody: aws.String("FIFO message without group"),
	})
	if err == nil {
		t.Error("expected error when sending message without MessageGroupId to FIFO queue")
	}
}

func TestSQS_FIFOQueue_ExplicitDeduplicationId(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-fifo-dedup.fifo"

	// Create FIFO queue without ContentBasedDeduplication.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"FifoQueue": "true",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send message with explicit MessageDeduplicationId.
	sendOutput, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:               createOutput.QueueUrl,
		MessageBody:            aws.String("FIFO message with dedup ID"),
		MessageGroupId:         aws.String("group1"),
		MessageDeduplicationId: aws.String("dedup-123"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("MessageId", "MD5OfMessageBody", "SequenceNumber", "ResultMetadata")).Assert(t.Name(), sendOutput)
}

func TestSQS_SendMessageBatch(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-send-batch"

	// Create queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send message batch.
	batchOutput, err := client.SendMessageBatch(ctx, &sqs.SendMessageBatchInput{
		QueueUrl: createOutput.QueueUrl,
		Entries: []types.SendMessageBatchRequestEntry{
			{
				Id:          aws.String("msg1"),
				MessageBody: aws.String("Hello, batch message 1"),
			},
			{
				Id:          aws.String("msg2"),
				MessageBody: aws.String("Hello, batch message 2"),
			},
			{
				Id:          aws.String("msg3"),
				MessageBody: aws.String("Hello, batch message 3"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("MessageId", "MD5OfMessageBody", "ResultMetadata")).Assert(t.Name(), batchOutput)
}

func TestSQS_DeleteMessageBatch(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-delete-batch"

	// Create queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send 3 messages.
	for i := range 3 {
		_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    createOutput.QueueUrl,
			MessageBody: aws.String(fmt.Sprintf("batch delete message %d", i)),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Receive messages.
	receiveOutput, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            createOutput.QueueUrl,
		MaxNumberOfMessages: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(receiveOutput.Messages) == 0 {
		t.Fatal("expected messages, got 0")
	}

	// Build batch delete entries.
	entries := make([]types.DeleteMessageBatchRequestEntry, len(receiveOutput.Messages))
	for i, msg := range receiveOutput.Messages {
		entries[i] = types.DeleteMessageBatchRequestEntry{
			Id:            aws.String(fmt.Sprintf("msg%d", i)),
			ReceiptHandle: msg.ReceiptHandle,
		}
	}

	// Delete message batch.
	batchOutput, err := client.DeleteMessageBatch(ctx, &sqs.DeleteMessageBatchInput{
		QueueUrl: createOutput.QueueUrl,
		Entries:  entries,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), batchOutput)

	// Verify queue is empty.
	receiveOutput2, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            createOutput.QueueUrl,
		MaxNumberOfMessages: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(receiveOutput2.Messages) != 0 {
		t.Errorf("expected 0 messages after batch delete, got %d", len(receiveOutput2.Messages))
	}
}

func TestSQS_FIFOQueue_MissingDeduplicationId(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-fifo-no-dedup.fifo"

	// Create FIFO queue without ContentBasedDeduplication.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"FifoQueue": "true",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send message without MessageDeduplicationId (should fail when CBD is false).
	_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:       createOutput.QueueUrl,
		MessageBody:    aws.String("FIFO message without dedup ID"),
		MessageGroupId: aws.String("group1"),
	})
	if err == nil {
		t.Error("expected error when sending message without MessageDeduplicationId and ContentBasedDeduplication disabled")
	}
}

func TestSQS_FIFOBatchReceive(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-fifo-batch-recv.fifo"

	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"FifoQueue":                 "true",
			"ContentBasedDeduplication": "true",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send 3 messages in the same group.
	for i := range 3 {
		_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:       createOutput.QueueUrl,
			MessageBody:    aws.String(fmt.Sprintf("msg-%d", i)),
			MessageGroupId: aws.String("group1"),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Receive should return all 3 in one batch.
	recvOutput, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            createOutput.QueueUrl,
		MaxNumberOfMessages: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recvOutput.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(recvOutput.Messages))
	}
}

func TestSQS_QueuePolicy(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-policy"

	// Create queue.
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	policyDoc := `{"Version":"2012-10-17","Statement":[{"Sid":"AllowSNS","Effect":"Allow","Principal":{"Service":"sns.amazonaws.com"},"Action":"sqs:SendMessage","Resource":"*"}]}`

	// SetQueueAttributes with Policy.
	_, err = client.SetQueueAttributes(ctx, &sqs.SetQueueAttributesInput{
		QueueUrl: createOutput.QueueUrl,
		Attributes: map[string]string{
			"Policy": policyDoc,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// GetQueueAttributes should return the Policy.
	getOutput, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       createOutput.QueueUrl,
		AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameAll},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields(
		"QueueArn", "CreatedTimestamp", "LastModifiedTimestamp", "ResultMetadata",
	)).Assert(t.Name(), getOutput)
}

func TestSQS_ChangeMessageVisibilityBatch(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-cmv-batch"

	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send a message.
	_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    createOutput.QueueUrl,
		MessageBody: aws.String("cmv-batch-test"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Receive message to get receipt handle.
	recvOutput, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            createOutput.QueueUrl,
		MaxNumberOfMessages: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recvOutput.Messages) == 0 {
		t.Fatal("expected at least 1 message")
	}

	// ChangeMessageVisibilityBatch.
	batchOutput, err := client.ChangeMessageVisibilityBatch(ctx, &sqs.ChangeMessageVisibilityBatchInput{
		QueueUrl: createOutput.QueueUrl,
		Entries: []types.ChangeMessageVisibilityBatchRequestEntry{
			{
				Id:                aws.String("entry-1"),
				ReceiptHandle:     recvOutput.Messages[0].ReceiptHandle,
				VisibilityTimeout: 0,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), batchOutput)
}

func TestSQS_VisibilityTimeoutRedelivery(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-visibility-redelivery"

	// Create queue with a short visibility timeout (1 second).
	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"VisibilityTimeout": "1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Send a message.
	_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    createOutput.QueueUrl,
		MessageBody: aws.String("visibility-timeout-test"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// First receive: message becomes invisible.
	recvOutput1, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            createOutput.QueueUrl,
		MaxNumberOfMessages: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recvOutput1.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(recvOutput1.Messages))
	}

	firstBody := aws.ToString(recvOutput1.Messages[0].Body)
	if firstBody != "visibility-timeout-test" {
		t.Fatalf("expected body %q, got %q", "visibility-timeout-test", firstBody)
	}

	// Immediate receive: message should still be invisible.
	recvOutput2, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            createOutput.QueueUrl,
		MaxNumberOfMessages: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recvOutput2.Messages) != 0 {
		t.Fatalf("expected 0 messages while invisible, got %d", len(recvOutput2.Messages))
	}

	// Poll instead of sleeping a fixed margin past the 1s visibility timeout.
	var recvOutput3 *sqs.ReceiveMessageOutput

	waitFor(t, 10*time.Second, "message redelivered after visibility timeout", func() bool {
		recvOutput3, err = client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            createOutput.QueueUrl,
			MaxNumberOfMessages: 1,
		})
		if err != nil {
			t.Fatal(err)
		}

		return len(recvOutput3.Messages) == 1
	})

	redeliveredBody := aws.ToString(recvOutput3.Messages[0].Body)
	if redeliveredBody != "visibility-timeout-test" {
		t.Fatalf("expected body %q, got %q", "visibility-timeout-test", redeliveredBody)
	}

	// Receive count should be "2" after redelivery.
	if recvOutput3.Messages[0].Attributes["ApproximateReceiveCount"] != "2" {
		t.Errorf("expected ApproximateReceiveCount=2, got %s", recvOutput3.Messages[0].Attributes["ApproximateReceiveCount"])
	}

	// Delete the message so it doesn't interfere with other tests.
	_, err = client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      createOutput.QueueUrl,
		ReceiptHandle: recvOutput3.Messages[0].ReceiptHandle,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSQS_VisibilityTimeoutDLQRedrive(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	dlqName := "test-queue-dlq-redrive"
	sourceName := "test-queue-redrive-source"

	// Create DLQ.
	dlqOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(dlqName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: dlqOutput.QueueUrl,
		})
	})

	// Get DLQ ARN.
	dlqAttrs, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       dlqOutput.QueueUrl,
		AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameQueueArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	dlqArn := dlqAttrs.Attributes["QueueArn"]

	// Create source queue with redrive policy (maxReceiveCount=2, visibility timeout=1s).
	redrivePolicy := fmt.Sprintf(`{"deadLetterTargetArn":"%s","maxReceiveCount":"2"}`, dlqArn)
	sourceOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(sourceName),
		Attributes: map[string]string{
			"VisibilityTimeout": "1",
			"RedrivePolicy":     redrivePolicy,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: sourceOutput.QueueUrl,
		})
	})

	// Send a message.
	_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    sourceOutput.QueueUrl,
		MessageBody: aws.String("dlq-redrive-test"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// First receive (ReceiveCount becomes 1).
	recvOutput1, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            sourceOutput.QueueUrl,
		MaxNumberOfMessages: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recvOutput1.Messages) != 1 {
		t.Fatalf("expected 1 message on first receive, got %d", len(recvOutput1.Messages))
	}

	// Second receive (ReceiveCount becomes 2, matches maxReceiveCount).
	// Poll past the 1s visibility timeout instead of sleeping a fixed margin.
	waitFor(t, 10*time.Second, "message redelivered after visibility timeout", func() bool {
		recvOutput2, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            sourceOutput.QueueUrl,
			MaxNumberOfMessages: 1,
		})
		if err != nil {
			t.Fatal(err)
		}

		return len(recvOutput2.Messages) == 1
	})

	// The redrive to the DLQ happens when a receive on the source queue scans
	// the message after its visibility timeout expired again, so each poll
	// receives from the source first and then checks the DLQ.
	var recvDLQ *sqs.ReceiveMessageOutput

	waitFor(t, 10*time.Second, "message moved to DLQ", func() bool {
		recvSource, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            sourceOutput.QueueUrl,
			MaxNumberOfMessages: 1,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(recvSource.Messages) != 0 {
			t.Fatalf("expected 0 messages from source (should move to DLQ), got %d", len(recvSource.Messages))
		}

		recvDLQ, err = client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            dlqOutput.QueueUrl,
			MaxNumberOfMessages: 1,
		})
		if err != nil {
			t.Fatal(err)
		}

		return len(recvDLQ.Messages) == 1
	})

	dlqBody := aws.ToString(recvDLQ.Messages[0].Body)
	if dlqBody != "dlq-redrive-test" {
		t.Fatalf("expected DLQ body %q, got %q", "dlq-redrive-test", dlqBody)
	}

	// Clean up DLQ message.
	_, err = client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      dlqOutput.QueueUrl,
		ReceiptHandle: recvDLQ.Messages[0].ReceiptHandle,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSQS_AddAndRemovePermission(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-add-remove-permission"

	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// AddPermission grants SendMessage to one account and ReceiveMessage to
	// another, per the AWS API reference example.
	_, err = client.AddPermission(ctx, &sqs.AddPermissionInput{
		QueueUrl:      createOutput.QueueUrl,
		Label:         aws.String("TestSharedAccess"),
		AWSAccountIds: []string{"177715257436", "111111111111"},
		Actions:       []string{"SendMessage", "ReceiveMessage"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// GetQueueAttributes should reflect the generated Policy.
	afterAdd, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       createOutput.QueueUrl,
		AttributeNames: []types.QueueAttributeName{types.QueueAttributeNamePolicy},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_after_add", afterAdd)

	// RemovePermission removes the statement by Label.
	_, err = client.RemovePermission(ctx, &sqs.RemovePermissionInput{
		QueueUrl: createOutput.QueueUrl,
		Label:    aws.String("TestSharedAccess"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// The Policy attribute should be gone now that the only statement was removed.
	afterRemove, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       createOutput.QueueUrl,
		AttributeNames: []types.QueueAttributeName{types.QueueAttributeNamePolicy},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_after_remove", afterRemove)
}

func TestSQS_AddPermissionErrors(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-add-permission-errors"

	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	// Too many actions in a single statement (limit is 7).
	_, err = client.AddPermission(ctx, &sqs.AddPermissionInput{
		QueueUrl:      createOutput.QueueUrl,
		Label:         aws.String("TooManyActions"),
		AWSAccountIds: []string{"111111111111"},
		Actions: []string{
			"SendMessage", "ReceiveMessage", "DeleteMessage", "GetQueueAttributes",
			"GetQueueUrl", "PurgeQueue", "ChangeMessageVisibility", "ListDeadLetterSourceQueues",
		},
	})

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "OverLimit" {
		t.Fatalf("expected OverLimit error, got %v", err)
	}

	// Duplicate label.
	_, err = client.AddPermission(ctx, &sqs.AddPermissionInput{
		QueueUrl:      createOutput.QueueUrl,
		Label:         aws.String("DupLabel"),
		AWSAccountIds: []string{"111111111111"},
		Actions:       []string{"SendMessage"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.AddPermission(ctx, &sqs.AddPermissionInput{
		QueueUrl:      createOutput.QueueUrl,
		Label:         aws.String("DupLabel"),
		AWSAccountIds: []string{"222222222222"},
		Actions:       []string{"ReceiveMessage"},
	})
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "InvalidParameterValue" {
		t.Fatalf("expected InvalidParameterValue error for duplicate label, got %v", err)
	}

	// RemovePermission with an unknown label.
	_, err = client.RemovePermission(ctx, &sqs.RemovePermissionInput{
		QueueUrl: createOutput.QueueUrl,
		Label:    aws.String("NoSuchLabel"),
	})
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "InvalidParameterValue" {
		t.Fatalf("expected InvalidParameterValue error for unknown label, got %v", err)
	}
}

func TestSQS_GetQueueAttributes_DelayedMessageIsNotAvailable(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-delayed-attributes"

	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	if _, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:     createOutput.QueueUrl,
		MessageBody:  aws.String("later"),
		DelaySeconds: 900,
	}); err != nil {
		t.Fatal(err)
	}

	output, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: createOutput.QueueUrl,
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameApproximateNumberOfMessages,
			types.QueueAttributeNameApproximateNumberOfMessagesDelayed,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := output.Attributes["ApproximateNumberOfMessages"]; got != "0" {
		t.Errorf("ApproximateNumberOfMessages = %q, want %q", got, "0")
	}

	if got := output.Attributes["ApproximateNumberOfMessagesDelayed"]; got != "1" {
		t.Errorf("ApproximateNumberOfMessagesDelayed = %q, want %q", got, "1")
	}

	received, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            createOutput.QueueUrl,
		MaxNumberOfMessages: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(received.Messages) != 0 {
		t.Errorf("ReceiveMessage returned %d messages for a delayed queue, want 0", len(received.Messages))
	}
}

func TestSQS_GetQueueAttributes_ReceivedMessageIsNotVisible(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-received-attributes"

	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: createOutput.QueueUrl,
		})
	})

	if _, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    createOutput.QueueUrl,
		MessageBody: aws.String("ready"),
	}); err != nil {
		t.Fatal(err)
	}

	attributes := func() map[string]string {
		t.Helper()

		output, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl: createOutput.QueueUrl,
			AttributeNames: []types.QueueAttributeName{
				types.QueueAttributeNameApproximateNumberOfMessages,
				types.QueueAttributeNameApproximateNumberOfMessagesDelayed,
				types.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		return output.Attributes
	}

	if got := attributes()["ApproximateNumberOfMessages"]; got != "1" {
		t.Errorf("ApproximateNumberOfMessages before receive = %q, want %q", got, "1")
	}

	received, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            createOutput.QueueUrl,
		MaxNumberOfMessages: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(received.Messages) != 1 {
		t.Fatalf("ReceiveMessage returned %d messages, want 1", len(received.Messages))
	}

	after := attributes()

	if got := after["ApproximateNumberOfMessages"]; got != "0" {
		t.Errorf("ApproximateNumberOfMessages after receive = %q, want %q", got, "0")
	}

	if got := after["ApproximateNumberOfMessagesDelayed"]; got != "0" {
		t.Errorf("ApproximateNumberOfMessagesDelayed after receive = %q, want %q", got, "0")
	}

	if got := after["ApproximateNumberOfMessagesNotVisible"]; got != "1" {
		t.Errorf("ApproximateNumberOfMessagesNotVisible after receive = %q, want %q", got, "1")
	}
}
