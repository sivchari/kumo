package pipes

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sivchari/kumo/internal/streams"
)

// Poller defaults.
const (
	defaultPollTick   = time.Second
	defaultBatchSize  = 100
	eventBusARNMarker = ":event-bus/"
	ddbServiceMarker  = ":dynamodb:"
	ddbStreamMarker   = "/stream/"
	trimHorizon       = "TRIM_HORIZON"
)

// EventPublisher is the minimal EventBridge surface a DynamoDB-stream pipe
// needs to forward records. It is satisfied by an adapter installed via
// SetEventPublisher in cross-service wiring, avoiding an import of the
// eventbridge package (and an import cycle).
type EventPublisher interface {
	PutEvents(ctx context.Context, busName, source, detailType, detail string) error
}

// SetEventPublisher installs the EventBridge publish target and starts pollers
// for any already-running DynamoDB-stream pipes (e.g. restored from disk).
func (m *MemoryStorage) SetEventPublisher(p EventPublisher) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.publisher = p

	for _, pipe := range m.Pipes {
		m.startPollerLocked(pipe)
	}
}

// startPollerLocked spawns a poller goroutine for a pipe if it is a running
// DynamoDB-stream -> EventBridge-bus pipe and a publisher is installed.
// Must be called while holding m.mu.
func (m *MemoryStorage) startPollerLocked(pipe *Pipe) {
	if m.publisher == nil || pipe.CurrentState != CurrentStateRunning {
		return
	}

	if _, running := m.pollers[pipe.Name]; running {
		return
	}

	poller, ok := newPipePoller(pipe, m.publisher)
	if !ok {
		return
	}

	ctx, cancel := context.WithCancel(m.runCtx)
	m.pollers[pipe.Name] = cancel

	go poller.run(ctx)
}

// stopPollerLocked cancels and forgets a pipe's poller. Must hold m.mu.
func (m *MemoryStorage) stopPollerLocked(name string) {
	if cancel, ok := m.pollers[name]; ok {
		cancel()
		delete(m.pollers, name)
	}
}

// pipePoller is an immutable snapshot of a pipe's config plus per-shard read
// positions. It is owned by a single goroutine, so positions needs no lock.
type pipePoller struct {
	streamARN   string
	busName     string
	source      string
	detailType  string
	batchSize   int
	startLatest bool
	publisher   EventPublisher
	positions   map[string]int // shardID -> next read position
}

// newPipePoller builds a poller from a pipe, returning false when the pipe is
// not a DynamoDB-stream source feeding an EventBridge event bus target.
func newPipePoller(pipe *Pipe, publisher EventPublisher) (*pipePoller, bool) {
	if !isDynamoDBStreamARN(pipe.Source) {
		return nil, false
	}

	busName, ok := eventBusName(pipe.Target)
	if !ok {
		return nil, false
	}

	source, detailType := "", ""
	startLatest := true
	batchSize := defaultBatchSize

	if pipe.TargetParameters != nil && pipe.TargetParameters.EventBridgeEventBusParameters != nil {
		ebp := pipe.TargetParameters.EventBridgeEventBusParameters
		source = ebp.Source
		detailType = ebp.DetailType
	}

	if pipe.SourceParameters != nil && pipe.SourceParameters.DynamoDBStreamParameters != nil {
		ddb := pipe.SourceParameters.DynamoDBStreamParameters
		if ddb.BatchSize > 0 {
			batchSize = int(ddb.BatchSize)
		}

		startLatest = !strings.EqualFold(ddb.StartingPosition, trimHorizon)
	}

	return &pipePoller{
		streamARN:   pipe.Source,
		busName:     busName,
		source:      source,
		detailType:  detailType,
		batchSize:   batchSize,
		startLatest: startLatest,
		publisher:   publisher,
		positions:   make(map[string]int),
	}, true
}

// run polls the source stream until ctx is cancelled.
func (p *pipePoller) run(ctx context.Context) {
	ticker := time.NewTicker(pollTick())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollOnce(ctx)
		}
	}
}

func (p *pipePoller) pollOnce(ctx context.Context) {
	_, shards, err := streams.Global.DescribeStream(p.streamARN)
	if err != nil {
		return
	}

	for _, sh := range shards {
		p.drainShard(ctx, sh.ShardID)
	}
}

// drainShard reads new records from a shard and forwards each as an event.
func (p *pipePoller) drainShard(ctx context.Context, shardID string) {
	pos, seen := p.positions[shardID]
	if !seen {
		pos = p.initialPosition(shardID)
	}

	records, next, err := streams.Global.GetRecords(p.streamARN, shardID, pos, p.batchSize)
	if err != nil {
		return
	}

	for _, rec := range records {
		detail := buildStreamDetail(rec)
		_ = p.publisher.PutEvents(ctx, p.busName, p.source, p.detailType, detail)
	}

	p.positions[shardID] = next
}

// initialPosition returns the starting read offset for a shard. LATEST skips
// records that already exist when the poller starts; TRIM_HORIZON starts at 0.
func (p *pipePoller) initialPosition(shardID string) int {
	if p.startLatest {
		return streams.Global.ShardRecordCount(p.streamARN, shardID)
	}

	return 0
}

// streamDetailEnvelope mirrors the DynamoDB Streams record shape that
// EventBridge Pipes pass to a target as the event detail.
//
//nolint:tagliatelle // AWS DynamoDB Streams uses these exact field names.
type streamDetailEnvelope struct {
	EventID      string             `json:"eventID"`
	EventName    string             `json:"eventName"`
	EventVersion string             `json:"eventVersion,omitempty"`
	EventSource  string             `json:"eventSource,omitempty"`
	AwsRegion    string             `json:"awsRegion,omitempty"`
	Dynamodb     streamDetailRecord `json:"dynamodb"`
}

//nolint:tagliatelle // AWS DynamoDB Streams uses PascalCase for these members.
type streamDetailRecord struct {
	Keys           map[string]streams.AttributeValue `json:"Keys,omitempty"`
	NewImage       map[string]streams.AttributeValue `json:"NewImage,omitempty"`
	OldImage       map[string]streams.AttributeValue `json:"OldImage,omitempty"`
	SequenceNumber string                            `json:"SequenceNumber,omitempty"`
	StreamViewType string                            `json:"StreamViewType,omitempty"`
	SizeBytes      int64                             `json:"SizeBytes,omitempty"`
}

func buildStreamDetail(rec *streams.StreamRecord) string {
	env := streamDetailEnvelope{
		EventID:      rec.EventID,
		EventName:    string(rec.EventName),
		EventVersion: rec.EventVersion,
		EventSource:  rec.EventSource,
		AwsRegion:    rec.AwsRegion,
		Dynamodb: streamDetailRecord{
			Keys:           rec.Keys,
			NewImage:       rec.NewImage,
			OldImage:       rec.OldImage,
			SequenceNumber: rec.SequenceNumber,
			StreamViewType: rec.StreamViewType,
			SizeBytes:      rec.SizeBytes,
		},
	}

	body, err := json.Marshal(env)
	if err != nil {
		return "{}"
	}

	return string(body)
}

func isDynamoDBStreamARN(arn string) bool {
	return strings.Contains(arn, ddbServiceMarker) && strings.Contains(arn, ddbStreamMarker)
}

// eventBusName extracts the bus name from an EventBridge event-bus ARN.
func eventBusName(arn string) (string, bool) {
	_, name, found := strings.Cut(arn, eventBusARNMarker)
	if !found {
		return "", false
	}

	return name, true
}

// pollTick is the poller loop interval. KUMO_PIPES_POLL_INTERVAL_MS overrides
// it so tests need not wait a full second.
func pollTick() time.Duration {
	v := os.Getenv("KUMO_PIPES_POLL_INTERVAL_MS")
	if v == "" {
		return defaultPollTick
	}

	ms, err := strconv.Atoi(v)
	if err != nil || ms <= 0 {
		return defaultPollTick
	}

	return time.Duration(ms) * time.Millisecond
}
