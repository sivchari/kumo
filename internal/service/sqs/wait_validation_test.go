package sqs

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestMemoryStorage_receiveWaitTime_Range(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")

	queue, err := s.CreateQueue(t.Context(), "wait-range-queue", nil, nil)
	if err != nil {
		t.Fatalf("CreateQueue() error = %v", err)
	}

	tests := []struct {
		name    string
		wait    int
		wantErr bool
	}{
		{name: "below the range", wait: -1, wantErr: true},
		{name: "lower bound", wait: 0},
		{name: "upper bound", wait: 20},
		{name: "above the range", wait: 21, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := s.receiveWaitTime(queue.URL, new(tt.wait))

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("receiveWaitTime(%d) error = %v", tt.wait, err)
				}

				if got != tt.wait {
					t.Errorf("receiveWaitTime(%d) = %d, want %d", tt.wait, got, tt.wait)
				}

				return
			}

			var qErr *QueueError
			if !errors.As(err, &qErr) || qErr.Code != errCodeInvalidParameterValue {
				t.Fatalf("receiveWaitTime(%d) error = %v, want a %s QueueError", tt.wait, err, errCodeInvalidParameterValue)
			}
		})
	}
}

func TestService_ReceiveMessage_RejectsOutOfRangeWaitTimeSeconds(t *testing.T) {
	t.Parallel()

	storage := NewMemoryStorage("http://localhost:4566")

	queue, err := storage.CreateQueue(t.Context(), "wait-handler-queue", nil, nil)
	if err != nil {
		t.Fatalf("CreateQueue() error = %v", err)
	}

	svc := New(storage, "http://localhost:4566")

	for _, wait := range []int{-1, 21} {
		body := fmt.Sprintf(`{"QueueUrl":%q,"WaitTimeSeconds":%d}`, queue.URL, wait)
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
		rec := httptest.NewRecorder()

		svc.ReceiveMessage(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("WaitTimeSeconds=%d: status = %d, want %d", wait, rec.Code, http.StatusBadRequest)
		}

		if !strings.Contains(rec.Body.String(), errCodeInvalidParameterValue) {
			t.Errorf("WaitTimeSeconds=%d: body = %s, want it to carry %s", wait, rec.Body.String(), errCodeInvalidParameterValue)
		}
	}
}

func TestMemoryStorage_QueueWaitTimeAttribute_Range(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "below the range", value: "-1", wantErr: true},
		{name: "lower bound", value: "0"},
		{name: "upper bound", value: "20"},
		{name: "above the range", value: "21", wantErr: true},
		{name: "not a number", value: "soon", wantErr: true},
		{name: "empty", value: "", wantErr: true},
	}

	for _, tt := range tests {
		attrs := map[string]string{receiveWaitAttribute: tt.value}

		t.Run("CreateQueue/"+tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewMemoryStorage("http://localhost:4566")

			_, err := s.CreateQueue(t.Context(), "attr-create-queue", attrs, nil)
			assertAttributeResult(t, err, tt.wantErr)
		})

		t.Run("SetQueueAttributes/"+tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewMemoryStorage("http://localhost:4566")

			queue, err := s.CreateQueue(t.Context(), "attr-set-queue", map[string]string{receiveWaitAttribute: "5"}, nil)
			if err != nil {
				t.Fatalf("CreateQueue() error = %v", err)
			}

			err = s.SetQueueAttributes(t.Context(), queue.URL, attrs)
			assertAttributeResult(t, err, tt.wantErr)

			want := 5

			if !tt.wantErr {
				want, err = strconv.Atoi(tt.value)
				if err != nil {
					t.Fatalf("test value %q is not an integer: %v", tt.value, err)
				}
			}

			if got := queueAttributeInt(t, s, queue.URL, receiveWaitAttribute); got != want {
				t.Errorf("%s = %d after SetQueueAttributes(%q), want %d", receiveWaitAttribute, got, tt.value, want)
			}
		})
	}
}

func TestMemoryStorage_SetQueueAttributes_RejectedCallAppliesNothing(t *testing.T) {
	t.Parallel()

	s := NewMemoryStorage("http://localhost:4566")

	queue, err := s.CreateQueue(t.Context(), "atomic-queue", nil, nil)
	if err != nil {
		t.Fatalf("CreateQueue() error = %v", err)
	}

	err = s.SetQueueAttributes(t.Context(), queue.URL, map[string]string{
		"VisibilityTimeout":  "60",
		receiveWaitAttribute: "21",
	})
	if err == nil {
		t.Fatal("SetQueueAttributes() accepted an out-of-range ReceiveMessageWaitTimeSeconds")
	}

	if got := queueAttributeInt(t, s, queue.URL, "VisibilityTimeout"); got != 30 {
		t.Errorf("VisibilityTimeout = %d after a rejected call, want the default 30", got)
	}
}

// assertAttributeResult fails the test unless err is an InvalidAttributeValue
// QueueError exactly when wantErr is set.
func assertAttributeResult(t *testing.T, err error, wantErr bool) {
	t.Helper()

	if !wantErr {
		if err != nil {
			t.Fatalf("error = %v, want none", err)
		}

		return
	}

	var qErr *QueueError
	if !errors.As(err, &qErr) || qErr.Code != errCodeInvalidAttributeValue {
		t.Fatalf("error = %v, want a %s QueueError", err, errCodeInvalidAttributeValue)
	}
}
