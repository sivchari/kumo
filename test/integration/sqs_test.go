//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/sivchari/golden"
)

func newSQSClient(t *testing.T) *sqs.Client {
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

	return sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
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
