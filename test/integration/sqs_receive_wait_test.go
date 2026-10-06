//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func TestSQS_ReceiveMessage_LongPollsTheQueueWaitTime(t *testing.T) {
	client := newSQSClient(t)
	ctx := t.Context()
	queueName := "test-queue-receive-wait"

	createOutput, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"ReceiveMessageWaitTimeSeconds": "1",
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

	type receiveResult struct {
		output *sqs.ReceiveMessageOutput
		err    error
	}

	results := make(chan receiveResult, 1)

	go func() {
		// No WaitTimeSeconds, so the queue's ReceiveMessageWaitTimeSeconds applies.
		output, err := client.ReceiveMessage(context.Background(), &sqs.ReceiveMessageInput{
			QueueUrl:            createOutput.QueueUrl,
			MaxNumberOfMessages: 1,
		})
		results <- receiveResult{output: output, err: err}
	}()

	// The call must still be polling: the queue asks for a one second wait, and
	// the queue is empty. A short poll would have returned it already.
	select {
	case result := <-results:
		t.Fatalf("ReceiveMessage returned before its wait elapsed: %v (err=%v)", result.output, result.err)
	case <-time.After(200 * time.Millisecond):
	}

	if _, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    createOutput.QueueUrl,
		MessageBody: aws.String("arrived"),
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case result := <-results:
		if result.err != nil {
			t.Fatal(result.err)
		}

		if len(result.output.Messages) != 1 || aws.ToString(result.output.Messages[0].Body) != "arrived" {
			t.Fatalf("ReceiveMessage returned %v, want the message sent while polling", result.output.Messages)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("long poll did not return the message that arrived while it was polling")
	}
}
