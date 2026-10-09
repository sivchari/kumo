package dynamodb

import (
	"context"
	"strconv"
	"testing"
	"testing/synctest"
	"time"
)

const (
	ttlTestTable = "ttl-test"
	ttlTestAttr  = "expires_at"
	ttlTestPK    = "pk"

	// ttlReaperInterval mirrors the ticker period in ttlReaper.
	ttlReaperInterval = 30 * time.Second
)

// newTTLTestStorage creates a storage with a single table inside a synctest bubble.
// It omits dataDir so that no snapshotter ticker outlives the bubble.
func newTTLTestStorage(t *testing.T) *MemoryStorage {
	t.Helper()

	store := NewMemoryStorage("http://localhost:4566")

	_, err := store.CreateTable(context.Background(), &CreateTableRequest{
		TableName:            ttlTestTable,
		KeySchema:            []KeySchemaElement{{AttributeName: ttlTestPK, KeyType: keyTypeHash}},
		AttributeDefinitions: []AttributeDefinition{{AttributeName: ttlTestPK, AttributeType: "S"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	return store
}

func putTTLItem(t *testing.T, store *MemoryStorage, pk string, expiresAt int64) {
	t.Helper()

	_, err := store.PutItem(context.Background(), ttlTestTable, Item{
		ttlTestPK:   {S: new(pk)},
		ttlTestAttr: {N: new(strconv.FormatInt(expiresAt, 10))},
	}, false, ConditionInput{})
	if err != nil {
		t.Fatal(err)
	}
}

func hasTTLItem(t *testing.T, store *MemoryStorage, pk string) bool {
	t.Helper()

	item, err := store.GetItem(context.Background(), ttlTestTable, Item{ttlTestPK: {S: new(pk)}})
	if err != nil {
		t.Fatal(err)
	}

	return item != nil
}

func TestTTLReaper_DeletesExpiredItems(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := newTTLTestStorage(t)
		defer func() { _ = store.Close() }()

		if err := store.UpdateTimeToLive(context.Background(), ttlTestTable, ttlTestAttr, true); err != nil {
			t.Fatal(err)
		}

		now := time.Now().Unix()
		putTTLItem(t, store, "expired", now-1)
		putTTLItem(t, store, "alive", now+3600)

		time.Sleep(ttlReaperInterval)
		synctest.Wait()

		if hasTTLItem(t, store, "expired") {
			t.Error("expired item must be deleted by the reaper")
		}

		if !hasTTLItem(t, store, "alive") {
			t.Error("unexpired item must be kept by the reaper")
		}
	})
}

func TestTTLReaper_DisabledKeepsExpiredItems(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := newTTLTestStorage(t)
		defer func() { _ = store.Close() }()

		putTTLItem(t, store, "expired", time.Now().Unix()-1)

		time.Sleep(ttlReaperInterval)
		synctest.Wait()

		if !hasTTLItem(t, store, "expired") {
			t.Error("item must be kept when TTL is not enabled on the table")
		}
	})
}

// TestTTLReaper_CloseStopsReaper relies on synctest.Test waiting for every goroutine in the bubble:
// it returns only if Close stops the reaper goroutine, otherwise the bubble deadlocks.
func TestTTLReaper_CloseStopsReaper(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		store := newTTLTestStorage(t)

		if err := store.Close(); err != nil {
			t.Fatal(err)
		}

		if err := store.Close(); err != nil {
			t.Fatalf("second Close must be a no-op: %v", err)
		}
	})
}
