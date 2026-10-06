package sqs

import (
	"strconv"
	"testing"
)

// queueAttributeInt reads one integer-valued queue attribute and fails the test
// when it is missing or not an integer.
func queueAttributeInt(t *testing.T, s *MemoryStorage, queueURL, name string) int {
	t.Helper()

	attrs, err := s.GetQueueAttributes(t.Context(), queueURL, []string{name})
	if err != nil {
		t.Fatalf("GetQueueAttributes(%s) error = %v", name, err)
	}

	raw, ok := attrs[name]
	if !ok {
		t.Fatalf("GetQueueAttributes(%s) did not return the attribute, got %v", name, attrs)
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("attribute %s = %q, want an integer: %v", name, raw, err)
	}

	return value
}

func TestMemoryStorage_GetQueueAttributes_CountsDelayedMessagesSeparately(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := t.Context()

	queue, err := s.CreateQueue(ctx, "delayed-queue", nil, nil)
	if err != nil {
		t.Fatalf("CreateQueue() error = %v", err)
	}

	if _, err := s.SendMessage(ctx, queue.URL, "ready", 0, nil, "", ""); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	if _, err := s.SendMessage(ctx, queue.URL, "later", 900, nil, "", ""); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	if got := queueAttributeInt(t, s, queue.URL, "ApproximateNumberOfMessages"); got != 1 {
		t.Errorf("ApproximateNumberOfMessages = %d, want 1", got)
	}

	if got := queueAttributeInt(t, s, queue.URL, "ApproximateNumberOfMessagesDelayed"); got != 1 {
		t.Errorf("ApproximateNumberOfMessagesDelayed = %d, want 1", got)
	}

	if got := queueAttributeInt(t, s, queue.URL, "ApproximateNumberOfMessagesNotVisible"); got != 0 {
		t.Errorf("ApproximateNumberOfMessagesNotVisible = %d, want 0", got)
	}
}

func TestMemoryStorage_GetQueueAttributes_CountsQueueDelayAsDelayed(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := t.Context()

	queue, err := s.CreateQueue(ctx, "delayed-default-queue", map[string]string{"DelaySeconds": "900"}, nil)
	if err != nil {
		t.Fatalf("CreateQueue() error = %v", err)
	}

	if _, err := s.SendMessage(ctx, queue.URL, "later", 0, nil, "", ""); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	if got := queueAttributeInt(t, s, queue.URL, "ApproximateNumberOfMessages"); got != 0 {
		t.Errorf("ApproximateNumberOfMessages = %d, want 0", got)
	}

	if got := queueAttributeInt(t, s, queue.URL, "ApproximateNumberOfMessagesDelayed"); got != 1 {
		t.Errorf("ApproximateNumberOfMessagesDelayed = %d, want 1", got)
	}
}
