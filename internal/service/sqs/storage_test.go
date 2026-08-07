package sqs

import (
	"errors"
	"testing"
)

func TestMemoryStorage_CreateQueueRejectsDelimiterNames(t *testing.T) {
	t.Parallel()

	tests := []string{
		"bad/name",
		"bad:name",
		"bad%2Fname",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := NewMemoryStorage("http://localhost:4566")

			_, err := s.CreateQueue(t.Context(), name, nil, nil)
			expectQueueErrorCode(t, err, errCodeInvalidParameterValue)
		})
	}
}

func TestMemoryStorage_CreateQueueAcceptsValidNames(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")

	tests := []struct {
		name  string
		attrs map[string]string
	}{
		{name: "queue_name-1"},
		{name: "queue-name.fifo", attrs: map[string]string{"FifoQueue": "true"}},
	}

	for _, tt := range tests {
		queue, err := s.CreateQueue(t.Context(), tt.name, tt.attrs, nil)
		if err != nil {
			t.Fatalf("CreateQueue(%q): %v", tt.name, err)
		}

		if queue.Name != tt.name {
			t.Fatalf("CreateQueue(%q) name = %q", tt.name, queue.Name)
		}
	}
}

func TestMemoryStorage_ResolveQueueData_HostnameMismatch(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")

	ctx := t.Context()

	_, err := s.CreateQueue(ctx, "test-queue", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		queueURL string
		wantErr  bool
	}{
		{
			name:     "exact match",
			queueURL: "http://localhost:4566/000000000000/test-queue",
		},
		{
			name:     "different hostname",
			queueURL: "http://kumo:4566/000000000000/test-queue",
		},
		{
			name:     "different scheme and hostname",
			queueURL: "https://sqs.us-east-1.amazonaws.com/000000000000/test-queue",
		},
		{
			name:     "non-existent queue",
			queueURL: "http://localhost:4566/000000000000/non-existent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg, err := s.SendMessage(ctx, tt.queueURL, "hello", 0, nil, "", "")
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("SendMessage() error = %v", err)
			}

			if msg == nil {
				t.Fatal("expected message, got nil")
			}
		})
	}
}

func TestMemoryStorage_DeleteQueue_HostnameMismatch(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")

	ctx := t.Context()

	_, err := s.CreateQueue(ctx, "delete-test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Delete using a different hostname.
	err = s.DeleteQueue(ctx, "http://kumo:4566/000000000000/delete-test")
	if err != nil {
		t.Fatalf("DeleteQueue() error = %v", err)
	}

	// Verify queue is gone.
	_, err = s.GetQueueURL(ctx, "delete-test")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestMemoryStorage_TagsLifecycle(t *testing.T) {
	t.Parallel()

	const tagValue2 = "val2"

	s := NewMemoryStorage("http://localhost:4566")
	ctx := t.Context()

	_, err := s.CreateQueue(ctx, "tagged-queue", nil, map[string]string{"key1": "val1"})
	if err != nil {
		t.Fatal(err)
	}

	tags, err := s.ListQueueTags(ctx, "http://kumo:4566/000000000000/tagged-queue")
	if err != nil {
		t.Fatalf("ListQueueTags() error = %v", err)
	}

	if len(tags) != 1 || tags["key1"] != "val1" {
		t.Fatalf("unexpected tags after create: %#v", tags)
	}

	err = s.TagQueue(ctx, "http://kumo:4566/000000000000/tagged-queue", map[string]string{"key2": tagValue2, "key1": "updated"})
	if err != nil {
		t.Fatalf("TagQueue() error = %v", err)
	}

	tags, err = s.ListQueueTags(ctx, "http://localhost:4566/000000000000/tagged-queue")
	if err != nil {
		t.Fatalf("ListQueueTags() error = %v", err)
	}

	if len(tags) != 2 || tags["key1"] != "updated" || tags["key2"] != tagValue2 {
		t.Fatalf("unexpected tags after tag: %#v", tags)
	}

	err = s.UntagQueue(ctx, "http://localhost:4566/000000000000/tagged-queue", []string{"key1"})
	if err != nil {
		t.Fatalf("UntagQueue() error = %v", err)
	}

	tags, err = s.ListQueueTags(ctx, "http://localhost:4566/000000000000/tagged-queue")
	if err != nil {
		t.Fatalf("ListQueueTags() error = %v", err)
	}

	if len(tags) != 1 || tags["key2"] != tagValue2 {
		t.Fatalf("unexpected tags after untag: %#v", tags)
	}
}

func TestMemoryStorage_PermissionLifecycle(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := t.Context()

	_, err := s.CreateQueue(ctx, "permission-queue", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	queueURL := "http://localhost:4566/000000000000/permission-queue"

	attrsBefore, err := s.GetQueueAttributes(ctx, queueURL, []string{attrPolicy})
	if err != nil {
		t.Fatalf("GetQueueAttributes() error = %v", err)
	}

	if _, ok := attrsBefore[attrPolicy]; ok {
		t.Fatalf("Policy attribute present before AddPermission: %v", attrsBefore)
	}

	if err := s.AddPermission(ctx, queueURL, "AliceSendMessage", []string{"111111111111"}, []string{actionSendMessage}); err != nil {
		t.Fatalf("AddPermission() error = %v", err)
	}

	attrsAfter, err := s.GetQueueAttributes(ctx, queueURL, []string{attrPolicy})
	if err != nil {
		t.Fatalf("GetQueueAttributes() error = %v", err)
	}

	if attrsAfter[attrPolicy] == "" {
		t.Fatal("Policy attribute missing after AddPermission")
	}

	if err := s.RemovePermission(ctx, queueURL, "AliceSendMessage"); err != nil {
		t.Fatalf("RemovePermission() error = %v", err)
	}

	attrsFinal, err := s.GetQueueAttributes(ctx, queueURL, []string{attrPolicy})
	if err != nil {
		t.Fatalf("GetQueueAttributes() error = %v", err)
	}

	if _, ok := attrsFinal[attrPolicy]; ok {
		t.Fatalf("Policy attribute still present after removing the only statement: %v", attrsFinal)
	}
}

func TestMemoryStorage_AddPermissionErrors(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := t.Context()

	_, err := s.CreateQueue(ctx, "permission-errors-queue", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	queueURL := "http://localhost:4566/000000000000/permission-errors-queue"

	tests := []struct {
		name          string
		label         string
		awsAccountIDs []string
		actions       []string
		wantCode      string
	}{
		{
			name:     "empty actions",
			label:    "Label1",
			actions:  nil,
			wantCode: errCodeMissingParameter,
		},
		{
			name:     "too many actions",
			label:    "Label2",
			actions:  []string{actionSendMessage, actionReceiveMessage, actionDeleteMessage, actionGetQueueAttributes, actionGetQueueURL, actionPurgeQueue, "ListDeadLetterSourceQueues", actionChangeMessageVisibility},
			wantCode: "OverLimit",
		},
		{
			name:          "no account ids",
			label:         "Label3",
			awsAccountIDs: nil,
			actions:       []string{actionSendMessage},
			wantCode:      errCodeInvalidParameterValue,
		},
		{
			name:          "invalid action",
			label:         "Label4",
			awsAccountIDs: []string{"111111111111"},
			actions:       []string{"CreateQueue"},
			wantCode:      errCodeInvalidParameterValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := s.AddPermission(ctx, queueURL, tt.label, tt.awsAccountIDs, tt.actions)
			expectQueueErrorCode(t, err, tt.wantCode)
		})
	}
}

func TestMemoryStorage_AddPermissionDuplicateLabel(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := t.Context()

	_, err := s.CreateQueue(ctx, "permission-dup-queue", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	queueURL := "http://localhost:4566/000000000000/permission-dup-queue"

	if err := s.AddPermission(ctx, queueURL, "DupLabel", []string{"111111111111"}, []string{actionSendMessage}); err != nil {
		t.Fatalf("AddPermission() error = %v", err)
	}

	err = s.AddPermission(ctx, queueURL, "DupLabel", []string{"222222222222"}, []string{actionReceiveMessage})
	expectQueueErrorCode(t, err, errCodeInvalidParameterValue)
}

func TestMemoryStorage_RemovePermissionUnknownLabel(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")
	ctx := t.Context()

	_, err := s.CreateQueue(ctx, "permission-remove-queue", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	queueURL := "http://localhost:4566/000000000000/permission-remove-queue"

	err = s.RemovePermission(ctx, queueURL, "NoSuchLabel")
	expectQueueErrorCode(t, err, errCodeInvalidParameterValue)
}

func expectQueueErrorCode(t *testing.T, err error, code string) {
	t.Helper()

	var queueErr *QueueError
	if !errors.As(err, &queueErr) {
		t.Fatalf("got err %v, want QueueError code %s", err, code)
	}

	if queueErr.Code != code {
		t.Fatalf("got QueueError code %s, want %s", queueErr.Code, code)
	}
}

func TestMemoryStorage_ReceiveMessageAfterPersistedReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := t.Context()

	s1 := NewMemoryStorage("http://localhost:4566", WithDataDir(dir))
	if _, err := s1.CreateQueue(ctx, "reload-queue", nil, nil); err != nil {
		t.Fatalf("CreateQueue() error = %v", err)
	}

	queueURL := "http://localhost:4566/000000000000/reload-queue"
	if _, err := s1.SendMessage(ctx, queueURL, "hello", 0, nil, "", ""); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	if err := s1.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	s2 := NewMemoryStorage("http://localhost:4566", WithDataDir(dir))

	t.Cleanup(func() { _ = s2.Close() })

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ReceiveMessage() panicked after reload: %v", r)
		}
	}()

	msgs, err := s2.ReceiveMessage(ctx, queueURL, 10, 0, 0)
	if err != nil {
		t.Fatalf("ReceiveMessage() error = %v", err)
	}

	if len(msgs) != 1 || msgs[0].Body != "hello" {
		t.Fatalf("unexpected messages after reload: %#v", msgs)
	}
}

func TestMemoryStorage_FIFODeduplicationCacheAfterReload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := t.Context()

	attrs := map[string]string{"FifoQueue": "true"}

	s1 := NewMemoryStorage("http://localhost:4566", WithDataDir(dir))
	if _, err := s1.CreateQueue(ctx, "reload-queue.fifo", attrs, nil); err != nil {
		t.Fatalf("CreateQueue() error = %v", err)
	}

	queueURL := "http://localhost:4566/000000000000/reload-queue.fifo"
	if _, err := s1.SendMessage(ctx, queueURL, "first", 0, nil, "group-1", "dedup-1"); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	if err := s1.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	s2 := NewMemoryStorage("http://localhost:4566", WithDataDir(dir))

	t.Cleanup(func() { _ = s2.Close() })

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SendMessage() panicked after reload: %v", r)
		}
	}()

	if _, err := s2.SendMessage(ctx, queueURL, "second", 0, nil, "group-1", "dedup-2"); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
}
