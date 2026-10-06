package sqs

import (
	"context"
	"testing"
	"time"
)

// receiveWaitAttribute is the queue attribute holding the default long-poll
// duration a receive uses when it does not send WaitTimeSeconds itself.
const receiveWaitAttribute = "ReceiveMessageWaitTimeSeconds"

// intPtr returns a pointer to v, for the receive parameters a caller sends.
func intPtr(v int) *int {
	return &v
}

func TestMemoryStorage_receiveWaitTime(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")

	queue, err := s.CreateQueue(t.Context(), "wait-time-queue", map[string]string{receiveWaitAttribute: "7"}, nil)
	if err != nil {
		t.Fatalf("CreateQueue() error = %v", err)
	}

	tests := []struct {
		name      string
		requested *int
		want      int
	}{
		{name: "omitted falls back to the queue attribute", requested: nil, want: 7},
		{name: "explicit zero short polls", requested: intPtr(0), want: 0},
		{name: "explicit value wins", requested: intPtr(3), want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := s.receiveWaitTime(queue.URL, tt.requested)
			if err != nil {
				t.Fatalf("receiveWaitTime() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("receiveWaitTime() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMemoryStorage_receiveWaitTime_UnknownQueue(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")

	if _, err := s.receiveWaitTime("http://localhost:4566/000000000000/missing", nil); err == nil {
		t.Fatal("receiveWaitTime() for an unknown queue returned no error")
	}
}

func TestMemoryStorage_ReceiveMessage_UsesTheQueueWaitTimeWhenOmitted(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := t.Context()

	queue, err := s.CreateQueue(ctx, "long-poll-queue", map[string]string{receiveWaitAttribute: "1"}, nil)
	if err != nil {
		t.Fatalf("CreateQueue() error = %v", err)
	}

	type receiveResult struct {
		messages []*Message
		err      error
	}

	results := make(chan receiveResult, 1)

	go func() {
		messages, err := s.ReceiveMessage(context.Background(), queue.URL, 1, 30, nil)
		results <- receiveResult{messages: messages, err: err}
	}()

	// The call must still be polling: the queue asks for a one second wait, and
	// the queue is empty. A short poll would have returned it already.
	select {
	case result := <-results:
		t.Fatalf("ReceiveMessage() returned before its wait elapsed: messages=%d err=%v",
			len(result.messages), result.err)
	case <-time.After(100 * time.Millisecond):
	}

	if _, err := s.SendMessage(ctx, queue.URL, "arrived", 0, nil, "", ""); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("ReceiveMessage() error = %v", result.err)
		}

		if len(result.messages) != 1 || result.messages[0].Body != "arrived" {
			t.Fatalf("ReceiveMessage() returned %#v, want the message sent while polling", result.messages)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReceiveMessage() did not return the message that arrived while it was polling")
	}
}

func TestMemoryStorage_ReceiveMessage_ReturnsImmediatelyForAnExplicitZeroWait(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := t.Context()

	queue, err := s.CreateQueue(ctx, "zero-wait-queue", map[string]string{receiveWaitAttribute: "30"}, nil)
	if err != nil {
		t.Fatalf("CreateQueue() error = %v", err)
	}

	start := time.Now()

	messages, err := s.ReceiveMessage(ctx, queue.URL, 1, 30, intPtr(0))
	if err != nil {
		t.Fatalf("ReceiveMessage() error = %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("ReceiveMessage() returned %d messages, want 0", len(messages))
	}

	// The queue asks for a 30 second wait, so anything near that means the
	// explicit zero was ignored.
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("ReceiveMessage() with an explicit zero wait took %s, want an immediate return", elapsed)
	}
}
