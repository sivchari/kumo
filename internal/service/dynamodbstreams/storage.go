package dynamodbstreams

import (
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/streams"
)

const (
	defaultMaxRecords       = 1000
	shardIteratorExpiration = 5 * time.Minute
)

// Storage defines the DynamoDB Streams storage interface.
type Storage interface {
	DescribeStream(streamARN string, limit int32, exclusiveStartShardID string) (*StreamDescription, error)
	GetShardIterator(streamARN, shardID, iteratorType, sequenceNumber string) (string, error)
	GetRecords(shardIterator string, limit int32) ([]RecordOutput, *string, error)
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// shardIteratorData holds shard iterator state.
type shardIteratorData struct {
	streamARN string
	shardID   string
	position  int
	expiresAt time.Time
}

// MemoryStorage implements Storage backed by the shared streams.Store.
type MemoryStorage struct {
	mu             sync.Mutex
	store          *streams.Store
	shardIterators map[string]*shardIteratorData
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(store *streams.Store, _ ...Option) *MemoryStorage {
	return &MemoryStorage{
		store:          store,
		shardIterators: make(map[string]*shardIteratorData),
	}
}

// DescribeStream returns stream metadata from the shared store.
func (m *MemoryStorage) DescribeStream(streamARN string, _ int32, _ string) (*StreamDescription, error) {
	info, shards, err := m.store.DescribeStream(streamARN)
	if err != nil {
		return nil, fmt.Errorf("describe-stream failed: %w", err)
	}

	outShards := make([]Shard, len(shards))
	for i, sh := range shards {
		outShards[i] = Shard{
			ShardID:       sh.ShardID,
			ParentShardID: sh.ParentShardID,
			SequenceNumberRange: SequenceNumberRange{
				StartingSequenceNumber: sh.SequenceNumberRangeStart,
			},
		}
	}

	keySchema := make([]KeySchema, len(info.KeySchema))
	for i, ks := range info.KeySchema {
		keySchema[i] = KeySchema{
			AttributeName: ks.AttributeName,
			KeyType:       ks.KeyType,
		}
	}

	return &StreamDescription{
		StreamArn:               info.StreamARN,
		StreamLabel:             info.StreamLabel,
		StreamStatus:            info.StreamStatus,
		StreamViewType:          info.StreamViewType,
		TableName:               info.TableName,
		KeySchema:               keySchema,
		Shards:                  outShards,
		CreationRequestDateTime: float64(info.CreationTime.Unix()),
	}, nil
}

// GetShardIterator creates a shard iterator for reading stream records.
func (m *MemoryStorage) GetShardIterator(streamARN, shardID, iteratorType, _ string) (string, error) {
	// Verify the stream and shard exist.
	_, shards, err := m.store.DescribeStream(streamARN)
	if err != nil {
		return "", fmt.Errorf("get-shard-iterator failed: %w", err)
	}

	found := false

	for _, sh := range shards {
		if sh.ShardID == shardID {
			found = true

			break
		}
	}

	if !found {
		return "", fmt.Errorf("shard not found: %s", shardID)
	}

	var position int

	switch iteratorType {
	case "TRIM_HORIZON":
		position = 0
	case "LATEST":
		position = m.store.ShardRecordCount(streamARN, shardID)
	default:
		position = 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	iteratorID := fmt.Sprintf("%s:%s:%d:%d", streamARN, shardID, position, time.Now().UnixNano())
	encoded := base64.StdEncoding.EncodeToString([]byte(iteratorID))

	m.shardIterators[encoded] = &shardIteratorData{
		streamARN: streamARN,
		shardID:   shardID,
		position:  position,
		expiresAt: time.Now().Add(shardIteratorExpiration),
	}

	return encoded, nil
}

// GetRecords reads stream records starting from the position encoded in the shard iterator.
func (m *MemoryStorage) GetRecords(shardIterator string, limit int32) ([]RecordOutput, *string, error) {
	m.mu.Lock()

	iterData, exists := m.shardIterators[shardIterator]
	if !exists {
		m.mu.Unlock()

		return nil, nil, fmt.Errorf("invalid shard iterator")
	}

	if time.Now().After(iterData.expiresAt) {
		delete(m.shardIterators, shardIterator)
		m.mu.Unlock()

		return nil, nil, fmt.Errorf("expired shard iterator")
	}

	// Copy iterator data so we can release the lock.
	streamARN := iterData.streamARN
	shardID := iterData.shardID
	position := iterData.position

	m.mu.Unlock()

	if limit <= 0 || limit > int32(defaultMaxRecords) {
		limit = int32(defaultMaxRecords)
	}

	records, nextPos, err := m.store.GetRecords(streamARN, shardID, position, int(limit))
	if err != nil {
		return nil, nil, fmt.Errorf("get-records failed: %w", err)
	}

	// Convert stream records to output format.
	outputs := make([]RecordOutput, len(records))
	for i, rec := range records {
		outputs[i] = convertStreamRecord(rec)
	}

	// Create next iterator.
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.shardIterators, shardIterator)

	nextIteratorID := fmt.Sprintf("%s:%s:%d:%d", streamARN, shardID, nextPos, time.Now().UnixNano())
	nextEncoded := base64.StdEncoding.EncodeToString([]byte(nextIteratorID))

	m.shardIterators[nextEncoded] = &shardIteratorData{
		streamARN: streamARN,
		shardID:   shardID,
		position:  nextPos,
		expiresAt: time.Now().Add(shardIteratorExpiration),
	}

	return outputs, &nextEncoded, nil
}

// convertStreamRecord converts an internal StreamRecord to the output format.
func convertStreamRecord(rec *streams.StreamRecord) RecordOutput {
	return RecordOutput{
		EventID:      rec.EventID,
		EventName:    string(rec.EventName),
		EventVersion: rec.EventVersion,
		EventSource:  rec.EventSource,
		AwsRegion:    rec.AwsRegion,
		Dynamodb: &StreamRecordOutput{
			ApproximateCreationDateTime: float64(rec.CreatedAt.Unix()),
			Keys:                        convertAttributes(rec.Keys),
			NewImage:                    convertAttributes(rec.NewImage),
			OldImage:                    convertAttributes(rec.OldImage),
			SequenceNumber:              rec.SequenceNumber,
			SizeBytes:                   rec.SizeBytes,
			StreamViewType:              rec.StreamViewType,
		},
	}
}

// convertAttributes converts streams.AttributeValue map to AttributeOutput map.
//
//nolint:gocritic // rangeValCopy: copy needed for recursive conversion.
func convertAttributes(attrs map[string]streams.AttributeValue) map[string]AttributeOutput {
	if len(attrs) == 0 {
		return nil
	}

	result := make(map[string]AttributeOutput, len(attrs))

	for k, v := range attrs {
		result[k] = convertSingleAttribute(v)
	}

	return result
}

// convertSingleAttribute converts a single streams.AttributeValue to AttributeOutput.
//
//nolint:gocritic // hugeParam: value needed for recursive conversion.
func convertSingleAttribute(av streams.AttributeValue) AttributeOutput {
	out := make(AttributeOutput)

	switch {
	case av.S != nil:
		out["S"] = *av.S
	case av.N != nil:
		out["N"] = *av.N
	case av.B != nil:
		out["B"] = av.B
	case av.SS != nil:
		out["SS"] = av.SS
	case av.NS != nil:
		out["NS"] = av.NS
	case av.BS != nil:
		out["BS"] = av.BS
	case av.M != nil:
		m := make(map[string]AttributeOutput, len(av.M))

		for k, v := range av.M {
			m[k] = convertSingleAttribute(*v)
		}

		out["M"] = m
	case av.L != nil:
		l := make([]AttributeOutput, len(av.L))

		for i, v := range av.L {
			l[i] = convertSingleAttribute(*v)
		}

		out["L"] = l
	case av.NULL != nil:
		out["NULL"] = *av.NULL
	case av.BOOL != nil:
		out["BOOL"] = *av.BOOL
	}

	return out
}
