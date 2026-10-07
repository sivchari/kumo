package dynamodbstreams

import (
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/sivchari/kumo/internal/streams"
)

const (
	testStreamARN = "arn:aws:dynamodb:us-east-1:000000000000:table/test-table/stream/2000-01-01T00:00:00.000"
	testShardID   = "shardId-000000000000"

	errMsgExpired = "expired shard iterator"
	errMsgInvalid = "invalid shard iterator"
)

// newIteratorTestStorage returns a storage holding one record and a fresh TRIM_HORIZON iterator.
//
// It uses a private streams.Store instead of streams.Global, which is shared process-wide,
// so the tests stay isolated from each other and can run in parallel.
// It must be called inside the synctest bubble because the iterator lifetime starts at creation.
func newIteratorTestStorage(t *testing.T) (*MemoryStorage, string) {
	t.Helper()

	store := streams.NewStore()
	store.RegisterStream(&streams.StreamInfo{
		StreamARN:      testStreamARN,
		TableName:      "test-table",
		StreamViewType: "NEW_AND_OLD_IMAGES",
		StreamStatus:   "ENABLED",
	})
	store.PutRecord(&streams.StreamRecord{
		StreamARN: testStreamARN,
		EventName: streams.OperationTypeInsert,
	})

	storage := NewMemoryStorage(store)

	iterator, err := storage.GetShardIterator(testStreamARN, testShardID, "TRIM_HORIZON", "")
	if err != nil {
		t.Fatalf("GetShardIterator: %v", err)
	}

	return storage, iterator
}

func expectIteratorError(t *testing.T, err error, msg string) {
	t.Helper()

	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Fatalf("got err %v, want message containing %q", err, msg)
	}
}

// TestGetRecords_IteratorValidAtExpirationBoundary verifies an iterator is still accepted after exactly 5 minutes.
//
// Expiry is "now after expiresAt", so the boundary itself must not be rejected.
func TestGetRecords_IteratorValidAtExpirationBoundary(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		storage, iterator := newIteratorTestStorage(t)

		time.Sleep(shardIteratorExpiration)

		records, nextIterator, err := storage.GetRecords(iterator, 0)
		if err != nil {
			t.Fatalf("GetRecords at expiration boundary: %v", err)
		}

		if len(records) != 1 {
			t.Fatalf("got %d records, want 1", len(records))
		}

		if nextIterator == nil || *nextIterator == "" {
			t.Fatal("next iterator should be issued")
		}
	})
}

// TestGetRecords_NextIteratorHasFreshExpiration verifies the next iterator gets its own 5 minute lifetime.
//
// A consumer polling every few minutes must not be cut off by the first iterator's deadline.
func TestGetRecords_NextIteratorHasFreshExpiration(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		storage, iterator := newIteratorTestStorage(t)

		time.Sleep(shardIteratorExpiration - time.Second)

		_, nextIterator, err := storage.GetRecords(iterator, 0)
		if err != nil {
			t.Fatalf("first GetRecords: %v", err)
		}

		time.Sleep(shardIteratorExpiration - time.Second)

		if _, _, err := storage.GetRecords(*nextIterator, 0); err != nil {
			t.Fatalf("second GetRecords with renewed iterator: %v", err)
		}
	})
}

// TestGetRecords_RejectsExpiredIterator verifies an iterator older than 5 minutes is rejected as expired.
func TestGetRecords_RejectsExpiredIterator(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		storage, iterator := newIteratorTestStorage(t)

		time.Sleep(shardIteratorExpiration + time.Nanosecond)

		_, _, err := storage.GetRecords(iterator, 0)
		expectIteratorError(t, err, errMsgExpired)
	})
}

// TestGetRecords_DeletesExpiredIterator verifies an expired iterator is dropped on first use.
//
// A retry with the same iterator must fail as invalid rather than expire again.
func TestGetRecords_DeletesExpiredIterator(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		storage, iterator := newIteratorTestStorage(t)

		time.Sleep(shardIteratorExpiration + time.Nanosecond)

		_, _, err := storage.GetRecords(iterator, 0)
		expectIteratorError(t, err, errMsgExpired)

		if _, ok := storage.shardIterators[iterator]; ok {
			t.Fatal("expired iterator should be deleted from storage")
		}

		_, _, err = storage.GetRecords(iterator, 0)
		expectIteratorError(t, err, errMsgInvalid)
	})
}
