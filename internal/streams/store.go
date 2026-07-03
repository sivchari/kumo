// Package streams provides a shared event store for DynamoDB Streams.
// Both the dynamodb service (producer) and the dynamodbstreams service
// (consumer) reference this package to avoid circular imports.
package streams

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// OperationType represents the type of DynamoDB stream event.
type OperationType string

// Operation type constants.
const (
	OperationTypeInsert OperationType = "INSERT"
	OperationTypeModify OperationType = "MODIFY"
	OperationTypeRemove OperationType = "REMOVE"
)

// AttributeValue mirrors DynamoDB's AttributeValue for stream records.
// We use a simplified representation to avoid importing the dynamodb package.
//
//nolint:tagliatelle // AWS DynamoDB uses PascalCase attribute type fields.
type AttributeValue struct {
	S    *string                    `json:"S,omitempty"`
	N    *string                    `json:"N,omitempty"`
	B    []byte                     `json:"B,omitempty"`
	SS   []string                   `json:"SS,omitempty"`
	NS   []string                   `json:"NS,omitempty"`
	BS   [][]byte                   `json:"BS,omitempty"`
	M    map[string]*AttributeValue `json:"M,omitempty"`
	L    []*AttributeValue          `json:"L,omitempty"`
	NULL *bool                      `json:"NULL,omitempty"`
	BOOL *bool                      `json:"BOOL,omitempty"`
}

// StreamRecord represents a single change event in a DynamoDB stream.
type StreamRecord struct {
	EventID        string
	EventName      OperationType
	EventVersion   string
	EventSource    string
	AwsRegion      string
	StreamViewType string
	TableName      string
	StreamARN      string
	Keys           map[string]AttributeValue
	NewImage       map[string]AttributeValue
	OldImage       map[string]AttributeValue
	SequenceNumber string
	CreatedAt      time.Time
	SizeBytes      int64
}

// StreamInfo holds metadata about a stream.
type StreamInfo struct {
	StreamARN      string
	TableName      string
	StreamViewType string
	StreamLabel    string
	StreamStatus   string
	KeySchema      []KeySchemaElement
	CreationTime   time.Time
}

// KeySchemaElement represents a key schema element.
type KeySchemaElement struct {
	AttributeName string
	KeyType       string // HASH or RANGE
}

// ShardInfo holds metadata about a shard within a stream.
type ShardInfo struct {
	ShardID                  string
	ParentShardID            string
	SequenceNumberRangeStart string
}

// Store is a global, concurrency-safe store for DynamoDB stream events.
// DynamoDB storage writes events here; DynamoDB Streams reads them.
type Store struct {
	mu              sync.RWMutex
	streams         map[string]*streamData // keyed by streamARN
	sequenceCounter uint64
}

type streamData struct {
	info   StreamInfo
	shards []*shardData
}

type shardData struct {
	info    ShardInfo
	records []*StreamRecord
}

// Global is the default global Store instance shared between
// the dynamodb and dynamodbstreams services.
var Global = NewStore()

// NewStore creates a new Store.
func NewStore() *Store {
	return &Store{
		streams: make(map[string]*streamData),
	}
}

// RegisterStream registers a stream for a table. Called when a DynamoDB table
// with StreamSpecification is created.
func (s *Store) RegisterStream(info *StreamInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.streams[info.StreamARN]; exists {
		return
	}

	seq := s.nextSequenceNumber()
	sd := &streamData{
		info: *info,
		shards: []*shardData{
			{
				info: ShardInfo{
					ShardID:                  "shardId-000000000000",
					SequenceNumberRangeStart: seq,
				},
				records: make([]*StreamRecord, 0),
			},
		},
	}

	s.streams[info.StreamARN] = sd
}

// PutRecord adds a stream record. Called by DynamoDB storage on item mutations.
func (s *Store) PutRecord(record *StreamRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sd, exists := s.streams[record.StreamARN]
	if !exists {
		return
	}

	record.SequenceNumber = s.nextSequenceNumber()
	record.CreatedAt = time.Now()
	record.EventVersion = "1.1"
	record.EventSource = "aws:dynamodb"

	// Append to the last (active) shard.
	activeShard := sd.shards[len(sd.shards)-1]
	activeShard.records = append(activeShard.records, record)
}

// DescribeStream returns stream info and shard list for the given ARN.
func (s *Store) DescribeStream(streamARN string) (*StreamInfo, []ShardInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sd, exists := s.streams[streamARN]
	if !exists {
		return nil, nil, fmt.Errorf("stream not found: %s", streamARN)
	}

	shards := make([]ShardInfo, len(sd.shards))
	for i, sh := range sd.shards {
		shards[i] = sh.info
	}

	return &sd.info, shards, nil
}

// GetRecords returns records from a given shard starting at the specified position.
func (s *Store) GetRecords(streamARN, shardID string, position, limit int) ([]*StreamRecord, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sd, exists := s.streams[streamARN]
	if !exists {
		return nil, 0, fmt.Errorf("stream not found: %s", streamARN)
	}

	for _, sh := range sd.shards {
		if sh.info.ShardID != shardID {
			continue
		}

		if position >= len(sh.records) {
			return nil, position, nil
		}

		end := position + limit
		if end > len(sh.records) {
			end = len(sh.records)
		}

		result := make([]*StreamRecord, end-position)
		copy(result, sh.records[position:end])

		return result, end, nil
	}

	return nil, 0, fmt.Errorf("shard not found: %s", shardID)
}

// ShardRecordCount returns the number of records in a shard (used for iterator positioning).
func (s *Store) ShardRecordCount(streamARN, shardID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sd, exists := s.streams[streamARN]
	if !exists {
		return 0
	}

	for _, sh := range sd.shards {
		if sh.info.ShardID == shardID {
			return len(sh.records)
		}
	}

	return 0
}

func (s *Store) nextSequenceNumber() string {
	seq := atomic.AddUint64(&s.sequenceCounter, 1)

	return fmt.Sprintf("%021d", seq)
}
