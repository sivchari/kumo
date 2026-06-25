package pipes

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sivchari/kumo/internal/streams"
)

const (
	testBusName    = "audit-bus"
	testBusARN     = "arn:aws:events:us-east-1:000000000000:event-bus/audit-bus"
	testEventSrc   = "authz.dynamodb"
	testDetailType = "permission-change"
)

// capturedEvent records one PutEvents call to the fake publisher.
type capturedEvent struct {
	bus        string
	source     string
	detailType string
	detail     string
}

type fakePublisher struct {
	mu     sync.Mutex
	events []capturedEvent
}

func (f *fakePublisher) PutEvents(_ context.Context, busName, source, detailType, detail string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.events = append(f.events, capturedEvent{bus: busName, source: source, detailType: detailType, detail: detail})

	return nil
}

func (f *fakePublisher) captured() []capturedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]capturedEvent(nil), f.events...)
}

func strptr(s string) *string {
	return &s
}

// registerTestStream registers a unique stream so parallel tests don't collide
// on the shared streams.Global store, and returns its ARN.
func registerTestStream(t *testing.T) string {
	t.Helper()

	arn := "arn:aws:dynamodb:us-east-1:000000000000:table/auth_permissions/stream/" + t.Name()
	streams.Global.RegisterStream(&streams.StreamInfo{
		StreamARN:      arn,
		TableName:      "auth_permissions",
		StreamViewType: "NEW_AND_OLD_IMAGES",
	})

	return arn
}

func putTestRecord(t *testing.T, arn, pk string) {
	t.Helper()

	streams.Global.PutRecord(&streams.StreamRecord{
		EventName:      streams.OperationTypeModify,
		StreamViewType: "NEW_AND_OLD_IMAGES",
		StreamARN:      arn,
		Keys:           map[string]streams.AttributeValue{"pk": {S: strptr(pk)}},
		NewImage:       map[string]streams.AttributeValue{"pk": {S: strptr(pk)}},
	})
}

func newTestPoller(arn string, pub EventPublisher, startLatest bool) (*pipePoller, bool) {
	start := "TRIM_HORIZON"
	if startLatest {
		start = "LATEST"
	}

	pipe := &Pipe{
		Source: arn,
		Target: testBusARN,
		SourceParameters: &SourceParameters{
			DynamoDBStreamParameters: &DynamoDBStreamSourceParameters{
				BatchSize:        10,
				StartingPosition: start,
			},
		},
		TargetParameters: &TargetParameters{
			EventBridgeEventBusParameters: &EventBridgeTargetParameters{
				Source:     testEventSrc,
				DetailType: testDetailType,
			},
		},
	}

	return newPipePoller(pipe, pub)
}

func TestNewPipePoller_Eligibility(t *testing.T) {
	t.Parallel()

	const (
		ddbStream = "arn:aws:dynamodb:us-east-1:000000000000:table/t/stream/2024"
		busArn    = "arn:aws:events:us-east-1:000000000000:event-bus/b"
		lambdaArn = "arn:aws:lambda:us-east-1:000000000000:function:f"
	)

	cases := []struct {
		name   string
		source string
		target string
		wantOK bool
	}{
		{"ddb stream to bus", ddbStream, busArn, true},
		{"non-ddb source", "arn:aws:sqs:us-east-1:000000000000:q", busArn, false},
		{"non-bus target", ddbStream, lambdaArn, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pipe := &Pipe{Source: tc.source, Target: tc.target}

			_, ok := newPipePoller(pipe, &fakePublisher{})
			if ok != tc.wantOK {
				t.Fatalf("newPipePoller ok: got %v, want %v", ok, tc.wantOK)
			}
		})
	}
}

func TestPipePoller_ForwardsRecords(t *testing.T) {
	t.Parallel()

	arn := registerTestStream(t)
	putTestRecord(t, arn, "COGNITO_USER#demo")
	putTestRecord(t, arn, "COGNITO_USER#alice")

	pub := &fakePublisher{}

	poller, ok := newTestPoller(arn, pub, false)
	if !ok {
		t.Fatalf("newPipePoller: expected eligible poller")
	}

	poller.pollOnce(t.Context())

	events := pub.captured()
	if len(events) != 2 {
		t.Fatalf("PutEvents calls: got %d, want 2", len(events))
	}

	first := events[0]
	if first.bus != testBusName {
		t.Errorf("bus: got %q, want %q", first.bus, testBusName)
	}

	if first.source != testEventSrc {
		t.Errorf("source: got %q, want %q", first.source, testEventSrc)
	}

	if first.detailType != testDetailType {
		t.Errorf("detailType: got %q, want %q", first.detailType, testDetailType)
	}

	if !strings.Contains(first.detail, "COGNITO_USER#demo") {
		t.Errorf("detail: got %q, want it to contain the record key", first.detail)
	}

	if !strings.Contains(first.detail, `"eventName":"MODIFY"`) {
		t.Errorf("detail: got %q, want it to contain the eventName", first.detail)
	}
}

func TestPipePoller_LatestSkipsExisting(t *testing.T) {
	t.Parallel()

	arn := registerTestStream(t)
	putTestRecord(t, arn, "pre-existing")

	pub := &fakePublisher{}

	poller, ok := newTestPoller(arn, pub, true)
	if !ok {
		t.Fatalf("newPipePoller: expected eligible poller")
	}

	// First poll establishes the LATEST position past the pre-existing record.
	poller.pollOnce(t.Context())

	if got := len(pub.captured()); got != 0 {
		t.Fatalf("PutEvents after first poll: got %d, want 0 (LATEST skips existing)", got)
	}

	putTestRecord(t, arn, "new-after-start")
	poller.pollOnce(t.Context())

	events := pub.captured()
	if len(events) != 1 {
		t.Fatalf("PutEvents calls: got %d, want 1", len(events))
	}

	if !strings.Contains(events[0].detail, "new-after-start") {
		t.Errorf("detail: got %q, want the post-start record", events[0].detail)
	}
}

// runningStreamPipe builds a running DynamoDB-stream -> EventBridge-bus pipe,
// the only shape startPollerLocked spawns a poller for.
func runningStreamPipe(name, arn string) *Pipe {
	return &Pipe{
		Name:         name,
		Source:       arn,
		Target:       testBusARN,
		CurrentState: CurrentStateRunning,
		SourceParameters: &SourceParameters{
			DynamoDBStreamParameters: &DynamoDBStreamSourceParameters{
				BatchSize:        10,
				StartingPosition: trimHorizon,
			},
		},
		TargetParameters: &TargetParameters{
			EventBridgeEventBusParameters: &EventBridgeTargetParameters{
				Source:     testEventSrc,
				DetailType: testDetailType,
			},
		},
	}
}

// waitForEvents polls the fake publisher until it has captured want events or
// the deadline passes (the poller forwards from a background goroutine).
func waitForEvents(t *testing.T, pub *fakePublisher, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(pub.captured()) >= want {
			return
		}

		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("waiting for %d events: got %d", want, len(pub.captured()))
}

func TestPollTick(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want time.Duration
	}{
		{"empty uses default", "", defaultPollTick},
		{"valid override", "50", 50 * time.Millisecond},
		{"non-numeric falls back", "abc", defaultPollTick},
		{"zero falls back", "0", defaultPollTick},
		{"negative falls back", "-5", defaultPollTick},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUMO_PIPES_POLL_INTERVAL_MS", tc.env)

			if got := pollTick(); got != tc.want {
				t.Fatalf("pollTick(): got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPipePoller_PollOnceUnknownStream(t *testing.T) {
	t.Parallel()

	// An ARN in DynamoDB-stream shape that was never registered: DescribeStream
	// returns an error and pollOnce must bail out without publishing.
	arn := "arn:aws:dynamodb:us-east-1:000000000000:table/missing/stream/" + t.Name()

	pub := &fakePublisher{}

	poller, ok := newTestPoller(arn, pub, false)
	if !ok {
		t.Fatalf("newPipePoller: expected eligible poller")
	}

	poller.pollOnce(t.Context())

	if got := len(pub.captured()); got != 0 {
		t.Fatalf("PutEvents: got %d, want 0 (unknown stream)", got)
	}
}

func TestPipePoller_DrainUnknownShard(t *testing.T) {
	t.Parallel()

	arn := registerTestStream(t)
	putTestRecord(t, arn, "rec")

	pub := &fakePublisher{}

	poller, ok := newTestPoller(arn, pub, false)
	if !ok {
		t.Fatalf("newPipePoller: expected eligible poller")
	}

	// A shard DescribeStream never returns: GetRecords errors and drainShard
	// returns before publishing or recording a position.
	poller.drainShard(t.Context(), "shardId-nonexistent")

	if got := len(pub.captured()); got != 0 {
		t.Fatalf("PutEvents: got %d, want 0 (unknown shard)", got)
	}
}

func TestStartPollerLocked(t *testing.T) {
	t.Parallel()

	arn := registerTestStream(t)

	cases := []struct {
		name       string
		publisher  EventPublisher
		mutate     func(*Pipe)
		wantPoller bool
	}{
		{"no publisher installed", nil, nil, false},
		{"pipe not running", &fakePublisher{}, func(p *Pipe) { p.CurrentState = CurrentStateStopped }, false},
		{"source not a ddb stream", &fakePublisher{}, func(p *Pipe) { p.Source = "arn:aws:sqs:us-east-1:000000000000:q" }, false},
		{"eligible running pipe", &fakePublisher{}, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := NewMemoryStorage()

			t.Cleanup(func() { _ = m.Close() })

			m.publisher = tc.publisher

			pipe := runningStreamPipe("p", arn)
			if tc.mutate != nil {
				tc.mutate(pipe)
			}

			m.mu.Lock()
			m.startPollerLocked(pipe)
			_, present := m.pollers[pipe.Name]
			m.mu.Unlock()

			if present != tc.wantPoller {
				t.Fatalf("poller present: got %v, want %v", present, tc.wantPoller)
			}
		})
	}
}

func TestStartStopPollerLocked_Idempotent(t *testing.T) {
	t.Parallel()

	arn := registerTestStream(t)

	m := NewMemoryStorage()

	t.Cleanup(func() { _ = m.Close() })

	m.publisher = &fakePublisher{}
	pipe := runningStreamPipe("p", arn)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.startPollerLocked(pipe)

	// A second start is a no-op while one is already running.
	m.startPollerLocked(pipe)

	if _, present := m.pollers[pipe.Name]; !present {
		t.Fatal("poller missing after start")
	}

	m.stopPollerLocked(pipe.Name)

	if _, present := m.pollers[pipe.Name]; present {
		t.Fatal("poller still present after stop")
	}

	// Stopping an unknown poller is a no-op.
	m.stopPollerLocked("does-not-exist")
}

func TestMemoryStorage_SetEventPublisherStartsPoller(t *testing.T) {
	t.Setenv("KUMO_PIPES_POLL_INTERVAL_MS", "5")

	arn := registerTestStream(t)
	putTestRecord(t, arn, "restored-record")

	m := NewMemoryStorage()

	t.Cleanup(func() { _ = m.Close() })

	// A running pipe as if restored from disk before wiring installs a publisher.
	m.Pipes["restored"] = runningStreamPipe("restored", arn)

	pub := &fakePublisher{}
	m.SetEventPublisher(pub)

	waitForEvents(t, pub, 1)

	if got := pub.captured()[0].detail; !strings.Contains(got, "restored-record") {
		t.Errorf("detail: got %q, want it to contain the record key", got)
	}
}

func TestEventBusName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		arn     string
		want    string
		wantHit bool
	}{
		{"event bus arn", testBusARN, testBusName, true},
		{"not an event bus", "arn:aws:lambda:us-east-1:000000000000:function:f", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := eventBusName(tc.arn)
			if ok != tc.wantHit || got != tc.want {
				t.Fatalf("eventBusName(%q): got (%q,%v), want (%q,%v)", tc.arn, got, ok, tc.want, tc.wantHit)
			}
		})
	}
}
