package cloudtrailevents

import (
	"strconv"
	"testing"
	"time"
)

// newEvent builds a recordable event tagged by RequestID so tests can assert
// ordering and identity after a Drain. It takes no *testing.T (thelper).
func newEvent(id string) *Event {
	return &Event{
		EventTime:   time.Unix(1700000000, 0).UTC(),
		EventSource: "dynamodb.amazonaws.com",
		EventName:   "PutItem",
		AwsRegion:   "us-east-1",
		SourceIP:    "127.0.0.1",
		UserAgent:   "aws-sdk-go-v2",
		RequestID:   id,
	}
}

func TestSink_LoggingToggle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		set  bool
		want bool
	}{
		{"enable", true, true},
		{"disable", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := &Sink{}

			if s.Logging() {
				t.Fatalf("new sink: Logging() = true, want false")
			}

			s.SetLogging(tc.set)

			if got := s.Logging(); got != tc.want {
				t.Fatalf("after SetLogging(%v): Logging() = %v, want %v", tc.set, got, tc.want)
			}
		})
	}
}

func TestRecord_NoopWhenLoggingDisabled(t *testing.T) {
	t.Parallel()

	s := &Sink{} // logging defaults to false

	s.Record(newEvent("a"))

	if got := s.Drain(); got != nil {
		t.Fatalf("Drain() after disabled Record = %v, want nil", got)
	}
}

func TestRecord_AppendsInOrderWhenLoggingEnabled(t *testing.T) {
	t.Parallel()

	s := &Sink{}
	s.SetLogging(true)

	want := newEvent("first")
	s.Record(want)
	s.Record(newEvent("second"))

	got := s.Drain()
	if len(got) != 2 {
		t.Fatalf("Drain() len = %d, want 2", len(got))
	}

	if got[0].RequestID != "first" || got[1].RequestID != "second" {
		t.Fatalf("Drain() order = [%s %s], want [first second]", got[0].RequestID, got[1].RequestID)
	}

	// Every field must round-trip through the sink unchanged.
	if first := got[0]; !first.EventTime.Equal(want.EventTime) ||
		first.EventSource != want.EventSource ||
		first.EventName != want.EventName ||
		first.AwsRegion != want.AwsRegion ||
		first.SourceIP != want.SourceIP ||
		first.UserAgent != want.UserAgent {
		t.Fatalf("Drain()[0] = %+v, want %+v", first, *want)
	}
}

func TestRecord_DropsOldestPastMaxBuffered(t *testing.T) {
	t.Parallel()

	s := &Sink{}
	s.SetLogging(true)

	const extra = 10

	total := maxBuffered + extra
	for i := range total {
		s.Record(newEvent(strconv.Itoa(i)))
	}

	got := s.Drain()
	if len(got) != maxBuffered {
		t.Fatalf("Drain() len = %d, want %d", len(got), maxBuffered)
	}

	// The oldest `extra` events are dropped; the buffer keeps the newest window.
	if first := got[0].RequestID; first != strconv.Itoa(extra) {
		t.Fatalf("oldest retained RequestID = %s, want %d", first, extra)
	}

	if last := got[len(got)-1].RequestID; last != strconv.Itoa(total-1) {
		t.Fatalf("newest retained RequestID = %s, want %d", last, total-1)
	}
}

func TestDrain_EmptyReturnsNil(t *testing.T) {
	t.Parallel()

	s := &Sink{}

	if got := s.Drain(); got != nil {
		t.Fatalf("Drain() on empty sink = %v, want nil", got)
	}
}

func TestDrain_ClearsBuffer(t *testing.T) {
	t.Parallel()

	s := &Sink{}
	s.SetLogging(true)

	s.Record(newEvent("a"))
	s.Record(newEvent("b"))

	if got := s.Drain(); len(got) != 2 {
		t.Fatalf("first Drain() len = %d, want 2", len(got))
	}

	if got := s.Drain(); got != nil {
		t.Fatalf("second Drain() = %v, want nil (buffer should be cleared)", got)
	}
}
