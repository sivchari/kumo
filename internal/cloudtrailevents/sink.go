// Package cloudtrailevents provides a process-global sink of management API
// calls for CloudTrail delivery. The server dispatcher records calls here (only
// while a trail is logging, gated by a cheap atomic flag), and the CloudTrail
// service drains them to write gzipped log files into S3. Keeping the sink in
// its own package lets both the server and the cloudtrail service reference it
// without an import cycle (mirroring internal/streams).
package cloudtrailevents

import (
	"sync"
	"sync/atomic"
	"time"
)

// maxBuffered bounds the in-memory ring so a long-running, never-drained sink
// cannot grow without limit.
const maxBuffered = 5000

// Event is a single recorded management API call.
type Event struct {
	EventTime   time.Time
	EventSource string // e.g. "dynamodb.amazonaws.com"
	EventName   string // e.g. "PutItem"
	AwsRegion   string
	SourceIP    string
	UserAgent   string
	RequestID   string
}

// Sink is a concurrency-safe, bounded buffer of recorded events.
type Sink struct {
	mu      sync.Mutex
	events  []Event
	logging atomic.Bool
}

// Global is the default sink shared by the server and the cloudtrail service.
var Global = &Sink{} //nolint:gochecknoglobals // single shared sink, mirrors streams.Global

// SetLogging enables or disables recording. When disabled, Record is a no-op
// after a single atomic load, so the request hot path pays almost nothing.
func (s *Sink) SetLogging(on bool) {
	s.logging.Store(on)
}

// Logging reports whether recording is currently enabled.
func (s *Sink) Logging() bool {
	return s.logging.Load()
}

// Record appends an event when logging is enabled. The oldest events are
// dropped once the buffer exceeds maxBuffered.
func (s *Sink) Record(e *Event) {
	if !s.logging.Load() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, *e)
	if len(s.events) > maxBuffered {
		s.events = s.events[len(s.events)-maxBuffered:]
	}
}

// Drain returns all buffered events and clears the buffer.
func (s *Sink) Drain() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.events) == 0 {
		return nil
	}

	out := s.events
	s.events = nil

	return out
}
