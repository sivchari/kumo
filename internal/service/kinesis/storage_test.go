package kinesis

import (
	"errors"
	"testing"
)

func TestPutRecordRejectsInvalidExplicitHashKey(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	createKinesisTestStream(t, store)

	_, _, err := store.PutRecord(t.Context(), "test-stream", []byte("data"), "pk", "not-a-number")
	expectKinesisErrorCode(t, err, errInvalidArgument)
}

func TestPutRecordRejectsEmptyPartitionKey(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	createKinesisTestStream(t, store)

	_, _, err := store.PutRecord(t.Context(), "test-stream", []byte("data"), "", "")
	expectKinesisErrorCode(t, err, errInvalidArgument)
}

func TestPutRecordsRejectsInvalidExplicitHashKeyEntry(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	createKinesisTestStream(t, store)

	results, failed, err := store.PutRecords(t.Context(), "test-stream", []PutRecordsRequestEntry{
		{Data: []byte("bad"), PartitionKey: "pk", ExplicitHashKey: "not-a-number"},
		{Data: []byte("good"), PartitionKey: "pk"},
	})
	if err != nil {
		t.Fatalf("PutRecords: %v", err)
	}

	if failed != 1 {
		t.Fatalf("failed count = %d, want 1", failed)
	}

	if results[0].ErrorCode != errInvalidArgument {
		t.Fatalf("first result error = %q, want %q", results[0].ErrorCode, errInvalidArgument)
	}

	if results[1].ShardID == "" || results[1].SequenceNumber == "" {
		t.Fatalf("second result should succeed: %#v", results[1])
	}
}

func TestPutRecordsRejectsEmptyPartitionKeyEntry(t *testing.T) {
	t.Parallel()

	store := NewMemoryStorage()
	createKinesisTestStream(t, store)

	results, failed, err := store.PutRecords(t.Context(), "test-stream", []PutRecordsRequestEntry{
		{Data: []byte("bad"), PartitionKey: ""},
		{Data: []byte("good"), PartitionKey: "pk"},
	})
	if err != nil {
		t.Fatalf("PutRecords: %v", err)
	}

	if failed != 1 {
		t.Fatalf("failed count = %d, want 1", failed)
	}

	if results[0].ErrorCode != errInvalidArgument {
		t.Fatalf("first result error = %q, want %q", results[0].ErrorCode, errInvalidArgument)
	}

	if results[1].ShardID == "" || results[1].SequenceNumber == "" {
		t.Fatalf("second result should succeed: %#v", results[1])
	}
}

func createKinesisTestStream(t *testing.T, store *MemoryStorage) {
	t.Helper()

	shardCount := int32(2)
	if err := store.CreateStream(t.Context(), &CreateStreamRequest{
		StreamName: "test-stream",
		ShardCount: &shardCount,
	}); err != nil {
		t.Fatalf("CreateStream: %v", err)
	}
}

func expectKinesisErrorCode(t *testing.T, err error, code string) {
	t.Helper()

	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) {
		t.Fatalf("got err %v, want ServiceError code %s", err, code)
	}

	if serviceErr.Code != code {
		t.Fatalf("got ServiceError code %s, want %s", serviceErr.Code, code)
	}
}
