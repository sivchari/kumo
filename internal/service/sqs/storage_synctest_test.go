package sqs

import (
	"context"
	"strconv"
	"testing"
	"testing/synctest"
	"time"
)

// These tests run inside a synctest bubble so that delivery delays, the
// 5 minute FIFO dedup window and long polling are driven by virtual time
// rather than wall-clock sleeps.

func mustCreateQueue(t *testing.T, s *MemoryStorage, name string, attrs map[string]string) string {
	t.Helper()

	q, err := s.CreateQueue(context.Background(), name, attrs, nil)
	if err != nil {
		t.Fatalf("CreateQueue(%q): %v", name, err)
	}

	return q.URL
}

func mustReceive(t *testing.T, s *MemoryStorage, queueURL string, waitTimeSeconds int) []*Message {
	t.Helper()

	msgs, err := s.ReceiveMessage(context.Background(), queueURL, 1, 0, &waitTimeSeconds)
	if err != nil {
		t.Fatalf("ReceiveMessage: %v", err)
	}

	return msgs
}

func TestMemoryStorage_DelaySeconds(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := NewMemoryStorage("http://localhost:4566")
		ctx := context.Background()
		queueURL := mustCreateQueue(t, s, "delay", nil)

		const delaySeconds = 5

		sent, err := s.SendMessage(ctx, queueURL, "delayed", delaySeconds, nil, "", "")
		if err != nil {
			t.Fatalf("SendMessage: %v", err)
		}

		if got := mustReceive(t, s, queueURL, 0); len(got) != 0 {
			t.Fatalf("receive immediately: got %d messages, want 0", len(got))
		}

		time.Sleep((delaySeconds - 1) * time.Second)

		if got := mustReceive(t, s, queueURL, 0); len(got) != 0 {
			t.Fatalf("receive before delay elapsed: got %d messages, want 0", len(got))
		}

		time.Sleep(time.Second)

		got := mustReceive(t, s, queueURL, 0)
		if len(got) != 1 {
			t.Fatalf("receive after delay: got %d messages, want 1", len(got))
		}

		if got[0].MessageID != sent.MessageID {
			t.Fatalf("receive after delay: got message %s, want %s", got[0].MessageID, sent.MessageID)
		}
	})
}

func TestMemoryStorage_DelaySecondsQueueDefault(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := NewMemoryStorage("http://localhost:4566")
		ctx := context.Background()
		queueURL := mustCreateQueue(t, s, "delay-default", map[string]string{"DelaySeconds": "3"})

		if _, err := s.SendMessage(ctx, queueURL, "delayed", 0, nil, "", ""); err != nil {
			t.Fatalf("SendMessage: %v", err)
		}

		if got := mustReceive(t, s, queueURL, 0); len(got) != 0 {
			t.Fatalf("receive immediately: got %d messages, want 0", len(got))
		}

		time.Sleep(4 * time.Second)

		if got := mustReceive(t, s, queueURL, 0); len(got) != 1 {
			t.Fatalf("receive after delay: got %d messages, want 1", len(got))
		}
	})
}

func TestMemoryStorage_FIFODeduplicationWindow(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := NewMemoryStorage("http://localhost:4566")
		ctx := context.Background()
		queueURL := mustCreateQueue(t, s, "dedup.fifo", map[string]string{attrFifoQueue: attrValueTrue})

		const (
			groupID = "g"
			dedupID = "dedup-1"
		)

		first, err := s.SendMessage(ctx, queueURL, "payload", 0, nil, groupID, dedupID)
		if err != nil {
			t.Fatalf("first SendMessage: %v", err)
		}

		// Just inside the 5 minute window the resend must be deduplicated.
		time.Sleep(5*time.Minute - time.Second)

		dup, err := s.SendMessage(ctx, queueURL, "payload", 0, nil, groupID, dedupID)
		if err != nil {
			t.Fatalf("duplicate SendMessage: %v", err)
		}

		if dup.MessageID != first.MessageID {
			t.Fatalf("resend inside window: got message %s, want deduplicated %s", dup.MessageID, first.MessageID)
		}

		// Past the window the same dedup ID is accepted as a new message.
		time.Sleep(2 * time.Second)

		fresh, err := s.SendMessage(ctx, queueURL, "payload", 0, nil, groupID, dedupID)
		if err != nil {
			t.Fatalf("resend SendMessage: %v", err)
		}

		if fresh.MessageID == first.MessageID {
			t.Fatalf("resend after window: got deduplicated message %s, want a new one", fresh.MessageID)
		}

		qd := s.Queues[queueURL]
		if got := len(qd.Messages); got != 2 {
			t.Fatalf("queued messages = %d, want 2", got)
		}
	})
}

func TestMemoryStorage_LongPollReturnsOnSend(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := NewMemoryStorage("http://localhost:4566")
		ctx := context.Background()
		queueURL := mustCreateQueue(t, s, "longpoll-send", nil)

		const waitTimeSeconds = 20

		type result struct {
			msgs    []*Message
			err     error
			elapsed time.Duration
		}

		start := time.Now()
		done := make(chan result, 1)

		go func() {
			msgs, err := s.ReceiveMessage(ctx, queueURL, 1, 0, intPtr(waitTimeSeconds))
			done <- result{msgs: msgs, err: err, elapsed: time.Since(start)}
		}()

		// Let the receiver block on the long-poll select before sending.
		synctest.Wait()

		const sendAfter = 3 * time.Second

		time.Sleep(sendAfter)

		sent, err := s.SendMessage(ctx, queueURL, "wake", 0, nil, "", "")
		if err != nil {
			t.Fatalf("SendMessage: %v", err)
		}

		res := <-done
		if res.err != nil {
			t.Fatalf("ReceiveMessage: %v", res.err)
		}

		if len(res.msgs) != 1 || res.msgs[0].MessageID != sent.MessageID {
			t.Fatalf("long poll: got %d messages, want the sent message %s", len(res.msgs), sent.MessageID)
		}

		if res.elapsed != sendAfter {
			t.Fatalf("long poll returned after %v, want %v (immediately on send)", res.elapsed, sendAfter)
		}
	})
}

func TestMemoryStorage_LongPollTimesOutEmpty(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := NewMemoryStorage("http://localhost:4566")
		queueURL := mustCreateQueue(t, s, "longpoll-empty", nil)

		const waitTimeSeconds = 20

		start := time.Now()

		got := mustReceive(t, s, queueURL, waitTimeSeconds)
		if len(got) != 0 {
			t.Fatalf("long poll on empty queue: got %d messages, want 0", len(got))
		}

		if elapsed := time.Since(start); elapsed != waitTimeSeconds*time.Second {
			t.Fatalf("long poll returned after %v, want %v", elapsed, waitTimeSeconds*time.Second)
		}
	})
}

func TestMemoryStorage_GetQueueAttributes_DelayedMessageBecomesVisible(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		s := NewMemoryStorage("http://localhost:4566")
		ctx := context.Background()
		queueURL := mustCreateQueue(t, s, "delayed-counts", nil)

		const delaySeconds = 5

		if _, err := s.SendMessage(ctx, queueURL, "later", delaySeconds, nil, "", ""); err != nil {
			t.Fatalf("SendMessage: %v", err)
		}

		counts := func() (visible, delayed int) {
			t.Helper()

			attrs, err := s.GetQueueAttributes(ctx, queueURL, []string{
				"ApproximateNumberOfMessages",
				"ApproximateNumberOfMessagesDelayed",
			})
			if err != nil {
				t.Fatalf("GetQueueAttributes: %v", err)
			}

			visible, err = strconv.Atoi(attrs["ApproximateNumberOfMessages"])
			if err != nil {
				t.Fatalf("ApproximateNumberOfMessages = %q: %v", attrs["ApproximateNumberOfMessages"], err)
			}

			delayed, err = strconv.Atoi(attrs["ApproximateNumberOfMessagesDelayed"])
			if err != nil {
				t.Fatalf("ApproximateNumberOfMessagesDelayed = %q: %v", attrs["ApproximateNumberOfMessagesDelayed"], err)
			}

			return visible, delayed
		}

		if visible, delayed := counts(); visible != 0 || delayed != 1 {
			t.Fatalf("right after send: visible=%d delayed=%d, want 0 and 1", visible, delayed)
		}

		time.Sleep((delaySeconds - 1) * time.Second)

		if visible, delayed := counts(); visible != 0 || delayed != 1 {
			t.Fatalf("1s before the delay expires: visible=%d delayed=%d, want 0 and 1", visible, delayed)
		}

		// At exactly VisibleAt the message is no longer After(now), so it counts as visible.
		time.Sleep(1 * time.Second)

		if visible, delayed := counts(); visible != 1 || delayed != 0 {
			t.Fatalf("exactly at expiry: visible=%d delayed=%d, want 1 and 0", visible, delayed)
		}

		time.Sleep(1 * time.Second)

		if visible, delayed := counts(); visible != 1 || delayed != 0 {
			t.Fatalf("after expiry: visible=%d delayed=%d, want 1 and 0", visible, delayed)
		}
	})
}
