package kinesis

import (
	"testing"
	"testing/synctest"
	"time"
)

// newIteratorTestShard creates a one-record stream and returns a fresh TRIM_HORIZON iterator.
//
// The iterator lifetime is measured from creation, so it must be called inside the synctest bubble.
func newIteratorTestShard(t *testing.T, store *MemoryStorage) string {
	t.Helper()

	shardCount := int32(1)
	if err := store.CreateStream(t.Context(), &CreateStreamRequest{
		StreamName: testStreamName,
		ShardCount: &shardCount,
	}); err != nil {
		t.Fatalf("CreateStream: %v", err)
	}

	shardID, _, err := store.PutRecord(t.Context(), testStreamName, []byte("data"), "pk", "")
	if err != nil {
		t.Fatalf("PutRecord: %v", err)
	}

	iterator, err := store.GetShardIterator(t.Context(), testStreamName, shardID, string(ShardIteratorTypeTrimHorizon), "", 0)
	if err != nil {
		t.Fatalf("GetShardIterator: %v", err)
	}

	return iterator
}

// TestGetRecords_IteratorValidAtExpirationBoundary verifies an iterator is still accepted after exactly 5 minutes.
//
// Expiry is "now after expiresAt", so the boundary itself must not be rejected.
func TestGetRecords_IteratorValidAtExpirationBoundary(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStorage()
		iterator := newIteratorTestShard(t, store)

		time.Sleep(shardIteratorExpiration)

		records, nextIterator, _, err := store.GetRecords(t.Context(), iterator, 0)
		if err != nil {
			t.Fatalf("GetRecords at expiration boundary: %v", err)
		}

		if len(records) != 1 {
			t.Fatalf("got %d records, want 1", len(records))
		}

		if nextIterator == "" {
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
		store := NewMemoryStorage()
		iterator := newIteratorTestShard(t, store)

		time.Sleep(shardIteratorExpiration - time.Second)

		_, nextIterator, _, err := store.GetRecords(t.Context(), iterator, 0)
		if err != nil {
			t.Fatalf("first GetRecords: %v", err)
		}

		time.Sleep(shardIteratorExpiration - time.Second)

		if _, _, _, err := store.GetRecords(t.Context(), nextIterator, 0); err != nil {
			t.Fatalf("second GetRecords with renewed iterator: %v", err)
		}
	})
}

// TestGetRecords_RejectsExpiredIterator verifies an iterator older than 5 minutes yields ExpiredIteratorException.
func TestGetRecords_RejectsExpiredIterator(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStorage()
		iterator := newIteratorTestShard(t, store)

		time.Sleep(shardIteratorExpiration + time.Nanosecond)

		_, _, _, err := store.GetRecords(t.Context(), iterator, 0)
		expectKinesisErrorCode(t, err, errExpiredIterator)
	})
}

// TestGetRecords_DeletesExpiredIterator verifies an expired iterator is dropped on first use.
//
// A retry with the same iterator must fail as invalid rather than expire again.
func TestGetRecords_DeletesExpiredIterator(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStorage()
		iterator := newIteratorTestShard(t, store)

		time.Sleep(shardIteratorExpiration + time.Nanosecond)

		_, _, _, err := store.GetRecords(t.Context(), iterator, 0)
		expectKinesisErrorCode(t, err, errExpiredIterator)

		if _, ok := store.shardIterators[iterator]; ok {
			t.Fatal("expired iterator should be deleted from storage")
		}

		_, _, _, err = store.GetRecords(t.Context(), iterator, 0)
		expectKinesisErrorCode(t, err, errInvalidArgument)
	})
}
